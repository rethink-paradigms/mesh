# Decisions

## D1: Filesystem-only snapshot (no memory state)

**Status**: implemented
**Date**: 2026-04-25T12:44:48Z

Context: Research showed memory snapshots (CRIU, Firecracker) are kernel/CPU-coupled and non-portable. Agents stop at task boundaries (graceful SIGTERM), not mid-thought. Memory is disposable.

Decision: Snapshot = capture the container filesystem only. No memory state, no CRIU, no process-tree dump. Agents are stopped cleanly, then their FS is captured.

Rationale: Eliminates kernel/CPU coupling entirely. Makes bodies fully portable across all substrates. Matches actual agent lifecycle (stop after task, not during).

Conflicts with: (none)
Enables: D2, D4, D3
Blocks: live-migration paths (explicitly deferred)

**Relationships:**
- constrains → D1
- enables → D2
- enables → D3
- enables → D4

---

## D10: Mesh is separate from Daytona — Daytona is valid substrate target via adapter, not dependency

**Status**: implemented
**Date**: 2026-04-25T12:44:49Z

Context: Daytona (72k stars, AGPL 3.0) is a managed AI code execution platform. Research showed fundamental mismatches with Mesh's constraints and goals for core integration. However, Daytona's API can be used as a substrate target for adapter generation.

Decision: Mesh builds independently. Daytona is not a dependency, not a platform component, and not required for Mesh operation. Mesh may reference Daytona's patterns (MCP implementation, provider plugin architecture, Tailscale networking) but does not use Daytona code or depend on it. Daytona IS a valid substrate target for adapter generation — a Mesh plugin can manage Daytona workspaces via their OpenAPI, treating Daytona as one of many possible substrates. Mesh does not depend on Daytona code or infrastructure.

Rationale:
1. Resource mismatch: Daytona requires 8-16GB RAM (11-service stack). Mesh targets 2GB VMs. Non-negotiable gap for self-hosting.
2. No body abstraction: Daytona workspaces are platform-bound (state in PostgreSQL + S3 + container). No portable identity surviving substrate changes.
3. Central dependency: Daytona IS a control plane. Mesh constraint is no central dependency (C4). Architecturally opposite.
4. AGPL 3.0: Modifying Daytona triggers copyleft. Commercial license costs money.
5. Different markets: Daytona = Heroku for AI code execution (managed, SaaS). Mesh = Nomad + Docker + Tailscale for agent bodies (self-hosted, portable).
6. Valid substrate target: Daytona's OpenAPI allows adapter generation. A Mesh plugin can provision/manage Daytona workspaces without depending on Daytona code.

Conflicts with: (none)
Enables: independent substrate adapter design, lightweight core path, 2GB VM deployment, Daytona as optional substrate via plugin
Blocks: Daytona as required dependency, Daytona as core component

**Relationships:**
- related_to → D10

---

## D2: OCI image + volume tarball as portable body format

**Status**: accepted
**Date**: 2026-04-25T12:44:48Z

Context: docker commit grows layered images monotonically and doesn't compose with repeated mutation. Nobody in production uses it for persistence. docker export produces a flat tarball of the live FS — captures everything, no overlay drama.

Decision: Agent body = base OCI image (immutable template) + exported filesystem tarball (mutable state). docker export | zstd for snapshots. Restores on any OCI-compatible runtime.

Rationale: Flat tarball is the only universally portable format. Every substrate can accept it. No whiteout metadata, no layer chain, no kernel coupling.

Implementation note: Pre-snapshot prune hook runs pip cache purge, huggingface-cli delete-cache, rm -rf /tmp/*, apt-get clean inside the container before export. Agent-lifecycle concern, not runtime concern.

Conflicts with: (none)
Enables: D4, substrate adapter simplicity (6 verbs only)
Blocks: (none)

[Implementation note: Tarball+manifest format works. Not yet a unified OCI+tarball composite. Separate concerns in code.]

**Relationships:**
- constrains → D2
- related_to → D2
- enables → D2
- enables → D4
- enables → D2

---

## D3: Nomad as fleet scheduler (not K8s)

**Status**: implemented
**Date**: 2026-04-25T12:44:48Z

Context: kubernetes-sigs/agent-sandbox exists but is K8s-to-the-bone (PVC, headless Service, RuntimeClass, NetworkPolicy, HPA). Dead on 2GB VMs. No serious Nomad-based agent-sandbox project exists in OSS — genuine whitespace.

Decision: Fleet pool uses Nomad as the scheduler. Nomad runs on 2GB VMs. K8s is never required.

Rationale: Nomad is lightweight (~80MB RAM), edge-capable, supports container workloads natively. The agent-sandbox CRD shape (Sandbox / Template / Claim / WarmPool) is worth copying as API mental model, but the K8s implementation is not.

Conflicts with: (none)
Enables: edge deployment, cheap fleet nodes
Blocks: adopting k8s-sigs/agent-sandbox directly (must reimplement concepts over Nomad)

**Relationships:**
- constrains → D3
- constrains → D3
- related_to → D3
- related_to → D3
- related_to → D3
- enables → D3

---

## D4: Cold migration only — no live migration in v0

**Status**: implemented
**Date**: 2026-04-25T12:44:49Z

Context: Live migration requires CRIU or memory snapshots, which are CPU/kernel-coupled. Cross-substrate live migration is not solved by anyone. Firecracker itself can't cross Intel-AMD.

Decision: All substrate changes are cold: stop agent, export FS, destroy form, instantiate on new substrate, import FS, start agent. Brief downtime accepted.

Rationale: Cold migration via OCI + tar is the only honest portable answer. Live migration within a single substrate (Fly-Fly, Daytona-Daytona) can use provider-native suspend/resume as an optimization, but that's provider-optional, not core.

Conflicts with: (none)
Enables: substrate adapter contract stays tiny (6 verbs)
Blocks: sub-second cross-substrate migration (explicitly out of scope)

**Relationships:**
- constrains → D4
- related_to → D4
- related_to → D4
- related_to → D4
- enables → D4
- enables → D4

---

## D5: MCP + skills as primary user interface (not CLI)

**Status**: implemented
**Date**: 2026-04-25T12:44:49Z

Context: In 2026, nobody runs CLI commands manually. Agent builders interact via their coding agent (Claude Code, Cursor, etc.). CLI-first design assumes manual operation that doesn't match user behavior.

Decision: Primary interface is MCP server + skills. Users talk to their agent, the agent talks to Mesh via MCP. CLI exists as a thin debugging/automation surface, not the primary UX.

Rationale: Agents managing their own bodies (spawn, snapshot, burst) naturally call MCP tools. A CLI for this would be wrapping MCP calls anyway — cut the middleman.

Conflicts with: (none)
Enables: recursive self-management (agents call MCP to manage their own bodies)
Blocks: (none)

**Relationships:**
- related_to → D5
- related_to → D5

---

## D6: Provider integrations are plugins — go-plugin for v1.0, OpenAPI codegen for v1.1

**Status**: accepted
**Date**: 2026-04-25T12:44:49Z

Context: Maintaining 13+ provider integrations in core was a maintenance burden. v1.0 implementation uses go-plugin + gRPC + protobuf. DE4 (from v1.1 design session) specifies OpenAPI + oapi-codegen v2 + AI mapping layer as the v1.1 generation pipeline, superseding the earlier Pulumi approach.

Decision: Core contains zero provider-specific code. Each provider is a plugin with a standard interface. Plugins can be AI-generated. Core ships with a plugin template and testing scaffold.

Rationale: Less code, fewer bugs, fewer security issues. Users own their provider code. No central maintenance burden.

[Implementation note: go-plugin+gRPC plugin system fully built. OpenAPI codegen pipeline NOT built. AI generation NOT built.]

**Relationships:**
- constrains → D6
- related_to → D6
- related_to → D6
- related_to → D6
- related_to → D6
- related_to → D6
- related_to → D6
- related_to → D6
- related_to → D6
- supersedes → D6

---

## D7: Agent body = container, not VM

**Status**: implemented
**Date**: 2026-04-25T12:44:49Z

Context: VMs give isolation but are heavy (minimum ~512MB overhead). Firecracker microVMs need /dev/kvm, not nestable on most cloud VMs. Containers are universally runnable, OCI-standard, and lightweight.

Decision: An agent body runs as a container. Not a VM, not a microVM. Substrates that offer microVM isolation (Daytona, E2B) wrap the container — that's their implementation detail.

Rationale: Containers are the universal unit. Every substrate in the landscape can run OCI containers. The body format (D2) is container-native.

Conflicts with: (none)
Enables: D2 (OCI image format), substrate adapter simplicity
Blocks: microVM-native features (memory snapshot at VM boundary)

**Relationships:**
- related_to → D7
- related_to → D7
- enables → D2

---

## D8: Inflatable container / PID-1 supervisor

**Status**: deferred
**Date**: 2026-04-25T12:44:49Z

Context: A sidecar binary (Go or Rust) at PID 1 that accepts deflate (shrink footprint) and inflate (restore) commands. Lets a body make room for siblings on the same VM without full hibernate.

Decision: Deferred to post-v0. The simpler path (D4 cold migration) handles the move between substrates case. In-place deflation without downtime is a v2 optimization.

Rationale: Requires agent cooperation (standby mode protocol), Nomad integration for resource resizing, and careful signal handling. Cold migration via snapshot+restore covers 80% of the use case with 10% of the complexity.

Conflicts with: (none — complementary to cold migration)
Enables: denser VM packing, faster scale-down than snapshot cycle
Blocks: (none)

---

## D9: Traefik / INGRESS / PRODUCTION tiers

**Status**: discarded
**Date**: 2026-04-25T12:44:49Z

Context: Previous Mesh MVP stripped Traefik and INGRESS/PRODUCTION tiers. Traefik deployment explicitly returned False (not yet automated). Caddy works for LITE/STANDARD.

Decision: Permanently discarded. Mesh is an agent substrate, not a web hosting platform. Ingress (if needed) is a agent's concern, not Mesh's.

Rationale: The old lightweight Kubernetes framing is dead. Mesh doesn't need ingress tiers. Agents that serve HTTP manage their own routing.

Conflicts with: (none — already stripped from codebase)
Enables: simpler core
Blocks: (nothing worth blocking)

---

## DE1: All sandbox providers equal — no primary designation for v1.1

**Status**: accepted
**Date**: 2026-05-03T13:56:51Z

All orchestrators are equal within their pool; all provisioners are equal within their pool. But orchestrators and provisioners are fundamentally different interfaces (DE17). Within each pool, no adapter gets special treatment — Nomad is not privileged over a future K8s adapter; Hetzner is not privileged over DigitalOcean. The old flat 'all substrates equal' framing is replaced by per-pool equality.

**Relationships:**
- related_to → D6
- related_to → DE1
- related_to → D10

---

## DE2: Static scheduler config for v1.1 — no auto-scheduling

**Status**: accepted
**Date**: 2026-05-03T13:57:05Z

Config specifies orchestrator + provisioner pairing, not a flat substrate name. User configuration declares which orchestrator to use (e.g., nomad) and which provisioner (e.g., hetzner). The daemon routes body lifecycle operations to the configured orchestrator and compute operations to the configured provisioner. Replaces the flat substrate routing model.

**Relationships:**
- related_to → D6

---

## DE3: Filesystem delta merge deferred to v2.0

**Status**: accepted
**Date**: 2026-05-03T13:57:17Z

Filesystem delta merging (clone body → run task on sandbox → merge changes back to parent) is DEFERRED beyond v1.1. Rationale: (1) A4 persona says 'optional merge' — not a hard requirement. Clone-and-run (no merge) satisfies the primary use case: burst compute, collect output artifacts (stdout/stderr/files), destroy clone. (2) Three-way merge semantics (base → parent changes + clone changes) add enormous complexity — conflict resolution, overlay diffing, selective merge paths — for a feature with no proven demand signal. (3) Technical approaches (OverlayFS diff, git-style merge) each have sharp edges and failure modes that aren't worth solving for v1. (4) If a user truly needs merge, they can implement it at the agent level (the clone outputs changes as a patch/diff that the parent applies). What v1.1 DOES support: clone body from snapshot → run on sandbox → collect output artifacts (files written to a specific output directory) → destroy clone. No filesystem merge. The output directory is explicitly specified by the user in the clone command. This keeps the burst clone primitive (A4) functional without the merge complexity.

**Relationships:**
- related_to → D4

---

## DE4: Pulumi unsuitable for Mesh — use OpenAPI + SDK + template pipeline

**Status**: accepted
**Date**: 2026-05-03T13:57:34Z

This decision re-evaluates D6 ('Provider integrations are plugins, AI-generated via Pulumi skill'). While the PLUGIN architecture in D6 is correct (provider integrations SHOULD be plugins), the AI-GENERATION aspect via Pulumi is NOT viable for v1.1. Findings: (1) Pulumi AI/Neo generates infrastructure-as-code (Pulumi programs in TypeScript/Python/Go), NOT plugin code. The generated code manages cloud resources — it doesn't implement a SubstrateAdapter interface. (2) The 'wrap in adapter' approach requires an LLM to bridge from Pulumi code to SubstrateAdapter interface — this is untested and likely brittle. (3) Quality gates are undefined — what 'passes' for a generated plugin? Compiles? Passes interface compliance check? Actually provisions correctly? Handles edge cases? (4) Maintenance model is unclear — who fixes bugs in generated plugins? What happens when provider API changes?

Decision: Pulumi is unsuitable for Mesh plugin generation. The correct approach is an OpenAPI + SDK + template pipeline: (1) Obtain provider OpenAPI spec, (2) Generate typed Go client with oapi-codegen v2, (3) Apply AI mapping layer to bridge generated client to SubstrateAdapter interface, (4) Output compilable plugin with tests.

For v1.1, the Fly Machines adapter and Docker adapter are hand-written reference implementations. The OpenAPI codegen pipeline is the v1.1 plugin generation path. This decision does NOT reject D6 — it refines the generation method from Pulumi to OpenAPI codegen.

**Relationships:**
- related_to → DE4
- supersedes → D6

---

## DE5: Git-based plugin distribution for v1.1, binary registry deferred

**Status**: accepted
**Date**: 2026-05-03T13:57:49Z

Plugins in v1.1 are distributed via Git repositories — users install with: mesh plugin install github.com/user/mesh-substrate-fly. The daemon clones the repo, runs go build, and loads the resulting binary. Rationale: (1) Git-based install aligns with Go ecosystem conventions (go install, go get). (2) No central plugin registry to maintain (C3, C4). (3) Users can pin specific commits/tags for versioning. (4) Source availability enables security review. (5) Low infrastructure overhead — no binary hosting, no signing infrastructure. Plugin naming convention: mesh-substrate-<name> (e.g., mesh-substrate-fly, mesh-substrate-digitalocean). Plugin directory: ~/.mesh/plugins/mesh-substrate-<name>@<version>/. Versioning: plugins declare their protocol version via GetPluginInfo().gRPCProtocolVersion. The daemon checks compatibility at load time. Multiple versions can be installed simultaneously. Upgrade path: install new version, update config to point to new version, daemon drains in-flight requests from old version, then unloads it. Security: process-level isolation via HashiCorp go-plugin (subprocess). Plugin signing and binary registry are deferred to v2.0. Cross-language plugins (Python, Rust, TypeScript) are possible via gRPC but not prioritized — Go is the primary plugin SDK language for v1.1. This decision extends D6 (provider integrations are plugins) by specifying the distribution mechanism.

**Relationships:**
- related_to → D6

---

## DE6: Skills live in daemon core, MCP resources, and agent-side — a mixed model

**Status**: accepted
**Date**: 2026-05-03T13:58:05Z

Mesh skills are classified into three tiers: TIER 1 (Daemon Core): Body lifecycle operations — create, start, stop, destroy, snapshot, restore, migrate, clone. These are compiled into the Mesh daemon binary. They are the 'operating system' of Mesh — always available, no external dependency. TIER 2 (MCP Resources): Operational skills — pack-vm (pack multiple bodies on one VM), warm-pool (pre-provision sandbox instances), garbage-collect (clean up orphaned instances and old snapshots). These are served as MCP resources by the daemon but implemented as separate Go packages that the daemon loads. They are optional — users can enable/disable them in config. TIER 3 (Agent-Side Compositions): User-level skills that compose Mesh primitives — e.g., 'nightly-reflection' (snapshot + clone + run analysis + collect results). These live in the user's AI agent (Claude Code skill, Codex skill, etc.), not in Mesh. The agent calls Mesh MCP tools to execute the primitives. Rationale: (1) Core lifecycle MUST be daemon-resident for reliability (no plugin crash can break body management). (2) Operational skills are medium-complexity compositions — they belong in the daemon for performance but shouldn't bloat the core binary. (3) Agent-side compositions let users build custom workflows without modifying Mesh. This model keeps the daemon small (C6), enables extensibility (D6), and provides a clear 'what goes where' boundary. Specifically: pack-vm and garbage-collect are Tier 2 MCP resources for v1.1. warm-pool is deferred to v2.0 (pre-provisioning requires scheduler integration first).

**Relationships:**
- related_to → D5

---

## DE7: Production hardening priorities for v1.1

**Status**: accepted
**Date**: 2026-05-03T13:58:23Z

Ranked production hardening improvements for v1.1, ordered by risk and user impact:

P1 CRITICAL - must fix before v1.1:
(a) Snapshot corruption recovery: SHA-256 verification on every pull, automatic retry from backup copy (local + S3 = two copies). Failed snapshots are quarantined, not deleted.
(b) Daemon crash resilience: SQLite WAL mode auto-recovers on restart. Reconciliation loop on daemon startup — scan Nomad/Docker for running bodies not in store, add them; check store entries against actual substrate state, fix discrepancies.

P2 HIGH - should fix:
(c) Secrets management: extend config to support env-var references with validation (ENV_VAR or ENV_VAR:default syntax). No plaintext secrets in config files. Plugin-to-provider auth via env vars only.
(d) Daemon PID file validation: check not just PID exists but that process at that PID is actually the Mesh daemon (verify via /proc/PID/cmdline on Linux).

P3 MEDIUM - nice to have:
(e) Networking warning without Tailscale: if MCP is exposed over HTTP without Tailscale, daemon emits warning on startup. Add mesh serve --insecure flag to explicitly acknowledge the risk.
(f) Daemon upgrades: SIGTERM the daemon, wait for graceful shutdown (drain MCP connections, complete in-flight migrations), start new version. Bodies on fleet VMs NOT affected by daemon restart — they keep running under Nomad.

P4 DEFER to v2.0:
(g) Zero-downtime daemon upgrades (rolling, blue-green).
(h) Daemon discovery after machine death (new daemon rediscovers bodies on fleet VMs).
(i) Body-to-body communication (fleet networking between bodies under Nomad).

[Implementation note: P1 (snapshot corruption recovery via SHA-256) DONE. P2 (crash resilience via migration resume + reconcile) DONE. P3 (secrets management) NOT DONE. P4 (audit logging) NOT DONE.]

---

## DE8: Docker adapter is a plugin, not built-in (EQ1)

**Status**: rejected
**Date**: 2026-05-03T13:58:38Z

INVALIDATED by DE17. Docker is not a Mesh adapter at all. Mesh never touches container runtimes directly (DE17 two-pool architecture). Docker is managed by orchestrators (Nomad via its Docker task driver). Making Docker a 'plugin' in Mesh is wrong because Docker is not a Mesh concern — it's an implementation detail of the orchestrator. The concept of 'Docker adapter as plugin' assumed a flat SubstrateAdapter model that has been replaced by the two-pool architecture (DE17).

**Relationships:**
- related_to → D7
- related_to → D6
- supersedes → DE8
- supersedes → DE8

---

## DE9: Single daemon architecture is correct for v1.1 (EQ2)

**Status**: implemented
**Date**: 2026-05-03T13:58:53Z

Resolves EQ2: Single-daemon architecture is correct for v1.1. A single Mesh daemon serves as the control plane for all user bodies across substrates. It maintains the SQLite store, manages MCP connections, and orchestrates body lifecycle. The daemon communicates with:
- Local: Docker plugin (local Docker daemon)
- Fleet: Nomad plugin (remote Nomad cluster)
- Sandbox: Fly Machines plugin (Fly API)
Bodies themselves run on their respective substrates (local Docker, fleet VM via Nomad, Fly sandbox), NOT in the daemon process. The daemon is a lightweight control plane (~50MB RAM), not a compute host. For a user with 50 agents across 10 VMs: the single daemon manages all 50 bodies. The daemon doesn't need to be on the same machine as the bodies. It can run on a laptop, a Pi, or a small VM. The daemon is a single point of control (SPOC), but bodies continue running even if the daemon stops (they're on Nomad/Fly/Docker, not in the daemon). The daemon is a single point of failure only for management operations — running bodies are unaffected. For v2.0, consider multi-daemon for high availability, but v1.1's single-daemon design is sufficient.

---

## DE10: SQLite store backup is manual dump + reconciliation (EQ4)

**Status**: accepted
**Date**: 2026-05-03T13:59:07Z

Resolves EQ4: SQLite backup strategy for v1.1 is: (1) SQLite WAL mode provides crash resilience — the database auto-recovers to the last committed transaction on restart. (2) Manual backup: mesh db backup writes a SQL dump to ~/.mesh/backups/state-YYYYMMDD-HHMMSS.sql. This is a human-initiated operation, not automatic. (3) No automatic backup in v1.1 — the store is small (metadata only: body IDs, states, snapshot locations). Losing it means losing track of bodies, not losing bodies themselves (they're on Nomad/Fly/Docker). Recovery: rescan substrates and rebuild the store from what's actually running (reconciliation). (4) For v2.0: periodic auto-backup with configurable interval, optional S3 backup target. Rationale: the SQLite store is metadata, not data. Bodies' filesystems are in snapshots (S3/R2/Fly volumes). The store is recoverable via reconciliation.

[Implementation note: WAL mode enabled. Reconciliation on startup works. Manual backup command (mesh backup) NOT built.]

**Relationships:**
- related_to → D4

---

## DE11: Daemon upgrades: stop-restart, bodies unaffected (EQ3)

**Status**: accepted
**Date**: 2026-05-03T13:59:20Z

Resolves EQ3: Daemon upgrades for v1.1 follow a stop-restart model: (1) SIGTERM the daemon, wait up to 30s for graceful shutdown (drain active MCP connections, complete any in-flight snapshots/migrations). (2) Start new daemon version — it reads the existing SQLite store and PID file. (3) Bodies on substrates (Docker, Nomad, Fly) are NOT affected — they keep running. The daemon reconciles on startup: rescan substrates, match running instances to store entries, add any new ones. No bodies are stopped during upgrade. Rolling upgrades and zero-downtime are deferred to v2.0. The daemon is NOT in the hot path for body execution — bodies run their own processes on substrates. The daemon only orchestrates lifecycle operations, so a brief restart (seconds) doesn't impact running agents.

[Implementation note: Graceful shutdown + reconcile on restart works. No explicit upgrade path or version migration logic.]

---

## DE12: Migration testing: mock layer + Docker-to-Docker CI + manual Fly/Nomad (EQ5)

**Status**: accepted
**Date**: 2026-05-03T13:59:35Z

Resolves EQ5: Testing cross-substrate migration for v1.1 uses a layered approach: LAYER 1 (unit tests): Mock SubstrateAdapter interface for each adapter. Test migration coordinator with mocks — verify the 7-step sequence (export, provision, transfer, import, verify, switch, cleanup) with injected failures at each step. LAYER 2 (integration tests): Docker-to-Docker migration on the same machine. Uses real Docker daemon. Tests: snapshot body on Docker, restore to new Docker container, verify SHA-256 match. LAYER 3 (manual integration tests): Docker-to-Fly migration requires real Fly API credentials. Run as a manual test script (mesh-test migrate --from docker --to fly). Produces a report. LAYER 4 (CI/CD): Docker-to-Docker migration test runs in GitHub Actions (Docker-in-Docker). Fly and Nomad tests are manual-only until we have CI-accessible test clusters. NOT in scope for v1.1: automated Fly/Nomad migration tests in CI. These require real API credentials and infrastructure.

---

## DE13: Fly Machines adapter: accept no-filesystem-export limitation

**Status**: accepted
**Date**: 2026-05-03T13:59:49Z

Fly adapter as first provisioner implementation. Fly Machines API implements ProvisionerAdapter (CreateMachine, DestroyMachine, ListMachines, GetStatus). This validates the provisioner interface design and the AI-generation pipeline (DE4, DE15). Fly is a provisioner, not the old flat substrate adapter.

**Relationships:**
- related_to → DE1

---

## DE14: database/sql pattern as Phase 1 plugin architecture

**Status**: accepted
**Date**: 2026-05-03T14:00:03Z

Both OrchestratorAdapter and ProvisionerAdapter registries use the database/sql driver pattern. Core interfaces remain minimal. Optional capabilities exposed via extension interfaces with type assertion discovery. Registration via init() side-effect pattern. This applies to BOTH adapter pools (orchestrators and provisioners), not the old single SubstrateAdapter which has been replaced by the two-pool architecture (DE17).

**Relationships:**
- related_to → D6
- related_to → DE14
- related_to → DE14

---

## DE15: oapi-codegen v2 as primary codegen tool for plugin generation

**Status**: accepted
**Date**: 2026-05-03T14:00:19Z

Context: Plugin generation requires converting provider OpenAPI specs into typed Go client code. Multiple tools were evaluated: swagger-codegen, openapi-generator, oapi-codegen v1, oapi-codegen v2, and custom templates.

Decision: oapi-codegen v2 (github.com/oapi-codegen/oapi-codegen) is the primary codegen tool for Mesh plugin generation. It generates typed Go clients, server stubs, and types from OpenAPI 3.0/3.1 specs with strict output control via configuration file.

Rationale: (1) Native Go — generates idiomatic Go code without Java dependency. (2) OpenAPI 3.1 support — modern spec format used by most providers. (3) Configurable output — can generate only client, only types, or full server via YAML config. (4) Active maintenance — v2 is actively maintained with regular releases. (5) Mesh-compatible — output can be wrapped by AI mapping layer to implement SubstrateAdapter. (6) No runtime dependency — generated code is pure Go with standard library HTTP client.

Alternatives rejected: swagger-codegen (Java dependency, heavy), openapi-generator (Java dependency, overwhelming output), oapi-codegen v1 (deprecated, limited OpenAPI 3.0 support), custom templates (maintenance burden, no spec validation).

Conflicts with: (none)
Enables: rapid provider adapter generation from published OpenAPI specs
Blocks: direct Pulumi integration (DE4), custom codegen tooling

**Relationships:**
- related_to → DE4

---

## DE16: Extension interfaces for optional capabilities, not ErrNotImplemented passthrough

**Status**: accepted
**Date**: 2026-05-03T14:00:35Z

Extension interfaces and type assertion for optional capabilities in both adapter types. OrchestratorAdapter extensions: Executor (exec into body), Logger (stream logs), Inspector (detailed status). ProvisionerAdapter extensions: Snapshotter (machine snapshots), NetworkConfigurator (network setup), LogFetcher (cloud-init output). Core checks via type assertion before calling extension methods. Applies to both pools per DE17.

**Relationships:**
- related_to → DE14
- related_to → DE16

---

## DE17: Two-pool adapter architecture

**Status**: accepted
**Date**: 2026-05-03T18:43:44Z

Two-pool adapter architecture: Orchestrators + Provisioners. The single SubstrateAdapter interface is replaced by two independent interfaces — OrchestratorAdapter (body lifecycle: schedule, stop, destroy, snapshot, restore) and ProvisionerAdapter (compute lifecycle: create machine, destroy, list, get status). These are independent concerns with different implementations and plugin strategies. Orchestrators (Nomad, future K8s/Incus) are hand-written core adapters. Provisioners (Hetzner, DO, Daytona, E2B) are AI-generated from OpenAPI specs. Mesh never touches container runtimes directly — that is the orchestrator's responsibility.

**Relationships:**
- related_to → DE14
- related_to → DE16
- related_to → DE17
- related_to → DE17
- related_to → DE17
- supersedes → DE8
- supersedes → DE8

---

## DE18: Mesh never touches container runtimes directly

**Status**: accepted
**Date**: 2026-05-03T18:44:03Z

Mesh never touches container runtimes (Docker, LXC) directly. Container runtimes are managed by orchestrators — Nomad manages Docker via its task driver, Incus manages LXC containers. Mesh talks to orchestrators only. The existing docker/adapter.go that calls the Docker SDK directly is architectural drift and should be removed. When a user wants a Docker body, Mesh tells Nomad to schedule it — Nomad uses its Docker task driver internally.

**Relationships:**
- related_to → DE17

---

## DE19: Cloud-init/user-data bootstrap pattern

**Status**: accepted
**Date**: 2026-05-03T18:44:22Z

VM bootstrapping uses cloud-init/user-data pattern. The orchestrator generates a bootstrap script (e.g., install Nomad, join cluster). The provisioner passes it as user-data at VM creation time. Neither adapter knows about the other. The daemon coordinates: (1) asks orchestrator for bootstrap config, (2) asks provisioner to create VM with that user-data, (3) polls machine status until running, (4) polls orchestrator node status until joined. No SSH in core. First node uses InitializeCluster() for leader bootstrap. Subsequent nodes use GetBootstrapConfig(role=client, leaderAddr). Matches kubeadm and Nomad's own patterns.

**Relationships:**
- related_to → DE17

---

## DE20: Local support deferred

**Status**: accepted
**Date**: 2026-05-03T18:44:40Z

Local support (Docker on laptop, Lima/Colima, direct-Docker orchestration) is deferred to a later version. v1 focuses on fleet (cloud VMs + Nomad orchestration) and sandbox capability (burst containers via provisioner). Local development requires separate research into Lima/Colima positioning and whether a lightweight local orchestrator is needed. Research found: Lima is a local VM provisioner for macOS, Colima wraps Lima with Docker/Incus runtime setup. LXC support via Nomad was dropped (Oct 2024). These need dedicated research before designing local support.

**Relationships:**
- related_to → DE17

---

