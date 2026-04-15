# Domain: Multi-Cloud Provider Support

**Description:**
Universal cloud node provisioning using Apache Libcloud with runtime discovery and exact value configuration. Enables infrastructure provisioning across 50+ cloud providers without hardcoded configuration files.

## Overview

The provider system uses Apache Libcloud's unified API to query providers at runtime for available options (sizes, regions, images). This eliminates the need for hardcoded configuration files and automatically stays current with provider offerings.

**Key Benefits:**
- **Runtime Discovery:** Query providers for available options in real-time
- **Exact Values:** Use provider-specific size_id, region, image_id without abstractions
- **No Configuration Files:** All metadata comes directly from provider APIs
- **Auto-Updating:** New instance types, regions, and images appear automatically
- **Simple Enum Mappings:** 40 lines of Python vs 386-line YAML registry

## Architecture

### Provider Enumeration

```python
PROVIDER_ENUMS: Dict[str, Provider] = {
    "aws": Provider.EC2,
    "digitalocean": Provider.DIGITAL_OCEAN,
    "gcp": Provider.GCE,
    "azure": Provider.AZURE_ARM,
    "linode": Provider.LINODE,
    "vultr": Provider.VULTR,
    "upcloud": Provider.UPCLOUD,
    # ... and 40+ more
}
```

### Credential Resolution

Credentials are auto-resolved from environment variables following cloud provider conventions:

```python
CREDENTIAL_ENV_VARS: Dict[str, List[str]] = {
    "aws": ["AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY"],
    "digitalocean": ["DIGITALOCEAN_API_TOKEN"],
    "gcp": ["GOOGLE_CREDENTIALS", "GOOGLE_APPLICATION_CREDENTIALS"],
    "azure": ["AZURE_CLIENT_ID", "AZURE_CLIENT_SECRET", ...],
}
```

## Usage Patterns

### Query Provider for Available Options

```python
from src.infrastructure.providers.discovery import list_sizes, list_regions

# List all available sizes for a provider
sizes = list_sizes("digitalocean")
for s in sizes:
    print(f"{s.id}: {s.name} - {s.ram}MB RAM")

# List all available regions
regions = list_regions("aws")
for r in regions:
    print(f"{r.id}: {r.name}")
```

### Validate Before Provisioning

```python
from src.infrastructure.providers.discovery import get_size, is_region_available

# Check if a specific size exists
size = get_size("digitalocean", "s-2vcpu-4gb")
if not size:
    raise ValueError("Invalid size_id")

# Check if a region is available
if not is_region_available("aws", "us-east-1"):
    raise ValueError("Invalid region")
```

### Provision with Exact Values

```python
from src.infrastructure.providers.libcloud_dynamic_provider import UniversalCloudNode

# Use exact size_id from provider's catalog
node = UniversalCloudNode(
    "my-worker",
    provider="digitalocean",
    region="nyc3",
    size_id="s-2vcpu-4gb",  # Exact size ID
    boot_script=boot_script_content
)

# Access outputs
pulumi.Output.all(node.public_ip, node.private_ip).apply(
    lambda args: print(f"Node public IP: {args[0]}, private IP: {args[1]}")
)
```

## Supported Providers

### Currently Mapped

| Provider | Enum | Credential Fields |
|:---------|:-----|:------------------|
| AWS | `Provider.EC2` | AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY |
| DigitalOcean | `Provider.DIGITAL_OCEAN` | DIGITALOCEAN_API_TOKEN |
| Google Cloud | `Provider.GCE` | GOOGLE_CREDENTIALS or GOOGLE_APPLICATION_CREDENTIALS |
| Azure | `Provider.AZURE_ARM` | AZURE_CLIENT_ID, AZURE_CLIENT_SECRET, AZURE_TENANT_ID, AZURE_SUBSCRIPTION_ID |
| Linode | `Provider.LINODE` | LINODE_API_KEY |
| Vultr | `Provider.VULTR` | VULTR_API_KEY |
| UpCloud | `Provider.UPCLOUD` | UPCLOUD_USERNAME, UPCLOUD_PASSWORD |
| Exoscale | `Provider.EXOSCALE` | EXOSCALE_API_KEY, EXOSCALE_API_SECRET |
| Scaleway | `Provider.SCALEWAY` | SCALEWAY_ACCESS_KEY, SCALEWAY_SECRET_KEY |
| OVHcloud | `Provider.OVH` | OVH_ENDPOINT, OVH_APPLICATION_KEY, OVH_APPLICATION_SECRET, OVH_CONSUMER_KEY |
| Equinix Metal | `Provider.EQUINIXMETAL` | EQUINIXMETAL_API_KEY, EQUINIXMETAL_PROJECT_ID |

### Adding New Providers

To add a provider, simply add one line to `PROVIDER_ENUMS`:

```python
PROVIDER_ENUMS: Dict[str, Provider] = {
    # ... existing providers
    "newprovider": Provider.NEWPROVIDER_ENUM,
}
```

Then add credential environment variables to `CREDENTIAL_ENV_VARS`:

```python
CREDENTIAL_ENV_VARS: Dict[str, List[str]] = {
    # ... existing providers
    "newprovider": ["NEWPROVIDER_API_KEY"],
}
```

See full list of supported providers: https://libcloud.readthedocs.io/en/stable/compute/supported_providers.html

## Size Specification

### Exact Size IDs

Use exact size IDs from provider catalogs:

```python
# AWS: Use instance types
size_id = "t3.nano"      # 2 vCPU, 0.5GB RAM
size_id = "t3.micro"     # 1 vCPU, 1GB RAM
size_id = "t3.small"     # 2 vCPU, 2GB RAM
size_id = "t3.medium"    # 2 vCPU, 4GB RAM
size_id = "m5.large"     # 2 vCPU, 8GB RAM

# DigitalOcean: Use size slugs
size_id = "s-1vcpu-1gb"  # 1 vCPU, 1GB RAM
size_id = "s-2vcpu-4gb"  # 2 vCPU, 4GB RAM
size_id = "c-2vcpu-4gb"  # 2 vCPU, 4GB RAM (optimized)

# Linode: Use type IDs
size_id = "g6-nanode-1"  # 1 vCPU, 1GB RAM
size_id = "g6-standard-2"  # 1 vCPU, 2GB RAM

# Vultr: Use plan IDs
size_id = "vc2-1c-1gb"   # 1 vCPU, 1GB RAM
size_id = "vc2-2c-4gb"   # 2 vCPU, 4GB RAM
```

### Discovery Methods

To see available sizes for a provider:

```python
from src.infrastructure.providers.discovery import list_sizes

sizes = list_sizes("digitalocean")
for s in sizes:
    print(f"{s.id}: {s.name} - {s.ram}MB RAM, {s.extra.get('vcpus')} vCPUs")
```

## Image Selection

### Auto-Discovery

Ubuntu 22.04 is auto-discovered by default:

```python
# Image is automatically discovered
node = UniversalCloudNode(
    "my-node",
    provider="digitalocean",
    region="nyc3",
    size_id="s-2vcpu-4gb",
    boot_script=boot_script_content
    # image_id not specified - auto-discovers Ubuntu 22.04
)
```

### Explicit Image Selection

Specify a specific image ID:

```python
node = UniversalCloudNode(
    "my-node",
    provider="digitalocean",
    region="nyc3",
    size_id="s-2vcpu-4gb",
    image_id="ubuntu-22-04-x64",  # Explicit image
    boot_script=boot_script_content
)
```

### Image Discovery

To see available images for a provider:

```python
from src.infrastructure.providers.discovery import list_images

images = list_images("digitalocean")
for img in images:
    print(f"{img.id}: {img.name}")
```

## Integration with provision_node()

The provider system integrates with `provision_node()` dispatcher:

```python
from src.infrastructure.provision_node import provision_node

# Cloud provider - region is required
node = provision_node(
    name="worker-1",
    provider="digitalocean",
    role="client",
    size="s-2vcpu-4gb",  # Exact size ID
    tailscale_auth_key=auth_key,
    leader_ip="100.100.100.1",
    region="nyc3"  # Required for cloud providers
)

# Multipass - no region needed
node = provision_node(
    name="local-worker",
    provider="multipass",
    role="client",
    size="2CPU,1GB",
    tailscale_auth_key=auth_key,
    leader_ip="100.100.100.1"
)
```

## Design Decisions

### Runtime Discovery vs. Static Configuration

**Decision:** Query provider APIs at runtime instead of using static configuration files.

**Rationale:**
- New instance types and regions appear automatically
- No configuration drift between code and reality
- Single source of truth (provider API)
- Eliminates 386-line YAML registry
- Reduces maintenance burden

### Exact Values vs. Size Tiers

**Decision:** Use exact size_id values instead of small/medium/large abstractions.

**Rationale:**
- Providers don't have uniform size tiers
- Users know exactly what they're getting
- No ambiguity or surprises
- Aligns with provider terminology
- Simpler implementation

### Environment Variable Credentials

**Decision:** Resolve credentials from environment variables following cloud provider conventions.

**Rationale:**
- 12-factor app methodology
- No secret management infrastructure needed
- Users already set these for provider CLIs
- Enables credential override via credentials parameter

## Dependencies

### Internal
- `provision_node/` - Uses UniversalCloudNode for cloud providers
- `boot_consul_nomad/` - Boot script generation (unchanged)

### External
- **Apache Libcloud** (>=3.8.0) - Provider driver implementations
- **Pulumi** - Dynamic Resource API

## Testing Strategy

### Unit Tests (27 tests)
- Provider enum validation
- Credential resolution
- Size/region/image discovery
- Dynamic Resource create/delete/read
- Error handling for invalid inputs

### Integration Tests (20 tests)
- Dispatcher routing to Libcloud
- Boot script pass-through
- Output mapping
- Backward compatibility

## API Reference

### UniversalCloudNode

Pulumi Dynamic Resource for multi-cloud node provisioning.

**Inputs:**
- `provider` (str): Cloud provider ID
- `region` (str): Provider region/zone
- `size_id` (str): Exact instance size ID
- `boot_script` (str): Cloud-init/userdata script
- `image_id` (str, optional): Image ID (auto-discovers Ubuntu if not specified)
- `credentials` (dict, optional): Credential overrides
- `node_name` (str, optional): Node name

**Outputs:**
- `public_ip` (pulumi.Output[str]): Public IP address
- `private_ip` (pulumi.Output[str]): Private IP address
- `instance_id` (pulumi.Output[str]): Provider instance ID
- `status` (pulumi.Output[str]): Node status

### Discovery Methods

```python
list_sizes(provider_id: str, region: str = None) -> List[NodeSize]
list_regions(provider_id: str) -> List[NodeLocation]
list_images(provider_id: str) -> List[NodeImage]
get_size(provider_id: str, size_id: str, region: str = None) -> Optional[NodeSize]
is_region_available(provider_id: str, region: str) -> bool
find_ubuntu_image(provider_id: str, version: str = "22.04") -> Optional[NodeImage]
```

## Troubleshooting

**Provider Not Supported:**
```python
# Check PROVIDER_ENUMS in __init__.py
from src.infrastructure.providers import list_providers
print(list_providers())
```

**Invalid Size ID:**
```python
# Query provider for available sizes
from src.infrastructure.providers.discovery import list_sizes
sizes = list_sizes("digitalocean")
for s in sizes:
    print(f"{s.id}: {s.name}")
```

**Credentials Missing:**
```python
# Check expected environment variables
import os
from src.infrastructure.providers import CREDENTIAL_ENV_VARS
for var in CREDENTIAL_ENV_VARS.get("aws", []):
    print(f"{var}: {os.getenv(var)}")
```
