# Explore Agent: 05-body-metaphor-capabilities

Session: `ses_239f4bf10ffeVOFt3oZaCz65hl`

---

Now I have complete coverage of the session. Let me compile the comprehensive UX/design extraction.

<results>
<files>
- /Users/samanvayayagsen/project/sessions/sessions/oss/claude/ses_24994c99bffeSpwllQcb6IB4qj.jsonl - The primary session file (506KB, 193 messages)
</files>

<answer>

# UX & Interaction Design Extraction: AgentBodies Design Session (Apr 22, 2026)

> **Session**: `ses_24994c99bffeSpwllQcb6IB4qj.jsonl` | 193 messages | 496KB | 9 AgentBodies mentions
> **Project**: Mesh — Portable Agent-Body Runtime (later branded around "AgentBodies")
> **Session character**: A pure design/intent session — explicitly NOT a coding session. No code was written. The entire session is about understanding what to build before building it.

---

## 1. THE "BODY" METAPHOR — What IS an Agent Body?

This session establishes the foundational metaphor for the entire product. The "body" concept is central to both the technical architecture and the UX.

### Core Definition (from `intent.md`, read at session start)

> **"A portable agent-body runtime. Gives an AI agent a persistent, elastic compute identity (a 'body') that can live on any substrate — always-on VM, shared-tenant fleet, ephemeral sandbox — and move between them without losing itself."**

> **"The body is a filesystem. An agent installs packages, writes files, modifies config. The body is the sum of all that state, portable as an OCI image + volume tarball. Where it physically runs (the 'form') is a cost/latency knob, not an architectural commitment."**

### The Body vs. Form Abstraction

The design establishes a critical UX metaphor:
- **Body**: permanent identity + filesystem state. Persists across substrate changes. The "who" of the agent.
- **Form**: current physical instantiation on a specific substrate. Ephemeral by nature. The "where" of the agent.
- **Substrate**: where a form runs. Three pools: Local (laptop/Pi), Fleet (BYO VMs via Nomad), Sandbox (Daytona, E2B, Fly, Modal, Cloudflare).

### How the Interface Communicates Body State

The system design defines a **state machine** for body lifecycle:
```
Created → Starting → Running → Stopping → Stopped → Destroyed
```

**Key UX principle**: Scheduling DECISIONS are made by the caller (skill/user), not by the system. The system provides capabilities; the user composes them into features.

> **"We are not making any decisions for the user. We are making capabilities easy enough to choose from. I am not going to decide does the container live on the fleet or on the [sandbox]. Maybe someone wants to snapshot it and save it, or someone wants to kill it and save it."** (User, Line 86)

---

## 2. INTERFACE DESIGN — MCP + Skills as Primary Interface

### The Primary Interface: MCP Server + Skills

**Decision D5 (accepted)**: MCP + skills as primary user interface (NOT CLI).

The interface module is designed with these principles:
- **Key tools**: `mesh.body.create`, `mesh.body.snapshot`, `mesh.body.migrate`, `mesh.provisioner.list`, `mesh.plugin.install`
- **Contract**: Receives commands from agents/skills via MCP tools. Returns body states, operation results.
- **Owns**: How external systems talk to Mesh
- **Does NOT know**: How bodies are provisioned, how snapshots work, how networking is configured
- **Error translation**: Translates structured errors to user-friendly MCP responses

### Skills as the "New Interface"

> **"The skill is the new interface, and we have to bet 100% on it only... they don't even have to do a pip install mesh. Everything should be handled by the skill."** (User, Line 86)

### CLI's Role: Backup/Debug Interface

Research into MCP architecture patterns revealed a recommended approach:

> **"Build good CLIs first, then wrap them as MCPs. Good CLIs are multi-interface. Usable from shell. Scriptable. Composable with pipes. Testable standalone. MCP-first locks you to the MCP protocol. CLI-first gives you flexibility."** — Robert Melton (cited in mcp-architecture.md)

**Pattern decided**: `CLI (Business Logic) → Thin MCP Wrapper (Schema + Protocol)`

### Language Decision for Interface

Research showed Go is first-class for MCP (official Go SDK, GitHub MCP server uses it). Decision: Go all the way. CLI and MCP both wrap the same Go core library.

> **"Go is not just possible for MCP—it's a first-class citizen with official SDK support and major production users (GitHub, Gopls). The 'TypeScript is de facto standard' narrative is outdated as of 2026."** (mcp-architecture.md verdict)

---

## 3. USER FLOW DESIGN — Three Core Operations

The system defines exactly three core operations that a user performs. These are the complete interaction surface:

### Flow 1: Create and Run a Body
```
1. Skill calls Interface: mesh.body.create(spec)
2. Interface calls Orchestration: BodyManager.Create(spec)
3. Orchestration calls Provisioning: Provisioner.Provision(spec) → substrate handle
4. Orchestration calls Networking: Network.AssignIdentity(bodyId) → network identity
5. Orchestration returns Body (with handle + identity) to Interface
6. Interface returns body info to skill
```

### Flow 2: Snapshot a Body
```
1. Skill calls Interface: mesh.body.snapshot(bodyId)
2. Interface calls Persistence: SnapshotEngine.Capture(bodyId)
3. Persistence calls Orchestration to get body handle
4. Persistence executes: pre-prune → docker export → zstd compress
5. Persistence calls StorageBackend.Put(snapshot) → stores to user's configured backend
6. Persistence returns SnapshotRef to Interface
7. Interface returns snapshot info to skill
```

### Flow 3: Migrate a Body (Cold Migration)
```
1. Skill calls Interface: mesh.body.migrate(bodyId, targetSubstrate)
2. Interface orchestrates the sequence:
   a. Orchestration.Stop(bodyId) — graceful stop
   b. Persistence.Capture(bodyId) → SnapshotRef — export FS
   c. Provisioning.Provision(spec on targetSubstrate) → new handle
   d. Persistence.Restore(SnapshotRef, newBodyId) — import FS
   e. Networking.AssignIdentity(newBodyId) — reassign identity (same name, new IP)
   f. Orchestration.Start(newBodyId) — bring up on new substrate
   g. Provisioning.Destroy(old handle) — clean up old substrate
3. Interface returns migrated body info to skill
```

### Transport as Coordination, NOT a Module

> **"Transport is NOT a module. It's a coordination script that calls modules 2-5 in sequence."** (SYSTEM.md, line 119)

This is a key design principle — the migration flow is a workflow, not a component. The user doesn't need to know about stop → snapshot → provision → restore → start individually; the skill orchestrates this.

---

## 4. DESIGN PHILOSOPHY — Capabilities Over Features

This is the most significant philosophical discussion in the session, with direct UX implications.

### The Capabilities-vs-Features Principle

> **"What we do wrong is, we normally take use case, we think of features, and then we start building features. But that is a wrong philosophy. First, we have to build capabilities. Features are just a grouping of capability, choices made in an order, so that you don't have to."** (User, Line 86)

> **"So today, our sole aspect is to build those capabilities... if you see the tree from the top, then as you build layers and layers and layers, then all the features would automatically be just a combination."** (User, Line 86)

### Philosophy-First Module Boundaries

> **"The system shouldn't be designed from a user's perspective. That is totally wrong. That is the experience. The system should be designed from how I am going to maintain and scale it... The system should be designed so that me and you, you are going to write the code. I am going to decide and design the things."** (User, Line 98)

**This is a deliberate UX philosophy choice**: The developer/agent who maintains the system is the "user" of the design, not the end user. The UX for end users comes from composing capabilities, not from designing features.

### The Abstraction Layer Principle

> **"As a human, I have evolved to only hold six to seven abstract ideas, but my evolution is about how complex abstractions I can make. You [AI] can hold forty, fifty abstract ideas, but it's my job to be sitting on a layer above you and you pushing me to sit above with one layer so that your fifty, sixty abstracts get into my five, six abstracts only."** (User, Line 99)

---

## 5. COMPETITOR/DISCOVERY RESEARCH — Competitor UIs

### Daytona (Rejected)

**Daytona MCP Interface** (studied as competitor research):
- **MCP Tools**: Sandbox management (create, destroy), file operations (upload, download), Git operations, command execution, computer use, preview link generation
- **Three-tier lifecycle**: running/stopped/archived with distinct costs
- **Configuration**: `daytona mcp init [claude|cursor|windsurf]` → `daytona mcp start`
- **REST API + SDKs**: Python, TypeScript, Ruby, Go

**Why rejected**: Resource mismatch (8-16GB RAM vs 2GB target), monolithic control plane, no body abstraction (workspaces are platform-bound), AGPL license. Fundamentally different market position ("Heroku for AI code execution" vs "Nomad + Docker + Tailscale for agent bodies").

### E2B (Studied)

- Firecracker mem+FS snapshots, create/pause/resume/kill lifecycle
- $0.0504/vCPU-hr billing
- Snapshots spawn many from one point
- **Key finding**: Bug where repeated pause/resume cycles lost FS deltas after first resume

### Daytona's Provider Plugin Pattern (Borrowed)

- Uses HashiCorp go-plugin for provider extensibility
- Configuration schema with target options, validation, UX enhancement suggestions
- Provider capabilities: compute provisioning, network configuration, storage management, lifecycle management

---

## 6. THE SIX MODULES AS UX COMPONENTS

The session produces a complete system decomposition into 6 modules, each with clear UX implications:

| Module | UX Role | Key Interface |
|--------|---------|---------------|
| **Interface** (MCP + Skills) | Entry point for all user interaction | `mesh.body.*` tools |
| **Provisioning** (Provider Plugins) | How users bring their own compute | `Provisioner.Provision(spec)` |
| **Orchestration** (Body Lifecycle) | Body state machine — what "state" the body is in | `BodyManager.Create/Start/Stop/Destroy` |
| **Persistence** (Snapshot + Storage) | How body state is captured and restored | `SnapshotEngine.Capture/Restore` |
| **Networking** (Tailscale + Identity) | How bodies get persistent network identity across substrate changes | `Network.AssignIdentity` |
| **Plugin Infrastructure** | How the system extends itself without user needing core changes | `PluginRegistry.Discover/Load` |

### Core vs Plugin Boundary (UX Impact)

- **Core** (always present): Orchestration, Interface, Networking, Plugin Infrastructure
- **Plugin** (user installs what they need): Provisioning providers, Storage backends, Scheduler policies

**Key principle**: If adding support for a new cloud/sandbox requires changing core code, the boundary is wrong.

### The Plugin UX

> **"I don't want to maintain... this core product won't maintain a number of providers in the main repo. That is a bloat. We would maintain a separate repo of plugins... if someone, some user comes up and he wants [a new provider], then maybe the skill can check the plugin repo. If it doesn't have a plugin for the user, then it can build one. And it would be extensively using the Pulumi skill inside of it."** (User, Line 86)

---

## 7. ERROR HANDLING & COMMUNICATION DESIGN

### Structured Error Types

Errors bubble up as structured types with gRPC status codes:
- `UNKNOWN`, `INSTANCE_NOT_FOUND`, `INVALID_STATE`, `INSUFFICIENT_RESOURCES`
- `NETWORK_ERROR`, `AUTHENTICATION_ERROR`, `NOT_SUPPORTED`, `TIMEOUT`
- `QUOTA_EXCEEDED`, `RATE_LIMITED`

The Interface module translates these to user-friendly MCP error responses. Plugins distinguish retryable errors (rate limits, network) from fatal errors (auth, quota).

### Configuration UX

- Single config file (YAML) with sections per module
- No module reads another module's config
- CLI provides `mesh config set <module>.<key> <value>`

---

## 8. SESSION CONTINUITY UX — The "Home Session" Vision

This is a meta-UX concept that emerged toward the end of the session — a vision for how AI-human design collaboration should work.

### The Problem with Current Summarization

> **"What people are doing wrong is they're summarizing, like summarize all the entire thing in this chat and then they are going for new and new sessions and then wondering what the fuck is happening... over a time of the chat, you develop some kind of a character or some kind of a, like, you start feeling human in the sense that you get things. It's all because of the context."** (User, Line 148)

### The "Smart Compression" Model

> **"The summary should not be based on text. It should be based on what is in the text... The upper parts should be deeply summarized and the lower parts which we are right now talking should be kept intact, in fact."** (User, Line 148)

This produced the **SESSION-HANDOFF.md** — a conceptual compression document with these properties:
- Early context → deeply compressed into abstractions (like human memory)
- Recent context → preserved with detail
- Journey arc matters more than transcript
- Philosophy and working style preserved
- Ends with "Welcome home."

### The "Home Session" Vision

> **"There should be one home session where the story from the start is there in maybe few like highly abstracted lines, but it should start to feel like your home, your agent, and you should start, that chat should start feeling like that you have built it from scratch because you know everything right from the start."** (User, Line 148)

Two products envisioned:
1. **Design Builder**: Takes a project from "I have an idea" to "I have a complete, tested design ready for implementation"
2. **Persistent Home Session**: One session that lives for the lifetime of a project, with smart conceptual compression

---

## 9. THE DESIGN BUILDER — Meta-UX for AI-Human Collaboration

Seven patterns extracted from this session form the Design Builder methodology:

| Pattern | Description | UX Implication |
|---------|-------------|----------------|
| **P1: Keystone Resolution** | Circular dependencies have a most-constrained node; find and resolve it first | Users stop going in circles |
| **P2: Dependency-Aware Parallelism** | After keystone breaks, fire parallel agents for independent questions | Research happens N× faster |
| **P3: Philosophy-First Module Boundaries** | Draw modules where an agent can work independently | Developer UX — bounded context |
| **P4: Thought Experiments Before Implementation** | 6 structured tests per module | Design flaws found before code |
| **P5: File-Based Persistence** | Files are persistent state; chat is working memory | Sessions are disposable |
| **P6: Conceptual Compression** | Early context → abstractions; recent → detail | "Welcome home" feeling |
| **P7: Research-First, Decide-Second** | Every decision backed by a research file | No premature decisions |

### The Vision Statement

> **"The Design Builder is not a tool. It's a new way of building software. Current paradigm: Human has idea → writes code → tests code → fixes code → ships code. Design Builder paradigm: Human has idea → designs with AI → validates design through thought experiments → generates code from validated design → ships."**
> **"The design is the source code. The code is the compiled output."** (DESIGN-BUILDER.md)

---

## 10. DIAGRAMS & VISUAL DESIGN

Two diagram approaches were produced (in draft, left for review):

1. **Module Diagram (diagram-v1.md)**: Traditional box-and-arrow Mermaid flowchart showing all 6 modules with core vs plugin boundary as visual separator
2. **Flow-Based Diagram (diagram-v2.md)**: Sequence diagram showing the Migrate flow (most complex — touches all modules) + ASCII flow matrix showing all three operations as swim lanes

**Key insight from diagrams**: "Migration is the archetypal flow. If you understand migration, you understand the system."

---

## 11. ALTERNATIVES CONSIDERED AND REJECTED

| Alternative | Why Rejected |
|-------------|--------------|
| **Daytona as substrate** | 8-16GB RAM, monolithic, AGPL, no body abstraction |
| **User-perspective design** | "The system shouldn't be designed from a user's perspective" — design for maintainers/agents instead |
| **Transport as a module** | Transport is coordination, not a bounded context |
| **K8s-based orchestration** | Hard constraint C2: no K8s ever |
| **TypeScript for MCP** | Go is first-class for MCP per research; TypeScript narrative outdated |
| **MCP-first (no CLI)** | CLI-first pattern recommended by practitioners; MCP wraps CLI |
| **Feature-based design** | Capabilities-first; features are compositions of capabilities |
| **Text-based summarization** | Conceptual compression preserves intent, not just text |
| **Memory state snapshots (CRIU)** | Kernel/CPU-coupled, non-portable; filesystem-only is the right abstraction |
| **Hot/live migration in v0** | Deferred — cold migration only for initial release |
| **Maintaining providers in core repo** | Plugins in separate repo; Pulumi skill generates new ones |

---

## 12. OPEN DESIGN QUESTIONS (UX-Impacting)

From SYSTEM.md, these remain unresolved and directly affect UX:

1. **How does a skill authenticate with Mesh?** (API key? Tailscale identity? No auth for local?)
2. **What's the minimum viable system for v0?** (Which modules can be deferred?)
3. **What's the body identity format?** (UUID? UUID + human-readable name? Include substrate info?)
4. **Hot/suspend-resume as optional optimization in v0?**
5. **How do we handle orphaned bodies?** (Garbage collection policy?)

---

## 13. KEY QUOTES — User's Design Vision

**On the design process:**
> "This is not a coding session. This is just intent to design session where we are trying to first define what we have to build... This imagination or like a design in our head so that without writing even a single line of code, first we understand that naturally with the given specs, is the system would work or not work."

**On capabilities vs features:**
> "First, we have to build capabilities. Features are just a grouping of capability, choices made in an order, so that you don't have to."

**On abstraction:**
> "As a human, I have evolved to only hold six to seven abstract ideas... you can hold forty, fifty abstract ideas, but it's my job to be sitting on a layer above you... that builds a picture in my mind."

**On the body metaphor:**
> "The body is a filesystem. An agent installs packages, writes files, modifies config. The body is the sum of all that state... Where it physically runs (the 'form') is a cost/latency knob, not an architectural commitment."

**On design as artifact:**
> "When you're building with AI, code is like compiled C. The truth thing lies in the intent and the design... The moat lies in designing this part, what we are doing right now."

**On peer collaboration:**
> "Don't think of me as your master. We are peers... If confidence < 85%, say so... If I am confused, then I would tell you. If you are confused, you can tell me."

</answer>

<next_steps>
This extraction covers ALL UX/interaction design discussions from the session. For your downstream compilation:

1. **Highest-value documents produced by this session** (these ARE the UX artifacts):
   - `discovery/design/SYSTEM.md` (211 lines) — Complete system design with interface contracts
   - `discovery/SESSION-HANDOFF.md` (279 lines) — Conceptual compression with user working style
   - `discovery/design/METHODOLOGY.md` (199 lines) — Thought experiment playbook
   - `discovery/design/drafts/DESIGN-BUILDER.md` (163 lines) — Meta-UX vision
   - `discovery/research/mcp-architecture.md` — MCP interface research and CLI-first pattern
   - `discovery/research/daytona-analysis.md` — Competitor UI analysis

2. **This session has NO discussions about**: Preview Render patterns, chat-first interface layouts, canvas/visual elements, or traditional UI components. The entire "interface" discussion is about the MCP/skill API surface, not a graphical UI.

3. **Gap to fill from other sessions**: The "Preview Render confirmation pattern" and "chat-first interface" mentioned in your request are NOT in this session — those may be in other AgentBodies sessions that discuss the visual/layered UI for body management.
</next_steps>
</results>