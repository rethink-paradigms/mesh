# AgentBodies UX Context — Synthesis

> Compiled from 11 mined sessions (Apr 22–26, 2026).
> This document captures the full context of ideas explored and decided for the AgentBodies UI/UX.

---

## Product Definition

- **Mesh** = open source engine (CLI + MCP + skills). No UI. The substrate.
- **AgentBodies** = the product. The UI layer. Where you configure agents, manage compute, see everything.
- **Analogy**: Mesh is Git. AgentBodies is GitHub.
- **North Star**: "The interface that thinks with you — AI proposes, you confirm, everything reversible."

---

## UX Philosophy: 2026 Anxiety-Free Computing

Five principles:

1. **No workflow builders** — n8n already won that space
2. **No tables/dashboards** — rows/columns are passive, not interactive
3. **Plain English + visual confirmation** — type what you want, see what happens
4. **AI always proposes first** — never a blank slate
5. **Everything is reversible** — no irreversible decisions without AI help

Key user insight (from session):
> "2026 is about typing in plain english and interacting at visuals. People are paranoid from making a decision without consulting AI. They've lost the ability to make quick decisions. User should always have the feel that something best is chosen for them or any decision can be reverted easily."

---

## Core Metaphor: Body / Form / Substrate

- **Body** = permanent identity + filesystem state. The "who" of the agent. Persists across substrate changes.
- **Form** = current physical instantiation on a substrate. The "where." Ephemeral by nature.
- **Substrate** = where a form runs. Three pools: Local (laptop/Pi), Fleet (BYO VMs via Nomad), Sandbox (Daytona, E2B, Fly, Modal, Cloudflare).

> "The body is a filesystem. Where it physically runs is a cost/latency knob, not an architectural commitment."

---

## Capabilities vs Features

> "What we do wrong is we take a use case, think of features, and build features. Wrong. First build capabilities. Features are just a grouping of capability, choices made in an order, so you don't have to."

> "We are not making decisions for the user. We are making capabilities easy enough to choose from."

---

## Interface Architecture: Three Zones

1. **The Prompt** (bottom, always present) — "A conversation, not a search bar." Where the user talks.
2. **The Canvas** (center, scrollable) — Where AI renders status cards, timelines, agent maps, provision previews. Cards appear/disappear based on conversation context.
3. **The Rail** (left, minimal, ~200px) — Body names with status dots. "Not full navigation. Just enough to see what exists without asking."

**Layout principle**: Google Homepage principle. AI IS the dashboard. Information on demand. The interface starts nearly empty.

**Grid**: Single column canvas with card grid (2-column at >640px). Rail is fixed-width. Max content width: 1280px.

---

## Six Core UI Components (Catalog-Driven)

Rather than AI generating UI from scratch, define a fixed catalog:

1. **Confirmation Popup** — For destructive/significant actions. Always has AI suggestion + revert promise + cancel.
2. **Status Card** — Living card per agent. Running/stopped, machine, last snapshot. Expands on click.
3. **Provision Card** — AI recommends specs. User types adjustments ("make it 16GB"). No forms.
4. **Activity Timeline** — Every action. Each entry has a revert button. Scroll back to any point.
5. **Agent Map** — Spatial view of agents and bodies. Topology, not table. Things pulse when running, dim when stopped.
6. **Snapshot Browser** — Time Machine style. Visual timeline of snapshots. Click any point to see state.

**Architecture flow**:
```
User types English → AI interprets intent → Routes to action (~20 fixed)
→ Renders pre-designed component from catalog (6 components)
→ Populates with context + AI suggestion → User confirms → Action executes
→ Result appears on living map + timeline → Revert available
```

---

## The "Preview Render" Confirmation Pattern

The flagship interaction innovation — replaces binary "Are you sure?" dialogs:

> "Every DevTools tool uses the same pattern: a modal says 'Are you sure?' Nobody reads it. They click Confirm on muscle memory. It's security theater."

**How it works**: When you say "destroy body x3f7a," the AI shows consequences BEFORE executing:
- The body card dims
- A timeline entry appears in preview mode
- Connected bodies pulse to show dependency impact
- A 5-second countdown starts with an 'undo' affordance

> "Confirmation is a demonstration, not a binary gate."

---

## Five Agent Personas

| Persona | Type | Key Needs |
|---------|------|-----------|
| **A1: Hermes Operator** | 24/7 persistent | Keep running reliably, snapshot periodically, burst to sandbox |
| **A2: Tool Agent** | Lightweight persistent | Pack multiple per VM, deflate when idle, quick restore on request |
| **A3: Task Runner** | Ephemeral | Fast spawn on capable substrate, collect output, destroy |
| **A4: Burst Clone** | Fork of persistent | Snapshot parent FS, spawn clone, optionally merge FS delta back |
| **A5: Developer Agent** | Local | Local runtime, burst-to-sandbox for heavy tasks, don't interfere with direct FS access |

Feature need matrix:
| | Stop/Start | Snapshot | Clone+Merge | Burst | Pack | Deflate |
|---|:---:|:---:|:---:|:---:|:---:|:---:|
| A1: Hermes | yes | yes | yes | yes | no | no |
| A2: Tool Agent | yes | opt | no | no | yes | yes |
| A3: Task Runner | no | no | no | no | no | no |
| A4: Burst Clone | no | inherits | yes | yes | no | no |
| A5: Dev Agent | yes | opt | opt | yes | no | no |

---

## Body Lifecycle States

```
Created → Starting → Running → Stopping → Stopped → Destroying → Destroyed
                                                    → Error (unrecoverable)
```

UX guarantees:
- No external caller observes intermediate states (Starting, Stopping)
- Destroy is idempotent (already-destroyed = success, not error)
- No orphaned resources after Destroy

---

## Three Core User Flows

**Create**: User says "create a body for Hermes on my fleet" → AI calls mesh.body.create → Canvas shows body card appearing with spring animation.

**Snapshot**: User says "snapshot Hermes" → Returns immediately with operation token → Canvas shows progress card → On completion, snapshot card appears with size and storage URI.

**Cold Migration** (the archetypal flow — if you understand migration, you understand the system):
User says "move Hermes to E2B" → Preview Render activates → 7-step sequence: stop → capture → provision new → restore → assign identity → start → destroy old. Rollback on any failure.

---

## Design System: "The Craftsman's Bench"

### Aesthetic
Dark, warm, instrument-like. Workshop dark. Like a watchmaker's bench at midnight: blackened surfaces, selective warm illumination, every tool has weight and purpose.

References: Linear (surgical restraint), Raycast (macOS-native precision), Vercel v0 (chat-first AI interface where the UI disappears).

### Typography
| Role | Font | Rationale |
|------|------|-----------|
| Display/Hero | Instrument Serif | "Every DevTool uses geometric sans; this is the risk that makes AgentBodies instantly recognizable." |
| Body/UI | Geist Sans | Designed by Vercel for developer interfaces. Tabular-nums. |
| Code | Berkeley Mono | "When reading a body ID at 2am, certainty beats personality." |

### Color
- **Primary accent**: #C8956C (burnished copper) — NOT blue, NOT indigo. Accent on <5% of pixels.
- **Background**: #0A0A0B (warm near-black, like oiled tool steel)
- **Text**: #E8E4DD (warm off-white, parchment under lamplight)
- **Success**: #7FB069 (sage green) | **Warning**: #D4A843 (aged brass) | **Error**: #C75C5C (oxidized red)

Three deliberate design risks:
1. Serif display font in a DevTool
2. Copper accent, not blue
3. No dashboard, no default view

### Motion
- **Approach**: Intentional. Every animation communicates something.
- **Card entrances**: Spring physics — cubic-bezier(0.34, 1.56, 0.64, 1)
- **Durations**: micro(50ms) / short(150ms) / medium(250ms) / long(400ms)
- **Rule**: Animate only the changed element, not the whole view.

### "The First 3 Seconds"
> "The screen is almost empty — just the dark surface, the copper accent on the cursor blink, and the warm Serif typeface asking a question. The emotional hit isn't 'wow that's cool' — it's relief. The relief of encountering a tool that respects your attention."

---

## MCP Tool Surface (Tiered)

**Tier 1 (always loaded, ~2KB)**: create, list, inspect, stop, destroy
**Tier 2 (on-demand via mesh.tools.discover)**: snapshot, migrate, start, provisioner.list, plugin.install, plugin.list, network.get_endpoint
**Tier 3 (admin/CLI-only)**: config.set, plugin.generate

Progressive disclosure keeps initial tool surface small while making full surface available.

---

## Error UX

- Internal gRPC errors are NEVER shown raw
- All errors translated to human-readable with actionable context
- Examples: `BODY_NOT_FOUND: Body 'b-abc123' does not exist` | `INVALID_STATE: Body is stopped, cannot snapshot. Required: running`
- Retryable vs non-retryable explicitly distinguished

---

## Bootstrap UX (Chicken-and-Egg Problem)

Problem: MCP is primary interface, but MCP requires running Mesh. How does first install happen?

**Resolution**: Two-phase bootstrap:
1. **CLI phase**: `mesh init` — installs binary, generates config, starts daemon, prints MCP connection string
2. **MCP phase**: Agent connects, installs providers, spawns first body

Longer-term: Mesh starts on laptop, then "smoothly transfers itself to first VM" via self-migration.

---

## Alternatives Considered and Rejected

| Alternative | Why Rejected |
|-------------|--------------|
| Workflow builders (n8n) | Mature space, already won |
| Tables/dashboards | Passive, not interactive |
| Substrate-centric navigation | Users think about agents, not compute |
| AWS provisioning forms (12 clicks, 3 screens) | Replaced by "I need compute" + AI proposes |
| Traditional "Are you sure?" dialogs | Security theater, nobody reads them |
| Clinical DevTools palette (cool blue/indigo) | Warm is intentional — workshop, not hospital |
| Generative UI from scratch | Unreliable for production |
| Dashboard as default view | Google Homepage principle — AI IS the dashboard |
| User-perspective system design | Design for maintainers/agents instead |

---

## Competitive Landscape (UX-Relevant)

Key insight: "Every single product manages agent conversations, costs, and tasks. None manage the compute the agent runs on."

- **ClawHQ/HELIX**: Fleet dashboards, no compute management, no body abstraction
- **Daytona**: 8-16GB RAM (Mesh targets 2GB), monolithic control plane, AGPL
- **Northflank**: Full agent cloud with BYOC — closest competitor but no body abstraction
- **FLORA** ($42M Series A): Interface layer for generative workflows — validates "value is in the interface, not the engine"

---

## Open UX Questions (Still Unresolved)

1. How does a skill authenticate with Mesh? (API key? Tailscale identity? No auth for local?)
2. Body identity format? (UUID only? UUID + human-readable name?)
3. Snapshot default: clean (stop first) or crash-consistent?
4. Self-migration UX flow details (laptop → first VM transition)
5. Air-gap UX (full offline capability as first-class requirement)

---

## Source Sessions

| Session ID | Agent | Date | Title |
|-----------|-------|------|-------|
| `05fbca4a-affa-4840-bee3-6c6f33bb6ccf` | Claude | Apr 23 | **Primary UX session** — product split, 2026 philosophy, three zones, Preview Render, six components, design system |
| `ses_24994c99bffeSpwllQcb6IB4qj` | Claude | Apr 22 | Body metaphor, capabilities philosophy, user flows, competitor research |
| `ses_248e15098ffeZTZPGGx0aAsKrx` | Claude | Apr 22 | 5 agent personas, MVP scope, error UX, body lifecycle, Metis gap analysis |
| `ses_23f350d28ffeN1VbQhTg4hal2p` | Claude | Apr 24 | Bootstrap chicken-and-egg, self-migration, air-gap, UI as separate product |
| `ses_244b59d36ffekk6TMTmB6CACFY` | Claude | Apr 23 | CLI command surface, tiered tools, snapshot UX, three-stage staging |
| `ses_23f34abe7ffeB0f2tOxJdMnqTe` | Claude | Apr 24 | DESIGN.md reconnaissance, interface deep-dive, D5 decision |
| `ses_23b631f23ffez1Rc7LL88ExU0h` | OpenCode | Apr 25 | Session continuity UX, seven interaction layers |

### Key Artifacts on Disk
| File | Location |
|------|----------|
| Design system | `mesh/DESIGN.md` |
| Interface deep-dive | `mesh/discovery/design/deep/interface.md` |
| Product intent | `mesh/discovery/intent.md` |
| Agent personas | `mesh/discovery/state/personas.md` |
| Architecture decisions | `mesh/discovery/decisions.md` |
| System design | `mesh/discovery/design/SYSTEM.md` |
| V1 architecture | `mesh/.sisyphus/drafts/v1-architecture.md` |
