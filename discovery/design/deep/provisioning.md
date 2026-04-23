# Deep-Dive: Provisioning (Provider Plugins)

> Module: Provisioning (Module 2)
> Agent: deep
> Date: 2026-04-23
> Status: draft

## Contract (Detailed)

### Inputs
- **From Orchestration**: `Provisioner.Provision(spec)` where spec = {image: string, resources: {cpu, memory, disk, gpu?}, network?: NetworkConfig, env: map<string,string>, labels: map<string,string>}
- **From Orchestration**: `Provisioner.Destroy(handle)` where handle = SubstrateHandle (returned by Provision)
- **From Interface/Skill**: `Provisioner.ListCapabilities() → CapabilitySet` — queries a loaded provider's capabilities
- **From Plugin Infrastructure**: Plugin discovery triggers — scan `~/.mesh/plugins/`, validate metadata, load via go-plugin
- **From Interface/Skill (generation)**: `mesh.plugin.generate --name <provider> --spec <yaml>` — triggers Pulumi skill to create a new provider plugin

### Outputs
- **To Orchestration**: `Provision(spec) → SubstrateHandle` — a running container with a network endpoint. Handle carries: provider_name, instance_id, endpoint (IP:port or URL), allocated_resources, metadata (provider-specific key-value pairs like region, zone, machine_type).
- **To Orchestration**: `Destroy(handle) → void` — confirmation that the substrate instance is gone.
- **To caller**: `ListCapabilities() → CapabilitySet` — structured declaration of what this provider supports (required verbs, optional verbs, resource limits, GPU, suspend/resume, snapshot type, regions).
- **Structured errors**: gRPC status codes (UNKNOWN, INSTANCE_NOT_FOUND, INVALID_STATE, INSUFFICIENT_RESOURCES, NETWORK_ERROR, AUTHENTICATION_ERROR, NOT_SUPPORTED, TIMEOUT, QUOTA_EXCEEDED, RATE_LIMITED).

### Guarantees (Invariants)

- **INV-1**: `Provision()` returns a handle with a reachable network endpoint OR returns a structured error. Never returns a handle to an unreachable instance.
- **INV-2**: `Destroy(handle)` is idempotent — calling it twice for the same handle succeeds silently on the second call.
- **INV-3**: Every loaded plugin satisfies to 6 required verbs (create, start, stop, destroy, getStatus, exec) — enforced at load time by Plugin Infrastructure.
- **INV-4**: `ListCapabilities()` is constant for a plugin's lifetime — capabilities don't change between calls without plugin reload.
- **INV-5**: Plugin crash does not affect Mesh core — subprocess isolation via go-plugin guarantees this.
- **INV-6**: Destroy releases all provider-side resources (containers, VMs, IPs). No resource leaks on success.

### Assumptions

- **ASM-1**: Orchestration calls `Provision()` only when, body is in a "Created" or "Migrating" state (never "Running"). Provisioning trusts to caller's state machine.
- **ASM-2**: Provider credentials (API keys, tokens) are configured by user before any Provision call. Provisioning does not handle credential acquisition.
- **ASM-3**: The substrate spec (image, resources) is valid — validated by Orchestration before reaching Provisioning.
- **ASM-4**: Plugin Infrastructure module handles plugin discovery, loading, health-checking, and restart. Provisioning calls through Plugin Infrastructure to get an adapter instance.
- **ASM-5**: Networking module assigns identity (Tailscale IP) after Provisioning returns to handle. Provisioning does not configure Tailscale.
- **ASM-6**: The provider substrate is reachable from Mesh core's network position (no firewalls blocking API calls).

## State Machine

### States
The Provisioning module itself is stateless (per D14). But, *adapter instance* (provider plugin subprocess) has lifecycle states:

- **Discovered** — plugin binary found on disk, metadata read
- **Loaded** — subprocess started, gRPC handshake complete, capabilities cached
- **Ready** — plugin configured (credentials passed), accepting requests
- **Degraded** — plugin health check failing, requests rerouted to other providers if available
- **Crashed** — subprocess dead, awaiting restart by Plugin Infrastructure
- **Unloaded** — graceful shutdown, in-flight requests drained

### Transitions

| From | Trigger | To | Side Effects |
|------|---------|----|-------------|
| Discovered | LoadPlugin() called | Loaded | Subprocess launched, handshake, capabilities cached |
| Loaded | Configure(credentials) succeeds | Ready | Credential validation passed |
| Loaded | Configure(credentials) fails | Unloaded | Subprocess killed, error surfaced |
| Ready | Health check fails | Degraded | Log warning, requests may fail-fast |
| Degraded | Health check passes | Ready | Log recovery |
| Degraded | Health check timeout | Crashed | Subprocess assumed dead |
| Ready/Degraded | Crash detected | Crashed | go-plugin auto-restart attempted |
| Crashed | Restart succeeds | Loaded | Capabilities re-cached, configure re-attempted |
| Crashed | Restart fails 3x | Unloaded | Plugin marked unavailable, error surfaced |
| Any | UnloadPlugin() | Unloaded | Drain in-flight, kill subprocess |

### Illegal Transitions

| From State | Rejected Trigger | Error Type | Recovery |
|-----------|-----------------|-----------|---------|
| Unloaded | Provision() | PLUGIN_NOT_LOADED | Caller must install/load plugin first |
| Loaded (not Ready) | Provision() | PLUGIN_NOT_CONFIGURED | Configure must succeed first |
| Crashed | Provision() | PLUGIN_UNAVAILABLE | Wait for restart or use different provider |
| Discovered | Provision() | PLUGIN_NOT_LOADED | LoadPlugin must be called first |

## T1: Happy Path Traces

### Provision a body on Docker (local)
1. Orchestration calls `Provisioning.Provision({image:"ubuntu:22.04", resources:{cpu:2, memory:2048}, env:{}})`
2. Provisioning resolves provider "docker" from loaded plugin registry
3. Provisioning calls `adapter.Create({image, resources, env})` via gRPC
4. Docker plugin: `POST /containers/create` + `POST /containers/{id}/start` → returns instance_id
5. Docker plugin: `GET /containers/{id}/json` → extracts IP, port mappings
6. Provisioning receives `{instance_id: "abc123", metadata: {ip: "172.17.0.2", ports: []}}`
7. Provisioning constructs SubstrateHandle: `{provider: "docker", instance_id: "abc123", endpoint: "172.17.0.2", resources: {cpu:2, memory:2048}}`
8. Returns handle to Orchestration

### Provision a body on E2B (sandbox)
1. Orchestration calls `Provisioning.Provision({image:"ubuntu:22.04", resources:{cpu:4, memory:8192}, env:{}})` with provider hint "e2b"
2. Provisioning resolves "e2b" plugin
3. Provisioning calls `adapter.Create(...)` → E2B API: `Sandbox.create(templateID, {cpuCount:4, memoryMB:8192})`
4. E2B plugin polls until sandbox is running, returns `{instance_id: "sandbox_xyz", metadata: {ssh_host: "...", ssh_port: 4321}}`
5. Handle: `{provider: "e2b", instance_id: "sandbox_xyz", endpoint: "ssh://...:4321", resources: {cpu:4, memory:8192}}`
6. Returns to Orchestration

### Generate a new provider plugin (Pulumi flow)
1. Skill calls `mesh.plugin.generate --name digitalocean --spec do-spec.yaml`
2. Plugin Infrastructure reads spec, checks registry — no existing plugin
3. Plugin Infrastructure invokes Pulumi skill: "Generate Pulumi code for DigitalOcean droplets"
4. Pulumi skill generates TypeScript/Go code using `@pulumi/digitalocean`
5. Plugin Infrastructure wraps generated code in SubstrateAdapter Go template
6. Plugin Infrastructure compiles to `mesh-substrate-digitalocean` binary
7. Plugin Infrastructure loads to new plugin, runs capability validation
8. Plugin registered in config. Available for Provision calls.

## T2: Failure Analysis

| External Call | Failure Mode | State Left | Recovery Path | Cleanup By |
|---------------|-------------|------------|---------------|------------|
| `adapter.Create()` | Provider API rate limit (429) | No instance created | Retry with exponential backoff (5s, 15s, 45s). Max 3 retries. | Provisioning |
| `adapter.Create()` | Auth expired (401/403) | No instance created | Fatal. Return AUTHENTICATION_ERROR to caller. User must update credentials. | Caller (skill/user) |
| `adapter.Create()` | Out-of-capacity / region full | No instance created | Return INSUFFICIENT_RESOURCES. Caller may retry with different region/provider. | Caller |
| `adapter.Create()` | Timeout (30s, no response) | Unknown — instance may or may not exist | Query `getStatus()`. If exists → return handle. If not → retry once. If still unknown → return TIMEOUT with partial state info. | Provisioning + Caller |
| `adapter.Create()` | Partial success (created but start failed) | Instance exists in stopped/errored state | Attempt `adapter.Destroy(instance_id)`. If destroy fails → orphan tracked in error response. | Provisioning (best-effort cleanup) |
| `adapter.Destroy()` | Instance already gone (404) | Clean (nothing to destroy) | Idempotent success (INV-2). | — |
| `adapter.Destroy()` | Timeout (instance stuck terminating) | Instance in unknown state | Retry once. If still stuck → return error. Orphan tracking needed at Orchestration level. | Caller |
| `adapter.Destroy()` | Auth expired | Instance still running, not destroyed | Fatal. Return AUTHENTICATION_ERROR. User must fix credentials, then retry destroy. | Caller |
| Plugin subprocess crash | go-plugin detects process exit | All in-flight gRPC calls fail with transport error | go-plugin auto-restarts subprocess. Plugin Infrastructure re-configures. New calls succeed. In-flight calls are lost — caller must retry. | Plugin Infrastructure |
| Pulumi generation | LLM produces incorrect code | Invalid plugin binary (won't compile or fails handshake) | Compilation fails → surface build error. Handshake fails → surface capability mismatch. User must fix spec and retry. | Caller |
| Pulumi generation | No Pulumi provider exists for target cloud | Cannot generate code | Return NOT_SUPPORTED. Suggest manual plugin implementation. | Caller |

## T3: Concurrency Analysis

### Conflicting Operations
- **Provision + Destroy on same instance ID**: Impossible in normal flow (Provision creates new, Destroy targets existing). But if two callers race to provision same logical body, two instances could be created. Orchestration must serialize at body ID level.
- **Two Destroys for same handle**: Benign — idempotent (INV-2). Second call returns success.
- **Provision + ListCapabilities on same plugin**: No conflict. ListCapabilities reads cached data.
- **Two Provisions on same plugin concurrently**: Provider-dependent. Docker handles it fine. E2B/Fly have API rate limits (see T4). Plugin must be safe for concurrent gRPC calls.
- **Plugin reload while Provision is in-flight**: Dangerous. The in-flight gRPC call could be to a dying subprocess. Plugin Infrastructure must drain in-flight requests before unloading.

### Proposed Locking
- **Body-level lock**: Orchestration holds a per-body-ID mutex. Prevents concurrent Provision/Destroy for same body. Provisioning itself is stateless and doesn't need internal locking.
- **Plugin-level concurrency limit**: Each adapter declares a max concurrent operations limit (in capabilities). Provisioning respects it via a semaphore per provider. Default: 10 concurrent provisions.
- **No deadlock risk**: Locking is one-level (body ID). No nested locks.

## T4: Scale Analysis

### Bottlenecks

| Resource | Limit | Threshold | Mitigation |
|----------|-------|-----------|------------|
| Provider API rate limits | E2B: 10 req/s, Fly: variable, Docker: no limit | 100+ bodies in 10s burst | Semaphore per provider + exponential backoff. Queue excess requests. |
| Plugin subprocess memory | ~20-50MB per plugin process (C1: 2GB VM) | 10+ loaded plugins on a 2GB VM | Lazy-load plugins. Unload after idle timeout. Max 3-5 plugins loaded simultaneously on constrained nodes. |
| gRPC connection pool | go-plugin: 1 connection per plugin subprocess | High request volume on single provider | Multiplex HTTP/2 streams. No per-request connection overhead. |
| Concurrent provisioning operations | Provider-dependent (cloud APIs: 10-50 concurrent) | A3 (ephemeral tasks) spawning 100+ bodies | Rate-limit at Provisioning layer. Queue overflow returns QUOTA_EXCEEDED immediately. |
| Pulumi generation time | 30-120 seconds per plugin (LLM + compile) | User wants a new provider NOW | Async generation. Returns immediately with a generation ID. Poll or callback for completion. Not blocking. |
| Destroy during bulk teardown | 100 bodies → 100 sequential API calls | Agent fleet shutdown | Parallel destroy with concurrency limit. Best-effort — log failures for manual cleanup. |

## T5: Edge Cases

### EC-1: Provision succeeds but endpoint is unreachable
Provisioning returns handle with endpoint. Orchestration or Networking later discovers that endpoint is unreachable (e.g., container started but port not exposed). **Fix**: INV-1 is violated. Provisioning must do a liveness check (TCP connect or HTTP health check) before returning to handle. Add `liveness_check` as an optional field in Provision spec: `{port: 8080, path: "/health", timeout: "10s"}`. If check fails, destroy to instance and return error.

### EC-2: Handle contains stale data after provider-side restart
Provider restarts a container (e.g., host migration on Fly). instance_id stays the same but IP changes. Handle's endpoint is now wrong. **Fix**: This is an Orchestration/Networking concern — they must refresh handle data via `getStatus()` periodically or on connection failure. Provisioning is not responsible for ongoing handle freshness. **Cross-module gap**: No defined mechanism for provider-initiated state change notifications. Need a `WatchStatus(instance_id) → stream<StatusUpdate>` streaming RPC (already defined in proto as `GetLogs` pattern).

### EC-3: User has Docker + Nomad + E2B loaded, no provider hint in spec
Orchestration calls `Provision(spec)` without specifying provider. **Fix**: Provisioning must select a provider. This is Q3 (scheduler core or plugin). Current design: caller always specifies provider. If caller doesn't, Provisioning returns error asking for explicit provider selection. **Open design question**: Should Provisioning support a default scheduling policy? See Q3.

### EC-4: Pulumi generation succeeds but plugin fails at runtime (wrong API calls)
Generated code compiles but makes incorrect API calls to the cloud provider (LLM hallucination). Discovered only when `Provision()` is called. **Fix**: Plugin Infrastructure should run a validation suite after generation: call `Create()` → `getStatus()` → `Destroy()` with a minimal test spec. If validation fails, mark plugin as "unvalidated" and warn user. Don't prevent loading — just surface risk.

### EC-5: Plugin loaded but provider credentials expire mid-operation
E2B API key rotates. Provision was working. Suddenly all calls return 401. **Fix**: Return AUTHENTICATION_ERROR. Orchestration should surface this to user via Interface. User updates credentials in config. Plugin Infrastructure re-configures to plugin. No automatic credential rotation — C3 says user owns all keys.

### EC-6: Destroy called for an instance that's being snapshotted by Persistence
Persistence is running `docker export` on instance. Orchestration concurrently calls Destroy. Race condition: export may be reading from a container that's being killed. **Fix**: Orchestration must serialize operations at body level (T3 locking). Snapshot and Destroy cannot run concurrently on same body. This is an **Orchestration-level invariant**, not Provisioning's responsibility, but Provisioning must fail cleanly: if Destroy is called on a busy instance, return INVALID_STATE.

## T6: Contract Verification

### Invariant Checklist

| Invariant | T1 | T2 | T3 | T4 | T5 | Status |
|-----------|----|----|----|----|----|----|
| INV-1: Handle has reachable endpoint | ✅ | ❌ (timeout creates phantom handle) | ✅ | ✅ | ❌ (EC-1: no liveness check) | BROKEN |
| INV-2: Destroy is idempotent | ✅ | ✅ | ✅ | ✅ | ✅ | OK |
| INV-3: All plugins satisfy required verbs | ✅ | ✅ | ✅ | ✅ | ❌ (EC-4: generated plugin may be broken) | WEAK |
| INV-4: Capabilities are constant | ✅ | ✅ | ✅ | ✅ | ✅ | OK |
| INV-5: Plugin crash doesn't affect core | ✅ | ✅ | ✅ | ✅ | ✅ | OK |
| INV-6: Destroy releases all resources | ✅ | ❌ (auth expired during destroy) | ✅ | ✅ | ❌ (EC-6: concurrent destroy during snapshot) | BROKEN |

### Broken Invariants → Design Changes

**INV-1 broken (phantom handle, unreachable endpoint)**:
- Add liveness probe to Provision flow: after `Create()` succeeds, TCP-connect to to endpoint. If unreachable within timeout → `Destroy()` to instance, return error.
- New field in Provision spec: `health_check: {type: "tcp"|"http", port: int, path: string?, timeout_seconds: int}`
- For substrates where endpoint is assigned dynamically (E2B, Fly), adapter must return endpoint in CreateResponse metadata before liveness check.

**INV-3 weakened (generated plugin may not satisfy verbs)**:
- Plugin Infrastructure runs a smoke test after Pulumi generation: `Create(minimal_spec) → getStatus() → Destroy()`. Only mark plugin as "Ready" if smoke test passes.
- If smoke test fails, plugin stays in "Loaded" state with a warning. User can force-load but gets a clear error on first use.

**INV-6 broken (auth expiry + concurrent ops prevent resource release)**:
- Auth expiry: Return AUTHENTICATION_ERROR. Add a "pending destroy" queue — when credentials are updated, retry queued destroys automatically.
- Concurrent ops: This is an Orchestration responsibility. Flag as cross-module dependency: Orchestration MUST hold body-level lock during destroy.

## Updated Interface

### SubstrateHandle (new detail)

```
SubstrateHandle {
  provider: string          // "docker", "e2b", "fly", etc.
  instance_id: string       // Provider-specific ID
  endpoint: string          // Reachable IP:port or URL
  allocated_resources: Resources  // What was actually allocated (may differ from request)
  metadata: map<string,string>    // Provider-specific: region, zone, machine_type, etc.
  created_at: timestamp
}
```

### Updated Provision signature

```
Provisioner.Provision(spec: ProvisionSpec) → SubstrateHandle | ProvisionError

ProvisionSpec {
  provider: string              // REQUIRED: explicit provider selection
  image: string
  resources: Resources
  network?: NetworkConfig
  env: map<string,string>
  labels: map<string,string>
  health_check?: HealthCheck    // NEW: optional liveness probe
}

HealthCheck {
  type: "tcp" | "http"
  port: int
  path?: string                 // For HTTP type
  timeout_seconds: int          // Default: 10
}
```

### New: PendingDestroy queue

```
Provisioner.QueueDestroy(handle) → DestroyReceipt   // For auth-failure cases
Provisioner.RetryPendingDestroys() → []DestroyResult // Called after credential update
```

## Open Questions

- **OQ-1**: Who selects to provider when caller doesn't specify? (Relates to Q3.) Options: (a) Require explicit provider in every Provision call — simplest, no scheduler. (b) Default to first loaded provider — fragile. (c) Implement trivial "pick cheapest available" in core — scope creep. **Recommendation**: Option (a) for v0. Let's skill/user decide.

- **OQ-2**: Should Provision block until to instance is fully reachable (liveness probe), or return immediately and let to caller poll? Blocking is simpler but makes Provision calls slow (10-30s). Async with polling is more resilient but adds complexity. **Recommendation**: Blocking with configurable timeout (default 30s). Timeout returns a handle with `reachable: false` flag — caller can decide to wait or destroy.

- **OQ-3**: How does Provisioning report orphaned instances? If Destroy fails and to instance exists but is untracked, how does Mesh discover it? Options: (a) `adapter.listInstances()` method — not all providers support listing. (b) Persistence layer tracks instance IDs — cross-module coupling. (c) Manual cleanup via CLI — honest but unpleasant. **Recommendation**: (a) as optional adapter method. Providers that support listing (Docker, E2B, Fly) implement it. Mesh CLI provides `mesh provider gc <provider>` command for manual cleanup.

- **OQ-4**: The Handle's `endpoint` field is underspecified. Is it always `ip:port`? What about SSH endpoints (E2B: `ssh://host:port`)? What about Unix sockets (Docker local)? **Recommendation**: `endpoint` is a URI string. Scheme determines protocol: `tcp://ip:port`, `ssh://host:port`, `unix:///path`. Networking module parses to scheme.

- **OQ-5**: Cross-module gap — no mechanism for provider-initiated notifications. If a Fly machine is suspended by platform (host maintenance), Mesh doesn't know until to next getStatus() call. Should to adapter support a `Watch(instance_id) → stream<StatusEvent>` for push notifications? **Recommendation**: Add optional `WatchEvents(instance_id) → stream<InstanceEvent>` to adapter proto. E2B already has lifecycle webhooks. Fly has machine events. Fall back to polling for providers without push.
