"""
mesh deploy — Deploy a containerized application to the mesh cluster.

Tier-aware deployment: auto-detects cluster tier (lite/standard/ingress/production)
and routes to the appropriate deployment path (Caddy or Traefik).
"""

import typer
from typing import Optional

from mesh.cli.ui.panels import console, show_success, show_error, show_info, show_step
from mesh.cli.ui.themes import MESH_CYAN

from rich.panel import Panel
from rich.text import Text


def run_deploy(
    name: str,
    image: str,
    image_tag: str = "latest",
    port: int = 8080,
    domain: Optional[str] = None,
    cpu: int = 100,
    memory: int = 128,
    count: int = 1,
    datacenter: str = "dc1",
    cluster_tier: Optional[str] = None,
    demo: bool = False,
):
    """Deploy a containerized application to the mesh cluster."""
    console.print()
    console.print(f"  🚀 [bold {MESH_CYAN}]Deploying: {name}[/]")
    console.print()

    # Show deployment config
    config_text = Text()
    config_text.append(f"  App:       ", style="dim")
    config_text.append(f"{name}\n", style=f"bold {MESH_CYAN}")
    config_text.append(f"  Image:     ", style="dim")
    config_text.append(f"{image}:{image_tag}\n", style=f"{MESH_CYAN}")
    config_text.append(f"  Port:      ", style="dim")
    config_text.append(f"{port}\n", style=f"{MESH_CYAN}")
    config_text.append(f"  Memory:    ", style="dim")
    config_text.append(f"{memory} MB\n", style=f"{MESH_CYAN}")
    config_text.append(f"  CPU:       ", style="dim")
    config_text.append(f"{cpu} MHz\n", style=f"{MESH_CYAN}")
    if domain:
        config_text.append(f"  Domain:    ", style="dim")
        config_text.append(f"{domain}\n", style=f"{MESH_CYAN}")

    console.print(Panel(config_text, border_style=MESH_CYAN, padding=(0, 1)))
    console.print()

    if demo:
        import time
        show_step(1, 4, "Detecting cluster tier...")
        time.sleep(0.3)
        show_step(2, 4, "Generating Nomad job spec...")
        time.sleep(0.3)
        show_step(3, 4, "Submitting to scheduler...")
        time.sleep(0.5)
        show_step(4, 4, "Waiting for allocation...")
        time.sleep(0.3)
    else:
        show_step(1, 3, "Detecting cluster tier...")
        try:
            from mesh.workloads.deploy_app.deploy import deploy_app

            show_step(2, 3, "Deploying application...")
            result = deploy_app(
                app_name=name,
                image=image,
                image_tag=image_tag,
                port=port,
                domain=domain,
                cpu=cpu,
                memory=memory,
                datacenter=datacenter,
                cluster_tier=cluster_tier,
            )
            if result:
                show_step(3, 3, "Application scheduled successfully")
            else:
                show_info("Application submitted — check mesh status for progress")
        except ImportError as e:
            show_error(f"Import error: {e}")
            show_info("Ensure the mesh platform is properly installed")
            return
        except Exception as e:
            show_error(f"Deployment failed: {e}")
            return

    console.print()
    show_success(f"Application '{name}' deployed to mesh")
    console.print(f"  [dim]View status: [bold]mesh status[/bold][/dim]")
    console.print(f"  [dim]View logs:   [bold]mesh logs {name}[/bold][/dim]")
    console.print()
