# Mesh Go Daemon — Architecture Audit

> Auto-generated from sub-agent exploration (bg_20d7536c) + direct code review.
> Date: 2026-05-08
> No code changes — findings only.

---

## 1. Entrypoints

Two binaries from same codebase:

| Binary | File | Commands | Config Format |
|--------|------|----------|---------------|
| `mesh-daemon` | `cmd/mesh-daemon/main.go` | `serve`, `version` | YAML |
| `mesh` | `cmd/mesh/main.go` | `snapshot`, `restore`, `list`, `inspect`, `prune`, `init`, `serve`, `stop`, `status` | TOML (v0 commands), YAML (v1 commands) |

**Finding:** Both binaries instantiate `daemon.New(cfg)` and call `d.Start(ctx)`. The CLI binary embeds daemon startup — no client/server separation at the command layer.

---

## 2. Internal Package Map

| Package | Purpose | Files |
|---------|---------|-------|
| `internal/daemon/` | Core daemon process, startup, shutdown, reconcile | `daemon.go` (427 lines) |
| `internal/body/` | Body state machine (8 states), lifecycle manager, migration coordinator | `body.go`, `manager.go` (310 lines), `migration.go` |
| `internal/orchestrator/` | OrchestratorAdapter interface, registry, capability extensions | `orchestrator.go` (157 lines), `extensions.go` |
| `internal/nomad/` | Nomad adapter — implements OrchestratorAdapter + 6 extension interfaces | `adapter.go` (463 lines) |
| `internal/provisioner/` | ProvisionerAdapter interface, registry | `provisioner.go` (136 lines) |
| `internal/store/` | SQLite store with WAL mode, body/snapshot/migration CRUD | `store.go` (524 lines) |
| `internal/api/` | REST API: healthz, bodies CRUD, nodes list | `router.go`, `bodies.go`, `nodes.go`, `middleware.go`, `types.go` |
| `internal/mcp/` | MCP server: 16 tools over stdio JSON-RPC | `server.go`, `handlers.go` (618 lines) |
| `internal/config/` | YAML config parsing for daemon | `config.go` (206 lines) |
| `internal/config-toml/` | TOML config parsing for v0 CLI | `config.go` (173 lines) |
| `internal/plugin/` | Plugin manager using go-plugin + gRPC | `manager.go`, `interface.go` |
| `internal/snapshot/` | Snapshot pipeline: docker export → zstd → sha256 | `snapshot.go` |
| `internal/restore/` | Snapshot restore | `restore.go` |
| `internal/manifest/` | Snapshot manifest | `manifest.go` |
| `internal/registry/` | S3 registry for snapshot transport | `push.go`, `pull.go`, `plugin.go` |
| `internal/ingress/` | Ingress adapter (noop only) | `noop.go`, `ingress.go` |

---

## 3. Daemon Core (`internal/daemon/daemon.go`)

**Daemon struct — 50 lines:**
```go
type Daemon struct {
    cfg           *config.Config
    store         *store.Store
    orchRegistry  *orchestrator.Registry
    provRegistry  *provisioner.Registry    // ← LEAK: VM provisioning belongs in separate project
    bodyMgr       *body.BodyManager
    pluginMgr     *plugin.PluginManager
    mcpServer     interface{ Stop(context.Context) error }
    httpServer    *http.Server
    // ...
}
```

**Startup sequence (`Start()`, lines 101-192):**
1. PID conflict check
2. Open SQLite store (WAL mode)
3. Register orchestrator adapters — **hardcoded Nomad switch** (lines 115-128)
4. Register provisioner adapters — **empty by default** (lines 134-139)
5. Create BodyManager with primary orchestrator
6. Start plugin manager (scan, load, health checks)
7. **Reconcile** — scan store bodies, verify against orchestrator, transition stale states
8. Write PID file
9. **Fail-closed auth** — refuses start if `auth_token` empty (line 167)
10. Start HTTP API server on `listen_addr`
11. Block on signals

**Reconcile logic (lines 228-302):**
- For each persisted body, validates container existence against orchestrator
- Running/Starting/Stopping bodies with missing containers → Error
- Error bodies with running containers → Running (recovery)
- Migrating bodies with no active migration record → Error

---

## 4. Config & Auth

**Two parallel config systems:**

| Package | Format | Default Path | Used By | Key Fields |
|---------|--------|-------------|---------|------------|
| `internal/config` | YAML | `~/.mesh/config.yaml` | Daemon, `mesh serve/init/stop/status` | `daemon.auth_token`, `daemon.listen_addr`, `daemon.pid_file`, `daemon.log_level`, `store.path`, `orchestrators`, `provisioners`, `bodies[]`, `registry` (S3), `plugin` |
| `internal/config-toml` | TOML | `~/.mesh/config.toml` | `mesh snapshot/restore/list/inspect/prune` | `machines[]` (SSH targets), `agents[]` (workdir, hooks) |

**Auth:** Single bearer token. `internal/api/middleware.go:8-26` — `BearerAuth()` checks `Authorization: Bearer <token>` against `cfg.Daemon.AuthToken`. No user model, no RBAC, no TLS config. Auth token is a shared secret.

**Fail-closed:** Daemon refuses to start if `auth_token` is empty (line 167: `"daemon: auth_token is not set in config — refusing to start with unprotected API"`).

---

## 5. Body Management — 8-State Machine

**States** (`internal/orchestrator/orchestrator.go:20-28`):
```
Created → Starting → Running → Stopping → Stopped → Destroyed (terminal)
   ↑___________↓      ↑________↓
   Error ←─────────────┘  ← recoverable
   ↑
   Migrating → Running or Error
```

**Valid transition rules** (`internal/body/body.go:22-32`):
```go
StateCreated:   {StateStarting, StateError}
StateStarting:  {StateRunning, StateError}
StateRunning:   {StateStopping, StateMigrating, StateError}
StateStopping:  {StateStopped, StateError}
StateStopped:   {StateStarting, StateDestroyed}
StateError:     {StateStarting, StateDestroyed, StateMigrating}
StateMigrating: {StateRunning, StateError}
StateDestroyed: {}
```

**Create() flow** (`manager.go:50-96`):
1. UUID generation
2. `orch.ScheduleBody(spec)` → gets handle (Nomad job ID)
3. Store insert: state=Created, substrate="local", instanceID=handle
4. Transition Starting → `orch.StartBody()` → Transition Running

**Boundary:** BodyManager never talks to provisioners directly. Provisioning is injected only in migration.

---

## 6. Orchestrator Adapter (`internal/orchestrator/`)

**Interface:**
```go
type OrchestratorAdapter interface {
    ScheduleBody(ctx context.Context, spec BodySpec) (Handle, error)
    StartBody(ctx context.Context, id Handle) error
    StopBody(ctx context.Context, id Handle) error
    DestroyBody(ctx context.Context, id Handle) error
    GetBodyStatus(ctx context.Context, id Handle) (BodyStatus, error)
    Name() string
    IsHealthy(ctx context.Context) bool
}
```

**Capability extensions** (`extensions.go`): `Exporter`, `Importer`, `Inspector`, `Executor`, `NodeLister`, `AllocQuerier`

---

## 7. Nomad Adapter (`internal/nomad/adapter.go`)

Implements `OrchestratorAdapter` + all 6 extension interfaces (463 lines).

- `ScheduleBody`: Creates Nomad job with Docker driver, count=0
- `StartBody`: Scales to count=1
- `StopBody`: Scales to count=0
- `DestroyBody`: Deregisters job
- `Exec`: Uses Nomad alloc exec API
- `ExportFilesystem`: Reads `/alloc/data/body.tar` via alloc FS API
- `ImportFilesystem`: Writes via alloc FS write API
- `ListNodes`: Lists Nomad clients with capacity

**Only built-in orchestrator.** No Docker-native adapter exists. The daemon hardcodes Nomad initialization at startup.

---

## 8. REST API (`internal/api/`)

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `GET` | `/healthz` | None | Health check, version, node/body counts |
| `GET` | `/api/v1/bodies` | Bearer | List all bodies |
| `POST` | `/api/v1/bodies` | Bearer | Create body |
| `GET` | `/api/v1/bodies/{id}` | Bearer | Get body by ID |
| `POST` | `/api/v1/bodies/{id}/stop` | Bearer | Stop body |
| `POST` | `/api/v1/bodies/{id}/start` | Bearer | Start body |
| `DELETE` | `/api/v1/bodies/{id}` | Bearer | Destroy body |
| `GET` | `/api/v1/nodes` | Bearer | List orchestrator nodes |

**Error format:**
```json
{"error": {"code": "body_not_found", "message": "...", "status": 404}}
```

**Error codes:** `unauthorized`, `body_not_found`, `node_not_found`, `body_conflict`, `bad_request`, `nomad_unreachable`, `resource_exhausted`, `internal`.

**Finding:** Health endpoint uses `nomad_connected` field name — substrate-specific naming in generic API.

---

## 9. MCP Server (`internal/mcp/`)

Transport: stdio JSON-RPC (line-delimited). 16 tools registered:

| Tool | Description |
|------|-------------|
| `ping` | Health check |
| `list_bodies` | List all managed bodies |
| `get_body` | Get body details by ID |
| `get_snapshot` | Get snapshot details by ID |
| `create_body` | Create and start a new body |
| `delete_body` | Destroy a stopped or errored body |
| `migrate_body` | Migrate body to different substrate |
| `execute_command` | Exec command inside a body |
| `create_snapshot` | Create filesystem snapshot |
| `list_snapshots` | List snapshots, optional body_id filter |
| `restore_body` | Restore body from snapshot |
| `start_body` | Start stopped body |
| `stop_body` | Stop running body |
| `get_body_logs` | Get logs from running body |
| `get_body_status` | Get runtime status (state, uptime, memory, cpu) |
| `list_plugins` | List loaded plugins |
| `plugin_health` | Get plugin health |

---

## 10. Plugin System (`internal/plugin/`)

Uses `go-plugin` + gRPC + protobuf. Substrate adapters loaded at runtime.

**Critical finding:** Plugin gRPC transport is **disabled**:
```go
func (s *stubPlugin) GRPCClient(...) (interface{}, error) {
    return nil, fmt.Errorf("plugin loading disabled: gRPC transport removed, redesign pending")
}
```
The plugin manager scans and loads plugins but cannot communicate with them. This makes D6 ("Provider integrations are plugins") currently unachievable.

---

## 11. Migration Coordinator (`internal/body/migration.go`)

7-step cold migration, crash-resumable via store:

| Step | Name | Action |
|------|------|--------|
| 1 | Export | Export filesystem from source container |
| 2 | Provision | Create target container on new substrate |
| 3 | Transfer | Copy snapshot (same-machine: direct; cross-machine: S3 registry) |
| 4 | Import | Idempotent checkpoint |
| 5 | Verify | `GetBodyStatus` + `Exec("echo ok")` + `Exec("ls /")` |
| 6 | Switch | Update instance_id, stop/destroy source |
| 7 | Cleanup | Remove snapshot file, delete migration record |

**Finding:** Migration hardcodes VM provisioning details (image name, memory, CPU, region) at `migration.go:317-322`. These are substrate-specific and should be configurable.

---

## 12. Store Schema (`internal/store/store.go`)

SQLite with WAL mode (CGO-free via `modernc.org/sqlite`). 4 tables:

```sql
bodies (id, name, state, spec_json, substrate, instance_id, created_at, updated_at)
snapshots (id, body_id, manifest_json, storage_path, size_bytes, created_at)
migrations (id, body_id, target_substrate, current_step, snapshot_id, started_at, error)
config (key, value)
```

**Indexes:** `idx_snapshots_body_id`, `idx_migrations_body_id`

---

## 13. Key Architecture Issues

| # | Issue | Severity | Location |
|---|-------|----------|----------|
| 1 | **Plugin system broken** — gRPC transport disabled, D6 unachievable | High | `internal/plugin/manager.go` |
| 2 | **Nomad hardcoded in daemon startup** — no factory pattern, switch-case in daemon.go | High | `internal/daemon/daemon.go:115-128` |
| 3 | **Dual config systems** — YAML for daemon, TOML for v0 CLI, no unified model | Medium | `internal/config/`, `internal/config-toml/` |
| 4 | **Migration leaks provisioning details** — hardcoded VM specs in body lifecycle code | Medium | `internal/body/migration.go:317-322` |
| 5 | **Provisioner adapter in core** — daemon has ProvisionerRegistry but shouldn't provision VMs | Medium | `internal/provisioner/`, `daemon.go:57` |
| 6 | **Health endpoint is substrate-specific** — `nomad_connected` field in generic API | Low | `internal/api/healthz.go:39-40` |
| 7 | **S3 registry in core** — per D6 should be plugin, not core dependency | Low | `internal/registry/` |
| 8 | **CLI embeds daemon** — `mesh serve` and `mesh-daemon serve` do the same thing | Low | `cmd/mesh/main.go` |
| 9 | **No Docker-native adapter** — only Nomad, can't run without a Nomad cluster | High | N/A — missing entirely |
| 10 | **Noop orchestrator on startup failure** — silent degradation instead of fast failure | Low | `daemon.go:403-427` |

---

## 14. Dependencies (`go.mod`)

| Dependency | Purpose | Coupling |
|------------|---------|----------|
| `hashicorp/nomad/api` | Only built-in orchestrator | **HIGH** |
| `aws/aws-sdk-go-v2/service/s3` | S3 registry in core | **MEDIUM** |
| `hashicorp/go-plugin` + gRPC | Plugin system (broken) | **MEDIUM** |
| `modernc.org/sqlite` | CGO-free SQLite | **LOW** |
| `klauspost/compress/zstd` | Snapshot compression | **LOW** |
| `BurntSushi/toml` | v0 config format | **LOW** (legacy) |
| `gopkg.in/yaml.v3` | v1 config format | **LOW** |
| `spf13/cobra` | CLI framework | **LOW** |
| `google/uuid` | Body ID generation | **LOW** |

---

## 15. What Works Well

- **8-state body machine** — clean, well-tested, validated transitions, persisted atomically
- **Store schema** — simple, WAL mode, migration-aware, no ORM
- **MCP server** — 16 tools, proper JSON-RPC 2.0, stdio transport
- **REST API** — clean endpoints, proper error codes, bearer auth
- **Nomad adapter** — comprehensive, implements all extension interfaces
- **Reconcile** — startup recovery handles orphaned containers and stale states
- **Snapshot pipeline** — deterministic via sorted walk, zstd compression, SHA-256 verification
- **Fail-closed auth** — daemon refuses to start without auth token

---

## 16. What Needs Attention

1. Plugin gRPC transport must be restored for D6 compliance
2. Orchestrator initialization needs a registry-based factory, not a hardcoded switch
3. Config consolidation — pick one format, deprecate the other with a clear migration path
4. Remove or gate provisioner registry from daemon core
5. Add a Docker-native adapter (no Nomad dependency for single-machine use)
6. Fix health endpoint to use generic orchestrator names
7. Move S3 registry behind the plugin boundary
8. Separate CLI from daemon binary responsibilities
