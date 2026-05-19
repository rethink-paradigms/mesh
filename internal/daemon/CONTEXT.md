# daemon — Process Lifecycle & Wiring Hub

## What it does
The daemon is the main process orchestrator. It wires together all subsystems (orchestrators, store, body manager, API server, MCP server, plugins, heartbeat, ingress), manages the process lifecycle (startup, signals, shutdown), and performs crash-recovery reconciliation.

## Key types

| Type | File | Purpose |
|------|------|---------|
| `Daemon` | `daemon.go` | Central struct holding all subsystem references |

## Lifecycle
1. **New()** — create Daemon struct (no side effects)
2. **Start(ctx)** — full bootstrap: PID check, store open, adapter registration, reconciliation, API server, heartbeat, signal wait, shutdown
3. **Stop(ctx)** — graceful shutdown: API server, MCP server, plugins, store

## Subsystems wired in Start()
- `store.Open()` — SQLite with WAL
- `orchestrator.Registry` — Docker + Nomad adapters
- `body.BodyManager` — lifecycle coordinator
- `service.BodyService` — validation layer
- `body.MigrationCoordinator` — migration with S3 registry
- `plugin.PluginManager` — go-plugin loader
- `api.NewRouter()` — HTTP handler with auth
- `heartbeat.Client` — gateway connectivity
- `mcp.Server` — set externally via SetMCP()

## Dependencies
Every internal package except cmd/*.

## Invariants
- PID file prevents duplicate daemon processes
- Auth validation prevents unprotected startup
- Reconciliation runs on every restart to fix state drift
- Caddy auto-detection runs on startup
- Registry config is persisted and restored across restarts
