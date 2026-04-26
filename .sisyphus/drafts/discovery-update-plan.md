# Discovery Folder Update Plan

> **Session**: 2026-04-25
> **Purpose**: Plan for curating and updating `discovery/` before generating the implementation plan. Prevents documentation bloat. Next session executes this plan first.

---

## The Problem We're Solving

Previous implementation had 16 ADRs + 3 PROD docs + tech debt doc → documentation flood → nobody could read/understand it all → architecture drifted because docs were unmanageable.

**This time's principle**: Lean documentation. Existing files update in-place. Minimal new files. Everything serves a clear purpose.

---

## Current Discovery Structure

```
discovery/
├── INDEX.md              # Dashboard — needs minor update
├── intent.md             # Stable, rarely changes
├── decisions.md          # D1-D10 — needs 6 new architecture decisions (now at state/decisions.md)
├── constraints.md        # C1-C6 — stable, no changes
├── open-questions.md     # Q1-Q5 — needs Q4 partially resolved (now at state/open-questions.md)
├── personas.md           # A1-A5 — stable (now at state/personas.md)
├── PORTING-GUIDE.md      # Reference from old repo (now at archive/PORTING-GUIDE.md)
├── SESSION-HANDOFF.md    # Journey arc — needs Phase 10 appended (now at archive/SESSION-HANDOFF.md)
├── research/             # 8 files, all complete — no changes
└── design/
    ├── SYSTEM.md         # 211 lines — needs 10 targeted updates
    ├── METHODOLOGY.md    # Stable — no changes
    ├── V1-ARCHITECTURE.md # NEW — capture v1 decisions
    ├── drafts/           # 2 diagram files — no changes
    └── deep/             # 6 deep-dive files — no changes (reference only)
```

---

## Changes to Make (Ordered)

### 1. NEW FILE: `discovery/design/V1-ARCHITECTURE.md`

Create this file. Content is in `.sisyphus/drafts/v1-architecture.md`. Copy it over.

**Why a new file**: The architecture decisions from this session are significant (process model, state management, plugin approach, scope). They deserve their own file because SYSTEM.md is the general system design, not the v1-specific execution plan. This file is the bridge between design (SYSTEM.md) and implementation (the plan).

**Principle check**: ONE new file. Not 5. Not ADRs. Just the v1 architecture.

### 2. UPDATE: `discovery/design/SYSTEM.md`

10 targeted edits. Do NOT rewrite the file. Edit specific sections.

| # | Action | Details |
|---|--------|---------|
| 1 | Add "Process Architecture" section after "Design Philosophy" | Single binary. `mesh serve` daemon. Direct Go calls between core modules. gRPC only at plugin boundary. |
| 2 | Add "State Management" section after "Cross-Cutting Concerns" | SQLite with WAL. Body registry, migration records. Single file, crash-safe. |
| 3 | Update "Orchestration" module (section 3) | Add: Migration ownership moved to Orchestration. MigrationRecord persisted in SQLite. Interface initiates, Orchestration executes. |
| 4 | Update substrate adapter interface in "Orchestration" section | Extend to 6 required + 4 optional capabilities. Include Go interface. |
| 5 | Add "Provisioning Levels" section | Level 1 (node, infrastructure) vs Level 2 (body, application). v1 does level 2 only. v1 assumes nodes exist. |
| 6 | Add "Body Identity Model" note | Bodies are stateful individuals. Not pods. No replicas. Clone model: snapshot → create copies → each diverges. |
| 7 | Add "Configuration" under "Cross-Cutting Concerns" | YAML format. Per-module sections. v0 TOML supported for migration. |
| 8 | Update "Core vs Plugin Boundary" section | Add v1 thin interface → v1.1 go-plugin upgrade path. Core = minimum for Mesh to function. |
| 9 | Resolve 4 "Open Design Questions" | Q4: CLI `mesh init` → `mesh serve`. Q5: Deferred to v2. Q6: User's responsibility, optional adapter methods. Q9: Orchestration GC. Mark them resolved with answers. |
| 10 | Add "v1 Scope" section at end | Explicit IN / DEFERRED v1.1 / DEFERRED v2 / OUT lists. Reference V1-ARCHITECTURE.md for details. |

### 3. UPDATE: `discovery/state/decisions.md`

Add 6 new architecture decisions (AD1-AD6) from this session. Format matches existing D1-D10 style:

- AD1: Single binary, direct Go calls
- AD2: Thin plugin veneer for v1
- AD3: SQLite with WAL for durable state
- AD4: Orchestration owns migration
- AD5: Extended substrate adapter contract
- AD6: Networking deferred to v1.1

Each with: status, context, decision, rationale, conflicts_with, enables, blocks.

### 4. UPDATE: `discovery/INDEX.md`

Minor updates:
- Decision count: "10 accepted" → "10 accepted | 6 architecture decisions"
- Add V1-ARCHITECTURE.md to the design files section
- Update open questions status (Q4 partially resolved)

### 5. UPDATE: `discovery/state/open-questions.md`

- Q4: Update status to "partially resolved" with answer (CLI bootstrap: `mesh init` → `mesh serve`)
- Note that Q1, Q2, Q3 remain unresolved (deferred to v1.1+)

### 6. UPDATE: `discovery/archive/SESSION-HANDOFF.md`

Append Phase 10 after Phase 9:

```
### Phase 10: v1 Architecture Decisions

Analyzed v0 → v1 gap. Resolved 6 architecture decisions. Defined v1 scope (core loop).
Key decisions: single binary, thin plugin veneer, SQLite state, Orchestration owns migration.
v1 = Orchestration + Persistence + MCP + Docker adapter. Networking and full plugins deferred.

Files produced: design/V1-ARCHITECTURE.md, decisions AD1-AD6
```

---

## Files NOT Changed (Intentionally)

- `intent.md` — stable, no changes needed
- `constraints.md` — C1-C6 are hard boundaries, no changes
- `personas.md` — A1-A5 are valid, no changes
- `research/*` — 8 files complete, reference only
- `design/deep/*` — 6 deep-dive drafts, reference only. Their open questions are noted but not all resolved.
- `design/METHODOLOGY.md` — methodology is stable
- `design/drafts/*` — diagram drafts, untouched
- `PORTING-GUIDE.md` — reference from old repo

---

## What NOT to Create (Anti-Bloat)

Do NOT create:
- ❌ Separate ADR files (decisions go in decisions.md)
- ❌ Architecture governance docs (rules go in V1-ARCHITECTURE.md)
- ❌ Change log files (SESSION-HANDOFF captures journey)
- ❌ Core boundary manifest (core vs plugin goes in SYSTEM.md update #8)
- ❌ Scope contract file (scope goes in V1-ARCHITECTURE.md)

**One new file. Five existing files updated. That's it.**

---

## Execution Order (Next Session)

1. Read `.sisyphus/drafts/v1-architecture.md` and this file
2. Create `discovery/design/V1-ARCHITECTURE.md` (copy from draft)
3. Update `discovery/design/SYSTEM.md` (10 targeted edits)
4. Update `discovery/state/decisions.md` (add AD1-AD6)
5. Update `discovery/INDEX.md` (counts and references)
6. Update `discovery/state/open-questions.md` (Q4 status)
7. Update `discovery/archive/SESSION-HANDOFF.md` (append Phase 10)
8. Clean up `.sisyphus/drafts/` (delete the draft files)
9. Then generate implementation plan to `.sisyphus/plans/v1-core-loop.md`

Steps 2-7 are quick edits. Step 9 is the main work.
