# Decisions

> Each decision has an ID (D1, D2, ...) for cross-referencing.
> Status: accepted | deferred | discarded | superseded
> Every new decision MUST list conflicts and enables.

---

### D1: Filesystem-only snapshot (no memory state)

- **Status**: accepted
- **Context**: Research showed memory snapshots (CRIU, Firecracker) are kernel/CPU-coupled and non-portable. Agents stop at task boundaries (graceful SIGTERM), not mid-thought. Memory is disposable.
- **Decision**: Snapshot = capture the container filesystem only. No memory state, no CRIU, no process-tree dump. Agents are stopped cleanly, then their FS is captured.
- **Rationale**: Eliminates kernel/CPU coupling entirely. Makes bodies fully portable across all substrates. Matches actual agent lifecycle (stop after task, not during).
- **Conflicts with**: (none — no prior decision contradicts this)
- **Enables**: D2 (OCI + tar as portable format), D4 (cold migration), D3 (Nomad on 2GB VMs)
- **Blocks**: live-migration paths (explicitly deferred)

---

### D2: OCI image + volume tarball as portable body format

- **Status**: accepted
- **Context**: `docker commit` grows layered images monotonically and doesn't compose with repeated mutation. Nobody in production uses it for persistence. `docker export` produces a flat tarball of the live FS — captures everything, no overlay drama.
- **Decision**: Agent body = base OCI image (immutable template) + exported filesystem tarball (mutable state). `docker export | zstd` for snapshots. Restores on any OCI-compatible runtime.
- **Rationale**: Flat tarball is the only universally portable format. Every substrate can accept it. No whiteout metadata, no layer chain, no kernel coupling.
- **Implementation note**: Pre-snapshot prune hook runs `pip cache purge`, `huggingface-cli delete-cache`, `rm -rf /tmp/*`, `apt-get clean` inside the container before export. Agent-lifecycle concern, not runtime concern.
- **Conflicts with**: (none)
- **Enables**: D4 (cold migration), substrate adapter simplicity (6 verbs only)
- **Blocks**: (none)

---

### D3: Nomad as fleet scheduler (not K8s)

- **Status**: accepted
- **Context**: kubernetes-sigs/agent-sandbox exists but is K8s-to-the-bone (PVC, headless Service, RuntimeClass, NetworkPolicy, HPA). Dead on 2GB VMs. No serious Nomad-based agent-sandbox project exists in OSS — genuine whitespace.
- **Decision**: Fleet pool uses Nomad as the scheduler. Nomad runs on 2GB VMs. K8s is never required.
- **Rationale**: Nomad is lightweight (~80MB RAM), edge-capable, supports container workloads natively. The `agent-sandbox` CRD shape (Sandbox / Template / Claim / WarmPool) is worth copying as API mental model, but the K8s implementation is not.
- **Conflicts with**: (none)
- **Enables**: edge deployment, cheap fleet nodes
- **Blocks**: adopting k8s-sigs/agent-sandbox directly (must reimplement concepts over Nomad)

---

### D4: Cold migration only — no live migration in v0

- **Status**: accepted
- **Context**: Live migration requires CRIU or memory snapshots, which are CPU/kernel-coupled. Cross-substrate live migration is not solved by anyone. Firecracker itself can't cross Intel↔AMD.
- **Decision**: All substrate changes are cold: stop agent → export FS → destroy form → instantiate on new substrate → import FS → start agent. Brief downtime accepted.
- **Rationale**: Cold migration via OCI + tar is the only honest portable answer. Live migration within a single substrate (Fly→Fly, Daytona→Daytona) can use provider-native suspend/resume as an optimization, but that's provider-optional, not core.
- **Conflicts with**: (none)
- **Enables**: substrate adapter contract stays tiny (6 verbs)
- **Blocks**: sub-second cross-substrate migration (explicitly out of scope)

---

### D5: MCP + skills as primary user interface (not CLI)

- **Status**: accepted
- **Context**: In 2026, nobody runs CLI commands manually. Agent builders interact via their coding agent (Claude Code, Cursor, etc.). CLI-first design assumes manual operation that doesn't match user behavior.
- **Decision**: Primary interface is MCP server + skills. Users talk to their agent, the agent talks to Mesh via MCP. CLI exists as a thin debugging/automation surface, not the primary UX.
- **Rationale**: Agents managing their own bodies (spawn, snapshot, burst) naturally call MCP tools. A CLI for this would be wrapping MCP calls anyway — cut the middleman.
- **Conflicts with**: (none)
- **Enables**: recursive self-management (agents call MCP to manage their own bodies)
- **Blocks**: (none)

---

### D6: Provider integrations are plugins, AI-generated via Pulumi skill

- **Status**: accepted
- **Context**: Maintaining 13+ provider integrations in core was a maintenance burden. Pulumi now has a skill that generates provider code. Users who need a provider can generate a plugin.
- **Decision**: Core contains zero provider-specific code. Each provider (DigitalOcean, AWS, Daytona, etc.) is a plugin with a standard interface. Plugins can be AI-generated. Core ships with a plugin template and testing scaffold.
- **Rationale**: Less code, fewer bugs, fewer security issues. Users own their provider code. No central maintenance burden.
- **Conflicts with**: (none)
- **Enables**: community growth without core maintenance scaling
- **Blocks**: "just works" experience for all 13+ providers out of the box (users must generate/install plugins)

---

### D7: Agent body = container, not VM

- **Status**: accepted
- **Context**: VMs give isolation but are heavy (minimum ~512MB overhead). Firecracker microVMs need `/dev/kvm`, not nestable on most cloud VMs. Containers are universally runnable, OCI-standard, and lightweight.
- **Decision**: An agent body runs as a container. Not a VM, not a microVM. Substrates that offer microVM isolation (Daytona, E2B) wrap the container — that's their implementation detail.
- **Rationale**: Containers are the universal unit. Every substrate in the landscape can run OCI containers. The body format (D2) is container-native.
- **Conflicts with**: (none)
- **Enables**: D2 (OCI image format), substrate adapter simplicity
- **Blocks**: microVM-native features (memory snapshot at VM boundary)

---

### D8: Inflatable container / PID-1 supervisor

- **Status**: deferred
- **Context**: A sidecar binary (Go or Rust) at PID 1 that accepts `deflate` (shrink footprint) and `inflate` (restore) commands. Lets a body make room for siblings on the same VM without full hibernate.
- **Decision**: Deferred to post-v0. The simpler path (D4 cold migration) handles the "move between substrates" case. In-place deflation without downtime is a v2 optimization.
- **Rationale**: Requires agent cooperation (standby mode protocol), Nomad integration for resource resizing, and careful signal handling. Cold migration via snapshot+restore covers 80% of the use case with 10% of the complexity.
- **Conflicts with**: (none — complementary to cold migration)
- **Enables**: denser VM packing, faster scale-down than snapshot cycle
- **Blocks**: (none)

---

### D9: Traefik / INGRESS / PRODUCTION tiers

- **Status**: discarded
- **Context**: Previous Mesh MVP stripped Traefik and INGRESS/PRODUCTION tiers. Traefik deployment explicitly returned False ("not yet automated"). Caddy works for LITE/STANDARD.
- **Decision**: Permanently discarded. Mesh is an agent substrate, not a web hosting platform. Ingress (if needed) is a agent's concern, not Mesh's.
- **Rationale**: The old "lightweight Kubernetes" framing is dead. Mesh doesn't need ingress tiers. Agents that serve HTTP manage their own routing.
- **Conflicts with**: (none — already stripped from codebase)
- **Enables**: simpler core
- **Blocks**: (nothing worth blocking)

---

### D10: Mesh is separate from Daytona — no integration

- **Status**: accepted
- **Context**: Daytona (72k stars, AGPL 3.0) is a managed AI code execution platform. Research showed fundamental mismatches with Mesh's constraints and goals.
- **Decision**: Mesh builds independently. Daytona is not a substrate, not a dependency, not a platform component. Mesh may reference Daytona's patterns (MCP implementation, provider plugin architecture, Tailscale networking) but does not use Daytona code or depend on it.
- **Rationale**:
  1. Resource mismatch: Daytona requires 8-16GB RAM (11-service stack). Mesh targets 2GB VMs. Non-negotiable gap.
  2. No body abstraction: Daytona workspaces are platform-bound (state in PostgreSQL + S3 + container). No portable identity surviving substrate changes.
  3. Central dependency: Daytona IS a control plane. Mesh constraint is "no central dependency" (C4). Architecturally opposite.
  4. AGPL 3.0: Modifying Daytona triggers copyleft. Commercial license costs money.
  5. Different markets: Daytona = "Heroku for AI code execution" (managed, SaaS). Mesh = "Nomad + Docker + Tailscale for agent bodies" (self-hosted, portable).
- **Conflicts with**: (none — no prior decision contradicts this)
- **Enables**: independent substrate adapter design, lightweight core path, 2GB VM deployment
- **Blocks**: Daytona as a substrate provider option (explicitly excluded)
- **Research source**: `research/daytona-analysis.md`
