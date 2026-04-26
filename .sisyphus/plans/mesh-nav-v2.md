# mesh-nav v2 — Governance Layer Rebuild

## TL;DR

> **Quick Summary**: Rebuild mesh-nav from scratch using GrafitoDB (Cypher + SQLite property graph) with 7 node types, 10 typed edge types, session continuity (auto-briefing + structured handoff), learnings with quality gates, and a gstack-style single-skill protocol with independent layers.
>
> **Deliverables**:
> - GrafitoDB-backed governance graph with 7 node types + 10 edge types
> - Python scripts as API (gov.py, session.py, generate.py, learning.py)
> - Migration of 34 existing entities + 28 edges to new graph
> - Auto-briefing at session start, structured handoff at session end
> - Learnings as first-class graph nodes with What/Why/Where/Learned + confidence scoring
> - Rewritten SKILL.md (≤400 lines) with 7 independent layers
> - Test suite using unittest (stdlib)
>
> **Estimated Effort**: Large
> **Parallel Execution**: YES — 4 waves
> **Critical Path**: T1 (smoke test) → T3 (graph layer) → T4 (migration) → T5-T8 (features) → T9-T10 (skill + views) → FINAL

---

## Context

### Original Request
User wanted session continuity for mesh-nav — zero re-explanation between AI agent sessions. Three pains: cold start, lost thinking, no follow-through.

### Interview Summary
**Key Discussions**:
- 30+ existing systems evaluated across 5 categories — nobody has a self-contained SQLite-backed context graph for project governance
- "Context graphs" validated as industry trend (ElixirData, Zep/Graphiti 25K★, Neo4j Labs)
- GrafitoDB selected as graph engine despite 3 transitive deps (networkx, orjson, zstandard) — user accepted tradeoff
- 7 design decisions made part-by-part: graph storage, node model, edge model, session continuity, learnings, generated views, skill protocol
- Skill protocol modeled after gstack: one skill with independent layers, not a rigid phase pipeline

**Research Findings**:
- **Semantica** (1,089★): Best domain model (decisions as first-class objects, causal chains, temporal validity, precedent search) but requires Neo4j — we borrow design patterns
- **GrafitoDB**: SQLite-backed property graph with Cypher queries, BFS/DFS traversal. v0.1.2 (proof of concept). Accepts 3 deps.
- **Engram**: What/Why/Where/Learned format for learnings
- **agent-os**: 3-strike quality gate for preventing learning bloat
- **Codevira**: `catch_me_up` auto-briefing pattern
- **DevMemory**: `context_handoff` structured session end

### Metis Review
**Identified Gaps** (addressed):
- GrafitoDB dependency lie → User accepted the 3 deps
- `depends_on` edge type not in new model (6 edges) → Map to `constrains` during migration
- GrafitoDB v0.1.2 maturity risk → Task 1 is mandatory smoke test; graph.py abstraction layer makes backend swappable
- SKILL.md ≤400 lines with 7 layers is tight → Push detail to reference files
- Data migration required for 34 entities + 28 edges → Dedicated migration task with verification

---

## Work Objectives

### Core Objective
Rebuild mesh-nav as a GrafitoDB-backed governance graph with full session continuity, learnings, and 10 typed edge types — replacing the hand-rolled SQLite tables while preserving all existing data.

### Concrete Deliverables
- `.mesh/governance.db` rebuilt with GrafitoDB property graph
- `~/.agents/skills/mesh-nav/scripts/gov.py` — Entity/edge CRUD + graph traversal via Cypher
- `~/.agents/skills/mesh-nav/scripts/session.py` — Session lifecycle (brief, start, end, handoff)
- `~/.agents/skills/mesh-nav/scripts/learning.py` — Learning CRUD + quality gate
- `~/.agents/skills/mesh-nav/scripts/generate.py` — Markdown generation from graph
- `~/.agents/skills/mesh-nav/scripts/migrate.py` — One-time data migration (old → new)
- `~/.agents/skills/mesh-nav/scripts/graph.py` — Abstraction layer wrapping GrafitoDB
- `~/.agents/skills/mesh-nav/SKILL.md` — Rewritten with 7 independent layers (≤400 lines)
- `~/.agents/skills/mesh-nav/references/` — Updated reference files (each ≤200 lines)
- `tests/` — unittest suite for all scripts
- Updated `AGENTS.md` and `CONTEXT.md`

### Definition of Done
- [ ] GrafitoDB smoke test passes (all CRUD + Cypher + traversal)
- [ ] All 34 entities migrated and queryable via new gov.py
- [ ] All 28 edges migrated (depends_on → constrains mapping applied)
- [ ] `session.py brief` produces coherent auto-briefing
- [ ] `session.py end` with structured handoff captures reasoning/dead ends/surprises/deferred
- [ ] `learning.py add/list` works with What/Why/Where/Learned + confidence
- [ ] Generated markdown semantically matches existing output
- [ ] SKILL.md ≤400 lines, all reference files ≤200 lines
- [ ] All tests pass: `python3 -m unittest discover tests/ -v`

### Must Have
- GrafitoDB as graph engine (Cypher + SQLite)
- 7 node types: decision, constraint, persona, question, learning, session, gov_decision
- 10 edge types: enables, conflicts_with, blocks, supersedes, related_to, resolved_by, validates_for, constrains, produced, learned_from
- Auto-briefing at session start
- Structured handoff at session end (reasoning, dead ends, surprises, deferred, next steps)
- Learnings with What/Why/Where/Learned format + confidence field
- graph.py abstraction layer (GrafitoDB not called directly from scripts)
- Semantic IDs (D1, C3, etc.) stored as mesh_id property on nodes
- All 34 existing entities + 28 edges preserved after migration
- Tests for every script

### Must NOT Have (Guardrails)
- No raw Cypher in SKILL.md — all graph access through Python scripts
- No complex Cypher patterns — Python API for CRUD, Cypher only for traversal
- No more than 1 new generated view (learnings-md) beyond existing 4
- No learning deduplication, similarity scoring, or semantic search
- No intermediate session tracking — only start (brief) + end (handoff)
- No new edge types beyond the 10 decided — use related_to with properties
- No direct GrafitoDB imports in any script except graph.py
- No feature without a test
- No SKILL.md over 400 lines or reference file over 200 lines
- No AGENTS.md over 80 lines
- No git hooks
- No meta-governance — SKILL.md is the final layer (D-GOV6)

---

## Verification Strategy

> **ZERO HUMAN INTERVENTION** — ALL verification is agent-executed. No exceptions.

### Test Decision
- **Infrastructure exists**: YES (Python unittest in stdlib)
- **Automated tests**: YES (tests alongside implementation)
- **Framework**: unittest (stdlib — no pytest, respects dependency minimization)
- **Test location**: `~/.agents/skills/mesh-nav/tests/`

### QA Policy
Every task includes agent-executed QA scenarios.
Evidence saved to `.sisyphus/evidence/task-{N}-{scenario-slug}.{ext}`.

- **Scripts**: Use Bash — Run commands, assert exit code + output content
- **Graph operations**: Use Bash — Run gov.py commands, verify node/edge creation
- **Generated views**: Use Bash — Diff generated output against expected

---

## Execution Strategy

### Parallel Execution Waves

```
Wave 0 (GATE — must pass before anything else):
└── Task 1: GrafitoDB smoke test [quick]

Wave 1 (After T1 passes — foundation, PARALLEL):
├── Task 2: graph.py abstraction layer (depends: T1) [deep]
├── Task 3: gov.py rewrite — entity/edge CRUD + traversal (depends: T2) [deep]
├── Task 4: migrate.py — data migration from old DB (depends: T2) [deep]
└── Task 5: Test infrastructure setup (depends: T1) [quick]

Wave 2 (After Wave 1 — features, MAX PARALLEL):
├── Task 6: session.py rewrite — brief + start + end + handoff (depends: T2) [deep]
├── Task 7: learning.py — CRUD + quality gate (depends: T2) [deep]
├── Task 8: generate.py rewrite — expanded views (depends: T3) [unspecified-high]
└── Task 9: Tests for all scripts (depends: T3, T6, T7) [unspecified-high]

Wave 3 (After Wave 2 — integration):
├── Task 10: SKILL.md rewrite — 7 independent layers (depends: T3, T6, T7, T8) [writing]
├── Task 11: Reference files update (depends: T10) [writing]
└── Task 12: AGENTS.md + CONTEXT.md update (depends: T8, T10) [quick]

Wave FINAL (After ALL tasks — 4 parallel reviews):
├── F1: Plan compliance audit (oracle)
├── F2: Code quality review (unspecified-high)
├── F3: Integration QA (unspecified-high)
└── F4: Scope fidelity check (deep)
-> Present results -> Get explicit user okay

Critical Path: T1 → T2 → T3 → T8 → T10 → T11 → F1-F4
Parallel Speedup: ~50% faster than sequential
Max Concurrent: 4 (Waves 2-3)
```

### Dependency Matrix

| Task | Depends On | Blocks | Wave |
|------|-----------|--------|------|
| T1 | — | T2, T5 | 0 |
| T2 | T1 | T3, T4, T6, T7 | 1 |
| T3 | T2 | T8, T9, T10 | 1 |
| T4 | T2 | T12 | 1 |
| T5 | T1 | T9 | 1 |
| T6 | T2 | T9, T10 | 2 |
| T7 | T2 | T9, T10 | 2 |
| T8 | T3 | T10, T12 | 2 |
| T9 | T3, T6, T7 | F1-F4 | 2 |
| T10 | T3, T6, T7, T8 | T11, T12 | 3 |
| T11 | T10 | F1-F4 | 3 |
| T12 | T8, T10 | F1-F4 | 3 |

### Agent Dispatch Summary

- **Wave 0**: 1 — T1 → `quick`
- **Wave 1**: 3 — T2 → `deep`, T3 → `deep`, T4 → `deep` (T3 and T4 sequential after T2)
- **Wave 2**: 4 — T6 → `deep`, T7 → `deep`, T8 → `unspecified-high`, T9 → `unspecified-high`
- **Wave 3**: 3 — T10 → `writing`, T11 → `writing`, T12 → `quick`
- **FINAL**: 4 — F1 → `oracle`, F2 → `unspecified-high`, F3 → `unspecified-high`, F4 → `deep`

---

## TODOs

- [x] 1. **GrafitoDB Smoke Test** (GATE — blocks everything)

  **What to do**:
  - Install GrafitoDB: `pip install grafito`
  - Write `tests/smoke_grafito.py` that validates ALL operations we need:
    - Create nodes with labels (decision, constraint, persona, question, learning, session, gov_decision) and properties (mesh_id, title, status, confidence, layer, body, created_at, updated_at)
    - Create typed edges between nodes (all 10 types)
    - Query nodes by label: `MATCH (n:decision) RETURN n`
    - Query nodes by property: `MATCH (n:decision {mesh_id: 'D1'}) RETURN n`
    - Query edges by type: `MATCH (a)-[r:conflicts_with]->(b) RETURN a, b`
    - BFS traversal: follow edges from a node to depth N
    - Delete a node and verify edge cascade
    - Update node properties
  - Run on actual Python 3.11.11 on this machine
  - If ANY operation fails → STOP. Report failure. Plan must pivot to raw SQLite property graph layer.

  **Must NOT do**:
  - Do NOT proceed to any other task if smoke test fails
  - Do NOT skip any operation type — all must be validated

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO — this is a gate
  - **Parallel Group**: Wave 0 (solo)
  - **Blocks**: T2, T3, T4, T5, T6, T7, T8, T9, T10, T11, T12 (everything)
  - **Blocked By**: None

  **References**:
  **External References**:
  - GrafitoDB docs: https://github.com/jpmanson/GrafitoDB — API reference, Cypher support
  - GrafitoDB PyPI: https://pypi.org/project/grafito/ — Version info, dependency list

  **Acceptance Criteria**:
  - [ ] `python3 tests/smoke_grafito.py` exits with code 0 and prints "PASS"
  - [ ] All 7 node types created and queryable
  - [ ] All 10 edge types created and queryable
  - [ ] Cypher MATCH queries return correct results
  - [ ] BFS traversal follows edges to depth 2+
  - [ ] Node deletion cascades to edges

  **QA Scenarios**:
  ```
  Scenario: GrafitoDB full operation validation
    Tool: Bash
    Preconditions: `pip install grafito` succeeded
    Steps:
      1. `python3 tests/smoke_grafito.py`
      2. Assert exit code 0
      3. Assert output contains "PASS"
      4. Assert output does NOT contain "FAIL" or "ERROR"
    Expected Result: Script exits 0 with "PASS", all operations validated
    Failure Indicators: Non-zero exit code, any "FAIL" in output, missing operation validation
    Evidence: .sisyphus/evidence/task-1-smoke-test.txt

  Scenario: GrafitoDB actual dependency check
    Tool: Bash
    Preconditions: grafito installed
    Steps:
      1. `python3 -c "import grafito; print(grafito.__version__)"`
      2. `python3 -c "import networkx; print('networkx:', networkx.__version__)"`
      3. `python3 -c "import orjson; print('orjson: ok')"`
    Expected Result: All imports succeed, version printed
    Failure Indicators: ImportError on any module
    Evidence: .sisyphus/evidence/task-1-deps-check.txt
  ```

  **Commit**: YES
  - Message: `test(mesh-nav): add GrafitoDB smoke test`
  - Files: `tests/smoke_grafito.py`
  - Pre-commit: `python3 tests/smoke_grafito.py`

- [x] 2. **graph.py — GrafitoDB Abstraction Layer**

  **What to do**:
  - Create `~/.agents/skills/mesh-nav/scripts/graph.py` wrapping GrafitoDB
  - This is the ONLY file that imports from grafito. All other scripts import from graph.py.
  - Functions needed:
    - `get_graph(db_path)` — Returns GrafitoDB instance (file-based at `.mesh/governance.db`)
    - `add_node(graph, mesh_id, node_type, title, **properties)` — Create node with label + mesh_id property + all structured metadata (status, confidence, valid_from, valid_until, layer, body, created_at, updated_at)
    - `get_node(graph, mesh_id)` — Get node by mesh_id property (NOT GrafitoDB's integer ID)
    - `update_node(graph, mesh_id, **properties)` — Update node properties
    - `delete_node(graph, mesh_id)` — Delete node by mesh_id
    - `list_nodes(graph, node_type=None)` — List nodes, optionally filtered by label
    - `add_edge(graph, source_mesh_id, target_mesh_id, relation, **properties)` — Create typed edge
    - `get_edges(graph, mesh_id, direction='both', relation=None)` — Get edges for a node
    - `trace(graph, mesh_id, depth=2)` — BFS traversal following edges to depth N
    - `query(graph, cypher, params=None)` — Raw Cypher query (for generate.py only)
  - Node types as constants: `NODE_TYPES = ['decision', 'constraint', 'persona', 'question', 'learning', 'session', 'gov_decision']`
  - Edge types as constants: `EDGE_TYPES = ['enables', 'conflicts_with', 'blocks', 'supersedes', 'related_to', 'resolved_by', 'validates_for', 'constrains', 'produced', 'learned_from']`
  - Validate edge types on creation (reject unknown types)
  - All functions use mesh_id (D1, C3, etc.) as the primary identifier, NOT GrafitoDB's internal integer ID
  - Set `PRAGMA journal_mode=WAL` on the underlying SQLite connection

  **Must NOT do**:
  - Do NOT expose GrafitoDB's integer IDs to callers — mesh_id is the only identifier
  - Do NOT allow raw Cypher execution from outside graph.py (except generate.py via query())
  - Do NOT import grafito in any file other than graph.py

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO — foundation for everything else
  - **Parallel Group**: Wave 1 (sequential start)
  - **Blocks**: T3, T4, T6, T7
  - **Blocked By**: T1 (smoke test must pass)

  **References**:
  **Pattern References**:
  - `~/.agents/skills/mesh-nav/scripts/gov.py:1-60` — Current get_db() helper pattern, argparse structure, try/finally/conn.close pattern

  **External References**:
  - GrafitoDB API: https://github.com/jpmanson/GrafitoDB — GrafitoDatabase class, Cypher methods

  **Acceptance Criteria**:
  - [ ] `python3 -c "from graph import get_graph, add_node, get_node"` succeeds
  - [ ] No grafito imports in any file except graph.py
  - [ ] `add_node()` creates node with correct label and mesh_id property
  - [ ] `get_node('D1')` returns node regardless of GrafitoDB's internal ID
  - [ ] `trace('D1', depth=2)` follows edges correctly

  **QA Scenarios**:
  ```
  Scenario: Graph layer CRUD operations
    Tool: Bash
    Preconditions: GrafitoDB smoke test passed (T1)
    Steps:
      1. `python3 -c "from scripts.graph import get_graph, add_node, get_node, delete_node; g = get_graph(':memory:'); add_node(g, 'TEST1', 'decision', 'Test'); n = get_node(g, 'TEST1'); assert n is not None; print('CRUD OK')"`
      2. Assert output contains "CRUD OK"
    Expected Result: All CRUD operations work through abstraction layer
    Failure Indicators: ImportError, assertion failure, GrafitoDB errors
    Evidence: .sisyphus/evidence/task-2-graph-crud.txt

  Scenario: Edge type validation
    Tool: Bash
    Preconditions: graph.py exists
    Steps:
      1. `python3 -c "from scripts.graph import add_edge, EDGE_TYPES; assert 'conflicts_with' in EDGE_TYPES; assert len(EDGE_TYPES) == 10; print('Edge types OK')"`
    Expected Result: All 10 edge types defined, accessible
    Evidence: .sisyphus/evidence/task-2-edge-types.txt
  ```

  **Commit**: YES
  - Message: `feat(mesh-nav): add graph.py abstraction layer`
  - Files: `~/.agents/skills/mesh-nav/scripts/graph.py`
  - Pre-commit: `python3 -c "from scripts.graph import get_graph"`

- [x] 3. **gov.py Rewrite — Entity/Edge CRUD + Traversal**

  **What to do**:
  - Rewrite `~/.agents/skills/mesh-nav/scripts/gov.py` to use graph.py instead of raw SQLite
  - Keep the same CLI interface (argparse subcommands) so AGENTS.md references still work
  - Subcommands: `add_entity`, `get`, `update`, `list`, `add_edge`, `trace`, `conflicts`, `blocked_by`
  - `add_entity <MESH_ID> <TYPE> "<TITLE>" --status <status> --body "<text>" --confidence <float> --layer <1-7> --valid-from <date> --valid-until <date>`
  - `get <MESH_ID>` — Print all node properties (mesh_id, type, title, status, confidence, layer, body, created_at, updated_at)
  - `update <MESH_ID> --status <new> --body <new>` — Update properties
  - `list --type <type> --layer <N>` — List nodes filtered by type and/or layer
  - `add_edge <SRC> <TGT> <RELATION>` — Create typed edge (validates relation is one of 10 types)
  - `trace <MESH_ID> --depth <N>` — BFS traversal, prints each hop: "D1 --enables--> D2"
  - `conflicts <MESH_ID>` — Find all nodes that conflict_with the given node (bidirectional)
  - `blocked_by <MESH_ID>` — Find all nodes that block the given node
  - All subcommands accept `--db <path>` (default `.mesh/governance.db`)

  **Must NOT do**:
  - Do NOT import grafito directly — use graph.py
  - Do NOT change the CLI interface names (add_entity, get, update, list, etc.) — AGENTS.md references them
  - Do NOT exceed ~350 lines for this file

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (depends on T2)
  - **Parallel Group**: Wave 1 (after T2)
  - **Blocks**: T8, T9, T10
  - **Blocked By**: T2

  **References**:
  **Pattern References**:
  - `~/.agents/skills/mesh-nav/scripts/gov.py:1-311` — Current CLI structure, argparse subcommands, output formatting
  - `~/.agents/skills/mesh-nav/scripts/gov.py:96-150` — cmd_list() and cmd_trace() patterns to follow

  **Acceptance Criteria**:
  - [ ] `python3 scripts/gov.py add_entity TEST1 decision "Test decision" --status accepted` creates node
  - [ ] `python3 scripts/gov.py get TEST1` prints node details
  - [ ] `python3 scripts/gov.py list --type decision` lists decision nodes
  - [ ] `python3 scripts/gov.py add_edge TEST1 D1 enables` creates edge
  - [ ] `python3 scripts/gov.py trace TEST1 --depth 2` shows connected nodes
  - [ ] `python3 scripts/gov.py conflicts D9` shows D3 (known conflict)

  **QA Scenarios**:
  ```
  Scenario: Entity CRUD round-trip
    Tool: Bash
    Steps:
      1. `python3 scripts/gov.py add_entity QA1 learning "QA test learning" --status accepted --confidence 3 --layer 2`
      2. `python3 scripts/gov.py get QA1` — assert title matches
      3. `python3 scripts/gov.py update QA1 --status deprecated`
      4. `python3 scripts/gov.py get QA1` — assert status is "deprecated"
    Expected Result: Full CRUD cycle works
    Evidence: .sisyphus/evidence/task-3-gov-crud.txt

  Scenario: Edge type rejection
    Tool: Bash
    Steps:
      1. `python3 scripts/gov.py add_edge D1 D2 invalid_relation` — assert exit code non-zero
      2. Assert stderr contains error about invalid relation type
    Expected Result: Invalid edge types are rejected
    Evidence: .sisyphus/evidence/task-3-edge-rejection.txt
  ```

  **Commit**: YES
  - Message: `feat(mesh-nav): rewrite gov.py with GrafitoDB backend`
  - Files: `~/.agents/skills/mesh-nav/scripts/gov.py`

- [x] 4. **migrate.py — Data Migration from Old DB**

  **What to do**:
  - Create `~/.agents/skills/mesh-nav/scripts/migrate.py` — one-time migration script
  - Reads old `.mesh/governance.db` (raw SQLite: entities + edges + sessions tables)
  - Writes to new GrafitoDB-backed `.mesh/governance.db` (or a fresh file)
  - Entity migration: 34 entities across 5 types → GrafitoDB nodes with labels + properties
    - Map `type` → GrafitoDB label
    - Map `id` → `mesh_id` property
    - Map `title`, `status`, `body`, `properties`, `created_at`, `updated_at` → node properties
    - Add default values for new properties: `confidence=5`, `layer` inferred from content, `valid_from=created_at`, `valid_until=null`
  - Edge migration: 28 edges → GrafitoDB typed edges
    - Map `depends_on` → `constrains` (6 edges: C1→D3, C2→D3, C5→D1, C5→D2, C5→D4, C6→D6)
    - Keep `enables` (12 edges), `related_to` (10 edges) as-is
  - Session migration: 4 sessions → GrafitoDB session nodes with `produced` edges to decisions
  - Verification: After migration, print counts per type and compare with old DB
  - Idempotent: detect if migration already ran (check for mesh_id property on nodes)

  **Must NOT do**:
  - Do NOT delete the old DB file — rename to `.mesh/governance.db.v1.bak`
  - Do NOT proceed if entity count doesn't match 34 after migration

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with T3, after T2)
  - **Parallel Group**: Wave 1
  - **Blocks**: T12 (verification)
  - **Blocked By**: T2

  **References**:
  **API References**:
  - `.mesh/governance.db` — Current schema: `CREATE TABLE entities (id TEXT PRIMARY KEY, type TEXT, title TEXT, status TEXT, body TEXT, properties TEXT, created_at TEXT, updated_at TEXT)`
  - `.mesh/governance.db` — Edge schema: `CREATE TABLE edges (id INTEGER PK, source_id TEXT, target_id TEXT, relation TEXT, properties TEXT)`

  **Acceptance Criteria**:
  - [ ] `python3 scripts/migrate.py --old .mesh/governance.db.v1.bak --new .mesh/governance.db` exits 0
  - [ ] Output shows: "Migrated 34 entities, 28 edges, 4 sessions"
  - [ ] `python3 scripts/gov.py list --type decision | wc -l` returns 10
  - [ ] `python3 scripts/gov.py list --type constraint | wc -l` returns 6
  - [ ] `python3 scripts/gov.py list --type gov_decision | wc -l` returns 8
  - [ ] `python3 scripts/gov.py list --type persona | wc -l` returns 5
  - [ ] `python3 scripts/gov.py list --type question | wc -l` returns 5
  - [ ] Old DB backed up at `.mesh/governance.db.v1.bak`

  **QA Scenarios**:
  ```
  Scenario: Full migration with count verification
    Tool: Bash
    Preconditions: Old governance.db exists with 34 entities
    Steps:
      1. `cp .mesh/governance.db .mesh/governance.db.pre-migration`
      2. `python3 scripts/migrate.py --db .mesh/governance.db`
      3. Verify output contains "Migrated 34 entities"
      4. `python3 scripts/gov.py list --type decision` — count 10 lines
      5. `python3 scripts/gov.py list --type constraint` — count 6 lines
    Expected Result: All entities migrated, counts match
    Evidence: .sisyphus/evidence/task-4-migration.txt

  Scenario: Edge type mapping verification
    Tool: Bash
    Steps:
      1. After migration, query all edges where relation is 'depends_on'
      2. Assert zero results (all mapped to 'constrains')
      3. Query edges with relation 'constrains'
      4. Assert 6 results (C1→D3, C2→D3, C5→D1, C5→D2, C5→D4, C6→D6)
    Expected Result: depends_on fully mapped to constrains
    Evidence: .sisyphus/evidence/task-4-edge-mapping.txt
  ```

  **Commit**: YES
  - Message: `feat(mesh-nav): add data migration script`
  - Files: `~/.agents/skills/mesh-nav/scripts/migrate.py`

- [x] 5. **Test Infrastructure Setup**

  **What to do**:
  - Create test directory structure at `~/.agents/skills/mesh-nav/tests/`
  - Create `tests/__init__.py` (empty)
  - Create `tests/helpers.py` with shared test utilities:
    - `create_test_graph()` — Returns GrafitoDB instance with in-memory DB and sample data (D1-D5, C1-C3, a few edges)
    - `assert_node_exists(graph, mesh_id, expected_type)` — Assertion helper
    - `assert_edge_exists(graph, src, tgt, relation)` — Assertion helper
  - Create `tests/test_graph.py` — Tests for graph.py abstraction layer:
    - test_create_node, test_get_node, test_update_node, test_delete_node
    - test_create_edge, test_get_edges, test_trace
    - test_invalid_edge_type_rejected
    - test_mesh_id_lookup (verify mesh_id property works as primary identifier)
  - All tests use `unittest.TestCase` (stdlib)
  - All tests use in-memory GrafitoDB (`:memory:`) for speed

  **Must NOT do**:
  - Do NOT use pytest — unittest only (stdlib constraint)
  - Do NOT test against file-based DB — in-memory only for unit tests

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with T3, T4 after T2)
  - **Parallel Group**: Wave 1
  - **Blocks**: T9
  - **Blocked By**: T1 (smoke test must pass)

  **References**:
  **Pattern References**:
  - `tests/smoke_grafito.py` (from T1) — GrafitoDB usage patterns, assertion style

  **Acceptance Criteria**:
  - [ ] `python3 -m unittest tests/test_graph.py -v` passes all tests
  - [ ] At least 8 test methods in test_graph.py

  **QA Scenarios**:
  ```
  Scenario: Test suite runs and passes
    Tool: Bash
    Steps:
      1. `python3 -m unittest discover tests/ -v`
      2. Assert exit code 0
      3. Assert output contains "OK" or "Ran N tests"
    Expected Result: All graph.py tests pass
    Evidence: .sisyphus/evidence/task-5-test-infra.txt
  ```

  **Commit**: YES
  - Message: `test(mesh-nav): add test infrastructure and graph.py tests`
  - Files: `tests/__init__.py`, `tests/helpers.py`, `tests/test_graph.py`

- [x] 6. **session.py Rewrite — Brief + Start + End + Handoff**

  **What to do**:
  - Rewrite `~/.agents/skills/mesh-nav/scripts/session.py` to use graph.py
  - Session nodes in the graph with label `session`, mesh_id like `S1`, `S2`, etc.
  - Subcommands:
    - `start --date YYYY-MM-DD --type <design|implement|review|explore>` — Create session node with status=active
    - `end --id <N> --summary "<text>" --reasoning "<why decisions were made>" --dead-ends '<JSON>' --surprises "<text>" --deferred "<text>" --blocked-on "<text>" --next-steps "<text>" --decisions '<JSON>' --files-created '<JSON>' --files-modified '<JSON>'` — Update session node with all handoff fields
    - `latest` — Print most recent session with full handoff data
    - `list --limit <N>` — List recent sessions (tabular)
    - `brief` — **Auto-briefing**: synthesizes graph state into a coherent brief
  - **Brief algorithm** (the core value):
    1. Get latest session node → print summary + next_steps
    2. Count entities by type → "10 decisions, 6 constraints, 5 personas, 5 questions, N learnings"
    3. List active blockers → query nodes with edges `blocks` pointing to them
    4. List new/changed entities since last session → nodes with updated_at > last session date
    5. Print current project phase and focus from CONTEXT.md (read file)
    6. Format as readable markdown output
  - Session properties: mesh_id, date, type, status (active/closed), summary, focus, reasoning, dead_ends (JSON array of {approach, why_failed, alternative}), surprises, deferred, blocked_on, next_steps, decisions_made (JSON), files_created (JSON), files_modified (JSON), created_at, updated_at

  **Must NOT do**:
  - Do NOT make briefing complex analytics — last session + counts + blockers + recent changes
  - Do NOT add intermediate session tracking — only start and end
  - Do NOT import grafito directly — use graph.py

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with T7, T8, T9 after T2)
  - **Parallel Group**: Wave 2
  - **Blocks**: T9, T10
  - **Blocked By**: T2

  **References**:
  **Pattern References**:
  - `~/.agents/skills/mesh-nav/scripts/session.py:1-132` — Current CLI structure, argparse pattern

  **Acceptance Criteria**:
  - [ ] `python3 scripts/session.py start --date 2026-04-25 --type design` creates session node
  - [ ] `python3 scripts/session.py brief` produces non-empty markdown output
  - [ ] `python3 scripts/session.py end --id <N> --summary "test" --reasoning "because X" --dead-ends '[{"approach":"Y","why_failed":"Z"}]' --next-steps "do A"` updates session
  - [ ] `python3 scripts/session.py latest` shows full handoff data

  **QA Scenarios**:
  ```
  Scenario: Full session lifecycle
    Tool: Bash
    Steps:
      1. `python3 scripts/session.py start --date 2026-04-25 --type design` — capture session ID
      2. `python3 scripts/session.py brief` — assert contains "10 decisions" and entity counts
      3. `python3 scripts/session.py end --id <ID> --summary "tested session lifecycle" --reasoning "validating the flow" --next-steps "continue testing"`
      4. `python3 scripts/session.py latest` — assert shows "tested session lifecycle" and reasoning
    Expected Result: Full start → brief → end → latest cycle works
    Evidence: .sisyphus/evidence/task-6-session-lifecycle.txt

  Scenario: Brief without prior session
    Tool: Bash
    Steps:
      1. Create fresh in-memory graph
      2. `python3 scripts/session.py brief` — assert still produces output (entity counts, no "last session" section)
    Expected Result: Brief works even without prior session (graceful degradation)
    Evidence: .sisyphus/evidence/task-6-brief-no-session.txt
  ```

  **Commit**: YES
  - Message: `feat(mesh-nav): rewrite session.py with auto-briefing and structured handoff`
  - Files: `~/.agents/skills/mesh-nav/scripts/session.py`

- [x] 7. **learning.py — CRUD + Quality Gate**

  **What to do**:
  - Create `~/.agents/skills/mesh-nav/scripts/learning.py`
  - Learning nodes in the graph with label `learning`, mesh_id like `L1`, `L2`, etc.
  - Properties: mesh_id, what (what was learned), why (context), where_ref (file/code reference), learned (the actual lesson), category (tool/project/pattern/trap), confidence (1-5), strike_count (times encountered), status (active/deprecated), created_at, updated_at
  - Subcommands:
    - `add --what "<text>" --why "<text>" --where "<ref>" --learned "<text>" --category <tool|project|pattern|trap> --confidence <1-5>` — Create learning node
    - `get <MESH_ID>` — Print learning details
    - `list --category <type> --min-confidence <N>` — List learnings with filters
    - `encounter <MESH_ID>` — Increment strike_count (quality gate: only persist after 3+ strikes)
    - `deprecate <MESH_ID>` — Mark learning as deprecated (supersedes edge to replacement if provided)
    - `link <MESH_ID> --decision <D_ID> --session <S_ID>` — Create learned_from edge to decision/session
  - Quality gate logic: When adding a learning, if confidence < 3, print warning "Low confidence learning. Consider accumulating more encounters before persisting." (Soft gate, not a hard reject — the agent decides)

  **Must NOT do**:
  - Do NOT implement deduplication or similarity scoring
  - Do NOT implement semantic search or FTS5
  - Do NOT auto-extract learnings from sessions — manual capture only

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with T6, T8, T9 after T2)
  - **Parallel Group**: Wave 2
  - **Blocks**: T9, T10
  - **Blocked By**: T2

  **References**:
  **External References**:
  - Engram format: What/Why/Where/Learned — the 4-field structured format for learnings

  **Acceptance Criteria**:
  - [ ] `python3 scripts/learning.py add --what "db flag position" --why "CLI parsing" --where "session.py" --learned "--db must come before subcommand" --category tool --confidence 4` creates learning
  - [ ] `python3 scripts/learning.py list --category tool` lists tool learnings
  - [ ] `python3 scripts/learning.py encounter L1` increments strike_count
  - [ ] `python3 scripts/learning.py link L1 --decision D4` creates learned_from edge

  **QA Scenarios**:
  ```
  Scenario: Learning CRUD round-trip
    Tool: Bash
    Steps:
      1. `python3 scripts/learning.py add --what "test" --why "QA" --where "test" --learned "works" --category pattern --confidence 3`
      2. Capture the mesh_id from output (e.g., L1)
      3. `python3 scripts/learning.py get L1` — assert all 4 fields present
      4. `python3 scripts/learning.py encounter L1`
      5. `python3 scripts/learning.py get L1` — assert strike_count is 2
      6. `python3 scripts/learning.py deprecate L1`
      7. `python3 scripts/learning.py get L1` — assert status is "deprecated"
    Expected Result: Full learning lifecycle works
    Evidence: .sisyphus/evidence/task-7-learning-crud.txt

  Scenario: Low confidence warning
    Tool: Bash
    Steps:
      1. `python3 scripts/learning.py add --what "vague" --why "uncertain" --where "nowhere" --learned "maybe" --category trap --confidence 1`
      2. Assert output contains "Low confidence" warning
    Expected Result: Warning printed but learning still created
    Evidence: .sisyphus/evidence/task-7-confidence-warning.txt
  ```

  **Commit**: YES
  - Message: `feat(mesh-nav): add learning.py with quality-gated CRUD`
  - Files: `~/.agents/skills/mesh-nav/scripts/learning.py`

- [x] 8. **generate.py Rewrite — Expanded Views**

  **What to do**:
  - Rewrite `~/.agents/skills/mesh-nav/scripts/generate.py` to use graph.py
  - Keep existing subcommands: `decisions-md`, `governance-md`, `questions-md`, `context-summary`
  - Add ONE new subcommand: `learnings-md` — Generate markdown list of learnings grouped by category
  - Each generated view queries the graph and produces markdown matching the current format
  - `decisions-md` must produce semantically identical output to current `discovery/state/decisions.md`
  - `context-summary` must produce entity summaries compatible with CONTEXT.md format
  - All subcommands accept `--db <path>`

  **Must NOT do**:
  - Do NOT add more than 1 new view (learnings-md)
  - Do NOT change the markdown format of existing views — must be semantically identical
  - Do NOT import grafito directly — use graph.py query() for Cypher

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (after T3)
  - **Parallel Group**: Wave 2
  - **Blocks**: T10, T12
  - **Blocked By**: T3

  **References**:
  **Pattern References**:
  - `~/.agents/skills/mesh-nav/scripts/generate.py:1-131` — Current generate pattern, _entity_section function, markdown formatting

  **Acceptance Criteria**:
  - [ ] `python3 scripts/generate.py decisions-md` produces valid markdown with all 10 decisions
  - [ ] `python3 scripts/generate.py governance-md` produces D-GOV1-8 markdown
  - [ ] `python3 scripts/generate.py questions-md` produces Q1-Q5 markdown
  - [ ] `python3 scripts/generate.py context-summary` produces entity summaries
  - [ ] `python3 scripts/generate.py learnings-md` produces learning list (may be empty if no learnings yet)

  **QA Scenarios**:
  ```
  Scenario: Decision markdown semantic equivalence
    Tool: Bash
    Preconditions: Migration complete (T4)
    Steps:
      1. `python3 scripts/generate.py decisions-md > /tmp/new-decisions.md`
      2. Verify /tmp/new-decisions.md contains all 10 decision IDs (D1-D10)
      3. Verify each decision has title, status, and body
    Expected Result: All decisions present with correct content
    Evidence: .sisyphus/evidence/task-8-decisions-md.txt
  ```

  **Commit**: YES
  - Message: `feat(mesh-nav): rewrite generate.py with learnings view`
  - Files: `~/.agents/skills/mesh-nav/scripts/generate.py`

- [x] 9. **Tests for All Scripts**

  **What to do**:
  - Create test files for each script:
    - `tests/test_gov.py` — Tests for gov.py: add_entity, get, update, list, add_edge, trace, conflicts, blocked_by
    - `tests/test_session.py` — Tests for session.py: start, end, latest, list, brief
    - `tests/test_learning.py` — Tests for learning.py: add, get, list, encounter, deprecate, link
    - `tests/test_generate.py` — Tests for generate.py: decisions-md, governance-md, questions-md, context-summary, learnings-md
  - Each test uses `helpers.create_test_graph()` for setup
  - All tests use `unittest.TestCase`
  - Minimum coverage: each subcommand has at least 1 test (happy path) + edge case tests where critical

  **Must NOT do**:
  - Do NOT test against file-based DB — in-memory only
  - Do NOT use pytest

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO — depends on T3, T6, T7 being complete
  - **Parallel Group**: Wave 2 (last in wave)
  - **Blocks**: F1-F4
  - **Blocked By**: T3, T6, T7

  **References**:
  **Pattern References**:
  - `tests/test_graph.py` (from T5) — Test style, assertion patterns, helper usage

  **Acceptance Criteria**:
  - [ ] `python3 -m unittest discover tests/ -v` passes ALL tests (0 failures)
  - [ ] At least 30 test methods total across all test files
  - [ ] Each gov.py subcommand has at least 1 test

  **QA Scenarios**:
  ```
  Scenario: Full test suite runs green
    Tool: Bash
    Steps:
      1. `python3 -m unittest discover tests/ -v`
      2. Assert exit code 0
      3. Assert output contains "OK" and test count ≥ 30
    Expected Result: All tests pass
    Evidence: .sisyphus/evidence/task-9-full-test-suite.txt
  ```

  **Commit**: YES
  - Message: `test(mesh-nav): add tests for gov, session, learning, generate`
  - Files: `tests/test_gov.py`, `tests/test_session.py`, `tests/test_learning.py`, `tests/test_generate.py`

- [x] 10. **SKILL.md Rewrite — 7 Independent Layers**

  **What to do**:
  - Rewrite `~/.agents/skills/mesh-nav/SKILL.md` from scratch (gstack-style, not rigid phase pipeline)
  - 7 independent layers — agent picks what it needs:
    1. **Brief** — Session start context. Trigger: "starting a session". Steps: `session.py brief`. Output: current state briefing.
    2. **Decide** — Record a decision. Trigger: "making a design/architecture decision". Steps: check conflicts → add entity → add edges → regenerate markdown.
    3. **Learn** — Capture operational knowledge. Trigger: "discovered something worth remembering". Steps: `learning.py add` → `learning.py link` to relevant decision/session.
    4. **Research** — Record findings. Trigger: "researching a topic". Steps: read existing → add findings → update questions if resolved.
    5. **Review** — Check existing state. Trigger: "reviewing decisions/architecture". Steps: `gov.py list` → `gov.py trace` → `gov.py conflicts` → validate against personas.
    6. **Handoff** — Session end capture. Trigger: "ending a session". Steps: `session.py end` with all fields → `generate.py` if changed → update CONTEXT.md.
    7. **Generate** — Regenerate views. Trigger: "after any entity mutation". Steps: run relevant generate.py subcommand.
  - Each layer: trigger condition + steps + output format (keep concise)
  - Include frontmatter (name, description), detection rule, quick reference table
  - Include guardrails section
  - Total: ≤400 lines (tight — push detail to reference files)

  **Must NOT do**:
  - Do NOT exceed 400 lines
  - Do NOT make layers sequential — each is independent
  - Do NOT include raw Cypher — all access through Python scripts
  - Do NOT remove the detection rule or guardrails

  **Recommended Agent Profile**:
  - **Category**: `writing`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO — depends on T3, T6, T7, T8
  - **Parallel Group**: Wave 3
  - **Blocks**: T11, T12
  - **Blocked By**: T3, T6, T7, T8

  **References**:
  **Pattern References**:
  - `~/.agents/skills/mesh-nav/SKILL.md:1-267` — Current SKILL.md structure (frontmatter, detection, phases, guardrails)
  - `~/.claude/skills/gstack/.gbrain/skills/gstack-context-save/SKILL.md` — Gstack skill style (independent capabilities, not phases)

  **Acceptance Criteria**:
  - [ ] `wc -l ~/.agents/skills/mesh-nav/SKILL.md` ≤ 400
  - [ ] All 7 layers documented with trigger + steps
  - [ ] Frontmatter with name + description present
  - [ ] Detection rule present ("If discovery/ directory does not exist...")
  - [ ] Guardrails section present
  - [ ] No raw Cypher in the file

  **QA Scenarios**:
  ```
  Scenario: SKILL.md structure validation
    Tool: Bash
    Steps:
      1. `wc -l ~/.agents/skills/mesh-nav/SKILL.md` — assert ≤ 400
      2. `grep -c "## " ~/.agents/skills/mesh-nav/SKILL.md` — assert ≥ 7 (one section per layer)
      3. `grep "MATCH" ~/.agents/skills/mesh-nav/SKILL.md` — assert no results (no raw Cypher)
      4. `grep "grafito" ~/.agents/skills/mesh-nav/SKILL.md` — assert no results (no GrafitoDB refs)
    Expected Result: Structure valid, no forbidden patterns
    Evidence: .sisyphus/evidence/task-10-skill-validation.txt
  ```

  **Commit**: YES
  - Message: `docs(mesh-nav): rewrite SKILL.md with 7 independent layers`
  - Files: `~/.agents/skills/mesh-nav/SKILL.md`

- [x] 11. **Reference Files Update**

  **What to do**:
  - Update all reference files in `~/.agents/skills/mesh-nav/references/`:
    - `reading-map.md` — Update to reflect new commands (add session.py brief, learning.py, etc.)
    - `update-protocols.md` — Update post-task checklists for new system
    - `folder-structure.md` — Update with new files (graph.py, learning.py, migrate.py, tests/)
  - Each file: ≤200 lines
  - Remove references to old raw SQL patterns
  - Add references to new GrafitoDB-backed commands

  **Must NOT do**:
  - Do NOT exceed 200 lines per file
  - Do NOT include GrafitoDB-specific documentation — keep it script-focused

  **Recommended Agent Profile**:
  - **Category**: `writing`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (depends on T10)
  - **Parallel Group**: Wave 3
  - **Blocks**: F1-F4
  - **Blocked By**: T10

  **References**:
  **Pattern References**:
  - `~/.agents/skills/mesh-nav/references/reading-map.md` — Current reading map (139 lines)
  - `~/.agents/skills/mesh-nav/references/update-protocols.md` — Current update protocols (123 lines)
  - `~/.agents/skills/mesh-nav/references/folder-structure.md` — Current folder structure (78 lines)

  **Acceptance Criteria**:
  - [ ] Each reference file ≤200 lines
  - [ ] reading-map.md includes session.py brief and learning.py commands
  - [ ] No references to raw SQL patterns

  **Commit**: YES
  - Message: `docs(mesh-nav): update reference files for v2`
  - Files: `~/.agents/skills/mesh-nav/references/*.md`

- [x] 12. **AGENTS.md + CONTEXT.md Update**

  **What to do**:
  - Update `AGENTS.md` to reflect new commands and system:
    - Update Quick Commands section with new CLI commands (session.py brief, learning.py add, etc.)
    - Keep ≤80 lines
  - Update `CONTEXT.md`:
    - Run `generate.py context-summary` and update the decision table
    - Update "Current Focus" to reflect mesh-nav v2
    - Keep existing format

  **Must NOT do**:
  - Do NOT exceed 80 lines for AGENTS.md
  - Do NOT hand-edit CONTEXT.md decision table — regenerate from DB

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (depends on T8, T10)
  - **Parallel Group**: Wave 3 (last)
  - **Blocks**: F1-F4
  - **Blocked By**: T8, T10

  **Acceptance Criteria**:
  - [ ] `wc -l AGENTS.md` ≤ 80
  - [ ] AGENTS.md references `session.py brief`, `learning.py add`
  - [ ] CONTEXT.md decision table matches generate.py output

  **Commit**: YES
  - Message: `docs(mesh): update AGENTS.md and CONTEXT.md for mesh-nav v2`
  - Files: `AGENTS.md`, `CONTEXT.md`

---

## Final Verification Wave (MANDATORY — after ALL implementation tasks)

> 4 review agents run in PARALLEL. ALL must APPROVE. Present consolidated results to user and get explicit "okay" before completing.
>
> **Do NOT auto-proceed after verification. Wait for user's explicit approval before marking work complete.**

- [x] F1. **Plan Compliance Audit** — `oracle`
  Read the plan end-to-end. For each "Must Have": verify implementation exists (read file, run command). For each "Must NOT Have": search codebase for forbidden patterns — reject with file:line if found. Check evidence files exist in .sisyphus/evidence/. Compare deliverables against plan.
  Output: `Must Have [N/N] | Must NOT Have [N/N] | Tasks [N/N] | VERDICT: APPROVE/REJECT`

- [x] F2. **Code Quality Review** — `unspecified-high`
  Run `python3 -m unittest discover tests/ -v` + `python3 -c "from grafito import GrafitoDB"`. Review all changed files for: bare except, print in prod, commented-out code, unused imports. Check for direct GrafitoDB imports outside graph.py. Verify graph.py abstraction is used everywhere.
  Output: `Tests [N pass/N fail] | Import check [PASS/FAIL] | Abstraction [CLEAN/N violations] | VERDICT`

- [x] F3. **Real Integration QA** — `unspecified-high`
  Start from clean state. Execute EVERY QA scenario from EVERY task — follow exact steps, capture evidence. Test cross-task integration: brief → decide → learn → handoff → brief again. Verify migration completeness: count all 34 entities + 28 edges in new DB. Save to `.sisyphus/evidence/final-qa/`.
  Output: `Scenarios [N/N pass] | Migration [34/34 entities, 28/28 edges] | Integration [N/N] | VERDICT`

- [x] F4. **Scope Fidelity Check** — `deep`
  For each task: read "What to do", read actual diff (git log/diff). Verify 1:1 — everything in spec was built (no missing), nothing beyond spec was built (no creep). Check "Must NOT do" compliance. Detect cross-task contamination. Flag unaccounted changes.
  Output: `Tasks [N/N compliant] | Contamination [CLEAN/N issues] | Unaccounted [CLEAN/N files] | VERDICT`

---

## Commit Strategy

- **T1**: `test(mesh-nav): add GrafitoDB smoke test` — tests/smoke_grafito.py
- **T2**: `feat(mesh-nav): add graph.py abstraction layer` — scripts/graph.py
- **T3**: `feat(mesh-nav): rewrite gov.py with GrafitoDB` — scripts/gov.py
- **T4**: `feat(mesh-nav): add data migration script` — scripts/migrate.py
- **T5**: `test(mesh-nav): add test infrastructure` — tests/__init__.py, tests/conftest.py, tests/test_graph.py
- **T6**: `feat(mesh-nav): rewrite session.py with brief + handoff` — scripts/session.py
- **T7**: `feat(mesh-nav): add learning.py with quality gate` — scripts/learning.py
- **T8**: `feat(mesh-nav): rewrite generate.py with expanded views` — scripts/generate.py
- **T9**: `test(mesh-nav): add tests for all scripts` — tests/test_gov.py, tests/test_session.py, tests/test_learning.py, tests/test_generate.py
- **T10**: `docs(mesh-nav): rewrite SKILL.md with 7 independent layers` — SKILL.md
- **T11**: `docs(mesh-nav): update reference files` — references/*.md
- **T12**: `docs(mesh): update AGENTS.md and CONTEXT.md` — AGENTS.md, CONTEXT.md

---

## Success Criteria

### Verification Commands
```bash
python3 tests/smoke_grafito.py                    # Expected: PASS
python3 -m unittest discover tests/ -v             # Expected: all tests green
python3 scripts/gov.py list --type decision         # Expected: 10 results
python3 scripts/gov.py list --type constraint       # Expected: 6 results
python3 scripts/gov.py list --type gov_decision     # Expected: 8 results
python3 scripts/gov.py list --type persona          # Expected: 5 results
python3 scripts/gov.py list --type question         # Expected: 5 results
python3 scripts/session.py brief                    # Expected: non-empty briefing
python3 scripts/learning.py list                    # Expected: 0 results (new type)
wc -l ~/.agents/skills/mesh-nav/SKILL.md           # Expected: ≤400
```

### Final Checklist
- [ ] All "Must Have" present
- [ ] All "Must NOT Have" absent
- [ ] All tests pass
- [ ] 34 entities + 28 edges migrated and queryable
- [ ] Auto-briefing produces coherent output
- [ ] Structured handoff captures all fields
- [ ] SKILL.md ≤400 lines
- [ ] All reference files ≤200 lines
- [ ] AGENTS.md ≤80 lines
- [ ] No direct GrafitoDB imports outside graph.py
