# v0 → v1 Migration Guide

## What Changed

Mesh v1 is a complete redesign. v0 was a snapshot and clone tool for managing agent processes. v1 is a portable agent-body runtime with a daemon, MCP server, Docker integration, and SQLite store.

## Conceptual Changes

| v0 | v1 |
|----|----|
| Agents (per-machine processes) | Bodies (portable compute identities) |
| TOML config (~/.mesh/config.toml) | YAML config (~/.mesh/config.yaml) |
| SSH-based clone/transport | Docker-based lifecycle |
| JSON sidecar manifests | SQLite store + enhanced manifests (v2) |
| CLI-only interface | MCP server + CLI |
| No daemon | Long-running daemon process |

## What Was Removed

- `mesh clone` — replaced by `mesh serve` + Docker lifecycle and migration coordinator
- `mesh status <agent>` — removed, replaced by `mesh status` (daemon status only)
- Agent process management (pgrep-based lifecycle checks)
- SSH transport for machine-to-machine cloning
- Machine definitions in config (no more `[[machines]]` section)
- `stop_signal`, `stop_timeout`, `pid_file`, `pre_snapshot_cmd`, `post_restore_cmd` in agent config (hooks moved to runtime)

## What Was Kept

- `mesh snapshot` — filesystem snapshot (tar + zstd + SHA-256), unchanged
- `mesh restore` — restore from snapshot, unchanged
- `mesh list` — list snapshots, unchanged
- `mesh inspect` — show manifest, unchanged
- `mesh prune` — remove old snapshots, unchanged
- JSON manifest files (enhanced with v2 fields)
- TOML config reading (for backward compatibility)
- Snapshot format and directory structure

## What's New

- `mesh init` — initialize v1 YAML config
- `mesh serve` — start daemon (long-running process with MCP server)
- `mesh stop` — stop daemon (SIGTERM via PID file)
- `mesh status` — check daemon status (no agent-level status)
- MCP protocol over stdio for agent communication
- SQLite store with WAL mode and per-body CRUD
- Docker adapter for container lifecycle
- Body state machine (8 states with enforced transitions)
- Migration coordinator (7-step export/provision/transfer/import/verify/switch/cleanup)
- Enhanced manifest format (v2 with additional metadata)

## Config Migration

### v0 Config (~/.mesh/config.toml)

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

### v1 Config (~/.mesh/config.yaml)

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

1. Run `mesh init` to generate a default v1 config
2. Review `~/.mesh/config.yaml` and adjust paths if needed
3. v0 TOML config is still read by v0 commands (snapshot, restore, list, inspect, prune)
4. You can remove `~/.mesh/config.toml` once you no longer use v0 commands

Note: Agent definitions from v0 are not automatically migrated to v1. In v1, bodies are managed through the MCP server and stored in SQLite, not in a config file.

## Snapshot Migration

Snapshots are fully compatible between v0 and v1. No migration needed.

If you have existing snapshots in `~/.mesh/snapshots/`, they will work with both v0 and v1 commands:

```bash
# v0 command (works with v1)
mesh list
mesh inspect my-agent
mesh restore my-agent
```

## Package Migration

If you imported any v0 Go packages, update your imports:

| Old Path | New Path | Status |
|----------|----------|--------|
| `internal/agent/` | (removed) | Agent process management removed |
| `internal/clone/` | (removed) | SSH-based cloning removed |
| `internal/transport/` | (removed) | SSH transport removed |
| `internal/config/` | `internal/config-toml/` | Renamed for v0 backward compat |
| `internal/config/Config` | `internal/config-toml/Config` | Update import |
| `internal/snapshot/` | `internal/snapshot/` | Unchanged |
| `internal/restore/` | `internal/restore/` | Unchanged |
| `internal/manifest/` | `internal/manifest/` | Unchanged |

New packages in v1:

| Package | Purpose |
|---------|---------|
| `internal/config/` | YAML config (v1) |
| `internal/adapter/` | SubstrateAdapter interface |
| `internal/body/` | Body state machine |
| `internal/daemon/` | Long-running daemon |
| `internal/docker/` | Docker adapter |
| `internal/mcp/` | MCP server |
| `internal/store/` | SQLite store |

## CLI Changes

### Removed Commands

- `mesh clone <agent> --target <machine>` — use daemon + Docker lifecycle instead
- `mesh status <agent>` — use `mesh status` for daemon status only

### Renamed Commands

- `mesh status <agent>` → `mesh status` (no arguments, shows daemon status)

### New Commands

- `mesh init` — initialize config
- `mesh serve` — start daemon
- `mesh stop` — stop daemon
- `mesh status` — show daemon status

## Workflow Migration

### v0 Workflow

```bash
# Configure agents
cat > ~/.mesh/config.toml << 'EOF'
[[agents]]
name = "my-agent"
workdir = "~/agents/my-agent"
start_cmd = "run-agent.sh"
EOF

# Take snapshot
mesh snapshot my-agent

# Check status
mesh status my-agent

# Clone to remote machine
mesh clone my-agent --target pi
```

### v1 Workflow

```bash
# Initialize config
mesh init

# Start daemon
mesh serve

# In another terminal, check daemon status
mesh status

# Take snapshot (v0 command still works)
mesh snapshot my-agent

# Interact via MCP (agent-to-daemon)
echo '{"jsonrpc":"2.0","id":1,"method":"ping"}' | mesh mcp
```

Note: v1 does not include `mesh clone`. Migration between substrates is handled by the daemon through the migration coordinator, which will be exposed via MCP tools.

## Rollback

If you need to roll back to v0:

1. Stop the v1 daemon: `mesh stop`
2. Uninstall v1: `rm /usr/local/bin/mesh`
3. Install v0: `go install github.com/rethink-paradigms/mesh/cmd/mesh@v0.0.0`
4. Your v0 config and snapshots are still in place

## Questions?

See [README.md](README.md) for v1 documentation and [CONTRIBUTING.md](CONTRIBUTING.md) for development guidelines.
