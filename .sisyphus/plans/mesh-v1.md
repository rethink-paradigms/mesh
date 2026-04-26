# Mesh v1 — Core Loop (Daemon + MCP + Docker + SQLite)

## TL;DR

> **Quick Summary**: Transform Mesh from a CLI snapshot tool into a long-running daemon with MCP server, Docker container runtime, and SQLite-backed body orchestration. Ship the core loop for A5 persona (Developer Agent, local Docker only). No fleet, no sandbox, no networking, no full plugin system.
>
> **Deliverables**:
> - Go daemon (`mesh serve`) with signal handling, graceful shutdown
> - MCP server (stdio transport, 8 P0 tools + 3 P1 tools)
> - Docker adapter implementing SubstrateAdapter interface
> - SQLite store (WAL mode) with body registry, snapshot metadata, state machine
> - Body orchestration with 8-state lifecycle, migration coordinator
> - Refactored persistence (Docker export/import wrapping v0 tar+zstd pipeline)
> - CLI: `mesh init`, `mesh serve`, `mesh stop`, `mesh status`
> - Integration test suite with testcontainers-go
>
> **Estimated Effort**: XL
> **Parallel Execution**: YES — 5 waves
> **Critical Path**: Task 1 (scaffold) → Task 3 (SQLite) → Task 5 (orchestration) → Task 8 (daemon) → Task 9 (MCP server) → Task 10 (CLI) → Final

---

## Context

### Original Request
Build the next version of Mesh after v0 CLI is complete. v1 = "ship the core loop" — daemon, MCP server, Docker adapter, SQLite state, body orchestration. Single binary, no networking, no full plugins. Target A5 persona (Developer Agent, local Docker only).

### Interview Summary
**Key Discussions**:
- v0 is shipping-complete (7 CLI commands, Go binary, all tests passing)
- v1-architecture.md draft (April 25) has 6 resolved architecture decisions (AD1-AD6)
- AD1: Single binary, direct Go calls (gRPC only at plugin boundary)
- AD2: Thin plugin veneer — Go interface for SubstrateAdapter, no go-plugin in v1
- AD3: SQLite with WAL for durable state
- AD4: Orchestration owns migration (not Interface)
- AD5: Extended substrate adapter: 6 required + 4 optional verbs
- AD6: Networking deferred to v1.1
- Q1-Q4 resolved with Metis defaults (idle bodies stay running, local FS storage, trivial scheduler, curl|bash bootstrap)
- v1 persona target: A5-only (local Docker). No fleet, no sandbox, no Nomad.
- v0→v1 migration: Clean break. v1 starts fresh. Document migration path.
- TDD strategy: Go standard testing + testcontainers-go for integration

**Research Findings**:
- Deep design docs exist for all 6 modules (interface.md, orchestration.md, persistence.md, provisioning.md, networking.md, plugin-infrastructure.md)
- v0 code reuse: internal/snapshot/ (HIGH), internal/restore/ (HIGH), internal/manifest/ (MEDIUM)
- v0 packages to NOT reuse: internal/clone/, internal/agent/, internal/transport/ — replaced by new architecture
- MCP protocol version: 2025-03-26 (stable). stdio transport. No auth (C4).
- Daemon lifecycle: ~500-1000 lines of boilerplate before Mesh logic

### Metis Review
**Identified Gaps** (all addressed):
- Q1-Q4 blocking v1 → Resolved with defaults: idle=running, registry=local FS, scheduler=trivial, bootstrap=curl|bash
- Persona scope → Pinned to A5-only (local Docker)
- MCP tool catalog undefined → Defined P0 (8 tools) + P1 (3 tools)
- v0→v1 migration → Clean break, documented
- Daemon complexity underestimated → Accounted for daemon infrastructure tasks
- SQLite concurrency → Per-body mutex + WAL mode
- Integration test strategy → testcontainers-go with build tags
- MCP protocol version → Pinned to 2025-03-26, stdio transport

---

## Work Objectives

### Core Objective
Transform Mesh from a CLI snapshot tool into a long-running daemon that manages agent bodies as Docker containers, exposes operations via MCP tools, and persists state in SQLite. Prove the core loop works end-to-end for a developer running agents on their laptop.

### Concrete Deliverables
- `cmd/mesh/` — CLI with `init`, `serve`, `stop`, `status` subcommands + Cobra root
- `internal/daemon/` — Long-running process with signal handling, PID file, graceful shutdown
- `internal/mcp/` — MCP server (stdio transport), tool registration, request routing
- `internal/docker/` — Docker adapter implementing SubstrateAdapter interface (6 required verbs)
- `internal/store/` — SQLite wrapper with WAL mode, body CRUD, snapshot metadata, migrations
- `internal/body/` — Body state machine (8 states), lifecycle operations, migration coordinator
- `internal/config/` — YAML config parsing (replaces v0 TOML)
- `internal/adapter/` — SubstrateAdapter Go interface definition
- `internal/snapshot/` — Refactored to accept Docker export streams (reuse tar+zstd pipeline)
- `internal/manifest/` — Extended manifest with container metadata (CMD, ENV, WORKDIR, platform)
- `go.mod`, `go.sum` — Updated dependencies

### Definition of Done
- [ ] `go build ./cmd/mesh/` produces working binary
- [ ] `go test -race ./...` passes with 0 failures
- [ ] `golangci-lint run` passes with 0 issues
- [ ] `go test -tags=integration ./...` passes (requires Docker)
- [ ] `mesh serve` starts daemon, responds to SIGTERM with graceful shutdown
- [ ] MCP `tools/list` returns catalog with 8 P0 tools
- [ ] `mesh_body_create` + `mesh_body_start` + `mesh_body_snapshot` + `mesh_body_restore` + `mesh_body_stop` + `mesh_body_destroy` works end-to-end
- [ ] Body state machine enforces valid transitions (Created→Starting→Running→Stopping→Stopped→Destroyed)
- [ ] SQLite survives daemon restart (bodies, snapshots, migrations persisted)

### Must Have
- Single Go binary, no CGo, no system dependencies beyond Docker socket
- MCP stdio transport per protocol version 2025-03-26
- 8 P0 MCP tools: body_create, body_start, body_stop, body_destroy, body_status, body_list, body_snapshot, body_restore
- Docker adapter with Create, Start, Stop, Destroy, GetStatus, Exec (6 required verbs)
- ExportFilesystem and ImportFilesystem via Docker (optional verbs, but implemented)
- SQLite WAL mode with per-body mutex for concurrent access
- Body state machine: Created, Starting, Running, Stopping, Stopped, Error, Migrating, Destroyed
- Graceful daemon shutdown (SIGTERM → stop all bodies → close MCP → exit)
- All v0 snapshot pipeline reused without modification (tar+zstd+SHA-256)
- Config in YAML format (not TOML)

### Must NOT Have (Guardrails)
- **NO Nomad, fleet scheduling, or multi-machine orchestration** — local Docker only
- **NO Tailscale, networking, DNS, or cross-body communication** — deferred to v1.1
- **NO go-plugin, gRPC subprocess, or plugin registry** — Docker is compile-time dependency
- **NO E2B, Fly, Daytona, or any sandbox provider** — Docker only
- **NO S3, R2, or remote storage backends** — local filesystem only for snapshots
- **NO auto-migration of v0 config or snapshots** — clean break, documented
- **NO telemetry, login, phone-home** — constraint C4
- **NO modification of v0 internal/snapshot/ pipeline** — wrap it, don't rewrite it
- **NO multi-machine clone** — replaced by single-machine body lifecycle
- **NO SSH, scp, or remote transport** — local Docker socket only
- **NO A1-A4 persona support** — A5 (Developer Agent, local) only
- **NO JSON output mode, color output, or table formatting** — plain text CLI

---

## Verification Strategy

> **ZERO HUMAN INTERVENTION** — ALL verification is agent-executed. No exceptions.
> Acceptance criteria requiring "user manually tests/confirms" are FORBIDDEN.

### Test Decision
- **Infrastructure exists**: YES (go test, 4 packages tested, go test -race working)
- **Automated tests**: YES (TDD — tests written alongside or before implementation)
- **Framework**: Go standard testing (`go test`) + `testcontainers-go` for Docker integration
- **If TDD**: Each internal package gets tests written alongside or before implementation
- **Integration tag**: `//go:build integration` for tests requiring Docker daemon

### QA Policy
Every task MUST include agent-executed QA scenarios (see TODO template below).
Evidence saved to `.sisyphus/evidence/task-{N}-{scenario-slug}.{ext}`.

- **Go library/module**: Use Bash (`go test -v -race ./...`) — run tests, assert pass/fail
- **CLI binary**: Use Bash (`go build && ./mesh --help`) — build, run commands, check output
- **MCP server**: Use Bash (`echo '{"jsonrpc":"2.0",...}' | ./mesh serve`) — send JSON-RPC, assert response
- **Docker integration**: Use Bash (`go test -tags=integration`) — testcontainers-go
- **SQLite**: Use Bash (`go test ./internal/store/`) — CRUD assertions

---

## Execution Strategy

### Parallel Execution Waves

```
Wave 1 (Start Immediately — foundation):
├── Task 1: Go module scaffold + new package structure [quick]
├── Task 2: SubstrateAdapter interface + types [quick]
├── Task 3: SQLite store — schema, CRUD, WAL mode [deep]
└── Task 4: YAML config parsing [quick]

Wave 2 (After Wave 1 — MAX PARALLEL):
├── Task 5: Body state machine + lifecycle (depends: 2, 3) [deep]
├── Task 6: Docker adapter — required verbs (depends: 2) [deep]
├── Task 7: Refactored persistence — Docker export/import wrapper (depends: 2, 6) [deep]
├── Task 8: Daemon infrastructure (depends: 3, 4) [deep]

Wave 3 (After Wave 2 — integration):
├── Task 9: MCP server — tool registration, request routing (depends: 5, 6, 7, 8) [deep]
├── Task 10: CLI wire-up — init, serve, stop, status (depends: 8) [unspecified-high]
├── Task 11: Manifest v2 — container metadata (depends: 6, 7) [quick]

Wave 4 (After Wave 3 — hardening):
├── Task 12: MCP P1 tools — migrate, exec, logs, prune (depends: 9) [unspecified-high]
├── Task 13: Integration tests — end-to-end body lifecycle (depends: 9, 10) [deep]
├── Task 14: Graceful shutdown + health checks (depends: 8, 9) [deep]
├── Task 15: README + docs + v0→v1 migration guide (depends: 10) [writing]

Wave FINAL (After ALL tasks — 4 parallel reviews):
├── Task F1: Plan compliance audit (oracle)
├── Task F2: Code quality review (unspecified-high)
├── Task F3: Real manual QA (unspecified-high)
└── Task F4: Scope fidelity check (deep)
-> Present results -> Get explicit user okay

Critical Path: T1 → T3 → T5 → T8 → T9 → T10 → T13 → F1-F4
Parallel Speedup: ~60% faster than sequential
Max Concurrent: 4 (Waves 2 & 4)
```

### Dependency Matrix

| Task | Depends On | Blocks | Wave |
|------|-----------|--------|------|
| 1    | —         | 2, 3, 4 | 1    |
| 2    | 1         | 5, 6, 7 | 1    |
| 3    | 1         | 5, 8    | 1    |
| 4    | 1         | 8       | 1    |
| 5    | 2, 3      | 9       | 2    |
| 6    | 2         | 7, 9, 11 | 2   |
| 7    | 2, 6      | 9, 11   | 2    |
| 8    | 3, 4      | 9, 10, 14 | 2  |
| 9    | 5, 6, 7, 8 | 12, 13 | 3  |
| 10   | 8         | 13, 15  | 3    |
| 11   | 6, 7      | 13      | 3    |
| 12   | 9         | 13      | 4    |
| 13   | 9, 10, 11, 12 | F1-F4 | 4 |
| 14   | 8, 9      | F1-F4   | 4    |
| 15   | 10        | F1-F4   | 4    |

### Agent Dispatch Summary

- **Wave 1**: **4** — T1 → `quick`, T2 → `quick`, T3 → `deep`, T4 → `quick`
- **Wave 2**: **4** — T5 → `deep`, T6 → `deep`, T7 → `deep`, T8 → `deep`
- **Wave 3**: **3** — T9 → `deep`, T10 → `unspecified-high`, T11 → `quick`
- **Wave 4**: **4** — T12 → `unspecified-high`, T13 → `deep`, T14 → `deep`, T15 → `writing`
- **FINAL**: **4** — F1 → `oracle`, F2 → `unspecified-high`, F3 → `unspecified-high`, F4 → `deep`

---

## TODOs

- [ ] 1. **Go Module Scaffold + Package Structure + Old Code Cleanup**

  **What to do**:
  - Add new Go dependencies: `github.com/docker/docker/client`, `github.com/testcontainers/testcontainers-go`, `github.com/mattn/go-sqlite3` (or modernc.org/sqlite), `gopkg.in/yaml.v3`, MCP SDK (e.g., `github.com/mark3labs/mcp-go` or `github.com/metoro-io/mcp-golang`)
  - Create new directory structure: `internal/daemon/`, `internal/mcp/`, `internal/docker/`, `internal/store/`, `internal/body/`, `internal/adapter/`, `internal/config/` (rename old to `internal/config-toml/` or delete)
  - Delete obsolete packages: `internal/clone/`, `internal/agent/`, `internal/transport/` (clone/transport/agent replaced by daemon + Docker)
  - Keep and DO NOT MODIFY: `internal/snapshot/`, `internal/restore/`, `internal/manifest/`
  - Add CONTEXT.md to each new package with one-line responsibility description
  - Wire `cmd/mesh/main.go` to import new `internal/config/` and `internal/daemon/`
  - Verify: `go build ./cmd/mesh/` succeeds, `go test ./...` succeeds (no new tests yet)

  **Must NOT do**:
  - Do NOT modify any files in `internal/snapshot/`, `internal/restore/`, or `internal/manifest/`
  - Do NOT modify files in `discovery/`
  - Do NOT add any business logic — pure scaffolding

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Pure scaffolding — directory creation, go get, file deletion, CONTEXT.md boilerplate.
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: NO (foundation task)
  - **Parallel Group**: Wave 1
  - **Blocks**: Tasks 2, 3, 4
  - **Blocked By**: None

  **References**:
  **Pattern References**:
  - `internal/snapshot/CONTEXT.md` — Template for new package CONTEXT.md files
  - `cmd/mesh/main.go` — Current Cobra CLI structure to extend
  - `go.mod` — Current dependencies (cobra, toml, klauspost/compress)
  **External References**:
  - Docker Go SDK: `github.com/docker/docker/client` — Official Docker Engine API client for Go
  - testcontainers-go: `github.com/testcontainers/testcontainers-go` — Programmatic Docker container management for integration tests
  - MCP Go SDK: `github.com/mark3labs/mcp-go` — MCP server implementation for Go

  **Acceptance Criteria**:
  - [ ] `go build ./cmd/mesh/` exits 0
  - [ ] `go test ./...` exits 0 (no tests, compilation check)
  - [ ] `internal/clone/`, `internal/agent/`, `internal/transport/` directories deleted
  - [ ] New directories exist: `internal/daemon/`, `internal/mcp/`, `internal/docker/`, `internal/store/`, `internal/body/`, `internal/adapter/`
  - [ ] `internal/snapshot/`, `internal/restore/`, `internal/manifest/` untouched

  **QA Scenarios**:
  ```
  Scenario: Build succeeds after scaffold
    Tool: Bash
    Preconditions: go.mod exists
    Steps:
      1. Run: go build ./cmd/mesh/
      2. Assert: exit code 0
      3. Run: go test ./...
      4. Assert: exit code 0
    Expected Result: Binary compiles, all packages load
    Failure Indicators: Build fails, import errors, missing packages
    Evidence: .sisyphus/evidence/task-1-build.txt

  Scenario: Old packages removed, new packages exist
    Tool: Bash
    Preconditions: Task 1 complete
    Steps:
      1. Run: test -d internal/clone && echo "FAIL: clone exists" || echo "OK: clone removed"
      2. Run: test -d internal/agent && echo "FAIL: agent exists" || echo "OK: agent removed"
      3. Run: test -d internal/daemon && echo "OK: daemon exists" || echo "FAIL: daemon missing"
      4. Run: test -d internal/mcp && echo "OK: mcp exists" || echo "FAIL: mcp missing"
      5. Run: test -d internal/docker && echo "OK: docker exists" || echo "FAIL: docker missing"
      6. Run: test -d internal/store && echo "OK: store exists" || echo "FAIL: store missing"
      7. Run: test -d internal/body && echo "OK: body exists" || echo "FAIL: body missing"
      8. Run: test -d internal/adapter && echo "OK: adapter exists" || echo "FAIL: adapter missing"
    Expected Result: Old packages gone, new packages present, snapshot/restore/manifest untouched
    Failure Indicators: Wrong directories present/absent
    Evidence: .sisyphus/evidence/task-1-structure.txt
  ```

  **Commit**: YES
  - Message: `feat(init): scaffold v1 packages, remove obsolete v0 code`
  - Files: go.mod, go.sum, cmd/mesh/main.go, internal/*/CONTEXT.md, deleted internal/{clone,agent,transport}/

- [ ] 2. **SubstrateAdapter Interface + Types**

  **What to do**:
  - In `internal/adapter/`, define the Go interface per AD5:
    - `SubstrateAdapter` interface with 6 required verbs: `Create`, `Start`, `Stop`, `Destroy`, `GetStatus`, `Exec`
    - 4 optional verbs (check via `Capabilities()`): `ExportFilesystem`, `ImportFilesystem`, `Inspect`
    - Supporting types: `Handle` (body instance ID), `BodySpec` (image + resource config), `BodyStatus` (state + uptime + resource usage), `StopOpts` (signal + timeout), `ExecResult` (stdout + stderr + exit code), `AdapterCapabilities`, `ImportOpts`, `ContainerMetadata`
  - Define the body state enum (8 states): `Created`, `Starting`, `Running`, `Stopping`, `Stopped`, `Error`, `Migrating`, `Destroyed`
  - Define `BodySpec` struct: `Image string`, `Workdir string`, `Env map[string]string`, `Cmd []string`, `MemoryMB int`, `CPUShares int`
  - Tests: interface compilation check, type marshaling round-trip (JSON), state enum string representations

  **Must NOT do**:
  - Do NOT implement any adapter — this is interface definition only
  - Do NOT add Docker-specific types — stay abstract

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Interface + type definitions. ~100 lines of code. Tests verify compilation + marshaling.
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Tasks 3, 4)
  - **Parallel Group**: Wave 1
  - **Blocks**: Tasks 5, 6, 7
  - **Blocked By**: Task 1

  **References**:
  **Pattern References**:
  - `discovery/design/deep/provisioning.md` — Substrate adapter contract definition (6 verbs, capabilities)
  - `discovery/design/deep/orchestration.md` — Body state machine (8 states, transitions)
  - v1-architecture.md AD5 — Extended adapter contract (optional verbs)

  **Acceptance Criteria**:
  - [ ] `internal/adapter/adapter.go` compiles
  - [ ] `go test ./internal/adapter/ -v` → PASS (type tests)
  - [ ] `go test -race ./internal/adapter/` → PASS

  **QA Scenarios**:
  ```
  Scenario: Interface compiles and types marshal correctly
    Tool: Bash
    Preconditions: Task 1 complete
    Steps:
      1. Run: go test ./internal/adapter/ -v -count=1
      2. Assert: exit code 0, all tests PASS
    Expected Result: SubstrateAdapter interface compiles, BodyStatus JSON marshals correctly
    Failure Indicators: Compilation errors, JSON field mismatch
    Evidence: .sisyphus/evidence/task-2-adapter-types.txt
  ```

  **Commit**: YES
  - Message: `feat(adapter): define SubstrateAdapter interface and body types`
  - Files: internal/adapter/adapter.go, internal/adapter/types.go, internal/adapter/adapter_test.go

- [ ] 3. **SQLite Store — Schema, CRUD, WAL Mode**

  **What to do**:
  - Add SQLite dependency (choose `modernc.org/sqlite` — pure Go, no CGo, or `github.com/mattn/go-sqlite3` if CGo acceptable)
  - In `internal/store/`, implement:
    - `Open(path string) (*Store, error)` — open SQLite DB with WAL mode, create schema if not exists
    - Schema (4 tables): `bodies` (id TEXT PK, name TEXT UNIQUE, state TEXT, spec_json TEXT, substrate TEXT, instance_id TEXT, created_at TEXT, updated_at TEXT), `snapshots` (id TEXT PK, body_id TEXT FK, manifest_json TEXT, storage_path TEXT, size_bytes INTEGER, created_at TEXT), `migrations` (id TEXT PK, body_id TEXT FK, target_substrate TEXT, current_step INTEGER, snapshot_id TEXT, started_at TEXT, error TEXT), `config` (key TEXT PK, value TEXT)
    - Add indexes: `idx_snapshots_body_id`, `idx_migrations_body_id`
    - Foreign key constraints enabled via `PRAGMA foreign_keys = ON`
    - Per-body mutex: `sync.Mutex` map keyed by body ID
    - CRUD operations: `CreateBody`, `GetBody`, `ListBodies`, `UpdateBodyState`, `DeleteBody`, `CreateSnapshot`, `ListSnapshots`, `GetSnapshot`, `DeleteSnapshot`, `CreateMigration`, `UpdateMigration`, `GetMigration`, `GetConfig`, `SetConfig`
  - Tests: create/get/list/delete body, state transition (Created→Running→Stopped), snapshot CRUD, concurrent access with mutex, WAL mode verified via PRAGMA

  **Must NOT do**:
  - Do NOT add body lifecycle logic — that's Task 5
  - Do NOT add migration execution logic — that's Task 5
  - Do NOT add Docker or MCP imports — pure SQLite

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Schema design, CRUD operations, WAL mode, per-body mutex concurrency. Data integrity foundation.
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Tasks 2, 4)
  - **Parallel Group**: Wave 1
  - **Blocks**: Tasks 5, 8
  - **Blocked By**: Task 1

  **References**:
  **External References**:
  - `modernc.org/sqlite` — Pure Go SQLite (no CGo). API: `sql.Open("sqlite", path)`, `db.Exec("PRAGMA journal_mode=WAL")`
  - Go `database/sql` — Standard DB API. `db.QueryRow`, `db.Exec`, prepared statements.
  - v1-architecture.md AD3 — SQLite with WAL, schema sketch (bodies, snapshots, migrations, config)

  **Acceptance Criteria**:
  - [ ] `go test ./internal/store/ -v -count=1` → PASS (CRUD + WAL + concurrency)
  - [ ] `go test -race ./internal/store/` → PASS (0 races)
  - [ ] Schema has 4 tables with foreign keys enforced
  - [ ] WAL mode confirmed via `PRAGMA journal_mode` returning "wal"

  **QA Scenarios**:
  ```
  Scenario: Body CRUD round-trip
    Tool: Bash
    Preconditions: Task 1 complete
    Steps:
      1. Run: go test ./internal/store/ -run TestBodyCRUD -v -count=1
      2. Assert: exit code 0, body created, read back with all fields preserved
    Expected Result: Create→Read→Update→Delete cycle works
    Failure Indicators: Data loss, wrong state, SQL errors
    Evidence: .sisyphus/evidence/task-3-store-crud.txt

  Scenario: WAL mode enabled
    Tool: Bash
    Preconditions: Task 1 complete
    Steps:
      1. Run: go test ./internal/store/ -run TestWALMode -v -count=1
      2. Assert: exit code 0, PRAGMA journal_mode returns "wal"
    Expected Result: SQLite database opens in WAL mode
    Failure Indicators: Journal mode is "delete" or "memory"
    Evidence: .sisyphus/evidence/task-3-store-wal.txt
  ```

  **Commit**: YES
  - Message: `feat(store): add SQLite store with WAL mode and body CRUD`
  - Files: internal/store/store.go, internal/store/schema.go, internal/store/store_test.go

- [ ] 4. **YAML Config Parsing**

  **What to do**:
  - In `internal/config/` (new, replacing old TOML config), implement:
    - `type Config struct` with fields: `Daemon DaemonConfig`, `Store StoreConfig`, `Docker DockerConfig`, `Bodies []BodyConfig`
    - `DaemonConfig`: `SocketPath string`, `PIDFile string`, `LogLevel string`
    - `StoreConfig`: `Path string` (default `~/.mesh/state.db`)
    - `DockerConfig`: `Host string` (default `unix:///var/run/docker.sock`), `APIVersion string`
    - `BodyConfig`: `Name string`, `Image string`, `Workdir string`, `Env map[string]string`, `Cmd []string`, `MemoryMB int`, `CPUShares int`
    - `Load(path string) (*Config, error)` — parse YAML file
    - `DefaultPath() string` — returns `~/.mesh/config.yaml`
    - Override via `--config` flag or `MESH_CONFIG` env var
  - Tests: valid config loads, missing required fields, invalid YAML, default values applied

  **Must NOT do**:
  - Do NOT read or modify v0 TOML config — clean break
  - Do NOT add CLI flags for config (that's Task 10)

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: YAML parsing + validation. Standard Go pattern with `gopkg.in/yaml.v3`.
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Tasks 2, 3)
  - **Parallel Group**: Wave 1
  - **Blocks**: Task 8
  - **Blocked By**: Task 1

  **References**:
  **External References**:
  - `gopkg.in/yaml.v3` — Go YAML library. API: `yaml.Unmarshal(data, &cfg)`, `yaml.Marshal(&cfg)`
  - v1-architecture.md AD1 — Config format changes (TOML → YAML)
  - `internal/config/` (v0) — Old TOML config for reference on pattern (NOT to reuse)

  **Acceptance Criteria**:
  - [ ] `go test ./internal/config/ -v -count=1` → PASS
  - [ ] `go test -race ./internal/config/` → PASS
  - [ ] Valid YAML parses without error
  - [ ] Missing required field returns descriptive error
  - [ ] Default values applied for optional fields

  **QA Scenarios**:
  ```
  Scenario: Valid config loads and validates
    Tool: Bash
    Preconditions: Task 1 complete
    Steps:
      1. Run: go test ./internal/config/ -run TestLoadValidConfig -v -count=1
      2. Assert: exit code 0, output contains "PASS"
    Expected Result: YAML config parses into struct with defaults applied
    Failure Indicators: Parse error, missing defaults, wrong types
    Evidence: .sisyphus/evidence/task-4-valid-config.txt

  Scenario: Invalid config rejected
    Tool: Bash
    Preconditions: Task 1 complete
    Steps:
      1. Run: go test ./internal/config/ -run TestValidationErrors -v -count=1
      2. Assert: exit code 0, each subtest checks specific validation error
    Expected Result: Missing fields, invalid paths, bad YAML all rejected with clear errors
    Failure Indicators: Invalid config accepted without error
    Evidence: .sisyphus/evidence/task-4-validation.txt
  ```

  **Commit**: YES
  - Message: `feat(config): add YAML config parsing (replaces v0 TOML)`
  - Files: internal/config/config.go, internal/config/config_test.go

- [ ] 5. **Body State Machine + Lifecycle Orchestration**

  **What to do**:
  - In `internal/body/`, implement the body lifecycle state machine per AD4:
    - 8 states with valid transitions: `Created→Starting→Running`, `Running→Stopping→Stopped`, `Running→Migrating→Running`, `*→Error`, `Stopped→Destroyed`, `Error→Destroyed`, `Error→Starting` (retry)
    - State transition validation: `func (b *Body) Transition(target State) error` — enforces valid transitions, returns error on invalid
    - `type Body struct` wrapping store record + adapter handle + mutex
    - `BodyManager` struct: creates bodies via store + adapter, manages lifecycle, owns per-body mutex
    - `Create(ctx, spec BodySpec) (*Body, error)` — insert into store (Created state), call adapter.Create, transition to Starting→Running
    - `Start(ctx, bodyID string) error` — call adapter.Start, transition Stopped→Starting→Running
    - `Stop(ctx, bodyID string, opts StopOpts) error` — call adapter.Stop with signal+timeout, transition Running→Stopping→Stopped
    - `Destroy(ctx, bodyID string) error` — call adapter.Destroy, delete from store, remove snapshots
    - `GetStatus(ctx, bodyID string) (BodyStatus, error)` — return store state + adapter.GetStatus
    - `List() ([]Body, error)` — `SELECT * FROM bodies`
    - Migration coordinator per AD4: `BeginMigration(bodyID, targetSubstrate) (string, error)` — creates migration record in store, 7-step sequence (snapshot → provision target → transfer → restore → verify → switch → cleanup), persists progress after each step, resumes from persisted step on crash
  - Tests: valid state transitions, invalid transitions rejected, Create+Start+Stop+Destroy lifecycle, migration record persistence, concurrent Start calls on same body (mutex serializes)

  **Must NOT do**:
  - Do NOT implement the MCP tools — that's Task 9
  - Do NOT call Docker directly — use adapter interface
  - Do NOT add networking or cross-machine logic

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: State machine validation, migration coordinator with durability, concurrency model. Core business logic.
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Tasks 6, 7, 8)
  - **Parallel Group**: Wave 2
  - **Blocks**: Task 9
  - **Blocked By**: Tasks 2, 3

  **References**:
  **Pattern References**:
  - `discovery/design/deep/orchestration.md` — Body state machine, migration ownership, compensating actions
  - `internal/store/` (Task 3) — Body CRUD operations to call
  - `internal/adapter/` (Task 2) — SubstrateAdapter interface, BodyStatus, StopOpts types
  - v1-architecture.md AD4 — Orchestration owns migration (not Interface)

  **Acceptance Criteria**:
  - [ ] `go test ./internal/body/ -v -count=1` → PASS (all lifecycle tests)
  - [ ] `go test -race ./internal/body/` → PASS (0 races)
  - [ ] State transitions enforced: Running→Destroyed rejected
  - [ ] Migration persists progress after each step
  - [ ] Concurrent Stop/Start on same body serialized by mutex

  **QA Scenarios**:
  ```
  Scenario: Full body lifecycle
    Tool: Bash
    Preconditions: Tasks 2, 3 complete
    Steps:
      1. Run: go test ./internal/body/ -run TestLifecycle -v -count=1
      2. Assert: exit code 0, body goes Created→Starting→Running→Stopping→Stopped→Destroyed
    Expected Result: All valid transitions succeed, invalid transitions rejected
    Failure Indicators: Valid transition fails, invalid transition accepted
    Evidence: .sisyphus/evidence/task-5-lifecycle.txt

  Scenario: Migration record durable
    Tool: Bash
    Preconditions: Tasks 2, 3 complete
    Steps:
      1. Run: go test ./internal/body/ -run TestMigrationDurability -v -count=1
      2. Assert: exit code 0, migration record survives simulated crash restart
    Expected Result: Migration resumes from last persisted step
    Failure Indicators: Migration restarts from step 0, data lost
    Evidence: .sisyphus/evidence/task-5-migration.txt
  ```

  **Commit**: YES
  - Message: `feat(body): add state machine and lifecycle orchestration`
  - Files: internal/body/body.go, internal/body/manager.go, internal/body/migration.go, internal/body/*_test.go

- [ ] 6. **Docker Adapter — Required Verbs**

  **What to do**:
  - In `internal/docker/`, implement `SubstrateAdapter` interface (from Task 2) using Docker SDK:
    - `Create(ctx, spec BodySpec) (Handle, error)` — `dockerClient.ContainerCreate()` with image, env, cmd, workdir, memory limits. Returns container ID.
    - `Start(ctx, id string) error` — `dockerClient.ContainerStart()` 
    - `Stop(ctx, id string, opts StopOpts) error` — `dockerClient.ContainerStop()` with timeout from opts
    - `Destroy(ctx, id string) error` — `dockerClient.ContainerRemove()` with force
    - `GetStatus(ctx, id string) (BodyStatus, error)` — `dockerClient.ContainerInspect()` → parse State (running/exited), StartedAt, parse memory/cpu from HostConfig
    - `Exec(ctx, id string, cmd []string) (ExecResult, error)` — `dockerClient.ContainerExecCreate()` + `ContainerExecAttach()`, collect stdout/stderr, check exit code
    - `ExportFilesystem(ctx, id string) (io.ReadCloser, error)` — `dockerClient.ContainerExport()` returns tar stream
    - `ImportFilesystem(ctx, id string, tarball io.Reader, opts ImportOpts) error` — `dockerClient.CopyToContainer()` with tar stream
    - `Inspect(ctx, id string) (ContainerMetadata, error)` — `dockerClient.ContainerInspect()` → extract Image, Env, Cmd, Workdir
    - `Capabilities() AdapterCapabilities` — all optional verbs supported (Docker can do everything)
  - Connection: `client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())`
  - Tests with `testcontainers-go` (integration tag): create alpine container, start, exec `echo hello`, verify stdout, stop, destroy. Export/import round-trip.

  **Must NOT do**:
  - Do NOT implement Docker image pulling (use local images for tests, `alpine:latest`)
  - Do NOT add networking configuration (ports, networks)
  - Do NOT add volume mounts beyond the workdir concept

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Docker SDK integration with streaming I/O (export/import), container lifecycle, error mapping. Multiple verbs with distinct failure modes.
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Tasks 5, 7, 8)
  - **Parallel Group**: Wave 2
  - **Blocks**: Tasks 7, 9, 11
  - **Blocked By**: Task 2

  **References**:
  **External References**:
  - `github.com/docker/docker/client` — Official Docker Engine API Go client
  - `github.com/testcontainers/testcontainers-go` — Integration testing with real Docker
  **Pattern References**:
  - `internal/adapter/adapter.go` (Task 2) — SubstrateAdapter interface to implement
  - `discovery/design/deep/provisioning.md` — Substrate adapter contract, compliance matrix

  **Acceptance Criteria**:
  - [ ] `go test ./internal/docker/ -v -count=1` → PASS (unit tests, mocked Docker)
  - [ ] `go test -tags=integration ./internal/docker/ -v -count=1` → PASS (real Docker)
  - [ ] `go test -race ./internal/docker/` → PASS
  - [ ] Create+Start+Exec+Stop+Destroy lifecycle works with real Docker
  - [ ] Export+Import round-trip preserves filesystem

  **QA Scenarios**:
  ```
  Scenario: Docker container lifecycle
    Tool: Bash (requires Docker daemon)
    Preconditions: Task 2 complete, Docker running
    Steps:
      1. Run: go test -tags=integration ./internal/docker/ -run TestContainerLifecycle -v -count=1
      2. Assert: exit code 0, container created, started, exec'd, stopped, destroyed
    Expected Result: Full container lifecycle works via Docker SDK
    Failure Indicators: Container create fails, exec returns empty, stop hangs
    Evidence: .sisyphus/evidence/task-6-docker-lifecycle.txt

  Scenario: Export/Import round-trip
    Tool: Bash (requires Docker daemon)
    Preconditions: Task 2 complete, Docker running
    Steps:
      1. Run: go test -tags=integration ./internal/docker/ -run TestExportImportRoundTrip -v -count=1
      2. Assert: exit code 0, exported filesystem restores to new container identically
    Expected Result: Files survive docker export→import cycle
    Failure Indicators: Missing files, content mismatch
    Evidence: .sisyphus/evidence/task-6-docker-export-import.txt
  ```

  **Commit**: YES
  - Message: `feat(docker): implement SubstrateAdapter with Docker SDK`
  - Files: internal/docker/adapter.go, internal/docker/adapter_test.go, internal/docker/adapter_integration_test.go

- [ ] 7. **Refactored Persistence — Docker Export/Import Wrapper**

  **What to do**:
  - In `internal/snapshot/`, add a new function (DO NOT modify existing `CreateSnapshot`):
    - `CreateFromReader(ctx context.Context, reader io.Reader, outputPath string) (string, error)` — compress reader with zstd, hash with SHA-256, write to outputPath, return hex hash. Reuses `internal/snapshot/` pipeline (zstd writer + sha256 tee).
  - In `internal/restore/`, add a new function:
    - `RestoreToWriter(ctx context.Context, snapshotPath string, writer io.Writer) error` — verify hash, decompress, pipe tar to writer (for docker import).
  - In `internal/persistence/` (new package), implement:
    - `SnapshotEngine` struct: wraps Docker adapter Export + snapshot pipeline + local FS storage
    - `Capture(ctx, adapter adapter.SubstrateAdapter, bodyID, bodyName string) (*manifest.Manifest, error)` — calls `adapter.ExportFilesystem()`, pipes to `snapshot.CreateFromReader()`, writes manifest, stores in `~/.mesh/snapshots/{body_name}/`
    - `Restore(ctx, adapter adapter.SubstrateAdapter, snapshotPath string) error` — reads snapshot, calls `restore.RestoreToWriter()`, pipes to `adapter.ImportFilesystem()`
    - `List(ctx, bodyName string) ([]manifest.Manifest, error)` — read all manifest JSON files in snapshot dir
    - `Prune(ctx, bodyName string, keepN int) error` — remove oldest snapshots, keep N
  - Tests: Capture (mock adapter that exports known tar), Restore (mock adapter that collects import), List, Prune.

  **Must NOT do**:
  - Do NOT modify `CreateSnapshot` or `Restore` in v0 packages
  - Do NOT add storage backends (S3, R2) — local FS only
  - Do NOT call Docker directly — always through adapter interface

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Wraps Docker adapter + v0 snapshot pipeline + local FS storage. Integration of three subsystems.
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Tasks 5, 6, 8)
  - **Parallel Group**: Wave 2
  - **Blocks**: Tasks 9, 11
  - **Blocked By**: Tasks 2, 6

  **References**:
  **Pattern References**:
  - `internal/snapshot/snapshot.go` — v0 tar+zstd+SHA-256 pipeline (DO NOT MODIFY, add new functions alongside)
  - `internal/restore/restore.go` — v0 extraction pipeline (DO NOT MODIFY, add new functions alongside)
  - `internal/manifest/manifest.go` — JSON manifest (to extend in Task 11)
  - `internal/adapter/` (Task 2) — SubstrateAdapter with ExportFilesystem/ImportFilesystem

  **Acceptance Criteria**:
  - [ ] `go test ./internal/persistence/ -v -count=1` → PASS
  - [ ] `go test -race ./internal/persistence/` → PASS
  - [ ] Capture produces .tar.zst + .sha256 + .json in snapshot dir
  - [ ] Restore pipes snapshot through adapter.ImportFilesystem
  - [ ] v0 snapshot/restore packages untouched (`git diff internal/snapshot/` shows additions only)

  **QA Scenarios**:
  ```
  Scenario: Capture and restore via mock adapter
    Tool: Bash
    Preconditions: Tasks 2, 6 complete
    Steps:
      1. Run: go test ./internal/persistence/ -run TestCaptureRestore -v -count=1
      2. Assert: exit code 0, snapshot files created, restore called with correct tarball
    Expected Result: Full capture→restore cycle works through adapter interface
    Failure Indicators: Capture fails, restore receives wrong data
    Evidence: .sisyphus/evidence/task-7-persistence.txt
  ```

  **Commit**: YES
  - Message: `feat(persistence): add Docker-aware snapshot engine wrapping v0 pipeline`
  - Files: internal/snapshot/reader.go, internal/restore/writer.go, internal/persistence/*.go, internal/persistence/*_test.go

- [ ] 8. **Daemon Infrastructure**

  **What to do**:
  - In `internal/daemon/`, implement the long-running Mesh daemon per AD1:
    - `Daemon` struct: holds config, store, body manager, Docker adapter, MCP server (later), signal channel
    - `New(cfg *config.Config) (*Daemon, error)` — initialize store, Docker adapter, body manager, register signal handlers (SIGTERM, SIGINT)
    - `Start(ctx context.Context) error` — blocking call: open SQLite, connect Docker, reconcile stored bodies with running containers (startup reconciliation), start MCP server (Task 9 integration), wait for shutdown signal
    - `Stop(ctx context.Context) error` — graceful shutdown: stop MCP server, stop all running bodies (SIGTERM + timeout), close SQLite, exit
    - Signal handling: on SIGTERM/SIGINT, call `Stop()` with 30s timeout, force-kill if timeout exceeded
    - PID file: write PID to configured path on start, remove on stop
    - Startup reconciliation: on boot, list bodies from SQLite, check if Docker containers exist/running, update state to match reality (e.g., Running→Error if container gone)
    - Health check endpoint (HTTP on localhost): `GET /healthz` returns 200 with `{"status":"ok"}`
  - Tests: daemon starts/stops, signal handling, PID file life, startup reconciliation, health check

  **Must NOT do**:
  - Do NOT start MCP server yet — that's Task 9 (provide hook/stub)
  - Do NOT add systemd integration (service file) — post-v1
  - Do NOT add config reload (SIGHUP) — post-v1

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Process lifecycle, signal handling, graceful shutdown, startup reconciliation. Foundation for everything above it.
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Tasks 5, 6, 7)
  - **Parallel Group**: Wave 2
  - **Blocks**: Tasks 9, 10, 14
  - **Blocked By**: Tasks 3, 4

  **References**:
  **Pattern References**:
  - v1-architecture.md AD1 — Single binary architecture
  - v1-architecture.md "Mesh Lifecycle" section — mesh init → mesh serve → mesh stop
  **External References**:
  - Go `os/signal` — `signal.NotifyContext(ctx, syscall.SIGTERM, syscall.SIGINT)`
  - Go `net/http` — Health check endpoint

  **Acceptance Criteria**:
  - [ ] `go test ./internal/daemon/ -v -count=1` → PASS
  - [ ] `go test -race ./internal/daemon/` → PASS
  - [ ] Daemon starts, writes PID file, responds to health check
  - [ ] SIGTERM triggers graceful shutdown (stop bodies, close store, remove PID)
  - [ ] Startup reconciliation detects missing containers

  **QA Scenarios**:
  ```
  Scenario: Daemon starts and stops cleanly
    Tool: Bash
    Preconditions: Tasks 3, 4 complete
    Steps:
      1. Run: go test ./internal/daemon/ -run TestStartStop -v -count=1
      2. Assert: exit code 0, PID file created on start, removed on stop
    Expected Result: Daemon lifecycle works
    Failure Indicators: Start hangs, PID file not cleaned, health check fails
    Evidence: .sisyphus/evidence/task-8-daemon-lifecycle.txt

  Scenario: Graceful shutdown on SIGTERM
    Tool: Bash
    Preconditions: Tasks 3, 4 complete
    Steps:
      1. Run: go test ./internal/daemon/ -run TestGracefulShutdown -v -count=1
      2. Assert: exit code 0, bodies stopped before daemon exits
    Expected Result: Daemon stops bodies then exits within timeout
    Failure Indicators: Daemon kills without stopping bodies, timeout exceeded
    Evidence: .sisyphus/evidence/task-8-daemon-shutdown.txt
  ```

  **Commit**: YES
  - Message: `feat(daemon): add daemon infrastructure with signal handling and graceful shutdown`
  - Files: internal/daemon/daemon.go, internal/daemon/daemon_test.go

- [ ] 9. **MCP Server — Tool Registration + Request Routing**

  **What to do**:
  - In `internal/mcp/`, implement MCP server (stdio transport) per protocol version 2025-03-26:
    - `Server` struct: wraps daemon services (body manager, persistence, Docker adapter)
    - `NewServer(daemon *daemon.Daemon) *Server` — wire daemon services
    - `Listen(ctx context.Context) error` — start stdio transport, accept JSON-RPC requests
    - Tool registration: register 8 P0 tools with JSON Schema input/output types
    - **P0 tools**:
      - `mesh_body_create` — input: `{name, image, workdir?, env?, cmd?, memory_mb?, cpu_shares?}`, output: `{body_id, name, state: "Running"}`
      - `mesh_body_start` — input: `{body_id}`, output: `{body_id, state: "Running"}`
      - `mesh_body_stop` — input: `{body_id, signal?, timeout_sec?}`, output: `{body_id, state: "Stopped"}`
      - `mesh_body_destroy` — input: `{body_id, force?}`, output: `{body_id, state: "Destroyed"}`
      - `mesh_body_status` — input: `{body_id}`, output: `{body_id, state, uptime_sec, memory_mb, cpu_percent}`
      - `mesh_body_list` — input: `{}`, output: `{bodies: [{body_id, name, state, image}]}`
      - `mesh_body_snapshot` — input: `{body_id}`, output: `{snapshot_id, checksum, size_bytes, path}`
      - `mesh_body_restore` — input: `{body_id, snapshot_id?}`, output: `{body_id, state: "Stopped"}`
    - Request routing: parse JSON-RPC method, validate params against schema, call daemon service, marshal response
    - Error mapping: daemon errors → JSON-RPC error codes (-32603 internal, -32602 invalid params, -32000 body not found)
    - Add `tools/list` and `tools/call` as standard MCP methods
  - Tests: `tools/list` returns catalog, each tool call with valid params, invalid params rejected, error mapping

  **Must NOT do**:
  - Do NOT implement P1 tools (migrate, exec, logs, prune) — that's Task 12
  - Do NOT add HTTP/SSE transport — stdio only
  - Do NOT add authentication

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: MCP protocol implementation, JSON-RPC handling, tool schema validation, error mapping. Integration with all daemon services.
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: NO (depends on daemon + body manager + Docker + persistence being complete)
  - **Parallel Group**: Wave 3
  - **Blocks**: Tasks 12, 13
  - **Blocked By**: Tasks 5, 6, 7, 8

  **References**:
  **External References**:
  - MCP protocol spec version 2025-03-26 — JSON-RPC 2.0 over stdio, `tools/list` and `tools/call` methods
  - `github.com/mark3labs/mcp-go` (or equivalent Go MCP SDK) — Server setup, tool registration, stdio transport
  **Pattern References**:
  - `internal/daemon/` (Task 8) — Daemon struct exposing body manager, persistence, Docker adapter
  - `internal/body/` (Task 5) — BodyManager API (Create, Start, Stop, Destroy, GetStatus, List)
  - `internal/persistence/` (Task 7) — SnapshotEngine API (Capture, Restore, List, Prune)

  **Acceptance Criteria**:
  - [ ] `go test ./internal/mcp/ -v -count=1` → PASS
  - [ ] `go test -race ./internal/mcp/` → PASS
  - [ ] `tools/list` returns JSON array with 8 P0 tools + their schemas
  - [ ] `tools/call` with valid params routes to correct daemon method
  - [ ] Invalid params return JSON-RPC error -32602
  - [ ] Non-existent body returns JSON-RPC error -32000

  **QA Scenarios**:
  ```
  Scenario: MCP tools/list returns catalog
    Tool: Bash
    Preconditions: Tasks 5-8 complete
    Steps:
      1. Run: go test ./internal/mcp/ -run TestToolsList -v -count=1
      2. Assert: exit code 0, response contains 8 tool definitions with input/output schemas
    Expected Result: MCP server exposes full tool catalog
    Failure Indicators: Missing tools, invalid JSON schema
    Evidence: .sisyphus/evidence/task-9-tools-list.txt

  Scenario: Create body via MCP tool
    Tool: Bash
    Preconditions: Tasks 5-8 complete
    Steps:
      1. Run: go test ./internal/mcp/ -run TestBodyCreate -v -count=1
      2. Assert: exit code 0, response contains body_id and state "Running"
    Expected Result: MCP tool creates body via daemon, returns correct response
    Failure Indicators: Body not created, wrong state, error response
    Evidence: .sisyphus/evidence/task-9-body-create.txt
  ```

  **Commit**: YES
  - Message: `feat(mcp): add MCP server with 8 P0 tools`
  - Files: internal/mcp/server.go, internal/mcp/tools.go, internal/mcp/server_test.go

- [ ] 10. **CLI Wire-Up — init, serve, stop, status**

  **What to do**:
  - In `cmd/mesh/main.go`, replace v0 CLI with v1 Cobra subcommands:
    - `mesh init` — create `~/.mesh/` dir, write default `config.yaml`, print "Mesh initialized. Run `mesh serve` to start."
    - `mesh serve` — load config, create daemon, start MCP server, block until SIGTERM. Print "Mesh serving on stdio MCP" on start.
    - `mesh stop` — send SIGTERM to PID from PID file (or find process). Print "Mesh stopped."
    - `mesh status` — read PID file, check if process running. Print daemon status + body count from MCP `body_list` call.
    - Global flags: `--config`, `--verbose`, `--quiet`
    - `--version` prints build info
  - Each subcommand calls `internal/daemon/` or `internal/mcp/` packages
  - Exit codes: 0 success, 1 error

  **Must NOT do**:
  - Do NOT keep v0 CLI commands (snapshot, restore, clone) in v1 binary
  - Do NOT add JSON output mode
  - Do NOT add color output

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Cobra CLI wiring across 4 subcommands with daemon lifecycle integration. Mostly boilerplate.
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Tasks 9, 11)
  - **Parallel Group**: Wave 3
  - **Blocks**: Tasks 13, 15
  - **Blocked By**: Task 8

  **References**:
  **Pattern References**:
  - `cmd/mesh/main.go` — Current Cobra CLI (to be rewritten)
  - `internal/daemon/` (Task 8) — Daemon start/stop/status API
  - `internal/mcp/` (Task 9) — MCP server (for mesh status to call)

  **Acceptance Criteria**:
  - [ ] `go build ./cmd/mesh/` exits 0
  - [ ] `./mesh --help` shows init, serve, stop, status subcommands
  - [ ] `./mesh --version` prints version
  - [ ] `./mesh init` creates ~/.mesh/ with default config.yaml
  - [ ] `./mesh serve` starts daemon and blocks

  **QA Scenarios**:
  ```
  Scenario: Help and version output
    Tool: Bash
    Preconditions: Task 8 complete
    Steps:
      1. Run: go build -o /tmp/mesh ./cmd/mesh/
      2. Assert: exit code 0
      3. Run: /tmp/mesh --help
      4. Assert: output contains "init", "serve", "stop", "status"
      5. Run: /tmp/mesh --version
      6. Assert: output contains version string
    Expected Result: CLI shows correct subcommands and version
    Failure Indicators: Missing subcommands, version missing, build fails
    Evidence: .sisyphus/evidence/task-10-cli-help.txt

  Scenario: Init creates config
    Tool: Bash
    Preconditions: Task 8 complete
    Steps:
      1. Run: MESH_CONFIG=/tmp/test-mesh-config.yaml /tmp/mesh init
      2. Assert: exit code 0
      3. Run: test -f /tmp/test-mesh-config.yaml && echo "OK" || echo "FAIL"
    Expected Result: Default config file created at specified path
    Failure Indicators: Config not created, wrong path
    Evidence: .sisyphus/evidence/task-10-init.txt
  ```

  **Commit**: YES
  - Message: `feat(cli): add v1 CLI with init, serve, stop, status`
  - Files: cmd/mesh/main.go

- [ ] 11. **Manifest v2 — Container Metadata**

  **What to do**:
  - In `internal/manifest/`, extend the v0 manifest to include container metadata:
    - Add to `Manifest` struct: `Image string`, `Env map[string]string`, `Cmd []string`, `Workdir string`, `Platform string`, `BodyID string`
    - Keep existing fields: `AgentName`, `Timestamp`, `SourceMachine`, `SourceWorkdir`, `StartCmd`, `StopTimeout`, `Checksum`, `Size`
    - `Write(path string, m *Manifest) error` — JSON serialize (backward compatible with v0)
    - `Read(path string) (*Manifest, error)` — JSON deserialize (v0 manifests still readable, new fields default to zero values)
    - Tests: v2 manifest round-trip, v0 manifest compatibility (read old format), new fields preserved

  **Must NOT do**:
  - Do NOT break v0 manifest compatibility — old JSON files must still parse
  - Do NOT add manifest versioning or migration

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Extend existing manifest struct with Docker metadata fields. Simple.
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Tasks 9, 10)
  - **Parallel Group**: Wave 3
  - **Blocks**: Task 13
  - **Blocked By**: Tasks 6, 7

  **References**:
  **Pattern References**:
  - `internal/manifest/manifest.go` — v0 manifest struct (DO NOT remove fields, only add new ones)
  - `internal/docker/` (Task 6) — ContainerMetadata type from Inspect

  **Acceptance Criteria**:
  - [ ] `go test ./internal/manifest/ -v -count=1` → PASS
  - [ ] `go test -race ./internal/manifest/` → PASS
  - [ ] v2 manifest writes and reads with all Docker metadata fields
  - [ ] v0 manifest JSON (no Docker fields) still reads correctly

  **QA Scenarios**:
  ```
  Scenario: v2 manifest round-trip with Docker metadata
    Tool: Bash
    Preconditions: Task 6 complete
    Steps:
      1. Run: go test ./internal/manifest/ -run TestManifestV2RoundTrip -v -count=1
      2. Assert: exit code 0, Image, Env, Cmd, Workdir, Platform preserved
    Expected Result: Docker metadata survives JSON serialization
    Failure Indicators: New fields missing after read, old fields lost
    Evidence: .sisyphus/evidence/task-11-manifest-v2.txt

  Scenario: v0 manifest backward compatibility
    Tool: Bash
    Preconditions: Task 6 complete
    Steps:
      1. Run: go test ./internal/manifest/ -run TestV0Compatibility -v -count=1
      2. Assert: exit code 0, v0 manifest JSON parses without error
    Expected Result: Old manifests still readable, new fields are zero values
    Failure Indicators: Parse error on v0 JSON
    Evidence: .sisyphus/evidence/task-11-manifest-compat.txt
  ```

  **Commit**: YES
  - Message: `feat(manifest): extend manifest with Docker container metadata`
  - Files: internal/manifest/manifest.go, internal/manifest/manifest_test.go

- [ ] 12. **MCP P1 Tools — migrate, exec, logs, prune**

  **What to do**:
  - In `internal/mcp/`, add 3 P1 tools to the MCP server:
    - `mesh_body_migrate` — input: `{body_id, target_substrate}`, output: `{migration_id, state, current_step}`. Calls body.BeginMigration().
    - `mesh_body_exec` — input: `{body_id, command: [string]}`, output: `{exit_code, stdout, stderr}`. Calls adapter.Exec().
    - `mesh_body_logs` — input: `{body_id, tail?}`, output: `{logs: string}`. Calls `dockerClient.ContainerLogs()`.
    - `mesh_snapshot_prune` — input: `{body_id, keep: int}`, output: `{pruned: int, remaining: int}`. Calls persistence.Prune().
  - Update `tools/list` to include 4 P1 tools (total: 12 tools)
  - Tests: migrate tool returns migration ID, exec returns command output, logs returns container logs, prune removes old snapshots

  **Must NOT do**:
  - Do NOT implement migration execution logic — already in Task 5
  - Do NOT implement snapshot pruning logic — already in Task 7

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: 4 additional tools, each thin wrapper over existing daemon services. Mostly wiring + schema + tests.
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Tasks 13, 14, 15)
  - **Parallel Group**: Wave 4
  - **Blocks**: Final verification
  - **Blocked By**: Task 9

  **References**:
  **Pattern References**:
  - `internal/mcp/` (Task 9) — Existing MCP server, tool registration pattern
  - `internal/body/` (Task 5) — BeginMigration
  - `internal/docker/` (Task 6) — Exec, ContainerLogs
  - `internal/persistence/` (Task 7) — Prune

  **Acceptance Criteria**:
  - [ ] `go test ./internal/mcp/ -v -count=1` → PASS (all 12 tools)
  - [ ] `go test -race ./internal/mcp/` → PASS
  - [ ] `tools/list` returns 12 tools (8 P0 + 4 P1)
  - [ ] Exec returns stdout/stderr from container command
  - [ ] Prune reduces snapshot count

  **QA Scenarios**:
  ```
  Scenario: Exec returns command output
    Tool: Bash
    Preconditions: Tasks 6, 9 complete
    Steps:
      1. Run: go test ./internal/mcp/ -run TestBodyExec -v -count=1
      2. Assert: exit code 0, response contains exit_code 0 and stdout "hello"
    Expected Result: Exec tool runs command in container and returns output
    Failure Indicators: Empty stdout, wrong exit code, timeout
    Evidence: .sisyphus/evidence/task-12-exec.txt
  ```

  **Commit**: YES
  - Message: `feat(mcp): add P1 tools — migrate, exec, logs, prune`
  - Files: internal/mcp/tools.go, internal/mcp/server_test.go

- [ ] 13. **Integration Tests — End-to-End Body Lifecycle**

  **What to do**:
  - In `internal/daemon/`, add integration tests (build tag `integration`):
    - `TestEndToEndBodyLifecycle`: start daemon, create body via MCP, verify running, snapshot, stop, restore, start, destroy. Full circle.
    - `TestConcurrentOperations`: create 2 bodies simultaneously, verify both succeed.
    - `TestCrashRecovery`: create body, kill daemon (SIGKILL), restart, verify startup reconciliation detects body.
    - `TestMigration`: create body, trigger migration, verify migration record progresses through steps.
  - Use `testcontainers-go` for Docker. Use real SQLite (temp file) for store.
  - Tests run with `go test -tags=integration ./internal/daemon/`
  - Skip if Docker unavailable (`t.Skip("docker not available")`)

  **Must NOT do**:
  - Do NOT require Docker for unit tests — build tag `integration` gates all Docker tests
  - Do NOT test remote scenarios — local Docker only

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Multi-step integration tests with real Docker, concurrent operations, crash recovery. Complex orchestration.
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Tasks 12, 14, 15)
  - **Parallel Group**: Wave 4
  - **Blocks**: Final verification
  - **Blocked By**: Tasks 9, 10, 11, 12

  **References**:
  **Pattern References**:
  - `internal/daemon/` (Task 8) — Daemon API
  - `internal/mcp/` (Tasks 9, 12) — All 12 MCP tools
  - `internal/docker/` (Task 6) — Docker adapter integration tests (pattern)
  **External References**:
  - `github.com/testcontainers/testcontainers-go` — Docker-in-Docker for CI

  **Acceptance Criteria**:
  - [ ] `go test -tags=integration ./internal/daemon/ -v -count=1` → PASS
  - [ ] `TestEndToEndBodyLifecycle` passes (create→snapshot→stop→restore→start→destroy)
  - [ ] `TestCrashRecovery` passes (daemon detects orphaned bodies on restart)
  - [ ] `TestConcurrentOperations` passes (2 concurrent creates succeed)

  **QA Scenarios**:
  ```
  Scenario: Full body lifecycle via MCP tools
    Tool: Bash (requires Docker)
    Preconditions: Tasks 9-12 complete, Docker running
    Steps:
      1. Run: go test -tags=integration ./internal/daemon/ -run TestEndToEndBodyLifecycle -v -count=1
      2. Assert: exit code 0, body survives create→snapshot→stop→restore→start→destroy
    Expected Result: Complete body lifecycle works end-to-end
    Failure Indicators: Any step fails, state mismatch, snapshot corruption
    Evidence: .sisyphus/evidence/task-13-e2e-lifecycle.txt
  ```

  **Commit**: YES
  - Message: `test(integration): add end-to-end body lifecycle tests`
  - Files: internal/daemon/daemon_integration_test.go

- [ ] 14. **Graceful Shutdown + Health Checks**

  **What to do**:
  - In `internal/daemon/`, harden graceful shutdown:
    - Stop all bodies in parallel with individual timeouts
    - Track body stop status — report any failures
    - Force-kill (SIGKILL) bodies that don't stop within timeout
    - Close MCP server (stop accepting new requests, drain in-flight)
    - Close SQLite (WAL checkpoint before close)
    - Remove PID file
    - Log shutdown summary: bodies stopped, bodies force-killed, time elapsed
  - Health check endpoint:
    - `GET /healthz` → 200 `{"status":"ok","bodies":N,"uptime_sec":T}`
    - `GET /healthz?verbose=true` → 200 with per-body status
    - Return 503 if Docker socket unreachable
  - Tests: parallel body stop, force-kill on timeout, health check during normal operation, health check with Docker down

  **Must NOT do**:
  - Do NOT add metrics or Prometheus endpoint — post-v1
  - Do NOT add readiness probe (just liveness)

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Parallel shutdown with timeouts, force-kill fallback, health check with Docker dependency. Edge cases matter.
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Tasks 12, 13, 15)
  - **Parallel Group**: Wave 4
  - **Blocks**: Final verification
  - **Blocked By**: Tasks 8, 9

  **References**:
  **Pattern References**:
  - `internal/daemon/daemon.go` (Task 8) — Existing shutdown, health check stub
  - `internal/docker/` (Task 6) — Docker adapter (health check depends on Docker reachable)

  **Acceptance Criteria**:
  - [ ] `go test ./internal/daemon/ -v -count=1` → PASS
  - [ ] `go test -race ./internal/daemon/` → PASS
  - [ ] 10 bodies stop in parallel within 30s
  - [ ] Force-kill triggers on timeout exceeded
  - [ ] Health check returns 503 when Docker down

  **QA Scenarios**:
  ```
  Scenario: Parallel body shutdown
    Tool: Bash
    Preconditions: Tasks 8, 9 complete
    Steps:
      1. Run: go test ./internal/daemon/ -run TestParallelShutdown -v -count=1
      2. Assert: exit code 0, all bodies stopped within timeout
    Expected Result: Multiple bodies stop concurrently, none left running
    Failure Indicators: Shutdown timeout, body left running, panic
    Evidence: .sisyphus/evidence/task-14-shutdown.txt

  Scenario: Health check with Docker down
    Tool: Bash
    Preconditions: Tasks 8, 9 complete
    Steps:
      1. Run: go test ./internal/daemon/ -run TestHealthCheckDockerDown -v -count=1
      2. Assert: exit code 0, health check returns 503
    Expected Result: Daemon reports unhealthy when Docker unreachable
    Failure Indicators: Health check returns 200 when Docker is down
    Evidence: .sisyphus/evidence/task-14-health-check.txt
  ```

  **Commit**: YES
  - Message: `feat(daemon): add parallel graceful shutdown and health checks`
  - Files: internal/daemon/shutdown.go, internal/daemon/health.go, internal/daemon/daemon_test.go

- [ ] 15. **README + v0→v1 Migration Guide**

  **What to do**:
  - Update `README.md` with v1 content:
    - What Mesh is (one paragraph: portable agent-body runtime, v1 = daemon + MCP + Docker)
    - Quick start: install, `mesh init`, `mesh serve`, MCP tool examples
    - Config reference (YAML schema)
    - CLI reference (4 commands: init, serve, stop, status)
    - MCP tool reference (12 tools with input/output examples)
    - Architecture overview (daemon, MCP, Docker adapter, SQLite, body state machine)
    - v0→v1 migration guide: v0 snapshots not compatible, export data from v0 containers, import to v1 Docker bodies
    - v1.1 roadmap (what's coming: networking, fleet, Nomad, plugins)
    - Build from source instructions
  - Tone: technical, concise, no marketing language

  **Must NOT do**:
  - Do NOT remove v0 documentation — preserve as historical reference section
  - Do NOT add badges or website content

  **Recommended Agent Profile**:
  - **Category**: `writing`
    - Reason: Documentation task with migration guide. Clear technical writing.
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Tasks 12, 13, 14)
  - **Parallel Group**: Wave 4
  - **Blocks**: Final verification
  - **Blocked By**: Task 10

  **References**:
  **Pattern References**:
  - `README.md` — Current v0 README (preserve history, add v1 sections)
  - v1-architecture.md — v1 scope, deferred items
  - v1-architecture.md AD2 — Plugin system deferred
  - v1-architecture.md AD6 — Networking deferred
  - Metis defaults — Q1-Q4 resolution for docs

  **Acceptance Criteria**:
  - [ ] `README.md` updated with v1 install, config, CLI, MCP tool reference
  - [ ] v0→v1 migration section present with clear steps
  - [ ] v1.1 roadmap section present
  - [ ] All 12 MCP tools documented with examples
  - [ ] Config YAML schema documented

  **QA Scenarios**:
  ```
  Scenario: README covers all v1 content
    Tool: Bash
    Preconditions: Task 10 complete
    Steps:
      1. Run: grep -c "mesh serve\|mesh init\|mesh stop\|mesh status\|body_create\|body_start\|body_snapshot" README.md
      2. Assert: count >= 7
      3. Run: grep -c "v0→v1\|migration\|upgrad" README.md
      4. Assert: count >= 2
    Expected Result: README covers v1 CLI, MCP tools, and migration path
    Failure Indicators: Missing commands, no migration guide
    Evidence: .sisyphus/evidence/task-15-readme.txt
  ```

  **Commit**: YES
  - Message: `docs: update README for v1 with migration guide and tool reference`
  - Files: README.md

---

## Final Verification Wave (MANDATORY — after ALL implementation tasks)

> 4 review agents run in PARALLEL. ALL must APPROVE. Present consolidated results to user and get explicit "okay" before completing.
>
> **Do NOT auto-proceed after verification. Wait for user's explicit approval before marking work complete.**
> **Never mark F1-F4 as checked before getting user's okay.** Rejection or user feedback -> fix -> re-run -> present again -> wait for okay.

- [ ] F1. **Plan Compliance Audit** — `oracle`
  Read the plan end-to-end. For each "Must Have": verify implementation exists (read file, go test, run command). For each "Must NOT Have": search codebase for forbidden patterns — reject with file:line if found (Nomad, Tailscale, gRPC, go-plugin, S3, scp, Docker Go SSH, CGo, telemetry). Check evidence files exist in `.sisyphus/evidence/`. Compare deliverables against plan.
  Output: `Must Have [N/N] | Must NOT Have [N/N] | Tasks [N/N] | VERDICT: APPROVE/REJECT`

- [ ] F2. **Code Quality Review** — `unspecified-high`
  Run `go vet ./...` + `golangci-lint run` + `go test -race ./...`. Review all .go files for: `as any`/type assertions without ok check, empty catches, `fmt.Println` in production packages, commented-out code, unused imports. Check AI slop: excessive comments, over-abstraction, generic names (data/result/item/temp). Verify no forbidden imports. Verify v0 snapshot/restore packages unchanged.
  Output: `Build [PASS/FAIL] | Lint [PASS/FAIL] | Tests [N pass/N fail] | Files [N clean/N issues] | VERDICT`

- [ ] F3. **Real Manual QA** — `unspecified-high`
  Start from clean state. Execute EVERY QA scenario from EVERY task — follow exact steps, capture evidence. Test cross-module integration: daemon start → MCP body_create → body_start → body_snapshot → body_stop → body_restore → body_start → body_destroy → daemon stop. Test edge cases: empty workdir, invalid image, concurrent operations, crash recovery. Save to `.sisyphus/evidence/final-qa/`.
  Output: `Scenarios [N/N pass] | Integration [N/N] | Edge Cases [N tested] | VERDICT`

- [ ] F4. **Scope Fidelity Check** — `deep`
  For each task: read "What to do", read actual diff (`git log --oneline`, `git diff`). Verify 1:1 — everything in spec was built (no missing), nothing beyond spec was built (no creep). Check "Must NOT do" compliance per task. Detect cross-task contamination: Task N touching Task M's files. Flag unaccounted changes. Verify v0 code untouched.
  Output: `Tasks [N/N compliant] | Contamination [CLEAN/N issues] | Unaccounted [CLEAN/N files] | VERDICT`

---

## Commit Strategy

- **Task 1**: `feat(init): scaffold v1 packages, remove obsolete v0 code` — go.mod, go.sum, cmd/mesh/main.go, internal/*/CONTEXT.md, deleted internal/{clone,agent,transport}/
- **Task 2**: `feat(adapter): define SubstrateAdapter interface and body types` — internal/adapter/adapter.go, internal/adapter/types.go, internal/adapter/adapter_test.go
- **Task 3**: `feat(store): add SQLite store with WAL mode and body CRUD` — internal/store/store.go, internal/store/schema.go, internal/store/store_test.go
- **Task 4**: `feat(config): add YAML config parsing (replaces v0 TOML)` — internal/config/config.go, internal/config/config_test.go
- **Task 5**: `feat(body): add state machine and lifecycle orchestration` — internal/body/*.go, internal/body/*_test.go
- **Task 6**: `feat(docker): implement SubstrateAdapter with Docker SDK` — internal/docker/adapter.go, internal/docker/adapter_test.go, internal/docker/adapter_integration_test.go
- **Task 7**: `feat(persistence): add Docker-aware snapshot engine wrapping v0 pipeline` — internal/snapshot/reader.go, internal/restore/writer.go, internal/persistence/*.go
- **Task 8**: `feat(daemon): add daemon infrastructure with signal handling and graceful shutdown` — internal/daemon/daemon.go, internal/daemon/daemon_test.go
- **Task 9**: `feat(mcp): add MCP server with 8 P0 tools` — internal/mcp/server.go, internal/mcp/tools.go, internal/mcp/server_test.go
- **Task 10**: `feat(cli): add v1 CLI with init, serve, stop, status` — cmd/mesh/main.go
- **Task 11**: `feat(manifest): extend manifest with Docker container metadata` — internal/manifest/manifest.go, internal/manifest/manifest_test.go
- **Task 12**: `feat(mcp): add P1 tools — migrate, exec, logs, prune` — internal/mcp/tools.go, internal/mcp/server_test.go
- **Task 13**: `test(integration): add end-to-end body lifecycle tests` — internal/daemon/daemon_integration_test.go
- **Task 14**: `feat(daemon): add parallel graceful shutdown and health checks` — internal/daemon/shutdown.go, internal/daemon/health.go
- **Task 15**: `docs: update README for v1 with migration guide and tool reference` — README.md

---

## Success Criteria

### Verification Commands
```bash
go build ./cmd/mesh/                    # Expected: compiles without error
go test -race ./...                     # Expected: all tests pass, 0 races
golangci-lint run                       # Expected: 0 issues
go test -tags=integration ./...         # Expected: integration tests pass (requires Docker)
./mesh --help                           # Expected: init, serve, stop, status
echo '{"jsonrpc":"2.0","method":"tools/list","id":1}' | ./mesh serve  # Expected: JSON with 12 tools
```

### Final Checklist
- [ ] All "Must Have" present: daemon, MCP, Docker adapter, SQLite, 8-state body machine, 12 MCP tools
- [ ] All "Must NOT Have" absent: no Nomad, no Tailscale, no gRPC, no go-plugin, no S3, no CGo, no telemetry
- [ ] `go test -race ./...` passes with 0 failures
- [ ] v0 snapshot/restore/manifest packages untouched
- [ ] Body lifecycle works end-to-end: create→start→snapshot→stop→restore→start→destroy
- [ ] Daemon survives SIGTERM with graceful shutdown
- [ ] Integration tests pass with real Docker