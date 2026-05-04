# Mesh Redesign — Discovery Index

> Last updated: 2026-05-04

## Status Dashboard

**Decisions:** 26 accepted | 1 rejected | 1 deferred | 1 discarded
**Open questions:** 4 unresolved | 0 in research
**Constraints:** 6 hard constraints established
**Research completed:** substrate-landscape, daytona-analysis, e2b-internals, snapshot-mechanics, agent-sandbox-k8s, substrate-adapter, registry-strategy, plugin-architecture, kimi_output
**Research pending:** (none — all complete)

## Decisions

| ID | Summary | Status |
|----|---------|--------|
| D1 | Filesystem-only snapshot (no memory state) | accepted |
| D2 | OCI image + volume tarball as portable body format | accepted |
| D3 | Nomad as fleet scheduler (not K8s) | accepted |
| D4 | Cold migration only — no live migration in v0 | accepted |
| D5 | MCP + skills as primary user interface (not CLI) | accepted |
| D6 | Provider integrations are plugins, AI-generated via Pulumi skill | accepted |
| D7 | Agent body = container, not VM | accepted |
| D8 | Inflatable container / PID-1 supervisor | deferred |
| D9 | Traefik / INGRESS / PRODUCTION tiers | discarded |
| D10 | Mesh is separate from Daytona — no integration | accepted |
| DE1 | All providers equal within their pool (orchestrators / provisioners) | accepted |
| DE2 | Config specifies orchestrator + provisioner pairing | accepted |
| DE3 | Delta-merge for incremental snapshot updates | accepted |
| DE4 | Pulumi unsuitable — use OpenAPI codegen pipeline | accepted |
| DE5 | Git-based plugin install and distribution | accepted |
| DE6 | Three skill tiers (core, community, generated) | accepted |
| DE7 | Production hardening roadmap (P1-P4) | accepted |
| DE8 | Docker adapter as plugin — REJECTED (superseded by DE17/DE18) | rejected |
| DE9 | Single daemon architecture | accepted |
| DE10 | Deterministic backup via SQLite .backup | accepted |
| DE11 | Atomic in-place upgrade with rollback | accepted |
| DE12 | Migration testing with real adapters | accepted |
| DE13 | Fly adapter as first provisioner implementation | accepted |
| DE14 | database/sql registration pattern for both adapter pools | accepted |
| DE15 | oapi-codegen v2 for SDK generation | accepted |
| DE16 | Extension interfaces for optional capabilities | accepted |
| DE17 | Two-pool adapter architecture (Orchestrators + Provisioners) | accepted |
| DE18 | Mesh never touches container runtimes directly | accepted |
| DE19 | Cloud-init/user-data bootstrap pattern | accepted |
| DE20 | Local support deferred | accepted |

## Open Questions

| # | Question | Status |
|---|----------|--------|
| Q1 | Where does a body live when idle? (Local / Fleet / Sandbox billing trade-off) | unresolved |
| Q2 | Registry strategy — where do body snapshots live? (Docker Hub / user S3 / provider-native) | unresolved |
| Q3 | Scheduler — is substrate selection core or plugin? | unresolved |
| Q4 | Bootstrap — how does the first "install mesh" happen without MCP? | unresolved |
| Q5 | Daytona OSS architecture — can we coexist or must we diverge? | resolved (D10) |

## Constraints

- C1: Must run on 2GB VMs (edge / cheap fleet nodes)
- C2: Must not require K8s control plane
- C3: User owns all compute, keys, network — no central dependency
- C4: No telemetry, no login, no mesh-controlled auth
- C5: Portable across substrates — no kernel/CPU coupling for core path
- C6: Core is tiny — provider code is plugin, not core library

## Agent Personas

| ID | Type | Primary Substrate | Key Need |
|----|------|-------------------|----------|
| A1 | Hermes Operator (heavy persistent) | Fleet | Periodic snapshot, burst to sandbox |
| A2 | Tool Agent (lightweight persistent) | Fleet (packed) | Deflate when idle, pack multiple |
| A3 | Ephemeral Task Runner | Sandbox | Fast spawn, collect output, destroy |
| A4 | Burst Clone (fork of persistent) | Sandbox | Snapshot parent, spawn clone, optional merge |
| A5 | Developer Agent (local) | Local | Burst to sandbox for heavy tasks |

See `state/personas.md` for full profiles, state patterns, and feature-needs matrix.

## Research Files

| File | Topic | Status |
|------|-------|--------|
| `research/substrate-landscape.md` | Stateful workload hosts comparison | complete |
| `research/daytona-analysis.md` | Daytona OSS architecture deep dive | complete |
| `research/e2b-internals.md` | E2B Firecracker sandbox mechanics | complete |
| `research/snapshot-mechanics.md` | Docker export, overlayfs, bloat management | complete |
| `research/agent-sandbox-k8s.md` | kubernetes-sigs/agent-sandbox analysis | complete |
| `research/substrate-adapter.md` | Substrate adapter interface — lifecycle verbs, compliance matrix | complete |
| `research/registry-strategy.md` | OCI registry and storage strategy — options and trade-offs | complete |
| `research/plugin-architecture.md` | Plugin system design — gRPC, go-plugin, Pulumi AI integration | complete |
| `research/kimi_output/` | Plugin architecture analysis from Kimi research | complete |
