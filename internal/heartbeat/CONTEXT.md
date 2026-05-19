# heartbeat — Gateway Connectivity Reporter

## What it does
Periodically sends heartbeat POSTs to the agent-bodies gateway to report daemon health, body count, body states, and orchestration status. Used by the control plane to detect disconnected VMs.

## Key types

| Type | File | Purpose |
|------|------|---------|
| `Client` | `heartbeat.go` | HTTP client that sends periodic POSTs with daemon state |
| `HeartbeatBodyInfo` | `heartbeat.go` | Per-body summary in heartbeat payload |
| `GatewayStatus` | `heartbeat.go` | Current gateway connectivity state |

## Dependencies
- `net/http` — HTTP client

## Invariants
- Heartbeat is optional — only starts if gateway URL AND interval are configured
- JWT-only mode blocks heartbeat if auth_token is missing
- Status is reachable/unreachable with consecutive failure tracking
- Payload includes: version, tier, orchestrator, bodies_count, health, cluster_id
