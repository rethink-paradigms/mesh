# Mesh

Mesh is a portable agent-body runtime for AI agents. v0 proves the core primitive: filesystem snapshot, restore, and clone, so an agent's state can be faithfully captured, moved, and restored with byte-perfect integrity.

An agent gets a persistent compute identity that lives on any substrate and can move between them without losing itself. v0 is a single binary, no daemon, no containers, no networking libraries.

## Install

### From Source

```bash
git clone https://github.com/rethink-paradigms/mesh.git
cd mesh
go build -o /usr/local/bin/mesh ./cmd/mesh/
```

Requires Go 1.25 or later. No CGo, no system dependencies beyond a working Go toolchain.

### From Release

Pre-built binaries are not yet available. Build from source for now.

## Quick Start

```bash
# Create the config directory
mkdir -p ~/.mesh

# Write a config file (see Configuration section below)
cat > ~/.mesh/config.toml << 'EOF'
[[agents]]
name = "my-agent"
workdir = "~/agents/my-agent"
start_cmd = "run-agent.sh"
EOF

# Create the workdir with some state
mkdir -p ~/agents/my-agent

# Take a snapshot
mesh snapshot my-agent

# List all snapshots
mesh list

# Check agent status
mesh status my-agent

# Restore from the latest snapshot
mesh restore my-agent

# Clone to another machine
mesh clone my-agent --target pi
```

## Configuration

Mesh reads a TOML config file. It looks for it at `~/.mesh/config.toml` by default. You can override this with the `MESH_CONFIG` environment variable or the `--config` flag.

### Full Schema

```toml
# A machine is a remote host where agents can be cloned to.
# Local-only setups don't need any [[machines]] entries.
[[machines]]
name = "pi"              # Required. Identifier used in --target flags.
host = "192.168.1.50"    # Required. Hostname or IP address.
port = 22                # Optional. SSH port. Default: 22.
user = "deploy"          # Optional. SSH user. Default: current user.
ssh_key = "~/.ssh/id_ed25519"  # Optional. Path to private key. Must be <= 0600 permissions.
agent_dir = "/opt/mesh/agents" # Optional. Directory on the remote machine for cloned agents.

[[agents]]
name = "my-agent"            # Required. Identifier used in all commands.
machine = ""                 # Optional. Machine name this agent runs on. Empty = local.
workdir = "~/agents/my-agent" # Required. Root directory to snapshot. ~ is expanded.
start_cmd = "run-agent.sh"   # Optional. Command recorded in manifest metadata.
stop_signal = "SIGTERM"      # Optional. Default: "SIGTERM".
stop_timeout = "30s"         # Optional. Go duration string. Default: "30s".
max_snapshots = 10           # Optional. Oldest snapshots pruned automatically. Default: 10.
pid_file = ""                # Optional. Path to PID file for status checks.
pre_snapshot_cmd = ""        # Optional. Shell command run in workdir before snapshot.
post_restore_cmd = ""        # Optional. Shell command run in workdir after restore.
```

### Config Resolution Order

1. `--config /path/to/config.toml` flag (highest priority)
2. `$MESH_CONFIG` environment variable
3. `~/.mesh/config.toml` (default)

### Validation Rules

Machine and agent names must be non-empty and unique. Agent `workdir` is required. If an agent references a `machine`, that machine must exist in the config. SSH keys must exist on disk with permissions no looser than 0600.

## Commands

### `mesh snapshot <agent>`

Create a filesystem snapshot of an agent's workdir.

The snapshot pipeline walks the workdir directory tree in sorted order, writes a tar archive, pipes it through zstd compression, and hashes the output with SHA-256. Output is deterministic: the same workdir produces the same archive byte-for-byte across runs (timestamps and uid/gid are stripped from tar headers).

Produces three files in `~/.mesh/snapshots/<agent>/`:

- `<agent>-YYYYMMDD-HHMMSS.tar.zst` - the compressed tarball
- `<agent>-YYYYMMDD-HHMMSS.tar.zst.sha256` - hex-encoded SHA-256 digest
- `<agent>-YYYYMMDD-HHMMSS.json` - manifest with metadata (agent name, timestamp, source machine, workdir, checksum, size)

If `pre_snapshot_cmd` is set on the agent, it runs in the workdir before the snapshot starts. If `max_snapshots` is set, the oldest snapshots are pruned after a successful snapshot.

```bash
mesh snapshot my-agent
# Snapshot created for my-agent
```

### `mesh restore <agent>`

Restore an agent's workdir from a snapshot. Overwrites the current workdir contents.

By default, restores from the latest snapshot. Use `--snapshot` to specify a particular snapshot file.

If `post_restore_cmd` is set on the agent, it runs in the workdir after extraction completes.

```bash
# Restore from the latest snapshot
mesh restore my-agent

# Restore a specific snapshot
mesh restore my-agent --snapshot ~/.mesh/snapshots/my-agent/my-agent-20260424-143000.tar.zst
```

**Flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `--snapshot` | latest | Path to a specific snapshot file |

### `mesh clone <agent> --target <machine>`

Clone an agent to a remote machine. Takes a snapshot, transfers it via `scp`, and extracts it on the target.

Uses `os/exec` to call `ssh` and `scp` directly. No Go SSH libraries. Your SSH agent or configured keys handle authentication.

```bash
mesh clone my-agent --target pi
# Cloned my-agent to pi
```

**Flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `--target` | required | Machine name from config (must match a `[[machines]]` entry) |

### `mesh status <agent>`

Show an agent's current state: running or stopped, snapshot count, most recent snapshot timestamp, and total cache size.

Running status is determined by checking for a PID file or process. If no PID file is configured, status shows "stopped".

```bash
mesh status my-agent
# Agent my-agent: stopped
# Snapshots: 3
# Last snapshot: 2026-04-24 14:30:00
# Cache size: 45.2 MB
```

### `mesh list [agent]`

List snapshots. Without an agent name, lists snapshots for all agents. With an agent name, filters to that agent only.

Each line shows the snapshot path, file size, source machine (if available from manifest), and timestamp.

```bash
# List all snapshots
mesh list

# List snapshots for one agent
mesh list my-agent
```

### `mesh inspect <snapshot>`

Show the manifest details for a snapshot: agent name, timestamp, source machine, source workdir, start command, stop timeout, SHA-256 checksum, and size.

Accepts either a full path to a `.tar.zst` file or an agent name (resolves to the latest snapshot).

```bash
mesh inspect my-agent
# Agent: my-agent
# Timestamp: 2026-04-24T14:30:00Z
# Source machine: workstation
# Source workdir: /home/user/agents/my-agent
# Start cmd: run-agent.sh
# Stop timeout: 30s
# Checksum: a1b2c3d4...
# Size: 45.2 MB
```

### `mesh prune <agent>`

Remove old snapshots, keeping only the most recent N. Deletes the `.tar.zst`, `.sha256`, and `.json` sidecar files.

```bash
# Keep only the 3 most recent snapshots
mesh prune my-agent --keep 3
# Pruned 2 snapshot(s) for my-agent (kept 3)
```

**Flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `--keep` | 5 | Number of most recent snapshots to keep |

## Global Flags

These flags apply to all commands.

| Flag | Description |
|------|-------------|
| `--config` | Path to config file. Default: `~/.mesh/config.toml` or `$MESH_CONFIG` |
| `--verbose` | Enable debug output to stderr |
| `--quiet` | Suppress progress output |
| `--version` | Print version and exit |

## Architecture

Library-first. All logic lives in `internal/` packages. The CLI is a thin Cobra wrapper around them.

```
cmd/mesh/           CLI entry point, Cobra commands
internal/config/    TOML config parsing and validation
internal/snapshot/  tar + zstd + SHA-256 streaming pipeline
internal/restore/   Snapshot extraction and post-restore hooks
internal/clone/     Remote clone via scp/ssh
internal/agent/     Process status detection
internal/manifest/  JSON sidecar read/write
```

Snapshot pipeline: sorted directory walk, tar archive, `io.Pipe` to zstd compression, SHA-256 hash teed alongside the output file. No full tarball buffered in memory. Context cancellation cleans up partial files.

Deterministic output is achieved by sorting directory entries lexicographically and stripping uid, gid, username, groupname, and device numbers from tar headers. The same workdir produces identical output across machines.

## v0 Scope

What's in v0:

- 7 commands: snapshot, restore, clone, status, list, inspect, prune
- TOML config with `[[machines]]` and `[[agents]]` sections
- Deterministic tar + zstd + SHA-256 snapshot pipeline
- JSON manifest sidecar for each snapshot
- Remote clone via `os/exec` calling `scp`/`ssh`
- Pre-snapshot and post-restore hook commands
- Automatic snapshot pruning by count
- Single static binary, no CGo, no system dependencies

## Limitations

What's not in v0:

- No daemon mode. Every command runs, completes, exits.
- No MCP server. The CLI is the only interface.
- No container runtime integration.
- No delta or incremental snapshots. Every snapshot is a full tarball.
- No concurrent operation protection. Don't run two snapshots on the same agent at once.
- No `.meshignore` file. The entire workdir is snapshotted.
- No Windows support. macOS and Linux only.
- No color or JSON output formatting.
- No encryption. Snapshots are compressed but not encrypted. Secure your transport (SSH) and storage.

## Build

```bash
# Build the binary
go build -o mesh ./cmd/mesh/

# Run tests with race detector
go test -race ./...

# Lint
golangci-lint run
```

## Dependencies

- [spf13/cobra](https://github.com/spf13/cobra) - CLI framework
- [BurntSushi/toml](https://github.com/BurntSushi/toml) - TOML parsing
- [klauspost/compress](https://github.com/klauspost/compress) - zstd compression

## License

MIT. See [LICENSE](LICENSE).
