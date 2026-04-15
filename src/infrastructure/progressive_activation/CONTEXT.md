# Feature: Progressive Activation - Tier Data Model

**Description:**
Defines the cluster tier data model that determines which platform components are activated based on cluster topology. This is the foundation for progressive activation, allowing the platform to scale from a single-node LITE tier to a full PRODUCTION deployment.

## Overview

The tier system automatically detects the appropriate cluster configuration based on node topology (node count, regions, spot instances) and enables only the components needed for that tier. This ensures memory-efficient deployments at every scale.

**Tiers:**
- **LITE**: Single node, minimal overhead (~200MB). Uses Caddy for ingress, no mesh networking.
- **STANDARD**: Multi-node, same region (~330MB). Adds Tailscale mesh and Consul service discovery.
- **INGRESS**: Multi-region (~590MB). Adds Traefik for TLS termination and cross-region routing.
- **PRODUCTION**: Full stack (~530MB + monitoring). Adds Telegraf metrics, spot instance handling.

## Interface

### TierConfig

```python
@dataclass
class TierConfig:
    tier: ClusterTier = ClusterTier.PRODUCTION
    enable_tailscale: bool = True
    enable_consul: bool = True
    enable_traefik: bool = True
    enable_telegraf: bool = True
    enable_caddy: bool = False
    enable_spot_handler: bool = False

    @classmethod
    def from_tier(cls, tier: ClusterTier) -> "TierConfig"
```

| Field | Type | Default | Description |
|:------|:-----|:---------|:------------|
| `tier` | `ClusterTier` | `PRODUCTION` | The cluster tier |
| `enable_tailscale` | `bool` | `True` | Enable Tailscale mesh networking |
| `enable_consul` | `bool` | `True` | Enable Consul service discovery |
| `enable_traefik` | `bool` | `True` | Enable Traefik ingress controller |
| `enable_telegraf` | `bool` | `True` | Enable Telegraf metrics collection |
| `enable_caddy` | `bool` | `False` | Enable Caddy lightweight ingress |
| `enable_spot_handler` | `bool` | `False` | Enable spot instance handling |

### ClusterTier

```python
class ClusterTier(Enum):
    LITE = "lite"
    STANDARD = "standard"
    INGRESS = "ingress"
    PRODUCTION = "production"
```

### NodeInfo

```python
@dataclass
class NodeInfo:
    name: str
    provider: str
    region: str
    role: str        # "server" or "client"
    is_spot: bool = False
```

### detect_cluster_tier

```python
def detect_cluster_tier(
    nodes: List[NodeInfo],
    override_tier: Optional[str] = None
) -> TierConfig
```

| Parameter | Type | Required | Description |
|:----------|:-----|:---------|:------------|
| `nodes` | `List[NodeInfo]` | Yes | List of cluster nodes |
| `override_tier` | `Optional[str]` | No | Explicit tier override (skips detection) |

**Detection Logic:**
1. If `override_tier` is provided, use it directly
2. If any node has `is_spot=True` → PRODUCTION
3. If nodes span multiple regions → INGRESS
4. If 1 node → LITE
5. If 2+ nodes, same region → STANDARD

### TierUpgradeError

```python
class TierUpgradeError(Exception):
    """Raised when attempting to use a feature not available in the current tier."""
```

## Tier Component Matrix

| Component | LITE | STANDARD | INGRESS | PRODUCTION |
|:----------|:-----|:---------|:---------|:-----------|
| Tailscale | No | Yes | Yes | Yes |
| Consul | No | Yes | Yes | Yes |
| Traefik | No | No | Yes | Yes |
| Telegraf | No | No | No | Yes |
| Caddy | Yes | Yes | No | No |
| Spot Handler | No | No | No | No |

## Usage Examples

### LITE Tier (Single Node)

```python
from src.infrastructure.progressive_activation import TierConfig, ClusterTier

config = TierConfig.from_tier(ClusterTier.LITE)
# enable_caddy=True, all others False

# Auto-detection
from src.infrastructure.progressive_activation import detect_cluster_tier, NodeInfo

nodes = [NodeInfo(name="node-1", provider="aws", region="us-east-1", role="server")]
config = detect_cluster_tier(nodes)
assert config.tier == ClusterTier.LITE
```

### STANDARD Tier (Multi-Node, Same Region)

```python
nodes = [
    NodeInfo(name="leader", provider="aws", region="us-east-1", role="server"),
    NodeInfo(name="worker-1", provider="aws", region="us-east-1", role="client"),
]
config = detect_cluster_tier(nodes)
assert config.tier == ClusterTier.STANDARD
assert config.enable_tailscale is True
assert config.enable_caddy is True
```

### INGRESS Tier (Multi-Region)

```python
nodes = [
    NodeInfo(name="leader", provider="aws", region="us-east-1", role="server"),
    NodeInfo(name="worker-eu", provider="aws", region="eu-west-1", role="client"),
]
config = detect_cluster_tier(nodes)
assert config.tier == ClusterTier.INGRESS
assert config.enable_traefik is True
```

### PRODUCTION Tier (Spot Instances)

```python
nodes = [
    NodeInfo(name="leader", provider="aws", region="us-east-1", role="server"),
    NodeInfo(name="spot-worker", provider="aws", region="us-east-1", role="client", is_spot=True),
]
config = detect_cluster_tier(nodes)
assert config.tier == ClusterTier.PRODUCTION
assert config.enable_telegraf is True
```

### Explicit Override

```python
nodes = [NodeInfo(name="node-1", provider="aws", region="us-east-1", role="server")]
config = detect_cluster_tier(nodes, override_tier="production")
assert config.tier == ClusterTier.PRODUCTION
```

## Dependencies

None. This is a foundation module with no external or internal dependencies.

## Tests

### Unit Tests (12 tests)

**Tier Configuration (6 tests):**
- test_lite_tier_enables_caddy_disables_others
- test_standard_tier_enables_caddy_and_consul
- test_ingress_tier_enables_traefik_disables_caddy
- test_production_tier_enables_all_except_caddy
- test_default_tier_is_production
- test_tier_config_from_tier_returns_correct_config

**Tier Detection (5 tests):**
- test_detect_single_node_is_lite
- test_detect_multi_node_same_region_is_standard
- test_detect_multi_region_is_ingress
- test_detect_spot_nodes_is_production
- test_override_tier_takes_precedence

**Error Handling (1 test):**
- test_tier_upgrade_error_is_exception
