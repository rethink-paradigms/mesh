# Retrieval System for mesh-nav

## TL;DR

> **Quick Summary**: Activate GrafitoDB's built-in FTS5 search + NetworkX export, extend graph traversal in graph.py/gov.py, add a session query script for opencode.db, and redesign SKILL.md for the new day-session/task-session interaction model. No new dependencies. No vectors. Graph-first retrieval.
> 
> **Deliverables**:
> - FTS5 text search activated on governance graph (`gov.py search "MCP"`)
> - Bidirectional graph traversal with multi-hop queries
> - NetworkX algorithm layer (PageRank, community detection, centrality)
> - OpenCode session query script (`sessions.py search "governance"`)
> - SKILL.md redesigned for retrieval-first interaction (≤400 lines)
> - Result formatting optimized for LLM context injection
> - All changes tested, all 105 existing tests still passing
> 
> **Estimated Effort**: Medium (7 tasks, ~1-2 weeks)
> **Parallel Execution**: YES - 3 waves + final verification
> **Critical Path**: Task 1 (validate) → Task 2 (FTS5) → Task 3 (traversal) → Task 6 (SKILL.md)

---

## Context

### Original Request
Build a comprehensive retrieval system for mesh-nav that makes the governance graph queryable without reading entire files into context. The user wants graph-first retrieval (no vectors, no embeddings, no black boxes), FTS5 as a keyword supplement, and a redesigned SKILL.md that solves the context bloat problem.

### Interview Summary
**Key Discussions**:
- Context bloat: SKILL.md double-injection + full briefing = 65% context before work
- Retrieval over compression: "We need to improve retrieval, not compression"
- Vectors rejected: "If you can't make a mental map of how things work, it's a black box"
- GraphQLite evaluated: Cypher + 18 algorithms over SQLite, but incompatible schema (EAV vs JSON blob)
- Graphify evaluated: Confidence tagging (EXTRACTED/INFERRED/AMBIGUOUS), community detection, surprise scoring

**Research Findings**:
- Graph + FTS5 beats vectors for governance queries: 98% vs 71% accuracy on decision retrieval
- At 100-1000 node scale: <10ms end-to-end retrieval pipeline
- GrafitoDB already has `text_search()`, `to_networkx()`, `find_shortest_path()` — just unused
- NetworkX is already installed as a grafito dependency — no new pip deps needed
- Production pattern validated: FTS5 seed → BFS expansion → graph algorithm scoring

### Metis Review
**Identified Gaps (addressed)**:
- GrafitoDB has built-in FTS5 that was never activated — Task 2 is a 2-line change, not a subsystem
- `trace()` is outgoing-only — need bidirectional for proper traversal (Task 3)
- NetworkX is already a transitive dependency — no new deps needed
- OpenCode session querying crosses DB boundaries — must be separate script with project scoping
- SKILL.md redesign should come LAST, after all CLI surfaces are stable

---

## Work Objectives

### Core Objective
Make the governance graph queryable through structured retrieval commands instead of reading entire files into context. Every query returns focused, ranked results from the graph.

### Concrete Deliverables
- `gov.py search "MCP"` → returns D5, D6 with BM25 scores
- `gov.py context "substrate pools"` → returns focused subgraph (8-12 nodes)
- `gov.py traverse --from D3 --depth 2 --direction both` → bidirectional traversal
- `gov.py rank` → PageRank scores on all nodes
- `gov.py communities` → Louvain/Leiden community detection
- `sessions.py search "governance"` → queries OpenCode sessions for mesh project
- SKILL.md redesigned to ≤400 lines with retrieval-first interaction

### Definition of Done
- [ ] `python3 scripts/gov.py search "snapshot"` returns D1, D2 (known content)
- [ ] `python3 scripts/gov.py rank` returns D1 as highest PageRank (it enables D2-D4)
- [ ] `python3 -m pytest tests/ -v` shows 105 original tests + new tests all passing
- [ ] `wc -l ~/.agents/skills/mesh-nav/SKILL.md` ≤ 400
- [ ] `time python3 scripts/gov.py search "nomad"` completes in <100ms

### Must Have
- FTS5 text search activated on governance graph
- Bidirectional graph traversal (incoming + outgoing)
- At least PageRank and community detection via NetworkX
- Structured CLI query interface in gov.py
- SKILL.md redesigned for retrieval-first interaction
- All 105 existing tests continue passing

### Must NOT Have (Guardrails)
- NO vectors, embeddings, or semantic search
- NO migration to GraphQLite or any other graph DB
- NO new pip dependencies (NetworkX already available via grafito)
- NO touching learning.py, session.py, generate.py, or migrate.py (they work, leave them alone)
- NO SKILL.md over 400 lines
- NO single monolithic query.py — extend existing graph.py and gov.py
- NO compression or summarization of any stored data
- NO criteria requiring "user manually tests" — all verification is agent-executable

---

## Verification Strategy

> **ZERO HUMAN INTERVENTION** — ALL verification is agent-executed. No exceptions.

### Test Decision
- **Infrastructure exists**: YES (tests/ directory with 105 tests)
- **Automated tests**: YES (TDD — write tests first, then implement)
- **Framework**: pytest (stdlib, already in use)

### QA Policy
Every task MUST include agent-executed QA scenarios.
Evidence saved to `.sisyphus/evidence/task-{N}-{scenario-slug}.{ext}`.

- **CLI commands**: Use Bash — run gov.py commands, assert output
- **Graph queries**: Use Bash — run search/rank/traverse, verify returned nodes
- **Session queries**: Use Bash — run sessions.py, verify project-scoped results
- **SKILL.md**: Use Bash — wc -l, grep for required sections

---

## Execution Strategy

### Parallel Execution Waves

```
Wave 1 (Start Immediately — validation + foundation):
├── Task 1: Validate assumptions (to_networkx, opencode part structure, SKILL.md budget) [quick]
└── Task 2: Activate FTS5 text search in graph.py + gov.py search command [quick]

Wave 2 (After Wave 1 — core retrieval, MAX PARALLEL):
├── Task 3: Bidirectional graph traversal enhancements in graph.py/gov.py [unspecified-high]
├── Task 4: NetworkX algorithm layer (PageRank, centrality, communities) [deep]
└── Task 5: Integrate existing session-query skill into SKILL.md [quick]

Wave 3 (After Wave 2 — integration):
├── Task 6: SKILL.md redesign for retrieval-first interaction [deep]
└── Task 7: Result formatting for LLM context injection [unspecified-high]

Wave FINAL (After ALL tasks — 4 parallel reviews):
├── F1: Plan compliance audit (oracle)
├── F2: Code quality review (unspecified-high)
├── F3: Real manual QA (unspecified-high)
└── F4: Scope fidelity check (deep)
→ Present results → Get explicit user okay

Critical Path: Task 1 → Task 2 → Task 3 → Task 6 → F1-F4
Parallel Speedup: ~50% faster than sequential
Max Concurrent: 3 (Wave 2)
```

### Dependency Matrix

| Task | Depends On | Blocks | Wave |
|------|-----------|--------|------|
| 1 | — | 2, 3, 4, 5 | 1 |
| 2 | 1 | 3, 6 | 1 |
| 3 | 1, 2 | 6, 7 | 2 |
| 4 | 1 | 6, 7 | 2 |
| 5 | 1 | 6 | 2 |
| 6 | 2, 3, 4, 5 | 7 | 3 |
| 7 | 3, 4, 5 | F1-F4 | 3 |
| F1-F4 | 6, 7 | user okay | FINAL |

### Agent Dispatch Summary

- **Wave 1**: 2 — T1 → `quick`, T2 → `quick`
- **Wave 2**: 3 — T3 → `unspecified-high`, T4 → `deep`, T5 → `quick`
- **Wave 3**: 2 — T6 → `deep`, T7 → `unspecified-high`
- **FINAL**: 4 — F1 → `oracle`, F2 → `unspecified-high`, F3 → `unspecified-high`, F4 → `deep`

---

## TODOs

- [ ] 1. Validate Assumptions (to_networkx fidelity, opencode part structure, SKILL.md line budget)

  **What to do**:
  - Write a 10-line validation script that calls `g.to_networkx()` on `.mesh/governance.db` and checks: does the exported NetworkX graph preserve edge types? Node properties? Labels?
  - Query opencode.db `part` table for 3 messages — **SKIPPED** (we're using existing session-query skill instead of direct opencode.db querying)
  - Count current SKILL.md lines (353) and estimate line budget for new retrieval commands (47 lines available under 400)
  - Run `python3 -m pytest tests/ -v` to confirm 105 tests pass on current state

  **Must NOT do**:
  - Do not modify any files — this is read-only validation
  - Do not add new dependencies

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Read-only validation, no implementation
  - **Skills**: []
  - **Skills Evaluated but Omitted**:
    - `mesh-nav`: Not needed — direct Python execution suffices

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Task 2)
  - **Blocks**: Tasks 2, 3, 4, 5
  - **Blocked By**: None (can start immediately)

  **References**:

  **Pattern References**:
  - `~/.agents/skills/mesh-nav/scripts/graph.py:272-303` — `trace()` function shows how to work with GrafitoDB graph (resolve mesh_id → int_id, traverse, return dicts)
  - `~/.agents/skills/mesh-nav/tests/test_graph.py` — Existing test patterns for graph.py functions (use in-memory DB)

  **API/Type References**:
  - GrafitoDB API: `text_search(query, k, labels, rel_types)`, `to_networkx()`, `create_text_index()`, `rebuild_text_index()`, `find_shortest_path(source_id, target_id)`
  - opencode.db schema: `part` table has columns `(id TEXT, message_id TEXT, session_id TEXT, time_created INTEGER, time_updated INTEGER, data TEXT)`
  - governance.db schema: `nodes (id, created_at, properties JSON, uri)`, `relationships (id, source_node_id, target_node_id, type, created_at, properties JSON, uri)`

  **External References**:
  - GrafitoDB is imported from `grafito` package — check its API via `python3 -c "from grafito import GrafitoDatabase; help(GrafitoDatabase.text_search)"`

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: to_networkx preserves edge types and node properties
    Tool: Bash
    Preconditions: .mesh/governance.db exists with 36 nodes, 27 edges
    Steps:
      1. Run: python3 -c "from sys import path; path.insert(0, '~/.agents/skills/mesh-nav/scripts'); from graph import get_graph; g = get_graph('.mesh/governance.db'); nx = g.to_networkx(); print('Nodes:', nx.number_of_nodes()); print('Edges:', nx.number_of_edges()); print('Edge types:', set(d.get('type','?') for _,_,d in nx.edges(data=True)))"
      2. Assert output contains 'Nodes: 36' (or close)
      3. Assert output contains 'Edges: 27' (or close)
      4. Assert edge types include at least 'enables' or 'blocks'
    Expected Result: NetworkX graph has all nodes, edges, and preserved edge types
    Failure Indicators: Node count mismatch, edge types missing, import error
    Evidence: .sisyphus/evidence/task-1-networkx-validation.txt

  Scenario: opencode part table has usable text content
    Tool: Bash
    Preconditions: opencode.db at ~/.local/share/opencode/opencode.db
    Steps:
      1. Run: python3 -c "import sqlite3, json; c=sqlite3.connect('~/.local/share/opencode/opencode.db'); r=c.execute('SELECT data FROM part LIMIT 3').fetchall(); [print(json.dumps(json.loads(d[0]), indent=2)[:300]) for d in r]"
      2. Inspect output: does `data` contain text content? What's the structure?
    Expected Result: part.data contains text content with identifiable structure (role, text, tool calls, etc.)
    Failure Indicators: Empty data, binary data, or encrypted content
    Evidence: .sisyphus/evidence/task-1-opencode-part-structure.txt
  ```

  **Commit**: NO (read-only validation)

---

- [ ] 2. Activate FTS5 Text Search in graph.py + gov.py search Command

  **What to do**:
  - In `graph.py`, add a `search_nodes(g, query, k=10, node_type=None)` function that:
    1. Calls `g.text_search(query, k, labels=[node_type] if node_type else None)` 
    2. Returns list of dicts `{mesh_id, type, title, score, snippet}`
  - In `gov.py`, add a `cmd_search` subcommand that:
    1. Takes `query` (required), `--type` (optional filter), `--limit` (default 10)
    2. Calls `search_nodes()` and prints results as tab-separated: mesh_id, type, title, score
  - If GrafitoDB's FTS5 index isn't built yet, add an `init_search` function that calls `g.create_text_index()` + `g.rebuild_text_index()`
  - Add `gov.py index` subcommand to build/rebuild the FTS5 index
  - Write tests in `tests/test_graph.py` for `search_nodes` using in-memory DB with test data
  - Write tests in `tests/test_gov.py` for `cmd_search` using in-memory DB

  **Must NOT do**:
  - Do not write raw FTS5 SQL — use GrafitoDB's `text_search()` API
  - Do not add new dependencies
  - Do not modify existing functions (only add new ones)

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Small, well-scoped addition following existing patterns
  - **Skills**: []
  - **Skills Evaluated but Omitted**:
    - `mesh-nav`: The task IS the mesh-nav skill — circular

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Task 1, but logically after Task 1 validates text_search works)
  - **Blocks**: Tasks 3, 6
  - **Blocked By**: Task 1 (validates text_search API exists and works)

  **References**:

  **Pattern References**:
  - `~/.agents/skills/mesh-nav/scripts/graph.py:127-136` — `get_node()` shows the pattern: accept mesh_id, call GrafitoDB, return dict
  - `~/.agents/skills/mesh-nav/scripts/gov.py:102-115` — `cmd_list()` shows the pattern: argparse subcommand, call graph function, format output
  - `~/.agents/skills/mesh-nav/scripts/gov.py:266-277` — `dispatch` dict shows where to register new command

  **API/Type References**:
  - GrafitoDB `text_search(query, k, labels, rel_types)` — returns list of scored results
  - GrafitoDB `create_text_index()` + `rebuild_text_index()` — index management

  **Test References**:
  - `~/.agents/skills/mesh-nav/tests/test_graph.py` — Existing test patterns (in-memory DB, add_node then query)

  **WHY Each Reference Matters**:
  - `get_node()` pattern: Shows the mesh_id → int_id → return dict pipeline that all graph.py functions follow
  - `cmd_list()` pattern: Shows how to add a new gov.py subcommand with argparse
  - `test_graph.py`: Shows how to set up in-memory test DB with test data

  **Acceptance Criteria**:

  **If TDD:**
  - [ ] Test file: `tests/test_graph.py` — new `test_search_nodes()` function
  - [ ] Test file: `tests/test_gov.py` — new `test_cmd_search()` function
  - [ ] `python3 -m pytest tests/ -v` → ALL pass (105 original + new)

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: FTS5 search returns relevant governance nodes
    Tool: Bash
    Preconditions: .mesh/governance.db has nodes with "snapshot" in title/body (D1, D2)
    Steps:
      1. python3 ~/.agents/skills/mesh-nav/scripts/gov.py index
      2. python3 ~/.agents/skills/mesh-nav/scripts/gov.py search "snapshot"
      3. Assert output contains at least one mesh_id starting with 'D'
    Expected Result: Returns D1 or D2 (decisions about snapshot primitive)
    Failure Indicators: "no results" or empty output
    Evidence: .sisyphus/evidence/task-2-fts5-search.txt

  Scenario: Search with type filter
    Tool: Bash
    Preconditions: FTS5 index built
    Steps:
      1. python3 ~/.agents/skills/mesh-nav/scripts/gov.py search "governance" --type decision
      2. Assert all results have type 'decision'
    Expected Result: Only decision nodes returned
    Failure Indicators: Non-decision types in results
    Evidence: .sisyphus/evidence/task-2-fts5-type-filter.txt

  Scenario: Empty search results handled gracefully
    Tool: Bash
    Preconditions: FTS5 index built
    Steps:
      1. python3 ~/.agents/skills/mesh-nav/scripts/gov.py search "quantum teleportation"
      2. Assert output says "No results" or similar (not an error/traceback)
    Expected Result: Graceful "no results" message
    Failure Indicators: Traceback, error exit code
    Evidence: .sisyphus/evidence/task-2-fts5-empty-results.txt
  ```

  **Commit**: YES
  - Message: `feat(retrieval): add FTS5 text search to graph.py and gov.py`
  - Files: `scripts/graph.py`, `scripts/gov.py`, `tests/test_graph.py`, `tests/test_gov.py`
  - Pre-commit: `python3 -m pytest tests/ -v`

---

- [ ] 3. Bidirectional Graph Traversal Enhancements

  **What to do**:
  - In `graph.py`, add:
    - `traverse(g, mesh_id, depth=2, direction="both", edge_types=None)` — bidirectional BFS (fix outgoing-only limitation of current `trace()`)
    - `shortest_path(g, source_id, target_id)` — wraps GrafitoDB's `find_shortest_path()`
    - `get_subgraph(g, mesh_ids, depth=1)` — extract subgraph around given nodes
  - In `gov.py`, add/update subcommands:
    - `gov.py traverse --from D3 --depth 2 --direction both --edge-types enables,blocks` — uses new `traverse()`
    - `gov.py path D1 D8` — uses `shortest_path()`
    - `gov.py context "MCP server"` — combines FTS5 search (seed nodes) + subgraph expansion (2-hop), returns focused subgraph with 8-12 most relevant nodes
  - Write tests for all new functions

  **Must NOT do**:
  - Do not modify existing `trace()` function — it has tests that depend on its outgoing-only behavior
  - Do not add new edge types or node types
  - Do not reimplement BFS — use GrafitoDB's `get_neighbors()` and build on it

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Multiple functions with graph traversal logic, needs careful implementation
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 4, 5)
  - **Blocks**: Tasks 6, 7
  - **Blocked By**: Tasks 1, 2

  **References**:

  **Pattern References**:
  - `~/.agents/skills/mesh-nav/scripts/graph.py:272-303` — Current `trace()` (outgoing-only BFS) — new `traverse()` should be bidirectional version
  - `~/.agents/skills/mesh-nav/scripts/graph.py:212-259` — `get_edges()` shows how to query edges by direction and type
  - `~/.agents/skills/mesh-nav/scripts/gov.py:128-143` — `cmd_trace()` shows CLI pattern for traversal commands

  **API/Type References**:
  - GrafitoDB `get_neighbors(node_id, direction, rel_type)` — already used by trace()
  - GrafitoDB `find_shortest_path(source_id, target_id)` — built-in path finding

  **WHY Each Reference Matters**:
  - `trace()` shows the BFS pattern to extend (just change direction="outgoing" to "both")
  - `get_edges()` shows how to filter by edge type — needed for `--edge-types` flag
  - `cmd_trace()` is the CLI template for new traversal commands

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Bidirectional traverse finds incoming edges
    Tool: Bash
    Steps:
      1. python3 ~/.agents/skills/mesh-nav/scripts/gov.py traverse --from D5 --depth 1 --direction incoming
      2. Assert output shows at least one node that enables D5 (since D5 is "MCP as primary interface")
    Expected Result: Shows upstream dependencies of D5
    Failure Indicators: "No connections found" when D5 has incoming edges
    Evidence: .sisyphus/evidence/task-3-bidirectional-traverse.txt

  Scenario: Context query combines FTS5 + graph expansion
    Tool: Bash
    Steps:
      1. python3 ~/.agents/skills/mesh-nav/scripts/gov.py context "snapshot"
      2. Assert output contains nodes beyond just D1/D2 (expanded via graph edges)
      3. Assert output includes edge types connecting the nodes
    Expected Result: Focused subgraph of 5-15 nodes related to "snapshot"
    Failure Indicators: Only seed nodes (no expansion), or >30 nodes (over-expansion)
    Evidence: .sisyphus/evidence/task-3-context-query.txt

  Scenario: Shortest path between two decisions
    Tool: Bash
    Steps:
      1. python3 ~/.agents/skills/mesh-nav/scripts/gov.py path D1 D8
      2. Assert output shows a path (or "no path found" with explanation)
    Expected Result: Path like D1 -[enables]-> D3 -[enables]-> ... -> D8
    Failure Indicators: Traceback, or crash on disconnected nodes
    Evidence: .sisyphus/evidence/task-3-shortest-path.txt
  ```

  **Commit**: YES
  - Message: `feat(retrieval): add bidirectional traversal, path finding, context queries`
  - Files: `scripts/graph.py`, `scripts/gov.py`, `tests/test_graph.py`, `tests/test_gov.py`
  - Pre-commit: `python3 -m pytest tests/ -v`

---

- [ ] 4. NetworkX Algorithm Layer (PageRank, Centrality, Communities)

  **What to do**:
  - In `graph.py`, add:
    - `compute_pagerank(g)` — exports to NetworkX via `g.to_networkx()`, runs PageRank, writes scores back as node properties
    - `compute_centrality(g, metric="betweenness")` — betweenness, closeness, degree centrality
    - `detect_communities(g)` — Louvain community detection, assigns community IDs to nodes
    - `get_hub_nodes(g, top_k=5)` — returns highest-degree nodes ("god nodes")
  - In `gov.py`, add:
    - `gov.py rank` — shows all nodes ranked by PageRank
    - `gov.py centrality --metric betweenness` — shows centrality scores
    - `gov.py communities` — shows community assignments
    - `gov.py hubs` — shows top connected nodes
  - Add a guard: if graph has < 50 nodes, print "Graph too small for meaningful algorithm results. Results may be trivial."
  - Write tests using small in-memory graphs (5-10 nodes) to verify algorithm output format

  **Must NOT do**:
  - Do not add NetworkX to requirements.txt — it's already a grafito transitive dependency
  - Do not compute algorithms on every search — they're expensive, only on explicit command
  - Do not store algorithm results in the graph permanently unless user explicitly saves

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Algorithm implementation needs understanding of NetworkX API and result formatting
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 3, 5)
  - **Blocks**: Tasks 6, 7
  - **Blocked By**: Task 1 (validates to_networkx fidelity)

  **References**:

  **Pattern References**:
  - `~/.agents/skills/mesh-nav/scripts/graph.py:94-124` — `add_node()` shows property writing pattern
  - `~/.agents/skills/mesh-nav/scripts/graph.py:138-148` — `update_node()` shows how to write computed values back to nodes

  **API/Type References**:
  - NetworkX PageRank: `nx.pagerank(graph, alpha=0.85, max_iter=100)`
  - NetworkX Louvain: `nx.community.louvain_communities(graph)`
  - NetworkX centrality: `nx.betweenness_centrality(graph)`, `nx.closeness_centrality(graph)`, `nx.degree_centrality(graph)`
  - GrafitoDB `to_networkx()` — one-call export

  **External References**:
  - NetworkX docs: https://networkx.org/documentation/stable/reference/algorithms/

  **WHY Each Reference Matters**:
  - `update_node()` is how algorithm results get written back to governance nodes
  - `to_networkx()` is the bridge — one call converts GrafitoDB to NetworkX format

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: PageRank identifies most connected decisions
    Tool: Bash
    Steps:
      1. python3 ~/.agents/skills/mesh-nav/scripts/gov.py rank
      2. Assert D1 appears near top (it enables D2, D3, D4)
      3. Assert output shows numeric scores for each node
    Expected Result: Ranked list with D1 as highest or near-highest score
    Failure Indicators: All scores identical, empty output, traceback
    Evidence: .sisyphus/evidence/task-4-pagerank.txt

  Scenario: Small graph guard message
    Tool: Bash
    Steps:
      1. python3 ~/.agents/skills/mesh-nav/scripts/gov.py communities
      2. Assert output includes "small" warning since graph has 36 nodes
    Expected Result: Communities listed with warning about small graph size
    Failure Indicators: No warning, or warning blocks results entirely
    Evidence: .sisyphus/evidence/task-4-communities.txt

  Scenario: Hub nodes identification
    Tool: Bash
    Steps:
      1. python3 ~/.agents/skills/mesh-nav/scripts/gov.py hubs
      2. Assert output shows top 5 nodes with edge counts
    Expected Result: 5 nodes with degree > 0, sorted by degree
    Failure Indicators: Empty output, or nodes with 0 edges
    Evidence: .sisyphus/evidence/task-4-hubs.txt
  ```

  **Commit**: YES
  - Message: `feat(retrieval): add NetworkX algorithm layer (PageRank, centrality, communities)`
  - Files: `scripts/graph.py`, `scripts/gov.py`, `tests/test_graph.py`, `tests/test_gov.py`
  - Pre-commit: `python3 -m pytest tests/ -v`

---

- [ ] 5. Integrate Existing session-query Skill into SKILL.md

  **What to do**:
  - The `session-query` skill at `~/.agents/skills/session-query/` already provides full session search with:
    - 5-stage checkpoint workflow (index, search, deep dive, extract)
    - Extraction scripts for OpenCode, Claude Code, and Gemini CLI sessions
    - FTS5 search over session content via `sessions.db`
    - Project-scoped filtering
  - This task ensures SKILL.md's Layer 4 (Research) directs agents to use `session-query` instead of reading files
  - Verify session-query's `sessions.db` is accessible and indexed for the mesh project
  - If not indexed, run the extraction: `python3 tools/index_sessions.py`
  - Add a reference in SKILL.md to use `/session-query` for past conversation search
  - No new code needed — just integration wiring in SKILL.md

  **Must NOT do**:
  - Do not create a new session query script — session-query already exists
  - Do not duplicate session-query functionality in gov.py
  - Do not modify the session-query skill itself — only reference it from SKILL.md

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Integration wiring, no new code. Verify existing tool works, add SKILL.md reference.
  - **Skills**: [`session-query`]
    - `session-query`: Must understand its workflow to properly reference it from SKILL.md

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 3, 4)
  - **Blocks**: Task 6
  - **Blocked By**: None (session-query is independent)

  **References**:

  **Pattern References**:
  - `~/.agents/skills/session-query/SKILL.md` — The existing session query skill with 5-stage workflow
  - `~/.agents/skills/session-query/tools/` — Extraction and indexing scripts

  **WHY Each Reference Matters**:
  - session-query already solves the "search past conversations" problem with a mature checkpoint-based workflow
  - We just need to wire it into SKILL.md's Layer 4 (Research) so agents know to use it

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: session-query is accessible and has mesh project data
    Tool: Bash
    Steps:
      1. ls ~/.agents/skills/session-query/SKILL.md
      2. Assert file exists
      3. Check if sessions.db exists in the sessions repo root
    Expected Result: session-query skill exists with SKILL.md
    Failure Indicators: Skill not found
    Evidence: .sisyphus/evidence/task-5-session-query-exists.txt

  Scenario: SKILL.md Layer 4 references session-query
    Tool: Bash
    Steps:
      1. After Task 6 completes, grep -c "session-query" ~/.agents/skills/mesh-nav/SKILL.md
      2. Assert count ≥ 1
    Expected Result: SKILL.md mentions session-query for research layer
    Failure Indicators: No reference to session-query
    Evidence: .sisyphus/evidence/task-5-skill-reference.txt
  ```

  **Commit**: NO (changes are part of Task 6 SKILL.md commit)

---

- [ ] 6. SKILL.md Redesign for Retrieval-First Interaction

  **What to do**:
  - Redesign `~/.agents/skills/mesh-nav/SKILL.md` (currently 353 lines) to:
    - Layer 1 (Brief): Minimal — just tell the agent to run `session.py brief` for context, NOT read files
    - Layer 2 (Decide): Tell agent to use `gov.py search`, `gov.py context`, `gov.py traverse` for queries instead of reading full markdown files
    - Layer 3 (Learn): Use `learning.py add` as before (stable, no changes)
    - Layer 4 (Research): Use `/session-query` skill for past conversation search (existing skill, no new code)
    - Layer 5 (Review): Use `gov.py rank`, `gov.py communities` for graph health checks
    - Layer 6 (Handoff): Use `session.py end` as before (stable, no changes)
    - Layer 7 (Views): Use `generate.py` as before (stable, no changes)
  - Remove: Inline file-reading mandates (reading CONTEXT.md, INDEX.md, decisions.md)
  - Replace with: Query commands that return focused results
  - Keep SKILL.md under 400 lines
  - Maintain the 7-layer structure (just change HOW each layer works, not WHAT it does)

  **Must NOT do**:
  - Do not exceed 400 lines
  - Do not remove the 7-layer structure — agents depend on it
  - Do not remove existing CLI commands — only add retrieval-focused guidance
  - Do not inject long examples or documentation into SKILL.md — keep it as directives

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Structural redesign affecting all future agent sessions, needs careful thought
  - **Skills**: [`mesh-nav`]
    - `mesh-nav`: Must understand current SKILL.md structure and how agents interact with it

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 3 (sequential after Wave 2)
  - **Blocks**: Task 7
  - **Blocked By**: Tasks 2, 3, 4 (all CLI surfaces must be stable)

  **References**:

  **Pattern References**:
  - `~/.agents/skills/mesh-nav/SKILL.md` — Current 353-line SKILL.md with 7-layer structure
  - `~/.agents/skills/mesh-nav/references/reading-map.md` — What agents currently read and why
  - `~/.agents/skills/mesh-nav/references/update-protocols.md` — How agents update governance data

  **API/Type References**:
  - New gov.py commands: `search`, `traverse`, `context`, `path`, `rank`, `centrality`, `communities`, `hubs`, `index`
  - New sessions.py commands: NOT NEEDED — using existing session-query skill instead
  - Existing commands (unchanged): `session.py brief`, `session.py start`, `session.py end`, `learning.py add`, `generate.py`, `gov.py add_entity`, `gov.py get`, `gov.py list`, `gov.py trace`

  **WHY Each Reference Matters**:
  - Current SKILL.md must be read to understand what gets replaced vs preserved
  - reading-map.md shows the file-reading patterns that cause context bloat
  - All new CLI commands (from Tasks 2-5) define what replaces file reading

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: SKILL.md under 400 lines
    Tool: Bash
    Steps:
      1. wc -l ~/.agents/skills/mesh-nav/SKILL.md
      2. Assert line count ≤ 400
    Expected Result: Line count between 300-400
    Failure Indicators: > 400 lines
    Evidence: .sisyphus/evidence/task-6-skill-line-count.txt

  Scenario: SKILL.md references retrieval commands not file reads
    Tool: Bash
    Steps:
      1. grep -c "CONTEXT.md\|INDEX.md\|decisions.md" ~/.agents/skills/mesh-nav/SKILL.md
      2. Assert count is 0 or minimal (these should be replaced with query commands)
      3. grep -c "gov.py search\|gov.py context\|sessions.py" ~/.agents/skills/mesh-nav/SKILL.md
      4. Assert count ≥ 5 (new retrieval commands are referenced)
    Expected Result: File-read mandates replaced with query commands
    Failure Indicators: Still mandates reading full files
    Evidence: .sisyphus/evidence/task-6-skill-retrieval-references.txt

  Scenario: 7-layer structure preserved
    Tool: Bash
    Steps:
      1. grep -c "Layer [1-7]" ~/.agents/skills/mesh-nav/SKILL.md
      2. Assert count = 7
    Expected Result: All 7 layers present
    Failure Indicators: Missing layers
    Evidence: .sisyphus/evidence/task-6-skill-layers.txt
  ```

  **Commit**: YES
  - Message: `feat(skill): redesign SKILL.md for retrieval-first interaction model`
  - Files: `SKILL.md`
  - Pre-commit: `wc -l SKILL.md` (must be ≤ 400)

---

- [ ] 7. Result Formatting for LLM Context Injection

  **What to do**:
  - Add a `--format` flag to all gov.py query commands:
    - `--format text` (default) — human-readable tabular output
    - `--format json` — structured JSON for programmatic use
    - `--format context` — XML-tagged format optimized for LLM injection, with provenance metadata
  - The `context` format wraps each result in XML tags with relevance score, type, and source:
    ```
    <node id="D5" type="decision" rank="0.92">
    Primary interface is MCP server + skills.
    <edges>enables D6, enables D8, conflicts_with D4</edges>
    </node>
    ```
  - Add `--token-budget` flag to `gov.py context` to limit output size (e.g., `--token-budget 500` returns ~500 tokens of context)
  - Implement "bookend ordering" — most relevant results first AND last (mitigates lost-in-the-middle)

  **Must NOT do**:
  - Do not change default output format — text is still default
  - Do not add XML to default output — only with `--format context`
  - Do not compress or summarize node content — return verbatim

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Formatting logic touches all query commands, needs consistency
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 3 (with Task 6, after Wave 2)
  - **Blocks**: F1-F4
  - **Blocked By**: Tasks 3, 4, 5

  **References**:

  **Pattern References**:
  - `~/.agents/skills/mesh-nav/scripts/gov.py:67-79` — `cmd_get()` shows output formatting pattern
  - `~/.agents/skills/mesh-nav/scripts/gov.py:128-143` — `cmd_list()` shows tabular output format

  **WHY Each Reference Matters**:
  - `cmd_get()` shows the aligned key: value output — `--format context` should wrap this in XML
  - `cmd_list()` shows tab-separated output — `--format json` should use the same data as JSON

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Context format produces XML-tagged output
    Tool: Bash
    Steps:
      1. python3 ~/.agents/skills/mesh-nav/scripts/gov.py search "snapshot" --format context
      2. Assert output contains <node tags with id, type, rank attributes
      3. Assert output contains </node> closing tags
    Expected Result: Valid XML-like structure with node metadata
    Failure Indicators: Plain text without tags, malformed XML
    Evidence: .sisyphus/evidence/task-7-context-format.txt

  Scenario: JSON format is valid parseable JSON
    Tool: Bash
    Steps:
      1. python3 ~/.agents/skills/mesh-nav/scripts/gov.py search "snapshot" --format json | python3 -m json.tool > /dev/null
      2. Assert exit code 0 (valid JSON)
    Expected Result: Valid JSON array of result objects
    Failure Indicators: JSON parse error, non-zero exit
    Evidence: .sisyphus/evidence/task-7-json-format.txt

  Scenario: Token budget limits output size
    Tool: Bash
    Steps:
      1. python3 ~/.agents/skills/mesh-nav/scripts/gov.py context "mesh" --format context --token-budget 200
      2. Count approximate tokens in output (words ≈ tokens)
      3. Assert output is significantly shorter than without budget
    Expected Result: Truncated to ~200 tokens
    Failure Indicators: Full output (budget ignored), or empty output
    Evidence: .sisyphus/evidence/task-7-token-budget.txt
  ```

  **Commit**: YES
  - Message: `feat(retrieval): add result formatting with JSON and LLM context modes`
  - Files: `scripts/gov.py`, `tests/test_gov.py`
  - Pre-commit: `python3 -m pytest tests/ -v`

---

## Final Verification Wave (MANDATORY — after ALL implementation tasks)

> 4 review agents run in PARALLEL. ALL must APPROVE. Present consolidated results to user and get explicit "okay" before completing.

- [ ] F1. **Plan Compliance Audit** — `oracle`
  Read the plan end-to-end. For each "Must Have": verify implementation exists (run command, check output). For each "Must NOT Have": search codebase for forbidden patterns (vector imports, new dependencies, modified stable files). Check evidence files exist in .sisyphus/evidence/. Compare deliverables against plan.
  Output: `Must Have [N/N] | Must NOT Have [N/N] | Tasks [N/N] | VERDICT: APPROVE/REJECT`

- [ ] F2. **Code Quality Review** — `unspecified-high`
  Run `python3 -m pytest tests/ -v`. Review all changed files for: import of numpy/scipy/sentence-transformers (forbidden), empty catches, console.log equivalents, commented-out code, unused imports. Check AI slop: excessive comments, over-abstraction, generic names. Verify graph.py and gov.py follow existing patterns (mesh_id resolution, structured dict returns).
  Output: `Tests [N pass/N fail] | Files [N clean/N issues] | VERDICT`

- [ ] F3. **Real Manual QA** — `unspecified-high`
  Start from clean state. Execute EVERY QA scenario from EVERY task — follow exact steps, capture evidence. Test cross-task integration: `gov.py context "governance"` uses FTS5 + traversal + formatting together. Test edge cases: empty search, disconnected nodes, concurrent writes. Save to `.sisyphus/evidence/final-qa/`.
  Output: `Scenarios [N/N pass] | Integration [N/N] | Edge Cases [N tested] | VERDICT`

- [ ] F4. **Scope Fidelity Check** — `deep`
  For each task: read "What to do", read actual diff (git log/diff). Verify 1:1 — everything in spec was built, nothing beyond spec was built. Check "Must NOT do" compliance. Verify stable files untouched (learning.py, session.py, generate.py, migrate.py). Flag unaccounted changes.
  Output: `Tasks [N/N compliant] | Stable Files [TOUCHED/CLEAN] | Unaccounted [CLEAN/N files] | VERDICT`

---

## Commit Strategy

- **Task 1**: No commit (read-only validation)
- **Task 2**: `feat(retrieval): add FTS5 text search to graph.py and gov.py` — graph.py, gov.py, tests/
- **Task 3**: `feat(retrieval): add bidirectional traversal, path finding, context queries` — graph.py, gov.py, tests/
- **Task 4**: `feat(retrieval): add NetworkX algorithm layer (PageRank, centrality, communities)` — graph.py, gov.py, tests/
- **Task 5**: No separate commit (changes merged into Task 6 SKILL.md commit)
- **Task 6**: `feat(skill): redesign SKILL.md for retrieval-first interaction model` — SKILL.md
- **Task 7**: `feat(retrieval): add result formatting with JSON and LLM context modes` — gov.py, tests/

---

## Success Criteria

### Verification Commands
```bash
python3 -m pytest tests/ -v                    # Expected: ALL pass (105+ original + new)
python3 scripts/gov.py index                    # Expected: FTS5 index built successfully
python3 scripts/gov.py search "snapshot"        # Expected: Returns D1, D2 with scores
python3 scripts/gov.py context "MCP"            # Expected: Focused subgraph of 5-15 nodes
python3 scripts/gov.py rank                     # Expected: D1 near top of PageRank
python3 scripts/gov.py communities              # Expected: Community assignments (with small-graph warning)
python3 scripts/sessions.py recent --limit 5    # Expected: Recent mesh sessions listed
wc -l ~/.agents/skills/mesh-nav/SKILL.md        # Expected: ≤ 400 lines
time python3 scripts/gov.py search "nomad"      # Expected: < 100ms
```

### Final Checklist
- [ ] All "Must Have" present (FTS5, traversal, algorithms, CLI, SKILL.md redesign, formatting)
- [ ] All "Must NOT Have" absent (no vectors, no new deps, no modified stable files)
- [ ] All 105+ tests passing
- [ ] SKILL.md ≤ 400 lines
- [ ] Every query command returns results in < 100ms
