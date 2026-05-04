# Architecture

Mesh is a portable agent-body runtime. It gives AI agents a persistent compute identity (a body) that can live on any substrate and move between them without losing itself. This document describes the system architecture at a component level.

## Core Abstractions

Mesh is built on three abstractions:

**Body.** A permanent compute identity with persistent filesystem state. The body is the agent's filesystem: installed packages, written files, modified configs. It persists across substrate changes. Bodies are created once and follow a state machine from creation through destruction, with snapshots as intermediate checkpoints.

**Form.** The physical instantiation of a body on a specific substrate at a given time. A body can take many forms over its lifetime: a Docker container on a laptop, a Nomad allocation on a fleet VM, or a sandbox instance on a cloud provider. Forms are ephemeral by design. Destroying a form does not destroy the body.

**Substrate.** The compute environment where a form runs. Mesh manages substrates through two independent adapter pools: **Provisioners** create and destroy raw compute (VMs on Hetzner/DigitalOcean, sandboxes on Daytona/E2B), and **Orchestrators** manage body lifecycle on that compute (Nomad for v1, with K8s/Incus possible later). Mesh never touches container runtimes directly — that is the orchestrator's responsibility. Docker, LXC, and other runtimes are managed by the orchestrator's internal drivers.

## Architecture Diagram

![Mesh System Architecture](assets/mesh-architecture.svg)

The diagram shows the three-layer architecture: the daemon process in the center, the substrate adapters at the bottom, and the MCP interface at the top. The daemon owns the SQLite store, body manager, migration coordinator, and plugin manager.

## Component Overview

The daemon (`mesh serve`) is a long-running process that orchestrates all operations. It is the only process users interact with directly (via CLI) or indirectly (via MCP).

### Store

SQLite with WAL mode. Single file, crash-safe, no external database required. Stores bodies, snapshots, migrations, and plugin metadata. The store is the single source of truth for body state. All state transitions are persisted atomically.

Core tables: `bodies`, `snapshots`, `migrations`, `plugins`.

### Adapter Registry (Orchestrators)

The orchestrator registry manages body lifecycle adapters. For v1, Nomad is the fleet orchestrator — it schedules Docker containers on provisioned VMs via Nomad job specs. The orchestrator also handles cluster management: generating bootstrap configs for new nodes, tracking cluster state, and monitoring node health. Orchestrators implement the `OrchestratorAdapter` interface. Both adapter pools use the database/sql registration pattern (DE14).

### Adapter Registry (Provisioners)

The provisioner registry manages compute lifecycle adapters. Provisioners create and destroy VMs or sandboxes by calling cloud provider APIs. They accept opaque user-data (cloud-init scripts) at machine creation time, passed through from the orchestrator. Provisioners are AI-generated from OpenAPI specs (DE4, DE15). They implement the `ProvisionerAdapter` interface.

### BodyManager

Orchestrates body lifecycle against the store and the orchestrator adapter. Creates, starts, stops, destroys, and queries bodies. Each operation follows the state machine, persists every transition, and handles adapter errors by transitioning to Error state.

### MigrationCoordinator

Manages cold migration between substrates. A 7-step process: export, provision, transfer, import, verify, switch, cleanup. Each step is persisted so migrations can resume after a daemon crash. Supports same-machine and cross-machine transfers. Cross-machine migrations use a Registry (S3) to push and pull snapshot tarballs with SHA-256 verification.

### PluginManager

Loader for go-plugin + gRPC + protobuf plugins. Scans a plugin directory on startup, loads enabled plugins, and runs periodic health checks. Plugins provide additional substrate adapters without core changes.

### MCP Server

JSON-RPC 2.0 server over stdio transport. Registers 16 tools for body CRUD, snapshots, migration, command execution, and plugin management. AI agents communicate with Mesh through this server. The server is the primary user interface (D5).

## Body State Machine

Bodies transition through 8 states. Every transition is validated and persisted before external operations execute.

```
                    ┌──────────┐
                    │  Created │
                    └────┬─────┘
                         │
                    ┌────▼─────┐
                    │ Starting │
                    └────┬─────┘
                         │
                    ┌────▼────┐
         ┌──────────│ Running │◄──────────┐
         │          └────┬────┘           │
         │               │                │
         │          ┌────▼──────┐         │
         │          │  Stopping │         │
         │          └────┬──────┘         │
         │               │                │
         │          ┌────▼─────┐          │
         │  ┌───────│  Stopped │──────────┘
         │  │       └────┬─────┘
         │  │            │
         │  │       ┌────▼───────┐
         │  │       │  Destroyed │ (terminal)
         │  │       └────────────┘
         │  │
         │  └──►┌───────┐
         │      │ Error │──►(recoverable)
         │      └───────┘
         │
         └──────►┌───────────┐
                  │ Migrating │──►Running or Error
                  └───────────┘
```

### State Descriptions

| State | Meaning | Can transition to |
|-------|---------|-------------------|
| Created | Body record allocated, no substrate resources | Starting, Error |
| Starting | Substrate provision in progress | Running, Error |
| Running | Body is accepting commands and work | Stopping, Migrating, Error |
| Stopping | Graceful shutdown in progress | Stopped, Error |
| Stopped | Substrate instance stopped, handle retained | Starting, Destroyed |
| Error | Unrecoverable failure, substrate state uncertain | Starting, Destroyed, Migrating |
| Migrating | Cold migration in progress | Running, Error |
| Destroyed | Terminal. All resources released. | (none) |

### Valid Transition Rules

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

The Error state is an escape hatch. When a body enters Error, the substrate state is uncertain. The daemon's startup reconciliation process identifies orphaned containers and transitions them back to Running or flags them for cleanup. Callers can attempt recovery by calling Start on an Error body, which triggers re-provisioning.

## Snapshot Pipeline

The snapshot pipeline produces a portable, deterministic filesystem archive. It runs inside a Docker container and captures the full filesystem.

```
docker export (container) │ zstd --compress │ tee (archive.tar.zst) │ sha256sum
                            │                                        │
                            │                                        └── digest
                            │
                            └── written to ~/.mesh/snapshots/<agent>/
                                ├── <agent>-YYYYMMDD-HHMMSS.tar.zst
                                ├── <agent>-YYYYMMDD-HHMMSS.tar.zst.sha256
                                └── <agent>-YYYYMMDD-HHMMSS.json (manifest)
```

The pipeline uses `MultiWriter` to feed the compressed output simultaneously to the file and to the SHA-256 hasher. The result is three files: the compressed tarball, a hex-encoded SHA-256 digest, and a JSON manifest with metadata (source agent, timestamp, checksum). The tarball format is a flat filesystem dump, not layered. This avoids docker commit's layered-image growth problem (D2).

Output is deterministic with a sorted workdir walk. The same workdir produces the same archive byte-for-byte, enabling change detection.

## Migration Coordinator

Cold migration is a 7-step coordinator that moves a body between substrates. All steps are persisted to the store so migration can resume after a crash.

| Step | Name | Action |
|------|------|--------|
| 1 | Export | Export the source container filesystem via `docker export`, compress with zstd, compute SHA-256 |
| 2 | Provision | Create a new container on the target substrate using the source container's metadata (image, env, cmd) |
| 3 | Transfer | Copy the snapshot to the target. Same-machine: local file copy. Cross-machine: push to S3 registry, pull on target, verify SHA-256 |
| 4 | Import | Extract the snapshot into the target container's root filesystem (idempotent -- already done in transfer step) |
| 5 | Verify | Health checks on the target: `echo ok`, `ls /`, `GetStatus` must return Running or Created |
| 6 | Switch | Stop the source container, update the body record to point to the new handle, destroy the source container |
| 7 | Cleanup | Remove the local snapshot file and migration record |

The coordinator uses exponential backoff with retries (up to 3 attempts) for registry operations. Cross-machine transfers verify SHA-256 checksums before and after transfer.

## Adapter Interfaces

Mesh uses two independent adapter interfaces, one for each pool.

```go
// OrchestratorAdapter — body lifecycle on compute
type OrchestratorAdapter interface {
    // Cluster management
    InitializeCluster(ctx context.Context) (ClusterInfo, error)
    GetBootstrapConfig(ctx context.Context, role string, info ClusterInfo) (string, error)
    NodeStatus(ctx context.Context, addr string) (NodeStatus, error)
    LeaveNode(ctx context.Context, addr string) error

    // Body lifecycle
    ScheduleBody(ctx context.Context, spec BodySpec) (Handle, error)
    StartBody(ctx context.Context, id Handle) error
    StopBody(ctx context.Context, id Handle) error
    DestroyBody(ctx context.Context, id Handle) error
    GetBodyStatus(ctx context.Context, id Handle) (BodyStatus, error)
    SnapshotBody(ctx context.Context, id Handle) (io.ReadCloser, error)
    RestoreBody(ctx context.Context, id Handle, tarball io.Reader) error

    // Metadata
    Capabilities() OrchestratorCapabilities
    Name() string
    IsHealthy(ctx context.Context) bool
}

// ProvisionerAdapter — compute lifecycle
type ProvisionerAdapter interface {
    CreateMachine(ctx context.Context, spec MachineSpec, userData string) (MachineID, error)
    DestroyMachine(ctx context.Context, id MachineID) error
    GetMachineStatus(ctx context.Context, id MachineID) (MachineStatus, error)
    GetMachineLogs(ctx context.Context, id MachineID) (string, error)
    ListMachines(ctx context.Context) ([]MachineInfo, error)

    // Metadata
    Capabilities() ProvisionerCapabilities
    Name() string
    IsHealthy(ctx context.Context) bool
}
```

Extension interfaces (DE16) provide optional capabilities via type assertion: `Executor` (exec into body), `Logger` (stream logs), `Inspector` (detailed status) for orchestrators; `Snapshotter` (machine snapshots), `NetworkConfigurator` (network setup) for provisioners.

Nomad is the v1 orchestrator. Mesh talks to Nomad to schedule bodies — Nomad manages Docker containers internally via its task driver system. Mesh never calls the Docker API directly (DE18). Provisioner adapters are AI-generated from OpenAPI specs.

## Config Resolution

The daemon reads YAML configuration. Config is resolved in this order:

1. `--config /path/to/config.yaml` flag (highest priority)
2. `$MESH_CONFIG` environment variable
3. `~/.mesh/config.yaml` (default for v1 commands: init, serve, stop, status)
4. `~/.mesh/config.toml` (default for v0 commands: snapshot, restore, list, inspect, prune)

The config defines daemon settings, store path, Docker host, registry credentials, plugin directory, Nomad address, and inline body definitions.

## Startup Sequence

When `mesh serve` runs, the daemon:

1. Checks for PID file conflicts (prevents double-start)
2. Opens the SQLite store with WAL mode
3. Initializes the orchestrator adapter (Nomad for v1) and registers it with the orchestrator registry
4. Creates the BodyManager
5. Scans and loads plugins, starts plugin health checks
5a. Initializes provisioner adapters from config and registers them with the provisioner registry
6. Runs reconciliation: checks every persisted body against its adapter, transitions orphans to Error, recovers running containers from Error state
7. Writes the PID file
8. Starts the HTTP health server on `127.0.0.1:0` (random port, reported via `HTTPAddr()`)
9. Registers SIGTERM and SIGINT handlers
10. Blocks until signal or context cancellation
11. Performs graceful shutdown: stops MCP server, stops plugins, closes store, removes PID file

## Design Decisions

| ID | Decision | Rationale |
|----|----------|-----------|
| D1 | Filesystem-only snapshot, no memory state | Eliminates kernel/CPU coupling (CRIU not portable). Agents stop at task boundaries. |
| D2 | OCI image + volume tarball as portable body format | Flat tarball is universally portable. No layer chain, no overlay complexity. |
| D3 | Nomad as fleet scheduler, not K8s | Nomad runs on 2GB VMs (~80MB RAM). K8s control plane does not fit. |
| D4 | Cold migration only, no live migration in v0 | Live migration requires CPU/kernel-coupled CRIU. Cold migration covers 80% of use cases. |
| D5 | MCP + skills as primary interface, not CLI | Agents manage their own bodies via MCP. CLI is a thin debugging surface. |
| D6 | Provider integrations are plugins, not core | Core has zero provider code. Plugins use go-plugin + gRPC + protobuf. |
| D7 | Agent body = container, not VM | Containers are universal, OCI-standard, and lightweight. MicroVMs add overhead. |
| DE17 | Two-pool adapter architecture: Orchestrators + Provisioners | Single SubstrateAdapter conflated orchestration with provisioning. Independent interfaces enable independent development. |
| DE18 | Mesh never touches container runtimes directly | Docker/LXC are managed by orchestrators. Mesh talks to Nomad, not Docker. |
| DE19 | Cloud-init/user-data bootstrap pattern | Orchestrator generates bootstrap script, provisioner passes as user-data. Clean separation. |
| DE20 | Local support deferred | Lima/Colima, direct Docker — deferred to later version. v1 focuses on fleet + sandbox. |
