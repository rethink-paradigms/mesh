# config-toml — CLI TOML Configuration

## What it does
Parses and validates the mesh CLI TOML config file (`~/.mesh/config.toml`). Used by the `mesh` CLI tool for snapshot/restore/status/stop commands.

> **⚠️ Note:** The daemon uses `internal/config` (YAML), NOT this package. This package is exclusively for the `mesh` CLI binary. See `internal/config/CONTEXT.md` for the daemon-side config.

## Key types

| Type | File | Purpose |
|------|------|---------|
| `Config` | `config.go` | Agent definitions with workdir, stop_timeout, post_restore_cmd |
| `AgentDef` | `config.go` | Per-agent config: name, workdir, snapshot settings |

## Dependencies
- `github.com/BurntSushi/toml` — TOML parsing

## Invariants
- Workdir paths support `~` expansion
- Only used by CLI commands (not daemon)
- Plans to deprecate in favor of unified YAML config
