# Feature: Provision Node

**Description:**
Provisions a generic compute node (VM or bare metal) on a specified provider, capable of running Nomad and Consul. This feature acts as an abstraction layer for different compute providers.

## Overview

The `provision_node()` function is the core abstraction for compute provisioning across multiple providers. It handles provider-specific differences while presenting a unified interface.

**Supported Providers:**
- **multipass**: Local virtual machines via Multipass CLI
- **aws**: Amazon Web Services EC2 (via Libcloud)
- **digitalocean**: DigitalOcean droplets (via Libcloud)
- **gcp**: Google Cloud Platform GCE (via Libcloud)
- **azure**: Microsoft Azure VMs (via Libcloud)
- **linode**: Linode cloud instances (via Libcloud)
- **vultr**: Vultr cloud instances (via Libcloud)
- **upcloud**: UpCloud instances (via Libcloud)
- And 40+ more providers via Apache Libcloud

## Interface

### Function Signature

```python
def provision_node(
    name: str,
    provider: str,
    role: str,
    size: str,
    tailscale_auth_key: pulumi.Output[str],
    leader_ip: str,
    region: Optional[str] = None,
    gpu_config: Optional[GPUConfig] = None,
    spot_config: Optional[SpotConfig] = None,
    tier_config: Optional[TierConfig] = None,
    depends_on_resources: Optional[List[pulumi.Resource]] = None,
    opts: Optional[pulumi.ResourceOptions] = None
) -> Dict[str, Any]
```

### Parameters

| Parameter | Type | Required | Description |
|:----------|:-----|:---------|:------------|
| `name` | `str` | Yes | Unique name/identifier for the node |
| `provider` | `str` | Yes | Compute provider ("multipass", "aws", "digitalocean", etc.) |
| `role` | `str` | Yes | Node role: "server" or "client" |
| `size` | `str` | Yes | Exact instance size ID (e.g., "t3.small", "s-2vcpu-4gb") |
| `tailscale_auth_key` | `pulumi.Output[str]` | Yes | Tailscale auth key for mesh network joining |
| `leader_ip` | `str` | Yes | IP of Leader node for cluster joining |
| `region` | `str` | No | Cloud region (required for cloud providers) |
| `gpu_config` | `GPUConfig` | No | GPU configuration for GPU worker nodes |
| `spot_config` | `SpotConfig` | No | Spot instance interruption handling configuration |
| `tier_config` | `TierConfig` | No | Progressive activation tier configuration |
| `depends_on_resources` | `List[pulumi.Resource]` | No | Resources this node depends on |
| `opts` | `pulumi.ResourceOptions` | No | Pulumi resource options |

### Return Value

```python
{
    "public_ip": pulumi.Output[str],   # Public IP address
    "private_ip": pulumi.Output[str],  # Private IP address
    "instance_id": pulumi.Output[str]  # Provider instance ID
}
```

### Optional Configuration Classes

**GPUConfig** - GPU worker node configuration:
```python
@dataclass
class GPUConfig:
    enable_gpu: bool = True              # Enable GPU support
    cuda_version: str = "12.1"           # CUDA runtime version
    nvidia_driver_version: str = "535"   # NVIDIA driver version
```

**SpotConfig** - Spot instance handling:
```python
@dataclass
class SpotConfig:
    enable_spot_handling: bool = False      # Enable spot handling
    spot_check_interval: int = 5            # Polling interval (seconds)
    spot_grace_period: int = 90             # Grace period for drain (seconds)
```

## Usage Patterns

### Multipass (Local Development)

```python
from src.infrastructure.provision_node import provision_node

# Local VM - no region needed
node = provision_node(
    name="local-worker",
    provider="multipass",
    role="client",
    size="2CPU,1GB",  # Multipass format
    tailscale_auth_key=auth_key,
    leader_ip="100.100.100.1"
)

# Access outputs
node["public_ip"].apply(lambda ip: print(f"Worker IP: {ip}"))
```

### Cloud Provider (DigitalOcean)

```python
# Query provider for available sizes first
from src.infrastructure.providers.discovery import list_sizes

sizes = list_sizes("digitalocean")
for s in sizes:
    print(f"{s.id}: {s.name} - {s.ram}MB RAM")

# Provision with exact size_id
node = provision_node(
    name="worker-1",
    provider="digitalocean",
    role="client",
    size="s-2vcpu-4gb",  # Exact size ID
    tailscale_auth_key=auth_key,
    leader_ip="100.100.100.1",
    region="nyc3"  # Required for cloud providers
)
```

### Cloud Provider (AWS)

```python
# Query AWS for available sizes
sizes = list_sizes("aws", region="us-east-1")
for s in sizes:
    print(f"{s.id}: {s.name} - {s.ram}MB RAM")

# Provision with exact instance type
node = provision_node(
    name="worker-2",
    provider="aws",
    role="client",
    size="t3.medium",  # Exact instance type
    tailscale_auth_key=auth_key,
    leader_ip="100.100.100.1",
    region="us-east-1"  # Required for AWS
)
```

### GPU Worker Node

```python
from src.infrastructure.provision_node import provision_node, GPUConfig

node = provision_node(
    name="gpu-worker",
    provider="aws",
    role="client",
    size="g4dn.xlarge",  # GPU instance type
    tailscale_auth_key=auth_key,
    leader_ip="100.100.100.1",
    region="us-east-1",
    gpu_config=GPUConfig(
        enable_gpu=True,
        cuda_version="12.1",
        nvidia_driver_version="535"
    )
)
```

### Spot Instance with Graceful Drain

```python
from src.infrastructure.provision_node import provision_node, SpotConfig

node = provision_node(
    name="spot-worker",
    provider="aws",
    role="client",
    size="t3.small",
    tailscale_auth_key=auth_key,
    leader_ip="100.100.100.1",
    region="us-east-1",
    spot_config=SpotConfig(
        enable_spot_handling=True,
        spot_check_interval=5,
        spot_grace_period=90
    )
)
```

## Size Specification by Provider

### AWS (Amazon Web Services)
Use EC2 instance types:
- `t3.nano`: 2 vCPU, 0.5GB RAM
- `t3.micro`: 1 vCPU, 1GB RAM
- `t3.small`: 2 vCPU, 2GB RAM
- `t3.medium`: 2 vCPU, 4GB RAM
- `m5.large`: 2 vCPU, 8GB RAM
- `g4dn.xlarge`: 4 vCPU, 16GB RAM, 1 GPU

### DigitalOcean
Use size slugs:
- `s-1vcpu-1gb`: 1 vCPU, 1GB RAM
- `s-2vcpu-2gb`: 2 vCPU, 2GB RAM
- `s-2vcpu-4gb`: 2 vCPU, 4GB RAM
- `c-2vcpu-4gb`: 2 vCPU, 4GB RAM (optimized compute)

### Linode
Use type IDs:
- `g6-nanode-1`: 1 vCPU, 1GB RAM
- `g6-standard-2`: 1 vCPU, 2GB RAM
- `g6-standard-4`: 2 vCPU, 4GB RAM

### Vultr
Use plan IDs:
- `vc2-1c-1gb`: 1 vCPU, 1GB RAM
- `vc2-2c-4gb`: 2 vCPU, 4GB RAM

### Multipass
Use CPU,RAM format:
- `1CPU,512MB`: 1 vCPU, 512MB RAM
- `2CPU,1GB`: 2 vCPU, 1GB RAM
- `4CPU,8GB`: 4 vCPU, 8GB RAM

## Discovery and Validation

Before provisioning, you can query providers for available options:

```python
from src.infrastructure.providers.discovery import (
    list_sizes,
    list_regions,
    get_size,
    is_region_available
)

# List all available sizes
sizes = list_sizes("digitalocean")
for s in sizes:
    print(f"{s.id}: {s.name} - {s.ram}MB RAM")

# List all available regions
regions = list_regions("aws")
for r in regions:
    print(f"{r.id}: {r.name}")

# Validate a specific size exists
size = get_size("digitalocean", "s-2vcpu-4gb")
if size:
    print(f"Valid: {size.name}")

# Check if a region is available
if is_region_available("aws", "us-east-1"):
    print("Region is available")
```

## Provider Routing

The function routes to appropriate implementation based on provider:

```python
if provider == "multipass":
    # Route to Multipass adapter
    return multipass_provider.provision_multipass_node(...)

elif provider == "bare-metal":
    # Not yet implemented
    raise NotImplementedError(...)

else:
    # Route to Libcloud dynamic provider
    # Requires region parameter
    if not region:
        raise ValueError(f"region is required for cloud provider '{provider}'")

    return _provision_via_libcloud(
        provider=provider,
        region=region,
        size_id=size,
        ...
    )
```

## Error Handling

### Unknown Provider
```python
try:
    node = provision_node(
        name="worker",
        provider="unknown-provider",
        ...
    )
except ValueError as e:
    print(f"Error: {e}")
    # Error: Unknown provider: unknown-provider
```

### Missing Region (Cloud Providers)
```python
try:
    node = provision_node(
        name="worker",
        provider="digitalocean",
        ...
        # region not specified
    )
except ValueError as e:
    print(f"Error: {e}")
    # Error: region is required for cloud provider 'digitalocean'
```

### Invalid Size ID
```python
try:
    node = provision_node(
        name="worker",
        provider="digitalocean",
        region="nyc3",
        size="invalid-size-id",
        ...
    )
except ValueError as e:
    print(f"Error: {e}")
    # Error: Invalid size_id 'invalid-size-id' for provider digitalocean
```

## Dependencies

### Internal
- `providers/` - UniversalCloudNode for cloud providers
- `boot_consul_nomad/` - Boot script generation
- `multipass/` - Multipass adapter implementation

### External
- **Apache Libcloud** - Multi-cloud provider abstraction
- **Pulumi** - Infrastructure as Code framework
- **Multipass CLI** - Local VM provisioning

## Tests

### Unit Tests (20 tests)

**Dispatcher Routing:**
- test_multipass_routes_to_multipass_adapter
- test_aws_routes_to_libcloud
- test_digitalocean_routes_to_libcloud
- test_unknown_provider_raises_error
- test_bare_metal_raises_not_implemented

**Libcloud Integration:**
- test_universal_cloud_node_instantiated
- test_boot_script_passed_through
- test_credentials_resolved_correctly
- test_outputs_returned_in_correct_format
- test_resource_options_propagated
- test_region_parameter_passed
- test_gpu_config_passed_to_boot_script
- test_spot_config_passed_to_boot_script
- test_depends_on_resources_propagated

**Backward Compatibility:**
- test_multipass_calls_still_work
- test_aws_calls_migrate_transparently

**General Tests:**
- test_provision_node_aws_dispatch
- test_provision_node_multipass_dispatch
- test_provision_node_unknown_provider
- test_provision_node_digitalocean_dispatch

## Implementation Details

### Multipass Path

Multipass uses local VMs via CLI commands:
- Creates VM with specified CPU and RAM
- Injects boot script via cloud-init
- Returns IP from Multipass CLI output

### Libcloud Path

Cloud providers use Pulumi Dynamic Resources:
1. Validates provider is supported
2. Validates region exists for provider
3. Validates size_id exists for provider
4. Auto-discovers Ubuntu 22.04 image (or uses explicit image_id)
5. Resolves credentials from environment
6. Creates node via Libcloud driver
7. Returns node outputs (public_ip, private_ip, instance_id)

## Boot Script Integration

The function integrates with `boot_consul_nomad/generate_boot_scripts.py` to generate node initialization scripts. The boot script is passed through to all providers:

- **Multipass**: Injected via cloud-init
- **Libcloud**: Passed via `ex_userdata` parameter

The boot script handles:
- Tailscale mesh network joining
- Consul service discovery configuration
- Nomad scheduler configuration
- GPU driver installation (if gpu_config specified)
- Spot interruption handling (if spot_config specified)

## Design Decisions

### Exact Size IDs vs. Size Tiers

**Decision:** Use exact size_id values instead of small/medium/large abstractions.

**Rationale:**
- Cloud providers don't have uniform size tiers
- Users know exactly what they're provisioning
- Aligns with provider terminology
- No ambiguity in resource allocation
- Simpler implementation without mapping logic

### Region Required for Cloud Providers

**Decision:** Require region parameter for all cloud providers.

**Rationale:**
- Pricing varies by region
- Instance availability varies by region
- Network latency depends on region
- Explicit is better than implicit

### Environment Variable Credentials

**Decision:** Resolve credentials from environment variables.

**Rationale:**
- 12-factor app methodology
- No secret management infrastructure needed
- Consistent with cloud provider CLIs
- Supports credential override for testing
