from dataclasses import dataclass
from typing import List, Optional


@dataclass
class NodeInfo:
    name: str
    provider: str
    region: str
    role: str
    is_spot: bool = False


def detect_cluster_tier(nodes: List[NodeInfo], override_tier: Optional[str] = None) -> "TierConfig":
    """Detect cluster tier from node topology.

    Logic:
    - If override_tier is provided, use it directly
    - 1 node -> LITE
    - 2+ nodes same region -> STANDARD
    - 2+ nodes different regions -> INGRESS
    - If any node has is_spot=True -> PRODUCTION
    """
    from .tier_config import ClusterTier, TierConfig

    if override_tier:
        tier = ClusterTier(override_tier)
        return TierConfig.from_tier(tier)

    if any(n.is_spot for n in nodes):
        return TierConfig.from_tier(ClusterTier.PRODUCTION)

    regions = set(n.region for n in nodes)
    if len(regions) > 1:
        return TierConfig.from_tier(ClusterTier.INGRESS)

    if len(nodes) <= 1:
        return TierConfig.from_tier(ClusterTier.LITE)

    return TierConfig.from_tier(ClusterTier.STANDARD)


class TierUpgradeError(Exception):
    """Raised when attempting to use a feature not available in the current tier."""

    pass
