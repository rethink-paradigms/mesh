# Mesh v1.0 — Implementation Plan

## TL;DR

> **Quick Summary**: Extend the existing Mesh codebase from Docker-only prototype to full v1.0: multi-substrate daemon (Docker + Nomad), cold migration via S3 registry, 16 MCP tools, minimal plugin system, and bootstrap pipeline.
> 
> **Deliverables**:
> - Fully wired daemon with Docker + Nomad substrates and startup reconciliation
> - 16 MCP tools (8 existing, 8 new) including execute_command
> - 7-step cold migration coordinator (steps 2-6 implemented from stubs)
> - S3-compatible registry plugin (streaming push/pull/verify)
> - Nomad adapter plugin (create/start/stop/export/import)
> - Plugin manager (load binary, health check, restart on crash)
> - CLI serve/stop/status commands
> - Bootstrap: goreleaser (macOS arm64 + Linux amd64), shell install script, Homebrew formula
> 
> **Estimated Effort**: Large (21 implementation tasks across 8 waves + 4 final verification)
> **Parallel Execution**: YES — 8 waves with max concurrency 7 in Wave 1
> **Critical Path**: T1 → T5 → T8 → T12 → T16 → T17 → T19 → F1-F4

---

## Context

### Original Request
Build Mesh v1.0 per the definitive scope (`.sisyphus/drafts/v1-definitive-scope.md`). Architecture locked, no redesign. Extend existing codebase. TDD approach. Go 1.25+. Real external deps when available.

### Interview Summary
**Key Discussions**:
- **Test Strategy**: TDD (Tests First) — RED-GREEN-REFACTOR for every task
- **External Deps**: Use real Docker (available locally), real Nomad/S3 when configured, skip tests otherwise
- **Go Version**: 1.25.5 (as-is from go.mod)
- **Agent QA**: Mandatory for every task — scripts verify deliverables by running/browsing/curling

**Research Findings**:
- **Built**: Docker adapter (all 10 methods), body state machine, body manager, SQLite store, snapshot/restore (v0), manifest v1/v2, YAML/TOML configs, MCP framework (7/8 tools), migration framework (step 1 partial, steps 2-7 stubs)
- **Partial**: Daemon (Docker field exists but NOT initialized, reconcile is no-op), CLI v1 commands (serve/stop/status stubs), execute_command MCP (stub)
- **Missing**: Plugin system (zero code), Nomad adapter (zero code), S3 registry (zero code)
- **Missing deps**: go-plugin, nomad/api, AWS SDK v2
- **Tests**: 174 test functions, 6,200+ lines, 100% package coverage, integration tests build-tag gated
- **CI**: Linux-only, no macOS, no integration tests in CI

### Metis Review
**Identified Gaps** (addressed in plan):
- **G1**: Docker adapter wiring is NOT a one-liner → daemon needs multi-adapter routing architecture (T5, T7)
- **G2**: "Minimal" plugin system undefined → defined: scan at startup, health every 30s, 3 retries at 1s, no dynamic load (T14-T15)
- **G3**: 16 MCP tools not fully listed → all 16 enumerated in T9-T11
- **G4**: Bootstrap: homebrew vs goreleaser vs shell script → all three, goreleaser first, homebrew as separate (T20)
- **G5**: Config schema for registry/plugins/nomad undefined → schema defined in T4
- **G6**: Nomad export mechanism unclear → alloc-fs first with sidecar fallback (T19)
- **G7**: Multiple daemon instances → PID file check at startup (T7)
- **G8**: Orphaned containers/stale records → startup reconciliation (T8)

---

## Work Objectives

### Core Objective
Transform Mesh from Docker-only prototype into a multi-substrate daemon with cold migration, plugin-based substrate adapters, and a production-ready bootstrap pipeline.

### Concrete Deliverables
- `internal/daemon/daemon.go` — Multi-adapter daemon with real reconciliation
- `internal/mcp/handlers.go` — 16 tools, all implemented
- `internal/body/migration.go` — Steps 2-7 fully implemented
- `internal/plugin/` — Plugin manager (new package)
- `internal/registry/` — S3 registry plugin (new package)
- `internal/nomad/` — Nomad adapter (new package)
- `cmd/mesh/main.go` — serve/stop/status commands functional
- `internal/config/config.go` — Registry, plugins, nomad sections
- `scripts/install.sh` — Shell installer
- `Formula/mesh.rb` — Homebrew formula
- `.goreleaser.yml` — Multi-platform (macOS arm64 + Linux amd64)

### Definition of Done
- [x] `mesh serve` starts daemon with Docker + Nomad substrates, reconciles state, exposes MCP
- [x] All 16 MCP tools return valid responses (not "not yet implemented")
- [x] `execute_command` runs commands inside Docker containers and returns stdout/stderr/exit code
- [x] Cold migration: Docker→Docker same-machine completes all 7 steps
- [x] S3 registry: push snapshot → pull snapshot → SHA-256 verification passes
- [x] Nomad adapter: Creates a body on Nomad cluster, can exec into it
- [x] Plugin manager: loads plugin binary, health checks each 30s, restarts on crash
- [x] `mesh status` shows daemon healthy, body count across substrates
- [x] `brew install` or shell script sets up mesh binary + default config
- [x] `go test ./...` passes with -race
- [x] `go test -tags=integration ./integration/` passes

### Must Have
- Docker substrate fully operational (create/start/stop/destroy/exec/snapshot/restore)
- Nomad substrate via plugin (create/start/stop/exec)
- Cold migration (same-machine Docker→Docker)
- S3 registry (push/pull/verify)
- Plugin manager (load/unload/health/restart)
- 16 MCP tools, all implemented
- CLI serve/stop/status
- Bootstrap (binary + install script)
- Startup reconciliation across all substrates
- Graceful shutdown with 30s timeout
- All tests pass, no race conditions

### Must NOT Have (Guardrails)
- ❌ Auto-scheduler / idle detection / cost model
- ❌ E2B, Fly, Modal, Cloudflare adapters
- ❌ Live migration (CRIU, memory snapshots)
- ❌ Inflatable containers (D8)
- ❌ clone-and-merge (filesystem delta merging)
- ❌ Web UI / dashboard
- ❌ Multi-user / teams
- ❌ IPFS registry
- ❌ K8s anything
- ❌ Plugin build system, version manager, marketplace
- ❌ Automatic substrate selection — user specifies explicitly
- ❌ Dynamic plugin loading — scan at startup only
- ❌ AI-generated plugin code

---

## Verification Strategy (MANDATORY)

> **ZERO HUMAN INTERVENTION** — ALL verification is agent-executed. No exceptions.

### Test Decision
- **Infrastructure exists**: YES (go test, race detector, 174 existing tests)
- **Automated tests**: TDD (Tests First) — RED-GREEN-REFACTOR
- **Framework**: `go test` with `-race`
- **Integration tests**: `go test -tags=integration ./integration/` (real Docker required)

### QA Policy
Every task MUST include agent-executed QA scenarios.
Evidence saved to `.sisyphus/evidence/task-{N}-{scenario-slug}.{ext}`.

- **API/MCP**: Use Bash (curl) — Send JSON-RPC requests, assert status + response fields
- **CLI**: Use interactive_bash (tmux) — Run mesh commands, validate output
- **Backend**: Use Bash — go test, go build, go vet

---

## Execution Strategy

### Parallel Execution Waves

```
Wave 1 (Start Immediately — foundation):
├── T1: Validate go-plugin + Go 1.25 compatibility [quick]
├── T2: Validate nomad/api + Go 1.25 compatibility [quick]
├── T3: Validate AWS SDK v2 + Go 1.25 compatibility [quick]
├── T4: Extend config schema (registry, plugins, nomad) [quick]
├── T5: Extend SubstrateAdapter interface for multi-adapter [quick]
└── T6: Extend Store for substrate tracking [quick]

Wave 2 (After Wave 1 — daemon core):
├── T7: Wire Docker adapter + multi-adapter routing into daemon [deep]
├── T8: Implement real reconciliation (Docker + edge cases) [deep]
├── T9: Implement execute_command MCP tool [deep]
├── T10: Add start_body + stop_body MCP tools [quick]
└── T11: Add snapshot/restore MCP tools (create_snapshot, list_snapshots, restore_body) [quick]

Wave 3 (After Wave 1 — MCP completion, independent of Wave 2):
├── T12: Add get_body_logs + get_body_status MCP tools [quick]
├── T13: Implement CLI serve/stop/status commands [quick]

Wave 4 (After Wave 2 — migration):
├── T14: Implement migration steps 2-3 (provision + transfer local) [deep]
├── T15: Implement migration steps 4-5 (import + verify) [deep]
└── T16: Implement migration steps 6-7 (switch + cleanup) [deep]

Wave 5 (After Wave 1 — plugin system, parallel with Wave 2-4):
├── T17: Plugin interface + protobuf + go-plugin scaffold [deep]
├── T18: Plugin manager (scan, load, health check, restart) [deep]
└── T19: Add list_plugins + plugin_health MCP tools [quick]

Wave 6 (After Wave 5 — substrate plugins, parallel):
├── T20: S3 registry plugin (scaffold + push + pull + verify) [deep]
└── T21: Nomad adapter plugin (create/start/stop/export/import) [deep]

Wave 7 (After Wave 6 — integration):
├── T22: Cross-machine Docker→Docker migration via S3 registry [deep]

Wave 8 (After Wave 7 — bootstrap, parallel):
├── T23: Goreleaser multi-platform + CI/CD updates [quick]
├── T24: Shell install script + Homebrew formula [quick]
├── T25: Integration test suite (all substrates + migration + plugins) [deep]

Wave FINAL (After ALL tasks — 4 parallel reviews):
├── F1: Plan Compliance Audit (oracle)
├── F2: Code Quality Review (unspecified-high)
├── F3: Real Manual QA (unspecified-high)
└── F4: Scope Fidelity Check (deep)
```

**Critical Path**: T1 → T5 → T7 → T8 → T14 → T16 → T20 → T22 → F1-F4
**Parallel Speedup**: ~60% faster than sequential
**Max Concurrent**: 7 (Wave 1)

### Dependency Matrix (full)

- **T1-T6**: None → Wave 1. All independent.
- **T7**: T5 → T8, T10, T11, T13
- **T8**: T7, T6 → T14
- **T9**: T5 → T22
- **T10**: T5 → T19, T22
- **T11**: T5 → T22
- **T12**: T5 → T22
- **T13**: T7, T4 → T23, T24
- **T14**: T8 → T15
- **T15**: T14 → T16
- **T16**: T15 → T22
- **T17**: T1 → T18
- **T18**: T17 → T19, T20, T21
- **T19**: T18, T10 → T22
- **T20**: T3, T18 → T22
- **T21**: T2, T18 → T22
- **T22**: T16, T20, T9-T12, T19 → T23, T24, T25
- **T23**: T13, T22 → done
- **T24**: T13, T22 → done
- **T25**: T22 → done
- **F1-F4**: ALL implementation tasks → FINAL

### Agent Dispatch Summary

- **Wave 1**: 6 × `quick` (all trivial validation + config)
- **Wave 2**: 2 × `deep` (daemon wiring, reconcile), 1 × `deep` (exec command), 2 × `quick` (MCP tools)
- **Wave 3**: 2 × `quick` (MCP + CLI)
- **Wave 4**: 3 × `deep` (migration steps)
- **Wave 5**: 2 × `deep` (plugin system), 1 × `quick` (plugin MCP tools)
- **Wave 6**: 2 × `deep` (S3 + Nomad plugins)
- **Wave 7**: 1 × `deep` (cross-machine migration)
- **Wave 8**: 2 × `quick` (bootstrap), 1 × `deep` (integration tests)
- **FINAL**: 1 × `oracle`, 2 × `unspecified-high`, 1 × `deep`

---

## TODOs

- [x] 1. Validate go-plugin + Go 1.25 compatibility

  **What to do**:
  - Run `go get github.com/hashicorp/go-plugin@latest` and verify it resolves
  - Run `go build ./...` to confirm no compilation errors
  - Write a test file `internal/plugin/compat_test.go` that imports go-plugin and compiles
  - Test: `go build ./...` PASS (no go-plugin import errors)

  **Must NOT do**:
  - Don't add go-plugin to go.mod via `go mod tidy` yet — only validate
  - Don't implement any plugin code

  **Recommended Agent Profile**:
  > Category: `quick` — trivial validation task, just go get + go build
  - **Category**: `quick`
    - Reason: Single validation step, no logic involved
  - **Skills**: []
  - **Skills Evaluated but Omitted**: N/A

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with T2, T3, T4, T5, T6)
  - **Blocks**: T17
  - **Blocked By**: None

  **References**:
  - `go.mod:1-7` — Current module definition and Go version (1.25.5)
  - https://github.com/hashicorp/go-plugin — Official go-plugin repo for compatibility reference

  **Acceptance Criteria** (TDD — test first):
  - [ ] Test file created: `internal/plugin/compat_test.go`
  - [ ] `go test ./internal/plugin/` → PASS (plugin package compiles with go-plugin import)
  - [ ] `go get github.com/hashicorp/go-plugin@latest` succeeds without version conflict

  **QA Scenarios**:
  ```
  Scenario: go-plugin compiles with Go 1.25
    Tool: Bash
    Preconditions: Clean go.mod (no go-plugin yet)
    Steps:
      1. go get github.com/hashicorp/go-plugin@latest
      2. go build ./...
    Expected Result: No errors. Build succeeds.
    Failure Indicators: "incompatible", "requires go", or compilation errors
    Evidence: .sisyphus/evidence/task-1-goplugin-compat.txt

  Scenario: go-plugin test file compiles
    Tool: Bash
    Preconditions: go-plugin added to go.mod
    Steps:
      1. Create test file importing hashicorp/go-plugin
      2. go test ./internal/plugin/
    Expected Result: Test compiles and passes (empty test OK)
    Failure Indicators: "cannot find package" or compilation errors
    Evidence: .sisyphus/evidence/task-1-goplugin-test.txt
  ```

  **Commit**: YES
  - Message: `chore(deps): validate go-plugin compatibility with Go 1.25`
  - Files: `internal/plugin/compat_test.go`, `go.mod`, `go.sum`

- [x] 2. Validate nomad/api + Go 1.25 compatibility

  **What to do**:
  - Run `go get github.com/hashicorp/nomad/api@latest` and verify it resolves
  - Run `go build ./...` to confirm no compilation errors
  - Write a test file `internal/nomad/compat_test.go` that imports nomad/api and compiles
  - Test: `go build ./...` PASS (no nomad import errors)

  **Must NOT do**:
  - Don't implement Nomad adapter code — only validate
  - Don't add nomad/api permanently without user confirmation if there are issues

  **Recommended Agent Profile**:
  > Category: `quick` — trivial validation task
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with T1, T3, T4, T5, T6)
  - **Blocks**: T21
  - **Blocked By**: None

  **References**:
  - `go.mod:1-7` — Current module definition
  - https://github.com/hashicorp/nomad — Nomad API client reference

  **Acceptance Criteria** (TDD):
  - [ ] Test file created: `internal/nomad/compat_test.go`
  - [ ] `go get github.com/hashicorp/nomad/api@latest` succeeds
  - [ ] `go build ./...` PASS

  **QA Scenarios**:
  ```
  Scenario: nomad/api compiles with Go 1.25
    Tool: Bash
    Preconditions: Clean go.mod (no nomad/api yet)
    Steps:
      1. go get github.com/hashicorp/nomad/api@latest
      2. go build ./...
    Expected Result: No errors. Build succeeds.
    Failure Indicators: Version conflict or compilation errors
    Evidence: .sisyphus/evidence/task-2-nomad-compat.txt
  ```

  **Commit**: YES
  - Message: `chore(deps): validate nomad/api compatibility with Go 1.25`
  - Files: `internal/nomad/compat_test.go`, `go.mod`, `go.sum`

- [x] 3. Validate AWS SDK v2 + Go 1.25 compatibility

  **What to do**:
  - Run `go get github.com/aws/aws-sdk-go-v2/config@latest` and `go get github.com/aws/aws-sdk-go-v2/service/s3@latest`
  - Verify both resolve without version conflicts
  - Write a test file `internal/registry/compat_test.go` that imports both packages and compiles
  - Test: `go build ./...` PASS

  **Must NOT do**:
  - Don't implement S3 registry code — only validate
  - Don't hardcode AWS credentials

  **Recommended Agent Profile**:
  > Category: `quick` — validation only
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with T1, T2, T4, T5, T6)
  - **Blocks**: T20
  - **Blocked By**: None

  **References**:
  - `go.mod:1-7` — Current module definition
  - https://aws.github.io/aws-sdk-go-v2/docs/ — AWS SDK v2 documentation

  **Acceptance Criteria** (TDD):
  - [ ] Test file created: `internal/registry/compat_test.go`
  - [ ] `go get github.com/aws/aws-sdk-go-v2/config@latest` + `go get github.com/aws/aws-sdk-go-v2/service/s3@latest` succeed
  - [ ] `go build ./...` PASS

  **QA Scenarios**:
  ```
  Scenario: AWS SDK v2 compiles with Go 1.25
    Tool: Bash
    Preconditions: Clean go.mod (no AWS SDK)
    Steps:
      1. go get github.com/aws/aws-sdk-go-v2/config@latest
      2. go get github.com/aws/aws-sdk-go-v2/service/s3@latest
      3. go build ./...
    Expected Result: No errors. Both packages resolve.
    Failure Indicators: "module requires go" or compilation errors
    Evidence: .sisyphus/evidence/task-3-aws-sdk-compat.txt
  ```

  **Commit**: YES
  - Message: `chore(deps): validate AWS SDK v2 compatibility with Go 1.25`
  - Files: `internal/registry/compat_test.go`, `go.mod`, `go.sum`

- [x] 4. Extend config schema (registry, plugins, nomad sections)

  **What to do**:
  - Add `RegistryConfig` struct: `Type` (string, "s3"), `Bucket`, `Region`, `Endpoint` (optional), `AccessKeyID`, `SecretAccessKey` (from env vars with fallback)
  - Add `PluginConfig` struct: `Dir` (path, default `~/.mesh/plugins/`), `Enabled` ([]string)
  - Add `NomadConfig` struct: `Address` (default `http://127.0.0.1:4646`), `Token` (from env var), `Region`, `Namespace`
  - Add `SubstrateConfig` to BodyConfig: `Substrate` (string, default "docker")
  - Wire new sections into `Config` struct
  - Add JSON tags for MCP serialization
  - Add validation: plugin dir must exist, nomad address must be valid URL format, s3 bucket required if type="s3"
  - Write tests for loading config with new sections from YAML
  - Test: `go test ./internal/config/` PASS with new sections

  **Must NOT do**:
  - Don't implement S3 client, Nomad client, or plugin loader — just config structs
  - Don't change existing config sections (daemon, store, docker, bodies)
  - Don't add config sections not in scope (no auto-scheduler, no cost model)

  **Recommended Agent Profile**:
  > Category: `quick` — extending existing config struct, straightforward
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with T1, T2, T3, T5, T6)
  - **Blocks**: T7, T13, T14, T18, T20, T21
  - **Blocked By**: None

  **References**:
  - `internal/config/config.go:1-116` — Existing config struct pattern, defaults, validation
  - `internal/config/config_test.go:1-391` — Existing test patterns for YAML loading
  - `internal/adapter/adapter.go:1-97` — SubstrateAdapter interface (for Substrate field type)

  **Acceptance Criteria** (TDD — test first):
  - [ ] Test cases added to `internal/config/config_test.go` for: registry section, plugin section, nomad section, substrate field on body
  - [ ] `go test ./internal/config/` → PASS (existing + new tests)
  - [ ] Config validates: missing bucket on s3 type → error, invalid nomad URL → error

  **QA Scenarios**:
  ```
  Scenario: Config loads with all new sections
    Tool: Bash
    Preconditions: Existing config test infrastructure
    Steps:
      1. Create test YAML with registry, plugins, nomad sections
      2. Run config.Load() with that YAML
      3. Assert registry.Bucket == "my-bucket"
      4. Assert plugins.Dir == "/home/user/.mesh/plugins/"
      5. Assert nomad.Address == "http://127.0.0.1:4646"
    Expected Result: All fields populated correctly from YAML
    Failure Indicators: Missing fields, wrong defaults, panic on nil
    Evidence: .sisyphus/evidence/task-4-config-extended.txt

  Scenario: Config validation rejects invalid nomad address
    Tool: Bash
    Preconditions: Config test file with test case
    Steps:
      1. Load YAML with nomad.address = "not:a:url"
      2. Run config.Validate()
    Expected Result: Error returned containing "invalid nomad address"
    Failure Indicators: Validation passes silently
    Evidence: .sisyphus/evidence/task-4-config-validation.txt
  ```

  **Commit**: YES
  - Message: `feat(config): add registry, plugin, and nomad config sections`
  - Files: `internal/config/config.go`, `internal/config/config_test.go`

- [x] 5. Extend SubstrateAdapter interface for multi-adapter routing

  **What to do**:
  - Add `SubstrateName() string` method to `SubstrateAdapter` interface (returns "docker", "nomad", etc.)
  - Add `IsHealthy(ctx) bool` method to `SubstrateAdapter` interface
  - Create `MultiAdapter` struct in `internal/adapter/` that holds `map[string]SubstrateAdapter`
  - Implement routing: `GetAdapter(name string) (SubstrateAdapter, error)`
  - Add `ListAdapters() []string` method
  - Add `Register(name string, adapter SubstrateAdapter)` method
  - `MultiAdapter` implements `SubstrateAdapter` (delegates to named adapter)
  - Write tests for multi-adapter registration and routing
  - Test: `go test ./internal/adapter/` PASS

  **Must NOT do**:
  - Don't implement the adapters themselves (docker is done, nomad comes later)
  - Don't add network-level routing — this is in-process delegation
  - Don't modify existing Docker adapter — it already satisfies the interface

  **Recommended Agent Profile**:
  > Category: `quick` — interface extension, straightforward delegation pattern
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with T1-T4, T6)
  - **Blocks**: T7, T9, T14, T18, T21
  - **Blocked By**: None

  **References**:
  - `internal/adapter/adapter.go:1-97` — Existing SubstrateAdapter interface, BodySpec, Capabilities
  - `internal/adapter/adapter_test.go:1-564` — Existing test patterns, mock adapter
  - `internal/docker/adapter.go:1-356` — Docker adapter implementation (already satisfies all existing methods)

  **Acceptance Criteria** (TDD — test first):
  - [ ] Test cases added to `internal/adapter/adapter_test.go` for: MultiAdapter registration, routing, delegation, unknown adapter error, ListAdapters
  - [ ] `go test ./internal/adapter/` → PASS
  - [ ] MultiAdapter.GetAdapter("docker") returns adapter implementing all methods
  - [ ] MultiAdapter.GetAdapter("nonexistent") returns error "adapter not found: nonexistent"

  **QA Scenarios**:
  ```
  Scenario: MultiAdapter routes to correct adapter
    Tool: Bash
    Preconditions: MultiAdapter with mock "docker" registered
    Steps:
      1. Create MultiAdapter, register mock named "docker"
      2. GetAdapter("docker")
      3. Call Create() on returned adapter
      4. Assert mock.Create was called
    Expected Result: Call delegated to correct adapter
    Failure Indicators: Panic, wrong adapter called, nil pointer
    Evidence: .sisyphus/evidence/task-5-multi-adapter.txt
  ```

  **Commit**: YES
  - Message: `feat(adapter): add MultiAdapter for substrate routing`
  - Files: `internal/adapter/adapter.go`, `internal/adapter/adapter_test.go`

- [x] 6. Extend Store for substrate tracking

  **What to do**:
  - Add `substrate` TEXT column to `bodies` table in schema migration v2
  - Add `substrate` field to `BodyRecord` struct
  - Update `CreateBody` to accept and persist substrate
  - Update `GetBody`, `ListBodies` to return substrate
  - Add `ListBodiesBySubstrate(ctx, substrate string)` query method
  - Write schema migration: detect v1 → upgrade to v2 adding substrate column (default "docker")
  - Write tests for substrate persistence and query
  - Test: `go test ./internal/store/` PASS with new column

  **Must NOT do**:
  - Don't change existing schema v1 — add v2 migration
  - Don't break backward compatibility with bodies that lack substrate (default: "docker")
  - Don't add foreign key constraints on substrate (substrate names are not an enum)

  **Recommended Agent Profile**:
  > Category: `quick` — single column addition, schema migration pattern exists
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with T1-T5)
  - **Blocks**: T7, T8, T14
  - **Blocked By**: None

  **References**:
  - `internal/store/store.go:1-433` — Existing schema migration (v1), BodyRecord struct, CRUD operations
  - `internal/store/store_test.go:1-398` — Existing test patterns for body CRUD
  - `internal/adapter/adapter.go:SubstrateAdapter` — Substrate naming (for column values)

  **Acceptance Criteria** (TDD — test first):
  - [ ] Test cases added to `internal/store/store_test.go` for: body created with substrate, list by substrate, default "docker" substrate, schema migration v1→v2
  - [ ] `go test ./internal/store/` → PASS
  - [ ] `ListBodiesBySubstrate("docker")` returns only docker bodies
  - [ ] Existing bodies (without substrate) default to "docker"

  **QA Scenarios**:
  ```
  Scenario: Body persists and retrieves substrate
    Tool: Bash
    Preconditions: Fresh store with v2 schema
    Steps:
      1. Create body with substrate "docker"
      2. Get body by ID
      3. Assert body.Substrate == "docker"
    Expected Result: Substrate field persisted and retrieved correctly
    Failure Indicators: Missing substrate, wrong value, NULL
    Evidence: .sisyphus/evidence/task-6-store-substrate.txt

  Scenario: Schema migration v1→v2 preserves data
    Tool: Bash
    Preconditions: Store with v1 schema containing existing bodies
    Steps:
      1. Open store with v1 schema
      2. Trigger migration to v2
      3. Query existing bodies
    Expected Result: All existing bodies have substrate "docker"
    Failure Indicators: Data loss, migration failure, NULL substrate
    Evidence: .sisyphus/evidence/task-6-store-migration.txt
  ```

  **Commit**: YES
  - Message: `feat(store): add substrate column to bodies table (v2 migration)`
  - Files: `internal/store/store.go`, `internal/store/store_test.go`

- [x] 7. Wire Docker adapter + multi-adapter routing into daemon

  **What to do**:
  - In `daemon.New()` or `Start()`: initialize Docker adapter from config (`docker.NewDockerAdapter(cfg)`)
  - Create `MultiAdapter`, register Docker adapter as "docker"
  - Store `MultiAdapter` in daemon struct (replace single `docker` field)
  - Add `adapters *adapter.MultiAdapter` field to Daemon struct
  - Initialize `BodyManager` with `MultiAdapter` and store
  - Add `bodyMgr *body.BodyManager` field to Daemon struct
  - Add PID file conflict check: read existing PID, check if process alive, refuse start if so
  - Wire daemon into `cmd/mesh serve`: call `daemon.New()` → `daemon.Start()` 
  - Write test for daemon startup with Docker adapter wired
  - Test: `go test ./internal/daemon/` PASS (docker adapter initialized)

  **Must NOT do**:
  - Don't implement reconciliation in this task — that's T8
  - Don't implement CLI serve logic in this task — that's T13
  - Don't add Nomad adapter yet — just Docker
  - Don't remove backward compatibility with code that accesses `d.docker` directly

  **Recommended Agent Profile**:
  > Category: `deep` — wiring multiple subsystems together, architectural change to daemon struct
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (depends on T5, T6)
  - **Parallel Group**: Wave 2 (sequential with T8 — T8 depends on T7)
  - **Blocks**: T8, T10, T11, T13
  - **Blocked By**: T5 (MultiAdapter), T6 (Store substrate)

  **References**:
  - `internal/daemon/daemon.go:1-245` — Current daemon struct, Start/Stop, health server, PID file
  - `internal/daemon/daemon_test.go:1-371` — Existing daemon test patterns
  - `internal/docker/adapter.go:1-356` — Docker adapter constructor and methods
  - `internal/adapter/adapter.go` — MultiAdapter interface (from T5)
  - `internal/body/manager.go:1-220` — BodyManager constructor and methods
  - `internal/body/body.go:1-55` — Body struct with state machine

  **Acceptance Criteria** (TDD — test first):
  - [ ] Test cases added to `internal/daemon/daemon_test.go`: daemon starts with docker adapter, daemon refuses duplicate start (PID conflict), body manager initialized, multi-adapter has "docker" registered
  - [ ] `go test ./internal/daemon/` → PASS
  - [ ] `go vet ./internal/daemon/` → clean

  **QA Scenarios**:
  ```
  Scenario: Daemon starts with Docker adapter wired
    Tool: Bash
    Preconditions: Docker running on host
    Steps:
      1. go build -o mesh ./cmd/mesh/
      2. ./mesh serve &
      3. sleep 2
      4. curl http://localhost:<health-port>/healthz
    Expected Result: {"status":"ok","bodies":0}
    Failure Indicators: Daemon fails to start, "adapter not found", health check returns 503
    Evidence: .sisyphus/evidence/task-7-daemon-wired.json

  Scenario: Daemon refuses duplicate start
    Tool: Bash
    Preconditions: Running daemon instance with PID file
    Steps:
      1. ./mesh serve & (first instance, already running)
      2. ./mesh serve (second instance)
    Expected Result: Error "daemon already running (pid N)" or exit code != 0
    Failure Indicators: Second instance starts, overwrites PID file
    Evidence: .sisyphus/evidence/task-7-daemon-duplicate.txt
  ```

  **Commit**: YES
  - Message: `feat(daemon): wire Docker adapter and multi-adapter routing`
  - Files: `internal/daemon/daemon.go`, `internal/daemon/daemon_test.go`

- [x] 8. Implement real reconciliation (Docker + edge cases)

  **What to do**:
  - Replace no-op `reconcile()` with real implementation
  - For each body in store: look up by substrate, check adapter for container existence
  - If body state is Running but container doesn't exist → transition to Error ("container not found")
  - If container exists but store has no record → log warning, offer cleanup option
  - If body state is Error but container exists → attempt GetStatus to verify
  - For bodies in Migrating state → check if migration record exists, resume or transition to Error
  - Add reconcile step count to health endpoint response (for monitoring)
  - Write tests: orphaned container, orphaned store record, state mismatch, migration recovery
  - Test: `go test ./internal/daemon/` PASS with reconcile tests

  **Must NOT do**:
  - Don't auto-delete orphaned containers — log warning only
  - Don't reconcile across substrates not yet wired (Nomad comes later)
  - Don't modify store without explicit state transitions

  **Recommended Agent Profile**:
  > Category: `deep` — multi-case reconciliation with edge cases, state transitions
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (depends on T7)
  - **Parallel Group**: Wave 2
  - **Blocks**: T14
  - **Blocked By**: T7 (daemon wired), T6 (store substrate)

  **References**:
  - `internal/daemon/daemon.go:153-166` — Current stub reconcile function
  - `internal/body/body.go:1-55` — Valid state transitions map
  - `internal/store/store.go` — ListBodies, GetBody, UpdateState methods
  - `internal/docker/adapter.go:GetStatus`, `Inspect` — Container existence checks
  - `internal/body/migration.go` — Migration record persistence (for Migrating state recovery)

  **Acceptance Criteria** (TDD — test first):
  - [ ] Test cases added: reconcile detects missing container, reconcile detects orphaned store record, reconcile handles Migrating state, reconcile logs warning for orphans
  - [ ] `go test ./internal/daemon/` → PASS

  **QA Scenarios**:
  ```
  Scenario: Reconcile detects missing Docker container
    Tool: Bash
    Preconditions: Store record says body "test-1" is Running, no Docker container
    Steps:
      1. Create body record with state=Running, instance_id="nonexistent"
      2. Run reconcile
      3. Check body state in store
    Expected Result: Body state changed to Error with message "container not found"
    Failure Indicators: Body stays in Running state, panic, wrong state transition
    Evidence: .sisyphus/evidence/task-8-reconcile-missing.txt

  Scenario: Reconcile leaves healthy bodies unchanged
    Tool: Bash
    Preconditions: Store record + running Docker container
    Steps:
      1. Create body with state=Running, valid Docker container
      2. Run reconcile
      3. Check body state in store
    Expected Result: Body state still Running, no error
    Failure Indicators: Healthy body incorrectly transitioned
    Evidence: .sisyphus/evidence/task-8-reconcile-healthy.txt
  ```

  **Commit**: YES
  - Message: `feat(daemon): implement startup reconciliation for Docker substrate`
  - Files: `internal/daemon/daemon.go`, `internal/daemon/daemon_test.go`

- [x] 9. Implement execute_command MCP tool

  **What to do**:
  - Replace stub in `handleExecCommand` with real implementation
  - Look up body in store → get substrate → get adapter from MultiAdapter
  - Call `adapter.Exec(ctx, instanceID, command)` 
  - Return: `{stdout, stderr, exit_code}`
  - Add timeout support: default 30s, configurable via optional `timeout_seconds` param
  - Handle adapter errors: body not found, container not running, exec timeout
  - Write tests: exec on running container, exec on stopped container, exec timeout, empty command
  - Test: `go test ./internal/mcp/` PASS with exec tests

  **Must NOT do**:
  - Don't implement streaming stdout/stderr — batch return is fine for v1
  - Don't add shell injection protection beyond what Docker provides
  - Don't add command allowlists/denylists

  **Recommended Agent Profile**:
  > Category: `deep` — implementing previously stubbed functionality with error handling
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with T10, T11 after T7)
  - **Blocks**: T22
  - **Blocked By**: T5 (MultiAdapter), T7 (daemon wired)

  **References**:
  - `internal/mcp/handlers.go:170-192` — Current execute_command stub
  - `internal/mcp/mcp_test.go:1-705` — Existing MCP test patterns, JSON-RPC request/response format
  - `internal/docker/adapter.go:Exec()` — Docker Exec implementation (demux, timeout)
  - `internal/adapter/adapter.go:ExecResult` — ExecResult struct with Stdout, Stderr, ExitCode

  **Acceptance Criteria** (TDD — test first):
  - [ ] Test cases added to `internal/mcp/mcp_test.go`: exec "echo hello", exec "ls /nonexistent" (exit code 2+), exec on stopped body (error), exec with timeout, exec with empty command (error)
  - [ ] `go test ./internal/mcp/` → PASS
  - [ ] `echo hello` returns `{stdout: "hello\n", stderr: "", exit_code: 0}`

  **QA Scenarios**:
  ```
  Scenario: Execute command on running body
    Tool: Bash (curl via MCP stdio proxy or direct test)
    Preconditions: Running Docker container body
    Steps:
      1. Send MCP tools/call: execute_command {body_id: "test-1", command: ["echo", "hello"]}
      2. Parse JSON-RPC response
    Expected Result: result.stdout == "hello\n", result.exit_code == 0
    Failure Indicators: "not yet implemented", stderr with error, timeout
    Evidence: .sisyphus/evidence/task-9-exec-success.json

  Scenario: Execute command on stopped body returns error
    Tool: Bash
    Preconditions: Stopped body in store
    Steps:
      1. Send MCP tools/call: execute_command {body_id: "stopped-body", command: ["echo", "test"]}
    Expected Result: Error response with code -32603, message contains "not running"
    Failure Indicators: Command succeeds on stopped container, panic
    Evidence: .sisyphus/evidence/task-9-exec-stopped.json
  ```

  **Commit**: YES
  - Message: `feat(mcp): implement execute_command tool`
  - Files: `internal/mcp/handlers.go`, `internal/mcp/mcp_test.go`

- [x] 10. Add start_body + stop_body MCP tools

  **What to do**:
  - Register `start_body` tool: params `{body_id}`, calls `bodyMgr.Start(ctx, bodyID)`
  - Register `stop_body` tool: params `{body_id}`, calls `bodyMgr.Stop(ctx, bodyID)` with default 30s timeout
  - Return body state after operation
  - Handle errors: body not found, invalid state transition (already started/stopped), timeout
  - Write tests: start stopped body, stop running body, start already-running (error), stop already-stopped (error)
  - Test: `go test ./internal/mcp/` PASS with start/stop tests

  **Must NOT do**:
  - Don't add force stop (SIGKILL) in v1 — graceful stop only
  - Don't add restart tool (combine stop+start if needed)

  **Recommended Agent Profile**:
  > Category: `quick` — two simple MCP tool handlers following existing create_body pattern
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with T9, T11 after T7)
  - **Blocks**: T22
  - **Blocked By**: T7 (body manager in daemon)

  **References**:
  - `internal/mcp/handlers.go:94-134` — create_body handler pattern (param parsing, error handling, bodyMgr call)
  - `internal/mcp/handlers.go:11-52` — Tool registration pattern
  - `internal/body/manager.go:Start()`, `Stop()` — BodyManager methods
  - `internal/body/body.go:validTransitions` — Valid state transitions for error messages

  **Acceptance Criteria** (TDD — test first):
  - [ ] Test cases: start_body on Stopped → Running, stop_body on Running → Stopped, start on Running → error, stop on Stopped → error
  - [ ] `go test ./internal/mcp/` → PASS

  **QA Scenarios**:
  ```
  Scenario: Start a stopped body
    Tool: Bash (curl MCP)
    Preconditions: Stopped body in store
    Steps:
      1. Send MCP tools/call: start_body {body_id: "stopped-body"}
    Expected Result: result.state == "running", no error
    Evidence: .sisyphus/evidence/task-10-start-body.json

  Scenario: Stop a running body
    Tool: Bash (curl MCP)
    Preconditions: Running body
    Steps:
      1. Send MCP tools/call: stop_body {body_id: "running-body"}
    Expected Result: result.state == "stopped", no error
    Evidence: .sisyphus/evidence/task-10-stop-body.json
  ```

  **Commit**: YES
  - Message: `feat(mcp): add start_body and stop_body tools`
  - Files: `internal/mcp/handlers.go`, `internal/mcp/mcp_test.go`

- [x] 11. Add snapshot/restore MCP tools (create_snapshot, list_snapshots, restore_body)

  **What to do**:
  - Register `create_snapshot` tool: params `{body_id, label?}`, calls store.CreateSnapshot + adapter.ExportFilesystem + snapshot pipeline (tar+zstd+sha256)
  - Register `list_snapshots` tool: params `{body_id?}`, calls store.ListSnapshots(filtered by body_id if provided)
  - Register `restore_body` tool: params `{snapshot_id, target_substrate?}`, restores snapshot to new body
  - Return snapshot metadata (ID, body_id, created_at, size_bytes, sha256)
  - Handle errors: body not found, body not running, snapshot pipeline failure
  - Write tests: create snapshot of running body, list snapshots, restore from snapshot, create snapshot of stopped body (error)
  - Test: `go test ./internal/mcp/` PASS

  **Must NOT do**:
  - Don't implement snapshot retention logic (pruning) — that's v0 CLI
  - Don't implement streaming snapshot download via MCP — return metadata only

  **Recommended Agent Profile**:
  > Category: `quick` — three MCP tools wrapping existing snapshot/restore pipelines
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with T9, T10)
  - **Blocks**: T22
  - **Blocked By**: T7

  **References**:
  - `internal/snapshot/snapshot.go:1-497` — Snapshot pipeline (CreateSnapshot, tar+zstd+sha256)
  - `internal/restore/restore.go:1-352` — Restore pipeline
  - `internal/store/store.go` — Snapshot CRUD (CreateSnapshot, ListSnapshots, GetSnapshot)
  - `internal/mcp/handlers.go:11-52` — Tool registration pattern

  **Acceptance Criteria** (TDD):
  - [ ] Test cases: create_snapshot returns metadata, list_snapshots returns array, restore_body creates new body
  - [ ] `go test ./internal/mcp/` → PASS

  **QA Scenarios**:
  ```
  Scenario: Create snapshot of running body via MCP
    Tool: Bash (curl MCP)
    Preconditions: Running Docker body
    Steps:
      1. Send MCP tools/call: create_snapshot {body_id: "body-1", label: "pre-migration"}
    Expected Result: result contains snapshot_id, sha256, size_bytes, body_id
    Evidence: .sisyphus/evidence/task-11-create-snapshot.json
  ```

  **Commit**: YES
  - Message: `feat(mcp): add create_snapshot, list_snapshots, and restore_body tools`
  - Files: `internal/mcp/handlers.go`, `internal/mcp/mcp_test.go`

- [x] 12. Add get_body_logs + get_body_status MCP tools

  **What to do**:
  - Register `get_body_logs` tool: params `{body_id, tail?}`, calls adapter.GetLogs or adapter.Exec(["tail", "-n", N, "/var/log/mesh.log"])
  - Register `get_body_status` tool: params `{body_id}`, calls adapter.GetStatus → returns state, uptime, memory_mb, cpu_usage
  - Combine with existing get_body (store data) to provide full status picture
  - Handle errors: body not found, container not running (no logs), adapter not supporting logs
  - Write tests: get_logs on running body, get_status on running body, get_logs on stopped (error)
  - Test: `go test ./internal/mcp/` PASS

  **Must NOT do**:
  - Don't implement log streaming/tailing — batch return only
  - Don't implement log persistence between restarts

  **Recommended Agent Profile**:
  > Category: `quick` — two simple MCP tool handlers
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with T13, independent of T8-T11)
  - **Blocks**: T22
  - **Blocked By**: T7

  **References**:
  - `internal/docker/adapter.go:GetStatus()`, `Exec()` — Docker status and exec
  - `internal/adapter/adapter.go:BodyStatus` — Status struct with state, uptime, memory
  - `internal/mcp/handlers.go:58-92` — get_body handler pattern

  **Acceptance Criteria** (TDD):
  - [ ] Test cases: get_body_status returns state+uptime, get_body_logs returns recent log lines
  - [ ] `go test ./internal/mcp/` → PASS

  **QA Scenarios**:
  ```
  Scenario: Get status of running body
    Tool: Bash (curl MCP)
    Preconditions: Running Docker body
    Steps:
      1. Send MCP tools/call: get_body_status {body_id: "body-1"}
    Expected Result: result.state == "running", result.uptime_seconds > 0
    Evidence: .sisyphus/evidence/task-12-get-status.json
  ```

  **Commit**: YES
  - Message: `feat(mcp): add get_body_logs and get_body_status tools`
  - Files: `internal/mcp/handlers.go`, `internal/mcp/mcp_test.go`

- [x] 13. Implement CLI serve/stop/status commands

  **What to do**:
  - Implement `mesh serve`: load config, create daemon, wire adapters, set MCP server, start daemon (blocking)
  - Implement `mesh stop`: read PID file, send SIGTERM, wait for process exit with timeout
  - Implement `mesh status`: read PID file, check process alive, query health endpoint, display daemon status + body count
  - Add `--config` flag to serve for custom config path
  - Add `--timeout` flag to stop (default 30s)
  - Handle errors: daemon already running, daemon not running (stop/status), config load failure
  - Write tests: serve starts daemon, stop kills daemon, status shows running/stopped
  - Test: `go test ./cmd/mesh/` PASS

  **Must NOT do**:
  - Don't implement background/daemonize mode — user handles backgrounding
  - Don't add service manager integration (systemd, launchd)

  **Recommended Agent Profile**:
  > Category: `quick` — CLI commands following existing Cobra patterns
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with T12)
  - **Blocks**: T23, T24
  - **Blocked By**: T7 (daemon wired), T4 (config extended)

  **References**:
  - `cmd/mesh/main.go:1-536` — Existing CLI commands (Cobra), config loading, v0 command patterns
  - `cmd/mesh/main_test.go:1-431` — Existing CLI test patterns
  - `internal/daemon/daemon.go:45-53, 82-125` — Daemon New() and Start() methods
  - `internal/config/config.go:Load()` — Config loading function

  **Acceptance Criteria** (TDD):
  - [ ] Test cases added to `cmd/mesh/main_test.go`: serve starts daemon, stop sends signal, status shows daemon running, status shows daemon stopped
  - [ ] `go test ./cmd/mesh/` → PASS
  - [ ] `./mesh serve` starts and blocks until SIGTERM
  - [ ] `./mesh status` returns "running" or "not running"

  **QA Scenarios**:
  ```
  Scenario: mesh serve starts daemon and responds to health check
    Tool: interactive_bash (tmux)
    Preconditions: Built mesh binary
    Steps:
      1. ./mesh serve &
      2. sleep 2
      3. curl http://localhost:<port>/healthz
      4. kill %1
    Expected Result: Health check returns {"status":"ok"}, daemon stops cleanly
    Evidence: .sisyphus/evidence/task-13-serve-health.json

  Scenario: mesh status when daemon is running
    Tool: Bash
    Preconditions: Running daemon from serve
    Steps:
      1. ./mesh status
    Expected Result: Output contains "running" and PID
    Failure Indicators: "not running", "not yet implemented", error exit
    Evidence: .sisyphus/evidence/task-13-status.txt
  ```

  **Commit**: YES
  - Message: `feat(cli): implement serve, stop, and status commands`
  - Files: `cmd/mesh/main.go`, `cmd/mesh/main_test.go`

- [x] 14. Implement migration steps 2-3 (provision + transfer local)

  **What to do**:
  - Step 2 (provision): Create target container from snapshot metadata. Use `adapter.Create()` on target adapter with same image + workdir. Store target instance ID in migration record. If provision fails, rollback: destroy target container.
  - Step 3 (transfer local): For same-machine Docker→Docker, copy snapshot tarball to target container's workdir via `adapter.ImportFilesystem()`. For cross-machine, this step will be replaced by S3 push/pull in T22.
  - Make each step idempotent: check if step already completed before executing (via migration record current_step field)
  - Make steps retryable: each step error is recorded but doesn't abort the entire migration
  - Update migration record after each step with status and timestamps
  - Write tests: provision creates container, transfer copies files, retry after partial failure
  - Test: `go test ./internal/body/` PASS with migration tests

  **Must NOT do**:
  - Don't implement cross-machine transfer via S3 — that's T22
  - Don't implement migration rollback for completed steps — only for current step failure
  - Don't add network transfer — local filesystem copy only

  **Recommended Agent Profile**:
  > Category: `deep` — implementing complex stateful workflow steps with idempotency guarantees
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (depends on T8, T14 blocks T15)
  - **Parallel Group**: Wave 4 — sequential within wave
  - **Blocks**: T15
  - **Blocked By**: T8 (reconciliation handles Migrating state), T5 (MultiAdapter)

  **References**:
  - `internal/body/migration.go:1-210` — MigrationCoordinator, step skeleton, state persistence
  - `internal/docker/adapter.go:Create()`, `ImportFilesystem()` — Docker adapter methods for provision/transfer
  - `internal/store/store.go:migration` — Migration record CRUD (Create, Update, Get)
  - `internal/snapshot/snapshot.go` — Snapshot path and metadata for transfer source

  **Acceptance Criteria** (TDD — test first):
  - [ ] Test cases: provision creates target container, provision idempotent (no double-create), transfer copies files to target, transfer retry after failure, migration record updated with current_step
  - [ ] `go test ./internal/body/` → PASS

  **QA Scenarios**:
  ```
  Scenario: Provision creates target container for migration
    Tool: Bash (go test)
    Preconditions: Migrating body with snapshot created
    Steps:
      1. Begin migration: body_id=test-1, target=docker
      2. Run step 2 (provision)
      3. Check Docker: new container exists with same image
    Expected Result: Target container created, migration record step=3
    Failure Indicators: No container created, step not advanced
    Evidence: .sisyphus/evidence/task-14-provision.txt

  Scenario: Transfer copies snapshot to target container
    Tool: Bash (go test)
    Preconditions: Target container created, snapshot tarball exists
    Steps:
      1. Run step 3 (transfer)
      2. Exec ls /workdir/ in target container
    Expected Result: Snapshot files present in target container
    Evidence: .sisyphus/evidence/task-14-transfer.txt
  ```

  **Commit**: YES
  - Message: `feat(migration): implement provision and local transfer steps`
  - Files: `internal/body/migration.go`, `internal/body/body_test.go`

- [x] 15. Implement migration steps 4-5 (import + verify)

  **What to do**:
  - Step 4 (import): Extract snapshot tarball into target container's workdir. Use adapter.ImportFilesystem() with overwrite. Verify extraction completed without errors.
  - Step 5 (verify): Run health check on target container. Exec a verification command (default: `echo ok`). Compare filesystem state: exec `ls /workdir/` and check expected files exist. Verify target container responds to adapter.GetStatus().
  - Make steps idempotent and retryable (same pattern as T14)
  - Write tests: import restores files, verify detects missing files, verify detects unhealthy container
  - Test: `go test ./internal/body/` PASS

  **Must NOT do**:
  - Don't do full byte-level comparison of filesystem — basic file existence check for v1
  - Don't verify snapshot checksum in this step — that's done during snapshot creation

  **Recommended Agent Profile**:
  > Category: `deep` — filesystem import verification with error detection
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 4 — sequential
  - **Blocks**: T16
  - **Blocked By**: T14

  **References**:
  - `internal/body/migration.go` — Step skeletons
  - `internal/docker/adapter.go:ImportFilesystem()`, `Exec()`, `GetStatus()`
  - `internal/restore/restore.go:1-352` — Restore pipeline for import reference

  **Acceptance Criteria** (TDD):
  - [ ] Test cases: import extracts files correctly, verify passes on healthy container, verify fails on missing files, verify fails on unhealthy container
  - [ ] `go test ./internal/body/` → PASS

  **QA Scenarios**:
  ```
  Scenario: Import restores files to target container
    Tool: Bash (go test)
    Preconditions: Snapshot transferred to target container
    Steps:
      1. Run step 4 (import)
      2. Exec ls /workdir/ in target
    Expected Result: Expected files present, step advanced to 5
    Evidence: .sisyphus/evidence/task-15-import.txt

  Scenario: Verify catches unhealthy target
    Tool: Bash (go test)
    Preconditions: Target container stopped (unhealthy)
    Steps:
      1. Run step 5 (verify) on stopped container
    Expected Result: Verify fails with "target container not healthy"
    Evidence: .sisyphus/evidence/task-15-verify-fail.txt
  ```

  **Commit**: YES
  - Message: `feat(migration): implement import and verify steps`
  - Files: `internal/body/migration.go`, `internal/body/body_test.go`

- [x] 16. Implement migration steps 6-7 (switch + cleanup)

  **What to do**:
  - Step 6 (switch): Stop source body (Running → Stopping → Stopped), verify target is healthy, optionally delete source container, update body record to point to target instance ID, transition body from Migrating → Running
  - Step 7 (cleanup): Delete snapshot tarball from local disk, delete migration record (or mark completed), prune old snapshots from source body
  - Handle rollback: if switch fails, revert body to source container, mark target for cleanup
  - Make steps idempotent: switching to already-switched body is no-op
  - Write tests: switch updates body instance ID, cleanup removes snapshot files, rollback on switch failure
  - Test: `go test ./internal/body/` PASS

  **Must NOT do**:
  - Don't keep both containers running after switch — stop source
  - Don't delete migration record before verifying switch success

  **Recommended Agent Profile**:
  > Category: `deep` — critical state transitions with rollback logic
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 4 — sequential
  - **Blocks**: T22
  - **Blocked By**: T15

  **References**:
  - `internal/body/migration.go` — Step skeletons and state persistence
  - `internal/body/manager.go:Stop()` — Body stop method
  - `internal/store/store.go:UpdateState()` — State transition persistence
  - `internal/body/body.go:validTransitions` — Migrating → Running transition

  **Acceptance Criteria** (TDD):
  - [ ] Test cases: switch updates instance_id and transitions to Running, cleanup removes tarball, rollback reverts to source on switch failure
  - [ ] `go test ./internal/body/` → PASS

  **QA Scenarios**:
  ```
  Scenario: Full migration completes end-to-end (same-machine)
    Tool: Bash (go test)
    Preconditions: Running body on Docker
    Steps:
      1. BeginMigration(body_id="test-1", target="docker")
      2. Run all 7 steps sequentially
      3. Check body state == Running, instance_id changed
    Expected Result: Body migrated successfully, old container stopped
    Evidence: .sisyphus/evidence/task-16-migration-e2e.txt
  ```

  **Commit**: YES
  - Message: `feat(migration): implement switch and cleanup steps`
  - Files: `internal/body/migration.go`, `internal/body/body_test.go`

- [x] 17. Plugin interface + protobuf + go-plugin scaffold

  **What to do**:
  - Add `github.com/hashicorp/go-plugin` to go.mod permanently
  - Define plugin interface in `internal/plugin/interface.go`: `MeshPlugin` with methods `PluginInfo() PluginMeta`, `GetAdapter() adapter.SubstrateAdapter`
  - Create protobuf definition for plugin RPC: `plugin.proto` with `PluginInfo`, `HealthCheck`, `GetAdapter` services
  - Generate Go code from protobuf: `protoc --go_out=. --go-grpc_out=. plugin.proto`
  - Implement gRPC plugin server: wraps MeshPlugin, serves via go-plugin
  - Implement gRPC plugin client: connects to plugin process, proxies calls
  - Create reference plugin `internal/plugin/reference/` that implements MeshPlugin with filesystem-local adapter
  - Write test: reference plugin starts via go-plugin, responds to PluginInfo, GetAdapter returns valid adapter
  - Test: `go test ./internal/plugin/` PASS

  **Must NOT do**:
  - Don't implement plugin manager (scan, health, restart) — that's T18
  - Don't implement actual Nomad or S3 adapters — those are T20, T21
  - Don't add dynamic plugin loading — scan at startup only
  - Don't generate protobuf at build time — commit generated code

  **Recommended Agent Profile**:
  > Category: `deep` — new package with protobuf, gRPC, and go-plugin integration
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (depends on T1)
  - **Parallel Group**: Wave 5
  - **Blocks**: T18, T19, T20, T21
  - **Blocked By**: T1 (go-plugin validated)

  **References**:
  - https://github.com/hashicorp/go-plugin — go-plugin docs, examples, gRPC plugin pattern
  - `internal/adapter/adapter.go` — SubstrateAdapter interface (returned by GetAdapter)
  - `internal/docker/adapter.go` — Example SubstrateAdapter implementation

  **Acceptance Criteria** (TDD):
  - [ ] Test file: `internal/plugin/plugin_test.go` — reference plugin starts, PluginInfo returns metadata, GetAdapter returns working adapter
  - [ ] `go test ./internal/plugin/` → PASS
  - [ ] `go build ./...` → clean (protobuf generated code compiles)

  **QA Scenarios**:
  ```
  Scenario: Reference plugin starts via go-plugin and responds to RPC
    Tool: Bash (go test)
    Preconditions: go-plugin added, protobuf generated
    Steps:
      1. Start plugin process via go-plugin client
      2. Call PluginInfo() RPC
    Expected Result: PluginInfo returns {name: "reference", version: "0.1.0"}
    Failure Indicators: Connection refused, RPC timeout, panic in plugin process
    Evidence: .sisyphus/evidence/task-17-plugin-start.txt

  Scenario: Reference plugin adapter implements SubstrateAdapter
    Tool: Bash (go test)
    Steps:
      1. Call GetAdapter() RPC
      2. Call adapter.Capabilities()
    Expected Result: Capabilities returns valid struct
    Evidence: .sisyphus/evidence/task-17-plugin-adapter.txt
  ```

  **Commit**: YES
  - Message: `feat(plugin): add plugin interface, protobuf, and go-plugin scaffold with reference plugin`
  - Files: `internal/plugin/interface.go`, `internal/plugin/plugin.proto`, `internal/plugin/*.pb.go`, `internal/plugin/reference/main.go`, `internal/plugin/plugin_test.go`, `go.mod`, `go.sum`

- [x] 18. Plugin manager (scan, load, health check, restart)

  **What to do**:
  - Create `PluginManager` struct in `internal/plugin/manager.go`
  - Scan `~/.mesh/plugins/` at startup: find executable binaries, validate they are go-plugin binaries via `plugin --info`
  - Load plugin: start go-plugin client, connect gRPC, call PluginInfo, store in plugin registry
  - Health check: call PluginInfo every 30s via gRPC. Mark unhealthy if 3 consecutive failures
  - Restart on crash: detect client disconnect, spawn new process, reconnect. Max 3 retry attempts at 1s intervals
  - Track plugin state: `Loaded`, `Healthy`, `Unhealthy`, `Crashed`, `Removed`
  - On daemon shutdown: send SIGTERM to all plugin processes, wait 5s, then SIGKILL. Verify no orphaned processes
  - Write tests: scan directory finds plugin, load plugin starts process, health check detects unhealthy, restart after crash, shutdown kills processes
  - Test: `go test ./internal/plugin/` PASS

  **Must NOT do**:
  - Don't watch directory for changes — scan at startup only
  - Don't implement plugin download/install/update — that's future scope
  - Don't auto-load plugins not in Enabled list
  - Don't restart plugins indefinitely — 3 retries then mark unhealthy

  **Recommended Agent Profile**:
  > Category: `deep` — process lifecycle management with health monitoring and crash recovery
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (depends on T17)
  - **Parallel Group**: Wave 5
  - **Blocks**: T19, T20, T21
  - **Blocked By**: T17 (plugin interface)

  **References**:
  - `internal/plugin/interface.go` — MeshPlugin interface (from T17)
  - `internal/plugin/reference/main.go` — Reference plugin binary for testing
  - `internal/config/config.go:PluginConfig` — Plugin config (Dir, Enabled)
  - `internal/daemon/daemon.go:Stop()` — Graceful shutdown pattern (30s timeout, signal handling)

  **Acceptance Criteria** (TDD):
  - [ ] Test cases: manager scans directory and finds plugin, manager loads plugin and gets PluginInfo, health check detects crashed plugin, restart spawns new process, shutdown kills all processes
  - [ ] `go test ./internal/plugin/` → PASS

  **QA Scenarios**:
  ```
  Scenario: Plugin manager loads reference plugin from directory
    Tool: Bash (go test)
    Preconditions: Reference plugin binary in test directory
    Steps:
      1. Create PluginManager with test plugin dir
      2. Call manager.LoadAll()
      3. Check manager.ListPlugins()
    Expected Result: One plugin loaded, state "Healthy"
    Evidence: .sisyphus/evidence/task-18-manager-load.txt

  Scenario: Plugin manager detects crash and restarts
    Tool: Bash (go test)
    Preconditions: Loaded and healthy plugin
    Steps:
      1. Kill plugin process (SIGKILL)
      2. Wait for health check interval (mocked to 1s for test)
      3. Check plugin state
    Expected Result: Plugin restarted, state back to "Healthy" after retry
    Evidence: .sisyphus/evidence/task-18-crash-restart.txt
  ```

  **Commit**: YES
  - Message: `feat(plugin): implement plugin manager with health checks and crash recovery`
  - Files: `internal/plugin/manager.go`, `internal/plugin/manager_test.go`

- [x] 19. Add list_plugins + plugin_health MCP tools + wire plugins into daemon

  **What to do**:
  - Register `list_plugins` MCP tool: returns all loaded plugins with name, version, state, health status
  - Register `plugin_health` MCP tool: params `{plugin_name}`, returns detailed health info (last check, uptime, restart count)
  - Wire PluginManager into daemon.Start(): initialize manager, scan plugins dir, load enabled plugins, register plugin adapters with MultiAdapter
  - On daemon shutdown: stop all plugin processes via PluginManager.Shutdown()
  - Write tests: list_plugins returns loaded plugins, plugin_health returns health details, daemon starts with plugin manager
  - Test: `go test ./internal/mcp/ ./internal/daemon/` PASS

  **Must NOT do**:
  - Don't add plugin install/uninstall MCP tools — that's CLI (T24)
  - Don't add dynamic plugin load/unload at runtime

  **Recommended Agent Profile**:
  > Category: `quick` — two MCP tools + wiring existing plugin manager into daemon
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 5 (with T18)
  - **Blocks**: T22
  - **Blocked By**: T18 (plugin manager)

  **References**:
  - `internal/mcp/handlers.go:11-52` — MCP tool registration pattern
  - `internal/plugin/manager.go` — PluginManager interface (from T18)
  - `internal/daemon/daemon.go:82-125` — Daemon Start method (for wiring)

  **Acceptance Criteria** (TDD):
  - [ ] Test cases: list_plugins returns empty when no plugins, list_plugins returns reference plugin, plugin_health returns health details
  - [ ] `go test ./internal/mcp/` → PASS

  **QA Scenarios**:
  ```
  Scenario: list_plugins returns loaded plugins via MCP
    Tool: Bash (curl MCP)
    Preconditions: Daemon running with reference plugin loaded
    Steps:
      1. Send MCP tools/call: list_plugins {}
    Expected Result: result.plugins array contains {name: "reference", state: "Healthy"}
    Evidence: .sisyphus/evidence/task-19-list-plugins.json
  ```

  **Commit**: YES
  - Message: `feat(mcp): add list_plugins and plugin_health tools, wire plugins into daemon`
  - Files: `internal/mcp/handlers.go`, `internal/mcp/mcp_test.go`, `internal/daemon/daemon.go`

- [x] 20. S3 registry plugin (scaffold + push + pull + verify)

  **What to do**:
  - Create `internal/registry/` package with S3 registry plugin implementing `MeshPlugin` interface
  - Plugin scaffold: main.go with go-plugin server, plugin info, adapter (wraps S3 client)
  - S3 push: accept `io.Reader` (snapshot stream), upload to S3 bucket with multipart upload, set SHA-256 metadata on object. No full buffering — stream directly to S3.
  - S3 pull: download object from S3 bucket, return `io.ReadCloser` stream. Read SHA-256 metadata from object. No full buffering.
  - Verify: compute SHA-256 of downloaded stream, compare with metadata. Mismatch → error.
  - Config: read bucket, region, endpoint, credentials from `RegistryConfig` (from T4). Support env vars: `MESH_S3_BUCKET`, `MESH_S3_REGION`, `MESH_S3_ENDPOINT`, `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`
  - Local cache: snapshots on disk AND in S3 for portability
  - Handle errors: S3 unreachable, credentials missing, bucket not found, upload interrupted
  - Write tests: push returns object key, pull returns same content, verify passes on matching hash, verify fails on mismatch
  - Test: `go test ./internal/registry/` PASS (mock S3 or real if configured)

  **Must NOT do**:
  - Don't implement OCI registry protocol — S3 only
  - Don't implement snapshot versioning or lifecycle policies — document as user setup
  - Don't implement presigned URLs or access control — IAM handles that

  **Recommended Agent Profile**:
  > Category: `deep` — new plugin package with streaming, multipart uploads, and SHA-256 verification
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 6 (with T21)
  - **Blocks**: T22
  - **Blocked By**: T3 (AWS SDK validated), T18 (plugin manager)

  **References**:
  - `internal/plugin/interface.go` — MeshPlugin interface (from T17)
  - `internal/plugin/reference/main.go` — Reference plugin pattern
  - `internal/config/config.go:RegistryConfig` — Config schema (from T4)
  - `internal/snapshot/snapshot.go` — Snapshot stream source for push
  - https://aws.github.io/aws-sdk-go-v2/docs/service/s3/ — AWS SDK v2 S3 docs (streaming upload)

  **Acceptance Criteria** (TDD):
  - [ ] Test cases: push uploads object with metadata, pull returns same content, verify passes, verify fails on hash mismatch, missing credentials returns error
  - [ ] `go test ./internal/registry/` → PASS
  - [ ] Plugin binary builds: `go build -o s3-registry ./internal/registry/cmd/`

  **QA Scenarios**:
  ```
  Scenario: Push snapshot to S3 and pull back with verification
    Tool: Bash (go test with real or mock S3)
    Preconditions: S3 credentials configured or mock S3 server
    Steps:
      1. Create test snapshot (known content + SHA-256)
      2. Push to S3 registry plugin
      3. Pull from S3 registry plugin
      4. Compute SHA-256 of pulled content
      5. Compare with original SHA-256
    Expected Result: Pulled content matches original, SHA-256 verification passes
    Failure Indicators: Upload error, SHA-256 mismatch, download timeout
    Evidence: .sisyphus/evidence/task-20-s3-push-pull.txt

  Scenario: S3 registry plugin handles unreachable endpoint
    Tool: Bash (go test)
    Preconditions: Invalid S3 endpoint configured
    Steps:
      1. Configure registry with nonexistent endpoint
      2. Attempt push
    Expected Result: Error returned, no panic, no hung connection
    Evidence: .sisyphus/evidence/task-20-s3-unreachable.txt
  ```

  **Commit**: YES
  - Message: `feat(registry): implement S3 registry plugin with streaming push/pull/verify`
  - Files: `internal/registry/plugin.go`, `internal/registry/push.go`, `internal/registry/pull.go`, `internal/registry/cmd/main.go`, `internal/registry/registry_test.go`, `go.mod`, `go.sum`

- [x] 21. Nomad adapter plugin (create/start/stop/export/import)

  **What to do**:
  - Create `internal/nomad/` package with Nomad adapter implementing both `SubstrateAdapter` and `MeshPlugin` interfaces
  - Plugin scaffold: main.go with go-plugin server
  - Create: submit Nomad job with Docker driver, pass image, env, workdir as job spec. Store job ID as instance ID.
  - Start/Stop: use Nomad job update (count=1/count=0). Map Nomad allocation states to BodyState.
  - GetStatus: query Nomad allocation for job, extract state, uptime, resource usage
  - Exec: use Nomad alloc exec API. Fall back to sidecar task if alloc exec unavailable.
  - ExportFilesystem: use Nomad alloc fs API to read container filesystem as tar stream. Fall back to sidecar task (one-shot container running tar+zstd on volume).
  - ImportFilesystem: use Nomad alloc fs API to write tar stream to container filesystem.
  - Config: read Nomad address, token from `NomadConfig` (from T4). Token from env var: `NOMAD_TOKEN`
  - Handle errors: Nomad unreachable, job submission failed, allocation stuck in pending, alloc fs not available
  - Write tests: create job returns job ID, start sets count=1, stop sets count=0, exec returns output
  - Test: `go test ./internal/nomad/` PASS (mock Nomad API or real if configured)

  **Must NOT do**:
  - Don't create Nomad namespace or ACL policies — user pre-configures
  - Don't implement Nomad job scheduling strategy — use default scheduler
  - Don't add Tailscale or networking setup — user configures
  - Don't use sidecar approach unless alloc fs is confirmed unavailable

  **Recommended Agent Profile**:
  > Category: `deep` — new adapter implementing full SubstrateAdapter interface with Nomad API
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 6 (with T20)
  - **Blocks**: T22
  - **Blocked By**: T2 (nomad/api validated), T18 (plugin manager)

  **References**:
  - `internal/adapter/adapter.go` — SubstrateAdapter interface (all methods to implement)
  - `internal/docker/adapter.go` — Docker adapter (pattern reference for Nomad adapter)
  - `internal/plugin/interface.go` — MeshPlugin interface (from T17)
  - `internal/config/config.go:NomadConfig` — Config schema (from T4)
  - https://developer.hashicorp.com/nomad/api-docs — Nomad HTTP API docs (jobs, allocations, alloc fs)

  **Acceptance Criteria** (TDD):
  - [ ] Test cases: create submits job with correct spec, start updates job count, stop updates job count, GetStatus maps allocation state, exec returns command output
  - [ ] `go test ./internal/nomad/` → PASS
  - [ ] Plugin binary builds: `go build -o nomad-adapter ./internal/nomad/cmd/`

  **QA Scenarios**:
  ```
  Scenario: Nomad adapter creates and starts a body job
    Tool: Bash (go test with mock Nomad API)
    Preconditions: Mock Nomad API server running
    Steps:
      1. Create body with Nomad adapter
      2. Call Start() on created body
      3. Call GetStatus()
    Expected Result: Body state == "running", job exists in Nomad
    Evidence: .sisyphus/evidence/task-21-nomad-create.txt

  Scenario: Nomad adapter export uses alloc fs
    Tool: Bash (go test)
    Preconditions: Running Nomad job with files
    Steps:
      1. Call ExportFilesystem() on running allocation
      2. Read tar stream
    Expected Result: Tar stream contains expected files
    Evidence: .sisyphus/evidence/task-21-nomad-export.txt
  ```

  **Commit**: YES
  - Message: `feat(nomad): implement Nomad adapter plugin with create/start/stop/export/import`
  - Files: `internal/nomad/adapter.go`, `internal/nomad/cmd/main.go`, `internal/nomad/adapter_test.go`, `go.mod`, `go.sum`

- [x] 22. Cross-machine Docker→Docker migration via S3 registry

  **What to do**:
  - Extend migration step 3 (transfer) for cross-machine: instead of local file copy, push snapshot to S3 registry, then pull from S3 on target
  - Add migration config: `source_registry` and `target_registry` fields (for cases where source/target use different registries)
  - Wire S3 registry plugin into migration coordinator: migration uses registry plugin for step 3
  - Handle cross-machine edge cases: network interruption during S3 transfer → retry upload/download; target machine unreachable → rollback
  - Verify after cross-machine transfer: compare SHA-256 from source snapshot with SHA-256 after S3 pull
  - Write integration test: Docker→Docker migration with S3 registry as transfer medium
  - Test: `go test -tags=integration ./integration/` PASS (requires real S3 or mock)

  **Must NOT do**:
  - Don't implement Docker→Nomad migration — scoped to Docker→Docker
  - Don't implement automatic registry selection — user specifies
  - Don't add SSH setup for cross-machine — assume network connectivity

  **Recommended Agent Profile**:
  > Category: `deep` — extending migration with S3 integration, network error handling
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 7
  - **Blocks**: T23, T24, T25
  - **Blocked By**: T16 (migration complete), T20 (S3 registry), T19 (plugins wired)

  **References**:
  - `internal/body/migration.go` — Migration coordinator step 3 (transfer)
  - `internal/registry/plugin.go` — S3 registry plugin (from T20)
  - `internal/plugin/manager.go` — Plugin manager (for loading registry plugin)

  **Acceptance Criteria** (TDD):
  - [ ] Test cases: cross-machine migration uses S3 for transfer, SHA-256 verified at both ends, retry after network failure, rollback if target unreachable
  - [ ] `go test -tags=integration ./integration/` → PASS
  - [ ] Integration test: create body on Docker → snapshot → push to S3 → pull from S3 → create new body → verify files match

  **QA Scenarios**:
  ```
  Scenario: Docker→Docker migration via S3 registry
    Tool: Bash (integration test)
    Preconditions: S3 registry configured, two Docker containers
    Steps:
      1. Create body "source" with test files
      2. Create snapshot
      3. Push snapshot to S3
      4. Pull snapshot from S3 to "target"
      5. Verify target files match source
    Expected Result: Target container has identical files to source
    Evidence: .sisyphus/evidence/task-22-cross-machine-migration.txt
  ```

  **Commit**: YES
  - Message: `feat(migration): implement cross-machine Docker migration via S3 registry`
  - Files: `internal/body/migration.go`, `integration/integration_test.go`

- [x] 23. Goreleaser multi-platform + CI/CD updates

  **What to do**:
  - Update `.goreleaser.yml`: add `darwin/arm64` target. Keep `linux/amd64`. Add `darwin/amd64` for broader macOS support. Format: binary archives.
  - Add `brews` section to goreleaser for automatic Homebrew formula generation
  - Update `.github/workflows/ci.yml`: add macOS runner for build verification. Add integration test job with Docker (ubuntu-latest with docker). Add goreleaser dry-run check on PR.
  - Test goreleaser locally: `goreleaser release --snapshot --clean` — verify both macOS and Linux binaries produced
  - Test: `goreleaser check` passes. CI passes on both ubuntu and macOS.

  **Must NOT do**:
  - Don't add Windows targets — scope is macOS + Linux
  - Don't add signing/notarization — stretch goal
  - Don't configure auto-publish to Homebrew — manual for v1

  **Recommended Agent Profile**:
  > Category: `quick` — config file updates, CI workflow modifications
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 8 (with T24, T25)
  - **Blocks**: None (final wave)
  - **Blocked By**: T13 (CLI complete), T22 (all features built)

  **References**:
  - `.goreleaser.yml` — Existing goreleaser config (linux/amd64 only)
  - `.github/workflows/ci.yml` — Existing CI workflow
  - https://goreleaser.com/customization/builds/ — Multi-platform build config
  - https://goreleaser.com/customization/brews/ — Homebrew formula generation

  **Acceptance Criteria**:
  - [ ] `.goreleaser.yml` produces darwin/arm64, darwin/amd64, linux/amd64 binaries
  - [ ] `goreleaser check` passes with no warnings
  - [ ] CI passes on push with all platforms verified

  **QA Scenarios**:
  ```
  Scenario: Goreleaser produces multi-platform binaries
    Tool: Bash
    Preconditions: All code built and tests passing
    Steps:
      1. goreleaser release --snapshot --clean
      2. ls dist/
    Expected Result: Binaries for darwin_arm64, darwin_amd64, linux_amd64 present
    Failure Indicators: Missing platform, build errors, goreleaser config error
    Evidence: .sisyphus/evidence/task-23-goreleaser.txt
  ```

  **Commit**: YES
  - Message: `ci: add multi-platform goreleaser config and CI updates`
  - Files: `.goreleaser.yml`, `.github/workflows/ci.yml`

- [x] 24. Shell install script + Homebrew formula

  **What to do**:
  - Create `scripts/install.sh`: detects OS/arch, downloads latest release from GitHub, verifies SHA-256 checksum, installs to `/usr/local/bin/mesh`, creates `~/.mesh/` directory, runs `mesh init`
  - Create `Formula/mesh.rb`: Homebrew formula pointing to GitHub releases. Defines install method, test block (`mesh status`), dependencies (none beyond Go for build-from-source)
  - Shell script handles: curl/wget fallback, sudo detection, PATH checking, cleanup of temp files
  - Test: run install.sh on macOS, verify mesh binary installed and functional
  - Test: `brew install --build-from-source ./Formula/mesh.rb` on macOS

  **Must NOT do**:
  - Don't add package manager repos (apt, yum) — just Homebrew + shell
  - Don't add GPG signing to install script — stretch goal
  - Don't auto-start daemon after install

  **Recommended Agent Profile**:
  > Category: `writing` — shell script + Homebrew formula, documentation-heavy
  - **Category**: `writing`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 8 (with T23, T25)
  - **Blocks**: None
  - **Blocked By**: T13 (CLI complete), T22 (all features built)

  **References**:
  - `cmd/mesh/main.go` — CLI commands for init, status
  - `.goreleaser.yml` — Release asset names and checksums
  - https://docs.brew.sh/Formula-Cookbook — Homebrew formula reference

  **Acceptance Criteria**:
  - [ ] `scripts/install.sh` installs mesh from GitHub releases
  - [ ] `Formula/mesh.rb` passes `brew audit` and `brew test`
  - [ ] After install: `mesh status` or `mesh init` works without errors

  **QA Scenarios**:
  ```
  Scenario: Shell install script downloads and installs mesh
    Tool: Bash
    Preconditions: Clean environment, no mesh binary
    Steps:
      1. bash scripts/install.sh
      2. which mesh
      3. mesh init
    Expected Result: mesh installed at /usr/local/bin/mesh, mesh init succeeds
    Evidence: .sisyphus/evidence/task-24-install.txt

  Scenario: Homebrew formula installs mesh
    Tool: Bash
    Preconditions: macOS with Homebrew
    Steps:
      1. brew install --build-from-source ./Formula/mesh.rb
      2. mesh --version
    Expected Result: mesh version printed, exit code 0
    Evidence: .sisyphus/evidence/task-24-brew.txt
  ```

  **Commit**: YES
  - Message: `feat(bootstrap): add shell install script and Homebrew formula`
  - Files: `scripts/install.sh`, `Formula/mesh.rb`

- [x] 25. Integration test suite (all substrates + migration + plugins)

  **What to do**:
  - Expand `integration/integration_test.go` to cover new v1 capabilities
  - Add test: daemon start → create Docker body → exec command → create snapshot → restore → verify files
  - Add test: daemon start → load reference plugin → list_plugins → verify plugin healthy
  - Add test: same-machine Docker→Docker migration (all 7 steps)
  - Add test: cross-machine Docker→Docker migration via S3 registry (if S3 configured)
  - Add test: daemon crash recovery → restart → reconcile detects orphaned containers
  - Add test: concurrent body create/start/stop (no race conditions)
  - Add test: plugin crash → manager restarts → plugin healthy again
  - Ensure all integration tests pass with `go test -tags=integration -race ./integration/`
  - Test: `go test -tags=integration -race ./integration/` → PASS

  **Must NOT do**:
  - Don't add Nomad integration tests unless Nomad cluster is available (tag with `nomad` build tag)
  - Don't add S3 integration tests unless S3 credentials configured (tag with `s3` build tag)
  - Don't remove existing integration tests — extend them

  **Recommended Agent Profile**:
  > Category: `deep` — comprehensive integration testing across all subsystems
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 8 (with T23, T24)
  - **Blocks**: None (final wave)
  - **Blocked By**: T22 (cross-machine migration), all previous tasks

  **References**:
  - `integration/integration_test.go:1-672` — Existing integration test patterns
  - `integration/mock_adapter.go:1-82` — Mock adapter for testing
  - All `internal/*/` packages — Implementation to test

  **Acceptance Criteria**:
  - [ ] All existing integration tests still pass
  - [ ] New integration tests cover: daemon lifecycle, body lifecycle, migration, plugin mgmt, crash recovery
  - [ ] `go test -tags=integration -race ./integration/` → PASS with 0 race conditions
  - [ ] Test coverage report: ≥80% of new code covered

  **QA Scenarios**:
  ```
  Scenario: Full integration test suite passes
    Tool: Bash
    Preconditions: Docker running, all plugins built
    Steps:
      1. go test -tags=integration -race -v ./integration/
    Expected Result: All tests PASS, no race conditions, clean exit
    Failure Indicators: Test failures, race detector warnings, timeout
    Evidence: .sisyphus/evidence/task-25-integration.txt
  ```

  **Commit**: YES
  - Message: `test(integration): add comprehensive v1 integration test suite`
  - Files: `integration/integration_test.go`



## Final Verification Wave (MANDATORY — after ALL implementation tasks)

> 4 review agents run in PARALLEL. ALL must APPROVE. Present consolidated results to user and get explicit "okay" before completing.

- [x] F1. **Plan Compliance Audit** — `oracle`
  - Must Have [11/11] | Must NOT Have [0/0] | Tasks [25/25] | Evidence [25/25] | VERDICT: APPROVE (after S3RegistryPlugin.Verify fix)
  Read the plan end-to-end. For each "Must Have": verify implementation exists (read file, curl endpoint, run command). For each "Must NOT Have": search codebase for forbidden patterns — reject with file:line if found. Check evidence files exist in `.sisyphus/evidence/`. Compare deliverables against plan.
  Output: `Must Have [N/N] | Must NOT Have [N/N] | Tasks [N/N] | VERDICT: APPROVE/REJECT`

- [x] F2. **Code Quality Review** — `unspecified-high`
  - Build [PASS] | Lint [PASS] | Tests [14 pass/0 fail] | Race [CLEAN] | VERDICT: PASS
  Run `go vet ./...` + `go test -race ./...` + `golangci-lint run`. Review all changed files for: `interface{}` where typed should be used, empty catch blocks, unreachable code, race conditions. Check AI slop: excessive comments, over-abstraction, generic names (data/result/item/temp).
  Output: `Build [PASS/FAIL] | Lint [PASS/FAIL] | Tests [N pass/N fail] | Race [CLEAN/ISSUES] | VERDICT`

- [x] F3. **Real Manual QA** — `unspecified-high`
  - Scenarios [16/16 pass] | Integration [16/16] | Edge Cases [3 tested] | VERDICT: PASS

- [x] F4. **Scope Fidelity Check** — `deep`
  - Tasks [25/25 compliant] | Contamination [CLEAN] | Unaccounted [CLEAN] | VERDICT: PASS
  - All tasks implemented within scope. No forbidden patterns found. No unaccounted file changes.

---

## Commit Strategy

- Commits grouped by wave. Each wave committed after all tasks in that wave pass tests.
- Commit format: `feat(scope): description`
- Pre-commit: `go vet ./... && go test -race ./...`

---

## Success Criteria

### Verification Commands
```bash
go build -o mesh ./cmd/mesh/              # Expected: binary built, no errors
go vet ./...                              # Expected: no output
go test -race ./...                       # Expected: PASS all tests
go test -tags=integration ./integration/  # Expected: PASS all 8+ integration tests
./mesh serve &                            # Expected: daemon starts with PID
./mesh status                             # Expected: daemon: running
curl http://localhost:<port>/healthz      # Expected: {"status":"ok","bodies":N}
```

### Final Checklist
- [x] All "Must Have" present (Docker, Nomad, Migration, S3, Plugins, 16 MCP, CLI, Bootstrap)
- [x] All "Must NOT Have" absent (no K8s, no auto-scheduler, no web UI, no live migration)
- [x] All tests pass with race detector
- [x] Integration tests pass with real Docker
- [x] Daemon starts, reconciles, shuts down cleanly
- [x] Cold migration completes all 7 steps
- [x] S3 registry push/pull/verify passes
- [x] Plugin manager handles crash and restart
- [x] Pre-built binaries for macOS arm64 + Linux amd64 exist
