# Open Questions

## Q1: Where does a body live when idle?

**Status**: resolved
**Date**: 2026-04-25T12:47:12Z

Partially resolved by DE2: user explicitly configures substrate in ~/.mesh/config.yaml. Idle location is user choice, not system decision. Fleet for persistent (A1/A2), Local for development (A5), Sandbox for burst (A3/A4). Full cost-model optimization is a v2.0 concern.

**Relationships:**
- related_to → D3
- related_to → D4
- related_to → D7

---

## Q2: Registry — where do body snapshots live?

**Status**: resolved
**Date**: 2026-04-25T12:47:13Z

Resolved by v1.0 implementation: S3/R2 for snapshot storage via registry plugin. Local snapshots at ~/.mesh/snapshots/. DE10 covers SQLite store backup (separate concern).

**Relationships:**
- related_to → D2
- related_to → D6

---

## Q3: Scheduler — is substrate selection core or plugin?

**Status**: resolved
**Date**: 2026-04-25T12:47:13Z

Resolved by DE2: static scheduler config for v1.1. Substrate selection is user-config static — neither core nor plugin. Plugin-based scheduler is v2.0 consideration.

**Relationships:**
- related_to → D3
- related_to → D6

---

## Q4: Bootstrap — how does the first install mesh happen?

**Status**: resolved
**Date**: 2026-04-25T12:47:13Z

Resolved by v1.0 implementation: CLI bootstrap via mesh init + install.sh/Homebrew formula. MCP is primary ongoing interface, initial installation is CLI-based.

**Relationships:**
- related_to → D5
- related_to → D6

---

## Q5: Daytona OSS — coexist or diverge?

**Status**: resolved
**Date**: 2026-04-25T12:47:13Z

Context: Daytona (72k stars, active) explicitly sells stateful snapshots for agent sandboxes. Their OSS core may overlap heavily with what we're building. Need to understand their architecture to answer: can Mesh run on top of Daytona? Alongside? Must it be separate?

Why it matters: If Daytona's OSS solves 80% of this, building Mesh is redundant. If it assumes hosted/K8s/own-orchestrator and breaks on 2GB edge VMs, there's clear space.

Related decisions: D3 (Nomad not K8s), C1 (2GB VMs), C2 (no K8s)

Resolution: Mesh must be separate from Daytona. Fundamental mismatches: 8-16GB RAM vs 2GB target, no portable body abstraction, monolithic control plane vs no-central-dependency. Daytona is a reference for patterns, not a component. See D10 and research/daytona-analysis.md.

**Relationships:**
- related_to → D3

---

