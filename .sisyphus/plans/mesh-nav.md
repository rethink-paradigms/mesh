# mesh-nav: Project Operating System for Discovery Governance

## TL;DR

> **Quick Summary**: Build a single skill (`mesh-nav`) that governs the Mesh project's discovery/ folder — backed by a SQLite graph database for queryable decisions/sessions, with Python helper scripts as the API layer. Handles session bootstrap, reading maps, update protocols, drift detection, and session compression. Restructure the discovery/ folder, create session continuity files, and update AGENTS.md.
>
> **Deliverables**:
> - `.mesh/governance.db` — SQLite graph DB (entities + edges tables, decisions/sessions/constraints as queryable objects)
> - `~/.agents/skills/mesh-nav/scripts/` — Python helper scripts (gov.py, session.py, generate.py) — pre-written query API
> - `~/.agents/skills/mesh-nav/SKILL.md` (~350 lines) + 3 reference files
> - Restructured `discovery/` folder (3 files + 4 directories)
> - `CONTEXT.md` at project root (compressed current state)
> - `SESSION-LOG.md` at project root (changelog with natural decay)
> - Updated `AGENTS.md` with governance section (<80 lines total)
> - `discovery/state/decisions.md` + `governance-decisions.md` — generated from DB, always in sync
>
> **Estimated Effort**: Medium-Large
> **Parallel Execution**: YES - 3 waves + 1 verification wave
> **Critical Path**: T1 (restructure) → T2 (DB + scripts) → T4 (generate markdown) → T7 (SKILL.md) → T8 (AGENTS.md) → F1-F4
>
> **Architecture**: Bottom-up layered — SQLite (data) → Python scripts (API) → SKILL.md (orchestration) → Agent (consumer). Skill never writes SQL. Calls Python scripts. Python scripts are pre-written, not generated.

---

## Context

### Original Request
User wants a meta-system ("project operating system") for the Mesh project's `discovery/` folder. The discovery folder is the PRIMARY project artifact — more important than code. Code is compiled output. Discovery is the true artifact. The problem: agents don't use it properly. Files get written once and never evolved. Sessions feel isolated — every session requires re-explaining context.

### Interview Summary
**Key Discussions**:
- Structure vs. Governance vs. Retrieval: User confirmed structure is fine, governance and retrieval are broken
- One skill vs. multiple: One skill with multiple modes (following gstack office-hours pattern)
- Git hooks: Too much ceremony. Skill + AGENTS.md is sufficient
- Session continuity: CONTEXT.md (current state) + SESSION-LOG.md (changelog with decay)
- SQLite for machine metadata: **Confirmed as first-class**. Plain SQLite with entities+edges graph schema. Python helper scripts as API layer. DB tracks decisions, sessions, questions, constraints and their relationships. Research files, deep-dives, narratives stay markdown. Some markdown files (decisions.md, governance-decisions.md) generated from DB. DB file in git is fine — any user clones and uses.
- SESSION-HANDOFF.md fate: Move to archive/ (historical, not current state)
- D-GOV1-D-GOV8 location: Separate governance-decisions.md, not mixed with product decisions

**Research Findings**:
- Pydantic AI: hierarchical conditional reading with stable rule IDs
- Vercel AI SDK: ADR check + "Do Not" list + task-type checklists
- LangChain: pre-modification gate protocol
- Rust/React RFC: living design docs with status lifecycle
- Anchored Development: four-tier drift detector (too complex for our scope)
- gstack office-hours: architectural pattern — one entry, mode-dependent flows, shared later phases
- opencode skills: lean pattern — <500 lines SKILL.md, references/ for larger content

### Metis Review
**Identified Gaps** (addressed):
- SESSION-HANDOFF.md overlap with CONTEXT.md → Move to archive/, different purpose
- D-GOV1-D-GOV8 not recorded → Create governance-decisions.md in state/
- File-by-file restructure mapping → Explicit source→destination for every file
- Cross-reference integrity after moves → Grep task after restructure
- Scope creep in drift detection → Lock down to timestamp + content hash on invocation
- Reading maps proliferation → Max 4 maps (bootstrap, continuation, decision, research)
- AGENTS.md bloat → Hard cap at 80 lines
- Simultaneous agent writes → Append-only SESSION-LOG.md, CONTEXT.md written by skill only
- Non-Mesh project invocation → Detect absence of discovery/ and exit gracefully

---

## Work Objectives

### Core Objective
Build a complete project operating system that makes every agent session a productive continuation — no re-explaining, no re-discovering, no drifting between discovery and code.

### Concrete Deliverables
- `.mesh/governance.db` — SQLite graph database (entities + edges + sessions tables)
- `~/.agents/skills/mesh-nav/scripts/gov.py` — Decision/entity CRUD and graph queries
- `~/.agents/skills/mesh-nav/scripts/session.py` — Session logging and retrieval
- `~/.agents/skills/mesh-nav/scripts/generate.py` — Markdown generation from DB
- `~/.agents/skills/mesh-nav/SKILL.md` — The main skill file (calls Python scripts, never raw SQL)
- `~/.agents/skills/mesh-nav/references/reading-map.md` — Task→file mapping (max 4 maps)
- `~/.agents/skills/mesh-nav/references/update-protocols.md` — Post-task checklists
- `~/.agents/skills/mesh-nav/references/folder-structure.md` — Folder map and file manifest
- `CONTEXT.md` — Compressed current state (~100 lines, decision summaries section generated from DB)
- `SESSION-LOG.md` — Session history with natural decay
- `AGENTS.md` — Updated with governance section (total <80 lines)
- `discovery/state/decisions.md` — Generated from DB (source of truth = DB, markdown = view)
- `discovery/state/governance-decisions.md` — Generated from DB
- Restructured `discovery/` folder (3 files + 4 directories)

### Definition of Done
- [ ] `.mesh/governance.db` exists with entities, edges, sessions tables
- [ ] All existing decisions (D1-D10, D-GOV1-8) seeded in DB
- [ ] Python scripts work: `python scripts/gov.py conflicts D3` returns related decisions
- [ ] `python scripts/generate.py decisions-md` regenerates decisions.md from DB
- [ ] `mesh-nav` skill loads without errors and calls Python scripts
- [ ] Fresh agent session bootstraps from CONTEXT.md alone (no other context needed)
- [ ] Discovery folder has 3 files + 4 directories at top level
- [ ] All cross-references in discovery/ resolve after restructure
- [ ] AGENTS.md is under 80 lines and references mesh-nav
- [ ] SESSION-LOG.md has at least one entry (this session)
- [ ] Generated markdown files match DB content (no drift)

### Must Have
- SQLite graph DB (.mesh/governance.db) with entities + edges + sessions tables
- Python helper scripts (gov.py, session.py, generate.py) as API layer over DB
- Single skill with mode-dependent flows (bootstrap, design, plan, implement, review, compress)
- Skill calls Python scripts, NEVER writes raw SQL
- Reading map with 4 task-type→file mappings
- CONTEXT.md that any agent can read to bootstrap a productive session
- Folder restructure with zero broken cross-references
- All decisions (D1-D10, D-GOV1-8) seeded in DB and queryable
- decisions.md + governance-decisions.md generated from DB (always in sync)

### Must NOT Have (Guardrails)
- **No git hooks** (D-GOV3)
- **No meta-governance** (governing how governance works) — the skill is the final layer
- **No file renames** — only directory moves. Files keep their names.
- **No drift detection daemons/cron** — checked on invocation only, timestamp + content hash
- **No more than 4 reading maps** — bootstrap, continuation, decision, research
- **No SKILL.md over 400 lines** — hard cap
- **No reference file over 200 lines** — hard cap
- **No AGENTS.md over 80 lines** — hard cap
- **No raw SQL in SKILL.md** — skill calls Python scripts only
- **No research/deep-dive/narrative content in DB** — only relational entities (decisions, questions, constraints, sessions, modules)
- **No duplication of what AGENTS.md already provides** — mesh-nav extends, not parallels

---

## Verification Strategy (MANDATORY)

> **ZERO HUMAN INTERVENTION** — ALL verification is agent-executed. No exceptions.

### Test Decision
- **Infrastructure exists**: NO (no test framework for skills)
- **Automated tests**: NONE (skills are markdown, not code)
- **Framework**: N/A
- **Verification**: Agent-executed QA scenarios per task

### QA Policy
Every task includes agent-executed QA scenarios.
Evidence saved to `.sisyphus/evidence/task-{N}-{scenario-slug}.{ext}`.

- **File operations**: Use Bash (ls, grep, cat) — verify files exist, paths resolve, content matches
- **Cross-references**: Use Bash (grep) — verify all references resolve
- **Skill loading**: Use Bash — verify skill loads and content is valid markdown
- **Content integrity**: Use Read — verify CONTEXT.md accuracy, AGENTS.md line count

---

## Execution Strategy

### Parallel Execution Waves

```
Wave 1 (Start Immediately — foundation + data layer):
├── Task 1: Restructure discovery/ folder [quick]
├── Task 2: Create SQLite schema + Python scripts [deep]
└── Task 3: Seed DB from existing markdown + generate files [unspecified-high]

Wave 2 (After Wave 1 — session files + references):
├── Task 4: Create CONTEXT.md (decision summaries from DB) [quick]
├── Task 5: Create SESSION-LOG.md [quick]
├── Task 6: Create folder-structure.md reference (depends: 1) [quick]
├── Task 7: Create reading-map.md reference (depends: 1, 6) [unspecified-high]
└── Task 8: Create update-protocols.md reference (depends: 1) [unspecified-high]

Wave 3 (After Wave 2 — skill + integration):
├── Task 9: Create mesh-nav SKILL.md core (depends: 2, 4, 5, 7, 8) [deep]
└── Task 10: Update AGENTS.md (depends: 9) [quick]

Wave FINAL (After ALL tasks — 4 parallel reviews):
├── F1: Plan compliance audit (oracle)
├── F2: Content quality review (unspecified-high)
├── F3: Real QA — fresh bootstrap test (unspecified-high)
└── F4: Scope fidelity check (deep)
-> Present results -> Get explicit user okay

Critical Path: T1 → T2 → T3 → T4 → T9 → T10 → F1-F4
Parallel Speedup: ~50% faster than sequential
Max Concurrent: 5 (Wave 2)
```

### Dependency Matrix

| Task | Depends On | Blocks | Wave |
|------|-----------|--------|------|
| 1    | -         | 3, 6, 7, 8 | 1   |
| 2    | -         | 3, 9    | 1    |
| 3    | 1, 2      | 4, 9    | 1    |
| 4    | 3         | 9       | 2    |
| 5    | -         | 9       | 2    |
| 6    | 1         | 7       | 2    |
| 7    | 1, 6      | 9       | 2    |
| 8    | 1         | 9       | 2    |
| 9    | 2, 4, 5, 7, 8 | 10 | 3    |
| 10   | 9         | F1-F4   | 3    |

### Agent Dispatch Summary

- **Wave 1**: **3** tasks — T1 → `quick`, T2 → `deep`, T3 → `unspecified-high`
- **Wave 2**: **5** tasks — T4 → `quick`, T5 → `quick`, T6 → `quick`, T7 → `unspecified-high`, T8 → `unspecified-high`
- **Wave 3**: **2** tasks — T9 → `deep`, T10 → `quick`
- **FINAL**: **4** tasks — F1 → `oracle`, F2 → `unspecified-high`, F3 → `unspecified-high`, F4 → `deep`

---

## TODOs

- [x] 1. Restructure discovery/ Folder

  **What to do**:
  - Create `discovery/state/` directory
  - Create `discovery/archive/` directory
  - Move files to new locations (preserving filenames):
    - `discovery/decisions.md` → `discovery/state/decisions.md`
    - `discovery/open-questions.md` → `discovery/state/open-questions.md`
    - `discovery/personas.md` → `discovery/state/personas.md`
    - `discovery/SESSION-HANDOFF.md` → `discovery/archive/SESSION-HANDOFF.md`
    - `discovery/PORTING-GUIDE.md` → `discovery/archive/PORTING-GUIDE.md`
  - Verify top level has exactly 3 files: INDEX.md, intent.md, constraints.md
  - Verify top level has exactly 4 directories: state/, design/, research/, archive/
  - **CRITICAL**: Update ALL cross-references in ALL markdown files across the project. Specifically:
    - `AGENTS.md` references `decisions.md`, `open-questions.md`, `personas.md` — update paths to `state/decisions.md`, etc.
    - `discovery/INDEX.md` references all files — update paths
    - `discovery/design/SYSTEM.md` references research files — verify paths still resolve
    - Any other file that references moved files — grep for old paths
  - Run `grep -rn "discovery/decisions.md\|discovery/open-questions.md\|discovery/personas.md\|discovery/SESSION-HANDOFF.md\|discovery/PORTING-GUIDE.md" --include="*.md"` to confirm zero stale references
  - Update `discovery/INDEX.md` to reflect new structure

  **Must NOT do**:
  - Do NOT rename any files — only move them to new directories
  - Do NOT modify file content (only path references)
  - Do NOT delete any files
  - Do NOT touch files inside design/ or research/ directories (they stay in place)

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: File moves and grep operations, straightforward bash work
  - **Skills**: []
  - **Skills Evaluated but Omitted**:
    - `git-master`: Not needed — moves are simple, no complex git operations

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Tasks 2, 3, 4)
  - **Parallel Group**: Wave 1
  - **Blocks**: Tasks 5, 6, 7
  - **Blocked By**: None (can start immediately)

  **References**:

  **Pattern References**:
  - `AGENTS.md:1-45` — Current AGENTS.md content, contains path references that must be updated
  - `discovery/INDEX.md:1-70` — Dashboard that references all discovery files, must be updated

  **API/Type References**:
  - `discovery/SESSION-HANDOFF.md:1-279` — File being moved to archive/, understand its content before moving

  **WHY Each Reference Matters**:
  - `AGENTS.md` — Contains "Read INDEX.md first" and path references that will break after restructure
  - `discovery/INDEX.md` — Lists all files with descriptions, paths will change
  - `discovery/SESSION-HANDOFF.md` — Understand this is historical journey, not current state, before archiving

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Folder structure is correct
    Tool: Bash
    Preconditions: Restructure is complete
    Steps:
      1. Run `ls discovery/*.md | wc -l` → assert output is "3"
      2. Run `ls -d discovery/*/` → assert output contains exactly state/, design/, research/, archive/
      3. Run `ls discovery/state/*.md | wc -l` → assert output is "3" (decisions, open-questions, personas)
      4. Run `ls discovery/archive/*.md | wc -l` → assert output is "2" (SESSION-HANDOFF, PORTING-GUIDE)
    Expected Result: 3 top-level files, 4 directories, correct files in each
    Failure Indicators: Wrong file count, missing directory, file not found in expected location
    Evidence: .sisyphus/evidence/task-1-folder-structure.txt

  Scenario: Cross-references resolve after restructure
    Tool: Bash
    Preconditions: Restructure and reference updates are complete
    Steps:
      1. Run `grep -rn "discovery/decisions.md\|discovery/open-questions\|discovery/personas\|discovery/SESSION-HANDOFF\|discovery/PORTING-GUIDE" --include="*.md"` → assert exit code 1 (no matches = no stale references)
      2. Run `grep -rn "state/decisions\|state/open-questions\|state/personas\|archive/SESSION\|archive/PORTING" --include="*.md"` → assert matches found (new references exist)
    Expected Result: Zero stale old-path references, new-path references present
    Failure Indicators: grep finds old paths = cross-references broken
    Evidence: .sisyphus/evidence/task-1-cross-refs.txt
  ```

  **Commit**: YES (groups with Wave 1)
  - Message: `chore(discovery): restructure folder into state/design/research/archive`
  - Files: `discovery/*`
  - Pre-commit: `grep -rn "discovery/decisions.md" --include="*.md"` (should find zero)


- [x] 2. Create SQLite Schema + Python Helper Scripts

  **What to do**:
  - Create `.mesh/` directory at project root
  - Initialize `.mesh/governance.db` with this schema:

  ```sql
  CREATE TABLE entities (
      id          TEXT PRIMARY KEY,
      type        TEXT NOT NULL,
      title       TEXT NOT NULL,
      status      TEXT DEFAULT 'accepted',
      body        TEXT,
      properties  TEXT,
      created_at  TEXT NOT NULL,
      updated_at  TEXT NOT NULL
  );

  CREATE TABLE edges (
      id          INTEGER PRIMARY KEY AUTOINCREMENT,
      source_id   TEXT NOT NULL REFERENCES entities(id),
      target_id   TEXT NOT NULL REFERENCES entities(id),
      relation    TEXT NOT NULL,
      properties  TEXT
  );

  CREATE TABLE sessions (
      id          INTEGER PRIMARY KEY AUTOINCREMENT,
      date        TEXT NOT NULL,
      type        TEXT NOT NULL,
      summary     TEXT,
      focus       TEXT,
      next_steps  TEXT,
      decisions_made TEXT,
      files_created  TEXT,
      files_modified TEXT
  );

  CREATE INDEX idx_edges_source ON edges(source_id);
  CREATE INDEX idx_edges_target ON edges(target_id);
  CREATE INDEX idx_edges_relation ON edges(relation);
  CREATE INDEX idx_entities_type ON entities(type);
  CREATE INDEX idx_entities_status ON entities(status);
  ```

  - Create `~/.agents/skills/mesh-nav/scripts/` directory
  - Create `scripts/gov.py` — CLI tool for entity/edge operations:
    - `python gov.py add_entity <id> <type> <title> [--status X] [--body "text"]` — Insert entity
    - `python gov.py get <id>` — Get entity details
    - `python gov.py update <id> [--status X] [--title X]` — Update entity
    - `python gov.py list [--type decision] [--status accepted]` — List with filters
    - `python gov.py add_edge <source> <target> <relation>` — Create relationship
    - `python gov.py trace <id> [--depth N]` — Graph traversal via recursive CTE
    - `python gov.py conflicts <id>` — Show conflicting entities
    - `python gov.py enabled_by <id>` — Show what this enables
    - `python gov.py blocked_by <id>` — Show what blocks this
  - Create `scripts/session.py` — CLI tool for session operations:
    - `python session.py start --date YYYY-MM-DD --type design`
    - `python session.py end --id N --summary "..." --decisions '["D11"]'`
    - `python session.py latest` — Most recent session
    - `python session.py list [--limit 10]` — Recent sessions
  - Create `scripts/generate.py` — CLI tool for markdown generation:
    - `python generate.py decisions-md` — Generate full decisions.md
    - `python generate.py governance-md` — Generate governance-decisions.md
    - `python generate.py questions-md` — Generate open-questions.md
    - `python generate.py context-summary` — One-line summaries for CONTEXT.md
  - All scripts: Python stdlib only (sqlite3, json, argparse, sys). Zero pip deps.
  - All scripts accept `--db` flag (default: `.mesh/governance.db`)

  **Must NOT do**:
  - Do NOT install pip packages — stdlib only
  - Do NOT use Cypher or graph query language — plain SQL with recursive CTEs
  - Do NOT use an ORM — raw sqlite3
  - Do NOT create a web server — CLI scripts only
  - Do NOT hardcode paths — use `--db` flag

  **Recommended Agent Profile**:
  - **Category**: `deep` — Data layer is the most critical foundation
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Task 1)
  - **Parallel Group**: Wave 1
  - **Blocks**: Tasks 3, 9
  - **Blocked By**: None

  **References**:
  - ctxgraph SQLite schema (from research) — entities + edges pattern
  - "We replaced Neo4j with 45 SQL statements" — recursive CTE traversal

  **Acceptance Criteria**:

  **QA Scenarios:**

  ```
  Scenario: DB schema and scripts work
    Tool: Bash
    Steps:
      1. ls .mesh/governance.db → exists
      2. sqlite3 .mesh/governance.db ".tables" → contains entities, edges, sessions
      3. python scripts/gov.py --help → exit 0
      4. python scripts/session.py --help → exit 0
      5. python scripts/generate.py --help → exit 0
    Expected: DB exists, 3 tables, 3 scripts respond
    Evidence: .sisyphus/evidence/task-2-db-schema.txt

  Scenario: Graph traversal works
    Tool: Bash
    Steps:
      1. python scripts/gov.py add_entity T1 decision "Test 1" --status accepted
      2. python scripts/gov.py add_entity T2 decision "Test 2" --status accepted
      3. python scripts/gov.py add_edge T1 T2 enables
      4. python scripts/gov.py trace T1 --depth 2 → contains T2
    Expected: Entities created, edge created, trace returns connected entities
    Evidence: .sisyphus/evidence/task-2-graph-traversal.txt
  ```

  **Commit**: YES
  - Message: `feat(governance): add SQLite graph DB and Python helper scripts`
  - Files: `.mesh/governance.db`, `~/.agents/skills/mesh-nav/scripts/*.py`

- [x] 3. Seed DB from Existing Markdown + Generate Files

  **What to do**:
  - Parse `discovery/state/decisions.md` (after Task 1 move) → insert D1-D10 as entities
  - Parse `discovery/state/open-questions.md` → insert Q1-Q5
  - Parse `discovery/constraints.md` → insert C1-C6
  - Parse `discovery/state/personas.md` → insert A1-A5
  - Parse `.sisyphus/drafts/discovery-governance.md` D-GOV section → insert D-GOV1-D-GOV8
  - Extract relationships (conflicts_with, enables, blocks) from cross-references
  - Run `python scripts/generate.py decisions-md > discovery/state/decisions.md`
  - Run `python scripts/generate.py governance-md > discovery/state/governance-decisions.md`
  - Run `python scripts/generate.py questions-md > discovery/state/open-questions.md`
  - Verify generated markdown matches original content (no data loss)

  **Must NOT do**:
  - Do NOT lose content from original markdown
  - Do NOT seed research files or deep-dives — those stay markdown only
  - Do NOT seed narrative content into DB

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high` — Parse structured markdown, extract entities, verify no data loss
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 1 (after Tasks 1 + 2)
  - **Blocks**: Tasks 4, 9
  - **Blocked By**: Tasks 1, 2

  **References**:
  - `discovery/state/decisions.md` (after Task 1) — Source of D1-D10
  - `discovery/state/open-questions.md` (after Task 1) — Source of Q1-Q5
  - `discovery/constraints.md` — Source of C1-C6
  - `.sisyphus/drafts/discovery-governance.md:254-340` — D-GOV1-D-GOV8
  - `scripts/gov.py` (from Task 2) — API to use

  **Acceptance Criteria**:

  **QA Scenarios:**

  ```
  Scenario: All entities seeded
    Tool: Bash
    Steps:
      1. python scripts/gov.py list --type decision → 10 (D1-D10)
      2. python scripts/gov.py list --type governance_decision → 8 (D-GOV1-8)
      3. python scripts/gov.py list --type question → 5 (Q1-Q5)
      4. python scripts/gov.py list --type constraint → 6 (C1-C6)
      5. Total = 34 entities
    Expected: All 34 entities seeded
    Evidence: .sisyphus/evidence/task-3-seed-count.txt

  Scenario: Generated markdown matches originals
    Tool: Bash
    Steps:
      1. python scripts/generate.py decisions-md → contains D1-D10 titles + rationale
      2. python scripts/generate.py governance-md → contains D-GOV1-D-GOV8
    Expected: No data loss, all content present
    Evidence: .sisyphus/evidence/task-3-generated-md.txt
  ```

  **Commit**: YES
  - Message: `feat(governance): seed DB and generate markdown from entities`
  - Files: `.mesh/governance.db`, `discovery/state/decisions.md`, `discovery/state/governance-decisions.md`, `discovery/state/open-questions.md`

- [x] 4. Create CONTEXT.md

  **What to do**:
  - Create `CONTEXT.md` at project root (~100 lines, no narrative)
  - Decision summaries: use `python scripts/generate.py context-summary` output
  - Structure: Phase → Summary → Decisions (from DB) → Governance (from DB) → Built/Not Built → Focus → Last Session → Key Files
  - Include `.mesh/governance.db` in Key Files section
  - Note that decisions.md is generated from DB

  **Must NOT do**: Do NOT exceed 120 lines. Do NOT hardcode decisions — use DB output.

  **Recommended Agent Profile**: `quick`
  **Parallelization**: Wave 2, parallel with 5,6,7,8. Blocks 9. Blocked by 3.
  **References**: `scripts/generate.py` (from Task 2), `discovery/INDEX.md`

  **QA Scenarios:**
  ```
  Scenario: CONTEXT.md uses DB for summaries
    Tool: Bash
    Steps:
      1. wc -l CONTEXT.md → <= 120
      2. grep "governance.db" CONTEXT.md → match
      3. grep -c "D[0-9]" CONTEXT.md → >= 10
    Expected: Under 120 lines, references DB, has all decisions
    Evidence: .sisyphus/evidence/task-4-context-md.txt
  ```

  **Commit**: YES — `chore(governance): add session continuity and references`

- [x] 5. Create SESSION-LOG.md

  **What to do**:
  - Create `SESSION-LOG.md` at project root
  - Log this session in DB: `python scripts/session.py start/end`
  - First entry documents THIS session (governance design, D-GOV1-8, research findings)

  **Must NOT do**: Do NOT exceed 50 lines per entry. Managed by mesh-nav only.

  **Recommended Agent Profile**: `quick`
  **Parallelization**: Wave 2, parallel with 4,6,7,8. Blocks 9. Blocked by: none.
  **References**: `scripts/session.py` (from Task 2)

  **QA Scenarios:**
  ```
  Scenario: Session in both markdown and DB
    Tool: Bash
    Steps:
      1. grep -c "## Session" SESSION-LOG.md → >= 1
      2. python scripts/session.py latest → returns entry
      3. grep -c "D-GOV" SESSION-LOG.md → >= 8
    Expected: Both markdown and DB have session
    Evidence: .sisyphus/evidence/task-5-session-log.txt
  ```

  **Commit**: YES — same as Task 4

- [x] 6. Create folder-structure.md Reference

  **What to do**:
  - Create `~/.agents/skills/mesh-nav/references/folder-structure.md`
  - Map full discovery/ structure (3 files + 4 dirs)
  - ADD: `.mesh/governance.db` to manifest
  - ADD: Mark decisions.md, governance-decisions.md as "generated from DB"
  - Under 200 lines

  **Recommended Agent Profile**: `quick`
  **Parallelization**: Wave 2, parallel with 4,5,7,8. Blocks 7. Blocked by 1.

  **QA Scenarios:**
  ```
  Scenario: Includes DB and marks generated files
    Tool: Read
    Steps:
      1. Verify .mesh/governance.db listed
      2. Verify decisions.md marked "generated from DB"
    Expected: DB listed, generated files marked
    Evidence: .sisyphus/evidence/task-6-folder-structure.txt
  ```

  **Commit**: YES — same as Task 4

- [x] 7. Create reading-map.md Reference

  **What to do**:
  - Create `~/.agents/skills/mesh-nav/references/reading-map.md`
  - 4 maps: bootstrap, continuation, decision, research
  - ADD: Decision map includes using `python scripts/gov.py conflicts` to check relationships, not just reading files
  - ADD: Bootstrap map includes `python scripts/session.py latest` for session context
  - Under 200 lines

  **Recommended Agent Profile**: `unspecified-high`
  **Parallelization**: Wave 2, parallel with 4,5,6,8. Blocks 9. Blocked by 1, 6.

  **QA Scenarios:**
  ```
  Scenario: Reading map includes DB queries
    Tool: Read
    Steps:
      1. Verify decision map mentions gov.py for conflict checking
      2. Verify bootstrap map mentions session.py
      3. Verify 4 maps total
    Expected: Scripts integrated, not just file reads
    Evidence: .sisyphus/evidence/task-7-reading-map.txt
  ```

  **Commit**: YES — same as Task 4

- [x] 8. Create update-protocols.md Reference

  **What to do**:
  - Create `~/.agents/skills/mesh-nav/references/update-protocols.md`
  - Same structure as before: post-task checklist, session end, drift detection
  - ADD: "After making a decision" → run `gov.py add_entity` + `gov.py add_edge` + `generate.py decisions-md`
  - ADD: "Session end" → run `session.py end`
  - ADD: Drift detection → compare generated markdown with on-disk, flag if different
  - Under 200 lines

  **Recommended Agent Profile**: `unspecified-high`
  **Parallelization**: Wave 2, parallel with 4,5,6,7. Blocks 9. Blocked by 1.

  **QA Scenarios:**
  ```
  Scenario: Protocols use scripts, not manual editing
    Tool: Read
    Steps:
      1. Verify decision protocol mentions gov.py + generate.py
      2. Verify session end mentions session.py
      3. Verify drift mentions comparing generated vs on-disk
    Expected: All protocols reference scripts
    Evidence: .sisyphus/evidence/task-8-update-protocols.txt
  ```

  **Commit**: YES — same as Task 4

- [x] 9. Create mesh-nav SKILL.md Core

  **What to do**:
  - Create `~/.agents/skills/mesh-nav/SKILL.md`
  - Follow office-hours pattern: Phase 0 bootstrap → 1A-1D modes → Phase 2 compress
  - **KEY**: Skill calls Python scripts, NEVER raw SQL
  - Phase 0: `session.py latest` + `generate.py context-summary`
  - Phase 1A: `gov.py conflicts <id>` before deciding, `gov.py add_entity` after
  - Phase 1A: `generate.py decisions-md` to regenerate markdown after DB change
  - Phase 2: `session.py end` + `generate.py` if decisions changed
  - Include "Scripts Reference" section listing all commands
  - 300-400 lines

  **Must NOT do**: Do NOT exceed 400 lines. Do NOT include SQL. Do NOT include gstack boilerplate.

  **Recommended Agent Profile**: `deep` — Most complex deliverable, ties everything together
  **Parallelization**: Wave 3. Blocks 10. Blocked by 2, 4, 5, 7, 8.

  **References**:
  - `~/.claude/skills/gstack/openclaw/skills/gstack-openclaw-office-hours/SKILL.md` — Pattern to follow
  - All Python scripts (from Task 2) — Commands to reference
  - All reference files (from Tasks 6, 7, 8) — Content to reference

  **QA Scenarios:**
  ```
  Scenario: No SQL, uses scripts
    Tool: Bash
    Steps:
      1. grep -c "sqlite3\|SELECT\|INSERT\|CREATE TABLE" SKILL.md → 0
      2. grep -c "scripts/gov.py\|scripts/session.py\|scripts/generate.py" SKILL.md → >= 5
      3. wc -l SKILL.md → <= 400
    Expected: Zero SQL, multiple script refs, under 400 lines
    Evidence: .sisyphus/evidence/task-9-skill.txt

  Scenario: All phases present
    Tool: Read
    Steps:
      1. Phase 0 (Bootstrap) exists → calls session.py
      2. Phase 1A (Design) exists → calls gov.py
      3. Phase 2 (Compress) exists → calls session.py + generate.py
    Expected: All phases reference appropriate scripts
    Evidence: .sisyphus/evidence/task-9-phases.txt
  ```

  **Commit**: YES — `feat(skills): add mesh-nav governance skill with SQLite integration`

- [x] 10. Update AGENTS.md

  **What to do**:
  - Update path references (from Task 1 restructure)
  - Add governance section (~15 lines): references mesh-nav, .mesh/governance.db
  - Add note: decisions.md is generated from DB (don't hand-edit)
  - Add: "Read CONTEXT.md first" before INDEX.md
  - Total under 80 lines

  **Must NOT do**: Do NOT exceed 80 lines. Do NOT remove existing content.

  **Recommended Agent Profile**: `quick`
  **Parallelization**: Wave 3. Blocks F1-F4. Blocked by 9.

  **QA Scenarios:**
  ```
  Scenario: References DB and generated files
    Tool: Bash
    Steps:
      1. wc -l AGENTS.md → <= 80
      2. grep "governance.db" AGENTS.md → match
      3. grep "generated from DB\|don't hand-edit" AGENTS.md → match
      4. grep "mesh-nav" AGENTS.md → match
      5. Verify all original content preserved
    Expected: Under 80 lines, DB ref, generated file warning
    Evidence: .sisyphus/evidence/task-10-agents-md.txt
  ```

  **Commit**: YES — same as Task 9

---

## Final Verification Wave (MANDATORY — after ALL implementation tasks)

> 4 review agents run in PARALLEL. ALL must APPROVE. Present consolidated results to user and get explicit "okay" before completing.

- [x] F1. **Plan Compliance Audit** — `oracle`
  Read the plan end-to-end. For each "Must Have": verify file exists (read file, check content). For each "Must NOT Have": search codebase for forbidden patterns (grep for SQL in SKILL.md, count lines). Check evidence files exist in .sisyphus/evidence/. Verify .mesh/governance.db has all 34 entities. Compare deliverables against plan.
  Output: `Must Have [N/N] | Must NOT Have [N/N] | Tasks [N/N] | VERDICT: APPROVE/REJECT`

- [x] F2. **Content Quality Review** — `unspecified-high`
  Read ALL created files. Check for: markdown errors, broken links, AI slop. Verify CONTEXT.md decision summaries match DB content (run generate.py context-summary and compare). Verify generated decisions.md matches DB entities. Check Python scripts for error handling, edge cases, correct CTE traversal.
  Output: `Files [N clean/N issues] | DB-Markdown Sync [PASS/FAIL] | VERDICT`

- [x] F3. **Real QA — Fresh Bootstrap Test** — `unspecified-high`
  Simulate fresh session: run `python scripts/session.py latest` → run `python scripts/generate.py context-summary` → run `python scripts/gov.py trace D3 --depth 2` → verify graph traversal returns connected decisions. Then read CONTEXT.md and verify it answers: what Mesh is, current phase, last 3 decisions, what to work on next.
  Output: `Bootstrap [PASS/FAIL] | Graph Queries [N/N pass] | Accuracy [PASS/FAIL] | VERDICT`

- [x] F4. **Scope Fidelity Check** — `deep`
  Verify: no file renamed (only moved), AGENTS.md under 80 lines, SKILL.md under 400 lines, reference files under 200 lines, Python scripts use only stdlib, zero pip deps, SKILL.md has zero SQL, DB has all 34 entities seeded.
  Output: `Tasks [N/N compliant] | Creep [CLEAN/N items] | VERDICT`

---

## Commit Strategy

- **Wave 1 commit**: `feat(governance): add SQLite graph DB, scripts, and seed from existing decisions`
- **Wave 2 commit**: `chore(governance): add session continuity files and references`
- **Wave 3 commit**: `feat(skills): add mesh-nav governance skill with SQLite integration`
- **Pre-commit**: verify markdown files parse, Python scripts respond to --help

---

## Success Criteria

### Verification Commands
```bash
# DB exists with correct tables
sqlite3 .mesh/governance.db ".tables"   # Expected: entities edges sessions

# Entity count
sqlite3 .mesh/governance.db "SELECT COUNT(*) FROM entities"  # Expected: 34

# Graph traversal works
python scripts/gov.py trace D1 --depth 2  # Expected: connected decisions

# Scripts generate markdown
python scripts/generate.py decisions-md | head -5  # Expected: valid markdown

# Folder structure
ls discovery/ | grep -c "\.md$"   # Expected: 3
ls -d discovery/*/               # Expected: state/ design/ research/ archive/

# Line counts
wc -l AGENTS.md                   # Expected: <80
wc -l ~/.agents/skills/mesh-nav/SKILL.md  # Expected: <400

# CONTEXT.md and SESSION-LOG.md exist
head -5 CONTEXT.md                # Expected: project state
grep -c "## Session" SESSION-LOG.md  # Expected: >= 1
```

### Final Checklist
- [ ] All "Must Have" present
- [ ] All "Must NOT Have" absent
- [ ] DB seeded with all 34 entities
- [ ] Graph traversal returns connected decisions
- [ ] Python scripts work (zero pip deps)
- [ ] SKILL.md has zero SQL references
- [ ] Generated markdown matches DB content
- [ ] Cross-references resolve after restructure
- [ ] Fresh agent can bootstrap from CONTEXT.md alone
