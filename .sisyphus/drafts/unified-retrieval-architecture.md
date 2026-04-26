# Draft: Unified Retrieval Architecture (mesh-nav + session-query)

## What I Understand of Your Intent

You realized the current retrieval plan (`retrieval-system.md`) is only about mesh-nav (governance graph). But the discussions in the last session were actually trying to cover BOTH systems:

1. **mesh-nav** — Product governance: decisions, constraints, questions, learnings, session handoffs in `.mesh/governance.db` (GrafitoDB property graph)
2. **session-query** — Coding sessions: 147 mesh sessions (7K messages) stored as flat SQL rows + JSONL files in `sessions.db`

The two got mixed in the last session's discussions because you were trying to apply the same graph thinking to session conversations, but couldn't articulate it clearly.

## Your Vision (as I understand it)

### Session conversations as graphs
Instead of flat JSONL files, each session should be a **graph structure**:
- **System Prompt** → root node
- **User Query** → branches from system prompt
- **Assistant Response** → branches from user query
- **Tool Calls** → branches from assistant response
- **Sub-agent delegation** (`task()`) → branches off into its own sub-graph
- **Sub-agent's independent conversation** → its own graph (system prompt, queries, responses, tools)

### Why this matters
- Sub-agents (explore, librarian, oracle, etc.) do extensive research — creating decisions, analyzing code, exploring patterns
- Currently, their work "flies off" — it disappears from the main conversation context after the task completes
- A graph structure preserves the full tree: what sub-agent was asked, what it found, what tools it used
- This makes ALL conversations across ALL sub-agents **queryable and retrievable**

### The unified vision
Both systems should speak the same graph language:
- **mesh-nav graph**: "What did we decide? What are the constraints? How do things relate?"
- **session graph**: "What did the explore agent find? What pattern did the librarian discover? What did the oracle conclude?"

Together they create **total recall** of every decision, every research, every dead end across all sessions and all sub-agents.

## Current Architecture Contrast

| Aspect | mesh-nav | session-query (current) | Your desired |
|--------|----------|-------------------------|--------------|
| Storage | GrafitoDB property graph | SQLite flat table + JSONL files | Graph-structured |
| Nodes | decisions, constraints, questions, learnings, sessions | sessions (row per session) | messages, tool calls, sub-agents |
| Edges | enables, blocks, conflicts_with, etc. | None (flat) | prompts, responds_to, invokes, delegates_to |
| Retrieval | Being built (plan) | SQL only | Graph traversal + FTS5 |
| Sub-agent data | N/A | Invisible (inside JSONL blobs) | First-class nodes |

## Decisions Made

1. **Separate DBs, cross-referenced** — Sessions get their own graph DB, mesh-nav stays in `.mesh/governance.db`. Cross-references via IDs (e.g., session node properties reference governance decision IDs).

2. **Depth: RESEARCH PENDING** — User wants sub-agents to research trade-offs (retrieval speed, DB size, what's happening in AI world, GitHub best practices)

3. **Backfill all 147 sessions** — All existing mesh session JSONL files will be converted to graph structure

4. **Separate plans** — mesh-nav retrieval plan stays as-is (`retrieval-system.md`). New plan created for session graph infrastructure.

## Additional Decisions

5. **Read-only for original content, write for metadata** — Original messages never change. Annotations, tags, links to governance decisions are separate nodes that reference originals. Immutable source, mutable metadata.

6. **Graph engine: lightweight preferred** — Heavy dedicated graph DBs (Neo4j, etc.) feel like overkill. GrafitoDB/SQLite-based approach preferred (same family as mesh-nav).

## Research: Session JSONL Structure (Explore #1)
- 69 mesh sessions, 1,248 messages, 12.9MB raw
- Messages: 81% assistant, 19% user — NO "tool" role messages exist
- Tool calls embedded as `tool_calls[]` on assistant messages, not separate nodes
- `task()` sub-agent output is PLAIN TEXT (~12-13KB avg), never structured JSON
- Node/edge ratio ≈ 1:1 (conversations are linear, not branching trees)
- Full graph for 147 sessions: ~69K nodes, ~69K edges — well within SQLite comfort
- No parent_id, no message IDs in extracted JSONL

## Research: Extraction Pipeline (Explore #2) — 🔴 CRITICAL DISCOVERY
- **`session.parent_id` exists in raw opencode.db for 84% of sessions (655/774)** — completely discarded during extraction!
- **`message.parentID` exists for 87% of messages (13,123/15,022)** — also discarded!
- Sub-agent hierarchy is the DOMINANT pattern: 64 task() calls, 655 parent-linked sessions
- Fixing this is ~5 lines of code: add parent_id to session record + message_id/parentID to messages
- `task()` tool calls in parent sessions DON'T link to child session IDs in extracted data — the link exists ONLY in `session.parent_id` (discarded)

## Research: AI Conversation Graph Storage (Librarian)
- **NOBODY stores raw conversations as property graphs in production**
- Production pattern: flat storage for raw text + graph layer for extracted entities/relationships
- Neo4j-agent-memory: most mature — conversations flat, entities in Neo4j graph
- Mem0 (51K stars): REMOVED graph store in April 2026 — overhead not worth it for most cases
- GrafitoDB (already used by mesh-nav) is the right lightweight choice
- Hybrid model is the consensus: immutable raw text + graph-based metadata/index

## Key Insight: The Graph Already Exists, It's Just Invisible
The raw opencode.db has ALL the graph structure — parent-child session links, message threading, tool call chains. The extraction pipeline discards it all, turning a rich graph into flat JSONL. The fix is ~5 lines of code. The real question isn't "how do we build a graph" — it's "how do we expose the graph that already exists and make it queryable."

- **147 mesh sessions** in sessions.db (69 opencode, 77 claude, 1 gemini), ~6,849 total messages
- **session-query skill** already has extraction tools but stores sessions as flat JSONL
- **mesh-nav** has GrafitoDB with graph capabilities (FTS5, to_networkx, path finding)
- The current retrieval plan has 7 tasks all focused on mesh-nav governance graph
- **Key limitation**: Sub-agent conversations are embedded inside main session JSONL as tool call responses — they're not independently stored or queryable

## What I Think You're Driving At

You want a system where every conversation — whether the main thread or a sub-agent's exploration — is structured as a graph that you can traverse, search, and retrieve from. This means:

1. Extending the session representation from flat JSONL to graph-structured
2. Making sub-agent conversations first-class entities in the retrieval system
3. Unifying the retrieval approach across both governance (mesh-nav) and conversation (session-query) data
4. Using the same graph traversal/FTS5/algorithm toolkit for both
