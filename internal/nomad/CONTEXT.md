# nomad — Nomad Adapter (OrchestratorAdapter impl)

## What it does
Implements `OrchestratorAdapter` against the HashiCorp Nomad HTTP API (port 4646). Manages agent bodies as Nomad jobs with Docker task driver. Supports: Schedule, Start, Stop, Destroy, GetStatus, Inspect, ListNodes, GetAllocations.

## Key types

| Type | File | Purpose |
|------|------|---------|
| `Adapter` | `adapter.go` | Nomad API client implementing OrchestratorAdapter + Inspector + NodeLister + AllocQuerier |
| `Config` | `adapter.go` | Nomad connection settings (address, token, region, namespace) |

## Dependencies
- `github.com/hashicorp/nomad/api` — Nomad API client
- `internal/orchestrator` — adapter interface + extensions

## Invariants
- Jobs are created with Docker task driver
- Body ID maps to Nomad job name (`mesh-{id}`)
- Health check verifies Nomad API reachability
- Default address: `http://127.0.0.1:4646`
- Node count determines cluster tier (solo vs cluster)
