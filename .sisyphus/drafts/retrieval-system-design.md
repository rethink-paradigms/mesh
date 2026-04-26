# Draft: Retrieval System Design for mesh-nav

## Requirements (confirmed)
- "We have the session, we have the schemas, we just need to think more about what can be the queries built"
- "Search extensively on GitHub on what retrieval systems people have built"
- "Use references from that, not directly, but the reference of how retrieval techniques are being used"
- Retrieval over compression (NO summarization, ONLY smarter retrieval)
- Skill-first approach (not Python-script-first)
- Query both: governance graph (.mesh/governance.db) AND OpenCode sessions (opencode.db)

## Technical Decisions
- Two databases to query: GrafitoDB graph (34 entities, 27 edges) + OpenCode SQLite (sessions, messages, parts with rich schema)
- GrafitoDB already has FTS5 tables (fts_index, fts_index_data, etc.) but we don't use them
- OpenCode already has parent_id on sessions — hierarchy is built in
- Our graph has typed edges that enable Graph-RAG patterns

## Research Findings

### Phase 1 (Previous Session - Complete)
- Yuan et al. (2026): Raw storage + good retrieval (77.2%) beats compression (~73%)
- Graph-RAG: 322% improvement over vector-only for causal/multi-hop queries
- Hierarchical sessions: Devin, PraisonAI, OpenClaw all have parent-child patterns
- SQLite + sqlite-vec + FTS5 is the proven lightweight stack

### Phase 2 (Current Session - Complete)

#### A. Graph Retrieval Techniques
**Key finding: We're not limited to text search. Five retrieval layers exist:**
1. **Keyword (FTS5/BM25)**: Exact term matching, <5ms for 10K rows. SQLite FTS5 supports phrase search, NEAR proximity, column filtering, snippet generation, highlight, prefix indexes for autocomplete, weighted BM25 scoring
2. **Graph Traversal**: Multi-hop via typed edges. Our 10 edge types already enable this. Variable-length paths `*1..3`, shortest path, BFS/DFS
3. **Semantic (Vector)**: For concept similarity. Would need sqlite-vec + embeddings. NOT needed for v1 given our small corpus
4. **Hybrid (RRF)**: Reciprocal Rank Fusion combines multiple retrievers. Formula: `RRF(d) = Σ(1/(k+rank_i))` where k=60. Production benchmarks: 85% recall@10 vs 72% vector-only
5. **Graph-RAG**: Our typed edges (enables, conflicts_with, blocks) already do this. 322% improvement over vector-only for causal queries

**Production benchmarks:**
| Method | Recall@10 | Latency | Best For |
|--------|-----------|---------|----------|
| Vector Only | 72% | 20ms | Semantic Q&A |
| Keyword Only | 55% | 10ms | Exact matches |
| Hybrid (RRF) | 85% | 35ms | General RAG |
| Hybrid + Re-Ranker | 94% | 150ms | Enterprise |

#### E. GraphQLite (Primary Candidate for Query Engine)
- **What**: SQLite extension (C-based) that adds Cypher query language + graph algorithms
- **Install**: `brew install graphqlite` or `pip install graphqlite`
- **Stars**: 243, MIT license, v0.4.0, last push March 2026, 5 contributors
- **Cypher support**: MATCH, CREATE, MERGE, SET, DELETE, WITH, UNWIND, RETURN
- **Algorithms**: PageRank, Louvain community detection, Dijkstra shortest path, BFS/DFS, connected components
- **Bindings**: Python (`from graphqlite import Graph`), Rust, raw SQL
- **Key API**: `g.query("MATCH (a:Person)-[:KNOWS]->(b) RETURN a.name, b.name")`
- **Critical question**: Can it work with our existing GrafitoDB schema (nodes with JSON properties), or does it need its own storage format?
- **Also has**: GraphRAG example with HotpotQA dataset, uses sqlite-vec for embeddings (but we don't need that part)

#### F. graphify (safishamsi) — Under investigation by agent

#### B. GitHub Projects People Actually Built (References)

**Session Search (closest to our use case):**
1. **gebeer/conversation-search** — MCP server, SQLite FTS5, sub-second incremental reindexing, snippet generation
2. **lee-fuhr/claude-session-index** — CLI + MCP, FTS5, cross-session synthesis, live topic tracking via hooks
3. **rodbland2021/claw-recall** — SQLite FTS5 + optional semantic (OpenAI embeddings), auto-detects keyword vs semantic
4. **CrowLoki/conversation-memory** — sqlite-vec (KNN) + FTS5 hybrid, 200k+ chunks, embedding cache

**Knowledge Graph on SQLite:**
5. **hiyenwong/sqlite-knowledge-graph** — Rust, two-stage RAG (TurboQuant ANN → cosine rerank → BFS graph expansion). Paper-driven algorithms from MemRL, RAPO, Memex(RL)
6. **colliery-io/graphqlite** — Full Cypher query language over SQLite! MATCH, CREATE, 15+ graph algorithms (PageRank, Louvain, Dijkstra). Zero config.
7. **getzep/graphiti** (25k stars) — Temporal knowledge graph, relationship-aware context, autonomous maintenance

**PKM Retrieval (Obsidian/Logseq):**
8. **obra/knowledge-graph** — Obsidian + sqlite-vec + FTS5 + graph algorithms (Louvain, betweenness, PageRank, all-simple-paths). MCP plugin with 10 operations.
9. **azainulbhai/logseq-search-mcp** — FTS5 with topic_dossier aggregation (own page + backlinks + FTS matches). ~5s to index 2000 files.

**AI Memory Systems:**
10. **mem0ai/mem0** (54k stars) — Multi-signal retrieval: semantic + BM25 + entity matching, fused scoring. Single-pass ADD-only extraction.
11. **getzep/zep** (4.4k stars) — Graph-based temporal reasoning + semantic similarity, <200ms latency at scale

#### C. Query Type Taxonomy (6 categories)

**MUST-HAVE for v1:**
1. **Relational** — "What conflicts with D3?", "Dependency chain D1→D8", "What enables X?"
   - Graph ops: Single-hop reverse, multi-hop forward, variable-length paths
   - Schema: ✅ FULLY SUPPORTED by our 10 edge types
   
2. **Contextual** — "Everything about MCP server", "Full context around D12"
   - Graph ops: Neighborhood expansion (1-2 hops), text-filtered expansion
   - Schema: ✅ With FTS5 for seed finding, then graph expansion
   
3. **Cross-reference** — "Which decisions reference this learning?", "What constraints apply?"
   - Graph ops: Single-type reverse, multi-type chains, variable-length paths
   - Schema: ✅ FULLY SUPPORTED
   
4. **Temporal** — "What changed since S12?", "Evolution of D3", "Decisions from last week"
   - Graph ops: Time-bounded matching, temporal paths
   - Schema: ⚠️ Needs time-based querying on created_at/updated_at fields

**NICE-TO-HAVE for v2:**
5. **Gap/Anomaly** — "Orphan nodes", "Unresolved questions", "Conflicting decisions"
6. **Session Continuity** — "Where did I leave off?", "Dead ends across sessions", "What was surprising?"

#### D. Retrieval Architecture

**For our scale (34-1000 nodes), the recommendation is:**
- 1 FTS5 index (external content mode, prefix='2 3' for autocomplete)
- 3 B-tree indexes (nodes by type, edges by source/target)
- 1-2 materialized views for frequent 2-hop traversals
- Real-time sync via triggers (appropriate for <1000 writes/day)

**Query interface design — 2026 consensus for AI agent CLIs:**
- JSON output only (no --json flag, JSON is default)
- Layered: Structured flags → Natural language → Direct Cypher
- Error envelopes with `retryable` field and `hint` for recovery
- Self-documenting with `--describe` command

## User Decisions (Confirmed This Session)
- **AMBITION**: Build it completely, not a minimal version
- **NO VECTORS**: Zero vector/embedding usage. Graph + FTS5 only. Rationale: "If you can't make a mental map of how things work, it's a black box. Graph I can simulate on a small model. I know the node, the edge, the retrieval."
- **GRAPH-FIRST**: All chips on graph — better nodes, better edges, better retrievals
- **GRAPHQLITE**: Strong interest in using graphqlite (colliery-io) for Cypher + 15 graph algorithms over SQLite
- **FTS5**: Acceptable as a supplement to graph, but not the primary retrieval mechanism

## Open Questions
- Can graphqlite work with our existing GrafitoDB SQLite format, or does it need its own schema?
- Should we index OpenCode sessions into our graph, or query opencode.db separately?
- Natural language query translation in v1 or v2?
- What's the materialized view refresh strategy?
- What graph algorithms matter most for our use case? (PageRank, centrality, path finding, community detection?)

## Scope Boundaries
- INCLUDE: Retrieval system design, query interface, result formatting
- EXCLUDE: Session hierarchy, relay mechanism (separate design concern)
- EXCLUDE: Compression/summarization of any kind

## Schemas Discovered
### Governance Graph (.mesh/governance.db)
- Tables: nodes (id, created_at, properties JSON, uri), relationships (id, source_node_id, target_node_id, type, properties JSON)
- Node properties schema: mesh_id, title, type, status, confidence, body, layer, valid_from, valid_until, created_at, updated_at
- Session-specific: session_type, session_date, summary, reasoning, dead_ends, surprises, deferred, blocked_on, next_steps, decisions, files_created, files_modified
- FTS5 tables exist but unused: fts_index, fts_index_data, fts_index_content, fts_index_docsize

### OpenCode Sessions (opencode.db)
- session: id, project_id, parent_id, slug, directory, title, version, time_created, time_updated, time_compacting
- message: id, session_id, time_created, data (JSON)
- part: id, message_id, session_id, data (JSON)
- session_entry: id, session_id, type, time_created, data (JSON)
- Already supports session hierarchy via parent_id!
