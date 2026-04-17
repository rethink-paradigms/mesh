# API Reference

Public interfaces for all Mesh modules, based on CONTEXT.md design contracts.

---

## Infrastructure Domain

### `provision_node`

Provisions a generic compute node (VM or bare metal) on any provider.

```python
from mesh.infrastructure.provision_node import provision_node, GPUConfig, SpotConfig

result = provision_node(
    name="server-1",
    provider="digitalocean",
    role="server",           # "server" or "client"
    size="s-2vcpu-4gb",
    tailscale_auth_key="tskey-auth-xxx",
    leader_ip=None,          # Required for clients
    region="nyc3",
    gpu_config=None,         # Optional GPUConfig
    spot_config=None,        # Optional SpotConfig
    tier_config=None,        # Optional TierConfig
    depends_on_resources=[], # Pulumi resource dependencies
    opts=None,               # Pulumi resource options
)
# Returns: { public_ip: pulumi.Output[str], private_ip: pulumi.Output[str], instance_id: pulumi.Output[str] }
```

**Data classes:**

```python
@dataclass
class GPUConfig:
    enable_gpu: bool = True
    cuda_version: str = "12.1"
    nvidia_driver_version: str = "535"

@dataclass
class SpotConfig:
    enable_spot_handling: bool = False
    spot_check_interval: int = 5
    spot_grace_period: int = 90
```

---

### `providers`

Universal cloud node provisioning across 13+ cloud providers using Apache Libcloud.

```python
from mesh.infrastructure.providers.libcloud_dynamic_provider import UniversalCloudNode

# Pulumi Dynamic Resource — used internally by provision_node
# Inputs: provider, region, size_id, boot_script
# Outputs: public_ip, private_ip, instance_id, status
```

Discovery methods are **module-level functions** in `discovery.py`, not class methods:

```python
from mesh.infrastructure.providers.discovery import list_sizes, list_regions, list_images, get_size, is_region_available, find_ubuntu_image

list_sizes(provider, region)        # List available VM sizes
list_regions(provider)              # List available regions
list_images(provider, region)       # List available OS images
get_size(provider, region, id)      # Get specific size details
is_region_available(provider, r)    # Check region availability
find_ubuntu_image(provider, r)      # Find latest Ubuntu image
```

---

### `boot_consul_nomad`

Renders modular Jinja2 boot scripts for node bootstrap.

```python
from mesh.infrastructure.boot_consul_nomad import generate_boot_scripts

result = generate_boot_scripts(
    tailscale_key="tskey-auth-xxx",
    leader_ip="100.x.y.z",
    role="worker",           # "leader" or "worker"
    cluster_tier="production",
    enable_caddy=False,
)
# Returns: BootScripts(boot_script_sh=str, boot_script_cloud_init_yaml=str, status=str)
```

**Modular script phases:**

| Phase | Script | Purpose |
|:---|:---|:---|
| 01 | `install-deps.sh` | Docker, curl, base packages |
| 02 | `install-tailscale.sh` | WireGuard mesh networking |
| 03 | `install-hashicorp.sh` | Nomad, Consul binaries |
| 06 | `configure-consul.sh` | Service discovery |
| 07 | `configure-nomad.sh` | Container scheduling |
| 10 | `install-caddy.sh` | (Lite mode) HTTPS ingress |

---

### `configure_tailscale`

Generates Tailscale auth keys for mesh networking.

```python
from mesh.infrastructure.configure_tailscale import generate_auth_key

auth_key = generate_auth_key(
    key_name="mesh-cluster-key",
    ephemeral=True,          # Nodes removed when offline
    reusable=True,           # Key can be used multiple times
    tags=["tag:mesh"],       # ACL tags
)
# Returns: Pulumi Secret wrapping the auth key string
```

---

### `progressive_activation`

Defines cluster tier data model and auto-detection.

```python
from mesh.infrastructure.progressive_activation import (
    ClusterTier,     # Enum: LITE, STANDARD, INGRESS, PRODUCTION
    TierConfig,      # Dataclass with component enable flags
    NodeInfo,        # Dataclass for node metadata
    detect_cluster_tier,  # Auto-detect tier from node topology
    TierUpgradeError,     # Exception for invalid tier transitions
)

tier_config = detect_cluster_tier(
    nodes=[NodeInfo(role="leader"), NodeInfo(role="worker"), NodeInfo(role="worker")],
    override_tier=None,      # Optional manual override
)
# Returns: TierConfig(
#   tier=ClusterTier.INGRESS,
#   enable_tailscale=True, enable_consul=True,
#   enable_traefik=True, enable_caddy=False,
#   enable_telegraf=False, enable_spot_handler=False
# )
```

---

## Workloads Domain

### `deploy_app`

Tier-aware unified deployment dispatcher.

```python
from mesh.workloads.deploy_app import deploy_app

success = deploy_app(
    app_name="my-api",
    image="nginx",
    image_tag="latest",
    port=8080,
    domain="api.example.com",
    cpu=100,              # MHz
    memory=128,           # MB
    datacenter="dc1",
    cluster_tier=None,    # Auto-detected if None
    nomad_addr="http://leader:4646",
)
# Returns: bool (True if deployment succeeded)
```

Auto-detects cluster tier from running Nomad nodes and routes to:

- **LITE / STANDARD** → `deploy_lite_web_service` (Caddy routing)
- **INGRESS / PRODUCTION** → Returns `False` (requires Traefik setup first)

---

### `deploy_lite_web_service`

Deploys web services in lite mode using Caddy routing.

```python
from mesh.workloads.deploy_lite_web_service import deploy_lite_web_service

success = deploy_lite_web_service(
    app_name="myapp",
    image="nginx",
    image_tag="latest",
    port=80,
    domain="myapp.example.com",
    cpu=100,
    memory=256,
    datacenter="dc1",
    nomad_addr="http://100.x.y.z:4646",
    caddy_admin_addr="http://100.x.y.z:2019",
)
# Returns: bool
```

Uses host network mode and Nomad native service registration (no Consul).

---

### `deploy_lite_ingress`

Deploys Caddy as a lightweight HTTPS ingress for single-VM deployments.

```python
from mesh.workloads.deploy_lite_ingress import deploy_lite_ingress, LiteIngressConfig, RouteManager

config = LiteIngressConfig(
    acme_email="admin@example.com",
    caddy_image="caddy:2",
    memory=50,
    cpu=100,
    datacenter="dc1",
    log_level="INFO",
    nomad_addr="http://100.x.y.z:4646",
)

success = deploy_lite_ingress(config)
# Returns: bool

# Manage routes dynamically:
manager = RouteManager(nomad_addr="http://100.x.y.z:4646", caddy_admin_addr="http://100.x.y.z:2019")
manager.add_route("myapp.example.com", "100.x.y.z:8080")
manager.list_routes()
manager.remove_route("myapp.example.com")
```

---

### `deploy_web_service`

Standard deployment with Traefik ingress and Consul service discovery.

```python
from mesh.workloads.deploy_web_service import deploy_web_service

service_name = deploy_web_service(
    app_name="myapp",
    image="nginx",
    image_tag="latest",
    count=2,               # Number of instances
    port=80,
    host_rule="myapp.example.com",
    cpu=100,
    memory=256,
)
# Returns: str — registered Consul service name
```

Creates Nomad job with Consul service registration and Traefik routing tags.

---

### `deploy_traefik`

Deploys Traefik ingress controller with automatic Let's Encrypt TLS.

```python
from mesh.workloads.deploy_traefik import deploy_traefik

success = deploy_traefik(
    acme_email="admin@example.com",
    acme_ca_server="letsencrypt-production",   # or "letsencrypt-staging"
    acme_tls_challenge=True,
    acme_http_challenge=False,
    memory=256,
    cpu=200,
    nomad_addr="http://100.x.y.z:4646",
)
# Returns: bool
```

---

### `manage_secrets`

Syncs secrets from CI/CD platforms to Nomad Variables.

```python
from mesh.workloads.manage_secrets import sync_secrets

status = sync_secrets(
    job_name="myapp",
    secrets_json='{"DB_URL": "postgres://...", "API_KEY": "sk-xxx"}',
)
# Returns: "Success" or error message
```

Flow: GitHub Secrets → Nomad Variables → Nomad template stanza → Container env vars.

---

## Verification Domain

### E2E Test Suites

```python
# App deployment E2E tests
from mesh.verification.e2e_app_deployment import run_e2e_tests
# Scenarios: Deploy & Reach, Ingress Routing
# Input: target_env ("local"/"aws"), leader_ip
# Output: "Pass" / "Fail"

# Lite mode E2E tests
from mesh.verification.e2e_lite_mode import run_lite_e2e
# Scenarios: Boot Verification, HTTPS Certificate, Domain Routing,
#            Zero-Downtime Deploy, Memory Budget

# Multi-node E2E tests
from mesh.verification.e2e_multi_node_scenarios import run_multi_node_e2e
# Scenarios: Multi-node Scheduling, Worker Failure Rescheduling,
#            Cross-cloud Mesh Connectivity, Service Discovery,
#            Traefik Routing After Deployment
```

**Test markers:**

```bash
pytest -m "not e2e"              # Skip all E2E tests
pytest -m "e2e"                  # Only E2E tests
pytest -m "cloud_only"           # Only cloud tests
pytest -m "cross_cloud"          # Only cross-cloud tests
pytest -m "local_only"           # Only local tests
pytest -m "destructive"          # Tests that mutate infrastructure
```
