# Branch-Prefix ID Collision Semantics

## Problem

When multiple worktrees (branches) create governance entities independently, they risk assigning the same canonical IDs (e.g., `D16`). This leads to collisions on merge and forces agents to think about ID allocation — a cognitive burden that should be invisible.

## Solution Overview

Branch-derived temporary IDs. Each worktree gets its own namespace. On graduation (merge to `main`), temporary IDs are rewritten to canonical IDs atomically.

## Temporary ID Format

```
{BRANCH_PREFIX}-{TYPE}{NUMBER}
```

Examples:
- `LC-D16` — branch `v1.2-lxc`, decision #16
- `VRF-L3` — branch `v1.1-refinements`, learning #3
- `MAIN-D1` — branch `main`, decision #1 (no prefix needed, but consistent)

## Branch Prefix Derivation

`get_branch_prefix()` in `graph.py` auto-detects the current git branch:

1. Run `git branch --show-current`
2. Split on hyphens/underscores
3. Take first letter of each segment, uppercase
4. Fallback: `XX` if not in a git repo

Examples:
| Branch | Prefix |
|--------|--------|
| `v1.2-lxc` | `VLX` |
| `v1.1-refinements` | `VRF` |
| `feature-auth` | `FA` |
| `main` | `MAIN` |

## Agent Workflow

The agent never sees or thinks about prefixes:

```bash
# Agent runs this — mesh-nav handles the prefix silently
gov.py add_entity --db .mesh/governance.db --title "Use zstd compression"
# Stored in DB as: mesh_id="LC-D16", type="decision"

# Agent references entities by the ID they know
gov.py add_edge --db .mesh/governance.db LC-D16 LC-C5 enables
```

If the agent explicitly provides `--id`, that ID is used as-is (no prefix added):

```bash
# Explicit ID — no prefix added
gov.py add_entity --db .mesh/governance.db --id D99 --title "Special case"
# Stored as: mesh_id="D99"
```

## max_id Tracking

The canonical DB (`main` branch) tracks a `_max_ids` node with per-type counters:

```json
{
  "mesh_id": "_max_ids",
  "type": "_max_ids",
  "decision": 15,
  "constraint": 3,
  "persona": 0,
  "question": 2,
  "learning": 3,
  "session": 7,
  "gov_decision": 1
}
```

- Created lazily on first `query_next_id()` call
- Updated atomically after each successful merge
- `query_next_id(db, "decision")` returns `"D16"` (increments counter internally)

## Graduation (Merge to Main)

`graduate.py merge` performs:

1. **Read worktree DB** — collect all proposed entities and edges
2. **Strip branch prefix** — `LC-D16` → `D16`
3. **Query canonical `_max_ids`** — get current max per type from `main` DB
4. **Assign canonical IDs** — for each stripped ID, assign next available canonical ID
   - If `D16` is next available: `LC-D16` → `D16`
   - If `D16` is taken (by another branch): `LC-D16` → `D17`
5. **Rewrite edges** — all edges referencing prefixed IDs get remapped to canonical IDs
   - Intra-worktree edges: both ends remapped
   - Cross-branch edges: flagged as conflict if target not yet graduated
6. **Update `_max_ids`** — atomically update counters on `main` DB
7. **Write to main DB** — insert graduated entities with canonical IDs

## Cross-Branch References

If a worktree entity references another worktree's entity (both ungraduated):

```
(LC-D16) --enables--> (VRF-D3)
```

`graduate.py` flags this as a conflict:

```
Conflict: LC-D16 enables VRF-D3
  VRF-D3 is from branch 'v1.1-refinements' and has not been graduated yet.
  Graduate dependency first, or resolve manually.
```

Resolution: graduate the dependency branch first, then retry.

## Edge Cases

| Scenario | Behavior |
|----------|----------|
| Agent provides explicit `--id D16` | Used as-is, no prefix added |
| Branch prefix collision (two `feature-x` branches) | Prefixes will differ if branch names differ; if identical, graduate tool handles via canonical ID assignment |
| `_max_ids` node missing | Created lazily with zero counters |
| Entity type without prefix mapping | Falls back to first letter of type name |
| Graduated entity modified | Body text is immutable; only status changes allowed |

## Implementation Status

- [x] `_max_ids` node type added to `NODE_TYPES`
- [x] `query_next_id(db, entity_type)` implemented in `graph.py`
- [x] `get_branch_prefix()` implemented in `graph.py`
- [x] `gov.py add_entity` auto-prefixes mesh_id when `--id` is not provided
- [ ] `graduate.py merge` — not yet implemented (future task)
- [ ] Cross-branch conflict detection — not yet implemented (future task)

## Files Modified

- `~/.agents/skills/mesh-nav/scripts/graph.py` — added `_max_ids` to `NODE_TYPES`, `query_next_id()`, `get_branch_prefix()`
- `~/.agents/skills/mesh-nav/scripts/gov.py` — `cmd_add_entity` auto-prefixes mesh_id, `--id` is now optional
- `~/.agents/skills/mesh-nav/tests/test_graph.py` — updated `test_node_types_complete` to expect `_max_ids`
