# v1.0 Merge to Main

## TL;DR

> **Quick Summary**: Fast-forward merge mesh-v1-implementation (13 commits) into main, cleanup accidental artifacts, update governance DB to reflect v1.0 completion, create v1.1 roadmap from DE1-DE16 design decisions, tag v1.0.0.
> 
> **Deliverables**:
> - Cleaned git working tree (no binary, no SQLite temps, no stale drafts)
> - Updated governance DB (S6 session, Q1-Q4 resolved, D6/D10 updated, learnings recorded)
> - Merged main branch at mesh-v1-implementation HEAD
> - v1.0.0 tag
> - `discovery/roadmap/v1.1-refinements.md`
> - Verified build + test + vet on main
> 
> **Estimated Effort**: Medium
> **Parallel Execution**: YES — 5 waves
> **Critical Path**: Task 1 → Task 5 (validate content) → Tasks 6-8 (apply validated writes) → Task 10 → Task 11 → Task 12 → Task 13 → F1-F3
> **Governance Integrity**: Content validation BEFORE writes (Task 5), content audit AFTER writes (F1). Zero unverified claims in DB.

---

## Context

### Original Request
Merge the completed mesh-v1-implementation branch (v1.0, 13 commits) into main. Fix governance DB staleness (S5 says "created plan" but code was built). Clean up accidental artifacts. Tag v1.0.0. Create v1.1 roadmap from parallel design session findings. Constraints: no GitHub push, no mesh-design worktree changes, no squash.

### Interview Summary
**Key Discussions**:
- **Merge strategy**: Fast-forward (main hasn't moved from e73d37c). No merge commit bubble needed.
- **Artifact cleanup**: Delete 30MB compiled `mesh` binary, `dist/` build artifacts, SQLite temp files (tracked — needs `git rm --cached`), ALL 14 drafts (7 tracked + 7 untracked in `.sisyphus/drafts/`).
- **Governance DB**: Full update — record S6, resolve Q1-Q4 via DE1-DE16, update D6, update D10, add learnings, regenerate views.
- **v1.1 roadmap**: Created in mesh-impl worktree, merged with everything else.
- **Verification**: `go build ./... && go test ./... && go vet ./...` pre-merge AND post-merge.

**Research Findings**:
- **Branch state**: 13 commits on mesh-v1-implementation, main at e73d37c. Clean fast-forward.
- **DE1-DE16**: 16 design decisions in mesh-design worktree. DE4 supersedes D6. DE8 specifies Docker as plugin (v1.1).
- **SQLite temps are TRACKED**: 5 WAL/SHM/BAK files in git. Cleanup requires `git rm --cached` + `.gitignore` fix.
- **D10 divergence**: mesh-impl D10 says "no integration"; mesh-design D10 says "valid substrate target via adapter."

### Metis Review
**Identified Gaps** (addressed):
- SQLite artifacts tracked → `git rm --cached` + gitignore
- Draft count mismatch → all 14 cleaned
- `dist/` artifacts → included in cleanup
- Pre-merge verification order → gate task first
- D10 divergence → included in governance update
- Rollback, guardrails, edge cases → documented per task

---

## Work Objectives

### Core Objective
Complete a clean, verifiable fast-forward merge of the v1.0 implementation into main with accurate governance records and a v1.1 roadmap.

### Concrete Deliverables
- Clean working tree on mesh-v1-implementation
- Updated governance DB (S6, Q1-Q4 resolved, D6/D10 updated, learnings, regenerated views)
- `discovery/roadmap/v1.1-refinements.md`
- Merged main at 7018aa8
- v1.0.0 tag
- Verified build + test + vet on main

### Definition of Done
- [ ] `go build ./... && go test ./... && go vet ./... → all pass` on main
- [ ] `git rev-parse main` = tip of mesh-v1-implementation
- [ ] v1.0.0 annotated tag exists
- [ ] S6 completed in governance DB
- [ ] Q1-Q4 all resolved
- [ ] D6/D10 updated
- [ ] Learnings ≥2 entries
- [ ] No build artifacts, SQLite temps, or drafts in working tree
- [ ] `discovery/roadmap/v1.1-refinements.md` exists with all 16 DE decisions

### Must Have
- Fast-forward merge (no merge commit, no squash)
- Governance DB reflects v1.0 completion reality
- All accidental artifacts cleaned
- v1.0.0 tag
- v1.1 roadmap document
- Verified build on main

### Must NOT Have (Guardrails)
- NO push to GitHub (local operations only)
- NO modifications to mesh-design worktree
- NO squash commits
- NO merge commit bubble
- NO hand-editing of generated governance views
- NO touching `mesh-v1-design-exploration` branch or worktree

---

## Verification Strategy

> **ZERO HUMAN INTERVENTION** — ALL verification is agent-executed.

### Test Decision
- **Infrastructure exists**: YES (Go test suite)
- **Automated tests**: Tests-after (run existing suite, no new tests required)
- **Framework**: `go test`
- **Verification gates**: Build → Test → Vet

### QA Policy
Every task includes agent-executed QA scenarios. Evidence saved to `.sisyphus/evidence/task-{N}-{scenario-slug}.{ext}`.

---

## Execution Strategy

### Parallel Execution Waves

```
Wave 1 (Gate — mesh-impl worktree):
└── Task 1: Pre-merge verification [quick]

Wave 2 (MAX PARALLEL — mesh-impl worktree):
├── Task 2: Cleanup build artifacts [quick]
├── Task 3: Remove tracked SQLite files + fix .gitignore [quick]
├── Task 4: Remove all drafts from .sisyphus/drafts/ [quick]
├── Task 5: Research & validate governance DB content [deep]
│     ↑ VALIDATION GATE — produces verified content spec before any DB writes
└── Task 9: Create v1.1 roadmap document [writing]

Wave 3 (Sequential on DB — mesh-impl worktree):
├── Task 6: Record S6 implementation session [quick]
├── Task 7: Resolve Q1-Q4 + update D6/D10 [quick]
├── Task 8: Add learnings + regenerate all views [quick]
│     ↑ All writes use content validated in Task 5

Wave 4 (Sequential — mesh worktree /main):
├── Task 10: Commit all changes on mesh-v1-implementation [quick]
├── Task 11: Fast-forward merge → main [quick]
└── Task 12: Tag v1.0.0 [quick]

Wave 5 (Sequential — mesh worktree /main):
├── Task 13: Post-merge verification [quick]

Wave FINAL (Parallel review):
├── F1: Governance content integrity audit [deep]
│     ↑ Cross-references every DB claim against source evidence
├── F2: Deliverable completeness check [quick]
└── F3: Edge case sweep [quick]
```

**Critical Path**: Task 1 → Task 5 (validate) → Tasks 6-8 (apply validated writes) → Task 10 → Task 11 → Task 12 → Task 13 → F1-F3
**Max Parallel**: 5 tasks in Wave 2 (Tasks 2, 3, 4, 5, 9)
**Governance Gate**: Task 5 must APPROVE all proposed DB records before Tasks 6-8 execute

---

## TODOs

- [x] 1. Pre-merge Verification

  **What to do**:
  - Run `go build ./...`, `go test ./...`, `go vet ./...` in mesh-impl worktree
  - If ANY fails, STOP — do not proceed

  **Must NOT do**: Do NOT fix code — verification only.

  **Recommended Agent Profile**:
  - **Category**: `quick` — Reason: Single verification with zero ambiguity
  - **Skills**: `[]`

  **Parallelization**: Wave 1 (Gate) | Blocks: Tasks 2-8 | Blocked By: None

  **References**:
  - `go.mod` — Module root defines all packages
  - `.sisyphus/plans/mesh-v1-implementation.md` — Implementation plan that produced the code

  **Acceptance Criteria**:
  - [ ] `go build ./...` exits 0
  - [ ] `go test ./...` exits 0, no FAIL
  - [ ] `go vet ./...` exits 0

  **QA Scenarios (MANDATORY)**:

  ```
  Scenario: All Go checks pass on implementation branch
    Tool: Bash (workdir: mesh-impl)
    Preconditions: On mesh-v1-implementation branch
    Steps:
      1. go build ./... && echo "BUILD: PASS"
      2. go test ./... 2>&1 | tee /tmp/pre-test.txt && echo "TEST: PASS"
      3. go vet ./... && echo "VET: PASS"
      4. grep "FAIL" /tmp/pre-test.txt && echo "FAILURES FOUND" || echo "NO FAILURES"
    Expected Result: All three exit 0, no FAIL lines
    Evidence: .sisyphus/evidence/task-1-premerge-verify.txt
  ```

  **Evidence**: `.sisyphus/evidence/task-1-premerge-verify.txt`

  **Commit**: NO

- [x] 2. Cleanup Build Artifacts

  **What to do**: Delete compiled `mesh` binary and `dist/` directory.

  **Must NOT do**: Do NOT delete source directories.

  **Recommended Agent Profile**:
  - **Category**: `quick` — Reason: Simple file deletion

  **Parallelization**: Wave 2 | YES (with Tasks 3, 4, 8) | Blocks: Task 9 | Blocked By: Task 1

  **Acceptance Criteria**:
  - [ ] `test ! -f mesh`
  - [ ] `test ! -d dist`

  **QA Scenarios**:

  ```
  Scenario: Build artifacts deleted
    Tool: Bash (workdir: mesh-impl)
    Steps:
      1. rm mesh && rm -rf dist/
      2. test ! -f mesh && echo "PASS: mesh gone"
      3. test ! -d dist && echo "PASS: dist gone"
    Evidence: .sisyphus/evidence/task-2-cleanup.txt
  ```

  **Evidence**: `.sisyphus/evidence/task-2-cleanup.txt`

  **Commit**: NO (committed in Task 9)

- [x] 3. Remove Tracked SQLite Files + Fix .gitignore

  **What to do**:
  - `git rm --cached` for 5 tracked SQLite temp files: `.mesh/governance.db-shm`, `.mesh/governance.db-wal`, `.mesh/governance.db.v1.bak`, `.mesh/tmpal_011ng.db-shm`, `.mesh/tmpal_011ng.db-wal`
  - `rm` the actual files from disk
  - Add to `.gitignore`: `*.db-shm`, `*.db-wal`, `*.db.v1.bak`, `tmpal_*.db-*` under `.mesh/` section

  **Must NOT do**: Do NOT remove `.mesh/governance.db` itself.

  **Recommended Agent Profile**:
  - **Category**: `quick` — Reason: Git index + file operations

  **Parallelization**: Wave 2 | YES (with Tasks 2, 4, 8) | Blocks: Task 9 | Blocked By: Task 1

  **Acceptance Criteria**:
  - [ ] `git ls-files .mesh/ | grep -c 'shm\|wal\|bak\|tmpal'` → 0
  - [ ] All 5 files absent from disk
  - [ ] `.gitignore` has SQLite exclusion patterns

  **QA Scenarios**:

  ```
  Scenario: SQLite temps removed from tracking and disk
    Tool: Bash (workdir: mesh-impl)
    Steps:
      1. git rm --cached .mesh/governance.db-shm .mesh/governance.db-wal .mesh/governance.db.v1.bak
      2. git rm --cached .mesh/tmpal_011ng.db-shm .mesh/tmpal_011ng.db-wal
      3. rm -f .mesh/governance.db-shm .mesh/governance.db-wal .mesh/governance.db.v1.bak .mesh/tmpal_011ng.db-shm .mesh/tmpal_011ng.db-wal
      4. git ls-files .mesh/ | grep -c 'shm\|wal\|bak\|tmpal' → "0"
    Evidence: .sisyphus/evidence/task-3-sqlite.txt
  ```

  **Evidence**: `.sisyphus/evidence/task-3-sqlite.txt`

  **Commit**: NO (committed in Task 9)

- [x] 4. Remove All Drafts from .sisyphus/drafts/

  **What to do**: Remove ALL 14 drafts (7 tracked via `git rm --cached` + `rm`, 7 untracked via `rm`).

  **Must NOT do**: Do NOT remove `.sisyphus/plans/`.

  **Recommended Agent Profile**:
  - **Category**: `quick` — Reason: File identification + deletion

  **Parallelization**: Wave 2 | YES (with Tasks 2, 3, 8) | Blocks: Task 9 | Blocked By: Task 1

  **Acceptance Criteria**:
  - [ ] `ls .sisyphus/drafts/*.md 2>/dev/null` → no output
  - [ ] `git ls-files .sisyphus/drafts/` → EMPTY

  **QA Scenarios**:

  ```
  Scenario: All drafts removed
    Tool: Bash (workdir: mesh-impl)
    Steps:
      1. for f in $(git ls-files .sisyphus/drafts/); do git rm --cached "$f"; rm "$f"; done
      2. rm -f .sisyphus/drafts/*.md
      3. ls .sisyphus/drafts/*.md 2>&1 | grep "No such file"
      4. git ls-files .sisyphus/drafts/ → empty
    Evidence: .sisyphus/evidence/task-4-drafts.txt
  ```

  **Evidence**: `.sisyphus/evidence/task-4-drafts.txt`

  **Commit**: NO (committed in Task 9)

- [x] 5. Research & Validate Governance DB Content

  **What to do**:
  **VALIDATION GATE — no DB writes until this passes. Every claim below must be cross-referenced against primary sources.**

  **S6 Session Content** (source: git log mesh-v1-implementation):
  - Verify by reading `git log mesh-v1-implementation --oneline -14` — confirm 13 implementation commits exist
  - Read `git diff --stat e73d37c..7018aa8` — confirm ~72 files changed
  - Read `go test ./...` output (from Task 1) — confirm 18 packages pass
  - Proposed S6 summary: "Completed Mesh v1.0 implementation. Built: daemon with Docker + Nomad multi-adapter routing, 16 MCP tools (create/start/stop/destroy/exec/snapshot/restore/migrate/logs/status/plugins), 7-step cold migration coordinator with S3 registry, plugin system (go-plugin + gRPC + protobuf), CLI (mesh serve/stop/status), bootstrap (goreleaser, install.sh, Homebrew formula), CI (GitHub Actions with integration tests). 72 files changed (+12,882/-139 lines). 18 packages test-passing."

  **Q1 Resolution** (Where does a body live when idle?):
  - **Validate**: Read DE1 body from mesh-design `discovery/state/decisions.md` — DE1 says "all sandbox providers equal, evaluated on same criteria." Read DE2 body — says "static scheduler config, user explicitly selects substrate."
  - **Risk**: DE1 doesn't directly answer "where when idle?" — it says evaluate equally. DE2 says user picks. The actual answer is: user config decides. No DE fully answers this. The most honest resolution references DE2 for the config mechanism but doesn't claim full resolution.
  - Proposed: "Partially resolved by DE2: user explicitly configures substrate in ~/.mesh/config.yaml. Idle location is user choice, not system decision. Fleet for persistent (A1/A2), Local for development (A5), Sandbox for burst (A3/A4). Full cost-model optimization is a v2.0 concern."
  - **Verify**: Read DE2 body, confirm it addresses static config

  **Q2 Resolution** (Registry strategy — where do snapshots live?):
  - **Validate**: Check actual v1.0 code — `git log mesh-v1-implementation --oneline | grep registry` confirms S3 registry plugin exists. Read `internal/` structure for registry code.
  - **Risk**: DE5 is about PLUGIN distribution (Git), not snapshot storage. DE10 is about SQLite STORE backup. Neither directly answers Q2 about snapshot registry. The implementation (S3 registry plugin) is the actual answer.
  - Proposed: "Resolved by v1.0 implementation: S3/R2 for snapshot storage via registry plugin (commit 9d2eda3). Local snapshots at ~/.mesh/snapshots/. DE10 covers SQLite store backup (separate concern)."
  - **Verify**: Read commit 9d2eda3 diff, confirm S3 registry code path exists

  **Q3 Resolution** (Scheduler — core or plugin?):
  - **Validate**: Read DE2 body — confirms "static scheduler config for v1.1 — no auto-scheduling" and "substrate selection is neither core nor plugin — it's user-config static."
  - Proposed: "Resolved by DE2: static scheduler config for v1.1. Substrate selection is user-config static — neither core nor plugin. Plugin-based scheduler is v2.0 consideration."
  - **Verify**: Read DE2 body, confirm it explicitly addresses core vs plugin

  **Q4 Resolution** (Bootstrap — first install without MCP?):
  - **Validate**: Check actual v1.0 code — `git log mesh-v1-implementation --oneline | grep bootstrap` confirms bootstrap commit (5b368a4). Read `scripts/install.sh` if it exists. Read `goreleaser` config.
  - Proposed: "Resolved by v1.0 implementation: CLI bootstrap via `mesh init` + install.sh/Homebrew formula (commit 5b368a4). MCP is primary ongoing interface, initial installation is CLI-based."
  - **Verify**: Read commit 5b368a4 diff, confirm bootstrap artifacts

  **D6 Update** (Provider integrations are plugins):
  - **Validate**: Read current D6 body from mesh-impl DB. Check what v1.0 actually shipped: `git log mesh-v1-implementation --oneline | grep plugin` confirms go-plugin + gRPC + protobuf. DE4 specifies OpenAPI codegen for v1.1.
  - **Risk**: Updating D6 to say "OpenAPI codegen" when v1.0 shipped with go-plugin is misleading. D6 should reflect BOTH: v1.0 reality (go-plugin) AND v1.1 direction (OpenAPI per DE4).
  - Proposed update to D6 body: "Provider integrations are plugins. v1.0 implementation uses go-plugin + gRPC + protobuf (commits d8eb6fc, 9d2eda3). DE4 (from v1.1 design session) specifies OpenAPI + oapi-codegen v2 + AI mapping layer as the v1.1 generation pipeline, superseding the earlier Pulumi approach."
  - **Verify**: Read commits d8eb6fc and 9d2eda3 diffs, confirm go-plugin usage

  **D10 Update** (Daytona relationship):
  - **Validate**: Read current D10 body from mesh-impl DB. The body ALREADY says "Daytona IS a valid substrate target for adapter generation." The title/summary says "no integration" — the body is correct, the title is stale.
  - Proposed: Update D10 title to: "Mesh is separate from Daytona — Daytona is valid substrate target via adapter, not dependency." Body already correct — verify.
  - **Verify**: Read D10 body from DB, confirm it already says "valid substrate target"

  **Learning L1** (Docker in core for v1.0):
  - **Validate**: DE8 from mesh-design says "Docker adapter is a plugin." Read v1.0 code — `internal/docker/` exists directly in core. User explicitly said "Docker staying in core for v1.0 is an intentional v1.0 decision."
  - Proposed: `learning.py add --what "Docker adapter is built-in for v1.0" --why "DE8 (v1.1) makes Docker a plugin, but v1.0 shipped with Docker in core as intentional simplification" --where "internal/docker/" --learned "Shipping with Docker in core is the right v1.0 decision; extraction to plugin is a v1.1 refinement per DE8" --category architecture --confidence 5`
  - **Verify**: Read `internal/docker/` directory, confirm Docker adapter code exists

  **Learning L2** (v1.0 implementation complete):
  - **Validate**: Task 1 output confirms 18 packages test-passing. Git log confirms 13 commits.
  - Proposed: `learning.py add --what "v1.0 implementation complete — 18 test-passing packages" --why "Daemon + multi-adapter + cold migration + plugin system form MVP agent-body runtime" --where "cmd/, internal/" --learned "The adapter pattern (Docker + Nomad via gRPC plugins) works at v1.0 scale. DB-backed body state machine enables reliable lifecycle management." --category implementation --confidence 5`
  - **Verify**: Task 1 test output, git log

  **Output**: Produce `.sisyphus/evidence/task-5-validated-content.md` containing:
  - [ ] Each proposed DB record with its source citation and verification status
  - [ ] Flagged risks (Q1 partial resolution, D6 dual-timeline) with explicit acknowledgment
  - [ ] Decision: APPROVE ALL / APPROVE WITH NOTES / BLOCKED (and which records)

  **Must NOT do**:
  - Do NOT write to the governance DB — this task only validates content
  - Do NOT resolve any question without citing the exact paragraph from the source
  - Do NOT update any decision without checking what v1.0 actually built

  **Recommended Agent Profile**:
  - **Category**: `deep` — Reason: Cross-referencing multiple source systems (git log, mesh-design decisions, DB state, code structure) requires thorough research
  - **Skills**: `[]`

  **Parallelization**: Wave 2 | YES (with Tasks 2, 3, 4, 9) | Blocks: Tasks 6, 7, 8 | Blocked By: Task 1

  **References**:
  - `git log mesh-v1-implementation --oneline -14` — Implementation commit history (primary source for S6, L2)
  - `git diff --stat e73d37c..7018aa8` — File change statistics
  - `/Users/samanvayayagsen/project/rethink-paradigms/mesh-design/discovery/state/decisions.md` — DE1-DE16 source for Q1-Q4 resolutions
  - `.mesh/governance.db` — Current DB state (READ ONLY for this task)
  - `discovery/state/decisions.md` — Current D6, D10 bodies
  - `discovery/state/open-questions.md` — Current Q1-Q4 state
  - `internal/docker/` — Verify Docker adapter location
  - Commit hashes: `9d2eda3` (S3 registry), `d8eb6fc` (go-plugin), `5b368a4` (bootstrap)

  **Acceptance Criteria**:
  - [ ] All 10 proposed DB records have explicit source citations
  - [ ] Any risks or partial resolutions flagged explicitly
  - [ ] Validated content document exists at `.sisyphus/evidence/task-5-validated-content.md`
  - [ ] Document includes APPROVE/BLOCKED verdict per record

  **QA Scenarios (MANDATORY)**:

  ```
  Scenario: Every proposed DB record cross-referenced against primary sources
    Tool: Bash + Read (workdir: mesh-impl)
    Preconditions: Task 1 complete, mesh-design worktree accessible
    Steps:
      1. Read git log for S6/L2 validation
      2. Read mesh-design decisions.md for Q1-Q4 validation
      3. Read mesh-impl decisions.md for D6/D10 current state
      4. Read internal/docker/ for L1 validation
      5. Read specific commit diffs (9d2eda3, d8eb6fc, 5b368a4) for implementation claims
      6. Write validated-content.md with per-record source citations and verdict
    Expected Result: All records have verified source citations. Any risks flagged.
    Failure Indicators: Record without source citation, mismatch between claim and source
    Evidence: .sisyphus/evidence/task-5-validated-content.md
  ```

  **Evidence to Capture**:
  - [ ] `.sisyphus/evidence/task-5-validated-content.md` — Complete validation report

  **Commit**: NO

- [x] 6. Governance DB — Record S6 Implementation Session

  **What to do**:
  - Read validated content from `.sisyphus/evidence/task-5-validated-content.md`
  - Start S6 with `session.py start --date 2026-04-30 --type implement`
  - End S6 with the validated summary from Task 5
  - Update S5 summary to reflect plan-to-implementation transition using content from Task 5

  **Must NOT do**: Do NOT deviate from Task 5 validated content.

  **Recommended Agent Profile**:
  - **Category**: `quick` — Reason: Apply pre-validated content

  **Parallelization**: Wave 3 | Sequential on DB | Blocks: Task 7 | Blocked By: Task 5

  **Acceptance Criteria**:
  - [ ] `session.py brief` shows 4 sessions (S3-S6)
  - [ ] S6 type=implement, status=completed
  - [ ] S6 summary matches validated content (verify via `gov.py get S6`)

  **QA Scenarios**:

  ```
  Scenario: S6 session recorded with validated content
    Tool: Bash (workdir: mesh-impl)
    Steps:
      1. python3 ~/.agents/skills/mesh-nav/scripts/session.py start --date 2026-04-30 --type implement
      2. python3 ~/.agents/skills/mesh-nav/scripts/session.py end --id <N> --summary "<from task-5-validated-content>"
      3. python3 ~/.agents/skills/mesh-nav/scripts/gov.py get S6
      4. diff <(gov.py get S6 summary) <(grep "S6" task-5-validated-content.md)
    Evidence: .sisyphus/evidence/task-6-s6.txt
  ```

  **Evidence**: `.sisyphus/evidence/task-6-s6.txt`

  **Commit**: NO (committed in Task 10)

- [x] 7. Governance DB — Resolve Q1-Q4 + Update D6/D10

  **What to do**:
  - Read validated resolutions from `.sisyphus/evidence/task-5-validated-content.md`
  - Apply each validated resolution via `gov.py resolve_question` or `gov.py update_decision`
  - Regenerate views: `generate.py decisions-md && generate.py questions-md`

  **Must NOT do**: Do NOT apply any resolution not validated in Task 5. Do NOT hand-edit generated files.

  **Recommended Agent Profile**:
  - **Category**: `quick` — Reason: Apply pre-validated content

  **Parallelization**: Wave 3 | Sequential on DB | Blocks: Task 8 | Blocked By: Task 6

  **Acceptance Criteria**:
  - [ ] Q1-Q4 all resolved (verify via `gov.py list --type question`)
  - [ ] Each resolution text matches Task 5 validated content
  - [ ] D6 body matches Task 5 validated content (verify via `gov.py get D6`)
  - [ ] D10 title/body match Task 5 validated content
  - [ ] `generate.py decisions-md questions-md` exits 0

  **QA Scenarios**:

  ```
  Scenario: Validated resolutions applied correctly
    Tool: Bash (workdir: mesh-impl)
    Steps:
      1. Apply each resolution from task-5-validated-content.md
      2. python3 ~/.agents/skills/mesh-nav/scripts/gov.py list --type question
      3. For each Q: gov.py get <Q> and diff against validated content
      4. python3 ~/.agents/skills/mesh-nav/scripts/gov.py get D6
      5. python3 ~/.agents/skills/mesh-nav/scripts/gov.py get D10
      6. python3 ~/.agents/skills/mesh-nav/scripts/generate.py decisions-md questions-md
    Evidence: .sisyphus/evidence/task-7-governance.txt
  ```

  **Evidence**: `.sisyphus/evidence/task-7-governance.txt`

  **Commit**: NO (committed in Task 10)

- [x] 8. Governance DB — Add Learnings + Regenerate All Views

  **What to do**:
  - Read validated learnings from `.sisyphus/evidence/task-5-validated-content.md`
  - Apply each via `learning.py add` with exact validated parameters
  - Regenerate all views: `generate.py decisions-md questions-md governance-md learnings-md context-summary`

  **Recommended Agent Profile**:
  - **Category**: `quick` — Reason: Apply pre-validated content

  **Parallelization**: Wave 3 | Sequential on DB | Blocks: Task 10 | Blocked By: Task 7

  **Acceptance Criteria**:
  - [ ] `learning.py list` shows ≥2 learnings matching validated content
  - [ ] All `generate.py` commands exit 0
  - [ ] CONTEXT.md updated with correct entity counts

  **QA Scenarios**:

  ```
  Scenario: Validated learnings applied and views regenerated
    Tool: Bash (workdir: mesh-impl)
    Steps:
      1. Apply each learning from task-5-validated-content.md
      2. python3 ~/.agents/skills/mesh-nav/scripts/learning.py list
      3. python3 ~/.agents/skills/mesh-nav/scripts/generate.py decisions-md questions-md governance-md learnings-md context-summary
    Evidence: .sisyphus/evidence/task-8-learnings.txt
  ```

  **Evidence**: `.sisyphus/evidence/task-8-learnings.txt`

  **Commit**: NO (committed in Task 10)

- [x] 9. Create v1.1 Roadmap Document

  **What to do**:
  - Create `discovery/roadmap/v1.1-refinements.md`
  - Document all 16 DE decisions (DE1-DE16) with title, status, date, 2-3 sentence rationale
  - Add summary section at top
  - Add "Cross-Cutting Impact" section: DE4 (supersedes D6), DE8 (Docker plugin), DE1 (all providers equal), DE2 (static scheduler)
  - Source (read-only): mesh-design worktree `discovery/state/decisions.md`

  **Must NOT do**: Do NOT copy verbatim, do NOT modify mesh-design worktree.

  **Recommended Agent Profile**:
  - **Category**: `writing` — Reason: Document synthesis from structured source

  **Parallelization**: Wave 2 | YES (with Tasks 2, 3, 4) | Blocks: Task 9 | Blocked By: Task 1

  **Acceptance Criteria**:
  - [ ] `test -f discovery/roadmap/v1.1-refinements.md`
  - [ ] `wc -l` > 50 lines
  - [ ] `grep -c "DE"` ≥ 16

  **QA Scenarios**:

  ```
  Scenario: v1.1 roadmap created with all DE decisions
    Tool: Bash (workdir: mesh-impl)
    Steps:
      1. mkdir -p discovery/roadmap
      2. Create discovery/roadmap/v1.1-refinements.md
      3. wc -l → >50
      4. grep -c "DE[0-9]" → ≥16
      5. grep "DE1" && grep "DE16"
    Evidence: .sisyphus/evidence/task-8-roadmap.txt
  ```

  **Evidence**: `.sisyphus/evidence/task-8-roadmap.txt`

  **Commit**: NO (committed in Task 9)

- [ ] 10. Commit Cleanup + Governance + Roadmap Changes

  **What to do**:
  - Stage all changes from Tasks 2-9
  - Create atomic commit on mesh-v1-implementation: `chore: cleanup artifacts, update governance DB for v1.0 completion, add v1.1 roadmap`
  - Verify: clean working tree, 14 total commits (13 original + 1 cleanup)

  **Must NOT do**: Do NOT commit on main.

  **Recommended Agent Profile**:
  - **Category**: `quick` — Reason: Git staging + commit

  **Parallelization**: Wave 4 | Sequential | Blocks: Task 11 | Blocked By: Tasks 2-9

  **Acceptance Criteria**:
  - [ ] `git log --oneline -1` starts with "chore: cleanup"
  - [ ] `git branch --show-current` → mesh-v1-implementation
  - [ ] `git status` → clean

  **QA Scenarios**:

  ```
  Scenario: Cleanup commit created
    Tool: Bash (workdir: mesh-impl)
    Steps:
      1. git add -A
      2. git commit -m "chore: cleanup artifacts, update governance DB for v1.0 completion, add v1.1 roadmap"
      3. git log --oneline -1
      4. git status → clean
    Evidence: .sisyphus/evidence/task-10-commit.txt
  ```

  **Evidence**: `.sisyphus/evidence/task-10-commit.txt`

  **Commit**: YES — `chore: cleanup artifacts, update governance DB for v1.0 completion, add v1.1 roadmap`

- [ ] 11. Fast-Forward Merge mesh-v1-implementation → main

  **What to do**:
  - Switch to mesh worktree (`/Users/samanvayayagsen/project/rethink-paradigms/mesh`)
  - **GUARD**: Verify `git rev-parse main` = `e73d37c08e16afc25d7d23cc9472046ec090d267`. If NOT, STOP and report.
  - `git merge --ff-only mesh-v1-implementation`
  - Verify main advanced to mesh-v1-implementation HEAD

  **Must NOT do**: Do NOT use `--no-ff`, do NOT squash.

  **Rollback**: `git checkout main && git reset --hard e73d37c08e16afc25d7d23cc9472046ec090d267`

  **Recommended Agent Profile**:
  - **Category**: `quick` — Reason: Git merge with guard

  **Parallelization**: Wave 4 | Sequential | Blocks: Task 12 | Blocked By: Task 10

  **Acceptance Criteria**:
  - [ ] `git rev-parse main` = mesh-v1-implementation HEAD
  - [ ] Working tree clean

  **QA Scenarios**:

  ```
  Scenario: Fast-forward merge succeeds
    Tool: Bash (workdir: /Users/samanvayayagsen/project/rethink-paradigms/mesh)
    Steps:
      1. git checkout main
      2. test "$(git rev-parse main)" = "e73d37c08e16afc25d7d23cc9472046ec090d267" || exit 1
      3. git merge --ff-only mesh-v1-implementation
      4. git log --oneline -5
    Evidence: .sisyphus/evidence/task-11-merge.txt
  ```

  **Evidence**: `.sisyphus/evidence/task-11-merge.txt`

  **Commit**: NO

- [ ] 12. Tag v1.0.0

  **What to do**:
  - Verify `git tag -l v1.0.0` → empty
  - `MAIN_SHA=$(git rev-parse main)`
  - `git tag -a v1.0.0 "$MAIN_SHA" -m "Mesh v1.0.0 — portable agent-body runtime"`
  - Verify: `git rev-parse v1.0.0` = `$MAIN_SHA`, `git cat-file -t v1.0.0` → "tag"

  **Must NOT do**: Do NOT tag without explicit SHA (never rely on HEAD context).

  **Recommended Agent Profile**:
  - **Category**: `quick` — Reason: Single git tag

  **Parallelization**: Wave 4 | Sequential | Blocks: Task 13 | Blocked By: Task 11

  **Acceptance Criteria**:
  - [ ] `git tag -l v1.0.0` → "v1.0.0"
  - [ ] `git rev-parse v1.0.0` = main HEAD
  - [ ] Annotated tag confirmed

  **QA Scenarios**:

  ```
  Scenario: v1.0.0 annotated tag created
    Tool: Bash (workdir: /Users/samanvayayagsen/project/rethink-paradigms/mesh)
    Steps:
      1. MAIN_SHA=$(git rev-parse main)
      2. git tag -a v1.0.0 "$MAIN_SHA" -m "Mesh v1.0.0 — portable agent-body runtime"
      3. git rev-parse v1.0.0 | diff - <(echo "$MAIN_SHA") && echo "MATCH"
      4. git cat-file -t v1.0.0 → "tag"
    Evidence: .sisyphus/evidence/task-12-tag.txt
  ```

  **Evidence**: `.sisyphus/evidence/task-12-tag.txt`

  **Commit**: NO

- [ ] 13. Post-Merge Verification

  **What to do**: In mesh worktree on main:
  - `go build ./... && go test ./... && go vet ./...`
  - Verify roadmap exists, binary absent, no SQLite temps tracked, no drafts

  **Recommended Agent Profile**:
  - **Category**: `quick` — Reason: Build/test/vet + file checks

  **Parallelization**: Wave 5 | Sequential | Blocks: Final Wave | Blocked By: Task 12

  **Acceptance Criteria**:
  - [ ] All go commands pass
  - [ ] `test -f discovery/roadmap/v1.1-refinements.md`
  - [ ] `test ! -f mesh && test ! -d dist`
  - [ ] `git ls-files .mesh/ | grep -c 'shm\|wal\|bak\|tmpal'` → 0
  - [ ] `git ls-files .sisyphus/drafts/ | wc -l` → 0

  **QA Scenarios**:

  ```
  Scenario: Full verification on merged main
    Tool: Bash (workdir: /Users/samanvayayagsen/project/rethink-paradigms/mesh)
    Steps:
      1. go build ./... && echo "BUILD: PASS"
      2. go test ./... 2>&1 | tee /tmp/post-test.txt && echo "TEST: PASS"
      3. go vet ./... && echo "VET: PASS"
      4. grep -c "FAIL" /tmp/post-test.txt → 0
      5. test -f discovery/roadmap/v1.1-refinements.md && echo "ROADMAP: OK"
      6. test ! -f mesh && echo "BINARY: CLEAN"
      7. git ls-files .mesh/ | grep -c 'shm\|wal\|bak\|tmpal' → 0
    Evidence: .sisyphus/evidence/task-13-verify.txt
  ```

  **Evidence**: `.sisyphus/evidence/task-13-verify.txt`

  **Commit**: NO

---

## Final Verification Wave (MANDATORY — after ALL implementation tasks)

> 3 review tasks run in PARALLEL. ALL must APPROVE.
> **Do NOT auto-proceed. Wait for user's explicit "okay" before marking complete.**

- [ ] F1. **Governance Content Integrity Audit** — `deep`
  **CRITICAL: This is a content audit, not a structure audit. Every claim in the DB is cross-referenced against primary sources.**
  
  **For each record in the governance DB, verify:**
  1. **S6**: Read `gov.py get S6` → does summary match what `git log mesh-v1-implementation --oneline -14` actually shows? Any fabricated claims (commits that don't exist, files that weren't changed)?
  2. **Q1-Q4**: Read each resolution via `gov.py get Q<N>` → does the resolution text cite a specific DE or commit that actually exists? Read the cited source — does it say what the resolution claims?
  3. **D6**: Read `gov.py get D6` → does body acknowledge BOTH v1.0 reality (go-plugin) AND v1.1 direction (OpenAPI)? Or does it claim OpenAPI is what was built?
  4. **D10**: Read `gov.py get D10` → does title match body? Does body reflect the actual relationship (adapter target, not dependency)?
  5. **Learnings**: Read each via `learning.py list` → are the confidence levels justified? Are the "where" paths real directories? Are the "why" statements verifiable against DE decisions or git history?
  
  **Red flags that trigger REJECT:**
  - Resolution cites a DE that doesn't answer the question
  - Decision body contradicts what code actually shipped
  - Learning claims confidence 5 for something not verified
  - Any record with a source citation that doesn't exist or says something different
  
  Output: `S6 [VERIFIED/N records mismatched] | Q1-Q4 [N/N valid/N fabrications] | D6 [OK body references both v1.0+v1.1 / MISLEADING claims OpenAPI shipped] | D10 [OK title matches body / STALE title] | Learnings [N/N verified/N unverifiable] | VERDICT: APPROVE/REJECT`
  
  **Evidence**: `.sisyphus/evidence/f1-content-audit.md` — Full audit report with per-record findings and source citations

- [ ] F2. **Deliverable Completeness Check** — `quick`
  Verify: roadmap exists (16 DEs), binary absent, dist absent, SQLite temps not tracked, drafts not tracked, tag v1.0.0 exists, .gitignore updated.
  Output: `Roadmap [OK/MISSING] | Binary [ABSENT/PRESENT] | SQLite [CLEAN/DIRTY] | Drafts [CLEAN/DIRTY] | Tag [OK/MISSING] | Gitignore [OK/MISSING] | VERDICT`

- [ ] F3. **Edge Case Sweep** — `quick`
  Verify: main HEAD is merged (not e73d37c), go.sum clean, mesh process not running, mesh-design worktree unmodified, mesh-impl worktree clean.
  Output: `Main HEAD [OK/STALE] | go.sum [CLEAN/DRIFT] | Mesh daemon [NONE/RUNNING] | mesh-design [CLEAN/DIRTY] | mesh-impl [CLEAN/DIRTY] | VERDICT`

---

## Commit Strategy

- **10**: `chore: cleanup artifacts, update governance DB for v1.0 completion, add v1.1 roadmap` — All cleanup + governance + roadmap files

---

## Success Criteria

### Verification Commands
```bash
# Pre-merge (mesh-impl worktree)
go build ./... && go test ./... && go vet ./...
# Expected: all pass

# Post-merge (mesh worktree, on main)
go build ./... && go test ./... && go vet ./...
# Expected: all pass

# Governance DB state
python3 ~/.agents/skills/mesh-nav/scripts/session.py brief
# Expected: 4 sessions, S6 completed

python3 ~/.agents/skills/mesh-nav/scripts/gov.py list --type question
# Expected: Q1-Q5 all resolved

# Tag
git rev-parse v1.0.0
# Expected: main HEAD SHA
```

### Final Checklist
- [ ] All "Must Have" present
- [ ] All "Must NOT Have" absent
- [ ] Pre-merge build/test/vet passed
- [ ] Post-merge build/test/vet passed
- [ ] Governance DB accurate
- [ ] v1.0.0 tag correctly placed
- [ ] v1.1 roadmap documented
- [ ] Artifacts cleaned
