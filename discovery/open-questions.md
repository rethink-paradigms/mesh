# Open Questions

> Status: unresolved | in research | blocked
> Each question links to relevant decisions and research.

---

### Q1: Where does a body live when idle?

- **Status**: unresolved
- **Context**: Three substrate pools: Local (laptop/Pi), Fleet (BYO VMs via Nomad), Sandbox (Daytona, E2B, Fly). A body at rest — no tasks, no inference — where does it sit? What's the idle cost?
- **Why it matters**: Determines what the scheduler optimizes for. If idle bodies on Fleet VMs cost $0 (VM already running), that's different from Sandbox (billed per-second while running).
- **Related decisions**: D3 (Nomad), D4 (cold migration), D7 (container body)
- **Hypothesis**: Idle bodies should be snapshotted and destroyed on Sandbox (pay nothing), kept running on Fleet (VM already paid for), optionally kept on Local.

---

### Q2: Registry — where do body snapshots live?

- **Status**: unresolved
- **Context**: Body = OCI image + FS tarball. Where are these stored? Docker Hub? User's S3/GCS? Provider-native storage? Something Mesh manages?
- **Why it matters**: Defines the dependency surface. If Mesh manages a registry, that's central infrastructure (violates C3/C4). If user brings their own, onboarding friction.
- **Related decisions**: D2 (OCI + tar format), D6 (plugin architecture)
- **Hypothesis**: User-configured blob storage (S3/GCS/R2) as default plugin. OCI image push to any registry the user has credentials for.

---

### Q3: Scheduler — is substrate selection core or plugin?

- **Status**: unresolved
- **Context**: When a user says "deploy my agent", something decides: Fleet VM? Sandbox? Which provider? Is that decision logic in Mesh core, or is it a plugin?
- **Why it matters**: If core, Mesh has opinions about placement. If plugin, users bring their own scheduler. Affects how smart Mesh needs to be.
- **Related decisions**: D3 (Nomad), D6 (plugins)
- **Hypothesis**: Core-but-trivial: default scheduler picks cheapest available substrate. Overridable via plugin for users with complex needs.

---

### Q4: Bootstrap — how does the first "install mesh" happen?

- **Status**: unresolved
- **Context**: D5 says MCP is the primary interface. But MCP requires a running Mesh. Chicken-and-egg: you can't install Mesh via MCP because Mesh isn't running yet.
- **Why it matters**: First-run experience defines whether people get past "hello world."
- **Related decisions**: D5 (MCP primary), D6 (plugins)
- **Hypothesis**: One-liner shell bootstrap (`curl ... | bash` or `pip install`) that installs Mesh + starts a minimal local agent. From there, MCP takes over.

---

### Q5: Daytona OSS — coexist or diverge?

- **Status**: resolved (D10)
- **Context**: Daytona (72k stars, active) explicitly sells stateful snapshots for agent sandboxes. Their OSS core may overlap heavily with what we're building. Need to understand their architecture to answer: can Mesh run on top of Daytona? Alongside? Must it be separate?
- **Why it matters**: If Daytona's OSS solves 80% of this, building Mesh is redundant. If it assumes hosted/K8s/own-orchestrator and breaks on 2GB edge VMs, there's clear space.
- **Related decisions**: D3 (Nomad not K8s), C1 (2GB VMs), C2 (no K8s)
- **Next step**: ~~Read Daytona OSS repo architecture — substrate assumptions, snapshot implementation, networking model.~~
- **Resolution**: Mesh must be separate from Daytona. Fundamental mismatches: 8-16GB RAM vs 2GB target, no portable body abstraction, monolithic control plane vs no-central-dependency. Daytona is a reference for patterns, not a component. See D10 and `research/daytona-analysis.md`.
