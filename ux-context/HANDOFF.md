# AgentBodies UX — Session Handoff

> Read this first if you're picking up the AgentBodies UI work.

## What Exists

| File | Purpose |
|------|---------|
| `00-SYNTHESIS.md` | **Read this first.** All UX decisions, philosophy, components, flows, personas, rejected alternatives, open questions. |
| `../DESIGN.md` | Design tokens — typography (Instrument Serif + Geist Sans + Berkeley Mono), color palette (burnished copper #C8956C), layout grid, motion specs. |
| `../discovery/design/deep/interface.md` | MCP tool surface (3 tiers), error UX patterns, bootstrap flow. |
| `../discovery/intent.md` | Product intent, core abstractions (Body/Form/Substrate), user profile. |
| `../discovery/state/personas.md` | Five agent personas with feature need matrix. |
| `../discovery/decisions.md` | Ten architectural decisions. D5 (chat-first, no dashboards) is the UX-critical one. |
| `../discovery/design/SYSTEM.md` | Six-module architecture, data flows. |
| `../.sisyphus/drafts/v1-architecture.md` | V1 implementation plan (Go, SQLite, 6 modules). |

## What to Build Next

The UX context is fully captured. The next step is **implementation** — building the AgentBodies UI.

### Suggested Starting Point

1. **Decide the frontend stack** — The design system specifies dark theme, specific fonts, spring animations, card-based layout. Likely candidates: Next.js 15 + Tailwind + Framer Motion, or a similar React stack.
2. **Build the three-zone layout first** — The Prompt (bottom), The Canvas (center), The Rail (left). This is the skeleton everything else lives in.
3. **Then the six catalog components** — Status Card, Confirmation Popup, Provision Card, Activity Timeline, Agent Map, Snapshot Browser. Each is a self-contained React component with defined behavior.
4. **Then the Preview Render pattern** — The flagship interaction (consequence demonstration before destructive actions).

### Key Constraints to Remember

- **No dashboards, no tables, no workflow builders** — This is non-negotiable from the design sessions.
- **Catalog-driven, not generative** — Six fixed component types. AI populates them, doesn't invent UI.
- **Warm dark aesthetic** — Copper accent (#C8956C), Instrument Serif display font, workshop-not-hospital palette.
- **AI proposes, user confirms** — Every interaction follows this pattern.
- **Everything reversible** — The Activity Timeline with per-entry undo is central to the UX.

## Open Questions (From Sessions)

1. Body identity format — UUID only vs UUID + human-readable name?
2. Snapshot default — clean (stop first) or crash-consistent?
3. Auth model — API key? Tailscale identity? No auth for local?
4. Self-migration UX — how does the laptop-to-VM transition look?
5. Air-gap UX — full offline as first-class?

## Source Sessions (In Archive)

The raw session files are in `/Users/samanvayayagsen/project/sessions/sessions/`:
- `mesh/claude/05fbca4a-*.jsonl` — Primary UX session (2.5MB, richest source)
- `oss/claude/ses_24994c99*.jsonl` — Body metaphor, capabilities philosophy
- `oss/claude/ses_248e1509*.jsonl` — Personas, MVP scope, error UX
