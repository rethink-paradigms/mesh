# service — Body Lifecycle Validation Layer

## What it does
Consolidates body lifecycle validation between REST and MCP handlers. Validates state transitions before delegating to BodyManager. Provides typed error types that map consistently to HTTP status codes and JSON-RPC error codes.

## Key types

| Type | File | Purpose |
|------|------|---------|
| `BodyService` | `body_service.go` | Validation + delegation for Create, Start, Stop, Destroy, List, Get, Exec |
| `NotFoundError` | `errors.go` | Maps to HTTP 404 / JSON-RPC -32001 |
| `ConflictError` | `errors.go` | Maps to HTTP 409 / JSON-RPC -32002 |
| `ValidationError` | `errors.go` | Maps to HTTP 400 / JSON-RPC -32602 |

## Dependencies
- `internal/body` — BodyManager + Body type
- `internal/store` — Store for cluster-scoped queries
- `internal/orchestrator` — Registry for substrate resolution

## Invariants
- All lifecycle validation happens HERE, not in handlers
- Create requires name + image; resolves substrate automatically when 1 adapter
- Start requires Stopped state
- Stop requires Running or Starting state
- Destroy requires Stopped or Error state (not Running)
- All errors returned are typed (NotFoundError, ConflictError, ValidationError)
