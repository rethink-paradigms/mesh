# Migration Guide

This guide covers migrating from Mesh v0 to v1. v0 was a snapshot and clone tool for managing agent processes. v1 is a portable agent-body runtime with a daemon, MCP server, Docker integration, and SQLite store.

## Conceptual Changes

| v0 | v1 |
|----|----|
| Agents (per-machine processes) | Bodies (portable compute identities) |
| TOML config (`~/.mesh/config.toml`) | YAML config (`~/.mesh/config.yaml`) |
| SSH-based clone/transport | Docker-based lifecycle |
| JSON sidecar manifests | SQLite store + enhanced manifests (v2) |
| CLI-only interface | MCP server + CLI |
| No daemon | Long-running daemon process |

## What Was Removed

- `mesh clone` -- replaced by `mesh serve` + Docker lifecycle and migration coordinator
- `mesh status <agent>` -- removed, replaced by `mesh status` (daemon status only)
- Agent process management (pgrep-based lifecycle checks)
- SSH transport for machine-to-machine cloning
- Machine definitions in config (no more `[[machines]]` section)
- `stop_signal`, `stop_timeout`, `pid_file`, `pre_snapshot_cmd`, `post_restore_cmd` in agent config (hooks moved to runtime)

## What Was Kept

- `mesh snapshot` -- filesystem snapshot (tar + zstd + SHA-256), unchanged
- `mesh restore` -- restore from snapshot, unchanged
- `mesh list` -- list snapshots, unchanged
- `mesh inspect` -- show manifest, unchanged
- `mesh prune` -- remove old snapshots, unchanged
- JSON manifest files (enhanced with v2 fields)
- TOML config reading (for backward compatibility)
- Snapshot format and directory structure

## What's New

- `mesh init` -- initialize v1 YAML config
- `mesh serve` -- start daemon (long-running process with MCP server)
- `mesh stop` -- stop daemon (SIGTERM via PID file)
- `mesh status` -- check daemon status (no agent-level status)
- MCP protocol over stdio for agent communication
- SQLite store with WAL mode and per-body CRUD
- Docker adapter for container lifecycle
- Body state machine (8 states with enforced transitions)
- Migration coordinator (7-step export/provision/transfer/import/verify/switch/cleanup)
- Enhanced manifest format (v2 with additional metadata)

## Config Migration

### v0 Config (`~/.mesh/config.toml`)

```toml
[[machines]]
name = "pi"
host = "192.168.1.50"
user = "deploy"
ssh_key = "~/.ssh/id_ed25519"

[[agents]]
name = "my-agent"
workdir = "~/agents/my-agent"
start_cmd = "run-agent.sh"
stop_signal = "SIGTERM"
stop_timeout = "30s"
max_snapshots = 10
pid_file = ""
pre_snapshot_cmd = ""
post_restore_cmd = ""
```

### v1 Config (`~/.mesh/config.yaml`)

```yaml
daemon:
  socket_path: /tmp/mesh.sock
  pid_file: /home/user/.mesh/mesh.pid
  log_level: info

store:
  path: /home/user/.mesh/state.db

docker:
  host: unix:///var/run/docker.sock
  api_version: "1.48"
```

### Migration Steps

1. Run `mesh init` to create the `~/.mesh/` directory
2. Review `~/.mesh/config.yaml` and adjust paths if needed
3. v0 TOML config is still read by v0 commands (snapshot, restore, list, inspect, prune)
4. You can remove `~/.mesh/config.toml` once you no longer use v0 commands

Note: Agent definitions from v0 are not automatically migrated to v1. In v1, bodies are managed through the MCP server and stored in SQLite, not in a config file.

## Snapshot Migration

Snapshots are fully compatible between v0 and v1. No migration needed.

Existing snapshots in `~/.mesh/snapshots/` work with both v0 and v1 commands:

```
mesh list
mesh inspect my-agent
mesh restore my-agent
```

## CLI Changes

### Removed Commands

- `mesh clone <agent> --target <machine>` -- use daemon + Docker lifecycle instead
- `mesh status <agent>` -- use `mesh status` for daemon status only

### New Commands

- `mesh init` -- initialize config
- `mesh serve` -- start daemon
- `mesh stop` -- stop daemon
- `mesh status` -- show daemon status

### Changed Commands

- `mesh status` no longer accepts an agent argument (shows daemon status only)

## New Packages

| Package | Purpose |
|---------|---------|
| `internal/adapter/` | SubstrateAdapter interface |
| `internal/body/` | Body state machine and lifecycle |
| `internal/config/` | YAML config (v1) |
| `internal/daemon/` | Long-running daemon |
| `internal/docker/` | Docker adapter |
| `internal/mcp/` | MCP server |
| `internal/store/` | SQLite store |

## Workflow Migration

### v0 Workflow

```
# Configure agents in TOML
mesh snapshot my-agent
mesh status my-agent
mesh clone my-agent --target pi
```

### v1 Workflow

```
# Initialize config and start daemon
mesh init
mesh serve &

# Create and manage bodies via MCP (or CLI for snapshots)
mesh snapshot my-agent
mesh status
```

v1 does not include `mesh clone`. Migration between substrates is handled by the daemon through the migration coordinator, exposed via MCP tools (`migrate_body`).

## Questions

See the [Architecture Overview](architecture) for system design, [CLI Reference](cli-reference) for command details, and [MCP API](mcp-api) for tool documentation.
