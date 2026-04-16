from dataclasses import dataclass
from enum import Enum


class ClusterTier(Enum):
    LITE = "lite"
    STANDARD = "standard"
    INGRESS = "ingress"
    PRODUCTION = "production"


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
    def from_tier(cls, tier: ClusterTier) -> "TierConfig":
        configs = {
            ClusterTier.LITE: cls(
                tier=tier,
                enable_tailscale=False,
                enable_consul=False,
                enable_traefik=False,
                enable_telegraf=False,
                enable_caddy=True,
            ),
            ClusterTier.STANDARD: cls(
                tier=tier,
                enable_tailscale=True,
                enable_consul=True,
                enable_traefik=False,
                enable_telegraf=False,
                enable_caddy=True,
            ),
            ClusterTier.INGRESS: cls(
                tier=tier,
                enable_tailscale=True,
                enable_consul=True,
                enable_traefik=True,
                enable_telegraf=False,
                enable_caddy=False,
            ),
            ClusterTier.PRODUCTION: cls(
                tier=tier,
                enable_tailscale=True,
                enable_consul=True,
                enable_traefik=True,
                enable_telegraf=True,
                enable_caddy=False,
            ),
        }
        return configs[tier]
