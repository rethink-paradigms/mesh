# Deep-Dive: Orchestration (Body Lifecycle + Substrate Adapter)

> Module: Orchestration (Module 3)
> Agent: deep
> Date: 2026-04-23
> Status: draft

## Contract (Detailed)

### Inputs
- `BodySpec` from Interface module: `{ baseImage, resources: {cpu, memory, disk, gpu}, env: Record<string,string>, substrateHint?: string, networkConfig?: NetworkConfig }`
- Lifecycle commands from Interface: `Start(bodyId)`, `Stop(bodyId)`, `Destroy(bodyId)`, `List()`
- Substrate adapter responses from Provisioning module: `Handle { instanceId, adapterName, endpoint? }`
- Network identity from Networking module: `NetworkIdentity { ip, dnsName, tailnetAddr }`
- Body state queries: `GetStatus(bodyId)`, `GetBody(bodyId)`

### Outputs
- `Body` struct: `{ id: BodyID, spec: BodySpec, state: BodyState, handle?: SubstrateHandle, network?: NetworkIdentity, createdAt, updatedAt }`
- `BodyID`: `{ id: UUID, substrate: string, instanceId: string }`
- Lifecycle operation results: `Result { success: bool, error?: AdapterError, body?: Body }`
- Structured errors per SYSTEM.md error codes: `INSTANCE_NOT_FOUND`, `INVALID_STATE`, `INSUFFICIENT_RESOURCES`, `NETWORK_ERROR`, `TIMEOUT`, `NOT_SUPPORTED`, `QUOTA_EXCEEDED`, `RATE_LIMITED`

### Guarantees (Invariants)
- **INV-1**: Every body returned by `Create` has a unique `BodyID`. No two bodies share the same ID.
- **INV-2**: A body in state `Running` always has both a valid substrate handle AND a valid network identity. Never one without the other.
- **INV-3**: `Destroy(id)` is idempotent. Calling it on an already-destroyed body returns success, not error.
- **INV-4**: After `Destroy(id)` completes, no substrate resources remain allocated for that body. No orphaned containers.
- **INV-5**: A body's state machine transitions are atomic — no external caller observes intermediate states (Starting, Stopping). They see the pre-state or post-state.
- **INV-6**: `List()` only returns bodies the caller owns. Never leaks bodies from other callers (multi-tenant safety).
- **INV-7**: Substrate adapter `create` and `destroy` calls are always paired — every successful `create` has a corresponding `destroy` during body cleanup.

### Assumptions
- **ASM-1**: Provisioning module returns within a bounded timeout. If it hangs, Orchestration must detect and recover.
- **ASM-2**: Networking module is idempotent — calling `AssignIdentity` twice for the same bodyId returns the same identity.
- **ASM-3**: Substrate adapters correctly report `InstanceStatus` and don't lie about state.
- **ASM-4**: The caller (Interface) serializes migration operations. Orchestration does not coordinate multi-step workflows.
- **ASM-5**: Body metadata storage (the registry tracking BodyID → state) is durable and single-writer per body.

## State Machine

### States
- **Created**: Body ID allocated, spec stored. No substrate resources yet.
- **Starting**: Provisioning and networking in progress. Not yet usable.
- **Running**: Body has substrate handle + network identity. Accepting work.
- **Stopping**: Graceful shutdown in progress. SIGTERM sent.
- **Stopped**: Substrate instance stopped but not destroyed. Handle retained.
- **Destroying**: Cleanup in progress. Network revocation, substrate destroy.
- **Destroyed**: Terminal state. All resources released.
- **Error**: Unrecoverable failure during Starting, Stopping, or Destroying. Substrate state is uncertain — may have leaked resources.

### Transitions

| From | Trigger | To | Side Effects |
|------|---------|----|-------------|
| Created | `Start(id)` | Starting | Calls `Provisioner.Provision(spec)`, then `Network.AssignIdentity(bodyId)` |
| Starting | Provision + Network succeed | Running | Persists handle + identity |
| Starting | Provision OR Network fails | Error | Compensating: destroy substrate if provisioned, revoke identity if assigned |
| Running | `Stop(id)` | Stopping | Calls `adapter.stop(instanceId)` |
| Stopping | Adapter confirms stopped | Stopped | Updates state |
| Stopping | Timeout (configurable) | Stopped | Calls `adapter.destroy(instanceId)` — force cleanup |
| Stopped | `Start(id)` | Starting | Calls `adapter.start(instanceId)` (may re-provision if substrate lost instance) |
| Stopped | `Destroy(id)` | Destroying | Calls `Network.RevokeIdentity(bodyId)`, then `adapter.destroy(instanceId)` |
| Running | `Destroy(id)` | Destroying | Calls `adapter.stop(instanceId)` then `adapter.destroy(instanceId)` |
| Destroying | Cleanup succeeds | Destroyed | All resources released |
| Destroying | Cleanup partial fail | Error | Orphaned resource flagged for GC |
| Error | `Destroy(id)` | Destroying | Attempts cleanup despite error state |
| Error | (manual intervention) | Destroyed | After operator confirms resource cleanup |

### Illegal Transitions

| From | Rejected Trigger | Error Type | Recovery |
|------|-----------------|------------|----------|
| Destroyed | `Start(id)` | `INSTANCE_NOT_FOUND` | Create a new body |
| Destroyed | `Stop(id)` | `INSTANCE_NOT_FOUND` | No-op, return success (INV-3) |
| Running | `Start(id)` | `INVALID_STATE` | No-op, return current body |
| Created | `Stop(id)` | `INVALID_STATE` | No-op, return current body |
| Destroying | `Start(id)` | `INVALID_STATE` | Wait for destroy to complete, then create new |
| Starting | `Start(id)` | `INVALID_STATE` | Wait for current start to complete |
| Stopping | `Stop(id)` | `INVALID_STATE` | Wait for current stop to complete |

## T1: Happy Path Traces

### Create and Start a Body
1. Interface calls `BodyManager.Create(spec)` where spec includes `substrateHint: "nomad-fleet"`
2. BodyManager generates `BodyID { id: UUID, substrate: "", instanceId: "" }`, sets state = Created
3. BodyManager persists body to metadata store
4. Interface calls `BodyManager.Start(bodyId)`
5. BodyManager transitions state → Starting
6. BodyManager resolves `substrateHint` → selects Provisioning plugin for Nomad
7. BodyManager calls `Provisioner.Provision(spec)` → returns `Handle { instanceId: "alloc-abc123", adapterName: "nomad" }`
8. BodyManager updates `BodyID.substrate = "nomad"`, `BodyID.instanceId = "alloc-abc123"`
9. BodyManager calls `Network.AssignIdentity(bodyId)` → returns `NetworkIdentity { ip: "100.64.0.5", dnsName: "hermes.mesh" }`
10. BodyManager transitions state → Running
11. BodyManager persists handle + identity
12. Returns `Body { id, spec, state: Running, handle, network, createdAt, updatedAt }`

### Stop and Destroy
1. Interface calls `BodyManager.Stop(bodyId)`
2. BodyManager validates state = Running ✓
3. BodyManager transitions state → Stopping
4. BodyManager calls `adapter.stop("alloc-abc123")` → success
5. BodyManager transitions state → Stopped
6. Interface calls `BodyManager.Destroy(bodyId)`
7. BodyManager transitions state → Destroying
8. BodyManager calls `Network.RevokeIdentity(bodyId)` → success
9. BodyManager calls `adapter.destroy("alloc-abc123")` → success
10. BodyManager transitions state → Destroyed
11. Body metadata retained (for audit) but all external resources freed

## T2: Failure Analysis

| External Call | Failure Mode | State Left | Recovery Path | Cleanup By |
|---|---|---|---|---|
| `Provisioner.Provision(spec)` | Timeout (>60s) | Created (never left) | Retry with same or different substrate hint | None needed |
| `Provisioner.Provision(spec)` | `INSUFFICIENT_RESOURCES` | Created | Return error to caller. Caller picks different substrate. | None needed |
| `Provisioner.Provision(spec)` | `AUTHENTICATION_ERROR` | Created | Return error to caller. User must fix credentials. | None needed |
| `Provisioner.Provision(spec)` | `RATE_LIMITED` | Created | Retry with backoff | None needed |
| `Network.AssignIdentity(bodyId)` after successful Provision | Timeout | **Starting** — substrate instance running, no network identity | **CRITICAL**: Must call `adapter.destroy(instanceId)` to release substrate. Transition to Error. | BodyManager (compensating action) |
| `Network.AssignIdentity(bodyId)` after successful Provision | `NETWORK_ERROR` persistent | Starting — same as above | Same compensating: destroy substrate, transition to Error | BodyManager |
| `adapter.start(instanceId)` | `INSTANCE_NOT_FOUND` | Stopped | Substrate lost the instance. Must re-provision: call `Provisioner.Provision(spec)` again, then `Network.AssignIdentity`. Transition to Error, caller retries. | BodyManager (re-provision path) |
| `adapter.start(instanceId)` | Timeout | Stopped | Retry once. If still fails, transition to Error. | BodyManager |
| `adapter.stop(instanceId)` | Timeout | Running → stuck Stopping | After configurable grace period (default 30s), force-destroy: call `adapter.destroy(instanceId)`. Transition to Stopped (or Error if destroy also fails). | BodyManager |
| `adapter.destroy(instanceId)` | `INSTANCE_NOT_FOUND` | Destroying | Instance already gone. Transition to Destroyed. | None needed |
| `adapter.destroy(instanceId)` | Timeout during Destroying | **Error** — potential orphan | Flag for garbage collection. Transition to Error. Background GC retries destroy periodically. | Background GC process |
| `Network.RevokeIdentity(bodyId)` | Failure | Destroying | Continue destroy anyway. Orphaned network identity is non-critical (Tailscale IPs are plentiful). Log warning. | None (acceptable leak) |
| Metadata store write (state persistence) | Write failure | **In-memory state diverges from persisted state** | Retry write. If persistent, crash-recovery on startup reconciles from substrate state. | Startup reconciliation |

**CRITICAL FINDING**: The `Provision → Network.AssignIdentity` sequence is the most dangerous failure window. The substrate instance exists but has no Mesh identity. BodyManager MUST implement a compensating action: if Network.AssignIdentity fails after successful Provision, destroy the substrate instance before transitioning to Error. This violates INV-2 but preserves INV-4 (no orphaned resources). INV-2 must be weakened: "A body in state `Running` always has both..." — The Error state is an escape hatch.

## T3: Concurrency Analysis

### Conflicting Operations

| Operation A | Operation B | Conflict | Resolution |
|---|---|---|---|
| `Start(id)` | `Stop(id)` | State transition race | First writer wins. Second gets `INVALID_STATE` if state already changed. |
| `Start(id)` | `Destroy(id)` | Create vs delete race | Destroy takes priority. If Start is in progress, Destroy waits for Starting→Running, then transitions to Stopping→Destroying. Alternatively: Destroy transitions Starting→Destroying immediately (cancel). |
| `Stop(id)` | `Destroy(id)` | Redundant — Destroy subsumes Stop | Destroy handles both: stop first, then destroy. If Stop is in progress, wait for completion. |
| `Destroy(id)` | `Destroy(id)` | Double-destroy | Idempotent (INV-3). First succeeds, second returns success. |
| `Create(spec)` | `Create(spec)` | Two bodies with same spec | Different UUIDs (INV-1). No conflict — both succeed. |
| `List()` | Any mutation | Read vs write | Snapshot read. No locking needed. |
| `Start(id)` | `Start(id)` | Double-start | First succeeds. Second gets `INVALID_STATE` (already Running). |

### Proposed Locking
**Body-level mutex** — one lock per body ID, not global. Operations acquire lock before checking/transitions state. Lock granularity: body-level, not operation-level.

**Lock lifecycle:**
1. Lock created when body is Created
2. Lock held for the duration of state transition (Starting, Stopping, Destroying)
3. Lock destroyed when body reaches the Destroyed state

**Deadlock prevention:** Single lock per operation. No operation holds more than one body lock simultaneously. Migration (orchestrated by Interface) acquires locks sequentially: old body first, then new body.

**Implementation note:** The lock must be persistent (not in-memory) if Orchestration can run multiple instances. A simple persisted state column with CAS (compare-and-swap) suffices — no need for a distributed lock manager on 2GB VMs.

## T4: Scale Analysis

### Bottlenecks

| Resource | Limit | Threshold | Mitigation |
|---|---|---|---|
| Metadata store (body registry) | Linear with body count | >1000 bodies: query latency | Index by owner, state. Paginated List(). In-memory cache for hot bodies. |
| Concurrent state transitions | Lock contention per body | >10 concurrent ops on same body (unlikely) | Body-level locks already handle this. Queue excess operations. |
| Provisioning plugin calls | Adapter-dependent (API rate limits) | E2B/Fly: ~100 req/s; Docker local: unlimited | Rate limiter per adapter. Queue excess. Backpressure to caller. |
| Network.AssignIdentity calls | Tailscale API rate limits | ~100 req/s | Batch identity assignments. Cache network state locally. |
| Memory (on 2GB VM) | ~200MB control plane + ~1.8GB workloads | 100+ tracked bodies: metadata overhead ~1KB/body = 100KB | Negligible. Not a bottleneck at this scale. |
| Substrate adapter subprocesses | go-plugin process per loaded adapter | ~5-10MB per subprocess | Load adapters lazily. Unload after idle timeout. Max ~50MB for 5 adapters. |
| Orchestration process itself | Single-instance by design | >100 concurrent Create/Destroy operations | Async operation queue. Worker pool. Return operation ID for polling. |

**Scale ceiling**: On a single 2GB VM, Orchestration can manage ~100 bodies with ~10 concurrent operations before queuing. This is sufficient for personas A1-A5 (A2 packs multiple on one VM, A3/A4 are ephemeral). For >100 bodies, run multiple Orchestration instances behind a coordinator (post-v0).

## T5: Edge Cases

### EC-1: Substrate loses instance while body is Running
- **Scenario**: Nomad node dies, Docker container killed externally, Fly machine destroyed out-of-band.
- **Expected**: BodyManager detects and transitions to Error.
- **Problem**: BodyManager only learns about this when it next polls `adapter.getStatus()` or when a caller tries to operate on the body.
- **Fix**: Background health checker polls Running bodies at a configurable interval (default 60s). If `getStatus` returns `INSTANCE_NOT_FOUND` or consistently returns non-Running status, transition to Error. Caller must Destroy and re-Create.
- **Open question**: Should auto-recovery be attempted? (Re-provision from last snapshot?) Or just flag and let the caller decide? **Recommendation**: Flag only. Auto-recovery is a scheduler decision, not Orchestration's job.

### EC-2: BodyManager crashes during Starting state
- **Scenario**: Provisioning succeeds, BodyManager crashes before persisting the handle or assigning network identity.
- **Expected**: On restart, BodyManager reconciles state.
- **Problem**: Substrate instance is running but BodyManager has no record of it. INV-4 violated.
- **Fix**: On startup, BodyManager queries all loaded adapters for running instances. Any instance not in the metadata store is an orphan. Options: (a) destroy the orphan immediately (aggressive), (b) flag for manual review (safe). **Recommendation**: Flag orphan with `ORPHANED` state, destroy after a configurable TTL (default 1 hour). Log loudly.

### EC-3: Two callers create bodies with same substrate simultaneously
- **Scenario**: Two agents call `Create` with `substrateHint: "nomad-fleet"` at the same time. Nomad has capacity for one.
- **Expected**: One succeeds, one gets `INSUFFICIENT_RESOURCES`.
- **Problem**: If Provisioner.Provision is called before the capacity check, both may succeed transiently, then one gets evicted.
- **Fix**: This is Provisioning's problem, not Orchestration's. Orchestration trusts Provisioner's capacity reporting. If Provisioner over-commits, Orchestration sees a failure at the adapter level and retries or reports an error.

### EC-4: Migration partial failure — old destroyed, new not started
- **Scenario**: Cold migration step sequence: Stop ✓ → Capture ✓ → Destroy old ✓ → Provision new ✓ → Restore ✓ → Network.AssignIdentity fails → Start never happens.
- **Problem**: Old body is gone. New body exists on substrate but has no identity and is in Error state. Agent is offline.
- **Fix**: This is Interface's coordination problem, but Orchestration must support it: (a) BodyManager.Start on the new body should re-attempt Network.AssignIdentity if the body is in Error state from a failed identity assignment. (b) The migration sequence in Interface should be wrapped in a compensating-transaction pattern: if any step after "Destroy old" fails, the new body becomes the canonical body (even in Error), and the agent's last known snapshot is the recovery point.
- **Cross-module gap flag**: SYSTEM.md says Interface orchestrates migration (line 109-117), but Orchestration has no concept of migration. If Interface crashes mid-migration, who finishes? **Recommendation**: Migration state (old body ID, new body ID, current step) must be persisted in a MigrationRecord that Orchestration owns. On restart, Orchestration resumes incomplete migrations. This partially contradicts ASM-4 (Interface serializes migration) — Interface should *initiate* migration, but Orchestration should *execute and persist* the multi-step sequence.

### EC-5: Destroy called on body in Error state with unknown substrate state
- **Scenario**: Body in Error state. Substrate may or may not have an instance running.
- **Expected**: Destroy attempts cleanup, succeeds even if substrate has nothing.
- **Fix**: `adapter.destroy()` must be idempotent — return success even if the instance doesn't exist (already handled by the adapter contract). `Network.RevokeIdentity` the same. Destroy in Error state always attempts both and transitions to Destroyed regardless of individual results.

### EC-6: Substrate adapter returns wrong status
- **Scenario**: Adapter reports `RUNNING` but the instance is actually dead. Or reports `STOPPED` but the instance is consuming resources.
- **Expected**: INV-3 violated.
- **Fix**: Cannot fully solve without substrate verification. Mitigation: health checker (EC-1) catches persistent lies. Trust but verify: periodic `exec("echo alive")` as a liveness check, not just `getStatus()`. This is a plugin quality issue — adapter contracts must document status accuracy requirements.

## T6: Contract Verification

### Invariant Checklist

| Invariant | T1 | T2 | T3 | T4 | T5 | Status |
|---|---|---|---|---|---|---|
| INV-1: Unique BodyID per body | ✅ | ✅ | ✅ | ✅ | ✅ | OK |
| INV-2: Running body has handle + identity | ✅ | ❌ (T2 row 5: network fails after provision) | ✅ | ✅ | ❌ (EC-1: substrate loses instance) | **WEAKENED** |
| INV-3: Destroy is idempotent | ✅ | ✅ | ✅ | ✅ | ✅ | OK |
| INV-4: No orphaned substrate resources | ✅ | ❌ (T2 row 6: destroy timeout, EC-2: crash during Starting) | ✅ | ✅ | ❌ (EC-2: crash orphans) | **BROKEN without GC** |
| INV-5: Atomic state transitions | ✅ | ❌ (T2: metadata write failure) | ✅ | ✅ | ❌ (EC-2: crash mid-transition) | **BROKEN without reconciliation** |
| INV-6: List returns only caller's bodies | ✅ | ✅ | ✅ | ✅ | ✅ | OK (assumes metadata store enforces ownership) |
| INV-7: create/destroy pairing | ✅ | ❌ (T2 row 5, EC-2) | ✅ | ✅ | ❌ | **BROKEN without GC** |

### Broken Invariants → Design Changes

**INV-2 WEAKENED**: Change to "A body in state `Running` always has both a valid substrate handle AND a valid network identity. Bodies in `Error` state may have partial resources." The Error state is an explicit escape hatch. Callers must check state before trusting the handle/network.

**INV-4 + INV-7 FIX**: Requires two additions:
1. **Compensating actions in BodyManager**: Every state transition that allocates an external resource must have a compensating action on failure. The `Starting → Error` path must destroy the substrate instance if it was provisioned.
2. **Background garbage collector**: On startup and periodically, query all adapters for running instances. Cross-reference with the metadata store. Flag or destroy orphans after a TTL.
3. **Persistent operation log**: Before calling any external resource-allocating API, write the intent to a durable log. On crash recovery, replay the log to find incomplete operations and execute compensating actions.

**INV-5 FIX**: State transitions must be persisted atomically with metadata. Use a write-ahead log: (1) write intended transition to WAL, (2) execute external calls, (3) write completed transition to metadata store, (4) clear WAL entry. On recovery, replay incomplete WAL entries.

**EC-4 cross-module gap FIX**: Add `MigrationRecord` to Orchestration's state:
```
MigrationRecord {
  id: UUID,
  oldBodyId: BodyID,
  newBodyId: BodyID,
  currentStep: "stop" | "capture" | "destroy_old" | "provision_new" | "restore" | "assign_identity" | "start" | "cleanup" | "done",
  snapshotRef?: string,
  startedAt: timestamp,
}
```
Interface initiates migration by calling `BodyManager.BeginMigration(oldId, targetSubstrate)`. Orchestration executes the sequence, persisting after each step. On crash, Orchestration resumes from the persisted step. Interface can poll migration status. This partially contradicts the current design where Interface orchestrates — recommend Orchestration owns the migration sequence, with Interface as the initiator.

## Updated Interface

```typescript
interface BodyManager {
  // Existing
  Create(spec: BodySpec): Promise<Body>;
  Start(id: string): Promise<Body>;
  Stop(id: string, opts?: { timeout?: number, force?: boolean }): Promise<Body>;
  Destroy(id: string): Promise<Body>;
  List(filter?: { state?: BodyState, owner?: string }): Promise<Body[]>;
  
  // Added from deep-dive
  GetStatus(id: string): Promise<Body>;
  
  // Migration (moved from Interface to Orchestration)
  BeginMigration(id: string, targetSubstrate: string): Promise<MigrationRecord>;
  GetMigrationStatus(migrationId: string): Promise<MigrationRecord>;
  CancelMigration(migrationId: string): Promise<MigrationRecord>;
  
  // Recovery
  Reconcile(): Promise<{ orphans: string[], recovered: string[] }>;
}
```

## Open Questions

- **OQ-1**: Should Orchestration own the migration sequence (recommended above) or should Interface orchestrate it? The current SYSTEM.md design has Interface coordinating, but EC-4 shows crash-recovery requires Orchestration ownership. **Recommendation**: Orchestration owns the sequence, Interface initiates. Update SYSTEM.md. — *This affects Q3 (scheduler core or plugin) — if Orchestration owns migration, it's taking on more responsibility than the current contract suggests.*

- **OQ-2**: How does Orchestration persist body metadata on a 2GB VM? SQLite? Flat files? BadgerDB? Must not require an external database (C4). Must be crash-safe (INV-5). — *SQLite with WAL mode is a pragmatic choice: single file, crash-safe, no daemon, query-capable.*

- **OQ-3**: Should Error state be recoverable (can transition back to Stopped via manual intervention) or terminal (must Destroy and re-Create)? — *Recommendation: Recoverable — `Start(id)` on an Error body attempts re-provision. If successful, transitions to Running. If not, stays in Error. Caller always has the option to Destroy.*

- **OQ-4**: What is the scheduling hint contract? If `substrateHint: "nomad-fleet"` is provided but the Nomad plugin isn't installed, does Create fail immediately or fall back to any available substrate? — *Recommendation: Fail immediately with `NOT_SUPPORTED`. Fallback is the caller's decision, not Orchestration's. This respects the "hints, not decisions" boundary.*

- **OQ-5**: Background health checker interval and cost. Polling 100 bodies at 60s intervals = ~1.67 QPS to adapters. Acceptable? Configurable? — *Configurable, default 60s. Adapter calls are local (gRPC to subprocess), not remote API calls. Cost is negligible.*
