# AGENTS.md — Mesh Project Context

## What This Project Is

Mesh is a portable agent-body runtime for AI agents. Gives an agent a persistent compute identity (filesystem state) that can live on any substrate — always-on VM, shared-tenant fleet, ephemeral sandbox — and move between them without losing itself. Self-hosted, user-owned, no central dependency.

**Previous identity:** Mesh was a "lightweight Kubernetes alternative" (Nomad + Consul + Tailscale + Caddy). That framing is dead. The codebase from that era exists but is being redesigned around a new intent.

## Discovery System

This project is in a **discovery and design phase**. We are NOT implementing yet. We are systematically exploring the problem space before committing to architecture.

All discovery artifacts live in `discovery/`. Read them in this order:

1. **`discovery/INDEX.md`** — Start here. Dashboard showing current state: decisions made, open questions, constraints, research status.
2. **`discovery/intent.md`** — What we're building and why. Stable. Rarely changes.
3. **`discovery/decisions.md`** — Numbered decisions (D1, D2, ...). Each has status, rationale, and cross-references to other decisions. **Every new design proposal must check this file for conflicts before proceeding.**
4. **`discovery/constraints.md`** — Hard boundaries. Non-negotiable. Any design that violates these is rejected.
5. **`discovery/open-questions.md`** — Unresolved questions with context. Track these so nothing falls through cracks.
6. **`discovery/personas.md`** — Agent personas (A1-A5) and their needs. Validate designs against these.
7. **`discovery/research/`** — Per-topic research files (substrate landscape, Daytona, k8s agent-sandbox, etc.).

## Rules for Working in This Project

1. **Read INDEX.md first.** Always. It tells you where things stand.
2. **Check decisions.md before proposing anything.** If your idea conflicts with an accepted decision, surface the conflict explicitly. Don't silently contradict a past decision.
3. **New decisions get the next available ID** (D10, D11, ...) and must include: status, context, the decision, rationale, conflicts_with, enables, and blocks.
4. **New open questions get the next available ID** (Q6, Q7, ...) and must link to related decisions.
5. **Research goes in `discovery/research/<topic-slug>.md`**. Update INDEX.md when adding a new research file.
6. **We are in discovery mode.** Do not write implementation code. Do not design architecture in detail. Do not create sprint plans. Explore, research, decide, document.
7. **Update INDEX.md** whenever you add or change a decision, question, or research file.

## Key Context (Quick Reference)

**Core abstraction:** Body (identity + filesystem) vs. Form (physical instantiation on a substrate). Body persists, form is ephemeral.

**Snapshot primitive:** `docker export | zstd` — flat filesystem tarball. No memory state. Fully portable. (D1, D2)

**Three substrate pools:** Local (laptop/Pi), Fleet (BYO VMs via Nomad), Sandbox (Daytona, E2B, Fly, Modal, Cloudflare).

**Primary interface:** MCP server + skills. Not CLI. (D5)

**No K8s. Ever.** Nomad on 2GB VMs. (D3)

**Provider integrations are plugins**, AI-generated, not maintained in core. (D6)
