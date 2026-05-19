# agent — Agent Manifest Installer

## What it does
Loads agent descriptor files from a directory and installs agents into running bodies. An "agent" is a pre-packaged AI agent (like Hermes) that runs inside a body container. The installer wires up ingress routes for agent access.

## Key types

| Type | File | Purpose |
|------|------|---------|
| `Descriptor` | `descriptor.go` | Agent manifest: name, image, command, endpoints, post-install hook |
| `Installer` | `installer.go` | Installs agents into bodies: pulls image, creates container, configures ingress |

## Dependencies
- `internal/body` — BodyManager for container lifecycle
- `internal/ingress` — IngressAdapter for route configuration
- `internal/orchestrator` — Registry for adapter selection

## Invariants
- Installer is optional — daemon operates without agents dir
- Agent descriptors are loaded at daemon startup, not hot-reloaded
