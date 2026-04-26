# V1 Architecture Decisions

> **Session**: 2026-04-25
> **Purpose**: Capture all architecture decisions from the v0→v1 planning session. Next session reads this + updates discovery/ files, then generates implementation plan.
> **Read with**: `discovery/archive/SESSION-HANDOFF.md`, `discovery/design/SYSTEM.md`

---

## Six Architecture Decisions (Resolved)

### AD1: Single Binary, Direct Go Calls

`mesh serve` = one Go binary. MCP server + all core modules + plugin subprocess management in one process. Core modules communicate via Go interfaces (direct function calls). gRPC is used ONLY at the substrate adapter plugin boundary.

**Why**: C6 (core is tiny). One binary fits. gRPC between in-process modules is ceremony without benefit on a 2GB VM. Plugin isolation via go-plugin subprocess is the only boundary that matters — external, untrusted code gets process isolation. Trusted core shares fate.

**Rejected**: gRPC between all modules (protobuf definitions, serialization overhead, service discovery — ceremony for 4-5 in-process modules).

### AD2: Thin Plugin Veneer for v1

Substrate adapters in v1 implement a Go interface (`SubstrateAdapter`). Docker implements it in-process. No go-plugin, no gRPC subprocess in v1. When full plugin system arrives (v1.1), same adapter code becomes a gRPC plugin — wrap it in subprocess launcher, zero logic changes.

**Why**: v1 scope is "ship core loop." Building go-plugin + gRPC protocol before proving the core loop is the biggest module first with no visible value. The thin interface IS the plugin boundary, just without subprocess transport. Upgrade path: Go interface → gRPC wrapper around same code.

**Rejected**: Build full go-plugin/gRPC first (delays core loop by weeks). Also rejected: Docker embedded directly in Orchestration (violates adapter boundary).

### AD3: SQLite with WAL for Durable State

Single SQLite file (WAL mode) at `~/.mesh/state.db`. Stores: body registry, migration records, snapshot metadata, plugin state, operation log.

**Why**: Mesh v1 is a service, not a CLI tool. Services need durable state. Bodies survive crashes. Migrations persist across restarts. On 2GB VM: ~1MB disk, negligible memory, no daemon, ACID, queryable.

**Schema sketch**:
- `bodies`: id, name, state, spec_json, substrate, instance_id, created_at, updated_at
- `migration_records`: id, body_id, target_substrate, current_step, snapshot_ref, started_at
- `snapshots`: ref, body_id, manifest_json, storage_backend, created_at, size_bytes
- `config`: key, value

### AD4: Orchestration Owns Migration

Orchestration (not Interface) owns the 7-step cold migration sequence. Interface calls `Orchestration.BeginMigration(bodyId, target)` → gets migration ID → polls status. Orchestration persists MigrationRecord in SQLite after each step. On crash, resumes from persisted step.

**Why**: If Interface crashes mid-migration, nobody resumes. Orchestration already owns body state machine — migration is a lifecycle transition. Interface is a stateless translator (MCP ↔ core), not a durable coordinator.

**SYSTEM.md change**: "Data Flow: Migrate a Body" steps 2a-2g move from Interface to Orchestration.

### AD5: Extended Substrate Adapter Contract

6 required verbs + 4 optional capabilities:

**Required**: Create, Start, Stop, Destroy, GetStatus, Exec
**Optional** (declared via Capabilities()): ExportFilesystem, ImportFilesystem, Inspect, WatchEvents

**Why**: Persistence deep-dive found cross-module gap — 6-verb adapter can't support snapshot/restore (Export/Import missing). These are optional because not all substrates support them. Adapters declare capabilities at load time.

**Go interface**:
```go
type SubstrateAdapter interface {
    // Required
    Create(ctx context.Context, spec BodySpec) (Handle, error)
    Start(ctx context.Context, id string) error
    Stop(ctx context.Context, id string, opts StopOpts) error
    Destroy(ctx context.Context, id string) error
    GetStatus(ctx context.Context, id string) (BodyStatus, error)
    Exec(ctx context.Context, id string, cmd []string) (ExecResult, error)

    // Optional — check via Capabilities()
    ExportFilesystem(ctx context.Context, id string) (io.ReadCloser, error)
    ImportFilesystem(ctx context.Context, id string, tarball io.Reader, opts ImportOpts) error
    Inspect(ctx context.Context, id string) (ContainerMetadata, error)

    Capabilities() AdapterCapabilities
}
```

### AD6: Networking Deferred to v1.1

v1 ships without Tailscale/networking. Bodies run on substrate-native networking (Docker bridge for local). Networking added in v1.1 as self-contained module.

**Why**: Bodies work on local Docker without Tailscale. You lose cross-substrate connectivity but keep the full core loop. Networking is self-contained — no refactoring needed to add it later. Cuts v1 scope ~20%.

---

## v1 Scope

### IN v1 (Core Loop)

1. **Orchestration**: Body lifecycle (8 states), body registry (SQLite), migration coordinator with durable records, health checker, GC, startup reconciliation
2. **Persistence**: Snapshot engine (elevated to docker export/import), storage backend abstraction (local FS for v1), manifest with container metadata, GC
3. **MCP Interface**: Streamable HTTP server, core tools (create, list, inspect, stop, destroy, snapshot, restore, migrate, start), operation tracking, error mapping
4. **Docker Adapter**: Implements SubstrateAdapter. Local only.
5. **CLI**: `mesh init`, `mesh serve`, `mesh stop`, `mesh status`. Thin debugging surface.
6. **State**: SQLite with WAL.

### DEFERRED to v1.1

- Networking (Tailscale, identity, DNS)
- Full plugin system (go-plugin, gRPC, registry, auto-reload)
- Nomad adapter (fleet scheduling)
- Fleet node provisioning (boot scripts, cluster join)
- E2B/Fly/Daytona adapters
- Storage backend plugins (S3, R2)
- Plugin generation via Pulumi skill

### DEFERRED to v2

- Self-migration (Mesh rehosting from laptop to VM)
- Hot-swap plugins
- A4 merge workflow (clone → parent FS merge)
- Console/UI integration
- Agent-to-Agent protocol support (A2A)

### Explicitly OUT (Forever)

- Kubernetes (D3)
- Horizontal scaling / replicas (bodies are stateful, not pods)
- Web hosting / ingress tiers (D9 discarded)
- Telemetry / phone-home (C4)
- Central dependency (C4)

---

## Key Architecture Concepts (Discussed)

### Two Provisioning Levels

- **Level 1 — Node provisioning** (infrastructure): Create VM, install Docker+Nomad+Tailscale, join cluster. This is what previous Python impl did with Pulumi + boot scripts. **v1.1+ scope.**
- **Level 2 — Body provisioning** (application): Create container on existing node. This is what the substrate adapter does. **v1 scope.**
- v1 assumes nodes exist (Docker on laptop, or manually set up Nomad cluster).

### Bodies ≠ Pods

Agent bodies are stateful individuals. Two "replicas" of same agent diverge because each learns independently. Mesh does NOT do K8s-style replicas. Instead:
- **Clone**: snapshot parent → create N copies → each diverges independently
- **Burst** (A4): create clones on sandbox → collect results → destroy
- **Merge** (v2): copy clone's FS changes back to parent

### Docker vs Nomad Roles

- **Docker**: body runtime (universal, D7: body = container). Every substrate that runs containers uses Docker or Docker-compatible runtime.
- **Nomad**: fleet scheduler (one substrate option, D3). NOT core. Only used for fleet pool.
- Mesh talks to Docker directly (local) or via Nomad API (fleet) or via provider APIs (sandbox).

### Mesh Lifecycle

`mesh init` → `mesh serve` (starts daemon, runs forever) → MCP tools available → `mesh stop` (graceful shutdown). On macOS: launchd. On Linux: systemd user service.

---

## Repository Structure

```
mesh/
├── cmd/mesh/              # CLI: mesh init, mesh serve, mesh stop, mesh status
├── internal/
│   ├── interface/         # MCP server, tool definitions, request routing
│   ├── orchestration/     # Body state machine, migration coordinator, health checker
│   ├── persistence/       # Snapshot engine, storage backends
│   ├── adapter/           # SubstrateAdapter Go interface (plugin boundary)
│   ├── providers/
│   │   └── docker/        # Docker provider (implements SubstrateAdapter in-process)
│   ├── store/             # SQLite wrapper (body registry, migrations, snapshots)
│   ├── config/            # YAML config parsing
│   └── snapshot/          # tar+zstd pipeline (reused from v0, elevated to docker export)
├── proto/                 # gRPC definitions (future v1.1 go-plugin boundary)
├── discovery/             # Design docs (reference only, not compiled)
├── go.mod
├── go.sum
├── AGENTS.md
└── README.md
```

---

## Previous Implementation Reference

Path: `/Users/samanvayayagsen/project/rethink-paradigms/infa/mesh-workspace/oss`

Python v0.4.0. Reusable patterns (conceptual, not code):
- **UniversalCloudNode** → substrate adapter concept
- **Progressive activation tiers** → topology-aware feature activation
- **Boot script pipeline** (Jinja2 + StrictUndefined) → Go text/template + validation
- **Nomad HCL templates** → agent body deployment templates
- **Atomic snapshot metadata** → temp-file-rename pattern

---

## V0 Code Reuse Assessment

| V0 Package | Reuse | Notes |
|------------|-------|-------|
| `internal/snapshot/` tar+zstd | HIGH | Core streaming pipeline reusable. Switch from filesystem walk → docker export stream. |
| `internal/restore/` extraction | HIGH | zstd decompression + tar extraction reusable. Add docker import + metadata application. |
| `internal/manifest/` JSON sidecar | MEDIUM | Schema needs expansion (add CMD, ENV, WORKDIR, platform, body_id). |
| `internal/config/` TOML parsing | LOW | Need YAML, per-module config sections, different schema entirely. |
| `internal/clone/` scp/ssh | LOW | Replaced by migration flow (snapshot → provision → restore). |
| `internal/agent/` PID check | LOW | Replaced by body state machine + substrate adapter status. |
| `internal/transport/` SSH | LOW | May be useful for specific providers but not core. |
| `cmd/mesh/` Cobra CLI | MEDIUM | Keep as thin debugging surface (mesh init, mesh serve, mesh status). |

---

## Unresolved for Later

- Q1: Where does a body live when idle? (v1.1 scheduler)
- Q2: Registry strategy — where snapshots live? (v1.1 storage plugins)
- Q3: Scheduler — core or plugin? (v1.1 fleet)
- Q4: Bootstrap flow (partially resolved: `mesh init` → `mesh serve`)
- 29 deep-dive OQs (mostly have recommendations, not yet formally decided)
