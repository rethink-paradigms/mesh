# body — Body Domain Model & Lifecycle

## What it does
Defines the Body type, its 8-state FSM, and lifecycle operations. Bodies are agent compute identities (container + filesystem). This package owns the state machine and all state transition rules.

## Key types

| Type | File | Purpose |
|------|------|---------|
| `Body` | `body.go` | Domain entity: ID, Name, State, InstanceID, PortAllocations |
| `BodyManager` | `manager.go` | Lifecycle coordinator: Create, Start, Stop, Destroy, List, Get, Exec |
| `MigrationCoordinator` | `migration.go` | 7-step cross-substrate migration (export→provision→transfer→import→verify→switch→cleanup) |
| `AllocatedPort` | `body.go` | Port allocation tracking per body |
| `Registry` | `migration.go` | Interface for snapshot push/pull during cross-machine migrations |

## State machine (8 states)
```
Created → Starting → Running → Stopping → Stopped → Destroyed
   ↓         ↓         ↓          ↓                    ↑
 Error ←─────┴─────────┴──────────┴────────────────────┘
                   Migrating ───────────────────────────┘
```
Valid transitions are enforced by `validTransitions` map in `body.go`.

## Dependencies
- `internal/orchestrator` — adapter interface for substrate operations
- `internal/store` — SQLite persistence
- `internal/ingress` — port allocation and route management

## Invariants
- State transitions are always persisted to store (never in-memory only)
- Destroy requires Stopped or Error state
- Migration requires Running state
- Port allocations are cleaned up on Stop/Destroy via `preStop()`
