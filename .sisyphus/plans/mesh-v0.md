# Mesh v0 — Snapshot, Restore, Clone

## TL;DR

> **Quick Summary**: Build a Go CLI tool that snapshots AI agent filesystems, restores them locally or remotely, and clones them to other machines via SSH. Proves the core primitive (tar + zstd + hash = portable agent state) that survives into v1 daemon/MCP.
>
> **Deliverables**:
> - Go binary `mesh` with 7 CLI commands: snapshot, restore, clone, status, inspect, prune, list
> - Library packages under `internal/` with clean contracts (survives into v1)
> - Hash round-trip test proving filesystem integrity through the full pipeline
> - TOML config for agents, machines, and hooks
> - CI pipeline (GitHub Actions) for build + test + lint
> - Single-binary distribution for linux/amd64
>
> **Estimated Effort**: Large
> **Parallel Execution**: YES — 4 waves
> **Critical Path**: Task 1 (scaffold) → Task 2 (hash round-trip) → Task 3 (snapshot) → Task 4 (restore) → Task 5 (clone) → Task 7 (CLI wire) → Task 10 (CI)

---

## Context

### Original Request
Implement Mesh v0. Start with the hash round-trip test. Based on design doc from office-hours session + CEO plan + eng review.

### Interview Summary
**Key Discussions**:
- Module path: `github.com/rethink-paradigms/mesh` (user confirmed)
- Go 1.25.5 available on darwin/arm64 (dev), target linux/amd64 (prod)
- All 7 CLI commands in scope (CEO plan is explicit)
- Hash round-trip test is the FIRST code after scaffolding
- Library-first architecture, CLI is thin wrapper

**Research Findings**:
- Greenfield Go project — zero .go files exist
- Old `.github/workflows/` from dead "lightweight K8s" era must be replaced
- Discovery docs (10 research files, 10 decisions, 6 constraints) define full architecture
- v0 deliberately deviates from D2 (no OCI images, bare directories) and D5 (CLI only, no MCP)

### Metis Review
**Identified Gaps** (all addressed):
- Old CI workflows: Delete in scaffolding task, replace with Go CI
- Module path: Resolved — `github.com/rethink-paradigms/mesh`
- Tar determinism: Must use `filepath.WalkDir` with sorted entries for reproducible hashes
- Snapshot storage: `~/.mesh/snapshots/{agent_name}/` — explicitly stated in design doc
- Agent identification: By name from config — explicitly stated in design doc
- Hooks: `pre_snapshot_cmd`/`post_restore_cmd` in agent config, failure aborts operation — CEO plan explicit
- Error cleanup: Transactional — remove all temp files on any failure in pipeline
- All 7 commands in scope — design doc and CEO plan are explicit

---

## Work Objectives

### Core Objective
Build a Go CLI that proves the core primitive: an agent's filesystem can be faithfully captured, compressed, hashed, moved, and restored with byte-perfect integrity. Everything else (clone, status, hooks) is sugar on this primitive.

### Concrete Deliverables
- `cmd/mesh/main.go` — CLI entrypoint with 7 subcommands
- `internal/snapshot/` — tar + zstd + hash + store + prune
- `internal/restore/` — verify hash + extract + atomic rename
- `internal/clone/` — orchestrates snapshot + transport + restore
- `internal/config/` — TOML parsing + validation
- `internal/agent/` — process management (pgrep, SIGTERM, start)
- `internal/transport/` — SSH transport (scp, remote exec via os/exec)
- `internal/manifest/` — JSON manifest per snapshot
- `.github/workflows/ci.yml` — Go CI pipeline
- `go.mod`, `go.sum` — dependency management

### Definition of Done
- [ ] `go build ./cmd/mesh/` produces working binary
- [ ] `go test -race ./...` passes with 0 failures
- [ ] `golangci-lint run` passes
- [ ] Hash round-trip test passes: tar → compress → hash → verify → decompress → extract → identical filesystem
- [ ] All 7 CLI commands work: snapshot, restore, clone, status, inspect, prune, list
- [ ] Clone works end-to-end: local snapshot + restore to different directory (local clone)

### Must Have
- Deterministic tar ordering (sorted filepath.WalkDir) — same dir always produces same hash
- Streaming pipeline: tar → zstd → sha256, no intermediate files for core pipeline
- SHA-256 integrity verification on every restore
- JSON manifest per snapshot (agent name, timestamp, source, size, checksum)
- Transactional cleanup on failure (remove temp files, partial extractions)
- Extract to temp dir + atomic rename for restore (EXDEV fallback for cross-mount)
- All tests hermetic (temp dirs, no network, no Docker)
- `go test -race ./...` passes with zero races

### Deep Design Cross-Validation (v0 vs. Full Architecture)

The `discovery/design/deep/` folder contains detailed specifications for the full 6-module architecture. v0 is a minimal vertical slice. Here's where v0's implementation MUST align with (or deliberately deviate from) the deep design:

| Deep Design Concept | v0 Implementation | Alignment |
|---------------------|-------------------|-----------|
| **persistence.md INV-1**: Complete snapshot or error | v0: Write `.json` manifest first, then `.tar.zst`. If tarball fails, manifest marks snapshot incomplete. Restore checks manifest exists AND tarball exists. | ✅ ALIGNED — manifest-first pattern applies to local filesystem too |
| **persistence.md INV-4**: Bounded memory (streaming) | v0: `io.Pipe` pipeline — tar → zstd → sha256 in one pass. No buffering. | ✅ ALIGNED — critical for 2GB VMs |
| **persistence.md**: `StorageBackend.Put(ref, manifest, tarball)` accepts `io.Reader` | v0: `CreateSnapshot` writes streaming tarball. No `[]byte` buffering. | ✅ ALIGNED |
| **persistence.md**: Snapshot pipeline = `export → zstd → storage.Put` | v0: `filepath.WalkDir → tar → zstd → file` (local filesystem, not Docker export) | ⚠️ DELIBERATE DEVIATION — v0 tars directories, not containers. Pipeline shape is identical. |
| **persistence.md**: Manifest schema (body_id, platform, metadata) | v0: Manifest = `{agent_name, timestamp, source_machine, source_workdir, start_cmd, stop_timeout, checksum, size}`. No `platform` field (no cross-arch in v0). | ⚠️ SIMPLIFIED — no platform, no container metadata (cmd, env, workdir, user, ports) |
| **persistence.md EC5**: A4 Burst Clone FS delta merge = full re-tarball | v0: Clone = snapshot + restore. No delta merge. Full re-tarball. | ✅ ALIGNED — v0 is full re-tarball only |
| **orchestration.md**: Body state machine (Created→Starting→Running→Stopped→Destroyed) | v0: No state machine. Agent = name + config. Process is running or not (pgrep). | ⚠️ DELIBERATE DEVIATION — v0 has no persistent body tracking |
| **orchestration.md INV-3**: Destroy is idempotent | v0: Prune removes files. Re-pruning already-pruned = no-op. | ✅ ALIGNED |
| **orchestration.md**: Body-level mutex for concurrent operations | v0: Single-user CLI. No concurrent operation protection. Document as known limitation. | ⚠️ DEFERRED — acceptable for v0 single-user context |
| **interface.md EC4**: Bootstrap via CLI, not MCP | v0: CLI only. No MCP. CLI IS the interface. | ✅ ALIGNED — v0 is the bootstrap escape hatch |
| **interface.md**: Error codes (INSTANCE_NOT_FOUND, INVALID_STATE, etc.) | v0: Plain error messages to stderr. Exit code 1. No structured error codes. | ⚠️ SIMPLIFIED — but error messages should be clear enough that structured codes can wrap them in v1 |
| **provisioning.md**: SubstrateAdapter 6 verbs (create, start, stop, destroy, getStatus, exec) | v0: `internal/agent/` handles stop (SIGTERM) and start (exec). No create/destroy (no containers). No getStatus (just pgrep). | ⚠️ PARTIAL — v0 agent package should design stop/start API to be replaceable by substrate adapter in v1 |
| **plugin-infrastructure.md**: go-plugin, gRPC, subprocess isolation | v0: None. No plugins. | ✅ ALIGNED — v0 has zero plugin infrastructure |

**Key principle for executors**: When implementing v0 packages, design the internal APIs so they can be *replaced* by the deep design interfaces in v1 without rewriting callers. For example, `internal/snapshot.CreateSnapshot()` should have a signature that could become `SnapshotEngine.Capture()` in v1. `internal/agent.Stop()` maps to `SubstrateAdapter.stop()` in v1. The CLI stays the same; the backend swaps from local filesystem to substrate adapters.

### Must NOT Have (Guardrails)
- **NO Docker/container runtime dependency** — v0 operates on bare directories, not containers
- **NO CGo** — pure Go only, single-binary distribution
- **NO Go SSH libraries** — use `os/exec` to call `scp`/`ssh` directly
- **NO JSON output mode, color output, or table formatting** — plain text to stdout, errors to stderr
- **NO Windows support** — linux/amd64 target, darwin/arm64 for dev
- **NO gRPC, protobuf, container runtime libraries** — nothing from the full architecture
- **NO telemetry, login, phone-home** — constraint C4
- **NO config schema versioning or migration** — v0 format is what it is
- **NO modification of files in `discovery/`** — that's a separate workflow
- **NO agent restart after snapshot** — agent management is optional (stop only, no auto-restart)
- **NO concurrency/locking in snapshot cache** — single-user tool, no concurrent snapshot guards
- **NO .meshignore** — deferred to post-v0 (in TODOS.md)
- **NO content-addressed storage** — simple timestamped filenames in v0

---

## Verification Strategy

> **ZERO HUMAN INTERVENTION** — ALL verification is agent-executed. No exceptions.

### Test Decision
- **Infrastructure exists**: NO (greenfield)
- **Automated tests**: YES (TDD — hash round-trip test first, then unit tests per package)
- **Framework**: Go standard testing (`go test`)
- **TDD flow**: Each internal package gets tests written alongside or before implementation

### QA Policy
Every task MUST include agent-executed QA scenarios.
Evidence saved to `.sisyphus/evidence/task-{N}-{scenario-slug}.{ext}`.

- **Go library/module**: Use Bash (`go test -v -race ./...`) — run tests, assert pass/fail
- **CLI binary**: Use Bash (`go build && ./mesh --help`) — build, run commands, check output
- **Integration**: Use Bash (`go test -tags=integration`) — local clone round-trip

---

## Execution Strategy

### Parallel Execution Waves

```
Wave 1 (Start Immediately — foundation + core primitive):
├── Task 1: Go module init + project scaffold + old CI cleanup [quick]
├── Task 2: Hash round-trip test + snapshot pipeline core [deep]
└── Task 3: TOML config parsing + validation [quick]

Wave 2 (After Wave 1 — vertical slices, MAX PARALLEL):
├── Task 4: Snapshot command (depends: 2, 3) [deep]
├── Task 5: Restore command (depends: 2, 3) [deep]
├── Task 6: Agent process management (depends: 3) [quick]
├── Task 7: Manifest package (depends: 2) [quick]

Wave 3 (After Wave 2 — integration + remaining commands):
├── Task 8: Clone command (depends: 4, 5, 6) [deep]
├── Task 9: CLI wire-up — Cobra subcommands (depends: 4, 5, 8) [unspecified-high]
├── Task 10: Operational commands — status, list, inspect, prune (depends: 4, 7, 9) [unspecified-high]
├── Task 11: Pre/post hooks (depends: 4, 5, 3) [quick]

Wave 4 (After Wave 3 — hardening + CI):
├── Task 12: GitHub Actions CI (depends: 9) [quick]
├── Task 13: README + install docs (depends: 9, 10) [writing]

Wave FINAL (After ALL tasks — 4 parallel reviews):
├── Task F1: Plan compliance audit (oracle)
├── Task F2: Code quality review (unspecified-high)
├── Task F3: Real manual QA (unspecified-high)
└── Task F4: Scope fidelity check (deep)
-> Present results -> Get explicit user okay

Critical Path: T1 → T2 → T4 → T5 → T8 → T9 → T10 → F1-F4
Parallel Speedup: ~50% faster than sequential
Max Concurrent: 4 (Wave 2)
```

### Dependency Matrix

| Task | Depends On | Blocks | Wave |
|------|-----------|--------|------|
| 1    | —         | 2, 3   | 1    |
| 2    | 1         | 4, 5, 7 | 1   |
| 3    | 1         | 4, 5, 6, 11 | 1 |
| 4    | 2, 3      | 8, 9, 10 | 2   |
| 5    | 2, 3      | 8, 9    | 2    |
| 6    | 3         | 8       | 2    |
| 7    | 2         | 10      | 2    |
| 8    | 4, 5, 6   | 9       | 3    |
| 9    | 4, 5, 8   | 10, 12, 13 | 3 |
| 10   | 4, 7, 9   | F1-F4   | 3    |
| 11   | 4, 5, 3   | F1-F4   | 3    |
| 12   | 9         | F1-F4   | 4    |
| 13   | 9, 10     | F1-F4   | 4    |

### Agent Dispatch Summary

- **Wave 1**: **3** — T1 → `quick`, T2 → `deep`, T3 → `quick`
- **Wave 2**: **4** — T4 → `deep`, T5 → `deep`, T6 → `quick`, T7 → `quick`
- **Wave 3**: **4** — T8 → `deep`, T9 → `unspecified-high`, T10 → `unspecified-high`, T11 → `quick`
- **Wave 4**: **2** — T12 → `quick`, T13 → `writing`
- **FINAL**: **4** — F1 → `oracle`, F2 → `unspecified-high`, F3 → `unspecified-high`, F4 → `deep`

---

## TODOs

- [x] 1. **Go Module Init + Project Scaffold + Old CI Cleanup**

  **What to do**:
  - Run `go mod init github.com/rethink-paradigms/mesh` in project root
  - Create directory structure: `cmd/mesh/`, `internal/snapshot/`, `internal/restore/`, `internal/clone/`, `internal/config/`, `internal/agent/`, `internal/transport/`, `internal/manifest/`
  - Add a minimal `cmd/mesh/main.go` that prints version and exits (proves compilation works)
  - Delete all old `.github/workflows/*.yml` files (publish.yml, reusable-docker-build.yml, reusable-nomad-deploy.yml, test.yml, docs.yml)
  - Create fresh `.github/workflows/ci.yml` with: `go fmt`, `go vet`, `go test -race -coverprofile=coverage.out ./...`, `golangci-lint run`, `go build ./cmd/mesh/`
  - Add `.golangci.yml` with sensible defaults (enable: errcheck, govet, staticcheck, unused, gosimple, ineffassign)
  - Add a minimal `internal/snapshot/CONTEXT.md`, `internal/restore/CONTEXT.md`, etc. with one-line description of each package's responsibility
  - Verify: `go build ./cmd/mesh/` succeeds, `go test ./...` succeeds (no tests yet, just compilation)

  **Must NOT do**:
  - Do not import any external dependencies yet (klauspost, toml, cobra come later)
  - Do not modify any files in `discovery/`
  - Do not add a README yet (that's Task 13)

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Pure scaffolding — directory creation, file deletion, boilerplate. No logic.
  - **Skills**: `[]`
  - **Skills Evaluated but Omitted**:
    - `git-master`: Not needed — no complex git operations

  **Parallelization**:
  - **Can Run In Parallel**: NO (foundation task)
  - **Parallel Group**: Wave 1 (alone, or with T2/T3 if they don't depend on go.mod existing)
  - **Blocks**: Tasks 2, 3
  - **Blocked By**: None

  **References**:
  **Pattern References**:
  - `.github/workflows/` — Old CI workflows to DELETE (publish.yml, reusable-docker-build.yml, reusable-nomad-deploy.yml, test.yml, docs.yml). These are from the dead "lightweight K8s" framing.
  - `.gitignore` — May need updating for Go artifacts (*.exe, mesh binary, coverage.out)

  **External References**:
  - Go module init: `go mod init github.com/rethink-paradigms/mesh`
  - Standard Go project layout: `cmd/` for binaries, `internal/` for packages

  **Acceptance Criteria**:
  - [ ] `go build ./cmd/mesh/` exits 0
  - [ ] `go test ./...` exits 0 (no tests, just compilation check)
  - [ ] Old `.github/workflows/*.yml` files deleted
  - [ ] New `.github/workflows/ci.yml` exists with Go CI pipeline
  - [ ] `.golangci.yml` exists
  - [ ] All `internal/` subdirectories exist with CONTEXT.md files

  **QA Scenarios:**

  ```
  Scenario: Build succeeds
    Tool: Bash
    Preconditions: go.mod exists with module path github.com/rethink-paradigms/mesh
    Steps:
      1. Run: go build ./cmd/mesh/
      2. Check exit code is 0
      3. Run: ./mesh --version
      4. Check output contains "mesh" or version string
    Expected Result: Binary compiles and runs
    Failure Indicators: Build fails, import errors
    Evidence: .sisyphus/evidence/task-1-build.txt

  Scenario: Old CI workflows deleted
    Tool: Bash
    Preconditions: None
    Steps:
      1. Run: ls .github/workflows/
      2. Assert: only ci.yml exists (no publish.yml, reusable-docker-build.yml, etc.)
    Expected Result: Only ci.yml in .github/workflows/
    Failure Indicators: Old workflow files still present
    Evidence: .sisyphus/evidence/task-1-ci-cleanup.txt
  ```

  **Commit**: YES
  - Message: `feat(init): scaffold Go project with module, dirs, and CI`
  - Files: go.mod, go.sum, cmd/mesh/main.go, internal/*/, .github/workflows/ci.yml, .golangci.yml

- [x] 2. **Hash Round-Trip Test + Snapshot Pipeline Core**

  **What to do**:
  - Add `go get github.com/klauspost/compress/zstd` dependency
  - In `internal/snapshot/`, implement the core streaming pipeline:
    - `CreateSnapshot(ctx context.Context, workdir string, outputPath string) error` — tars workdir, compresses with zstd, hashes with SHA-256, writes to outputPath
    - Use `archive/tar` + `filepath.WalkDir` with **sorted entries** for deterministic output
    - Use `io.Pipe` for streaming: tar writer → pipe → zstd writer + sha256 hash tee → file
    - Write `.sha256` sidecar file containing the hex-encoded hash
  - In `internal/snapshot/snapshot_test.go`, write the **hash round-trip test**:
    1. Create temp dir with known files (regular files, subdirs, symlinks, files with permissions 0755/0600, empty files, Unicode filenames, nested symlinks)
    2. Call `CreateSnapshot` to produce `.tar.zst` + `.sha256`
    3. Read the `.sha256` file, compute SHA-256 of the `.tar.zst` independently, verify match
    4. Decompress the `.tar.zst` to get raw tar
    5. Extract the tar to a new temp dir
    6. Compare original dir vs restored dir: file contents (byte-for-byte), permissions, symlink targets, directory structure
  - Add determinism test: snapshot same dir twice, verify hashes are identical
  - Add tests for: empty directory, large file (>1MB), permission preservation, symlink preservation
  - All tests hermetic: use `t.TempDir()`, clean up automatically

  **Must NOT do**:
  - Do NOT write CLI commands (that's Task 4+)
  - Do NOT add config parsing (that's Task 3)
  - Do NOT add manifest generation (that's Task 7)
  - Do NOT use CGo or external binaries

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Core algorithm — streaming pipeline with tar, zstd, sha256, determinism guarantees. Must get io.Pipe + tee pattern right. Tests are the product here.
  - **Skills**: `[]`
  - **Skills Evaluated but Omitted**:
    - `python-development:python-pro`: Wrong language
    - `code-refactoring:code-reviewer`: Not reviewing, building from scratch

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Task 3 if go.mod exists)
  - **Parallel Group**: Wave 1 (with Task 3)
  - **Blocks**: Tasks 4, 5, 7
  - **Blocked By**: Task 1

  **References**:
  **External References**:
  - `github.com/klauspost/compress/zstd` — Pure Go zstd. API: `zstd.NewWriter(w)` for compression, `zstd.NewReader(r)` for decompression. Supports `io.Writer`/`io.Reader` streaming.
  - Go `archive/tar` — `tar.NewWriter(w)`, `tar.FileInfoHeader(fi, link)` for creating headers from `os.FileInfo`. Uses PAX format by default (handles long paths, large files).
  - Go `crypto/sha256` — `sha256.New()` returns hash.Hash which implements `io.Writer`. Use `io.TeeReader` or `io.MultiWriter` to hash while writing.
  - Go `filepath.WalkDir` — Returns entries in lexical order within each directory. Use `sort.Slice` on `DirEntry` slice if additional ordering needed.

  **Pattern References**:
  - Design doc "Snapshot Protocol" section: tar the agent's working directory, SHA-256 hash, store tarball in `~/.mesh/snapshots/{agent_name}/`
  - Eng review decision #2: "Tar on agent machine (local I/O, fast), transfer tarball to control machine via SCP"
  - Eng review decision #8: "Test strategy: Hash round-trip test (tar → compress → hash → verify → decompress → extract → compare) is the critical path, built first"

  **Why Each Reference Matters**:
  - klauspost/compress/zstd: The ONLY external dependency in the core pipeline. Must understand its streaming API to avoid buffering entire tarball in memory.
  - filepath.WalkDir sorting: Critical for determinism — same directory must always produce identical hash. Without sorting, filesystem-dependent ordering breaks hash stability.
  - io.Pipe + io.MultiWriter: The streaming architecture — tar → pipe → zstd + sha256 in one pass. No intermediate files.

  **Acceptance Criteria**:
  - [ ] `go test ./internal/snapshot/ -run TestHashRoundTrip -v` → PASS
  - [ ] `go test ./internal/snapshot/ -run TestDeterministicHash -v` → PASS (same dir, two snapshots, hashes match)
  - [ ] `go test ./internal/snapshot/ -run TestEmptyDirectory -v` → PASS
  - [ ] `go test ./internal/snapshot/ -run TestPermissionPreservation -v` → PASS
  - [ ] `go test ./internal/snapshot/ -run TestSymlinkPreservation -v` → PASS
  - [ ] `go test -race ./internal/snapshot/` → PASS (0 races)

  **QA Scenarios:**

  ```
  Scenario: Full hash round-trip integrity
    Tool: Bash
    Preconditions: Task 1 complete (go.mod exists)
    Steps:
      1. Run: go test ./internal/snapshot/ -run TestHashRoundTrip -v -count=1
      2. Assert: exit code 0
      3. Assert: output contains "PASS"
      4. Assert: output does NOT contain "FAIL"
    Expected Result: Files survive tar→zstd→hash→verify→decompress→extract with byte-perfect fidelity
    Failure Indicators: Hash mismatch, missing files, wrong permissions, broken symlinks
    Evidence: .sisyphus/evidence/task-2-round-trip.txt

  Scenario: Deterministic hash (same dir, same hash)
    Tool: Bash
    Preconditions: Task 1 complete
    Steps:
      1. Run: go test ./internal/snapshot/ -run TestDeterministicHash -v -count=1
      2. Assert: exit code 0
      3. Assert: output contains two hashes that are identical
    Expected Result: Two snapshots of same directory produce identical SHA-256 hashes
    Failure Indicators: Hashes differ between two runs
    Evidence: .sisyphus/evidence/task-2-determinism.txt

  Scenario: Race-free under concurrent access
    Tool: Bash
    Preconditions: Task 1 complete
    Steps:
      1. Run: go test -race ./internal/snapshot/ -count=1
      2. Assert: exit code 0
      3. Assert: output does NOT contain "DATA RACE"
    Expected Result: No data races detected
    Failure Indicators: Race detector reports data race
    Evidence: .sisyphus/evidence/task-2-race.txt
  ```

  **Commit**: YES
  - Message: `feat(snapshot): add hash round-trip test and streaming pipeline`
  - Files: internal/snapshot/*.go, internal/snapshot/*_test.go, go.mod (updated with klauspost dep)

- [x] 3. **TOML Config Parsing + Validation**

  **What to do**:
  - Add `go get github.com/BurntSushi/toml` dependency
  - In `internal/config/`, implement:
    - Go structs for config: `Config`, `Machine`, `Agent` matching the schema from the design doc
    - `Load(path string) (*Config, error)` — parse TOML file into struct
    - `Validate(cfg *Config) error` — validate: machine references exist, SSH key files exist with correct permissions (0600), no duplicate agent names, workdir paths are valid
    - Default values: `port = 22`, `stop_signal = "SIGTERM"`, `stop_timeout = "30s"`, `max_snapshots = 10`
    - `DefaultPath() string` — returns `~/.mesh/config.toml` (expand `~` to home dir)
    - Override via `--config` flag or `MESH_CONFIG` env var
  - Config struct per design doc:
    ```go
    type Config struct {
        Machines []Machine `toml:"machines"`
        Agents   []Agent   `toml:"agents"`
    }
    type Machine struct {
        Name    string `toml:"name"`
        Host    string `toml:"host"`
        Port    int    `toml:"port"`
        User    string `toml:"user"`
        SSHKey  string `toml:"ssh_key"`
        AgentDir string `toml:"agent_dir"`
    }
    type Agent struct {
        Name         string `toml:"name"`
        Machine      string `toml:"machine"`
        Workdir      string `toml:"workdir"`
        StartCmd     string `toml:"start_cmd"`
        StopSignal   string `toml:"stop_signal"`
        StopTimeout  string `toml:"stop_timeout"`
        MaxSnapshots int    `toml:"max_snapshots"`
        PIDFile      string `toml:"pid_file"`
        PreSnapshotCmd  string `toml:"pre_snapshot_cmd"`
        PostRestoreCmd  string `toml:"post_restore_cmd"`
    }
    ```
  - Tests: valid config, missing required fields, duplicate agent names, non-existent machine reference, non-existent SSH key, env var override, `~` expansion in paths

  **Must NOT do**:
  - Do NOT add CLI flags (that's Task 9)
  - Do NOT validate SSH connectivity (just check key file exists)
  - Do NOT add config schema versioning

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Standard Go struct + TOML parsing + validation. Well-understood pattern.
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Task 2)
  - **Parallel Group**: Wave 1 (with Task 2)
  - **Blocks**: Tasks 4, 5, 6, 11
  - **Blocked By**: Task 1

  **References**:
  **External References**:
  - `github.com/BurntSushi/toml` — Standard Go TOML library. API: `toml.DecodeFile(path, &cfg)` for parsing, `toml.Encode(w, &cfg)` for writing.
  - Design doc config schema section — exact field names, types, defaults

  **Pattern References**:
  - Design doc "Config Schema (Stage 1)": exact TOML layout with `[[machines]]` and `[[agents]]` arrays
  - Eng review decision: "Config validation: At startup, validate: machine references exist, SSH key files exist with correct permissions, no duplicate agent names, workdir paths are valid"
  - CEO plan hooks: `pre_snapshot_cmd`, `post_restore_cmd` in agent config

  **Why Each Reference Matters**:
  - BurntSushi/toml: The TOML parser. Must use `toml.DecodeFile` for loading. Struct tags map TOML keys to Go fields.
  - Config schema: Every CLI command reads config. Getting field names and types right here is foundational for Tasks 4-11.

  **Acceptance Criteria**:
  - [ ] `go test ./internal/config/ -v -count=1` → PASS (all config tests)
  - [ ] `go test -race ./internal/config/` → PASS
  - [ ] Valid config parses without error
  - [ ] Missing required field returns descriptive error
  - [ ] Duplicate agent name returns descriptive error
  - [ ] Non-existent machine reference returns descriptive error
  - [ ] `~` in paths expanded to home directory

  **QA Scenarios:**

  ```
  Scenario: Valid config loads and validates
    Tool: Bash
    Preconditions: Task 1 complete
    Steps:
      1. Run: go test ./internal/config/ -run TestLoadValidConfig -v -count=1
      2. Assert: exit code 0, output contains "PASS"
    Expected Result: TOML with machines and agents parses into struct with defaults applied
    Failure Indicators: Parse error, missing defaults, wrong types
    Evidence: .sisyphus/evidence/task-3-valid-config.txt

  Scenario: Invalid configs rejected with clear errors
    Tool: Bash
    Preconditions: Task 1 complete
    Steps:
      1. Run: go test ./internal/config/ -run TestValidation -v -count=1
      2. Assert: exit code 0, each subtest checks a specific validation error
    Expected Result: Duplicate names, missing machines, bad SSH keys all rejected
    Failure Indicators: Invalid config accepted without error
    Evidence: .sisyphus/evidence/task-3-validation.txt
  ```

  **Commit**: YES
  - Message: `feat(config): add TOML config parsing and validation`
  - Files: internal/config/*.go, internal/config/*_test.go, go.mod (updated with toml dep)

- [x] 4. **Snapshot Command (Full Workflow)**

  **What to do**:
  - In `internal/snapshot/`, expand beyond the core pipeline to add the full snapshot workflow:
    - `Run(ctx context.Context, cfg *config.Config, agentName string) error` — orchestrates the full snapshot
    - Resolve agent name to config.Agent
    - Create snapshot cache dir: `~/.mesh/snapshots/{agent_name}/`
    - Call `CreateSnapshot` with the agent's workdir
    - Generate timestamped filename: `{agent_name}-{YYYYMMDD-HHMMSS}.tar.zst`
    - Enforce `max_snapshots` limit: prune oldest snapshots if count exceeds config value
    - Return error if workdir doesn't exist or isn't readable
  - Tests: snapshot creates files in correct directory, enforces max_snapshots, handles non-existent workdir, handles unreadable workdir, filename format is correct

  **Must NOT do**:
  - Do NOT stop/restart the agent process (that's Task 6 integration, not this task)
  - Do NOT call pre/post hooks (that's Task 11)
  - Do NOT add CLI wiring (that's Task 9)

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Integrates config + core pipeline + file management. Business logic with edge cases.
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Tasks 5, 6, 7)
  - **Parallel Group**: Wave 2
  - **Blocks**: Tasks 8, 9, 10
  - **Blocked By**: Tasks 2, 3

  **References**:
  **Pattern References**:
  - `internal/snapshot/` (from Task 2) — Core pipeline (`CreateSnapshot`) to call from `Run`
  - `internal/config/` (from Task 3) — Config struct to read agent config from

  **API/Type References**:
  - `config.Agent` — Contains `Name`, `Workdir`, `MaxSnapshots` fields needed by snapshot
  - Design doc: "Store tarball in local snapshot cache at `~/.mesh/snapshots/{agent_name}/`"

  **Why Each Reference Matters**:
  - `CreateSnapshot` is the core function from Task 2. `Run` wraps it with config resolution, path management, and pruning.
  - `config.Agent` provides workdir path, max_snapshots count — must read these from config, not hardcode.

  **Acceptance Criteria**:
  - [ ] `go test ./internal/snapshot/ -run TestRun -v -count=1` → PASS
  - [ ] `go test ./internal/snapshot/ -run TestMaxSnapshots -v -count=1` → PASS
  - [ ] `go test ./internal/snapshot/ -run TestNonExistentWorkdir -v -count=1` → PASS
  - [ ] `go test -race ./internal/snapshot/` → PASS

  **QA Scenarios:**

  ```
  Scenario: Snapshot creates correct file structure
    Tool: Bash
    Preconditions: Tasks 2 and 3 complete
    Steps:
      1. Run: go test ./internal/snapshot/ -run TestRun -v -count=1
      2. Assert: exit code 0, output shows .tar.zst and .sha256 files created in snapshot cache dir
    Expected Result: Snapshot files created at ~/.mesh/snapshots/{agent}/ with correct naming
    Failure Indicators: Wrong directory, wrong filename format, missing sha256 sidecar
    Evidence: .sisyphus/evidence/task-4-snapshot-run.txt

  Scenario: Max snapshots enforced
    Tool: Bash
    Preconditions: Tasks 2 and 3 complete
    Steps:
      1. Run: go test ./internal/snapshot/ -run TestMaxSnapshots -v -count=1
      2. Assert: after exceeding max_snapshots, oldest snapshot is removed
    Expected Result: Only max_snapshots most recent snapshots remain
    Failure Indicators: Old snapshots not pruned, count exceeds limit
    Evidence: .sisyphus/evidence/task-4-max-snapshots.txt
  ```

  **Commit**: YES
  - Message: `feat(snapshot): add full snapshot workflow with config and pruning`
  - Files: internal/snapshot/snapshot.go, internal/snapshot/snapshot_test.go

- [x] 5. **Restore Command (Full Workflow)**

  **What to do**:
  - In `internal/restore/`, implement:
    - `Restore(ctx context.Context, snapshotPath string, targetDir string) error` — verify hash, extract to temp, atomic rename
    - `VerifyHash(tarPath string, hashPath string) error` — read .sha256 file, compute SHA-256 of .tar.zst, compare
    - Extract pipeline: decompress .tar.zst → extract tar to temp dir (same filesystem as target) → `os.Rename` temp dir to target dir
    - EXDEV fallback: if `os.Rename` fails with `EXDEV`, fall back to recursive copy + delete temp
    - Transactional cleanup: on any failure, remove temp extraction dir
    - Pre-flight checks: verify target filesystem has enough disk space (statfs), verify target parent dir is writable
  - Tests: restore produces identical filesystem, hash mismatch rejected, disk space check, atomic rename, EXDEV fallback, cleanup on failure, restore to non-empty target (overwrite behavior: atomic swap means old content replaced), restore from non-existent snapshot path

  **Must NOT do**:
  - Do NOT start the agent after restore (that's Task 6 integration)
  - Do NOT call post_restore hook (that's Task 11)
  - Do NOT add CLI wiring

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Filesystem integrity — atomic rename, EXDEV fallback, transactional cleanup. Edge cases matter.
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Tasks 4, 6, 7)
  - **Parallel Group**: Wave 2
  - **Blocks**: Tasks 8, 9
  - **Blocked By**: Tasks 2, 3

  **References**:
  **Pattern References**:
  - `internal/snapshot/` (from Task 2) — Produces the `.tar.zst` + `.sha256` files that restore consumes
  - Eng review decision #7: "Error handling: Transactional cleanup. Failed operations remove partial state (temp tarballs, partial extractions). Restore uses extract-to-temp-dir + atomic rename (with EXDEV fallback for cross-mount renames)."

  **External References**:
  - Go `syscall.Statfs_t` — For disk space pre-flight check. `syscall.Statfs(path, &stat)` then `stat.Bavail * stat.Bsize`.
  - Go `os.Rename` — Atomic on same filesystem. Returns `EXDEV` error if cross-filesystem.

  **Why Each Reference Matters**:
  - EXDEV fallback is a real production concern: `~/.mesh/snapshots/` and `/opt/agents/` may be on different mounts on fleet VMs. The fallback must work correctly.
  - Transactional cleanup prevents partial restores that leave the target in an inconsistent state.

  **Acceptance Criteria**:
  - [ ] `go test ./internal/restore/ -run TestRestore -v -count=1` → PASS (filesystem identical after restore)
  - [ ] `go test ./internal/restore/ -run TestHashMismatch -v -count=1` → PASS (corrupted tarball rejected)
  - [ ] `go test ./internal/restore/ -run TestAtomicRename -v -count=1` → PASS
  - [ ] `go test ./internal/restore/ -run TestCleanupOnFailure -v -count=1` → PASS
  - [ ] `go test -race ./internal/restore/` → PASS

  **QA Scenarios:**

  ```
  Scenario: Restore produces identical filesystem
    Tool: Bash
    Preconditions: Task 2 complete (snapshot pipeline works)
    Steps:
      1. Run: go test ./internal/restore/ -run TestRestore -v -count=1
      2. Assert: exit code 0, restored files match original byte-for-byte
    Expected Result: After snapshot + restore, target dir is identical to source
    Failure Indicators: File content mismatch, permission changes, missing symlinks
    Evidence: .sisyphus/evidence/task-5-restore.txt

  Scenario: Corrupted tarball rejected
    Tool: Bash
    Preconditions: Task 2 complete
    Steps:
      1. Run: go test ./internal/restore/ -run TestHashMismatch -v -count=1
      2. Assert: exit code 0, error message mentions hash mismatch
    Expected Result: Restore of modified tarball fails with clear error
    Failure Indicators: Corrupted tarball accepted without error
    Evidence: .sisyphus/evidence/task-5-hash-mismatch.txt
  ```

  **Commit**: YES
  - Message: `feat(restore): add restore command with hash verify and atomic rename`
  - Files: internal/restore/*.go, internal/restore/*_test.go

- [x] 6. **Agent Process Management**

  **What to do**:
  - In `internal/agent/`, implement:
    - `FindPID(workdir string) (int, error)` — run `pgrep -f <workdir>`, parse PID. Error if 0 or 2+ matches.
    - `Stop(pid int, signal string, timeout time.Duration) error` — send signal, wait for exit with timeout. Return error if process doesn't exit in time.
    - `Start(cmd string, workdir string) error` — execute start_cmd in workdir via `os/exec.CommandContext`. Use `nohup` equivalent (detached process).
    - `IsRunning(workdir string) bool` — check if agent process exists via pgrep
    - Optional PID file support: if `pid_file` in config, read PID from file instead of pgrep
  - Tests (hermetic — use short-lived test processes, not real agents):
    - FindPID finds test process, errors on multiple matches, errors on no match
    - Stop sends signal and waits, errors on timeout
    - Start launches process, process is running after return
    - IsRunning returns correct state

  **Must NOT do**:
  - Do NOT integrate with snapshot/restore commands (that's Task 8)
  - Do NOT manage process lifecycle beyond start/stop (no monitoring, no restart)

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Thin wrapper over `os/exec` + `pgrep`. Standard Go patterns.
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Tasks 4, 5, 7)
  - **Parallel Group**: Wave 2
  - **Blocks**: Task 8
  - **Blocked By**: Task 3 (config struct defines agent fields)

  **References**:
  **Pattern References**:
  - Eng review decision #6: "Process management: `pgrep -f` matching workdir to find agent PID. SIGTERM + configurable timeout (default 30s). Optional `pid_file` in config."
  - Design doc: "Send SIGTERM to agent process, Wait for process exit (with configurable timeout, default 30s)"
  - Config struct `Agent.StopSignal`, `Agent.StopTimeout`, `Agent.PIDFile` — fields to use

  **External References**:
  - Go `os/exec` — `exec.Command("pgrep", "-f", workdir)` for PID lookup. `exec.CommandContext(ctx, "kill", "-s", signal, strconv.Itoa(pid))` for stopping.
  - Go `syscall` — `syscall.Kill(pid, syscall.SIGTERM)` for direct signal sending.

  **Why Each Reference Matters**:
  - pgrep pattern matching: The core of agent identification. Must handle 0 matches (not running) and 2+ matches (ambiguous) as errors.
  - Configurable timeout: Agent config specifies stop_timeout. Must parse as `time.Duration`.

  **Acceptance Criteria**:
  - [ ] `go test ./internal/agent/ -v -count=1` → PASS (all process management tests)
  - [ ] `go test -race ./internal/agent/` → PASS

  **QA Scenarios:**

  ```
  Scenario: Find and stop test process
    Tool: Bash
    Preconditions: Task 3 complete
    Steps:
      1. Run: go test ./internal/agent/ -run TestFindPID -v -count=1
      2. Assert: exit code 0, test process found by workdir
    Expected Result: pgrep-based PID lookup works for test process
    Failure Indicators: PID not found, wrong PID found
    Evidence: .sisyphus/evidence/task-6-agent.txt
  ```

  **Commit**: YES
  - Message: `feat(agent): add process management via pgrep`
  - Files: internal/agent/*.go, internal/agent/*_test.go

- [x] 7. **Manifest Package**

  **What to do**:
  - In `internal/manifest/`, implement:
    - `Manifest` struct: `AgentName string`, `Timestamp time.Time`, `SourceMachine string`, `SourceWorkdir string`, `StartCmd string`, `StopTimeout string`, `Checksum string`, `Size int64`
    - `Write(path string, m *Manifest) error` — write JSON manifest file
    - `Read(path string) (*Manifest, error)` — read JSON manifest file
    - `ManifestPath(snapshotPath string) string` — derive manifest path from snapshot path (replace `.tar.zst` with `.json`)
  - Tests: write + read round-trip, all fields preserved, malformed JSON returns error

  **Must NOT do**:
  - Do NOT add manifest versioning or migration
  - Do NOT embed manifest in the tarball (it's a sidecar file)

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Simple JSON serialization with Go stdlib `encoding/json`. 30-minute task.
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Tasks 4, 5, 6)
  - **Parallel Group**: Wave 2
  - **Blocks**: Task 10
  - **Blocked By**: Task 2 (snapshot pipeline creates the files manifest describes)

  **References**:
  **Pattern References**:
  - Eng review decision #4: "Manifest per snapshot: JSON manifest alongside each tarball containing agent name, timestamp, source machine, source workdir, start_cmd, stop_timeout, checksum. Snapshots are self-describing."
  - Snapshot format: `.tar.zst` + `.sha256` + `.json` sidecar

  **Acceptance Criteria**:
  - [ ] `go test ./internal/manifest/ -v -count=1` → PASS
  - [ ] `go test -race ./internal/manifest/` → PASS

  **QA Scenarios:**

  ```
  Scenario: Manifest write/read round-trip
    Tool: Bash
    Preconditions: Task 2 complete
    Steps:
      1. Run: go test ./internal/manifest/ -run TestRoundTrip -v -count=1
      2. Assert: exit code 0, all fields preserved after write→read
    Expected Result: Manifest survives serialization with zero data loss
    Failure Indicators: Fields missing or wrong after round-trip
    Evidence: .sisyphus/evidence/task-7-manifest.txt
  ```

  **Commit**: YES
  - Message: `feat(manifest): add JSON manifest per snapshot`
  - Files: internal/manifest/*.go, internal/manifest/*_test.go

- [x] 8. **Clone Command**

  **What to do**:
  - In `internal/clone/`, implement:
    - `Run(ctx context.Context, cfg *config.Config, agentName string, targetMachineName string) error`
    - Orchestrates: snapshot agent on source machine → SCP transfer to target → pre-flight on target → restore on target → start agent on target
    - For v0, source is always local (agent's workdir is on the control machine)
    - SCP transfer: `os/exec.Command("scp", snapshotPath, fmt.Sprintf("%s@%s:%s", user, host, targetDir))` — reads SSH config from machine config
    - Pre-flight on target via SSH: `os/exec.Command("ssh", host, "df", targetDir)` for disk space, `os.exec.Command("ssh", host, "test", "-w", targetDir)` for writable
    - Restore on target via SSH: extract tarball to target `agent_dir/agent_name/` on remote machine
    - Start agent on target via SSH: execute `start_cmd` via `os/exec.Command("ssh", host, startCmd)`
    - Print new agent endpoint (machine name + status message)
    - Config is NOT auto-updated; user adds `[[agents]]` entry manually (per design doc)
  - For local clone (target = same machine): skip SCP, just restore to a different directory
  - Tests: local clone round-trip (snapshot + restore to different dir), config not modified, non-existent target machine errors, SCP failure simulation (use invalid host)
  - Integration test tag: `//go:build integration` for tests requiring SSH, local-only tests run without tag

  **Must NOT do**:
  - Do NOT auto-update config file with new agent entry
  - Do NOT implement direct agent-to-agent transfer (v0 always goes through control machine)
  - Do NOT add progress reporting for SCP transfers

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Multi-step orchestration with failure modes at each step. Transactional cleanup across remote operations.
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: NO (depends on snapshot, restore, and agent packages being complete)
  - **Parallel Group**: Wave 3
  - **Blocks**: Task 9
  - **Blocked By**: Tasks 4, 5, 6

  **References**:
  **Pattern References**:
  - `internal/snapshot/` (Task 4) — `Run()` function to call for snapshot step
  - `internal/restore/` (Task 5) — `Restore()` function to call for restore step
  - `internal/agent/` (Task 6) — `Start()`, `Stop()`, `IsRunning()` for process management
  - `internal/config/` (Task 3) — Machine and Agent config structs
  - `internal/transport/` — SSH transport via `os/exec`

  **API/Type References**:
  - Design doc "Clone Workflow" section: "mesh clone hermes --target fleet-vm-2 does: 1. Snapshot hermes, 2. Transfer tarball + sha256, 3. Pre-flight check on target, 4. Extract tarball, 5. Execute start_cmd, 6. Print new agent endpoint"
  - CEO plan "Network Tradeoff": "Clone transfers data twice: agent-to-control, then control-to-target. v0 accepts this simplicity tradeoff."

  **Why Each Reference Matters**:
  - Clone is the integration point — it calls snapshot, restore, and agent packages. Understanding their APIs is critical.
  - The "double transfer" tradeoff means source is always local to control machine, target is always remote. This simplifies the clone flow significantly.

  **Acceptance Criteria**:
  - [ ] `go test ./internal/clone/ -run TestLocalCloneRoundTrip -v -count=1` → PASS
  - [ ] `go test ./internal/clone/ -run TestNonExistentTarget -v -count=1` → PASS
  - [ ] `go test -race ./internal/clone/` → PASS
  - [ ] Local clone: snapshot → restore to different dir → filesystem identical

  **QA Scenarios:**

  ```
  Scenario: Local clone round-trip
    Tool: Bash
    Preconditions: Tasks 4, 5, 6 complete
    Steps:
      1. Create temp source dir with test files
      2. Run: go test ./internal/clone/ -run TestLocalCloneRoundTrip -v -count=1
      3. Assert: exit code 0, target dir identical to source dir
    Expected Result: Local clone produces byte-identical copy in different directory
    Failure Indicators: Files missing, content mismatch, source dir modified
    Evidence: .sisyphus/evidence/task-8-local-clone.txt

  Scenario: Non-existent target machine error
    Tool: Bash
    Preconditions: Tasks 4, 5, 6 complete
    Steps:
      1. Run: go test ./internal/clone/ -run TestNonExistentTarget -v -count=1
      2. Assert: exit code 0 (test passes by checking error), descriptive error message
    Expected Result: Clone to non-existent machine name fails with clear error
    Failure Indicators: Panic, unclear error, attempt to connect to invalid host
    Evidence: .sisyphus/evidence/task-8-bad-target.txt
  ```

  **Commit**: YES
  - Message: `feat(clone): add clone command orchestrating snapshot+transport+restore`
  - Files: internal/clone/*.go, internal/clone/*_test.go

- [x] 9. **CLI Wire-Up — Cobra Subcommands**

  **What to do**:
  - Add `go get github.com/spf13/cobra` dependency
  - In `cmd/mesh/main.go`, implement the CLI structure:
    - Root command: `mesh` with `--config` flag, `--verbose`/`--quiet` flags, `--version` flag
    - Subcommands: `snapshot`, `restore`, `clone`, `status`, `list`, `inspect`, `prune`
    - Each subcommand parses args, loads config, calls the corresponding internal package
    - `mesh snapshot <agent>` — calls `snapshot.Run()`
    - `mesh restore <agent> [--snapshot <path>]` — calls `restore.Restore()` with latest or specified snapshot
    - `mesh clone <agent> --target <machine>` — calls `clone.Run()`
    - Version derived from `runtime/debug.ReadBuildInfo()` or ldflags
    - Logging: `--verbose` enables debug output to stderr, `--quiet` suppresses progress output, default shows normal progress
  - Each subcommand validates args before calling internal package
  - Exit codes: 0 = success, 1 = general error, 2 = usage error

  **Must NOT do**:
  - Do NOT implement status, list, inspect, prune logic (that's Task 10)
  - Do NOT add JSON output mode
  - Do NOT add color output
  - Do NOT add shell completion (post-v0)

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Wiring 7 subcommands with proper arg validation, config loading, and error handling. Not deep logic but lots of boilerplate.
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Tasks 10, 11)
  - **Parallel Group**: Wave 3
  - **Blocks**: Tasks 10, 12, 13
  - **Blocked By**: Tasks 4, 5, 8

  **References**:
  **External References**:
  - `github.com/spf13/cobra` — Go CLI framework. API: `cobra.Command{Use, Short, Run}`, `rootCmd.AddCommand()`, `cmd.Flags().String()`.
  - Design doc "Full Command Surface": exact command names, args, and flags for each subcommand

  **Pattern References**:
  - Design doc: all 7 command definitions
  - CEO plan: accepted additions (mesh status, mesh inspect, pre/post hooks)

  **Why Each Reference Matters**:
  - Cobra is the CLI framework. Must understand its Command struct, flag parsing, and error handling patterns.
  - Command surface defines the exact UX. Each subcommand's args and flags must match the design doc exactly.

  **Acceptance Criteria**:
  - [ ] `go build ./cmd/mesh/` exits 0
  - [ ] `./mesh --help` shows usage with all 7 subcommands
  - [ ] `./mesh snapshot --help` shows snapshot usage
  - [ ] `./mesh restore --help` shows restore usage
  - [ ] `./mesh clone --help` shows clone usage with --target flag
  - [ ] `./mesh --version` prints version
  - [ ] `./mesh snapshot nonexistent` exits 1 with "agent not found" error
  - [ ] `go test -race ./cmd/mesh/` → PASS

  **QA Scenarios:**

  ```
  Scenario: CLI builds and responds to help
    Tool: Bash
    Preconditions: Tasks 4, 5, 8 complete
    Steps:
      1. Run: go build -o /tmp/mesh ./cmd/mesh/
      2. Assert: exit code 0
      3. Run: /tmp/mesh --help
      4. Assert: output contains "snapshot", "restore", "clone", "status", "list", "inspect", "prune"
      5. Run: /tmp/mesh --version
      6. Assert: output contains version string
    Expected Result: Binary compiles, help shows all 7 subcommands, version prints
    Failure Indicators: Build fails, missing subcommands, version missing
    Evidence: .sisyphus/evidence/task-9-cli-help.txt

  Scenario: Error handling for bad invocations
    Tool: Bash
    Preconditions: Tasks 4, 5, 8 complete
    Steps:
      1. Run: /tmp/mesh snapshot nonexistent
      2. Assert: exit code 1, stderr contains "agent not found" or similar
      3. Run: /tmp/mesh clone hermes
      4. Assert: exit code 2, stderr contains "required flag" or "target" (missing --target)
    Expected Result: Bad args produce clear errors with non-zero exit codes
    Failure Indicators: Panic, exit code 0, unclear error message
    Evidence: .sisyphus/evidence/task-9-cli-errors.txt
  ```

  **Commit**: YES
  - Message: `feat(cli): wire all commands with Cobra subcommands`
  - Files: cmd/mesh/main.go, go.mod (updated with cobra dep)

- [x] 10. **Operational Commands — status, list, inspect, prune**

  **What to do**:
  - `mesh status <agent>` — print: running/stopped (via `agent.IsRunning()`), last snapshot timestamp, snapshot count, cache size (total bytes in `~/.mesh/snapshots/{agent}/`)
  - `mesh list [agent]` — list all snapshots across all agents, optionally filtered to one agent. Output: timestamp, size, source machine per snapshot. Read manifests from snapshot cache.
  - `mesh inspect <snapshot>` — print manifest contents: timestamp, source machine, start_cmd, size, checksum. Accept snapshot path or derive from agent name + latest.
  - `mesh prune <agent> --keep N` — remove oldest snapshots, keeping only N. Delete .tar.zst + .sha256 + .json for each pruned snapshot.
  - Each command reads config, resolves agent name, reads snapshot cache directory
  - Plain text output to stdout, no tables, no JSON mode

  **Must NOT do**:
  - Do NOT add JSON output mode
  - Do NOT add color
  - Do NOT add filtering flags beyond `--keep` for prune

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Four commands, each with file I/O, manifest parsing, and output formatting. Straightforward but lots of it.
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Task 11)
  - **Parallel Group**: Wave 3
  - **Blocks**: Tasks 12, 13
  - **Blocked By**: Tasks 4, 7, 9

  **References**:
  **Pattern References**:
  - `internal/manifest/` (Task 7) — Read manifest for inspect, list
  - `internal/agent/` (Task 6) — `IsRunning()` for status command
  - `internal/snapshot/` (Task 4) — Snapshot cache directory structure, prune logic already in snapshot
  - CEO plan accepted scope: "mesh status <agent>", "mesh inspect <snapshot>", "mesh prune <agent> --keep N", "mesh list [agent]"

  **Acceptance Criteria**:
  - [ ] `mesh status <agent>` prints running state, last snapshot, count, cache size
  - [ ] `mesh list` prints all snapshots across agents
  - [ ] `mesh list <agent>` prints snapshots for one agent
  - [ ] `mesh inspect <snapshot>` prints manifest contents
  - [ ] `mesh prune <agent> --keep 3` removes oldest snapshots, keeps 3
  - [ ] `go test -race ./...` → PASS

  **QA Scenarios:**

  ```
  Scenario: Status command output
    Tool: Bash
    Preconditions: Tasks 4, 7, 9 complete, at least one snapshot exists
    Steps:
      1. Create a test snapshot via the snapshot command
      2. Run: /tmp/mesh status test-agent
      3. Assert: output contains "stopped" or "running", snapshot count, cache size
    Expected Result: Status shows correct agent state and snapshot info
    Failure Indicators: Wrong state, missing snapshot count, empty output
    Evidence: .sisyphus/evidence/task-10-status.txt

  Scenario: Prune removes old snapshots
    Tool: Bash
    Preconditions: Tasks 4, 7, 9 complete, 5+ snapshots exist
    Steps:
      1. Create 5 test snapshots
      2. Run: /tmp/mesh prune test-agent --keep 3
      3. Assert: only 3 snapshots remain in cache directory
      4. Assert: removed snapshot files (.tar.zst, .sha256, .json) are deleted
    Expected Result: Only 3 most recent snapshots remain
    Failure Indicators: Wrong count, orphaned sidecar files
    Evidence: .sisyphus/evidence/task-10-prune.txt
  ```

  **Commit**: YES
  - Message: `feat(cli): add status, list, inspect, prune commands`
  - Files: cmd/mesh/*.go, internal/snapshot/prune.go (if extracted)

- [x] 11. **Pre/Post Hooks**

  **What to do**:
  - In `internal/config/`, add `PreSnapshotCmd` and `PostRestoreCmd` fields to Agent struct (already defined in Task 3 schema)
  - In `internal/snapshot/`, before starting the tar pipeline, run `pre_snapshot_cmd` if configured:
    - `exec.CommandContext(ctx, "sh", "-c", agent.PreSnapshotCmd).Run()`
    - Hook runs in the agent's workdir
    - Hook inherits the parent timeout (30s default)
    - Hook failure → abort snapshot with clear error
  - In `internal/restore/`, after successful extraction + rename, run `post_restore_cmd` if configured:
    - `exec.CommandContext(ctx, "sh", "-c", agent.PostRestoreCmd).Run()`
    - Hook runs in the restored workdir
    - Hook failure → return error (restore already completed, but error reported)
  - Tests: hook runs and succeeds, hook failure aborts operation, hook timeout enforced, no hook configured = skip

  **Must NOT do**:
  - Do NOT add template variables to hooks (CEO plan: "No template variables in v0")
  - Do NOT sandbox hooks (CEO plan: "Hooks run as the SSH user with no sandboxing")
  - Do NOT add hook timeout configuration (inherits parent timeout)

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Two `exec.CommandContext` calls in the right places. Simple integration.
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Tasks 9, 10)
  - **Parallel Group**: Wave 3
  - **Blocks**: Final verification
  - **Blocked By**: Tasks 4, 5, 3

  **References**:
  **Pattern References**:
  - CEO plan accepted scope: "Pre/post hooks: `pre_snapshot_cmd`, `post_restore_cmd` in agent config"
  - CEO plan hook contract: "Hook execution: run pre_snapshot_cmd on source machine before tar, run post_restore_cmd on target machine after extract"
  - CEO plan failure model: "Hook failure aborts the operation with a clear error. Hooks inherit the same timeout as the parent operation (30s default). No template variables in v0."

  **Acceptance Criteria**:
  - [ ] `go test ./internal/snapshot/ -run TestPreSnapshotHook -v -count=1` → PASS
  - [ ] `go test ./internal/restore/ -run TestPostRestoreHook -v -count=1` → PASS
  - [ ] `go test ./internal/snapshot/ -run TestHookFailure -v -count=1` → PASS (aborts snapshot)
  - [ ] `go test ./internal/snapshot/ -run TestHookTimeout -v -count=1` → PASS
  - [ ] `go test -race ./...` → PASS

  **QA Scenarios:**

  ```
  Scenario: Pre-snapshot hook runs and affects behavior
    Tool: Bash
    Preconditions: Tasks 4, 5, 3 complete
    Steps:
      1. Run: go test ./internal/snapshot/ -run TestPreSnapshotHook -v -count=1
      2. Assert: exit code 0, hook ran before tar was created
    Expected Result: Hook executes in workdir before snapshot starts
    Failure Indicators: Hook not executed, snapshot proceeds despite hook failure
    Evidence: .sisyphus/evidence/task-11-hooks.txt

  Scenario: Hook failure aborts snapshot
    Tool: Bash
    Preconditions: Tasks 4, 5, 3 complete
    Steps:
      1. Run: go test ./internal/snapshot/ -run TestHookFailure -v -count=1
      2. Assert: exit code 0, snapshot aborted with descriptive error
    Expected Result: Failed hook prevents snapshot from proceeding
    Failure Indicators: Snapshot completes despite hook returning error
    Evidence: .sisyphus/evidence/task-11-hook-failure.txt
  ```

  **Commit**: YES
  - Message: `feat(hooks): add pre/post snapshot/restore hooks`
  - Files: internal/snapshot/snapshot.go, internal/restore/restore.go

- [x] 12. **GitHub Actions CI Pipeline**

  **What to do**:
  - Update `.github/workflows/ci.yml` (created in Task 1 as placeholder) with actual Go CI:
    - Trigger: push to main, pull_request to main
    - Jobs: lint, test, build
    - lint: `go fmt ./...` (check), `go vet ./...`, `golangci-lint run`
    - test: `go test -race -coverprofile=coverage.out ./...`, upload coverage artifact
    - build: `go build -o mesh ./cmd/mesh/`, upload binary artifact
    - Go version: 1.25.x (use `setup-go` action)
    - Cache Go modules
    - Run on `ubuntu-latest` (linux/amd64 target)
  - Add goreleaser config for release builds: `goreleaser.yml` — builds linux/amd64 binary on tag
  - Verify CI passes by pushing to a branch

  **Must NOT do**:
  - Do NOT add release automation (that's ship workflow, post-v0)
  - Do NOT add cross-compilation matrix (darwin, windows, arm64) — linux/amd64 only
  - Do NOT set up Homebrew tap

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Standard GitHub Actions YAML. Boilerplate.
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Task 13)
  - **Parallel Group**: Wave 4
  - **Blocks**: Final verification
  - **Blocked By**: Task 9 (need CLI to exist for build step)

  **References**:
  **Pattern References**:
  - `.github/workflows/ci.yml` (from Task 1) — Placeholder CI file to update with real Go pipeline
  - Eng review decision: "CI pipeline: GitHub Actions with go fmt, go vet, go test -race, golangci-lint, goreleaser for linux/amd64 binary"

  **Acceptance Criteria**:
  - [ ] `.github/workflows/ci.yml` has lint, test, build jobs
  - [ ] `goreleaser.yml` exists and configures linux/amd64 build
  - [ ] CI workflow triggers on push to main and pull_request

  **QA Scenarios:**

  ```
  Scenario: CI config is valid YAML with expected jobs
    Tool: Bash
    Preconditions: Task 9 complete
    Steps:
      1. Run: cat .github/workflows/ci.yml
      2. Assert: contains "go test -race", "golangci-lint", "go build"
      3. Assert: trigger includes "push" to "main" and "pull_request"
    Expected Result: CI config has all three jobs with correct Go commands
    Failure Indicators: Missing jobs, wrong commands, no triggers
    Evidence: .sisyphus/evidence/task-12-ci.txt
  ```

  **Commit**: YES
  - Message: `ci: add GitHub Actions CI pipeline with lint, test, build`
  - Files: .github/workflows/ci.yml, goreleaser.yml

- [x] 13. **README + Install Instructions**

  **What to do**:
  - Create `README.md` with:
    - What Mesh is (one paragraph: portable agent-body runtime, v0 = filesystem snapshot/restore/clone)
    - Quick start: install, configure, snapshot, restore, clone
    - Config reference (TOML schema)
    - Command reference (7 commands with examples)
    - Architecture overview (library-first, vertical slices, v0 scope)
    - v0 limitations and what's coming (daemon, MCP, containers in v1)
    - Build from source instructions
  - Tone: technical, concise, no marketing language. The user is a developer building for themselves.

  **Must NOT do**:
  - Do NOT create a website or landing page
  - Do NOT add badges (CI status, coverage) — add after first green CI run
  - Do NOT add CONTRIBUTING.md changes (existing one is fine)

  **Recommended Agent Profile**:
  - **Category**: `writing`
    - Reason: Documentation task. Needs clear technical writing.
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Task 12)
  - **Parallel Group**: Wave 4
  - **Blocks**: Final verification
  - **Blocked By**: Tasks 9, 10 (need to know all commands to document)

  **References**:
  **Pattern References**:
  - Design doc: "Distribution Plan: Single binary (Go). GitHub Releases. linux/amd64 only. `curl | bash` installer."
  - Design doc: "Full Command Surface" section — all 7 commands
  - Design doc: "Config Schema (Stage 1)" — TOML layout

  **Acceptance Criteria**:
  - [ ] `README.md` exists and is >500 words
  - [ ] All 7 commands documented with examples
  - [ ] Config schema documented
  - [ ] Install instructions present

  **QA Scenarios:**

  ```
  Scenario: README covers all commands
    Tool: Bash
    Preconditions: Tasks 9, 10 complete
    Steps:
      1. Run: grep -c "snapshot\|restore\|clone\|status\|list\|inspect\|prune" README.md
      2. Assert: count >= 7 (each command mentioned at least once)
    Expected Result: README documents all 7 CLI commands
    Failure Indicators: Missing commands
    Evidence: .sisyphus/evidence/task-13-readme.txt
  ```

  **Commit**: YES
  - Message: `docs: add README and install instructions`
  - Files: README.md

---

## Final Verification Wave

> 4 review agents run in PARALLEL. ALL must APPROVE. Present consolidated results to user and get explicit "okay" before completing.

- [x] F1. **Plan Compliance Audit** — `oracle`
  Read the plan end-to-end. For each "Must Have": verify implementation exists (read file, run command). For each "Must NOT Have": search codebase for forbidden patterns — reject with file:line if found. Check evidence files exist in `.sisyphus/evidence/`. Compare deliverables against plan.
  Output: `Must Have [N/N] | Must NOT Have [N/N] | Tasks [N/N] | VERDICT: APPROVE/REJECT`

- [x] F2. **Code Quality Review** — `unspecified-high`
  Run `go vet ./...` + `golangci-lint run` + `go test -race ./...`. Review all .go files for: `as any`/type assertions without ok check, empty catches, `fmt.Println` in production packages (only allowed in cmd/), commented-out code, unused imports. Check AI slop: excessive comments, over-abstraction, generic names (data/result/item/temp). Verify no forbidden imports (docker, containerd, gRPC, protobuf).
  Output: `Build [PASS/FAIL] | Lint [PASS/FAIL] | Tests [N pass/N fail] | Files [N clean/N issues] | VERDICT`

- [x] F3. **Real Manual QA** — `unspecified-high`
  Start from clean state (`go build ./cmd/mesh/`). Execute EVERY QA scenario from EVERY task — follow exact steps, capture evidence. Test cross-command integration (snapshot → list → inspect → restore → status). Test edge cases: empty workdir, non-existent agent, invalid config. Save to `.sisyphus/evidence/final-qa/`.
  Output: `Scenarios [N/N pass] | Integration [N/N] | Edge Cases [N tested] | VERDICT`

- [x] F4. **Scope Fidelity Check** — `deep`
  For each task: read "What to do", read actual diff (`git log --oneline`). Verify 1:1 — everything in spec was built (no missing), nothing beyond spec was built (no creep). Check "Must NOT do" compliance. Detect cross-task contamination. Flag unaccounted changes.
  Output: `Tasks [N/N compliant] | Contamination [CLEAN/N issues] | Unaccounted [CLEAN/N files] | VERDICT`

---

## Commit Strategy

- **Task 1**: `feat(init): scaffold Go project with module, dirs, and CI` — go.mod, go.sum, internal/*/, .github/workflows/ci.yml
- **Task 2**: `feat(snapshot): add hash round-trip test and streaming pipeline` — internal/snapshot/*.go, internal/snapshot/*_test.go
- **Task 3**: `feat(config): add TOML config parsing and validation` — internal/config/*.go, internal/config/*_test.go
- **Task 4**: `feat(snapshot): add snapshot command with tar+zstd+hash pipeline` — internal/snapshot/snapshot.go, cmd/
- **Task 5**: `feat(restore): add restore command with hash verify and atomic rename` — internal/restore/*.go
- **Task 6**: `feat(agent): add process management via pgrep` — internal/agent/*.go
- **Task 7**: `feat(manifest): add JSON manifest per snapshot` — internal/manifest/*.go
- **Task 8**: `feat(clone): add clone command orchestrating snapshot+transport+restore` — internal/clone/*.go
- **Task 9**: `feat(cli): wire all commands with Cobra subcommands` — cmd/mesh/main.go
- **Task 10**: `feat(cli): add status, list, inspect, prune commands` — cmd/mesh/*.go
- **Task 11**: `feat(hooks): add pre/post snapshot/restore hooks` — internal/config/, internal/snapshot/, internal/restore/
- **Task 12**: `ci: add GitHub Actions CI pipeline` — .github/workflows/ci.yml
- **Task 13**: `docs: add README and install instructions` — README.md

---

## Success Criteria

### Verification Commands
```bash
go build ./cmd/mesh/                    # Expected: compiles without error
go test -race ./...                      # Expected: all tests pass, 0 races
golangci-lint run                        # Expected: 0 issues
go run ./cmd/mesh snapshot --help        # Expected: usage text, exit 0
go run ./cmd/mesh restore --help         # Expected: usage text, exit 0
go run ./cmd/mesh clone --help           # Expected: usage text, exit 0
```

### Final Checklist
- [ ] All "Must Have" present
- [ ] All "Must NOT Have" absent (no Docker imports, no CGo, no SSH libraries)
- [ ] All tests pass with `-race` flag
- [ ] Hash round-trip test proves byte-perfect filesystem fidelity
- [ ] All 7 CLI commands work
- [ ] CI pipeline green on push
