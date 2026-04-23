# Research: Substrate Adapter Interface

> Completed: 2026-04-23
> Source: substrate-landscape.md, daytona-analysis.md, agent-sandbox-k8s.md, Docker/Nomad/E2B/Fly/Modal/CF docs

## Per-Substrate Lifecycle Survey

### Docker (local)

**Exact lifecycle verbs:**
- `POST /containers/create` - Create container
- `POST /containers/{id}/start` - Start container
- `POST /containers/{id}/stop` - Stop container (graceful, SIGTERM then SIGKILL)
- `POST /containers/{id}/kill` - Kill container (force, SIGKILL)
- `POST /containers/{id}/restart` - Restart container
- `POST /containers/{id}/pause` - Pause container
- `POST /containers/{id}/unpause` - Unpause container
- `DELETE /containers/{id}` - Remove container
- `POST /containers/{id}/exec` - Create exec instance
- `POST /exec/{id}/start` - Start exec command
- `GET /containers/{id}/export` - Export filesystem as tarball stream
- `POST /images/create` - Import from tarball (docker import)
- `POST /containers/{id}/archive` - Copy files (cp)

**State transitions:**
- `created` → `running` → `paused` → `running`
- `running` → `exited` (on stop/kill)
- `exited` → `running` (on start)
- `running` → `dead` (on crash)
- Any state → `removed` (on rm)

**Snapshot/export mechanism:**
- `docker export <container_id>` - Streams flat tarball of container's filesystem
- No memory state captured
- Produces POSIX-compliant tarball
- Captures all files from container root

**Import/restore mechanism:**
- `docker import <tarball> <image_name>` - Creates image from tarball
- `docker create --name <name> <image>` - Creates new container from imported image
- Container starts with imported filesystem state

**Resource specification:**
- CPU: `NanoCpus` (integer, units of 10^-9 CPUs)
- Memory: `Memory` (bytes), `MemoryReservation`, `MemorySwap`, `MemorySwappiness`
- Disk: Not specified (uses host filesystem)
- PIDs: `PidsLimit`
- Ulimits: Array of `{Name, Soft, Hard}`

**Networking:**
- NetworkMode: `bridge`, `host`, `none`, `container:<id|name>`, or custom network name
- PortBindings: Map container ports to host ports
- DNS: Custom DNS servers
- ExtraHosts: Add host-to-IP mappings

**Unique constraints:**
- Most portable substrate (runs on any Linux host with Docker)
- OCI image format is universal
- Export/import is filesystem-only, no memory state
- Container ID is SHA256 hash of config
- Volume mounts persist beyond container lifecycle

**Missing capabilities:**
- No native suspend/resume (pause/unpause pauses processes but doesn't free memory)
- No built-in snapshots with memory state
- No automatic scaling or resource management
- No multi-host orchestration (need Swarm or K8s)

---

### Nomad

**Exact lifecycle verbs:**
- `POST /v1/jobs` - Submit job
- `DELETE /v1/job/{job_id}` - Stop job
- `POST /v1/allocation/{alloc_id}/stop` - Stop specific allocation
- `WebSocket /v1/client/allocation/{alloc_id}/exec` - Execute command in allocation
- Job-based: `nomad job run`, `nomad job stop`
- Allocation-based: `nomad alloc stop`, `nomad alloc exec`, `nomad alloc signal`

**State transitions:**
- `pending` → `running` → `complete` (successful exit)
- `pending` → `running` → `failed` (error exit)
- `running` → `lost` (node lost)
- `running` → `dead` (explicit stop)
- Job states: `pending`, `running`, `complete`, `failed`, `lost`

**Snapshot/export mechanism:**
- No native Nomad snapshot primitive
- Depends on task driver:
  - Docker driver: Can use `docker export` on the container
  - raw_exec driver: No filesystem isolation, no snapshot
- FS access via `nomad alloc exec` for manual tarball creation

**Import/restore mechanism:**
- Depends on task driver:
  - Docker driver: `docker import` + new job/alloc
  - raw_exec driver: Manual FS restoration
- No automatic restore mechanism

**Resource specification:**
- CPU: `CPU` (MHz), `Cores` (integer)
- Memory: `MemoryMB` (MiB)
- Disk: `DiskMB` (MiB) - for task driver
- Networks: `MBits` (bandwidth)
- GenericResources: Custom resources (GPU, SSD, etc.)

**Networking:**
- Network modes: `host`, `group`, `task` (driver-dependent)
- Dynamic ports: `DynamicPorts` in task group
- DNS: Consul integration for service discovery
- CNI plugins for custom networking

**Unique constraints:**
- Job scheduler, not container runtime
- Delegates isolation to task drivers (Docker, raw_exec, exec, etc.)
- Allocations are first-class entities (groups of tasks)
- Supports constraints, affinities, spread
- Multi-region deployment support

**Missing capabilities:**
- No native snapshot/export across all drivers
- No built-in suspend/resume
- No portable state between jobs
- Filesystem isolation varies by driver

---

### E2B

**Exact lifecycle verbs:**
- `Sandbox.create()` - Create new sandbox
- `POST /sandboxes/{sandboxID}/pause` - Pause sandbox
- `POST /sandboxes/{sandboxID}/resume` - Resume sandbox
- `POST /sandboxes/{sandboxID}/kill` - Kill sandbox
- `Snapshot.create()` - Create snapshot from running sandbox
- `Sandbox.connect()` - Reconnect to existing sandbox

**State transitions:**
- `running` → `paused` (on pause)
- `paused` → `running` (on resume or auto-resume)
- `running/paused` → `killed` (on kill)
- `paused` → `running` (auto-resume with `autoResume: true`)

**Snapshot/export mechanism:**
- `Snapshot.create(sandbox_id)` - Captures FS + memory state
- Snapshot is brief pause (~1s) then sandbox continues running
- One-to-many: Single snapshot can spawn many new sandboxes
- Full Firecracker VM snapshot (memory + filesystem)

**Import/restore mechanism:**
- `Sandbox.create({ snapshotID: <id> })` - Spawn new sandbox from snapshot
- New sandbox starts with exact FS and memory state from snapshot
- Original sandbox unchanged

**Resource specification:**
- CPU: `cpuCount` (vCPUs, 1-100 depending on tier)
- Memory: `memoryMB` (MiB, 1024-100000 depending on tier)
- Disk: `diskSizeGB` (GiB, 1-1000 depending on tier)
- GPU: Not available in base tier

**Networking:**
- SSH access: `ssh -p <port> root@<host>`
- Port forwarding: Expose ports for web services
- Outbound: Full internet access
- No custom networking or VPN

**Unique constraints:**
- Firecracker microVM isolation
- Memory+FS snapshots (portable within E2B)
- Auto-resume feature for paused sandboxes
- Lifecycle API for event tracking
- Per-second billing

**Missing capabilities:**
- Snapshots not portable outside E2B platform
- No cold migration (stop, export, import elsewhere)
- No substrate-level resource limits
- No custom networking or VPN

---

### Fly.io Machines

**Exact lifecycle verbs:**
- `POST /v1/apps/{app_name}/machines` - Create machine
- `POST /v1/apps/{app_name}/machines/{machine_id}/start` - Start machine
- `POST /v1/apps/{app_name}/machines/{machine_id}/stop` - Stop machine
- `POST /v1/apps/{app_name}/machines/{machine_id}/suspend` - Suspend machine
- `POST /v1/apps/{app_name}/machines/{machine_id}/restart` - Restart machine
- `DELETE /v1/apps/{app_name}/machines/{machine_id}` - Destroy machine
- CLI: `fly machine suspend`, `fly machine start`, `fly machine stop`

**State transitions:**
- `started` → `stopped` (on stop)
- `started` → `suspended` (on suspend)
- `stopped` → `started` (cold start)
- `suspended` → `started` (resume from snapshot)
- `suspended` → `stopped` (discard snapshot)
- `started/stoppped/suspended` → `destroyed` (on delete)

**Snapshot/export mechanism:**
- Suspend creates VM snapshot (memory + FS)
- Snapshot stored on Fly infrastructure
- Automatic on suspend, manual via API
- Discarded on stop or deploy

**Import/restore mechanism:**
- Start on suspended machine resumes from snapshot
- Falls back to cold start if snapshot unavailable
- Snapshots not exportable (Fly platform-bound)
- Auto-resume via Fly Proxy if enabled

**Resource specification:**
- CPU: `cpus` (1-16 vCPUs)
- Memory: `memory_mb` (256-65536 MiB)
- Disk: Volume size (persistent)
- GPU: `gpus` (if available)
- Constraints: Suspend requires ≤2 GiB memory, no GPUs, no schedules

**Networking:**
- Private IPv6 address per machine
- Public IPv6 (optional)
- Services with port bindings
- Fly Proxy for load balancing
- Tailscale integration (optional)

**Unique constraints:**
- Firecracker VM isolation
- Suspend/resume with memory state (hundreds of ms)
- Snapshots discarded on host migration or deploy
- Global regional deployment
- Per-second billing

**Missing capabilities:**
- Snapshots not portable outside Fly
- No export/import to other platforms
- Suspend limited to 2 GiB memory machines
- No manual snapshot control (implicit in suspend)

---

### Modal

**Exact lifecycle verbs:**
- `Sandbox.create(app, image, ...)` - Create sandbox
- `sandbox.exec(command, ...)` - Execute command
- `sandbox.terminate()` - Terminate sandbox
- `sandbox.detach()` - Detach from sandbox
- `Sandbox.from_id(id)` - Reattach to existing sandbox
- Functions: `@app.function`, `@app.method`, stub functions

**State transitions:**
- Implicit - sandbox created on first operation
- `running` → `terminated` (on terminate or timeout)
- Idle timeout can auto-terminate
- No explicit pause/resume states

**Snapshot/export mechanism:**
- Filesystem snapshots (read-only templates)
- Created via `sandbox.snapshot()`
- Read-only - cannot be modified
- Used as base for new sandboxes

**Import/restore mechanism:**
- `Sandbox.create({ snapshot: <snapshot_id> })` - Create from snapshot
- Snapshot is template, not full state
- Does not capture memory state
- Fan-out: One snapshot spawns many sandboxes

**Resource specification:**
- CPU: `cpus` (1-16 vCPUs)
- Memory: `memory` (MiB)
- Disk: `disk` (GiB) - ephemeral
- GPU: `gpu` (type, count)
- Timeout: `timeout` (max 24h)
- Idle timeout: `idle_timeout` (auto-terminate on inactivity)

**Networking:**
- Tunnels: Expose local services
- Private networking between Modal apps
- Web endpoints: `@app.web_endpoint`
- No custom network configuration

**Unique constraints:**
- gVisor container isolation
- CRIU for memory snapshots (experimental)
- Implicit lifecycle (create-on-demand)
- Read-only snapshots as templates
- Fan-out from snapshots

**Missing capabilities:**
- No explicit pause/resume
- No portable snapshots (Modal-bound)
- No memory state in snapshots (read-only only)
- No custom networking control
- No persistent disks (volumes separate)

---

### Cloudflare Containers

**Exact lifecycle verbs:**
- `container.start()` - Start container
- `container.startAndWaitForPorts()` - Start and wait for ports
- `container.stop(signal)` - Stop container (SIGTERM default)
- `container.destroy()` - Force destroy (SIGKILL)
- `container.renewActivityTimeout()` - Reset sleep timer
- `container.setKeepAlive()` - Enable/disable auto-sleep

**State transitions:**
- `not created` → `starting` → `running`
- `running` → `sleeping` (after inactivity timeout)
- `sleeping` → `running` (on next request - auto-resume)
- `running/sleeping` → `destroyed` (on destroy)
- `sleepAfter` timer resets on activity

**Snapshot/export mechanism:**
- No native snapshot/export
- Filesystem is ephemeral (resets on sleep)
- Backup/restore API exists but not snapshot-style
- Persistent storage via external mounts (R2)

**Import/restore mechanism:**
- No import mechanism
- Persistent state requires external storage (R2 buckets)
- Backup API for workspace state
- Restore from backup

**Resource specification:**
- CPU: Not specified (limits account-based)
- Memory: Not specified (limits account-based)
- Disk: Ephemeral (container image only)
- Limits: Active CPU pricing, concurrency limits

**Networking:**
- Worker-to-Container communication via fetch
- Container-to-Container via private network
- Expose services via Workers
- Outbound internet access
- No custom network configuration

**Unique constraints:**
- Durable Object per container (identity)
- Auto-sleep after inactivity (configurable)
- Auto-resume on next request
- Ephemeral disk (resets on sleep)
- Worker as control plane

**Missing capabilities:**
- No snapshot/export
- No persistent filesystem (ephemeral only)
- No memory state capture
- No explicit pause/resume (sleep is implicit)
- No custom resource limits

---

### Daytona

**Exact lifecycle verbs:**
- `daytona create` - Create workspace
- `daytona start <workspace>` - Start workspace
- `daytona stop <workspace>` - Stop workspace
- `daytona delete <workspace>` - Delete workspace
- `daytona snapshot create <workspace>` - Create snapshot
- `daytona workspace list` - List workspaces

**State transitions:**
- `created` → `starting` → `started`
- `started` → `stopping` → `stopped`
- `stopped` → `started` (on start)
- Any state → `destroyed` (on delete)
- Archive state for long-term storage

**Snapshot/export mechanism:**
- OCI-compliant container images
- Stored in S3-compatible object storage
- Filesystem-only (no memory state)
- Created from: public images, local images, Dockerfiles

**Import/restore mechanism:**
- Snapshot ID used in workspace creation
- No direct export to tarball
- Requires Daytona control plane
- Cross-region snapshot availability

**Resource specification:**
- CPU: 1-100 vCPU (tier-dependent)
- Memory: 1-100 GiB (tier-dependent)
- Disk: 1-1000 GiB (tier-dependent)
- Runner: 4 CPU / 8 GB RAM minimum

**Networking:**
- Native per-sandbox network stack
- VPN integration: Tailscale, OpenVPN
- SSH gateway with token auth
- Port forwarding for services
- Firewall rules per sandbox

**Unique constraints:**
- Control plane + compute plane architecture
- PostgreSQL metadata store
- Three-tier lifecycle (running/stopped/archived)
- Per-second compute billing
- Volume-based persistent storage

**Missing capabilities:**
- No standalone migration format
- No memory state in snapshots
- Platform-bound (requires control plane)
- No `docker export | zstd` equivalent
- Too heavy for 2GB VMs (8GB+ minimum)

---

### Kubernetes (agent-sandbox reference)

**Exact lifecycle verbs:**
- `Sandbox` CRD: create, update, delete
- `SandboxTemplate` CRD: define template
- `SandboxClaim` CRD: claim from template
- `SandboxWarmPool` CRD: pre-warm pods
- `replicas: 0|1` - Scale to zero or one

**State transitions:**
- `Pending` → `Running` → `Succeeded`/`Failed`
- `Running` → `Terminating` (on replicas=0)
- `Terminating` → `Terminated` (pod deleted, PVC retained)
- PVC survives pod deletion
- Service + CR persist for identity

**Snapshot/export mechanism:**
- PVC-based state (no native snapshot in CRD)
- CSI VolumeSnapshot (planned, not implemented)
- No image-commit flow
- No portable tarball export

**Import/restore mechanism:**
- PVC clone via CSI VolumeSnapshot
- Not implemented in agent-sandbox
- Requires CSI driver support
- No cross-cluster migration in CRD

**Resource specification:**
- CPU: `resources.requests.cpu`, `resources.limits.cpu`
- Memory: `resources.requests.memory`, `resources.limits.memory`
- Disk: PVC size (via storage class)
- GPU: `nvidia.com/gpu` resource

**Networking:**
- Service (ClusterIP, LoadBalancer)
- Ingress for external access
- NetworkPolicy (default-deny with Managed policy)
- CNI plugin for actual networking
- DNS via cluster DNS

**Unique constraints:**
- CRD-based control plane
- Singleton pod with stable identity
- Headless Service for DNS
- Scale-to-zero via replicas=0
- Template/Claim separation for security

**Missing capabilities:**
- No portable snapshot format
- No memory state capture
- K8s dependency (not portable)
- No cross-substrate migration
- PVC resume not implemented

---

## Common Verbs

Operations present in **ALL** substrates:

1. **create** - Create a new workload instance
   - Docker: `POST /containers/create`
   - Nomad: `POST /v1/jobs`
   - E2B: `Sandbox.create()`
   - Fly.io: `POST /v1/apps/{app}/machines`
   - Modal: `Sandbox.create()`
   - CF Containers: Implicit on first operation
   - Daytona: `daytona create`
   - K8s agent-sandbox: Create `Sandbox` CRD

2. **start** - Start a stopped/created instance
   - Docker: `POST /containers/{id}/start`
   - Nomad: Implicit on job submission
   - E2B: Implicit on create, `Sandbox.connect()` for resume
   - Fly.io: `POST /machines/{id}/start`
   - Modal: Implicit on create
   - CF Containers: `container.start()`
   - Daytona: `daytona start`
   - K8s agent-sandbox: Set `replicas: 1`

3. **stop** - Stop a running instance
   - Docker: `POST /containers/{id}/stop`
   - Nomad: `DELETE /v1/job/{id}` or `POST /v1/allocation/{id}/stop`
   - E2B: `POST /sandboxes/{id}/pause` (pause, not stop)
   - Fly.io: `POST /machines/{id}/stop`
   - Modal: `sandbox.terminate()`
   - CF Containers: `container.stop()` or auto-sleep
   - Daytona: `daytona stop`
   - K8s agent-sandbox: Set `replicas: 0`

4. **destroy/remove** - Permanently delete instance
   - Docker: `DELETE /containers/{id}`
   - Nomad: `DELETE /v1/job/{id}`
   - E2B: `POST /sandboxes/{id}/kill`
   - Fly.io: `DELETE /machines/{id}`
   - Modal: `sandbox.terminate()`
   - CF Containers: `container.destroy()`
   - Daytona: `daytona delete`
   - K8s agent-sandbox: Delete `Sandbox` CRD

5. **exec** - Execute command in running instance
   - Docker: `POST /containers/{id}/exec`
   - Nomad: `WebSocket /v1/client/allocation/{id}/exec`
   - E2B: `sandbox.exec()` (via SDK)
   - Fly.io: Via SSH or `fly machine exec`
   - Modal: `sandbox.exec()`
   - CF Containers: Via Worker-to-Container fetch
   - Daytona: Via SSH gateway
   - K8s agent-sandbox: `kubectl exec`

6. **inspect/status** - Get instance state
   - Docker: `GET /containers/{id}/json`
   - Nomad: `GET /v1/allocation/{id}`
   - E2B: `GET /sandboxes/{id}` (via SDK)
   - Fly.io: `GET /v1/apps/{app}/machines/{id}`
   - Modal: `Sandbox.from_id()` + status checks
   - CF Containers: Worker state + container state
   - Daytona: `daytona workspace list`
   - K8s agent-sandbox: `get Sandbox`

---

## Optional Verbs

Operations present in **SOME** substrates only:

1. **pause/suspend** - Suspend with memory state
   - Docker: `POST /containers/{id}/pause` (no memory state)
   - E2B: `POST /sandboxes/{id}/pause` (with memory state)
   - Fly.io: `POST /machines/{id}/suspend` (with memory state)
   - Modal: ❌ No pause
   - CF Containers: ❌ No pause (auto-sleep only)
   - Daytona: ❌ No pause
   - K8s agent-sandbox: ❌ No pause (scale-to-zero only)
   - Nomad: ❌ No pause

2. **resume** - Resume from suspended state
   - Docker: `POST /containers/{id}/unpause` (no memory restore)
   - E2B: `POST /sandboxes/{id}/resume` (with memory restore)
   - Fly.io: `POST /machines/{id}/start` (auto-resumes if suspended)
   - Modal: ❌ No resume
   - CF Containers: ✅ Auto-resume on request
   - Daytona: ❌ No resume
   - K8s agent-sandbox: ✅ Auto-resume (scale up from 0 to 1)
   - Nomad: ❌ No resume

3. **snapshot** - Create point-in-time capture
   - Docker: `docker export` (FS only) / `docker commit` (image)
   - E2B: `Snapshot.create()` (FS + memory)
   - Fly.io: Implicit in suspend (FS + memory)
   - Modal: `sandbox.snapshot()` (FS only, read-only)
   - CF Containers: ❌ No snapshot
   - Daytona: `daytona snapshot create` (FS only, OCI image)
   - K8s agent-sandbox: ❌ No snapshot (planned: PVC VolumeSnapshot)
   - Nomad: ❌ No native snapshot (driver-dependent)

4. **restore/spawn from snapshot** - Create from snapshot
   - Docker: `docker import` + `docker create`
   - E2B: `Sandbox.create({ snapshotID })`
   - Fly.io: ❌ Not exportable, implicit resume only
   - Modal: `Sandbox.create({ snapshot })`
   - CF Containers: ❌ No restore (backup API exists)
   - Daytona: Use snapshot ID in workspace creation
   - K8s agent-sandbox: ❌ Not implemented
   - Nomad: ❌ No native restore

5. **export** - Export state to portable format
   - Docker: `GET /containers/{id}/export` (tarball stream)
   - E2B: ❌ No export (snapshots platform-bound)
   - Fly.io: ❌ No export (snapshots platform-bound)
   - Modal: ❌ No export (snapshots read-only templates)
   - CF Containers: ❌ No export
   - Daytona: ❌ No export (OCI images stored in S3)
   - K8s agent-sandbox: ❌ No export
   - Nomad: ❌ No native export

6. **import** - Import from portable format
   - Docker: `POST /images/create` (from tarball)
   - E2B: ❌ No import
   - Fly.io: ❌ No import
   - Modal: ❌ No import
   - CF Containers: ❌ No import
   - Daytona: ❌ No import (OCI images from registry only)
   - K8s agent-sandbox: ❌ No import
   - Nomad: ❌ No native import

7. **cp/file transfer** - Copy files in/out
   - Docker: `POST /containers/{id}/archive`
   - E2B: SDK file operations
   - Fly.io: Via SCP or `fly machine ssh`
   - Modal: SDK file operations
   - CF Containers: ❌ No direct file copy (via exec)
   - Daytona: Via SSH or SDK
   - K8s agent-sandbox: `kubectl cp`
   - Nomad: `nomad alloc exec` + tar

---

## Missing Verbs

Operations that **Mesh needs** but **no substrate provides natively**:

1. **Cold migration** - Stop, export FS, destroy, instantiate elsewhere, import FS
   - ❌ **No substrate provides this end-to-end**
   - Docker has export/import but requires manual orchestration
   - All other substrates are platform-bound
   - **Mesh must implement this** as:
     1. `stop()` - Stop instance
     2. `export_fs()` - Extract filesystem (tarball)
     3. `destroy()` - Delete instance
     4. `instantiate_elsewhere()` - Create on new substrate
     5. `import_fs()` - Load filesystem into new instance

2. **Portable memory state** - Migrate running memory across substrates
   - ❌ **Impossible at primitive level** (hardware constraint, see substrate-landscape.md F1)
   - Firecracker, CRIU snapshots are CPU/kernel-bound
   - **Mesh must accept this limitation** and use filesystem-only snapshots

3. **Universal snapshot format** - Single format works on all substrates
   - ✅ OCI images + POSIX tarballs (universal but FS-only)
   - ❌ Memory+FS format (doesn't exist portably)
   - **Mesh must use OCI + tarball** as the portable format

4. **Cross-substrate identity** - Same identity moves across substrates
   - ❌ **No substrate provides this**
   - Each substrate has its own ID scheme
   - **Mesh must provide body abstraction** that transcends substrate IDs

5. **Automatic resource adaptation** - Adapt resources when migrating
   - ❌ **No substrate provides this**
   - Resource specs vary by substrate (CPU units, memory units)
   - **Mesh must translate** resource specs between substrates

---

## Proposed Adapter Interface

### Core Types

```typescript
// Universal resource specification
interface Resources {
  cpu?: number;        // vCPUs (normalized)
  memory?: number;     // MiB
  disk?: number;       // GiB
  gpu?: {              // Optional GPU
    type: string;
    count: number;
  };
}

// Network configuration
interface NetworkConfig {
  mode: 'bridge' | 'host' | 'none' | 'private';
  ports?: PortMapping[];
  dns?: string[];
}

// Port mapping
interface PortMapping {
  container: number;
  host?: number;
  protocol: 'tcp' | 'udp';
}

// Body identity (Mesh-level)
interface BodyID {
  id: string;          // UUID
  substrate: string;   // Substrate adapter name
  instance_id: string; // Substrate-specific instance ID
}

// Snapshot metadata
interface Snapshot {
  id: string;
  body_id: BodyID;
  created_at: Date;
  size_bytes: number;
  format: 'oci-image' | 'tarball';
  storage_uri: string; // Where the snapshot is stored
}
```

### Adapter Interface

```typescript
interface SubstrateAdapter {
  // Capability declaration
  readonly capabilities: AdapterCapabilities;
  readonly name: string;
  
  // Core lifecycle
  create(
    image: string, 
    resources: Resources,
    network?: NetworkConfig,
    env?: Record<string, string>
  ): Promise<string>; // Returns instance_id
  
  start(instance_id: string): Promise<void>;
  stop(instance_id: string): Promise<void>;
  destroy(instance_id: string): Promise<void>;
  
  // State inspection
  getStatus(instance_id: string): Promise<InstanceStatus>;
  getLogs(instance_id: string): Promise<AsyncIterable<string>>;
  
  // Command execution
  exec(
    instance_id: string,
    command: string[],
    env?: Record<string, string>
  ): Promise<ExecResult>;
  
  // Filesystem operations (optional)
  readFile?(instance_id: string, path: string): Promise<Buffer>;
  writeFile?(instance_id: string, path: string, data: Buffer): Promise<void>;
  copyFiles?(instance_id: string, src: string, dst: string): Promise<void>;
  
  // Snapshot/export (optional)
  exportFilesystem?(instance_id: string): Promise<ReadableStream>; // Tarball stream
  importFilesystem?(image: string, tarball: ReadableStream): Promise<string>; // Returns new instance_id
  createSnapshot?(instance_id: string): Promise<Snapshot>;
  restoreFromSnapshot?(snapshot: Snapshot): Promise<string>; // Returns instance_id
  
  // Suspend/resume (optional)
  suspend?(instance_id: string): Promise<void>;
  resume?(instance_id: string): Promise<void>;
  
  // Resource queries
  getAllocatedResources(instance_id: string): Promise<Resources>;
  getAvailableResources(): Promise<Resources>;
}

// Status enum
enum InstanceStatus {
  CREATED = 'created',
  STARTING = 'starting',
  RUNNING = 'running',
  PAUSED = 'paused',
  STOPPED = 'stopped',
  SUSPENDED = 'suspended',
  TERMINATING = 'terminating',
  TERMINATED = 'terminated',
  ERROR = 'error'
}

// Exec result
interface ExecResult {
  exitCode: number;
  stdout: string;
  stderr: string;
}

// Capability declaration
interface AdapterCapabilities {
  // Required capabilities
  create: boolean;
  start: boolean;
  stop: boolean;
  destroy: boolean;
  getStatus: boolean;
  exec: boolean;
  
  // Optional capabilities
  readFile?: boolean;
  writeFile?: boolean;
  copyFiles?: boolean;
  
  exportFilesystem?: boolean;
  importFilesystem?: boolean;
  createSnapshot?: boolean;
  restoreFromSnapshot?: boolean;
  
  suspend?: boolean;
  resume?: boolean;
  
  // Snapshot type
  snapshotType?: 'none' | 'filesystem-only' | 'memory+filesystem' | 'read-only-template';
  
  // Resource limits
  minCPU?: number;
  maxCPU?: number;
  minMemory?: number; // MiB
  maxMemory?: number; // MiB
  minDisk?: number;  // GiB
  maxDisk?: number;  // GiB
  
  // GPU support
  gpu?: boolean;
  gpuTypes?: string[];
  
  // Platform characteristics
  persistentDisk?: boolean;      // Disk survives stop/destroy
  portableSnapshots?: boolean;    // Snapshots can move off-platform
  memorySnapshots?: boolean;      // Snapshots include memory state
  autoResume?: boolean;          // Auto-resume on activity
  scaleToZero?: boolean;         // Can run with 0 instances
}
```

### Error Handling

```typescript
enum AdapterErrorCode {
  INSTANCE_NOT_FOUND = 'INSTANCE_NOT_FOUND',
  INSTANCE_ALREADY_EXISTS = 'INSTANCE_ALREADY_EXISTS',
  INVALID_STATE = 'INVALID_STATE',
  INSUFFICIENT_RESOURCES = 'INSUFFICIENT_RESOURCES',
  NETWORK_ERROR = 'NETWORK_ERROR',
  AUTHENTICATION_ERROR = 'AUTHENTICATION_ERROR',
  NOT_SUPPORTED = 'NOT_SUPPORTED',
  TIMEOUT = 'TIMEOUT',
  UNKNOWN = 'UNKNOWN'
}

class AdapterError extends Error {
  constructor(
    public code: AdapterErrorCode,
    message: string,
    public details?: any
  ) {
    super(message);
    this.name = 'AdapterError';
  }
}
```

---

## Capability Model

### Capability Discovery

Each adapter declares its capabilities via the `capabilities` property. Mesh core can:

1. **Query capabilities** before using a feature:
```typescript
if (adapter.capabilities.suspend && adapter.capabilities.resume) {
  await adapter.suspend(instance_id);
  // Later...
  await adapter.resume(instance_id);
} else {
  // Fallback: stop/start instead
  await adapter.stop(instance_id);
  // Later...
  await adapter.start(instance_id);
}
```

2. **Validate requirements** before operation:
```typescript
function validateCapability(adapter: SubstrateAdapter, operation: string) {
  const cap = adapter.capabilities[operation as keyof AdapterCapabilities];
  if (cap === undefined || cap === false) {
    throw new AdapterError(
      AdapterErrorCode.NOT_SUPPORTED,
      `Operation '${operation}' not supported by ${adapter.name}`
    );
  }
}
```

3. **Feature detection** for routing logic:
```typescript
function chooseSnapshotStrategy(adapter: SubstrateAdapter): 'native' | 'export-import' {
  if (adapter.capabilities.snapshotType === 'memory+filesystem') {
    return 'native';
  } else if (adapter.capabilities.exportFilesystem && adapter.capabilities.importFilesystem) {
    return 'export-import';
  } else {
    throw new Error('No snapshot strategy available');
  }
}
```

### Required vs Optional Capabilities

**Required** (all adapters must implement):
- `create` - Create new instance
- `start` - Start instance
- `stop` - Stop instance
- `destroy` - Destroy instance
- `getStatus` - Get instance status
- `exec` - Execute commands

**Optional** (substrate-dependent):
- Filesystem I/O: `readFile`, `writeFile`, `copyFiles`
- Snapshots: `exportFilesystem`, `importFilesystem`, `createSnapshot`, `restoreFromSnapshot`
- Suspend/Resume: `suspend`, `resume`
- Memory snapshots: indicated by `snapshotType`

### Resource Constraints

Adapters declare min/max resource limits:
```typescript
interface ResourceConstraints {
  minCPU: number;      // e.g., 0.1 vCPUs
  maxCPU: number;      // e.g., 64 vCPUs
  minMemory: number;   // e.g., 128 MiB
  maxMemory: number;   // e.g., 256 GiB
  minDisk: number;     // e.g., 1 GiB
  maxDisk: number;     // e.g., 10 TiB
  gpu: boolean;
  gpuTypes: string[];  // e.g., ['nvidia-a100', 'nvidia-v100']
}
```

Mesh validates resource requests against these constraints before passing to adapter.

---

## Compliance Matrix

| Operation | Docker | Nomad | E2B | Fly.io | Modal | CF Containers | Daytona | K8s agent-sandbox |
|-----------|--------|-------|-----|--------|-------|---------------|---------|-------------------|
| **Required** |
| `create` | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| `start` | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| `stop` | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| `destroy` | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| `getStatus` | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| `exec` | ✅ | ✅ | ✅ | ✅ | ✅ | ⚠️ | ✅ | ✅ |
| **Filesystem I/O** |
| `readFile` | ⚠️ (via exec) | ⚠️ (via exec) | ✅ | ⚠️ (via ssh) | ✅ | ⚠️ (via exec) | ⚠️ (via ssh) | ⚠️ (via exec) |
| `writeFile` | ⚠️ (via exec) | ⚠️ (via exec) | ✅ | ⚠️ (via ssh) | ✅ | ⚠️ (via exec) | ⚠️ (via ssh) | ⚠️ (via exec) |
| `copyFiles` | ✅ | ⚠️ (via exec) | ✅ | ⚠️ (via ssh) | ✅ | ❌ | ⚠️ (via ssh) | ✅ |
| **Snapshots** |
| `exportFilesystem` | ✅ | ⚠️ (driver-dep) | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ |
| `importFilesystem` | ✅ | ⚠️ (driver-dep) | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ |
| `createSnapshot` | ⚠️ (commit) | ❌ | ✅ | ✅ | ✅ | ❌ | ✅ | ❌ |
| `restoreFromSnapshot` | ✅ | ❌ | ✅ | ⚠️ (resume) | ✅ | ❌ | ✅ | ❌ |
| **Suspend/Resume** |
| `suspend` | ✅ (no mem) | ❌ | ✅ | ✅ | ❌ | ❌ | ❌ | ❌ |
| `resume` | ✅ (no mem) | ❌ | ✅ | ✅ | ❌ | ✅ (auto) | ❌ | ✅ (auto) |
| **Snapshot Type** |
| `snapshotType` | filesystem-only | none | memory+filesystem | memory+filesystem | read-only-template | none | filesystem-only | none |
| **Platform Characteristics** |
| `persistentDisk` | ✅ | ⚠️ (driver-dep) | ❌ | ✅ | ❌ | ❌ | ✅ | ✅ (PVC) |
| `portableSnapshots` | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ |
| `memorySnapshots` | ❌ | ❌ | ✅ | ✅ | ❌ | ❌ | ❌ | ❌ |
| `autoResume` | ❌ | ❌ | ✅ | ✅ | ❌ | ✅ | ❌ | ✅ |
| `scaleToZero` | ❌ | ⚠️ (driver-dep) | ❌ | ❌ | ❌ | ✅ | ❌ | ✅ |

**Legend:**
- ✅ = Fully supported natively
- ⚠️ = Partially supported (requires workaround, driver-dependent, or limited)
- ❌ = Not supported

---

## Key Findings

### F1: Universal lifecycle verbs are limited
All substrates support the basic CRUD operations: create, start, stop, destroy, exec, getStatus. This is the **core contract** that Mesh can rely on across all adapters.

### F2: Suspend/resume is not universal
Only E2B and Fly.io provide true suspend/resume with memory state. Docker has pause/unpause but it doesn't free memory. Modal, CF Containers, Daytona, and Nomad lack suspend entirely. Mesh must implement cold migration (stop + export + destroy + instantiate + import) as the universal fallback.

### F3: Portable snapshots exist only for filesystem
Docker is the only substrate with truly portable snapshots (tarball via `docker export`). All other substrates' snapshots are platform-bound. This validates Mesh's "filesystem-only snapshots" design decision (D1, D2).

### F4: Memory snapshots are impossible across substrates
E2B and Fly.io provide memory snapshots, but they're not portable (CPU/kernel-bound). This confirms the finding from substrate-landscape.md F1: "No portable live-snapshot format exists."

### F5: No substrate provides cold migration natively
No substrate has a built-in "stop, export, migrate, import" workflow. Mesh must orchestrate this across multiple adapter calls.

### F6: Resource units vary by substrate
- Docker: CPU in 10^-9 units, memory in bytes
- Nomad: CPU in MHz, memory in MiB
- E2B/Fly/Modal: CPU in vCPUs, memory in MiB
- K8s: CPU in units (500m = 0.5 vCPU), memory in MiB/GiB

Mesh's adapter must normalize these to a common unit (e.g., vCPUs and MiB).

### F7: Networking models are incompatible
- Docker: bridge, host, none, custom networks
- Nomad: host, group, task (driver-dependent)
- E2B/Fly/Modal: Managed private networking
- CF Containers: Worker-to-Container fetch
- K8s: Service + Ingress + NetworkPolicy

Mesh must abstract networking or delegate to substrate-specific configuration.

### F8: Filesystem I/O is inconsistent
- Docker: `cp` endpoint (tar-based)
- Nomad: Via `exec` (no native I/O)
- E2B/Modal: SDK file operations
- Fly.io: Via SSH
- CF Containers: No direct I/O (via exec)
- K8s: `kubectl cp`

Mesh should provide a consistent file I/O interface that falls back to `exec` + tar when needed.

### F9: Identity models are substrate-specific
- Docker: Container ID (SHA256)
- Nomad: Allocation ID (UUID)
- E2B: Sandbox ID (UUID)
- Fly.io: Machine ID (UUID)
- Modal: Sandbox ID (UUID)
- CF Containers: Durable Object ID
- K8s: Pod name + namespace

Mesh must provide a **Body ID** abstraction that wraps substrate-specific IDs.

### F10: Auto-resume is a pattern, not a primitive
E2B, Fly.io, CF Containers, and K8s agent-sandbox all have auto-resume (resume on activity). This suggests Mesh should support "auto-resume" as a cross-substrate feature, implemented via:
- Native suspend/resume (where available)
- Stop + recreate + import (fallback)

---

## Verdict

### Recommended Adapter Contract

The proposed adapter interface is **sufficient** for Mesh's requirements:

1. **Core lifecycle** (create/start/stop/destroy/exec/getStatus) is universal and can be relied upon.

2. **Optional capabilities** (snapshots, suspend/resume, file I/O) are correctly identified as substrate-dependent. The capability model allows Mesh to:
   - Detect feature availability at runtime
   - Choose optimal strategies (native suspend vs. cold migration)
   - Fallback gracefully when features are missing

3. **Cold migration** is correctly identified as a Mesh-level orchestration over adapter primitives:
   ```
   stop() → exportFilesystem() → destroy() → 
   create() → importFilesystem() → start()
   ```
   This works on any substrate that supports `exportFilesystem` and `importFilesystem` (currently only Docker). For other substrates, Mesh must implement substrate-specific export/import (e.g., tarball via `exec`).

4. **Resource normalization** is necessary. All adapters should accept resources in Mesh's units (vCPUs, MiB, GiB) and translate to substrate-specific units internally.

5. **Body ID abstraction** is critical. Mesh's `BodyID` type correctly wraps substrate-specific IDs and enables cross-substrate identity.

### Critical Decisions

**D1: Require all adapters to implement file I/O**
Even if the substrate doesn't provide native file I/O, adapters must implement `readFile`/`writeFile`/`copyFiles` using `exec` + tar as a fallback. This provides a consistent interface to Mesh core.

**D2: Use OCI images + tarballs as the portable format**
This is the only format that works across all substrates (after adapters implement export/import). Mesh's body format (D2) is validated.

**D3: Implement cold migration in Mesh core**
Do not expect substrates to provide migration. Mesh must orchestrate the stop→export→destroy→create→import→start workflow.

**D4: Make suspend/resume optional optimization**
Native suspend/resume (E2B, Fly.io) should be used when available for faster resume, but cold migration must always work as the fallback.

**D5: Expose network configuration as substrate-specific**
Do not try to abstract networking uniformly. Let each substrate adapter accept its own network config (e.g., Docker's NetworkMode, Fly's regional config, K8s's Service spec).

### Next Steps

1. **Implement Docker adapter** as the reference implementation (fully portable, all features).

2. **Implement Nomad adapter** (resource constraints: min 2GB VMs, no suspend, exec-only file I/O).

3. **Define adapter plugin system** for D6 (provider integrations are plugins).

4. **Implement cold migration orchestration** in Mesh core (the missing verb that no substrate provides).

5. **Normalize resource units** across all adapters (vCPUs, MiB, GiB).

6. **Design Body ID persistence** (how Mesh tracks bodies across substrate changes).

The adapter contract is sound. The remaining work is implementation and orchestration, not interface design.
