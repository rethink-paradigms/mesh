# plugin — go-plugin Loader & Health Checker

## What it does
Loads external provider plugins using HashiCorp's go-plugin over gRPC. Scans a directory for plugin binaries, loads them, and runs periodic health checks with auto-restart (3 retries). Plugins are the extension mechanism for adding new substrate adapters / providers.

## Key types

| Type | File | Purpose |
|------|------|---------|
| `PluginManager` | `manager.go` | Scans directory, loads plugins, runs health checks |
| `Plugin` interface | `interface.go` | Plugin contract (gRPC client, health check) |

## Dependencies
- `github.com/hashicorp/go-plugin` — plugin framework
- `google.golang.org/grpc` — gRPC transport

## Invariants
- Plugins communicate over gRPC with go-plugin handshake
- Max 3 restart retries for failed plugins
- Health checks run periodically after initial load
- Plugin directory is configurable (default: `~/.mesh/plugins`)
- Plugin system is optional — daemon operates without plugins
