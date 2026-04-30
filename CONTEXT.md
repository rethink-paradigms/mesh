# Mesh — Current State

> Auto-generated context. For full details, see `discovery/` files.
> Decision summaries sourced from `.mesh/governance.db`.

## Phase
v1.0 Implementation Complete — 17 packages test-passing. Governance system (mesh-nav) operational.

## What Mesh Is
Portable agent-body runtime. Gives an AI agent a persistent compute identity (filesystem state) that can live on any substrate and move between them. Self-hosted, user-owned, no central dependency.

## Decisions (D1–D10)

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
| D10 | Mesh is separate from Daytona — Daytona is valid substrate target via adapter, not dependency | accepted |

## Governance Decisions (D-GOV1–8)

| ID | Summary | Status |
|----|---------|--------|
| D-GOV1 | Discovery folder is the primary artifact | accepted |
| D-GOV2 | One skill governs the discovery system | accepted |
| D-GOV3 | No git hooks for governance | accepted |
| D-GOV4 | decisions.md generated from GrafitoDB graph DB | accepted |
| D-GOV5 | Session continuity via session.py (brief + structured handoff) | accepted |
| D-GOV6 | mesh-nav is the final governance layer | accepted |
| D-GOV7 | Python scripts are the API over GrafitoDB | accepted |
| D-GOV8 | Drift detection on invocation only | accepted |

## Open Questions

| ID | Summary | Status |
|----|---------|--------|
| Q1 | Where does a body live when idle? | resolved |
| Q2 | Registry — where do body snapshots live? | resolved |
| Q3 | Scheduler — is substrate selection core or plugin? | resolved |
| Q4 | Bootstrap — how does the first install mesh happen? | resolved |
| Q5 | Daytona OSS — coexist or diverge? | resolved |

## Constraints

- C1: Must run on 2GB VMs
- C2: Must not require K8s control plane
- C3: User owns all compute, keys, network
- C4: No telemetry, no login, no central dependency
- C5: Portable across substrates — no kernel/CPU coupling for core path
- C6: Core is tiny — provider code is plugin, not core library

## Personas

- A1: Hermes Operator — A2: Tool Agent (Go/Rust) — A3: Ephemeral Task Runner — A4: Burst Clone — A5: Developer Agent (laptop)

## Built (v1.0)

- Daemon with Docker + Nomad multi-adapter routing
- 16 MCP tools for body CRUD and migration
- 7-step cold migration coordinator with S3 registry
- Plugin system (go-plugin + gRPC + protobuf)
- CLI (mesh serve/stop/status/init)
- Bootstrap (goreleaser, install.sh, Homebrew formula)
- CI (GitHub Actions with integration tests)
- 17 packages test-passing

## Learnings

- L1: Docker adapter is built-in for v1.0 (pattern, confidence 5)
- L2: v1.0 implementation complete — 17 test-passing packages (project, confidence 5)

## Current Focus

mesh-nav v2 complete: GrafitoDB property graph backend (7 node types, 10 edge types), session continuity (auto-briefing + structured handoff), learnings as first-class nodes (Engram format), 105 tests passing. Regen this summary with `generate.py context-summary`.

**Entity counts**: 10 decisions | 8 governance decisions | 5 questions (all resolved) | 6 constraints | 5 personas | 4 sessions | 2 learnings

## Key Files

- `discovery/INDEX.md` — Project dashboard
- `discovery/intent.md` — What we're building and why
- `discovery/state/decisions.md` — Generated from DB (don't hand-edit)
- `discovery/state/governance-decisions.md` — Governance decisions from DB
- `.mesh/governance.db` — GrafitoDB property graph (7 node types, 10 edge types)
- `~/.agents/skills/mesh-nav/scripts/` — Python helper scripts
- `AGENTS.md` — Agent instructions

## Quick Commands

```bash
# Session management
python3 ~/.agents/skills/mesh-nav/scripts/session.py brief                                        # Auto-briefing
python3 ~/.agents/skills/mesh-nav/scripts/session.py start --date YYYY-MM-DD --type <TYPE>        # Start session
python3 ~/.agents/skills/mesh-nav/scripts/session.py end --id <N> --summary "<TEXT>"              # Structured handoff

# Learnings
python3 ~/.agents/skills/mesh-nav/scripts/learning.py add --what "<W>" --why "<Y>" --where "<R>" --learned "<L>" --category <CAT> --confidence <1-5>

# Governance
python3 ~/.agents/skills/mesh-nav/scripts/gov.py list --type decision                            # List decisions
python3 ~/.agents/skills/mesh-nav/scripts/generate.py context-summary                            # This summary
```
