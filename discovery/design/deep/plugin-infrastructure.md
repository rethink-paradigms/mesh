# Deep-Dive: Plugin Infrastructure (Discovery + Loading + Generation)

> Module: Plugin Infrastructure
> Agent: deep
> Date: 2026-04-23
> Status: draft

## Contract (Detailed)

### Inputs
- Plugin type (provider, storage, scheduler) + name string, from Orchestration (provisioning requests) or Interface (user commands via `mesh.plugin.install`, `mesh.plugin.generate`).
- Plugin spec for generation: target provider name, API requirements, resource constraints. Comes from user/agent via MCP.
- Plugin configuration (credentials, region, API keys). Comes from user's `~/.mesh/config.yaml`. Passed to plugin at Configure step, never stored in core.

### Outputs
- `PluginRegistry.Discover(type) → []PluginMeta` — list of available plugins (installed + discovered).
- `PluginRegistry.Load(name) → Plugin` — loaded plugin client (gRPC client to subprocess). Implements SubstrateAdapter interface.
- `PluginGenerator.Generate(spec) → Plugin` — AI-generated plugin binary, compiled and installed. Returns loaded plugin.
- Lifecycle results: install confirmation, removal confirmation, update status.

### Guarantees (Invariants)
- **INV-1**: Plugin crash NEVER crashes Mesh core. go-plugin subprocess isolation guarantees this.
- **INV-2**: Every loaded plugin satisfies the SubstrateAdapter protocol (required capabilities: create, start, stop, destroy, get_status, exec). Validation happens at load time, not at call time.
- **INV-3**: Plugin state is always recoverable. Plugins are stateless (D14); all state lives in Mesh core. A crashed/restarted plugin re-reads configuration and resumes.
- **INV-4**: No plugin discovery mechanism phones home. Directory-based, local only. C4 compliance.
- **INV-5**: Only one version of a given plugin is loaded at a time. Loading v2 while v1 is active requires an explicit unload → load cycle.
- **INV-6**: Generated plugins compile before being registered. If Pulumi generation produces uncompilable code, the plugin is NOT registered and the user gets a clear error.

### Assumptions
- **ASM-1**: Plugin binaries exist on the local filesystem at known paths (`~/.mesh/plugins/`). No remote plugin fetching (C4: no central dependency).
- **ASM-2**: The host OS can execute the plugin binary (correct architecture, executable permission).
- **ASM-3**: The Pulumi skill has access to an LLM endpoint for code generation. If LLM is unavailable, generation fails gracefully (not silently).
- **ASM-4**: Plugin dependencies (Pulumi providers, cloud SDKs) are available at plugin compile/runtime. Plugin manages its own deps.
- **ASM-5**: go-plugin's gRPC transport works on the host (Unix domain sockets or TCP loopback).

## State Machine

### States
Plugin instances (not the module itself) have a lifecycle:
- **Discovered** — metadata found on disk, not yet loaded.
- **Loading** — subprocess launching, handshake in progress.
- **Ready** — loaded, capabilities validated, accepting calls.
- **Failed** — load failed or plugin crashed, not accepting calls.
- **Unloading** — draining in-flight requests, shutting down subprocess.
- **Removed** — binary deleted from disk.

### Transitions

| From | Trigger | To | Side Effects |
|------|---------|----|-------------|
| Discovered | `Load(name)` called | Loading | Subprocess launched |
| Loading | Handshake success + capability validation pass | Ready | Plugin cached in registry |
| Loading | Handshake fail / timeout / capability mismatch | Failed | Subprocess killed, error returned |
| Ready | Plugin process crashes | Failed | In-flight calls get errors, retry possible |
| Ready | `Unload(name)` called | Unloading | New calls rejected |
| Failed | `Reload(name)` called | Loading | Fresh subprocess launch |
| Failed | `Remove(name)` called | Removed | Binary deleted |
| Unloading | All in-flight requests complete | Discovered | Subprocess killed |
| Ready | Health check fails repeatedly | Failed | Marked unhealthy, callers get error |

### Illegal Transitions

| From | Rejected Trigger | Error Type | Recovery |
|------|-----------------|------------|----------|
| Loading | `Load(name)` again | ALREADY_LOADING | Wait for first load to complete |
| Ready | `Load(name)` again | ALREADY_LOADED | Call `Unload` first, or `Reload` |
| Removed | Any call | PLUGIN_NOT_FOUND | Install plugin first |
| Unloading | New gRPC call | PLUGIN_UNLOADING | Caller retries after unload completes |

## T1: Happy Path Traces

### Operation 1: Discover and Load a Plugin
1. Orchestration calls `PluginRegistry.Discover("provider")` → returns `[PluginMeta{type:provider, name:"docker", version:"1.0.0", path:"~/.mesh/plugins/mesh-substrate-docker"}]`
2. Orchestration calls `PluginRegistry.Load("docker")` → registry checks cache (miss) → launches subprocess via go-plugin (`exec.Command(pluginPath)`) → handshake on stdout → gRPC connection established → calls `GetCapabilities()` → validates required capabilities present → stores in cache → returns loaded Plugin client.
3. Orchestration calls `plugin.Create(spec)` → gRPC call to plugin subprocess → plugin provisions Docker container → returns `CreateResponse{instance_id: "abc123"}`.

### Operation 2: Generate a Missing Plugin
1. User calls `mesh.plugin.generate --name digitalocean --spec do-spec.yaml` via MCP.
2. Interface calls `PluginGenerator.Generate(spec)`.
3. Generator invokes Mesh skill → extracts DO API requirements from spec.
4. Mesh skill calls Pulumi AI → generates Pulumi TypeScript/Go code for DO droplet CRUD.
5. Mesh skill wraps generated code in SubstrateAdapter Go template.
6. Generator compiles Go code → `go build` → produces `mesh-substrate-digitalocean` binary.
7. Generator places binary in `~/.mesh/plugins/`, writes `plugin.yaml`.
8. Generator calls `PluginRegistry.Load("digitalocean")` → validates capabilities → returns loaded plugin.
9. Plugin is Ready. User can now deploy to DO.

### Operation 3: Unload and Remove a Plugin
1. User calls `mesh.plugin.remove digitalocean` via MCP.
2. Registry checks: are there running bodies using this plugin? If yes → reject with BODIES_ACTIVE error.
3. Registry calls `Unload("digitalocean")` → stops accepting new calls → waits for in-flight requests (timeout: 30s) → kills subprocess.
4. Registry deletes binary from `~/.mesh/plugins/`.
5. State → Removed.

## T2: Failure Analysis

| External Call | Failure Mode | State Left | Recovery Path | Cleanup By |
|---|---|---|---|---|
| Plugin subprocess launch | Binary not found / not executable | Discovered (never left) | Return PLUGIN_NOT_FOUND, suggest install/generate | Caller |
| go-plugin handshake | Timeout (plugin hangs on startup) | Loading → Failed | Kill subprocess, return HANDSHAKE_TIMEOUT. Retry by caller. | PluginRegistry |
| `GetCapabilities()` call | gRPC error / capability mismatch | Loading → Failed | Kill subprocess, return CAPABILITY_MISMATCH with details. Plugin author must fix. | PluginRegistry |
| `go build` (generation) | Compilation error | No binary produced | Return GENERATION_FAILED with compiler output. User must fix spec or retry. | PluginGenerator |
| Pulumi AI generation | LLM unavailable / rate limited | No code produced | Return GENERATION_UNAVAILABLE. Retry by user. | PluginGenerator |
| Pulumi AI generation | Code compiles but fails at runtime | Plugin in Failed state | Plugin falls to Failed on first call. User gets error. Fix: update plugin. | Plugin author |
| Plugin subprocess crash mid-operation | Process killed (OOM, segfault) | Failed | In-flight gRPC calls get connection error. Orchestration treats as provision failure. PluginRegistry auto-restarts on next Load call. | PluginRegistry |
| Plugin subprocess crash (idle) | Process dies between calls | Failed | Next call detects dead connection. Auto-reload: launch new subprocess, handshake, return new client. Transparent to caller. | PluginRegistry |
| Config read (credentials) | File missing / malformed | Plugin load succeeds but Configure fails | Return CONFIGURATION_ERROR. User must fix config. | User |
| Plugin binary overwrite (upgrade) | Binary replaced while plugin is loaded | Loaded plugin continues with old binary in memory | Unload → replace binary → Reload. If binary deleted while loaded: subprocess continues until killed, then fails to reload. | PluginRegistry |

**Critical finding**: Auto-reload on subprocess crash is NOT in the current design. go-plugin's `Client.Kill()` and re-launch works, but the registry must implement it. Without auto-reload, a plugin crash means every subsequent call fails until manual intervention. **Design change: PluginRegistry must wrap all gRPC calls with crash-detection and auto-reload logic.**

## T3: Concurrency Analysis

### Conflicting Operations
- **Load + Load (same plugin)**: Two goroutines try to load "docker" simultaneously. Must serialize — second call should wait for first to complete, then return cached client.
- **Load + Unload (same plugin)**: Race condition. Unload kills subprocess while Load is trying to use it. Must serialize at plugin-name granularity.
- **Discover + Remove**: Scanning directory while deleting files. Read-only scan + delete is safe on most OSes, but metadata may be stale.
- **Generate + Install**: Two generations of "digitalocean" running simultaneously. Last write wins — one binary overwrites the other. Need a file lock or temp-file-rename pattern.

### Proposed Locking
- **Per-plugin mutex** (keyed by plugin name). Serializes Load, Unload, Reload, Remove for the same plugin.
- **Registry-level RWMutex**: Write-lock for install/remove/generate. Read-lock for discover/list.
- **Generation temp directory**: Generate to `~/.mesh/plugins/.tmp/<name>-<uuid>/`, then atomic rename to final path. Prevents partial writes.
- **No deadlock risk**: Lock hierarchy is always registry-level → per-plugin. Never reversed.

## T4: Scale Analysis

### Bottlenecks

| Resource | Limit | Threshold | Mitigation |
|----------|-------|-----------|------------|
| Subprocess memory | Each plugin ~20-100MB | 10 plugins on 2GB VM = 200-1000MB | Lazy-load: only load plugins when needed. Unload after idle timeout. |
| gRPC connections | One TCP/Unix socket per plugin | Unlikely to hit limits (<1000 plugins) | Pool connections if needed |
| Plugin binary size | Typically 10-50MB | Disk space on 2GB VM | Plugins stored outside core; user manages cleanup |
| Generation time | Pulumi AI: 30-120 seconds + compile: 10-30 seconds | User experience: "I want DO and I want it now" | Show progress. Pre-generate common providers. Cache generation results. |
| Directory scan (Discover) | O(n) where n = files in plugin dir | Matters at 100+ plugins | Cache discovery results. Invalidate on install/remove. |
| Concurrent gRPC calls to same plugin | go-plugin uses single subprocess, gRPC multiplexes | High call volume on one provider | gRPC HTTP/2 handles this. Plugin implementation must be thread-safe. |

**Scale constraint**: On a 2GB VM (C1), running more than ~5 plugin subprocesses simultaneously is risky. Design must support lazy-loading (load on first use) and idle-unloading (kill subprocess after N minutes of no calls).

## T5: Edge Cases

### EC1: Generated plugin compiles but has runtime bug
- **Scenario**: Pulumi AI generates code for DigitalOcean. Code compiles. Plugin loads. First `Create()` call panics inside the plugin.
- **Expected**: Plugin crashes → subprocess dies → PluginRegistry detects failure → caller gets error → auto-reload attempted → same crash → plugin marked Failed.
- **Actual if no auto-reload**: Every subsequent call fails with connection error. No way to recover without manual restart.
- **Fix**: Auto-reload with crash counter. After N crashes (default: 3), mark plugin as Failed permanently and return an actionable error ("Plugin digitalocean crashed 3 times. Check plugin logs or regenerate.").

### EC2: Plugin declares capability but doesn't implement it correctly
- **Scenario**: Plugin declares `export_filesystem: true` in capabilities, but `ExportFilesystem()` returns UNIMPLEMENTED gRPC error.
- **Expected**: Mesh core checks capabilities before calling optional methods. If capability declared, call should work.
- **Actual**: Capability check passes, call fails at runtime.
- **Fix**: Two-layer defense: (1) Capability check at load time (declared vs required). (2) Graceful error handling at call time for optional methods — treat UNIMPLEMENTED as "capability not actually available" and fall back to core implementation (e.g., `docker export` directly).

### EC3: Upgrade plugin while bodies are running on it
- **Scenario**: Plugin v1 created 5 bodies. User upgrades to v2. Bodies are still running, managed by v1 subprocess.
- **Expected**: Unload rejects if active bodies exist. OR: v1 subprocess keeps running for existing bodies, v2 is loaded for new bodies.
- **Actual design**: Unload checks for active bodies and rejects. User must stop/destroy all bodies first, then upgrade.
- **Gap**: This is disruptive. For A1 (Hermes, 24/7 agent), upgrading the provider means stopping the agent. Consider hot-swap: load v2 alongside v1, route new calls to v2, drain v1 when all its bodies are destroyed.

### EC4: Pulumi AI generates code for a provider with no Terraform/Pulumi provider
- **Scenario**: User wants a plugin for a niche platform with no existing Pulumi/Terraform provider.
- **Expected**: Generation fails with a clear message.
- **Actual**: Pulumi AI might hallucinate API calls. Code compiles against generated stubs but fails at runtime.
- **Fix**: Generation step must validate that `@pulumi/<provider>` or `terraform-provider-<name>` exists before generating code. If not found, return PROVIDER_NOT_FOUND with instructions for manual implementation.

### EC5: Plugin directory doesn't exist (fresh install)
- **Scenario**: First-time Mesh install. `~/.mesh/plugins/` doesn't exist.
- **Expected**: Discover returns an empty list. Load returns PLUGIN_NOT_FOUND. Generate creates the directory.
- **Fix**: Mesh bootstrap creates `~/.mesh/plugins/` on first run. Discover creates directory if missing (idempotent).

### EC6: Two plugins claim same name
- **Scenario**: User installs `mesh-substrate-digitalocean` from two sources. Binaries collide.
- **Expected**: Second install overwrites the first. Or: reject with name conflict.
- **Fix**: Install validates no existing plugin with same name. If exists, require `--force` flag or explicit version pin.

## T6: Contract Verification

### Invariant Checklist

| Invariant | T1 | T2 | T3 | T4 | T5 | Status |
|-----------|----|----|----|----|----|----|
| INV-1: Plugin crash doesn't crash core | ✅ | ✅ | ✅ | ✅ | ✅ | OK — go-plugin subprocess isolation guarantees this |
| INV-2: Loaded plugins satisfy required capabilities | ✅ | ✅ | ✅ | ✅ | ❌ | BROKEN in EC2 — capability declared but unimplemented |
| INV-3: Plugin state always recoverable | ✅ | ✅ | ✅ | ✅ | ✅ | OK — plugins stateless by design |
| INV-4: No phone-home during discovery | ✅ | N/A | ✅ | ✅ | ✅ | OK — directory-based only |
| INV-5: One version loaded at a time | ✅ | ✅ | ✅ | ✅ | ❌ | BROKEN in EC3 — upgrade during active use needs hot-swap |
| INV-6: Generated plugins compile before registration | ✅ | ✅ | ✅ | ✅ | ✅ | OK — `go build` gate exists |

### Broken Invariants → Design Changes

**INV-2 broken (EC2: declared but unimplemented capability):**
- **Fix**: Add a capability-probe step after load. For each declared optional capability, call a lightweight `PingCapability(capability_name)` RPC. Plugin responds with supported/unsupported. Trust the probe, not the declaration. If probe says unsupported, a downgraded capability set is stored.
- **Alternative**: Accept the risk (capability declarations are best-effort). Callers handle UNIMPLEMENTED gracefully. Lower effort, higher runtime error surface.
- **Recommendation**: Accept the risk for v0. Add probe in v1 when the plugin ecosystem matures.

**INV-5 broken (EC3: upgrade during active use):**
- **Fix**: Support versioned plugin slots. Registry tracks: `plugins["docker"] = {active: v1, pending: v2, activeBodies: 5}`. New calls route to v2. When all v1 bodies are destroyed, v1 is unloaded. This is complex.
- **Alternative**: Keep current design (reject upgrade if active bodies). Document the constraint. Simpler.
- **Recommendation**: Keep simple for v0. Hot-swap is a v1 feature.

## Updated Interface

**Additions discovered during analysis:**

```go
// PluginRegistry — added methods
type PluginRegistry interface {
    Discover(pluginType string) ([]PluginMeta, error)
    Load(name string) (Plugin, error)
    Unload(name string) error           // NEW: explicit unload
    Reload(name string) (Plugin, error) // NEW: unload + load (for crash recovery / upgrade)
    Remove(name string) error           // NEW: unload + delete binary
    IsLoaded(name string) bool          // NEW: check without triggering load
    ActiveBodies(name string) ([]string, error) // NEW: bodies using this plugin
}

// PluginGenerator — refined
type PluginGenerator interface {
    Generate(spec PluginSpec) (PluginMeta, error) // Returns metadata, NOT loaded plugin
    // Generation is async — returns immediately, user polls or gets notified
    Status(correlationID string) (GenerationStatus, error) // NEW: check generation progress
}

// Plugin — crash-resilient wrapper
type Plugin interface {
    SubstrateAdapter // embedded gRPC client
    HealthCheck(ctx context.Context) error // NEW: explicit health check
}
```

**Key change**: Plugin calls go through a resilient wrapper that detects subprocess crashes and auto-reloads. Callers don't see gRPC connection errors — they see either a valid response or a PLUGIN_FAILED error after N retry attempts.

## Open Questions

- **OQ1: Auto-reload vs manual reload on plugin crash** — Auto-reload is more resilient but risks crash loops (plugin keeps dying, keeps getting relaunched). Option A: auto-reload with crash counter (max 3, then manual). Option B: manual only, user runs `mesh plugin reload <name>`. Recommendation: A with exponential backoff.
- **OQ2: Plugin generation latency** — Pulumi AI + compile = 40-150 seconds. Is this acceptable for interactive use? Option A: async with progress events via MCP. Option B: pre-generate popular providers (Docker, Fly, E2B) and ship as optional downloads. Recommendation: Both.
- **OQ3: Cross-module gap: Who checks that a plugin supports operations needed for migration?** — Cold migration needs ExportFilesystem + ImportFilesystem (optional capabilities). Orchestration triggers migration. Plugin Registry validates capabilities. But if the target substrate plugin doesn't support import, migration fails at step 3c (SYSTEM.md data flow). This check should happen BEFORE step 2a (stop body). Flagged as cross-module design gap between Orchestration and Plugin Infrastructure.
- **OQ4: Plugin language for generated plugins** — Research says Go, but Pulumi AI generates TypeScript most fluently. Generated plugin could be TypeScript wrapped in a Go shim, or pure Go using pulumi-go-provider. Trade-off: generation quality vs. runtime simplicity. Recommendation: Go via pulumi-go-provider for v0.
- **OQ5: Plugin signing / trust model** — C3 says user owns compute. But users install community plugins. How do they verify a plugin isn't malicious? Optional signature verification is mentioned in SYSTEM.md but not designed. Defer to v1.
