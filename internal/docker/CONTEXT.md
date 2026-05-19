# docker — Docker Adapter (OrchestratorAdapter impl)

## What it does
Implements `OrchestratorAdapter` against the Docker Engine API (unix socket). Full implementation: Schedule, Start, Stop, Destroy, GetStatus, Export, Import, Exec. Uses HTTP client over Unix domain socket.

## Key types

| Type | File | Purpose |
|------|------|---------|
| `Adapter` | `adapter.go` | Docker API client implementing OrchestratorAdapter + Exporter + Importer + Executor |
| `Config` | `adapter.go` | Socket path configuration |

## Dependencies
- `internal/orchestrator` — adapter interface
- Docker Engine API v1.43 via Unix socket (`/var/run/docker.sock`)

## Invariants
- Container names are `mesh-{uuid}` to avoid conflicts
- Client is lazily created and cached (thread-safe via mutex)
- HTTP timeout is 5 minutes (large image pulls)
- Docker state mapping: running→Running, exited→Stopped, dead→Error, etc.
- Supports `DOCKER_HOST` env override for socket path
