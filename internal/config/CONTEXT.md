# config — Daemon YAML Configuration

## What it does
Parses and validates the mesh daemon YAML config file (`~/.mesh/config.yaml` or `MESH_CONFIG` env). Applies defaults and validates required fields. This is the canonical config for the daemon.

> **⚠️ Note:** There is also `internal/config-toml` which provides a TOML-based config for the `mesh` CLI tool (snapshot/restore/status commands). These two config systems are NOT unified. The daemon uses YAML; the CLI tool uses TOML. Both parse from `~/.mesh/` but serve different binaries. See `internal/config-toml/CONTEXT.md` for the CLI-side config.

## Key types

| Type | File | Purpose |
|------|------|---------|
| `Config` | `config.go` | Top-level config: Daemon, Store, Orchestrators, Bodies, Registry, Plugin, Ingress |
| `DaemonConfig` | `config.go` | Daemon runtime settings (auth, gateway, heartbeat) |
| `PluginConfig` | `config.go` | Plugin directory and enabled list |
| `IngressConfig` | `config.go` | Ingress adapter, Caddy URL, port pool |
| `AuthConfig` | `config.go` | Subset of auth settings for passing to services |

## Dependencies
- `gopkg.in/yaml.v3` — YAML parsing
- `net/url` — URL validation for Nomad address

## Invariants
- `DefaultPath()` respects `MESH_CONFIG` env var
- Auth validation depends on `auth_mode` (token, jwt, both)
- Legacy `[nomad]` section auto-migrated to `orchestrators.nomad`
- `EnsureDirs()` creates all required directories
- Default limits: max_bodies=10, max_snapshots=5
