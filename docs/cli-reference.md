# CLI Reference

The Mesh CLI (`mesh`) provides commands for daemon management and snapshot operations. The CLI is a secondary interface; the primary interface is the MCP server over stdio (D5). Use the CLI for debugging, automation, and one-off operations.

## Global Flags

| Flag | Type | Description |
|------|------|-------------|
| `--config` | `string` | Path to config file (default: `~/.mesh/config.yaml` for v1, `~/.mesh/config.toml` for v0 commands) |
| `--verbose` | `bool` | Enable debug output to stderr |
| `--quiet` | `bool` | Suppress progress output |

## Commands

### `mesh init`

Initialize Mesh configuration. Creates the `~/.mesh/` directory.

```
mesh init
Mesh initialized. Run 'mesh serve' to start.
```

Does not write a config file; run `mesh serve` afterward to generate one if needed.

---

### `mesh serve`

Start the Mesh daemon. A long-running process that opens the SQLite store, initializes the Docker adapter, starts the MCP server on stdio, and registers signal handlers for graceful shutdown.

```
mesh serve
```

The daemon writes a PID file to `~/.mesh/mesh.pid`. Run in the background:

```
mesh serve &
```

Or use a process manager (systemd, supervisord). The daemon exits when it receives SIGTERM or SIGINT.

Daemon startup sequence:

1. Check for PID file conflicts
2. Open SQLite store with WAL mode
3. Initialize Docker adapter and MultiAdapter router
4. Create BodyManager
5. Scan and load plugins
6. Run startup reconciliation (verify body states against adapters)
7. Write PID file
8. Start HTTP health server on `127.0.0.1:0`
9. Register signal handlers
10. Block until signal or context cancellation
11. Graceful shutdown

---

### `mesh stop`

Stop the Mesh daemon by sending SIGTERM to the process recorded in the PID file. Waits for the daemon to exit, then escalates to SIGKILL if the timeout expires.

```
mesh stop
Stopping mesh daemon (pid 12345)...
Stopped mesh daemon
```

Flags:

| Flag | Default | Description |
|------|---------|-------------|
| `--timeout` | `30s` | Timeout to wait for daemon to stop before sending SIGKILL |

---

### `mesh status`

Show the daemon's running status by checking the PID file.

```
mesh status
Mesh daemon: running (pid 12345)
```

When the daemon is stopped:

```
mesh status
Mesh daemon: stopped
```

Exit codes: 0 if running, non-zero if stopped.

---

### `mesh snapshot <agent>`

Create a filesystem snapshot of an agent's working directory. Walks the workdir tree in sorted order, creates a tar archive, pipes it through zstd compression, and computes a SHA-256 digest. The pipeline is deterministic: the same workdir produces the same archive byte-for-byte across runs.

```
mesh snapshot my-agent
Snapshot created for my-agent
```

Produces three files in `~/.mesh/snapshots/<agent>/`:

- `<agent>-YYYYMMDD-HHMMSS.tar.zst` -- the compressed tarball
- `<agent>-YYYYMMDD-HHMMSS.tar.zst.sha256` -- hex-encoded SHA-256 digest
- `<agent>-YYYYMMDD-HHMMSS.json` -- manifest with agent name, timestamp, source machine, source workdir, start command, checksum, and size

Uses `~/.mesh/config.toml` for agent definitions. Compatible with v0 snapshot format.

---

### `mesh restore <agent>`

Restore an agent from its most recent snapshot. Reads the snapshot tarball, verifies the SHA-256 checksum, and extracts the files to the agent's workdir. Optionally runs a post-restore command hook.

```
mesh restore my-agent
Restored my-agent from my-agent-20260427-143000.tar.zst
```

Flags:

| Flag | Description |
|------|-------------|
| `--snapshot` | Path to a specific snapshot file (default: latest) |

---

### `mesh list [agent]`

List snapshots for one or all agents. Shows each snapshot's file path, human-readable size, source machine, and timestamp.

```
mesh list
/home/user/.mesh/snapshots/my-agent/my-agent-20260427-143000.tar.zst  45.2 MB  from workstation  2026-04-27 14:30:00
```

```
mesh list my-agent
/home/user/.mesh/snapshots/my-agent/my-agent-20260427-143000.tar.zst  45.2 MB  from workstation  2026-04-27 14:30:00
```

Output format: `<path>  <size>  from <machine>  <timestamp>`

---

### `mesh inspect <snapshot>`

Show the manifest details of a snapshot. The argument can be a full path to a `.tar.zst` file or just an agent name (resolves to the latest snapshot).

```
mesh inspect my-agent
Agent: my-agent
Timestamp: 2026-04-27T14:30:00Z
Source machine: workstation
Source workdir: /home/user/agents/my-agent
Start cmd: run-agent.sh
Stop timeout: 30s
Checksum: a1b2c3d4e5f6...
Size: 45.2 MB
```

Fields shown:

| Field | Source |
|-------|--------|
| Agent | Snapshot manifest |
| Timestamp | Snapshot creation time (RFC 3339) |
| Source machine | Hostname where snapshot was taken |
| Source workdir | Working directory of the agent |
| Start cmd | Command used to start the agent |
| Stop timeout | Graceful shutdown timeout |
| Checksum | SHA-256 digest of the tarball |
| Size | Human-readable file size |

---

### `mesh prune <agent>`

Remove old snapshots for an agent, keeping only the most recent N. Deletes the `.tar.zst`, `.sha256`, and `.json` manifest files.

```
mesh prune my-agent --keep 3
Pruned 2 snapshot(s) for my-agent (kept 3)
```

```
mesh prune my-agent --keep 5
Only 3 snapshots, nothing to prune (keep=5)
```

Flags:

| Flag | Default | Description |
|------|---------|-------------|
| `--keep` | `5` | Number of most recent snapshots to retain |

Designed for automated cleanup in cron or CI pipelines.

## Config Resolution

The CLI loads configuration differently depending on the command:

| Command Group | Config File | Priority |
|---------------|-------------|----------|
| v1 commands (`init`, `serve`, `stop`, `status`) | `~/.mesh/config.yaml` | `--config` flag > `$MESH_CONFIG` > default path |
| v0 commands (`snapshot`, `restore`, `list`, `inspect`, `prune`) | `~/.mesh/config.toml` | `--config` flag > `$MESH_CONFIG` > default path |

v1 commands use YAML config. v0 commands use TOML config for backward compatibility. Both can coexist.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | Runtime error (daemon not running, snapshot not found, etc.) |
| 2 | Usage error (missing required argument, invalid flag) |

## Environment Variables

| Variable | Description |
|----------|-------------|
| `MESH_CONFIG` | Override the default config file path |
| `MESH_TESTING` | When set, relaxes certain validation checks (plugin dir existence, etc.) |
