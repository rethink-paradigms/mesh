# ingress — HTTP Routing & Port Management

## What it does
Defines the `IngressAdapter` interface for managing HTTP routes to agent bodies. Provides two implementations: Caddy (production) and Noop (development fallback). Handles port pool allocation and URL building.

## Key types

| Type | File | Purpose |
|------|------|---------|
| `IngressAdapter` | `ingress.go` | Core interface: AddRoute, RemoveRoute, ListRoutes, AllocPort, FreePort, BuildURL |
| `Route` | `ingress.go` | Domain → upstream:port mapping |
| `CaddyAdapter` | `caddy.go` | Caddy Admin API implementation |
| `NoopAdapter` | `noop.go` | Stub that logs would-do messages to stderr |

## Dependencies
- `net/http` — Caddy Admin API calls
- `sync` — Port pool thread safety

## Invariants
- Port pool: configurable range (default 9000-9999)
- When `public_domain` is set, builds `https://{name}.{domain}` URLs
- When empty, builds `http://127.0.0.1:{port}` for VM-local access
- Caddy auto-detection runs at daemon startup
- Noop adapter is used when no ingress controller is configured
