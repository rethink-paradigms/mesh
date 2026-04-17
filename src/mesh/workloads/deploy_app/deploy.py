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
) -> Optional[bool]:
    tier = (
        ClusterTier(cluster_tier)
        if cluster_tier
        else _detect_tier_from_nomad(nomad_addr)
    )
    tier_config = TierConfig.from_tier(tier)

    if tier in (ClusterTier.LITE, ClusterTier.STANDARD):
        from mesh.workloads.deploy_lite_web_service.deploy import (
            deploy_lite_web_service,
        )

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
        # INGRESS and PRODUCTION tiers require Traefik — not yet automated
        from mesh.cli.ui.panels import console
        from mesh.cli.ui.themes import MESH_CYAN, MESH_GREEN
        from rich.panel import Panel
        from rich.text import Text

        body = Text()
        body.append(
            f"\n  ⚠️  Tier '{tier.value}' deployment is not yet automated.\n\n",
            style="bold yellow",
        )
        body.append("  This tier requires Traefik ingress controller.\n\n", style="dim")
        body.append("  To deploy manually:\n", style="bold")
        body.append("    1. Deploy Traefik: ", style="dim")
        body.append("nomad job run traefik.nomad.hcl\n", style=f"bold {MESH_CYAN}")
        body.append("    2. Deploy your app: ", style="dim")
        body.append("nomad job run web_service.nomad.hcl\n", style=f"bold {MESH_CYAN}")
        body.append(f"\n  For now, use LITE or STANDARD tier ", style="dim")
        body.append(
            "(single VM or multi-VM with Caddy).\n\n", style=f"bold {MESH_GREEN}"
        )

        console.print(
            Panel(
                body,
                title="[bold]Traefik Deployment Required[/]",
                border_style="yellow",
                padding=(0, 1),
            )
        )
        return False
