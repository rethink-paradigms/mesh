# Explore Agent: 08-cli-tools-snapshot-ux

Session: `ses_239f75d95ffeRsOpmQx6kjNDQ4`

---

Now I have a complete picture of the session. Here is the comprehensive extraction:

<results>
<files>
- /Users/samanvayayagsen/project/sessions/sessions/mesh/claude/ses_244b59d36ffekk6TMTmB6CACFY.jsonl - The source session file (98 JSONL lines, 356KB)
- /Users/samanvayayagsen/project/rethink-paradigms/mesh/discovery/design/deep/interface.md - THE primary UX/interface design document (MCP tool surface, tiered tools, error UX, progress notifications, bootstrap flow)
- /Users/samanvayayagsen/project/rethink-paradigms/mesh/discovery/decisions.md - 10 architectural decisions including D5 (MCP as primary interface, not CLI) and D8 (inflatable containers, deferred)
- /Users/samanvayayagsen/project/rethink-paradigms/mesh/discovery/constraints.md - 6 hard constraints (C1-C6) that shape the UX
- /Users/samanvayayagsen/.gstack/projects/mesh/samanvayagsen-main-design-20260423-211421.md - The design doc from office-hours session (22KB, the strategic vision document)
- /Users/samanvayayagsen/.gstack/projects/mesh/ceo-plans/2026-04-23-mesh-v0-snapshot-restore-clone.md - CEO plan with scope decisions affecting interface
- /Users/samanvayayagsen/project/rethink-paradigms/mesh/discovery/design/deep/orchestration.md - Body lifecycle state machine (Created->Starting->Running->Stopped->Destroyed) - the UX states the user sees
- /Users/samanvayayagsen/project/rethink-paradigms/mesh/discovery/design/deep/persistence.md - Snapshot pipeline UX (progress streaming, crash-consistent vs clean snapshots)
- /Users/samanvayayagsen/project/rethink-paradigms/mesh/discovery/design/deep/provisioning.md - Plugin lifecycle states (Discovered->Loaded->Ready->Degraded->Crashed)
- /Users/samanvayayagsen/project/rethink-paradigms/mesh/discovery/design/deep/plugin-infrastructure.md - Plugin discovery/generation UX
- /Users/samanvayayagsen/project/rethink-paradigms/mesh/.sisyphus/plans/mesh-v0.md - The 13-task work plan with UX guardrails
</files>

<answer>

# UX/Interaction Design Extraction: Mesh v0 Session (Apr 23)

## IMPORTANT CONTEXT NOTE

This session is about **Mesh** -- an open-source infrastructure substrate for AI agent body management. It is the FOUNDATION layer that "AgentBodies" and "Agent Labs" would be built on top of. The session is a planning session (no code was written) focused on designing and planning the v0 implementation. The UX discussions here are about **how AI agents and developers interact with the Mesh system**, not about a graphical/web interface for end users.

---

## 1. PRIMARY INTERFACE PARADIGM: MCP + Skills (NOT CLI)

**Decision D5 (accepted):**
> "In 2026, nobody runs CLI commands manually. Agent builders interact via their coding agent (Claude Code, Cursor, etc.). CLI-first design assumes manual operation that doesn't match user behavior."

**The primary interface is MCP server + skills.** Users talk to their agent, the agent talks to Mesh via MCP. CLI exists as a thin debugging/automation surface, not the primary UX.

**Key quote from design doc:**
> "The target user interacts with Mesh through AI (MCP + skills), not through CLI commands or configuration files. The CLI exists as a bootstrap escape hatch only. No one installs providers by hand."

**Rationale:** Agents managing their own bodies (spawn, snapshot, burst) naturally call MCP tools. A CLI for this would be wrapping MCP calls anyway -- cut the middleman.

**Enables:** Recursive self-management (agents call MCP to manage their own bodies).

**v0 exception:** v0 is CLI-only (Stage 1). The full MCP interface is Stage 3 (post-v0). This is a deliberate staging decision: "Build the environment for growth, not the evolved form."

---

## 2. TIERED TOOL SURFACE (Interface Design from deep/interface.md)

The MCP tool surface is explicitly tiered to manage agent context window bloat:

### Tier 1 -- Always Loaded (agent gets these on connect):
- `mesh.body.create(spec)` -- Create and start a body
- `mesh.body.list(filter?)` -- List bodies
- `mesh.body.inspect(body_id)` -- Get body details
- `mesh.body.stop(body_id)` -- Stop a body
- `mesh.body.destroy(body_id)` -- Destroy a body permanently

### Tier 2 -- Discoverable (via `mesh.tools.discover`):
- `mesh.body.snapshot(body_id, opts?)` -- Snapshot body filesystem
- `mesh.body.migrate(body_id, target_substrate)` -- Cold migrate body
- `mesh.body.start(body_id)` -- Start a stopped body
- `mesh.provisioner.list()` -- List available substrate providers
- `mesh.plugin.install(name, source)` -- Install a plugin
- `mesh.plugin.list()` -- List installed plugins
- `mesh.network.get_endpoint(body_id)` -- Get body's network endpoint

### Tier 3 -- Admin (CLI-only or with admin flag):
- `mesh.config.set(key, value)` -- Set config value
- `mesh.plugin.generate(spec)` -- Generate a new provider plugin

**Key design reasoning:**
> "Every MCP tool definition consumes agent context window tokens. With 30+ tools, `tools/list` could be 5-10KB of JSON. Group tools into tiers... This keeps initial tool surface small (~2KB) while making full surface available."

---

## 3. INTERACTION PATTERNS

### 3a. Agent Self-Management via Natural Language
**The platonic ideal from the design doc:**
> "Every substrate machine runs a Mesh daemon. Agents call MCP tools to snapshot, clone, burst, and specialize themselves. The filesystem IS the state, and the state is portable. One agent becomes many. A generalist spawns a GPU specialist for 10 minutes, then absorbs the results."

> "An agent says 'clone me to the GPU machine' and Mesh does it."

### 3b. Bootstrap Flow (Two-Phase)
1. **CLI phase:** `mesh init` -- installs Mesh binary, generates config, starts Mesh daemon, prints MCP connection string.
2. **MCP phase:** Agent connects via MCP, uses `mesh.plugin.install` to add providers, `mesh.body.create` to spawn first body.

> "CLI is the bootstrap escape hatch. It must be minimal (~5 commands: `init`, `start`, `stop`, `status`, `config`)."

### 3c. Long-Running Operations with Progress
Snapshot and migration return immediately with `{ operation_id, status: "started" }`. Interface emits MCP `notifications/progress` with `progressToken`. Agent can poll via `mesh.operation.status(operation_id)`.

### 3d. Skills as Agent-Side Compositions
> "Skills are NOT part of Interface. Skills are agent-side compositions of MCP tools (e.g., a 'deploy' skill calls `mesh.body.create` + `mesh.network.get_endpoint`). Interface exposes tools; skills are consumer-side patterns."

From CEO plan: "Skills are first-class product artifacts. When building MCP server, write skills for it. When building CLI, write skills for it."

---

## 4. ERROR UX PRINCIPLES

### Error Translation (INV-3 from interface.md):
> "Error codes from internal modules (gRPC status) are never leaked raw to MCP callers. Interface always translates to human-readable error with actionable context."

Example mappings:
- `INSTANCE_NOT_FOUND` -> `"BODY_NOT_FOUND: Body '{id}' does not exist"`
- `INSUFFICIENT_RESOURCES` -> `"INSUFFICIENT_RESOURCES: Substrate '{name}' cannot fulfill {resources}. Available: {available}"`
- `TIMEOUT` -> `"TIMEOUT: Operation '{op}' timed out after {duration}. Body may be in transitional state."`

### v0 CLI Error UX (from Metis review guardrail):
> "v0 output = plain text to stdout, errors to stderr. No JSON output mode. No color. `--quiet` suppresses progress output. `--verbose` adds debug logging. That's it."

### Snapshot Error UX (from design doc):
> "Hook failure model: hook failure aborts the operation with a clear error. Hooks inherit the same timeout as the parent operation (30s default)."

---

## 5. BODY LIFECYCLE STATES (What Users See)

From orchestration.md -- the state machine that drives all user-facing states:

```
Created -> Starting -> Running -> Stopping -> Stopped -> Destroying -> Destroyed
                                                    \-> Error (unrecoverable)
```

Key UX guarantees:
- **INV-5:** No external caller observes intermediate states (Starting, Stopping). They see the pre-state or post-state.
- **INV-3:** Destroy is idempotent. Calling it on an already-destroyed body returns success, not error.
- **INV-4:** No orphaned resources after Destroy.

---

## 6. SNAPSHOT UX DISCUSSION: Clean vs Crash-Consistent

**Open Question OQ1 from interface.md -- explicitly discussed:**
> "Should `mesh.body.snapshot` default to clean (stop agent first) or crash-consistent?"

Options considered:
- (a) Default crash-consistent, `clean: true` option for migration
- (b) Always clean (matches D1 -- agents stop at task boundaries)
- (c) Agent decides via parameter

**Resolution in design doc:**
> "Agent personas suggest periodic snapshots should be crash-consistent (cheap), while migration snapshots should be clean (stop first)."

**v0 resolution:** The agent is **stopped** before snapshot (SIGTERM, wait for exit). The directory is then static. No concurrent writes, no partial files. This is the simpler path.

---

## 7. COLD MIGRATION AS CORE INTERACTION PATTERN

The entire user flow for moving agent bodies:

1. Agent calls `mesh.body.migrate({ body_id, target_substrate })`
2. 7-step sequence: stop -> snapshot -> provision new -> restore -> network -> start -> cleanup old
3. Either completes atomically or rolls back

**Critical UX finding from interface.md:**
> "Migration is not truly atomic. Steps c-g can fail leaving partial state." -> Design change: Interface must track migration state persistently (not just in-memory) so it can resume or rollback after a crash. Proposed: write a migration intent record to local state before starting."

---

## 8. V0 CLI COMMAND SURFACE (User-Facing Commands)

7 commands, deliberately plain:
- `mesh snapshot <agent>` -- stop, tar+zstd, hash, store, (optional) restart
- `mesh restore <agent>` -- verify hash, pre-flight, extract, start
- `mesh clone <agent> --target <machine>` -- snapshot + transfer + restore
- `mesh status <agent>` -- running state, last snapshot, cache usage
- `mesh inspect <snapshot>` -- manifest contents
- `mesh prune <agent> --keep N` -- remove old snapshots
- `mesh list [agent]` -- list all snapshots

**Key UX decision:** "Config is NOT auto-updated" after clone. User adds `[[agents]]` entry manually. This is intentional friction -- prevents accidental configuration drift.

---

## 9. DESIGN PRINCIPLES / METAPHORS DISCUSSED

### "The Filesystem IS the State" (Core Metaphor)
> "The filesystem IS the state, and the state is portable. One agent becomes many."
> 
> "AI agents are persistent, stateful processes. An agent writes files as it works, installs packages, accumulates memory. The filesystem IS the state."

This metaphor collapses the entire problem space: if you can move the filesystem, you can move the agent.

### "Build the Environment, Not the Evolved Form" (Growth Metaphor)
> "Build the environment for growth, not the evolved form. Like a cell growing because the environment (Earth) provides favorable conditions."

### "Cells Can Be Destroyed and Rebuilt" (Resilience Metaphor)
> "Small sections can be destroyed and rebuilt. The system doesn't collapse. Everything is easily addressable and replaceable."

### Three-Stage Staging Philosophy
1. Stage 1 (CLI) = v0 -- proves the core primitive
2. Stage 2 (Daemon) = wraps CLI, exposes local API
3. Stage 3 (MCP) = natural language interface, agent self-management

---

## 10. APPROACHES CONSIDERED AND REJECTED

### Rejected: Approach A (CLI-first minimal)
> "Snapshot/restore CLI. Ships in a weekend. Proves the core primitive." -- Rejected because it doesn't build toward the daemon+MCP vision.

### Rejected: Approach C (Nomad-native)
> "Build a Nomad CSI driver + job specification for stateful agent workloads." -- Rejected because it ties Mesh to Nomad, contradicting substrate-agnostic vision.

### Accepted: Approach B (Daemon + MCP, built in stages)
The three-stage approach ensures v0 proves the primitive while the architecture supports growth.

### Deferred: D8 Inflatable Container / PID-1 Supervisor
> "A sidecar binary at PID 1 that accepts `deflate` (shrink footprint) and `inflate` (restore) commands." -- Deferred because "cold migration via snapshot+restore covers 80% of the use case with 10% of the complexity."

### Rejected: JSON output, color, table formatting
> "v0 output = plain text to stdout, errors to stderr. No JSON output mode. No color."

### Rejected: Agent restart after snapshot
> "Agent management is optional (stop only, no auto-restart)"

---

## 11. USER'S FEEDBACK ON DESIGN DEEP DOCS (Second Message)

The user explicitly asked that the `discovery/design/deep/` folder be used as a reference for cross-validation:

> "There is a folder named deep inside of design, which is itself inside of discovery. That folder extensively has coverage of all the layers, verticals which are to be designed, and it covers the common pitfalls which can occur and a structure. So, I am not saying directly pick it, but yeah, it is a good point of analyzing your plan. Like if you have to design an interface, then when you design an interface, you can at least review, get it reviewed and match with what the deep interface has already been, what the deep has suggested, and then there could be a room for questioning and we can select whatever is best or a hybrid or something new, but it is a good reference point, so it should be actively used."

**This led to a significant plan edit** (line 72) adding a "Deep Design Cross-Validation" table comparing v0 against the full architecture across persistence, orchestration, and interface modules -- with explicit ALIGNED / DELIBERATE DEVIATION / SIMPLIFIED / DEFERRED annotations.

---

## 12. AUTH UX DISCUSSION

From interface.md EC6:
> - **API key in config** (simple, sufficient for self-hosted): Mesh generates a key on `init`, agents pass it as MCP header.
> - **Tailscale identity** (elegant): If Mesh runs on tailnet, Tailscale WhoIs authenticates the caller.
> - **No auth**: Only for local stdio transport. Dangerous for HTTP transport.

**Recommendation:** API key for v0 (zero complexity). Tailscale identity as v1 enhancement. Matches C4 (no login, no central dependency).

---

## 13. CONSTRAINTS SHAPING UX

- **C1: Must run on 2GB VMs** -- Limits UI complexity, forces streaming pipeline
- **C3: User owns all compute, keys, network** -- No Mesh-controlled auth, no hosted registry, no Mesh telemetry
- **C4: No telemetry, no login, no central dependency** -- No phone-home, no account system
- **C6: Core is tiny** -- Provider code is plugin, not core library; minimal surface area

</answer>

<next_steps>

## Assessment for Downstream Use

This session contains **infrastructure-level interface design** for Mesh, NOT a graphical/web UI for "AgentBodies." The UX discussions here are about:
1. How AI agents interact with the system (via MCP tools)
2. How the CLI provides a bootstrap/debugging surface
3. How the system presents state, errors, and progress to agent callers

**For your "AgentBodies" UX context document**, this session provides:
- The **conceptual model** (agent body = portable filesystem state)
- The **interaction paradigm** (MCP + skills as primary, CLI as bootstrap)
- The **tool surface design** (tiered discovery, progressive disclosure)
- The **body lifecycle states** that any UI would need to represent
- The **error presentation philosophy** (human-readable, actionable)
- The **growth metaphor** (build the environment, not the evolved form)

**What this session does NOT contain:**
- Chat-first interface design discussions
- Canvas/visual element discussions
- User flow wireframes or layout decisions
- End-user (non-developer) interface considerations
- Web or mobile UI discussions

You may want to search for additional sessions that contain "AgentBodies," "Agent Labs," or specific UI/UX design sessions that came AFTER this infrastructure planning session.

</next_steps>
</results>