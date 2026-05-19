# mcp — MCP Server (JSON-RPC over stdio)

## What it does
Implements a Model Context Protocol (MCP) server over stdio transport. Registers 16+ tools for body lifecycle, snapshots, migration, plugins, and daemon status. All tools are authenticated when JWT is configured.

## Key types

| Type | File | Purpose |
|------|------|---------|
| `Server` | `server.go` | MCP server: reads JSON-RPC from stdin, writes to stdout, routes to handlers |
| `ToolDefinition` | `server.go` | Tool metadata for tools/list response |
| `ToolHandler` | `server.go` | Function signature for tool implementation |
| `Request` / `Response` | `server.go` | JSON-RPC 2.0 message types |
| `RPCError` | `server.go` | JSON-RPC error object |

## Tools registered (handlers.go)
- `ping` — health check
- `list_bodies`, `get_body`, `create_body`, `delete_body`, `start_body`, `stop_body`
- `create_snapshot`, `list_snapshots`, `get_snapshot`, `restore_body`
- `migrate_body` — 7-step cross-substrate migration
- `exec_command` — exec inside body
- `get_body_logs`, `get_body_status`
- `list_plugins`, `plugin_health`
- `list_capabilities`, `daemon_status`
- `install_agent`

## Dependencies
- `internal/service` — body lifecycle validation
- `internal/body` — BodyManager, MigrationCoordinator
- `internal/api` — JWT validation
- `internal/ingress` — ingress adapter (set via SetIngress)
- `internal/orchestrator` — registry, extensions

## Invariants
- Stderr is reserved for logging; stdout is MCP protocol
- Tools map service errors to JSON-RPC error codes
- Auth is passed via SetAuth() after server construction
