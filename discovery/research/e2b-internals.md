# Research: E2B Firecracker Sandbox Internals

> Completed: 2026-04-23
> Source: E2B Documentation (e2b.dev/docs), GitHub repos (e2b-dev/e2b, e2b-dev/infra), pricing pages, blog posts, and competitive analysis

## Architecture

E2B is built on **Firecracker microVMs** for hardware-level isolation, orchestrated by a Go-based control plane.

**Core Components**:

1. **Firecracker MicroVMs** - Each sandbox is a lightweight VM with:
   - Dedicated Linux kernel (LTS 6.1.x, fixed at template build time)
   - Root filesystem using **OverlayFS** for copy-on-write efficiency
   - Jailer process wrapping for cgroups, namespaces, and seccomp filters
   - Boot time: ~150ms (cold), ~80ms (same-region)
   
   Evidence from E2B's Firecracker fork:
   - https://github.com/e2b-dev/firecracker (fork of firecracker-microvm/firecracker)

2. **Orchestrator** (`packages/orchestrator/` in e2b-dev/infra):
   - Go service managing Firecracker lifecycle
   - Requires sudo for `/dev/kvm` access, TAP device creation, cgroup management, NBD mounting
   - Uses gRPC/Connect RPC for service communication
   - Port: 5008 (local), deployed via Nomad on GCP/AWS
   
   Evidence: https://github.com/e2b-dev/infra/blob/main/CLAUDE.md

3. **Envd Daemon** - In-VM agent running on port 49983:
   - Handles filesystem operations, process execution, terminal management
   - Hybrid protocol: gRPC (metadata) + HTTP (bulk data transfer)
   - Protocol definitions in `spec/envd/` protobuf files
   
   Evidence: https://deepwiki.com/e2b-dev/E2B/6.2-grpc-and-envd-protocol

4. **Control Plane Services**:
   - PostgreSQL: Primary data (users, teams, templates)
   - Redis: Caching, pub/sub, orchestrator coordination
   - ClickHouse: Analytics and sandbox metrics
   - Client Proxy: Traffic routing (port 3002)
   - API: REST endpoints for sandbox lifecycle (port 3000)

   Evidence: https://github.com/e2b-dev/infra/blob/main/DEV-LOCAL.md

**Data Flow**:
```
Client SDK → Client Proxy → API (REST) ⟷ PostgreSQL
                      ↓              ⟷ Redis
                   Orchestrator     ⟷ ClickHouse
                      ↓ (gRPC)
                   Firecracker VMs
                      ↓
                   Envd (in-VM daemon)
```

## Sandbox Lifecycle

**States**: Running → Paused → Running, Running → Snapshotting → Running, Any → Killed (terminal)

**Operations**:

| Operation | Effect | Timing | API |
|-----------|--------|--------|-----|
| `Sandbox.create()` | Creates new sandbox from template or snapshot | ~150ms cold | SDK |
| `sandbox.pause()` | Saves FS + memory, stops execution | ~4s per 1GB RAM | SDK |
| `Sandbox.resume()` | Restores FS + memory, resumes execution | ~1s | SDK |
| `sandbox.kill()` | Terminates sandbox, releases resources | Immediate | SDK |
| `sandbox.createSnapshot()` | Creates FS+memory checkpoint, sandbox continues | Brief pause | SDK |
| `Sandbox.connect()` | Connect to running/paused sandbox | Immediate | SDK |

**Timeout Behavior**:
- Default timeout: 5 minutes (resets on each connect)
- Auto-pause: Set `lifecycle.onTimeout = 'pause'` for automatic pause on timeout
- Auto-resume: Set `lifecycle.autoResume = true` for auto-wake on activity
- Paused sandboxes kept **indefinitely** (no TTL)

Evidence: https://e2b.dev/docs/sandbox/persistence

**Max Runtime** (continuous, without pause):
- Hobby: 1 hour
- Pro: 24 hours
- After pause/resume, continuous runtime limit **resets**

## Snapshot Mechanics

**What Snapshots Capture**: Both filesystem AND memory state
- Running processes
- Loaded variables
- In-memory data
- Filesystem state

**Requirements**: 
- Templates with envd version `v0.5.0` or above
- Snapshot briefly pauses sandbox, then auto-resumes
- All active connections (WebSocket, PTY, command streams) **dropped** during snapshot

Evidence: https://e2b.dev/docs/sandbox/snapshots

**Snapshot vs Pause/Resume**:

| Aspect | Pause/Resume | Snapshots |
|--------|--------------|-----------|
| Effect on original | Stops sandbox | Briefly pauses, continues |
| Relationship | One-to-one (same sandbox) | One-to-many (can spawn many) |
| Use case | Suspend/resume single sandbox | Checkpoint, rollback, fork |
| API | `sandbox.pause()`, `Sandbox.resume()` | `sandbox.createSnapshot()`, `Sandbox.create(snapshotId)` |

**Template vs Snapshot**:

| Aspect | Templates | Snapshots |
|--------|-----------|-----------|
| Defined by | Declarative code (Template builder) | Capturing running sandbox |
| Reproducibility | Same definition = same sandbox | Captures whatever state exists |
| Best for | Repeatable base environments | Checkpointing runtime state |

Evidence: https://e2b.dev/docs/sandbox/snapshots

## Filesystem Access

**No Docker Export Equivalent**: E2B does **not** provide a docker export-like operation to extract the entire filesystem as a tarball.

**Available Operations** (via SDK `sandbox.files`):

| Operation | Method | Notes |
|-----------|--------|-------|
| Read file | `files.read(path)` | Returns text/bytes/stream |
| Write file | `files.write(path, data)` | Creates if doesn't exist |
| List entries | `files.list(path)` | Returns file/directory metadata |
| Watch directory | `files.watch(path)` | Subscribe to changes |
| Upload multiple | `files.write([{path, content}])` | Batch upload, no directory support |
| Pre-signed upload | `sandbox.uploadUrl(path)` | For browser/unauthorized clients |

**Limitations**:
- **No easy directory upload/download**: Must upload/download files individually
- **No recursive operations**: Each file must be handled separately
- **No filesystem export**: Cannot extract full FS as tarball
- Workaround: "We're working on a better solution" (as of 2026)

Evidence: https://e2b.dev/docs/quickstart/upload-download-files

**Storage**:
- Hobby: 10 GB disk space
- Pro: 20+ GB disk space (customizable)
- No persistent volumes
- Ephemeral by design (unless using pause/resume or snapshots)

Evidence: https://e2b.dev/docs/filesystem

## Resource Model & Limits

**Compute Configuration**:

| Resource | Hobby | Pro | Enterprise |
|----------|-------|-----|------------|
| vCPUs | 1-8 | 8+ (custom) | Custom |
| RAM | 512MB - 8GB | 8GB+ (custom) | Custom |
| Disk | 10 GB | 20+ GB | Custom |
| Max session length | 1 hour | 24 hours | Custom |
| Concurrent sandboxes | 20 | 100 - 1,100 | 1,100+ |
| Sandbox creation rate | 1/sec | 5/sec | Custom |
| Base price | $0/mo | $150/mo | Custom |
| Free credits | $100 (one-time) | $100 (one-time) | Custom |

Evidence: https://e2b.dev/docs/billing

**Custom Resources** (Pro only):
```javascript
await Template.build(template, 'my-template', {
  cpuCount: 8,        // vCPUs
  memoryMB: 8192,     // RAM in MB
})
```

**Kernel Versions** (fixed at template build time):
- Templates built ≥ 27.11.2025: Linux 6.1.158
- Templates built < 27.11.2025: Linux 6.1.102

Evidence: https://e2b.dev/docs/template/how-it-works

## Networking

**Internet Access**:
- Default: Enabled (`allowInternetAccess: true`)
- Can disable for security: `allowInternetAccess: false`
- Disabling = setting `network.denyOut = ['0.0.0.0/0']` (deny all traffic)

**Granular Control** (via `network` config):
- `allowOut`: List of allowed domains/IPs/CIDRs
- `denyOut`: List of blocked destinations
- `ALL_TRAFFIC` constant = `0.0.0.0/0` (all IPs)

**Domain Filtering**:
- Works for: HTTP (port 80) via Host header, HTTPS (port 443) via SNI inspection
- Does NOT work for: Other ports, UDP/QUIC/HTTP3
- Domains NOT supported in `denyOut` list

Evidence: https://e2b.dev/docs/sandbox/internet-access

**Sandbox Public URL**:
- Every sandbox gets a public URL
- Accessible from outside for services running inside
- Can restrict with `allowPublicTraffic: false` (requires auth)
- Custom `Host` header via `maskRequestHost` option

**Network Behavior During Pause/Resume**:
- Services become inaccessible when paused
- All clients disconnected
- Must reconnect clients after resume

Evidence: https://e2b.dev/docs/sandbox/persistence

**Advanced Networking**:
- Proxy tunneling via Shadowsocks (custom template)
- VPC peering for BYOC deployments
- Private network traffic stays within VPC

Evidence: https://e2b.dev/docs/sandbox/ip-tunneling

## SDK/API Surface

**SDKs Available**:
- JavaScript/TypeScript: `npm i e2b` (v2.19.0 as of 2026-04-02)
- Python: `pip install e2b` (v2.20.0 as of 2026-04-02)
- CLI: `e2b` command-line tool

Evidence: https://github.com/e2b-dev/E2B/releases

**Main SDK Classes**:

```javascript
// Create sandbox
const sandbox = await Sandbox.create(templateId, {
  timeoutMs: 10 * 60 * 1000,
  allowInternetAccess: false,
  network: { denyOut: [ALL_TRAFFIC] },
  lifecycle: {
    onTimeout: 'pause',
    autoResume: true,
  },
})

// Commands
const result = await sandbox.commands.run('echo "hello"')

// Filesystem
await sandbox.files.write('/path', content)
const content = await sandbox.files.read('/path')

// Snapshots
const snapshot = await sandbox.createSnapshot()
const newSandbox = await Sandbox.create(snapshot.snapshotId)

// Pause/Resume
await sandbox.pause()
await Sandbox.resume(sandbox.sandboxId)

// Lifecycle
await sandbox.kill()
```

**API Endpoints**:

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `api.e2b.app/sandboxes` | POST | Create sandbox |
| `api.e2b.app/sandboxes/{id}/pause` | POST | Pause sandbox |
| `api.e2b.app/sandboxes/{id}/resume` | POST | Resume sandbox |
| `api.e2b.app/sandboxes/{id}/kill` | POST | Kill sandbox |
| `api.e2b.app/snapshots` | POST | Create snapshot |
| `api.e2b.app/events/sandboxes` | GET | Lifecycle events |

Evidence: https://e2b.dev/docs/api-reference/sandboxes/pause-sandbox

**Envd API** (in-VM, port 49983):
- gRPC services: Filesystem, Process (via Connect RPC)
- HTTP endpoints: File read/write (bulk data)
- Protocol: Protocol Buffers in `spec/envd/` directory

Evidence: https://deepwiki.com/e2b-dev/E2B/6.2-grpc-and-envd-protocol

## Limitations for Mesh

**Critical Limitations** (vs Mesh requirements):

1. **No Docker Export Equivalent**
   - Mesh D2 requires: `docker export | zstd` tarball extraction
   - E2B cannot extract full filesystem as tarball
   - Only file-by-file read/write via SDK
   - **Blocking for Mesh migration**

2. **Cannot Import Custom Filesystem Tarball**
   - Mesh D4 requires: instantiate → import tarball → start
   - E2B can only create sandboxes from templates or snapshots
   - No API to bootstrap a sandbox from an external tarball
   - **Blocking for Mesh body import**

3. **Snapshot Captures Memory (Violates D1)**
   - Mesh D1: Filesystem-only snapshots, no memory
   - E2B snapshots: FS + memory + running processes
   - Memory capture adds complexity, security surface
   - **Partially compatible** (can ignore memory if not needed)

4. **No GPU Support**
   - As of early 2026: "E2B does not support GPU-equipped sandboxes"
   - No ML model training, GPU inference, CUDA-dependent code
   - **May not be blocking** if Mesh doesn't require GPU

5. **Max Runtime Limits**
   - Hobby: 1 hour
   - Pro: 24 hours
   - Cannot run indefinitely without pause/resume cycles
   - **Potentially blocking** for long-running agents

6. **No Persistent Volumes**
   - Sandboxes are ephemeral by design
   - Must use pause/resume or snapshots for persistence
   - **Potentially blocking** for persistent agent bodies

7. **Pause/Resume Bug (Historical)**
   - Issue #736: Resume failed on second attempt after pause/resume cycles
   - Status: Closed, marked as fixed
   - **Not blocking** (claimed fixed, but worth verifying)

Evidence: https://github.com/e2b-dev/E2B/issues/736

**Non-Blocking Limitations**:

1. **No Directory Upload/Download**
   - Must handle files individually
   - Workaround: Implement recursive upload/download in adapter

2. **BYOC Only for Enterprise**
   - AWS/GCP self-hosting available, but requires enterprise contract
   - Mesh could potentially use self-hosted E2B on local/fleet substrates
   - But requires managing Nomad + Firecracker infrastructure

Evidence: https://e2b.dev/docs/byoc

## Cost Model

**Usage-Based Pricing** (per second of running sandbox):

| Resource | Cost | Notes |
|----------|------|-------|
| vCPU | $0.000014/s (~$0.05/hour) | Default: 2 vCPU |
| RAM | $0.0000045/GiB/s (~$0.018/hour) | Default: 512 MiB |
| Storage | Free | 10 GB (Hobby), 20 GB (Pro) |
| Snapshots | Not specified | Likely billed as storage |

**Plan Pricing**:

| Tier | Base Price | Includes |
|------|------------|----------|
| Hobby | $0/mo | $100 one-time credits, 1h sessions, 20 concurrent |
| Pro | $150/mo | $100 credits, 24h sessions, 100 concurrent, custom CPU/RAM |
| Enterprise | Custom | BYOC, custom limits, dedicated support |

**Example Costs** (2 vCPU, 512 MiB RAM):
- $0.000028/s + $0.00000225/s = $0.00003025/s
- $0.109/hour
- $2.61/day (24 hours)
- $78.30/month (continuous 24/7)

**Comparison** (for 30s run):
- E2B: ~$0.005/run
- Modal: ~$0.0016/run
- Fly Machines: ~$0.0055/run
- Daytona: ~$0.006+/run

Evidence: https://awesomeagents.ai/pricing/agent-platform-pricing/

## Competitive Position

**vs Fly Machines**:

| Aspect | E2B | Fly Machines |
|--------|-----|--------------|
| Isolation | Firecracker microVM | Docker containers (shared kernel) |
| Cold start | ~80-150ms | 10-300ms (pre-created), 2-10s (reactive) |
| Max runtime | 1-24h | Unlimited |
| GPU support | ❌ No | ✅ Yes |
| Cost | $0.0828/hour (2vCPU) | ~$0.003/hour (usable) |
| Security | Hardware-level (VM) | Container-level (weaker) |
| Purpose | Ephemeral AI agents | General-purpose apps |
| Persistent storage | ❌ No | ✅ Sprites (100GB NVMe) |

**Verdict**: E2B is faster and more secure for untrusted code, but 30x more expensive. Fly Machines better for cost-sensitive, long-running workloads.

Evidence: https://bertomill.medium.com/e2b-vs-fly-machines

**vs Modal**:

| Aspect | E2B | Modal |
|--------|-----|-------|
| Primary use case | General AI agents | Python ML workloads |
| SDK languages | JS/TS, Python | Python, JS/TS (beta), Go (beta) |
| Cold start | ~150ms | Sub-second |
| GPU support | ❌ No | ✅ Yes |
| Cost | $0.0828/hour | $0.1193/hour |
| Pricing model | Per-second | Per-second |

**Verdict**: Modal better for ML/GPU workloads, E2B better for general-purpose AI code execution.

**vs Daytona**:

| Aspect | E2B | Daytona |
|--------|-----|---------|
| Cold start | ~150ms | ~90ms |
| Max runtime | 24h (Pro) | Unlimited |
| Pricing | Clear, per-second | Sparse, requires sales contact |
| Free tier | $100 credits | 2 workspaces |
| SDK languages | JS/TS, Python | Python, TypeScript |

**Verdict**: Daytona faster and potentially cheaper for dev environments, but E2B has clearer pricing for production.

Evidence: https://www.superagent.sh/blog/ai-code-sandbox-benchmark-2026

## Key Findings

**F1: E2B Snapshots Capture Memory, Violating Mesh D1**
- E2B snapshots: FS + memory + running processes
- Mesh D1: Filesystem-only snapshots
- Incompatibility: Memory capture adds security risk and complexity

**F2: No Docker Export Equivalent, Blocking Mesh D2/D4**
- Mesh requires: `docker export | zstd` for body migration
- E2B cannot extract full filesystem as tarball
- Only file-by-file read/write available
- **Blocking**: Cannot implement Mesh body = OCI + tarball pattern

**F3: Cannot Import Custom Filesystem Tarball**
- Mesh D4: instantiate → import tarball → start
- E2B only creates from templates or snapshots
- No API to bootstrap from external tarball
- **Blocking**: Cannot restore a Mesh body onto E2B sandbox

**F4: OverlayFS for Efficient Storage**
- E2B uses OverlayFS copy-on-write for root filesystem
- Shares base image across multiple instances
- Saves disk space and time
- **Potential optimization** for Mesh if using E2B

**F5: Firecracker Provides Strong Isolation**
- Hardware-level isolation via dedicated kernel per sandbox
- Jailer process adds cgroups, namespaces, seccomp
- Better than containers for untrusted code
- **Aligns with Mesh security requirements**

**F6: Pause/Resume Performance Characteristics**
- Pause: ~4 seconds per 1GB RAM
- Resume: ~1 second (fixed cost)
- Paused sandboxes kept indefinitely
- **Acceptable for Mesh cold migration** (D4)

**F7: No Persistent Volumes, Ephemeral by Design**
- Must use pause/resume or snapshots for persistence
- Not ideal for long-lived agent bodies
- **Partially compatible** with Mesh cold migration pattern

**F8: BYOC Available But Enterprise-Only**
- AWS/GCP self-hosting possible via e2b-dev/infra
- Requires managing Nomad + Firecracker + infrastructure
- Not suitable for Mesh fleet pool (which targets 2GB VMs)

**F9: Network Filtering Has Limitations**
- Domain filtering only works for HTTP (80) and HTTPS (443)
- UDP/QUIC/HTTP3 not supported
- `denyOut` doesn't support domains
- **Acceptable** for most Mesh use cases

**F10: Pricing is Premium vs Alternatives**
- $0.0828/hour for 2vCPU (30x Fly Machines)
- Designed for short-lived AI agent workloads
- **Expensive** for always-on agent bodies

## Verdict

**E2B is NOT viable as a Mesh substrate in its current form.**

**Critical Blockers**:
1. **No docker export equivalent** (violates D2)
2. **Cannot import custom filesystem tarball** (violates D4)
3. **Snapshots capture memory** (violates D1)
4. **No persistent volumes** (incompatible with long-lived bodies)

**Adapter Verbs Supported** (with limitations):
- ✅ `create` (from template only, not from tarball)
- ✅ `pause` (~4s per 1GB RAM)
- ✅ `resume` (~1s)
- ✅ `kill`
- ❌ `export` (cannot extract full FS as tarball)
- ❌ `import` (cannot import tarball to sandbox)
- ⚠️ `snapshot` (captures memory, not FS-only)
- ⚠️ `migrate` (possible via pause/resume, but no export/import)

**Recommendation**:
- **Do NOT integrate E2B as a Mesh substrate** in v1
- Monitor E2B for future API additions (filesystem export, import tarball)
- Consider E2B as a reference architecture for Firecracker-based isolation
- Use E2B's self-hosting docs if building a similar fleet substrate

**Potential Future Work** (if E2B adds needed APIs):
1. Build E2B adapter using file-by-file export (slow but functional)
2. Leverage E2B BYOC for fleet-like deployments (enterprise customers only)
3. Use E2B snapshots as-is if memory capture is acceptable
4. Optimize for E2B's OverlayFS efficiency in fleet design

**Sources**:
- E2B Documentation: https://e2b.dev/docs
- E2B GitHub: https://github.com/e2b-dev/E2B
- E2B Infra: https://github.com/e2b-dev/infra
- Pricing: https://e2b.dev/pricing
- Comparison: https://www.superagent.sh/blog/ai-code-sandbox-benchmark-2026
