# Feature: Deploy App (Tier-Aware Unified Dispatcher)

**Description:**
Tier-aware unified deployment dispatcher that auto-detects cluster tier and routes to the appropriate deployment function (lite web service or full-mode Traefik-based deployment).

## Interface

### Python API

**Function:** `deploy_app()`

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `app_name` | `str` | required | Application name |
| `image` | `str` | required | Docker image |
| `image_tag` | `str` | `"latest"` | Docker image tag |
| `port` | `int` | `8080` | Application port |
| `domain` | `str` | `None` | Domain for routing |
| `cpu` | `int` | `100` | CPU allocation in MHz |
| `memory` | `int` | `128` | Memory allocation in MB |
| `datacenter` | `str` | `"dc1"` | Nomad datacenter |
| `cluster_tier` | `str` | `None` | Override cluster tier (lite/standard/ingress/production) |
| `nomad_addr` | `str` | `None` | Nomad server address |

**Returns:** `bool` - True if deployment succeeded

## Tier Routing Logic

| Cluster Tier | Routing | Ingress |
|--------------|---------|---------|
| LITE | `deploy_lite_web_service()` | Caddy |
| STANDARD | `deploy_lite_web_service()` | Caddy |
| INGRESS | Full-mode (returns False, requires Traefik) | Traefik |
| PRODUCTION | Full-mode (returns False, requires Traefik) | Traefik |

## Tier Detection

1. If `cluster_tier` is explicitly provided, use it directly
2. Otherwise, query Nomad via `nomad node status -json`
3. Build `NodeInfo` list from node data
4. Use `detect_cluster_tier()` to determine tier
5. On any failure, defaults to `PRODUCTION` (safe fallback)

## Dependencies

- `src.infrastructure.progressive_activation.tier_config` — ClusterTier, TierConfig
- `src.infrastructure.progressive_activation.tier_manager` — detect_cluster_tier, NodeInfo
- `src.workloads.deploy_lite_web_service.deploy` — deploy_lite_web_service (lite/standard tiers)
- `src.workloads.deploy_lite_ingress.deploy` — deploy_lite_ingress (indirect, via Caddy routing)

## Tests

- [x] Test: Lite tier routes to deploy_lite_web_service
- [x] Test: Standard tier routes to deploy_lite_web_service
- [x] Test: Ingress tier returns False with Traefik message
- [x] Test: Production tier returns False with Traefik message
- [x] Test: Explicit cluster_tier overrides auto-detection
- [x] Test: Single Nomad node detected as LITE
- [x] Test: Multiple Nomad nodes same dc detected as STANDARD
- [x] Test: Subprocess failure defaults to PRODUCTION
