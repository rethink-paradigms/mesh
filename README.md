# Mesh

Portable agent-body runtime for AI agents. Gives an agent a persistent compute identity (filesystem state) that can live on any substrate and move between them. Self-hosted, user-owned, no central dependency.

## What is Mesh

Mesh provides two core abstractions:

- **Body**: A portable compute identity with persistent filesystem state. Bodies can move between substrates without losing themselves.
- **Form**: The physical instantiation of a body on a substrate. A body can take many forms over its lifetime (laptop, VM, container).

The snapshot primitive is `docker export | zstd` — a flat filesystem tarball. No memory state, fully portable.

## Substrate Pools

Mesh supports three substrate pools:

- **Local**: Laptop, workstation, or Raspberry Pi
- **Fleet**: BYO VMs scheduled via Nomad (not Kubernetes)
- **Sandbox**: Cloud environments like Daytona, E2B, Fly, Modal, Cloudflare

## Primary Interface

The primary interface is the **MCP server** over stdio. AI agents communicate with Mesh via JSON-RPC. A CLI is provided for human operators.

## Install

### From Source

```bash
git clone https://github.com/rethink-paradigms/mesh.git
cd mesh
go build -o /usr/local/bin/mesh ./cmd/mesh/
```

Requires Go 1.25 or later. No CGo, no system dependencies beyond a working Go toolchain and Docker.

### From Release

Pre-built binaries are not yet available. Build from source for now.

## Quick Start

```bash
# Initialize config
mesh init

# Start the daemon
mesh serve

# In another terminal, check daemon status
mesh status

# Use v0 commands for snapshot/restore
mesh snapshot my-agent
mesh list
```

## Architecture

Mesh v1 is a daemon-based system:

```
mesh serve (daemon)
  ├─ Store (SQLite with WAL mode)
  ├─ Docker adapter (SubstrateAdapter)
  ├─ Body manager (state machine)
  └─ MCP server (stdio JSON-RPC)
```

### Body State Machine

Bodies transition through 8 states with enforced valid transitions:

```
Created → Starting → Running → Paused → Stopping → Stopped
          ↓           ↓         ↓
         Failed    Migrating  Restoring
```

### Snapshot and Restore

- **Snapshot**: `docker export | zstd | sha256sum` produces a portable tarball
- **Restore**: Extract tarball to new location, optionally provision Docker container
- **Migration**: 7-step coordinator (export, provision, transfer, import, verify, switch, cleanup)

## Commands

### `mesh init`

Initialize Mesh configuration. Creates `~/.mesh/config.yaml` with defaults.

```bash
mesh init
# Mesh initialized at /home/user/.mesh/config.yaml
```

### `mesh serve`

Start the Mesh daemon. This is a long-running process that exposes an MCP server.

```bash
mesh serve
```

The daemon:
- Reads config from `~/.mesh/config.yaml`
- Opens SQLite store with WAL mode
- Initializes Docker adapter
- Starts MCP server on stdio
- Writes PID file for `mesh status` and `mesh stop`

Run in background:

```bash
mesh serve &
# Or use a process manager like systemd, supervisord, etc.
```

### `mesh stop`

Stop the Mesh daemon by sending SIGTERM to the process recorded in the PID file.

```bash
mesh stop
# Stopped mesh daemon (pid 12345)
```

### `mesh status`

Show daemon status. Checks if the process from the PID file is running.

```bash
mesh status
# Mesh daemon: running (pid 12345)
```

### `mesh snapshot <agent>`

Create a filesystem snapshot (v0 command, still available).

The snapshot pipeline walks the workdir directory tree in sorted order, writes a tar archive, pipes it through zstd compression, and hashes the output with SHA-256. Output is deterministic: the same workdir produces the same archive byte-for-byte across runs.

Produces three files in `~/.mesh/snapshots/<agent>/`:

- `<agent>-YYYYMMDD-HHMMSS.tar.zst` - the compressed tarball
- `<agent>-YYYYMMDD-HHMMSS.tar.zst.sha256` - hex-encoded SHA-256 digest
- `<agent>-YYYYMMDD-HHMMSS.json` - manifest with metadata

```bash
mesh snapshot my-agent
# Snapshot created for my-agent
```

### `mesh restore <agent>`

Restore an agent from a snapshot (v0 command, still available).

```bash
mesh restore my-agent
# Restored my-agent from my-agent-20260427-143000.tar.zst
```

### `mesh list [agent]`

List snapshots (v0 command, still available).

```bash
mesh list
mesh list my-agent
```

### `mesh inspect <snapshot>`

Show snapshot manifest details (v0 command, still available).

```bash
mesh inspect my-agent
# Agent: my-agent
# Timestamp: 2026-04-27T14:30:00Z
# Source machine: workstation
# Source workdir: /home/user/agents/my-agent
# Checksum: a1b2c3d4...
# Size: 45.2 MB
```

### `mesh prune <agent>`

Remove old snapshots, keeping the most recent N (v0 command, still available).

```bash
mesh prune my-agent --keep 3
# Pruned 2 snapshot(s) for my-agent (kept 3)
```

## Project Structure

```
cmd/mesh/main.go       CLI entry point (Cobra)
internal/
  adapter/             SubstrateAdapter interface
  body/                Body state machine + lifecycle
  config/              YAML config (v1)
  config-toml/         TOML config (v0, backward compat)
  daemon/              Long-running daemon process
  docker/              Docker SubstrateAdapter
  manifest/            Snapshot manifest (v1 + v2)
  mcp/                 MCP server (stdio JSON-RPC)
  restore/             Restore from snapshot
  snapshot/            Create filesystem snapshots
  store/               SQLite store (WAL mode)
```

## Configuration

Mesh v1 uses YAML configuration at `~/.mesh/config.yaml`. Run `mesh init` to create a default config.

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

### Config Resolution Order

1. `--config /path/to/config.yaml` flag (highest priority)
2. `$MESH_CONFIG` environment variable
3. `~/.mesh/config.yaml` (default for v1 commands: init, serve, stop, status)
4. `~/.mesh/config.toml` (default for v0 commands: snapshot, restore, list, inspect, prune)

## Development

```bash
# Build
go build ./cmd/mesh/

# Test
go test ./...

# Vet
go vet ./...

# Lint
golangci-lint run
```

## v0 Compatibility

v0 commands (snapshot, restore, list, inspect, prune) are still available. They read from `~/.mesh/config.toml` and operate on the same snapshot format. This allows gradual migration from v0 to v1.

For v0→v1 migration details, see [MIGRATION-v0-to-v1.md](MIGRATION-v0-to-v1.md).

## License

MIT. See [LICENSE](LICENSE).
