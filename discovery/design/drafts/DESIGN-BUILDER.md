# Design Builder — A Meta-Agent System

> Seed document. Extracted from a real design session (Mesh project, April 2026).
> This is the first artifact for building a general-purpose design builder agent.

## What This Is

A Design Builder takes a project from "I have an idea" to "I have a complete, tested design ready for implementation." It's not a code generator. It's a design engineer — the thing that happens before code exists.

Accompanied by a Persistent Home Session system — one session that lives for the lifetime of a project, with smart compression that keeps early context as abstractions and recent context fresh.

Together, these form a new type of AI agent interaction model.

---

## Part 1: Patterns Extracted from Real Work

Seven patterns emerged from a real design session. These are the building blocks of the Design Builder.

### P1: Keystone Resolution
Circular dependency webs have a most-constrained node. Find it. Resolve it first. Everything cascades.
Implementation: When the user is stuck circling between N open questions, analyze the dependency graph. Find the node with the most outgoing edges. Resolve it. Then cascade.

### P2: Dependency-Aware Parallelism
After the keystone breaks, map the dependency graph. Independent nodes get parallel research agents. One-at-a-time becomes N simultaneous agents because dependencies were mapped.
Implementation: Maintain a dependency matrix. After each resolution, recalculate. Fire agents for all newly-unblocked items simultaneously.

### P3: Philosophy-First Module Boundaries
Don't draw modules from features or user stories. Draw them where an agent can work independently with bounded context. Let the count emerge from the philosophy, not from a target number.
Implementation: Ask "who is the user of this design?" If the answer is "the developer/agent who builds it," use bounded context philosophy. If the answer is "the end user," you're designing the wrong thing at this layer.

### P4: Thought Experiments Before Implementation
6 structured tests per module: Happy Path, Failure at Each Step, Concurrency, Scale, State Machine Edge Cases, Contract Verification. Strict output template. No exceptions.
Implementation: METHODOLOGY.md pattern. Each deep-dive follows the template. The 6 tests are non-negotiable. Output is 150-250 lines per module.

### P5: File-Based Persistence
Files are the persistent state. Chat is working memory. Every session reconstructs full context from files alone. Research files are reusable across sessions.
Implementation: Defined file structure. INDEX.md as the dashboard. Research files in research/. Design files in design/. Every file is self-contained and cross-referenced.

### P6: Conceptual Compression
Not textual summarization. Early context becomes abstract patterns (like human memory). Recent context retains detail. The journey arc matters more than the transcript.
Implementation: SESSION-HANDOFF.md pattern. Phases compressed at different levels. Philosophy and working style preserved at the end. "Welcome home" as the contract.

### P7: Research-First, Decide-Second
Every major decision backed by a research file. Decisions aren't made until research is complete. Research files are reusable across sessions.
Implementation: Librarian agents for external knowledge. Explore agents for codebase knowledge. Both write to research/ files. Decisions reference research files by name.

---

## Part 2: The Design Builder System

### Definition
A Design Builder is an agent (or agent orchestrator) that guides a project through structured phases of discovery, research, decision-making, system design, and thought-experiment validation. It produces a complete design artifact that any competent team can implement.

### Phase Model

Phase 0: Intent Capture — What are we building? Why? For whom? Hard constraints? Explicitly out of scope? Output: intent.md, constraints.md

Phase 1: Keystone Analysis — List all open questions. Map dependencies. Find the keystone (most-constrained node). Resolve keystone first. Output: open-questions.md, decisions.md (first entry)

Phase 2: Parallel Research — For each unresolved question: is it research or design? Fire parallel agents for independent research. Each agent writes to research/ file. Update INDEX.md as files land. Output: research/*.md files

Phase 3: Philosophy Establishment — What design philosophy drives module boundaries? Who is the "user" of this design? (developer, agent, end-user?) What's the module boundary philosophy? Let the count emerge. Output: framing for SYSTEM.md

Phase 4: System Design — Define modules based on philosophy. Contracts between modules (inputs, outputs, guarantees). Data flows (walk through primary operations). Core vs plugin boundary. Cross-cutting concerns. Output: design/SYSTEM.md

Phase 5: Deep-Dive Thought Experiments — For each module: run 6 tests (happy path, failure, concurrency, scale, edge cases, contract verification). Strict template per module. Flag broken invariants. Propose fixes. Update SYSTEM.md if contracts changed. Output: design/deep/*.md files

Phase 6: Review and Lock — Cross-module consistency check. Present findings to human. Resolve open questions. Final approval. Output: Updated SYSTEM.md, locked decisions

### Agent Roles

Orchestrator (Prometheus): Manages phases, decides what to research, writes design docs
Researcher (Librarian/Explore): Deep research on specific topics, writes to files
Analyst (Oracle/Deep): Runs thought experiments, finds design flaws
Writer (Writing agent): Produces design documents from analysis

### Artifact Taxonomy

intent.md: What and why — stable after Phase 0
constraints.md: Hard boundaries — stable after Phase 0
open-questions.md: Unresolved questions — updated through Phase 2-6
decisions.md: Numbered decisions with rationale — grows through all phases
personas.md: Who uses this system — stable after Phase 0
research/*.md: Research deep-dives — written in Phase 2
design/SYSTEM.md: The system design — written in Phase 4, updated in Phase 5-6
design/METHODOLOGY.md: How to run thought experiments — written in Phase 5
design/deep/*.md: Module deep-dives — written in Phase 5
INDEX.md: Dashboard — updated after every change

### Decision Protocol

Every decision gets: An ID (D1, D2, ...), Status (accepted/deferred/discarded/superseded), Context (why this came up), The decision itself, Rationale (why this choice), Conflicts with (which other decisions), Enables (what this unblocks), Blocks (what this prevents), Research source (which research file backs it).

---

## Part 3: Persistent Home Session

### The Problem
AI sessions have limited context windows. Long projects exceed them. Current solutions (summarization, RAG, new sessions) lose the "feel" of the conversation — the shared understanding, the inside jokes, the "we tried that and it didn't work" knowledge.

### The Model: Human Episodic Memory
Human memory doesn't store transcripts. It stores: High-level patterns (abstractions), Key decisions and why they were made, Recent events in detail, Emotional valence (what felt important).

The Persistent Home Session mimics this: Early phases deeply compressed into abstractions and patterns, Recent phases preserved with detail, Journey arc maintained as a narrative, Philosophy and working style always fresh.

### Session Topology

Home Session contains: Compressed History (Phase 0-3 compressed to 1-10 lines each), Fresh Context (Phase 6 and current work in full detail), Always Loaded (Philosophy + Working Style + File Index + Active tasks).

Worker sessions branch off from Home, do work (deep-dives, research, implementation), write results to files. Home reads the files and integrates.

### Compression Algorithm Sketch

For each phase in session history: if age > threshold_old, compress to 1-3 lines (pattern + decision + lesson). If age > threshold_recent, compress to key points with some detail. Else, preserve with light compression. Always append: philosophy, working style, file index, next steps.

### Home to Worker Contract

Worker sessions MUST: Read INDEX.md first. Read relevant design/research files. Write outputs to files (not chat). Follow artifact taxonomy. Update INDEX.md.

Home session MUST: Read updated files after worker completes. Integrate findings. Present to human at right abstraction level. Never lose the journey arc.

### File-as-State Contract

Files are the single source of truth. Chat is working memory — disposable between sessions. Every significant output goes to a file. Every file is self-contained. INDEX.md is always current. Decisions are numbered and cross-referenced.

---

## Part 4: Architecture Sketch

Components: Phase Manager, Dependency Analyzer, Research Dispatcher, Design Engine, Thought Experiment Runner, Compression Engine, File Manager.

Key innovations: (1) Conceptual compression — mirrors human episodic memory, not textual summarization. (2) Home + Worker topology — persistent home with ephemeral workers, novel for AI systems. (3) Design as artifact — optimizes for design quality, not code generation. (4) Structured methodology enforcement — methodology-as-code for design work.

---

## Part 5: Open Questions

1. How to automate compression quality? SESSION-HANDOFF.md was hand-crafted.
2. Hierarchical index for long projects. Even compressed, 6-month projects accumulate context.
3. Worker-to-Home write protocol. Standardized format to prevent format drift.
4. Relationship to existing agents. New agent type or methodology for existing agents?
5. Completeness criterion. How do you know the design is "done enough"?
6. Multi-human projects. What changes when multiple humans contribute?
7. Design evolution. How to handle requirement changes after Phase 6 (lock)?
8. Tool integration. Files, Figma, Git, Linear, Notion?
9. Compression granularity. How much recent context to preserve?
10. Bootstrapping. What's the "hello world" of the Design Builder?

---

## The Vision

The Design Builder is not a tool. It's a new way of building software.

Current paradigm: Human has idea -> writes code -> tests code -> fixes code -> ships code.
Design Builder paradigm: Human has idea -> designs with AI -> validates design through thought experiments -> generates code from validated design -> ships.

The design is the source code. The code is the compiled output.

The Persistent Home Session is the environment where this happens — a living workspace that remembers the journey, compresses the past, and keeps the present fresh.

This is the seed. Plant it.
