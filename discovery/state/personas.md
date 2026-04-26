# Agents — User Personas and Behaviors

> Who uses Mesh, what they run on it, and how they behave.
> Every design decision should be validated against at least one agent persona.

---

## A1: Hermes Operator

- **Type**: Heavy persistent (24/7)
- **What it does**: Self-reflective AI agent. Runs a continuous while-loop. Processes scheduled jobs (daily reflection, research tasks) and on-demand requests (user prompts via chat).
- **Resource profile**: 2-4GB RAM, moderate CPU. Runs for days/weeks without restart.
- **State pattern**: Writes to filesystem constantly — conversation logs, memory files, tool outputs, downloaded models. State grows over time.
- **Lifecycle**: Started once, runs continuously. Stopped gracefully (Ctrl+C / SIGTERM) between tasks. Restarts from clean boot + restored filesystem.
- **Substrate expectation**: Fleet VM (always-on, user pays for uptime). Could burst to Sandbox for heavy tasks.
- **Key behaviors**:
  - Self-reflection loops on a schedule
  - May clone itself for parallel research tasks
  - Installs packages (pip, npm) during runtime
  - Accumulates long-term memory files
- **What Mesh must do for this agent**: Keep it running reliably. Snapshot filesystem periodically. Allow graceful stop/start. Provide a way to burst compute-heavy subtasks.

---

## A2: Tool Agent (Go/Rust binary)

- **Type**: Lightweight persistent
- **What it does**: Single-purpose tool exposed as a service — web scraper, code linter, file converter, API gateway. Listens on a port, processes requests, returns results.
- **Resource profile**: 50-256MB RAM, low CPU. Runs indefinitely but idle most of the time.
- **State pattern**: Minimal. Maybe a config file, a small SQLite DB, a queue directory. State changes rarely.
- **Lifecycle**: Started once, runs forever. Rarely needs restart. Can be stopped and started without data loss.
- **Substrate expectation**: Fleet VM (shared with other agents) or Sandbox (if cost-optimized). Deflate candidate — shrink to minimum when idle.
- **Key behaviors**:
  - Processes requests on demand
  - Mostly idle between requests
  - Low state mutation
  - Doesn't install anything at runtime
- **What Mesh must do for this agent**: Pack multiple instances on one VM. Deflate when idle to make room for others. Quick restore when a request arrives.

---

## A3: Ephemeral Task Runner

- **Type**: Ephemeral (one job then dies)
- **What it does**: Spun up for a single task — run a test suite, process a dataset, generate a report, execute a code review. Completes and exits.
- **Resource profile**: Variable — 512MB to 16GB+ depending on task. May need GPU.
- **State pattern**: Starts from a known base image. Writes results to filesystem during execution. On completion, results are collected (pushed to storage or returned via MCP). Container is destroyed.
- **Lifecycle**: Created → runs task → outputs result → destroyed. Minutes to hours of lifetime.
- **Substrate expectation**: Sandbox (Daytona, E2B, Fly). Pay per-second. No need for persistence after task completes.
- **Key behaviors**:
  - May download large dependencies at start (HF models, datasets)
  - Produces output artifacts that must be collected
  - No need for filesystem persistence after task
  - May need specialized hardware (GPU, high RAM)
- **What Mesh must do for this agent**: Fast spawn on capable substrate. Collect output before destroy. Bill accurately. Clean up completely.

---

## A4: Burst Clone

- **Type**: Ephemeral fork of a persistent agent
- **What it does**: A persistent agent (A1) clones itself to handle a heavy subtask — parallel research, data processing, batch inference. Clone runs the task, returns results to parent, dies.
- **Resource profile**: Same as parent or larger (upgraded substrate for the burst).
- **State pattern**: Starts with parent's filesystem snapshot. May modify it during task. On completion, relevant changes are optionally merged back to parent. Clone's state is then discarded.
- **Lifecycle**: Cloned from parent → runs task → returns result → destroyed. Minutes to hours.
- **Substrate expectation**: Sandbox (powerful, ephemeral). Parent stays on Fleet. Clone bursts to wherever is cheapest/fastest.
- **Key behaviors**:
  - Inherits parent's installed packages and accumulated state
  - May or may not need to merge filesystem changes back
  - Merge decision is user-dependent (some want it, some don't)
  - Parent remains running and unaffected during burst
- **What Mesh must do for this agent**: Snapshot parent FS → spawn clone on Sandbox → collect result → optionally merge FS delta → destroy clone. The merge step is the hard part — must be opt-in, not default.

---

## A5: Developer Agent (on laptop)

- **Type**: Local persistent
- **What it does**: Claude Code, Cursor agent, Codex — runs on the developer's machine. Edits code, runs tests, manages the repo. The "daily driver."
- **Resource profile**: 1-4GB RAM. Uses the laptop's filesystem directly.
- **State pattern**: Heavily stateful — project context, tool configurations, session history. State lives on the laptop.
- **Lifecycle**: Started/stopped by the developer as they work. Not 24/7.
- **Substrate expectation**: Local (laptop). May occasionally need to burst to Sandbox for heavy tasks (large test suite, model inference).
- **Key behaviors**:
  - Deep integration with local filesystem and tools
  - Needs low-latency access to project files
  - May need to offload heavy compute occasionally
- **What Mesh must do for this agent**: Provide a local runtime option. Enable burst-to-sandbox for heavy tasks. Don't interfere with the agent's direct filesystem access.

---

## Agent × Substrate Matrix

| | Local (laptop) | Fleet (BYO VM / Nomad) | Sandbox (Daytona, E2B, Fly) |
|---|:---:|:---:|:---:|
| **A1: Hermes** | dev/testing | primary | burst |
| **A2: Tool Agent** | dev/testing | primary (packed) | cost-optimized |
| **A3: Task Runner** | — | — | primary |
| **A4: Burst Clone** | — | — | primary |
| **A5: Dev Agent** | primary | — | burst |

---

## Agent × Feature Need Matrix

| | Graceful stop/start | Periodic snapshot | Clone + merge | Burst to sandbox | Pack multiple on VM | Deflate when idle |
|---|:---:|:---:|:---:|:---:|:---:|:---:|
| **A1: Hermes** | yes | yes | yes | yes | no | no |
| **A2: Tool Agent** | yes | optional | no | no | yes | yes (D8) |
| **A3: Task Runner** | no | no | no | no | no | no |
| **A4: Burst Clone** | no | inherits | yes | yes | no | no |
| **A5: Dev Agent** | yes | optional | optional | yes | no | no |
