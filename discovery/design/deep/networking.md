# Deep-Dive: Networking (Tailscale + Identity)

> Module: Networking
> Agent: deep
> Date: 2026-04-23
> Status: draft

## Contract (Detailed)

### Inputs
- `bodyId` (UUID) — from Orchestration, when a body reaches "Running" state
- `formId` (substrate-specific instance ID) — from Provisioning, identifies the running container
- Substrate capabilities — from Provisioning adapter, declares whether `/dev/net/tun` + NET_ADMIN available
- User config — Tailscale auth key, tailnet name, headscale URL, or "no networking" flag

### Outputs
- `NetworkIdentity` — `{ dnsName: string, ip: string, tailnetIp: string | null, url: string, reachable: boolean }`
- `URL` — `http(s)://<dnsName>.mesh.local:<port>` or `http://<tailnetIp>:<port>`
- Network policy enforcement — allow/deny rules between bodies
- SSH access endpoint — `ssh://<dnsName>.mesh.local`

### Guarantees (Invariants)
- **INV-1**: A body in "Running" state with networking enabled ALWAYS has a resolvable DNS name, or returns an explicit error. Never returns stale/unreachable identity.
- **INV-2**: `dnsName` survives substrate migration unchanged. IP may change, DNS name does not.
- **INV-3**: `GetEndpoint(bodyId)` returns a URL that is reachable within 5 seconds of identity assignment, or returns `NETWORK_ERROR`.
- **INV-4**: When a body transitions to "Destroyed", its network identity is revoked within 30 seconds.
- **INV-5**: Any two bodies on the same user tailnet can establish TCP connectivity to each other's exposed ports.
- **INV-6**: Networking is optional. Bodies can run without Tailscale. GetEndpoint returns `NOT_CONFIGURED` in this case.

### Assumptions
- **ASM-1**: Orchestration calls `AssignIdentity` only after body reaches "Running" state. Calling on a non-running body is undefined.
- **ASM-2**: User's Tailscale account or headscale instance is operational and accepting new devices.
- **ASM-3**: Substrate either grants `/dev/net/tun` + NET_ADMIN (full Tailscale) OR supports userspace networking (SOCKS proxy). If neither, Networking degrades to "no connectivity" mode.
- **ASM-4**: Orchestration calls `RevokeIdentity` before `Destroy` during normal lifecycle. If Orchestration crashes, garbage collection handles orphaned identities.

## State Machine

### States
- `Unassigned` — body exists but has no network identity
- `Assigning` — Tailscale auth key requested, device joining tailnet
- `Assigned` — body has DNS name, IP, is reachable
- `Reassigning` — migration in progress, identity moving to new form
- `Revoking` — identity being removed from tailnet
- `Failed` — assignment or reassignment failed, identity in unknown state

### Transitions

| From | Trigger | To | Side Effects |
|------|---------|----|-------------|
| Unassigned | AssignIdentity(bodyId) | Assigning | Generate Tailscale auth key, start Tailscale in container |
| Assigning | Device joined tailnet | Assigned | Register DNS name, update body metadata |
| Assigning | Tailscale timeout/error | Failed | Log error, body remains running but unreachable |
| Assigned | ReassignIdentity(bodyId, newFormId) | Reassigning | Start Tailscale in new container, prepare DNS update |
| Reassigning | New device joined | Assigned | Update DNS name→IP mapping, revoke old device |
| Reassigning | New device failed | Assigned | Keep old identity, log warning, migration incomplete |
| Assigned | RevokeIdentity(bodyId) | Revoking | Remove device from tailnet |
| Revoking | Device removed | Unassigned | Clean up DNS entry |
| Failed | Retry AssignIdentity | Assigning | Fresh auth key, new attempt |

### Illegal Transitions

| From | Rejected Trigger | Error Type | Recovery |
|------|-----------------|------------|----------|
| Assigned | AssignIdentity(bodyId) | ALREADY_ASSIGNED | Use ReassignIdentity for migration |
| Unassigned | RevokeIdentity(bodyId) | NOT_ASSIGNED | No-op, return success |
| Assigning | ReassignIdentity(bodyId) | IN_PROGRESS | Wait for assignment to complete, then reassign |
| Revoked | GetEndpoint(bodyId) | NOT_ASSIGNED | Body must be reassigned first |

## T1: Happy Path Traces

### AssignIdentity (new body on Fleet VM)
1. Orchestration calls `Network.AssignIdentity(bodyId="a1-hermes", formId="nomad-alloc-123")`
2. Networking checks substrate capabilities: Docker driver, `/dev/net/tun` available → full Tailscale mode
3. Networking generates ephemeral Tailscale auth key via Tailscale API (or headscale)
4. Networking calls `exec(formId, ["tailscaled", "--tun=userspace-networking"])` — OR injects auth key via container env
5. Tailscale joins tailnet, gets IP `100.64.0.42`
6. Networking registers DNS: `a1-hermes.mesh.local → 100.64.0.42` (via MagicDNS custom domain or internal DNS overlay)
7. Returns `NetworkIdentity{ dnsName: "a1-hermes", ip: "100.64.0.42", url: "http://a1-hermes.mesh.local:8080", reachable: true }`

### Migration Reassignment (Fleet VM → Sandbox)
1. Migration step 2e: `Network.ReassignIdentity(bodyId="a1-hermes", newFormId="fly-machine-456")`
2. Networking checks new substrate capabilities: Fly Machine → full Tailscale available
3. Generate new auth key, start Tailscale in new container
4. New device joins tailnet, gets IP `100.64.0.99`
5. Update DNS: `a1-hermes.mesh.local → 100.64.0.99` (atomic swap)
6. Revoke old device from tailnet (remove `nomad-alloc-123` device)
7. Return `NetworkIdentity{ dnsName: "a1-hermes", ip: "100.64.0.99", ... }` — DNS name unchanged

### Burst Clone Connectivity (A4 → A1)
1. A4 clone on Sandbox, A1 parent on Fleet, both on same tailnet
2. A4 calls `Network.GetEndpoint("a1-hermes")` → `http://a1-hermes.mesh.local:8080`
3. Tailscale mesh routes directly (no Mesh intervention needed) — both on same WireGuard mesh
4. No extra Mesh logic required — this is Tailscale's normal behavior

## T2: Failure Analysis

| External Call | Failure Mode | State Left | Recovery Path | Cleanup By |
|--------------|-------------|------------|---------------|------------|
| Tailscale auth key generation | API timeout, rate limit, invalid credentials | Assigning (stuck) | Retry with exponential backoff (3 attempts). If credentials invalid, transition to Failed, surface to user via MCP error. | Networking module |
| Container Tailscale startup | Exec fails, `/dev/net/tun` missing, NET_ADMIN denied | Assigning (stuck) | Detect capability mismatch. If userspace networking possible, retry with `--tun=userspace`. If not, transition to Failed, set `reachable: false`, body runs without networking. | Networking module |
| DNS registration/update | MagicDNS unavailable, custom DNS API error | Assigned but unreachable by name | IP still works. DNS is best-effort. Retry DNS update asynchronously. Body reachable via IP directly. | Networking module (async) |
| Tailnet device removal (migration) | Old device removal fails after new device assigned | Both devices on tailnet (wasted slot) | Log warning. Old device orphaned but harmless. Garbage collect on next reconciliation pass. | Networking module (periodic GC) |
| Tailscale coordination server down during migration | New device can't join, old device revoked | Body unreachable (neither old nor new) | **CRITICAL**: Do NOT revoke old device until new device is confirmed joined. Migration should stall at Reassigning, keeping old identity active. | Orchestration (caller aborts migration) |
| RevokeIdentity on body destroy | Tailscale API error | Device orphaned on tailnet | Log warning. Add to orphan cleanup queue. Next reconciliation removes it. Not blocking for body destruction. | Networking module (periodic GC) |

**Critical design decision from T2**: Migration identity swap MUST be atomic from the consumer's perspective. The new device must be confirmed reachable BEFORE the old device is revoked. If this order is reversed and Tailscale is down, the body becomes unreachable with no rollback path.

## T3: Concurrency Analysis

### Conflicting Operations
1. **AssignIdentity + ReassignIdentity for same bodyId** — race: assignment completing while migration starts
2. **Two ReassignIdentity for same bodyId** — two simultaneous migrations (Orchestration bug, but must handle)
3. **AssignIdentity + RevokeIdentity for same bodyId** — create and destroy racing

### Proposed Locking
- **Body-level locking**: One network operation per bodyId at a time. Use a per-bodyId mutex (in-memory map with sync.Mutex).
- **No global lock needed**: Different bodies can be assigned/reassigned independently. Tailscale API handles device-level concurrency.
- **Deadlock prevention**: Lock is always acquired in bodyId order. Migration (which touches two bodies) acquires locks in bodyId alphabetical order to prevent ABBA deadlock.
- **Timeout**: Lock acquisition has a 30-second timeout. If lock can't be acquired, return `CONFLICT` error to caller.

## T4: Scale Analysis

### Bottlenecks

| Resource | Limit | Threshold | Mitigation |
|----------|-------|-----------|------------|
| Tailscale API rate limit | ~10 req/sec (auth keys) | ~10 bodies/sec creation rate | Batch auth key generation. Pre-generate keys in pool of 50. Refill asynchronously. |
| Devices per tailnet | 100 (personal), 500 (business), unlimited (enterprise/headscale) | 100 bodies for personal tier | Document tailnet limits. Recommend headscale for fleet-scale. Support multiple tailnets. |
| Tailscale per-VM memory | ~20MB per instance | 50 bodies on one VM = 1GB just for Tailscale | Shared Tailscale proxy per VM for packed bodies (see multiple bodies below). |
| DNS update throughput | ~100 updates/sec (MagicDNS) | 100 simultaneous migrations | Batch DNS updates. Use TTL of 10s for fast propagation. |
| Multiple bodies per VM | 1 Tailscale instance per host (with tun) | >1 body per VM (A2 packing) | **ARCHITECTURAL**: Need per-VM Tailscale proxy. See design gap below. |

### Multiple Bodies on One VM (Critical Design Gap)
A2 Tool Agents packed 10-20 per Fleet VM. Each needs a unique DNS name and port. Running 20 Tailscale daemons on one VM is wasteful and may conflict on `/dev/net/tun`.

**Proposed solution**: Per-VM Tailscale sidecar proxy pattern:
- One Tailscale instance per VM (the "node identity")
- Each body gets a unique port range on that VM
- Mesh runs a local reverse proxy (Caddy/envoy) that routes `{dnsName}.mesh.local` → `localhost:{port}`
- Networking module maintains port allocation table per VM
- Trade-off: all bodies on one VM share one tailnet IP. Fine for A2 (tool agents). Problem if one body needs unique network identity for firewall rules.

**Alternative**: Userspace Tailscale per container (no `/dev/net/tun` needed). Each container runs `tailscaled --tun=userspace` with SOCKS5 proxy. Higher memory (~10MB each) but true per-body identity. Viable for up to ~10 bodies per VM on a 2GB node (100MB for networking, 1.9GB for workloads).

## T5: Edge Cases

### EC1: No Tailscale — User Runs Without Networking
User doesn't configure Tailscale or headscale. `mesh.config.yaml` has `networking.enabled: false`.
- All `AssignIdentity` calls return `NOT_CONFIGURED` immediately.
- Bodies still run on substrate-native networking (Docker bridge, Nomad group network).
- Cross-substrate connectivity lost. Burst clone (A4) cannot reach parent (A1) directly.
- **Mitigation**: Document that networking is required for cross-substrate features. MCP tools that require connectivity (burst, migrate) should warn when networking is disabled.
- **Verdict**: Valid supported configuration. Some features degraded.

### EC2: Tailscale Auth Key Exhausted
Auth keys have limits (ephemeral keys: 100 devices). At scale, keys run out.
- `AssignIdentity` fails with `AUTHENTICATION_ERROR`.
- Bodies are created but unreachable.
- **Mitigation**: Networking module monitors key usage. Pre-generates new keys before exhaustion. Surfaces warning at 80% usage via MCP.

### EC3: Substrate Doesn't Support Tailscale (E2B, Cloudflare)
E2B Firecracker VMs don't expose `/dev/net/tun`. Cloudflare Containers are isolated.
- `AssignIdentity` detects missing capability → `ASM-3` violated.
- **Mitigation 1**: Userspace networking mode (`--tun=userspace`). Works without `/dev/net/tun`. Body runs a local SOCKS5 proxy. Mesh injects `ALL_PROXY=socks5://localhost:1055` into container env.
- **Mitigation 2**: Gateway proxy pattern. A Mesh-controlled gateway (on a Fleet VM with full Tailscale) proxies traffic to sandbox. Sandbox doesn't need Tailscale; it's reachable via gateway.
- **Mitigation 3**: Accept degraded mode. Body has no tailnet identity. Only accessible via substrate-native mechanisms (E2B SDK, CF Worker).

### EC4: Network Partition During Reassignment
Migration step 2e: old body revoked, new body can't reach Tailscale coordination server.
- Body is running on new substrate but has no tailnet IP.
- DNS still points to old (now revoked) IP.
- **Mitigation**: As established in T2 — NEVER revoke old device until new one confirms join. If new device fails to join within 60s, abort migration, keep old identity. The caller (Orchestration) handles abort.

### EC5: DNS TTL Causes Stale Routing After Migration
DNS `a1-hermes.mesh.local` points to old IP. Clients cache old IP for TTL duration.
- **Mitigation**: Use very short TTL (10s) for mesh DNS entries. After DNS update, wait TTL before revoking old device. For MagicDNS, Tailscale handles propagation automatically.

### EC6: Headscale vs Tailscale SaaS — Different API Surfaces
User might use headscale (self-hosted) or Tailscale SaaS. APIs differ.
- **Mitigation**: Abstract behind a `TailnetProvider` interface. Two implementations: `TailscaleSaaS` (uses OAuth + API) and `Headscale` (uses API key + gRPC). Selection in config.

## T6: Contract Verification

### Invariant Checklist

| Invariant | T1 | T2 | T3 | T4 | T5 | Status |
|-----------|----|----|----|----|-----|--------|
| INV-1: Running body has reachable DNS or error | ✅ | ❌ (Tailscale down → stuck in Assigning) | ✅ | ✅ | ❌ (no-TS mode returns NOT_CONFIGURED, not error) | BROKEN |
| INV-2: DNS name survives migration | ✅ | ✅ | ✅ | ✅ | ✅ | OK |
| INV-3: GetEndpoint returns reachable URL in 5s | ✅ | ❌ (DNS propagation delay) | ✅ | ✅ | ❌ (partition during reassignment) | BROKEN |
| INV-4: Identity revoked within 30s of Destroy | ✅ | ✅ | ✅ | ✅ | ✅ | OK |
| INV-5: Two bodies on same tailnet can connect | ✅ | ✅ | ✅ | ❌ (packed bodies on same VM share IP) | ✅ | BROKEN |
| INV-6: Networking optional, degrades gracefully | ✅ | N/A | N/A | N/A | ✅ | OK |

### Broken Invariants → Design Changes

**INV-1 BROKEN (T2)**: Tailscale down during assignment leaves body in "Assigning" state indefinitely.
→ **Fix**: Add assignment timeout (60s). If not completed, transition to `Failed` and return `NETWORK_ERROR` from `GetEndpoint`. Orchestration can retry later. Body is running but unreachable — surface as warning in MCP.

**INV-1 BROKEN (T5, no-TS mode)**: `NOT_CONFIGURED` is not an error in the strict sense.
→ **Fix**: Refine INV-1 to: "When networking is enabled, a running body has a reachable DNS name or an explicit error. When networking is disabled, GetEndpoint returns NOT_CONFIGURED (not an error)."

**INV-3 BROKEN (T2)**: DNS propagation delay after migration makes endpoint temporarily unreachable.
→ **Fix**: INV-3 should allow a propagation window. Refined: "GetEndpoint returns a URL that is reachable within 15 seconds of identity assignment or reassignment. During propagation, GetEndpoint may return previous IP." Alternatively: always return both old and new IP during reassignment transition, let clients try both.

**INV-5 BROKEN (T4)**: Multiple packed bodies on one VM share one tailnet IP. Port-based routing means A2 body "x" cannot be reached at a unique IP — it's `100.64.0.42:8081` vs `100.64.0.42:8082`. Two A2 bodies are technically reachable but don't have unique IPs.
→ **Fix**: Refined: "Two bodies on the same tailnet can establish TCP connectivity to each other's exposed ports. Unique IP per body is a best-effort guarantee, not a hard requirement." For packed bodies, use DNS-based routing (Caddy reverse proxy) instead of IP-based routing.

## Updated Interface

```typescript
interface NetworkIdentity {
  dnsName: string;           // Stable across migrations (e.g., "a1-hermes")
  ip: string;                // Current IP (may change on migration)
  tailnetIp: string | null;  // Tailscale IP, null if no tailnet
  url: string;               // Primary access URL
  reachable: boolean;        // Is endpoint currently reachable?
  mode: 'full' | 'userspace' | 'proxied' | 'disabled';
}

interface Network {
  AssignIdentity(bodyId: string, formId: string, capabilities: SubstrateCapabilities): Promise<NetworkIdentity>;
  ReassignIdentity(bodyId: string, newFormId: string, capabilities: SubstrateCapabilities): Promise<NetworkIdentity>;
  RevokeIdentity(bodyId: string): Promise<void>;
  GetEndpoint(bodyId: string, port?: number): Promise<string>;
  Connect(bodyA: string, bodyB: string): Promise<void>;  // Optional: install policy
  GetStatus(bodyId: string): Promise<NetworkStatus>;       // NEW: check reachability
}

enum NetworkStatus {
  UNASSIGNED = 'unassigned',
  ASSIGNING = 'assigning',
  ASSIGNED = 'assigned',
  REASSIGNING = 'reassigning',
  REVOKING = 'revoking',
  FAILED = 'failed',
  NOT_CONFIGURED = 'not_configured',
}
```

## Open Questions

- **OQ-N1**: **Shared vs per-body Tailscale on packed VMs** — Userspace networking per container (true identity, higher memory) vs per-VM proxy (lower memory, shared identity). Which is the default for A2 tool agents? — Matters for C1 (2GB VM constraint) and A2 packing density. — Options: (a) userspace per-container default, proxy as opt-in; (b) proxy default, userspace for identity-sensitive workloads; (c) make it a substrate adapter concern.

- **OQ-N2**: **DNS implementation** — MagicDNS (built into Tailscale, limited to `*.tailnet-name.ts.net`) vs Mesh-managed DNS overlay (CoreDNS/etcd, `*.mesh.local`, more flexible but more code). — Matters for INV-2 (DNS persistence) and operational complexity. — Options: (a) MagicDNS only, accept Tailscale naming constraints; (b) Mesh DNS overlay, MagicDNS as backend; (c) config-driven: user picks.

- **OQ-N3**: **SSH gateway implementation** — Tailscale's built-in SSH (zero-config, uses Tailscale identity) vs Mesh-provided SSH gateway (like Daytona, token-based, works without Tailscale). — Matters for EC1 (no-Tailscale scenario). — Options: (a) Tailscale SSH only, requires Tailscale; (b) Mesh SSH gateway always, independent of Tailscale; (c) Tailscale SSH when available, fallback to Mesh gateway.

- **OQ-N4**: **Cross-module gap with Orchestration** — Migration step 2e (Networking.AssignIdentity) happens AFTER step 2c (Provisioning.Provision) and step 2d (Persistence.Restore). If Networking fails, the body is running with restored state but unreachable. Who decides to abort vs continue? Orchestration? Interface? This is a **cross-module design gap** that needs resolution in migration orchestration protocol. Currently undefined in SYSTEM.md.
