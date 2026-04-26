# Handoff: v1 Architecture Session

**Date**: 2026-04-25
**What happened**: Analyzed v0→v1 gap, resolved 6 architecture decisions, defined v1 scope, planned discovery updates. User wants to continue discussions before any execution.

---

## Where Things Are

**Three draft files (read these first):**
- `.sisyphus/drafts/v1-architecture.md` — All decisions, scope, structure, adapter interface, code reuse table
- `.sisyphus/drafts/discovery-update-plan.md` — Exact edits to make in discovery/ (when user is ready)
- `.sisyphus/drafts/v1-gap-analysis.md` — Module-by-module gap analysis with previous implementation findings (supplementary detail)

**Existing discovery/ (read for context):**
- `discovery/archive/SESSION-HANDOFF.md` — Journey arc through design phase
- `discovery/design/SYSTEM.md` — The 6-module system design (needs 10 edits, listed in update plan)
- `discovery/design/deep/` — 6 thorough deep-dive drafts per module (state machines, failure tables, scale analysis)

**Previous implementation (reference only):**
- `/Users/samanvayayagsen/project/rethink-paradigms/infa/mesh-workspace/oss` — Python v0.4.0 with Pulumi+Nomad+Tailscale

---

## What's Decided (So Far)

v1 = **core loop only**: Orchestration + Persistence + MCP Interface + Docker adapter. Single Go binary (`mesh serve`). SQLite for state. Thin Go interface for substrate adapter (not full go-plugin/gRPC yet). Networking deferred. No fleet provisioning. No plugin generation.

Full decisions in `v1-architecture.md` (AD1-AD6).

**These are proposed decisions, not final.** The user may want to revisit or adjust before committing.

---

## What Was Discussed But Not Fully Resolved

The user had deeper questions about:
- How provisioning actually works at each substrate level (Docker local vs Nomad fleet vs sandbox)
- Where Nomad sits — it's a fleet scheduler, not core, but its role wasn't clear before this session
- The chicken-and-egg bootstrap problem (no VM → no UI → can't add first VM)
- Self-migration concept (Mesh starts on laptop, relocates to first VM)
- State durability (cluster destroyed → restore from SQLite backup on cloud)
- Air-gap capability — user wants full air-gap support (disconnect from any Mesh server, run independently)
- Console product as separate concern (UI runs on user's VM, not hosted by us)
- Agent-to-Agent protocol (A2A) — runs inside containers, Mesh provides substrate, not protocol
- Core philosophy: core is minimal, anything optional is a plugin
- Acceptable coupling: user acknowledged that some core coupling is necessary ("you can't have theoretically a completely decoupled system, then it's not a system"). The core has hardline boundaries and some brittleness is acceptable.

The user may want to continue discussing any of these.

---

## The User's Working Style

- Previous implementation suffered architecture drift — pivots without traceability, documentation bloated to 16+ ADRs. This time: lean docs, minimal new files, no ADR sprawl.
- Core philosophy: anything without which Mesh can't survive = core. Anything optional = plugin. Core binary stays minimal.
- Peers, not master-servant. Talk as equals.
- Likes to discuss and ask questions before committing to decisions.
- Code is compiled output. Design is the true artifact. Architecture before implementation, always.

---

## State of the Session

This session was a consultation, not an execution session. Decisions were proposed, drafts were written, but nothing in discovery/ was modified. The user wants to pick up in the next session with continued discussion — more questions, more analysis, more clarity — before any files are changed or plans are generated.

**Do NOT jump to execution.** Let the user drive the conversation. The draft files are reference material for discussion, not a to-do list.
