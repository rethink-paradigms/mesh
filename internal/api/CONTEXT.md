# api — REST API HTTP Layer

## What it does
HTTP request/response handling for the mesh daemon REST API. Routes are defined in `router.go` and registered on Go 1.22+ pattern mux (`GET /api/v1/bodies/{id}`).

## Key types

| Type | File | Purpose |
|------|------|---------|
| `Handler` | `handler.go` | Holds `RouterConfig`; all HTTP handler methods |
| `RouterConfig` | `router.go` | Dependency injection config for the router |
| `CreateBodyRequest` | `types.go` | POST /api/v1/bodies request DTO |
| `BodyResponse` | `types.go` | Single body response DTO |
| `APIError` / `ErrorResponse` | `errors.go` | Standard error envelope |
| `JWTValidator` | `jwt.go` | Auth0 JWT validation |

## Dependencies
- `internal/service` — body lifecycle validation
- `internal/body` — body domain types
- `internal/orchestrator` — adapter interface + status types
- `internal/ingress` — ingress adapter (for installer wiring)

## Invariants
- All `/api/v1/*` routes are behind auth middleware (Bearer token or JWT)
- `GET /healthz` is always unauthenticated
- Service errors map to HTTP status codes via `mapServiceError()`
- Response content-type is always `application/json`
