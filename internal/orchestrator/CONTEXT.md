# orchestrator — Substrate Abstraction Layer

## What it does
Defines the `OrchestratorAdapter` interface that all compute substrates (Docker, Nomad, future providers) must implement. Also provides a `Registry` for adapter discovery (database/sql Driver pattern).

## Key types

| Type | File | Purpose |
|------|------|---------|
| `OrchestratorAdapter` | `orchestrator.go` | Core interface: Schedule, Start, Stop, Destroy, GetStatus, Name, IsHealthy |
| `Registry` | `orchestrator.go` | Thread-safe adapter registry with Register/Open/Default |
| `Handle` | `orchestrator.go` | Opaque string identifier for a body instance |
| `BodyState` | `orchestrator.go` | Lifecycle state constants (Created, Starting, Running, etc.) |
| `BodySpec` | `orchestrator.go` | Desired state for body creation (Image, Env, Cmd, Ports, etc.) |
| `BodyStatus` | `orchestrator.go` | Runtime status (State, Uptime, Memory, CPU) |
| `Exporter` | `extensions.go` | Optional: export container filesystem as tar stream |
| `Importer` | `extensions.go` | Optional: import tar stream into container |
| `Executor` | `extensions.go` | Optional: exec commands inside container |
| `Inspector` | `extensions.go` | Optional: inspect container metadata |
| `NodeLister` | `extensions.go` | Optional: list cluster nodes |

## Dependencies
None — this is the bottom of the dependency graph. Only stdlib.

## Invariants
- Registry is designed after `database/sql` pattern (open by name)
- All adapters must implement `OrchestratorAdapter` at minimum
- Extension interfaces (Exporter, Importer, etc.) are checked via type assertion at call sites
- Handles are opaque strings — no routing info encoded
