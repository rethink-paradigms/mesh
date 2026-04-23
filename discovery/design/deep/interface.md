# Deep-Dive: Interface (MCP + Skills)

> Module: Interface
> Agent: deep
> Date: 2026-04-23
> Status: draft

## Contract (Detailed)

### Inputs

MCP tool invocations from AI agents (Claude Code, Cursor, Codex, etc.) via Streamable HTTP transport. Each invocation is a JSON-RPC 2.0 `tools/call` with tool name + structured arguments. Secondary input: CLI commands (thin debugging surface) calling same core logic. Tools are grouped into namespaces: `mesh.body.*`, `mesh.snapshot.*`, `mesh.provisioner.*`, `mesh.plugin.*`, `mesh.network.*`.

### Outputs

MCP tool results: structured JSON with body states, operation IDs, snapshot references. Errors as MCP `isError: true` responses with structured error codes. Long-running operations yield MCP progress notifications (`notifications/progress`) with operation tokens for polling.

### Guarantees (Invariants)

- **INV-1**: Every MCP tool call returns a response (result or error). Never silently drops.
- **INV-2**: Migration sequence is atomic at the Interface level — either completes all 7 steps or rolls back to the original form's last known-good state.
- **INV-3**: Error codes from internal modules (gRPC status) are never leaked raw to MCP callers. Interface always translates to human-readable error with actionable context.
- **INV-4**: Two concurrent MCP calls for the same body are serialized. Interface enforces per-body operation queuing.
- **INV-5**: Tool surface is stable — adding a new tool never breaks existing tools. Removing a tool requires a deprecation cycle.
- **INV-6**: Bootstrap (Q4) — first interaction with Mesh happens via CLI, not MCP. MCP is available only after Mesh is running.

### Assumptions

- **ASM-1**: Orchestration module provides body state machine with well-defined states and transitions. Interface does not duplicate this logic.
- **ASM-2**: All internal modules communicate via gRPC with structured error codes (from SYSTEM.md cross-cutting concerns).
- **ASM-3**: The MCP client (agent) is cooperative — it handles rate limits and retries on `RATE_LIMITED` errors.
- **ASM-4**: For local-only deployments (A5 persona), no authentication is required. Auth is only needed when Mesh is exposed over network.
- **ASM-5**: Skills are external to Interface — they are agent-side constructs that compose MCP tools. Interface does not load or manage skills.

## State Machine

### States

Interface itself is stateless. It tracks no body state — that's Orchestration's job. However, Interface manages **operation state** for long-running sequences (migration):

- `idle` — no in-flight operation for this body
- `executing` — a tool call is actively processing
- `migrating` — cold migration sequence in progress (steps a-g)
- `error` — operation failed, awaiting cleanup or retry

### Transitions

| from_state | trigger | to_state | side_effects |
|---|---|---|---|
| idle | any tool call for body | executing | acquire per-body lock |
| executing | tool completes | idle | release lock, return result |
| executing | `mesh.body.migrate` called | migrating | execute 7-step sequence |
| migrating | all 7 steps complete | idle | release lock, return result |
| migrating | any step fails | error | trigger rollback |
| error | rollback completes | idle | release lock, return error |
| error | rollback fails | error | log critical, surface to caller |

### Illegal Transitions

| from_state | rejected_trigger | error_type | recovery |
|---|---|---|---|
| executing | any tool call for same body | `OPERATION_IN_PROGRESS` | caller retries after polling |
| migrating | `mesh.body.stop`, `mesh.body.destroy`, `mesh.body.snapshot` | `OPERATION_IN_PROGRESS` | wait for migration to complete |
| migrating | `mesh.body.migrate` (duplicate) | `OPERATION_IN_PROGRESS` | already migrating |

## T1: Happy Path Traces

### Create and Run a Body

1. Agent calls `mesh.body.create({ name: "hermes-1", image: "ubuntu:22.04", resources: { cpu: 2, memory: 4096 }, substrate: "fleet-nomad" })`
2. Interface validates args (image format, resource limits against substrate capabilities from Plugin Infra)
3. Interface calls `Orchestration.BodyManager.Create(spec)` → gRPC
4. Orchestration calls `Provisioning.Provisioner.Provision(spec)` → gets substrate handle
5. Orchestration calls `Networking.Network.AssignIdentity(bodyId)` → gets tailnet IP
6. Orchestration returns `Body{ id, state: "running", handle, networkIdentity }` to Interface
7. Interface returns MCP tool result: `{ id: "b-abc123", name: "hermes-1", state: "running", ip: "100.64.0.5", substrate: "fleet-nomad" }`

### Snapshot a Body

1. Agent calls `mesh.body.snapshot({ body_id: "b-abc123" })`
2. Interface returns immediately with operation token: `{ operation_id: "op-snap-001", status: "started", progress_token: "tok-xyz" }`
3. Interface calls `Persistence.SnapshotEngine.Capture(bodyId)` → gRPC (async, streaming progress)
4. Persistence: pre-prune → docker export → zstd compress → StorageBackend.Put
5. Persistence streams progress via gRPC metadata → Interface emits MCP `notifications/progress`
6. Persistence returns `SnapshotRef` to Interface
7. Interface returns final MCP result: `{ snapshot_id: "snap-def456", body_id: "b-abc123", size_bytes: 2147483648, storage_uri: "s3://..." }`

### Cold Migration

1. Agent calls `mesh.body.migrate({ body_id: "b-abc123", target_substrate: "e2b-sandbox" })`
2. Interface acquires per-body lock, sets state → migrating
3. Step (a): `Orchestration.Stop(bodyId)` → body enters "Stopped"
4. Step (b): `Persistence.Capture(bodyId)` → SnapshotRef "snap-mig-789"
5. Step (c): `Provisioning.Provision(spec, "e2b-sandbox")` → new handle
6. Step (d): `Persistence.Restore(SnapshotRef, newBodyId)` → FS imported
7. Step (e): `Networking.AssignIdentity(newBodyId)` → same name, new IP
8. Step (f): `Orchestration.Start(newBodyId)` → body running on new substrate
9. Step (g): `Provisioning.Destroy(oldHandle)` → old substrate cleaned
10. Interface releases lock, returns: `{ body_id: "b-abc123", state: "running", substrate: "e2b-sandbox", ip: "100.64.0.9" }`

## T2: Failure Analysis

| External Call | Failure Mode | State Left | Recovery Path | Cleanup By |
|---|---|---|---|---|
| Orchestration.Create | timeout | no body created | Retry (idempotent for new body) | Interface returns `TIMEOUT` error |
| Orchestration.Create | INSUFFICIENT_RESOURCES | no body created | Return error, agent chooses different substrate | Interface |
| Persistence.Capture | export fails (disk full) | body still running, no snapshot | Return error, no cleanup needed | Interface |
| Persistence.Capture | compress fails mid-stream | partial snapshot in storage | Retry; storage backend handles partial cleanup | Persistence (garbage collect) |
| Provisioning.Provision (during migration step c) | AUTHENTICATION_ERROR | body stopped, snapshot exists, old form intact | Rollback: restart body on original substrate | Interface |
| Persistence.Restore (during migration step d) | import fails | new provisioned but empty container, old snapshot valid | Rollback: destroy new container, restart old | Interface |
| Networking.AssignIdentity (step e) | Tailscale down | body restored on new substrate but no network | Rollback: destroy new, restart old. Or: retry networking (non-destructive) | Interface |
| Provisioning.Destroy (step g) | fails | body running on new substrate, old form still exists | Log warning, retry destroy async. Body is functional. | Interface (async retry) |
| Plugin discovery/load | plugin binary missing | tool returns `PROVIDER_NOT_FOUND` | Agent installs plugin first via `mesh.plugin.install` | Interface |

### Critical Finding: Migration Rollback is Complex

The migration sequence has 7 steps. Failure at steps c-f creates a split-brain scenario: old form exists (stopped), new form partially exists. **Design change needed**: Interface must track migration state persistently (not just in-memory) so it can resume or rollback after a crash. Proposed: write a migration intent record to local state before starting, delete on completion. On Interface restart, check for incomplete migrations and either resume or rollback.

## T3: Concurrency Analysis

### Conflicting Operations

- **Two creates for same body name**: Low risk — body IDs are unique. But duplicate names could confuse agents. **Mitigation**: `mesh.body.create` rejects duplicate names with `NAME_ALREADY_EXISTS`.
- **Snapshot + destroy for same body**: Snapshot may be mid-capture when destroy starts. **Mitigation**: Per-body operation lock prevents this.
- **Two migrations for same body**: Second migration rejected with `OPERATION_IN_PROGRESS`.
- **Migrate + snapshot for same body**: Rejected — migration holds per-body lock.
- **List bodies during migration**: Safe — read-only, doesn't conflict.

### Proposed Locking

**Per-body operation lock** (body ID → operation mutex). One mutating operation per body at a time. Read operations (list, inspect, get-logs) are lock-free. Migration acquires the lock for the entire 7-step sequence. Lock is in-memory (Interface process). **Risk**: if Interface crashes during migration, lock is lost. Mitigated by migration intent record (T2 finding).

**No cross-body locking needed** — operations on different bodies are independent.

**Deadlock prevention**: Migration touches two bodies (old and new). Lock ordering: always lock by body ID alphabetically to prevent AB-BA deadlock. But migration only locks source body — new body doesn't exist yet until step c, so there's no concurrent lock on the destination.

## T4: Scale Analysis

### Bottlenecks

| Resource | Limit | Threshold | Mitigation |
|---|---|---|---|
| MCP connection concurrency | ~1000 concurrent tool calls (Go HTTP server) | 100+ agents using Mesh simultaneously | Streamable HTTP is stateless; horizontal scaling behind load balancer |
| Per-body operation queue | In-memory goroutine per body | 1000+ bodies with pending operations | Operation queue backed by embedded DB (bbolt/pebble) |
| Migration orchestration | Each migration = 7 sequential gRPC calls | 10+ simultaneous migrations | Migrations are I/O-bound (snapshot transfer), not CPU-bound. Limit concurrent migrations per substrate. |
| Tool schema size | MCP `tools/list` response grows with tool count | 30+ tools slows agent context | Namespace tools; progressive disclosure (agent starts with core tools, discovers more via `mesh.tools.list`) |
| Plugin subprocesses | Each loaded plugin = 1 OS process + gRPC connection | 10+ plugins on 2GB VM | Lazy-load plugins; unload after idle timeout. Core stays under ~50MB. |

### Key Scale Concern: Tool Surface Bloat

Every MCP tool definition consumes agent context window tokens. With 30+ tools, `tools/list` could be 5-10KB of JSON. **Design change**: Group tools into tiers:
- **Tier 1 (always loaded)**: `mesh.body.create`, `mesh.body.list`, `mesh.body.destroy`, `mesh.body.snapshot`, `mesh.body.migrate` — 5 tools.
- **Tier 2 (on-demand)**: `mesh.provisioner.list`, `mesh.plugin.install`, `mesh.network.*` — loaded when agent calls `mesh.tools.discover`.

This keeps initial tool surface small (~2KB) while making full surface available.

## T5: Edge Cases

### EC1: Snapshot of a body that's being written to

Snapshot (docker export) captures a point-in-time FS. If agent inside is actively writing, the snapshot may be inconsistent (partial file writes). **Expected**: Snapshot is crash-consistent, not transactionally consistent. **Mitigation**: Pre-snapshot hook sends SIGTERM to agent, waits for graceful stop, then exports. But this contradicts "snapshot a running body" intent. **Design question**: Should `mesh.body.snapshot` stop the agent first? Or accept crash-consistent snapshots? Agent personas suggest periodic snapshots should be crash-consistent (cheap), while migration snapshots should be clean (stop first).

### EC2: Migration fails at step g (destroy old form)

Old form still exists on original substrate. New form is running on target. Body is effectively duplicated. **Fix**: This is non-fatal. Log warning, add old form to "cleanup queue," retry destroy asynchronously. Agent gets success response because new form is functional.

### EC3: MCP client disconnects during long-running operation

Agent calls `mesh.body.snapshot`, disconnects (network issue, context window overflow). Operation continues on Mesh side. When agent reconnects, it has no operation token. **Fix**: Operations produce results that are queryable by body ID. `mesh.body.inspect(body_id)` returns last snapshot ref if operation completed. Operation tokens are best-effort, not required.

### EC4: Bootstrap chicken-and-egg (Q4)

MCP requires running Mesh. Mesh isn't installed. **Fix**: Two-phase bootstrap:
1. **CLI phase**: `mesh init` — installs Mesh binary, generates config, starts Mesh daemon, prints MCP connection string.
2. **MCP phase**: Agent connects via MCP, uses `mesh.plugin.install` to add providers, `mesh.body.create` to spawn first body.

CLI is the bootstrap escape hatch. It must be minimal (~5 commands: `init`, `start`, `stop`, `status`, `config`). D5 (MCP primary) is preserved — CLI is only for first-run and debugging.

### EC5: Plugin crash during tool call

Substrate plugin subprocess crashes mid-operation (e.g., during `Provision`). go-plugin detects crash, returns error to Interface. **Fix**: Interface returns `PROVIDER_ERROR` with retry hint. go-plugin auto-restarts subprocess. Agent retries tool call. If crash is persistent, Interface marks plugin as unhealthy and returns `PROVIDER_UNAVAILABLE`.

### EC6: Auth for network-exposed Mesh

Local-only: no auth needed. Network-exposed (multiple agents, remote access): need auth. **Options**:
- **API key in config** (simple, sufficient for self-hosted): Mesh generates a key on `init`, agents pass it as MCP header.
- **Tailscale identity** (elegant): If Mesh runs on tailnet, Tailscale WhoIs authenticates the caller. Aligns with C3 (user owns network).
- **No auth**: Only for local stdio transport. Dangerous for HTTP transport.

**Recommendation**: API key for v0 (zero complexity). Tailscale identity as v1 enhancement. Matches C4 (no login, no central dependency).

## T6: Contract Verification

### Invariant Checklist

| Invariant | T1 | T2 | T3 | T4 | T5 | Status |
|---|---|---|---|---|---|---|
| INV-1: Every call returns response or error | ✅ | ✅ | ✅ | ✅ | ✅ | OK |
| INV-2: Migration atomic (complete or rollback) | ✅ | ❌ | ✅ | ✅ | ❌ | BROKEN (T2: step g failure leaves old form; T5: rollback can fail) |
| INV-3: Internal errors never leak raw | ✅ | ✅ | ✅ | ✅ | ✅ | OK |
| INV-4: Concurrent ops on same body serialized | ✅ | ✅ | ✅ | ✅ | ✅ | OK (with per-body lock) |
| INV-5: Tool surface is stable/additive | ✅ | ✅ | ✅ | ✅ | ✅ | OK (with tiered tool loading) |
| INV-6: Bootstrap via CLI, not MCP | ✅ | ✅ | ✅ | ✅ | ✅ | OK |

### Broken Invariants → Design Changes

**INV-2 BROKEN (T2)**: Migration is not truly atomic. Steps c-g can fail leaving partial state. **Fix**:
1. Write migration intent record before starting (body_id, target_substrate, current_step).
2. On any failure at steps c-f: execute rollback sequence (destroy new form if created, restart old form).
3. On step g failure: non-fatal, async cleanup. Mark migration as "completed with cleanup pending."
4. On Interface crash during migration: on restart, read intent records, resume or rollback.
5. This makes Interface a **durable coordinator**, not just a stateless proxy.

**INV-2 BROKEN (T5)**: Rollback itself can fail (e.g., old substrate is down). **Fix**: Migration intent record tracks state. If rollback fails, surface as `MIGRATION_PARTIAL` with both form references. Agent or human must resolve manually.

## Updated Interface

### Tool Surface (Tiered)

**Tier 1 — Always Loaded** (agent gets these on connect):
- `mesh.body.create(spec) → Body` — Create and start a body
- `mesh.body.list(filter?) → [Body]` — List bodies
- `mesh.body.inspect(body_id) → Body` — Get body details
- `mesh.body.stop(body_id) → Body` — Stop a body
- `mesh.body.destroy(body_id) → void` — Destroy a body permanently

**Tier 2 — Discoverable** (`mesh.tools.discover`):
- `mesh.body.snapshot(body_id, opts?) → Operation` — Snapshot body filesystem
- `mesh.body.migrate(body_id, target_substrate) → Operation` — Cold migrate body
- `mesh.body.start(body_id) → Body` — Start a stopped body
- `mesh.provisioner.list() → [Provider]` — List available substrate providers
- `mesh.plugin.install(name, source) → Plugin` — Install a plugin
- `mesh.plugin.list() → [Plugin]` — List installed plugins
- `mesh.network.get_endpoint(body_id) → URL` — Get body's network endpoint

**Tier 3 — Admin** (CLI-only or `mesh.tools.discover` with admin flag):
- `mesh.config.set(key, value)` — Set config value
- `mesh.plugin.generate(spec) → Plugin` — Generate a new provider plugin

### Error Code Mapping (gRPC → MCP)

| gRPC Code | MCP Error Response | Retryable |
|---|---|---|
| INSTANCE_NOT_FOUND | `BODY_NOT_FOUND: Body '{id}' does not exist` | No |
| INVALID_STATE | `INVALID_STATE: Body '{id}' is {state}, cannot {operation}. Required states: {valid_states}` | No |
| INSUFFICIENT_RESOURCES | `INSUFFICIENT_RESOURCES: Substrate '{name}' cannot fulfill {resources}. Available: {available}` | No |
| NETWORK_ERROR | `NETWORK_ERROR: Failed to reach substrate '{name}'. Retrying may help.` | Yes |
| AUTHENTICATION_ERROR | `AUTH_ERROR: Substrate '{name}' rejected credentials. Check plugin config.` | No |
| NOT_SUPPORTED | `NOT_SUPPORTED: Operation '{op}' not supported by substrate '{name}'. Capabilities: {caps}` | No |
| TIMEOUT | `TIMEOUT: Operation '{op}' timed out after {duration}. Body may be in transitional state.` | Yes |
| QUOTA_EXCEEDED | `QUOTA_EXCEEDED: Substrate '{name}' quota exceeded. Contact provider.` | No |
| RATE_LIMITED | `RATE_LIMITED: Substrate '{name}' rate limit hit. Retry after {retry_after}s.` | Yes |
| UNKNOWN | `INTERNAL_ERROR: Unexpected error in {module}. Error ID: {trace_id} for debugging.` | No |

### Long-Running Operations

Snapshot and migration return immediately with `{ operation_id, status: "started" }`. Interface emits MCP `notifications/progress` with `progressToken`. Agent can poll via `mesh.operation.status(operation_id)`. On completion, final result available at `mesh.operation.result(operation_id)`.

## Open Questions

- **OQ1**: Should `mesh.body.snapshot` default to clean (stop agent first) or crash-consistent? — **Why it matters**: Clean snapshots are safer but cause downtime. Crash-consistent are faster but may capture partial writes. — **Options**: (a) Default crash-consistent, `clean: true` option for migration. (b) Always clean (matches D1 — agents stop at task boundaries). (c) Agent decides via parameter.

- **OQ2**: Is Interface a single Go binary (MCP server + core logic + plugin loader), or should MCP server be a thin wrapper over a separate Mesh daemon? — **Why it matters**: Single binary is simpler deployment (C6: core is tiny). Separate daemon allows MCP restart without disrupting running bodies. — **Options**: (a) Single binary (`mesh serve` starts everything). (b) Mesh daemon + thin MCP wrapper (like Daytona's `daytona mcp start`).

- **OQ3**: Should migration intent records be stored in a local file (bbolt/pebble), or in-memory with accept-loss-on-crash? — **Why it matters**: Persistent records survive Interface restart (safer). In-memory is simpler but risks orphaned migration state. — **Options**: (a) bbolt embedded DB (persistent, ~1MB overhead). (b) In-memory with startup reconciliation from Orchestration state. (c) No tracking — accept that crash during migration requires manual cleanup.

- **OQ4**: How does Interface discover which module to call for a given tool? — **Why it matters**: Hardcoded routing is simple but fragile. Dynamic routing requires a registry. — **Options**: (a) Hardcoded per tool (e.g., `mesh.body.*` → Orchestration, `mesh.plugin.*` → Plugin Infra). Simple, correct. (b) Internal service registry that modules register with on startup. Overkill for 4-5 modules. **Recommendation**: (a) hardcoded — Interface IS router, and the module graph is static and small.

- **OQ5**: What is a "skill" in Interface context? — **Why it matters**: SYSTEM.md lists "skills" as part of Interface, but the concept is undefined. — **Clarification**: Skills are NOT part of Interface. Skills are agent-side compositions of MCP tools (e.g., a "deploy" skill calls `mesh.body.create` + `mesh.network.get_endpoint`). Interface exposes tools; skills are consumer-side patterns. This should be clarified in SYSTEM.md.