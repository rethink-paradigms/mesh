# Hard Constraints

> These are non-negotiable boundaries extracted from decisions and intent.
> Any future design or decision MUST respect all of these.

---

### C1: Must run on 2GB VMs

- **Source**: D3 (Nomad as scheduler), user's edge/cheap-fleet requirement
- **Implication**: K8s is out. Nomad (~80MB) + Tailscale (~20MB) + Docker (~100MB) = ~200MB control plane. Leaves ~1.8GB for agent workloads.
- **Violated by**: Any design that requires etcd, apiserver, CSI drivers, or multi-GB runtimes.

### C2: Must not require K8s control plane

- **Source**: D3, explicit non-goal
- **Implication**: No CRDs, no PVC/CSI, no Service/NetworkPolicy, no controller-runtime. Concepts from k8s-sigs/agent-sandbox can be borrowed at the API-shape level, but the implementation must be Nomad-native or orchestrator-agnostic.

### C3: User owns all compute, keys, network

- **Source**: Intent (point 5 — you own your compute)
- **Implication**: No Mesh-controlled auth, no Mesh-hosted registry, no Mesh telemetry. Mesh is a tool that runs on the user's infra, not a platform the user logs into.

### C4: No telemetry, no login, no central dependency

- **Source**: Intent (point 5, point 10 — built for yourself first)
- **Implication**: No phone-home, no account system, no Mesh-controlled coordination server. Tailscale coordination is the user's Tailscale account (or headscale). Mesh never sees the user's infrastructure.

### C5: Portable across substrates — no kernel/CPU coupling for core path

- **Source**: D1 (FS-only snapshot), D2 (OCI + tar), D4 (cold migration)
- **Implication**: Core path uses OCI images and filesystem tarballs only. Provider-native optimizations (suspend/resume, memory snapshot) are optional accelerations within a single substrate, never required.

### C6: Core is tiny — provider code is plugin, not core library

- **Source**: D6 (plugin architecture), Intent (point 8 — core is tiny and stable)
- **Implication**: The core Mesh binary/library contains: body lifecycle, substrate adapter interface, MCP server, networking (Tailscale). Everything else is a plugin. Provider integrations, registry connectors, scheduler policies — all plugins.
