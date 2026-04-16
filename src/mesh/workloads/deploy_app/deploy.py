import json
import subprocess
from typing import Optional

from mesh.infrastructure.progressive_activation.tier_config import (
    ClusterTier,
    TierConfig,
)
from mesh.infrastructure.progressive_activation.tier_manager import (
    NodeInfo,
    detect_cluster_tier,
)


def _detect_tier_from_nomad(nomad_addr: Optional[str] = None) -> ClusterTier:
    try:
        cmd = ["nomad", "node", "status", "-json"]
        if nomad_addr:
            cmd.extend(["-address", nomad_addr])
        result = subprocess.run(cmd, capture_output=True, text=True, timeout=10)
        if result.returncode != 0:
            return ClusterTier.PRODUCTION

        nodes = json.loads(result.stdout)
        if not nodes:
            return ClusterTier.LITE

        node_infos = []
        for n in nodes:
            node_infos.append(
                NodeInfo(
                    name=n.get("Name", ""),
                    provider="nomad",
                    region=n.get("Datacenter", "dc1"),
                    role=n.get("Role", "client"),
                    is_spot=False,
                )
            )

        config = detect_cluster_tier(node_infos)
        return config.tier
    except Exception:
        return ClusterTier.PRODUCTION


def deploy_app(
    app_name: str,
    image: str,
    image_tag: str = "latest",
    port: int = 8080,
    domain: Optional[str] = None,
    cpu: int = 100,
    memory: int = 128,
    datacenter: str = "dc1",
    cluster_tier: Optional[str] = None,
    nomad_addr: Optional[str] = None,
) -> bool:
    tier = (
        ClusterTier(cluster_tier)
        if cluster_tier
        else _detect_tier_from_nomad(nomad_addr)
    )
    tier_config = TierConfig.from_tier(tier)

    if tier in (ClusterTier.LITE, ClusterTier.STANDARD):
        from mesh.workloads.deploy_lite_web_service.deploy import deploy_lite_web_service

        return deploy_lite_web_service(
            app_name=app_name,
            image=image,
            image_tag=image_tag,
            port=port,
            domain=domain,
            cpu=cpu,
            memory=memory,
            datacenter=datacenter,
            nomad_addr=nomad_addr,
        )
    else:
        print(f"Full-mode deployment for tier '{tier.value}' requires Traefik.")
        print("Use deploy_web_service template with Traefik tags directly.")
        print("Example: nomad job run web_service.nomad.hcl")
        return False
