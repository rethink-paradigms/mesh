# Feature: Deploy Lite Web Service

**Description:**
Deploys web services in lite mode without Consul service registration or Traefik ingress. Uses Nomad native service registration and Caddy for domain routing.

## Interface

### Python API

**Function:** `deploy_lite_web_service()`

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `app_name` | `str` | required | Application name |
| `image` | `str` | required | Docker image |
| `image_tag` | `str` | `"latest"` | Docker image tag |
| `port` | `int` | `8080` | Application port |
| `domain` | `str` | `None` | Domain for Caddy routing |
| `cpu` | `int` | `100` | CPU allocation in MHz |
| `memory` | `int` | `128` | Memory allocation in MB |
| `datacenter` | `str` | `"dc1"` | Nomad datacenter |
| `nomad_addr` | `str` | `None` | Nomad server address |
| `caddy_admin_addr` | `str` | `"http://127.0.0.1:2019"` | Caddy admin API address |

**Returns:** `bool` - True if deployment succeeded

## Key Differences from deploy_web_service

| Aspect | deploy_web_service | deploy_lite_web_service |
|--------|--------------------|-------------------------|
| Service discovery | Consul | Nomad native |
| Ingress | Traefik (tags) | Caddy (route registration) |
| Network mode | bridge | host |
| Service tags | traefik.enable=true | None |
| Secrets injection | nomadVar template | None |

## Dependencies

- Nomad cluster running
- Caddy (for domain route registration, optional)

## Tests

- [x] Test: Nomad job template has valid syntax
- [x] Test: Template has no Consul-specific tags
- [x] Test: Template has no Traefik tags
- [x] Test: Template uses Nomad native service registration
- [x] Test: Template variables are all present
- [x] Test: Template uses host network mode
- [x] Test: Deploy returns False when job file missing
- [x] Test: Deploy succeeds and registers Caddy route when domain provided
- [x] Test: Deploy succeeds without Caddy when no domain

## Example Usage

### Python API
```python
from src.workloads.deploy_lite_web_service import deploy_lite_web_service

deploy_lite_web_service(
    app_name="my-api",
    image="my-api",
    image_tag="v1.0",
    port=8080,
    domain="api.example.com",
)
```

### CLI
```bash
python src/workloads/deploy_lite_web_service/deploy.py \
    --app-name my-api \
    --image my-api \
    --domain api.example.com
```
