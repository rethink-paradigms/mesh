# Mesh System Design

> This is the true artifact — design that any competent team could use to build Mesh.

## Design Philosophy

The system is designed for people and agents who BUILD and MAINTAIN it, not for end-users. Module boundaries are drawn where a developer or AI agent can work independently with bounded context. Each module has a clear contract (what it receives, what it returns), encapsulates its own complexity (internals don't leak), is independently implementable and testable, and can be built by an agent who understands contracts but NOT internals of other modules.

Code is compiled output. The design is the true artifact. If this document is complete enough, any competent system with the right tools can generate the implementation.

## The Six Modules

### 1. Interface (MCP + Skills)

**Owns:** How external systems talk to Mesh

**Contract:** Receives commands from agents/skills via MCP tools. Returns body states, operation results.

**Internal complexity:** MCP tool definitions, skill lifecycle, request routing to appropriate modules. Translates structured errors to user-friendly MCP responses.

**Does NOT know:** How bodies are provisioned, how snapshots work, how networking is configured.

**Key tools (examples):** `mesh.body.create`, `mesh.body.snapshot`, `mesh.body.migrate`, `mesh.provisioner.list`, `mesh.plugin.install`

### 2. Provisioning (Provider Plugins)

**Owns:** How Mesh reaches compute substrates

**Contract:** Receives substrate requirements (CPU, RAM, disk, region). Returns a substrate handle (running container with network endpoint).

**Internal complexity:** Plugin discovery, loading, validation. Individual provider implementations (Docker local, Nomad fleet, E2B sandbox, Fly machine). If no plugin exists, skill triggers Pulumi to generate one.

**Does NOT know:** What's inside the body, how snapshots are stored, how MCP works.

**Key interface:** `Provisioner.Provision(spec) → Handle`, `Provisioner.Destroy(handle)`, `Provisioner.ListCapabilities() → CapabilitySet`

### 3. Orchestration (Body Lifecycle + Substrate Adapter)

**Owns:** How Mesh runs bodies — the core runtime

**Contract:** Receives body spec (base image, resources, env). Returns running body with identity. Accepts lifecycle commands (start, stop, destroy). Manages body state machine.

**Internal complexity:** Body state machine (Created → Starting → Running → Stopping → Stopped → Destroyed). Substrate adapter contract — a uniform interface that Provisioning plugins implement. Scheduling hints (where to run) — but scheduling DECISIONS are made by the caller (skill/user), not by this module.

**Does NOT know:** How snapshots are captured, how storage works, how MCP routes requests.

**Key interface:** `BodyManager.Create(spec) → Body`, `BodyManager.Start(id)`, `BodyManager.Stop(id)`, `BodyManager.Destroy(id)`, `BodyManager.List() → []Body`

### 4. Persistence (Snapshot + Storage)

**Owns:** How Mesh captures, stores, and retrieves body state

**Contract:** Receives a running body. Returns a portable snapshot (OCI image + FS tarball). Receives a snapshot reference. Restores body state. Stores and retrieves snapshots from pluggable storage backends.

**Internal complexity:** The snapshot pipeline (pre-prune → docker export → zstd compress → store). Storage backend abstraction (S3, local FS, OCI registry, R2). Snapshot lifecycle (create, list, get, delete, garbage collect). Bloat management (prune caches before export). Metadata storage alongside snapshots (env vars, entrypoint, working dir).

**Does NOT know:** Where the body is running, how it was provisioned, how users interact with Mesh.

**Key interface:** `SnapshotEngine.Capture(bodyId) → SnapshotRef`, `SnapshotEngine.Restore(snapshotRef, targetBody) → Body`, `StorageBackend.Put(snapshot)`, `StorageBackend.Get(ref) → Snapshot`

### 5. Networking (Tailscale + Identity)

**Owns:** How bodies get network identity and connectivity

**Contract:** Receives a body. Returns network identity (IP, DNS name, connectivity). Connects bodies to each other and to the internet. Enforces network policies.

**Internal complexity:** Tailscale integration (each body gets a tailnet IP). Body identity that survives substrate changes (name → IP mapping). DNS resolution. Optional firewall rules. SSH gateway. Tailnet management using user's Tailscale account or headscale.

**Does NOT know:** What's running inside the body, how it was provisioned, how snapshots work.

**Key interface:** `Network.AssignIdentity(bodyId) → NetworkIdentity`, `Network.Connect(bodyA, bodyB)`, `Network.GetEndpoint(bodyId) → URL`

### 6. Plugin Infrastructure (Discovery + Loading + Generation)

**Owns:** How Mesh extends itself

**Contract:** Receives plugin type (provider, storage, scheduler) and name. Returns loaded plugin instance. Handles plugin lifecycle (discover, install, configure, use, update, remove). Triggers Pulumi skill for generation when plugin doesn't exist.

**Internal complexity:** Plugin registry (what's available, what's installed). Plugin loading (discovery, validation, sandboxing via go-plugin with gRPC protocol). Plugin generation (integration with Pulumi skill to generate infrastructure code, then wrap in SubstrateAdapter interface). Plugin repo structure (mesh-plugins/providers/*, mesh-plugins/storage/*). Capability declaration at load time (required vs optional features).

**Does NOT know:** What any specific plugin does internally.

**Key interface:** `PluginRegistry.Discover(type) → []PluginMeta`, `PluginRegistry.Load(name) → Plugin`, `PluginGenerator.Generate(spec) → Plugin`

## Data Flow: A Body's Journey

### Create and Run a Body

1. Skill calls Interface: `mesh.body.create(spec)`
2. Interface calls Orchestration: `BodyManager.Create(spec)`
3. Orchestration calls Provisioning: `Provisioner.Provision(spec) → substrate handle`
4. Orchestration calls Networking: `Network.AssignIdentity(bodyId) → network identity`
5. Orchestration returns Body (with handle + identity) to Interface
6. Interface returns body info to skill

### Snapshot a Body

1. Skill calls Interface: `mesh.body.snapshot(bodyId)`
2. Interface calls Persistence: `SnapshotEngine.Capture(bodyId)`
3. Persistence calls Orchestration to get body handle
4. Persistence executes: pre-prune → docker export → zstd compress
5. Persistence calls `StorageBackend.Put(snapshot)` → stores to user's configured backend
6. Persistence returns SnapshotRef to Interface
7. Interface returns snapshot info to skill

### Migrate a Body (Cold Migration)

1. Skill calls Interface: `mesh.body.migrate(bodyId, targetSubstrate)`
2. Interface orchestrates the sequence:
   a. Orchestration.Stop(bodyId) — graceful stop
   b. Persistence.Capture(bodyId) → SnapshotRef — export FS
   c. Provisioning.Provision(spec on targetSubstrate) → new handle — provision on new substrate
   d. Persistence.Restore(SnapshotRef, newBodyId) — import FS
   e. Networking.AssignIdentity(newBodyId) — reassign identity (same name, new IP)
   f. Orchestration.Start(newBodyId) — bring up on new substrate
   g. Provisioning.Destroy(old handle) — clean up old substrate
3. Interface returns migrated body info to skill

Note: Transport is NOT a module. It's a coordination script that calls modules 2-5 in sequence.

## Core vs Plugin Boundary

**Core (always present):**

- Orchestration (body lifecycle, substrate adapter contract)
- Interface (MCP server, tool definitions)
- Networking (Tailscale integration)
- Plugin Infrastructure (registry, loading, gRPC protocol, lifecycle)

**Plugin (user installs what they need):**

- Provisioning providers (Docker local, Nomad fleet, E2B sandbox, Fly machine, etc.)
- Storage backends (S3, local FS, OCI registry, R2, MinIO, GCS, Azure)
- Scheduler policies (cheapest, lowest-latency, user-specified, multi-cloud)

**Principle:** If adding support for a new cloud/sandbox requires changing core code, the boundary is wrong. This enforces D6 (provider integrations are plugins) and enables community contributions without core maintenance.

## Cross-Cutting Concerns

**Error handling:** Each module owns its errors. Errors bubble up as structured types (gRPC status codes: UNKNOWN, INSTANCE_NOT_FOUND, INVALID_STATE, INSUFFICIENT_RESOURCES, NETWORK_ERROR, AUTHENTICATION_ERROR, NOT_SUPPORTED, TIMEOUT, QUOTA_EXCEEDED, RATE_LIMITED), not raw exceptions. Interface module translates to user-friendly MCP error responses. Plugins distinguish retryable errors (rate limits, network) from fatal errors (auth, quota).

**Logging/observability:** Each module logs its own operations. No centralized logging dependency. Logs are structured JSON. Module boundaries are visible in logs.

**Configuration:** Each module has its own config section. No module reads another module's config. Single config file (YAML) with sections per module. Mesh CLI provides `mesh config set <module>.<key> <value>` interface.

**Testing:** Each module can be tested in isolation with mock contracts for its dependencies. Plugin system supports mock adapters for unit testing. Go SDK includes testing framework (SubstrateAdapter interface mocks).

**Security:** Plugin subprocess isolation (go-plugin) limits plugin access to host. Plugin signature verification (optional). Encryption at rest supported via storage backend plugins (S3 SSE-S3, R2 encryption). User controls all credentials — never stored in Mesh core.

## Research Backing

**Interface:** D5 (MCP primary), Daytona analysis (MCP patterns, Tailscale integration)

**Provisioning:** D6 (plugins), D7 (container body), plugin-architecture.md (gRPC protocol, go-plugin lifecycle, capability model, Pulumi skill integration)

**Orchestration:** D1 (FS-only), D4 (cold migration), substrate-adapter.md (6 verbs, capability discovery, compliance matrix)

**Persistence:** D2 (OCI + tar), snapshot-mechanics.md (docker export pipeline, zstd compression, bloat management, metadata handling), registry-strategy.md (S3/R2/GCS/Azure, OCI registries, storage plugin interface)

**Networking:** Daytona analysis (Tailscale patterns), intent.md (Tailscale for identity), C3 (user owns network)

**Plugin Infrastructure:** D6 (plugins), plugin-architecture.md (gRPC protocol, go-plugin, Pulumi AI integration, plugin discovery, capability model)

## Open Design Questions (NOT implementation questions)

1. What language is the core written in? (Go for gRPC plugins and Nomad integration? Rust for lightweight? TypeScript for MCP ecosystem?)
2. How are body configs (env vars, entrypoint, working dir) persisted alongside the FS snapshot? (Metadata manifest in storage backend vs. OCI image labels)
3. What's the minimum viable system — which modules are needed for v0? (Can we ship without Networking if users don't need Tailscale? Can we ship with only Docker provider?)
4. How does a skill authenticate with Mesh? (API key stored in config? Tailscale identity? No auth for local only?)
5. Should we support hot/suspend-resume as optional optimization in v0, or defer to v1? (Substrate adapter has suspend/resume optional methods)
6. How do we handle volume backups? (Volumes are excluded from docker export; separate backup/restore required)
7. Should the substrate adapter support volume operations as optional methods? (Or keep volumes as user's responsibility outside Mesh)
8. What's the body identity format? (UUID only? UUID + human-readable name? Include substrate info?)
9. How do we handle orphaned bodies? (Bodies that exist but no caller is tracking them. Garbage collection policy?)
10. Should we support streaming logs and file transfers via gRPC streams? (Better than buffering in memory)

## Constraints (reminder)

**C1: Must run on 2GB VMs**
- Orchestration: Minimal runtime (~200MB for Tailscale ~20MB + runtime). Interface small. Plugins are subprocesses (isolated memory).
- Networking: Tailscale ~20MB RAM.
- Persistence: Streaming operations, minimal memory.
- Plugin Infra: go-plugin subprocess overhead is small.

**C2: Must not require K8s control plane**
- Orchestration: Nomad-based scheduler, no CRDs, no PVC/CSI, no Service/NetworkPolicy.
- Plugins: Docker local, E2B, Fly — no K8s dependencies.

**C3: User owns all compute, keys, network**
- Provisioning: User provides API keys (never stored in core). User owns cloud accounts.
- Networking: User's Tailscale account or headscale instance.
- Storage: User's S3/R2/GCS credentials.
- Plugin Infra: No Mesh-controlled auth, no phone-home.

**C4: No telemetry, no login, no central dependency**
- All modules: No metrics collection, no crash reporting, no analytics. All config local.
- Interface: No Mesh server to call. Self-hosted MCP server.
- Plugin Infra: No plugin registry that phones home. Directory-based discovery.

**C5: Portable across substrates — no kernel/CPU coupling for core path**
- Persistence: docker export (FS-only), zstd compression — universally portable. No CRIU, no memory state.
- Orchestration: OCI image + tarball format works on all OCI runtimes. Substrate adapter abstracts differences.
- Snapshots are portable tarballs, not provider-native snapshots (which are platform-bound).

**C6: Core is tiny — provider code is plugin, not core library**
- Provisioning: Zero provider code in core. All providers are plugins.
- Storage: Zero storage code in core. All backends are plugins.
- Orchestration: Substrate adapter interface is only abstraction. Plugins implement it.
- Plugin Infra: Plugin registry and loader. Pulumi skill generates plugins, not core code.

This design respects all constraints. Every module boundary is drawn to enable independent development, testing, and maintenance. Any competent team with this document can build Mesh.
