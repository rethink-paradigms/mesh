# Feature: Deploy Caddy as Lite Ingress

**Description:**
Deploys Caddy as a lightweight HTTPS ingress for single-VM deployments. Uses ~25MB RAM vs Traefik's ~256MB, ideal for memory-constrained nodes.

## Interface

### Python API

**Function:** `deploy_lite_ingress()`

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `config` | `LiteIngressConfig` | required | Configuration dataclass |

**LiteIngressConfig fields:**

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `acme_email` | `str` | required | Email for Let's Encrypt notifications |
| `caddy_image` | `str` | `"caddy:2"` | Caddy Docker image |
| `memory` | `int` | `25` | Memory allocation in MB |
| `cpu` | `int` | `100` | CPU allocation in MHz |
| `datacenter` | `str` | `"dc1"` | Nomad datacenter name |
| `log_level` | `str` | `"INFO"` | Caddy log level |
| `nomad_addr` | `Optional[str]` | `None` | Nomad server address |

**Returns:** `bool` - True if deployment succeeded

### RouteManager API

| Method | Parameters | Returns | Description |
|--------|-----------|---------|-------------|
| `add_route` | domain, upstream_host, upstream_port | `bool` | Add reverse proxy route via Caddy admin API |
| `remove_route` | domain | `bool` | Remove route by domain |
| `update_route` | domain, upstream_host, upstream_port | `bool` | Update existing route (remove + add) |
| `list_routes` | - | `list` | List all current routes |

### Nomad Job Template

**File:** `lite_ingress.nomad.hcl`

## Dependencies

- Nomad cluster running
- Caddy Docker image (`caddy:2`)
- Public IP address or domain name
- Host volume `caddy-data` for certificate persistence
- Port 80, 443, 2019 accessible

## Tests

- [x] Test: LiteIngressConfig defaults
- [x] Test: LiteIngressConfig custom values
- [x] Test: Deploy with missing job file
- [x] Test: Nomad HCL template valid syntax
- [x] Test: Template has correct ports (80, 443, 2019)
- [x] Test: Template has HTTP redirect
- [x] Test: RouteManager add route
- [x] Test: RouteManager remove route
- [x] Test: RouteManager list routes
- [x] Test: RouteManager retry on failure

## Example Usage

### Python API
```python
from src.workloads.deploy_lite_ingress import deploy_lite_ingress
from src.workloads.deploy_lite_ingress.deploy import LiteIngressConfig
from src.workloads.deploy_lite_ingress.route_manager import RouteManager

config = LiteIngressConfig(
    acme_email="admin@example.com",
    memory=25
)
deploy_lite_ingress(config)

rm = RouteManager("http://127.0.0.1:2019")
rm.add_route("app.example.com", "10.0.0.5", 8080)
```

### CLI
```bash
python src/workloads/deploy_lite_ingress/deploy.py \
    --acme-email admin@example.com \
    --memory 25
```

### Nomad CLI
```bash
nomad job run \
  -var="acme_email=admin@example.com" \
  src/workloads/deploy_lite_ingress/lite_ingress.nomad.hcl
```

## How It Works

1. **Deployment**: Caddy deployed as Nomad system job on server nodes
2. **Automatic HTTPS**: Caddy auto-provisions Let's Encrypt certificates
3. **HTTP Redirect**: Port 80 redirects to HTTPS automatically
4. **Admin API**: Port 2019 for dynamic route management
5. **Certificate Storage**: Persisted in host volume `caddy-data`

## Resource Comparison

| Component | Memory | Use Case |
|-----------|--------|----------|
| Caddy (lite) | 25 MB | Single-VM, minimal footprint |
| Traefik | 256 MB | Multi-node, Consul service discovery |
