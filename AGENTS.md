# AGENTS.md — Mesh Project Context

## What This Project Is

Mesh is a portable agent-body runtime for AI agents. Gives an agent a persistent compute identity (filesystem state) that can live on any substrate — always-on VM, shared-tenant fleet, ephemeral sandbox — and move between them without losing itself. Self-hosted, user-owned, no central dependency.

**Previous identity:** Mesh was a "lightweight Kubernetes alternative" (Nomad + Consul + Tailscale + Caddy). That framing is dead. The codebase from that era exists but is being redesigned around a new intent.

## Discovery System

All discovery artifacts live in `discovery/`. Read them in this order:

0. **`CONTEXT.md`** (project root) — Compressed current state. Read this BEFORE INDEX.md for quick orientation.
1. **`discovery/INDEX.md`** — Dashboard showing current state: decisions made, open questions, constraints, research status.
2. **`discovery/intent.md`** — What we're building and why. Stable. Rarely changes.
3. **`discovery/state/decisions.md`** — Numbered decisions (D1, D2, ...). ⚠️ Generated from DB — don't hand-edit. Regenerate with `generate.py decisions-md`. **Every new design proposal must check this file for conflicts before proceeding.**
4. **`discovery/constraints.md`** — Hard boundaries. Non-negotiable. Any design that violates these is rejected.
5. **`discovery/state/open-questions.md`** — Unresolved questions with context. Track these so nothing falls through cracks.
6. **`discovery/state/personas.md`** — Agent personas (A1-A5) and their needs. Validate designs against these.
7. **`discovery/research/`** — Per-topic research files (substrate landscape, Daytona, k8s agent-sandbox, etc.).

## Rules for Working in This Project

1. **Read CONTEXT.md first, then INDEX.md.** Always. They tell you where things stand.
2. **Check decisions.md before proposing anything.** If your idea conflicts with an accepted decision, surface the conflict explicitly.
3. **New decisions go through governance DB.** Use `gov.py add_entity` then `generate.py decisions-md`. Don't hand-edit generated markdown.
4. **New open questions get the next available ID** (Q6, Q7, ...) and must link to related decisions.
5. **New learnings go through `learning.py`** with Engram format (What/Why/Where/Learned), category, and confidence 1–5.
6. **Sessions use structured handoff** — capture reasoning, dead_ends, next_steps via `session.py end`.
7. **Research goes in `discovery/research/<topic-slug>.md`**. Update INDEX.md when adding a new research file.
8. **Update INDEX.md** whenever you add or change a decision, question, learning, or research file.

## Governance

The discovery system is governed by `mesh-nav` skill backed by `.mesh/governance.db` — a **GrafitoDB** property graph with 7 node types (decision, constraint, persona, question, learning, session, gov_decision) and 10 edge types (enables, conflicts_with, blocks, supersedes, related_to, resolved_by, validates_for, constrains, produced, learned_from).

- **mesh-nav skill**: `~/.agents/skills/mesh-nav/SKILL.md` — Session bootstrap, decision tracking, drift detection
- **DB**: `.mesh/governance.db` — GrafitoDB graph engine
- **Scripts** (`~/.agents/skills/mesh-nav/scripts/`): `graph.py`, `gov.py`, `session.py`, `learning.py`, `generate.py`, `migrate.py` — Python CLI tools (stdlib only, zero pip deps)
- **Tests**: `~/.agents/skills/mesh-nav/tests/` — 105 tests
- **Generated views**: `decisions-md`, `governance-md`, `questions-md`, `context-summary`, `learnings-md` — all via `generate.py`. Don't hand-edit.
- **Session continuity**: auto-briefing at start (`session.py brief`), structured handoff at end (`session.py end` with reasoning, dead_ends, surprises, deferred, blocked_on, next_steps, decisions, files_created, files_modified)
- **Learnings**: first-class nodes with Engram format — `learning.py add --what --why --where --learned --category --confidence`

Governance decisions (D-GOV1-8) live in `discovery/state/governance-decisions.md`.

## Quick Commands

```bash
# Session management
python3 ~/.agents/skills/mesh-nav/scripts/session.py brief                                        # Auto-briefing (entity counts, latest session, blockers)
python3 ~/.agents/skills/mesh-nav/scripts/session.py start --date YYYY-MM-DD --type <TYPE>        # Start session
python3 ~/.agents/skills/mesh-nav/scripts/session.py end --id <N> --summary "<TEXT>"              # End with structured handoff

# Learnings
python3 ~/.agents/skills/mesh-nav/scripts/learning.py add --what "<W>" --why "<Y>" --where "<R>" --learned "<L>" --category <CAT> --confidence <1-5>

# Governance
python3 ~/.agents/skills/mesh-nav/scripts/gov.py list --type decision                            # List decisions
python3 ~/.agents/skills/mesh-nav/scripts/generate.py decisions-md                                # Regenerate views
```

## Key Context (Quick Reference)

**Core abstraction:** Body (identity + filesystem) vs. Form (physical instantiation on a substrate). Body persists, form is ephemeral.

**Snapshot primitive:** `docker export | zstd` — flat filesystem tarball. No memory state. Fully portable. (D1, D2)

**Three substrate pools:** Local (laptop/Pi), Fleet (BYO VMs via Nomad), Sandbox (Daytona, E2B, Fly, Modal, Cloudflare).

**Primary interface:** MCP server + skills. Not CLI. (D5)

**No K8s. Ever.** Nomad on 2GB VMs. (D3)

**Provider integrations are plugins**, AI-generated, not maintained in core. (D6)
