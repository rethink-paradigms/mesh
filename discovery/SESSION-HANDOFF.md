# SESSION-HANDOFF — Mesh Design Journey

**What this is**: A conceptual compression of the design session. Not a transcript summary. This captures the journey arc, key decisions, and current state so a new session feels like it was there from the start.

---

## The Journey Arc

### Phase 1: Breaking the Circle

We started stuck. Five open questions (Q1-Q5) forming a circular dependency web. No progress possible.

Root cause: Q5 (Daytona) was the keystone. Everything else depended on it. We went round and round because we couldn't answer Q5 first.

Solution: Pick a starting point. Break the circle. Research Q5 in isolation, then work through the rest in dependency order.

**Lesson learned**: When facing circular dependencies, pick the most constrained node. Resolve it, then cascade through the web.

---

### Phase 2: Daytona Decision

Researched Daytona OSS architecture. Found fundamental incompatibility with Mesh's constraints:

- **Resource mismatch**: Daytona assumes 8-16GB RAM. Mesh assumes 2GB VMs (Nomad on cheap fleet).
- **Architecture mismatch**: Daytona is monolithic control plane. Mesh needs distributed coordination.
- **No body abstraction**: Daytona doesn't separate identity from physical instantiation.
- **AGPL license**: Conflict with permissive preference.

Decision D10 recorded: Mesh is separate from Daytona. Q5 resolved.

**Files produced**: research/daytona-analysis.md

---

### Phase 3: Parallel Research Sprint

With Q5 resolved, we saw a window. Four research questions were now independent. No dependencies between them.

Launched 4 librarian agents simultaneously:

1. Substrate adapter interface → research/substrate-adapter.md
2. E2B internals → research/e2b-internals.md
3. Snapshot mechanics → research/snapshot-mechanics.md
4. Registry strategy → research/registry-strategy.md

All completed. Total: 8 research files now exist in discovery/research/.

**Lesson learned**: When dependencies disappear, exploit parallelism. Research is embarrassingly parallel when constraints are clear.

---

### Phase 4: The Philosophy Shift

This was the turning point. User made a critical insight:

> "The system should NOT be designed from end-user perspective. It should be designed from the DEVELOPER/AGENT perspective — the people and agents who build and maintain it."

This reframed everything.

Instead of asking "what should the user experience?", we asked "where are the natural boundaries where someone could work independently?"

This led to **bounded context philosophy**: Modules are drawn where an agent can work alone, knowing only contracts, not internals.

Six modules emerged from this philosophy. We didn't target six. We let the philosophy reveal the natural boundaries.

**Modules**:
1. Interface (MCP + Skills)
2. Provisioning (Provider Plugins)
3. Orchestration (Body Lifecycle)
4. Persistence (Snapshot + Storage)
5. Networking (Tailscale + Identity)
6. Plugin Infrastructure (Discovery + Loading)

**Transport was explicitly NOT a module**. Why? It's coordination (stop → snapshot → provision → restore → start), not a bounded context. No independent work to do there.

**Lesson learned**: Design for the builders, not the users. Let boundaries emerge from independent workability, not architectural abstraction.

---

### Phase 5: System Design Document

Produced SYSTEM.md (211 lines) — the skeleton of the entire system.

Six modules with contracts, data flows (Create, Snapshot, Migrate), core vs plugin boundary. Cross-cutting concerns mapped (errors, logging, config, testing, security). Constraints linked to decisions (C1-C6).

Ten open design questions surfaced. Not resolved yet. Recorded for future deep-dives.

**Lesson learned**: A system skeleton is more valuable than detailed specs. It exposes the right questions to ask.

---

### Phase 6: MCP Architecture Decision

Research showed Go is first-class for MCP. Official Go SDK exists. GitHub uses it for their MCP integration.

**Decision**: Go all the way. CLI and MCP both wrap the same Go core library. No TypeScript needed.

CLI-first pattern recommended by practitioners. Skills (MCP servers) are the primary interface. CLI is a convenience wrapper.

**Files produced**: research/mcp-architecture.md

**Lesson learned**: When a first-class tool exists, bet on it. Don't fight the ecosystem.

---

### Phase 7: Diagram Drafts

Two diagram drafts produced in design/drafts/. Not yet integrated into SYSTEM.md. Left for user review.

- drafts/diagram-v1.md — box-and-arrow module diagram
- drafts/diagram-v2.md — flow-based migration sequence

**Lesson learned**: Visuals help intuition, but don't rush integration. Let the text design stabilize first.

---

### Phase 8: Deep-Dive Methodology

User's philosophy: "Code is compiled output. The design is the true artifact."

We must run thorough thought experiments on each module before implementation. Thought experiments are cheap. Code is expensive.

Produced METHODOLOGY.md (199 lines) — the playbook for deep-dives.

**Six tests per module**:
1. Happy Path
2. Failure at Each Step
3. Concurrency
4. Scale
5. State Machine Edge Cases
6. Contract Verification

**Strict template for each deep-dive output**. Consistency matters for cross-module reasoning.

**Execution order**: Orchestration → Provisioning + Networking (parallel) → Persistence → Interface. Plugin Infra whenever.

**Lesson learned**: Thought experiments before implementation. Methodology matters. Consistency enables cross-module reasoning.

---

### Phase 9: Session Continuity & Meta-Vision

User's insight: Compression should be conceptual, not textual. Human memory works this way. Early context becomes abstract patterns. Recent context retains detail.

User wants to build two meta-systems:
1. A design builder (like Prometheus, but for Mesh)
2. A persistent "home session" that survives context loss

This handoff document is an experiment in smart compression. If it works, future sessions will feel like "home."

Deep-dives have been started in a separate session. This session ended with handoff.

**Lesson learned**: Session continuity is hard but valuable. Compression should feel like human memory — abstract patterns early, detail later.

---

## Key Decisions Made This Session

| ID | Decision | Rationale |
|----|----------|-----------|
| D10 | Mesh separate from Daytona | Resource mismatch (8-16GB vs 2GB), no body abstraction, monolithic control plane, AGPL |
| Implicit | Language = Go all the way | Official Go SDK for MCP exists, used by GitHub. CLI + MCP both wrap same Go core. |
| Implicit | Module boundaries = bounded context | Design for developers/agents who build it, not end-users. Each module independently implementable. |
| Implicit | Transport is NOT a module | It's coordination (stop → snapshot → provision → restore → start), not a bounded context. |
| Implicit | Philosophy: capabilities, not features | Build capabilities. Features emerge from combinations of capabilities. Don't decide for the user. |
| Implicit | Skills are the interface | User installs a skill. Skill installs Mesh. Bet 100% on skills. Not CLI, not just MCP. |

---

## Current State of All Artifacts

### Discovery Files
- **INDEX.md** — Dashboard (8 decisions, 4 open questions, 6 constraints, 5 personas)
- **intent.md** — What we're building and why. Stable. Rarely changes.
- **decisions.md** — D1-D10 (8 accepted, 1 deferred, 1 discarded). D10 is the big one this session.
- **open-questions.md** — Q5 resolved. Q1-Q4 still open but reframed as capability questions.
- **constraints.md** — C1-C6 (non-negotiable). Hard boundaries.
- **personas.md** — A1-A5 agent types. Validate designs against these.

### Research (9 files, all complete)
- substrate-landscape.md — 11 systems compared
- agent-sandbox-k8s.md — K8s agent-sandbox analysis
- daytona-analysis.md — Daytona OSS deep dive (keystone resolution)
- e2b-internals.md — E2B Firecracker mechanics
- snapshot-mechanics.md — docker export, overlayfs, compression
- substrate-adapter.md — lifecycle verbs, compliance matrix
- registry-strategy.md — storage options and trade-offs
- plugin-architecture.md — plugin system design
- mcp-architecture.md — MCP patterns, Go SDK, CLI relationship

### Design (3 files + 2 diagram drafts)
- **SYSTEM.md** — 6-module system design (211 lines). The skeleton.
- **METHODOLOGY.md** — deep-dive playbook (199 lines). The execution plan.
- **drafts/diagram-v1.md** — box-and-arrow module diagram. Awaiting integration.
- **drafts/diagram-v2.md** — flow-based migration sequence. Awaiting integration.

---

## What's Next

- Deep-dives on all 6 modules (started in separate session)
- Diagram integration into SYSTEM.md (after deep-dive approval)
- Open questions Q1-Q4 to resolve after deep-dives reveal constraints
- Implementation planning only after deep-dives are approved

---

## The User's Working Style (important for future sessions)

1. **Peers, not master-servant.** Talk as equals. Be direct. If confidence < 85%, say so.
2. **Capabilities over features.** Don't build features. Build capabilities. Features are combinations.
3. **Code is compiled output.** Design is the true artifact. Thought experiments before implementation.
4. **Don't decide for the user.** Provide capabilities, let them choose.
5. **Philosophy drives structure, not numbers.** Define WHY, let the count emerge.
6. **Abstraction layers.** Human holds 6-7 abstractions. AI holds 40-50. AI's job is to compress to 6.
7. **Files are persistent state.** Chat is disposable. Design for session continuity through files.
8. **Smart compression, not dumb summary.** Early context → deep abstractions. Recent context → preserved detail.

---

## Open Design Questions (from SYSTEM.md, not yet resolved)

1. What language? (Likely Go, but not formally decided)
2. How are body configs persisted alongside FS snapshot?
3. What's the minimum viable system for v0?
4. How does a skill authenticate with Mesh?
5. Hot/suspend-resume as optional optimization in v0?
6. Volume backup handling?
7. Volume operations as optional substrate adapter methods?
8. Body identity format?
9. Orphaned body garbage collection?
10. Streaming logs and file transfers via gRPC?

---

## The Philosophy We're Working From

**Core abstraction**: Body (identity + filesystem) vs. Form (physical instantiation on a substrate). Body persists, form is ephemeral.

**Snapshot primitive**: `docker export | zstd` — flat filesystem tarball. No memory state. Fully portable. (D1, D2)

**Three substrate pools**: Local (laptop/Pi), Fleet (BYO VMs via Nomad), Sandbox (Daytona, E2B, Fly, Modal, Cloudflare).

**Primary interface**: MCP server + skills. Not CLI. (D5)

**No K8s. Ever.** Nomad on 2GB VMs. (D3)

**Provider integrations are plugins**, AI-generated, not maintained in core. (D6)

**Capabilities, not features.** We build primitive capabilities. Users compose them into features. We don't decide their use cases.

**Bounded context modules.** Each module is independently implementable by an agent who only knows contracts, not internals.

**Transport is coordination, not a module.** Stop → snapshot → provision → restore → start is a workflow, not a component.

**Skills are the interface.** Bet 100% on skills. Users install skills. Skills install Mesh. CLI is a convenience wrapper, not the primary interface.

---

## The "Home" Feeling

This is what it should feel like to read this handoff:

- You know the journey. You know why we made the decisions we made.
- You understand the philosophy that drives the design.
- You know the current state — what artifacts exist, what's done, what's open.
- You know the methodology — how to think about each module, what tests to run.
- You know the user's style — peers, not master-servant. Direct. Capabilities over features.

You're ready to work. You're not starting from zero. You're continuing a conversation that feels familiar.

That's the goal.

---

**This handoff is the first experiment in smart compression.** It may evolve. But this is the shape: conceptual, not textual. Human memory compressed to abstractions. Recent context preserved with detail. The journey arc matters. The philosophy drives everything. The artifacts are the persistent state.

Welcome home.
