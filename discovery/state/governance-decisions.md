# Governance Decisions

## D-GOV1: Discovery folder is the primary artifact

**Status**: accepted
**Date**: 2026-04-25T12:45:23Z

The discovery/ folder is more important than code. Code is compiled output. Discovery is the true artifact — it captures what was decided, why, and what's still open. All agent sessions must treat discovery/ as the source of truth for project state.

Enables: D-GOV4, D-GOV5, D-GOV6

**Relationships:**
- enables → D-GOV4
- enables → D-GOV5
- enables → D-GOV6

---

## D-GOV2: One skill governs the discovery system

**Status**: accepted
**Date**: 2026-04-25T12:45:23Z

mesh-nav is the single governance skill. Not multiple skills, not git hooks, not manual processes. One entry point with mode-dependent flows (bootstrap, design, plan, implement, review, compress). Follows gstack office-hours pattern.

Enables: D-GOV6, D-GOV7

**Relationships:**
- enables → D-GOV6
- enables → D-GOV7

---

## D-GOV3: No git hooks for governance

**Status**: accepted
**Date**: 2026-04-25T12:45:23Z

Governance enforcement via git hooks is too much ceremony. The skill + AGENTS.md provides sufficient governance. Pre-commit hooks, post-commit hooks, and CI gates are explicitly excluded. The mesh-nav skill is invoked by agents, not by git.

Conflicts with: any approach using git hooks

---

## D-GOV4: decisions.md generated from SQLite DB

**Status**: accepted
**Date**: 2026-04-25T12:45:23Z

discovery/state/decisions.md is a VIEW of the governance DB, not a hand-editable file. Changes to decisions go through gov.py (which updates the DB) followed by generate.py (which regenerates the markdown). This prevents drift between DB and markdown. A note at the top of generated files warns against manual editing.

Enables: D-GOV8 (drift detection)

**Relationships:**
- enables → D-GOV4
- enables → D-GOV8

---

## D-GOV5: Session continuity via CONTEXT.md + SESSION-LOG.md

**Status**: accepted
**Date**: 2026-04-25T12:45:47Z

CONTEXT.md captures current project state (~100 lines, decision summaries from DB). SESSION-LOG.md is an append-only changelog with natural decay. Together they ensure every agent session can bootstrap productively without re-explaining context. CONTEXT.md is written by the mesh-nav skill only (never hand-edited). SESSION-LOG.md is append-only.

Enables: productive agent sessions

**Relationships:**
- enables → D-GOV5

---

## D-GOV6: mesh-nav is the final governance layer

**Status**: accepted
**Date**: 2026-04-25T12:45:48Z

No meta-governance. mesh-nav governs discovery. There is no skill that governs how mesh-nav works. The skill is the final layer. AGENTS.md provides bootstrapping rules. Beyond that, mesh-nav is self-contained.

Blocks: meta-governance proposals

**Relationships:**
- enables → D-GOV6
- enables → D-GOV6

---

## D-GOV7: Python scripts are the API over SQLite

**Status**: accepted
**Date**: 2026-04-25T12:45:49Z

The mesh-nav skill calls Python scripts (gov.py, session.py, generate.py), NEVER writes raw SQL. Python scripts use stdlib only (sqlite3, json, argparse). This separation means the skill remains a markdown orchestration document, while Python handles data integrity.

Enables: skill simplicity, data integrity

**Relationships:**
- enables → D-GOV7

---

## D-GOV8: Drift detection on invocation only

**Status**: accepted
**Date**: 2026-04-25T12:45:50Z

Drift between generated markdown and DB is checked when mesh-nav is invoked, not via daemons, cron, or file watchers. Compare generated markdown with on-disk content using content hash. Flag if different. No background processes, no scheduled tasks.

Blocks: drift detection daemon proposals

**Relationships:**
- enables → D-GOV8

---

