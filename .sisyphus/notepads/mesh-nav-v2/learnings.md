# Learnings — mesh-nav-v2

## 2026-04-25 Session Start
- Plan: 12 tasks + 4 final verification = 16 total
- Wave 0: T1 (GrafitoDB smoke test) is GATE — blocks everything
- GrafitoDB: SQLite-backed property graph, Cypher queries, BFS/DFS
- 3 transitive deps: networkx, orjson, zstandard
- Key files: scripts under ~/.agents/skills/mesh-nav/scripts/
- Tests under ~/.agents/skills/mesh-nav/tests/

## GrafitoDB API Patterns (2025-04-26)

**Package**: `grafito` v0.1.0 (pip reports 0.1.0, metadata says 0.1.2)
**Main class**: `GrafitoDatabase` (NOT `GrafitoDB`)
**Import**: `from grafito import GrafitoDatabase, Node, Relationship`
**Deps**: networkx 3.6.1, orjson 3.11.8, zstandard 0.25.0

### Key API methods:
- `db = GrafitoDatabase(":memory:")` — in-memory DB
- `db.create_node(labels=["label"], properties={...})` → `Node` (has `.id`, `.labels`, `.properties`)
- `db.create_relationship(source_id, target_id, "TYPE", properties={...})` → `Relationship` (has `.id`, `.source_id`, `.target_id`, `.type`, `.properties`)
- `db.execute("MATCH (n:Label) RETURN n")` → `list[dict]` where each dict has key "n" → `{id, labels, properties, uri}`
- `db.execute("MATCH (a)-[r:TYPE]->(b) RETURN a, b")` → `list[dict]` with keys "a", "b"
- `db.find_shortest_path(src_id, tgt_id)` → `list[Node]` (BFS) or None
- `db.get_neighbors(node_id, direction="outgoing")` → `list[Node]`
- `db.delete_node(node_id)` → `bool` (cascades to relationships)
- `db.update_node_properties(node_id, {k: v})` — merges with existing props
- `db.match_nodes(labels=["label"])` → `list[Node]`
- `db.match_relationships(rel_type="TYPE")` → `list[Relationship]`

### Smoke test result: ALL 8/8 operations PASS
- 7 node types, 10 edge types, Cypher MATCH (label/property/edge), BFS depth 2+, delete cascade, property update — all work.
- **Gate PASSED. Plan does NOT pivot to raw SQLite.**

## graph.py Abstraction Layer (T2 completed)

### GrafitoDB limitations discovered:
- **No `type(r)` Cypher function.** Cannot do `RETURN type(r)`. Must use `match_relationships(rel_type=TYPE)` to enumerate relationships by type, then filter by source/target IDs.
- **No parameterized queries.** `db.execute()` takes a plain string. graph.py's `query()` does basic string substitution for named params.
- **`_connection` is private** — Pyright flags it. Need `# pyright: ignore` for WAL pragma. Works at runtime.

### Architecture decisions for graph.py:
- All functions take `graph` (GrafitoDatabase instance) as first arg
- mesh_id is the only identifier exposed to callers — integer IDs are internal
- `_find_node_by_mesh_id()` does Cypher lookup `MATCH (n {mesh_id: 'X'}) RETURN n`, then `graph.get_node(int_id)` to get the actual Node object
- `_node_to_dict()` converts Node to flat dict, injects `type` from labels[0]
- `get_edges()` uses `match_relationships(rel_type=...)` for each EDGE_TYPE, filters by source/target integer ID
- `trace()` uses BFS via `get_neighbors(direction="outgoing")`, with `_find_edge_type()` helper that checks all EDGE_TYPES
- Return types: dicts/lists/None/bool — never Node or Relationship objects

### Test coverage: 23/23 checks pass
- Full CRUD, edge type validation, nonexistent lookups, cascade delete, extra properties, BFS trace depth 2
