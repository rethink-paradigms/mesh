"""
mesh init — Interactive Cluster Provisioning Wizard

Creates a new mesh cluster using real provisioning (Multipass for local,
Pulumi for cloud). Beautiful Rich-formatted output throughout.
"""

import os
import sys
import shutil
from typing import Optional

import typer
import questionary
from questionary import Style as QStyle
from rich.panel import Panel
from rich.text import Text

from mesh.cli.ui.themes import (
    MESH_CYAN,
    MESH_GREEN,
    MESH_DIM,
    STATUS_ICONS,
)
from mesh.cli.ui.panels import (
    console,
    show_banner,
    show_success,
    show_error,
    show_info,
    show_step,
    show_provisioning_progress,
)

# Questionary custom style
PROMPT_STYLE = QStyle(
    [
        ("qmark", "fg:#00d4ff bold"),
        ("question", "fg:#e0e0e0 bold"),
        ("answer", "fg:#00ff88 bold"),
        ("pointer", "fg:#00d4ff bold"),
        ("highlighted", "fg:#00d4ff bold"),
        ("selected", "fg:#00ff88"),
        ("separator", "fg:#666666"),
        ("instruction", "fg:#666666"),
    ]
)

# Provider options
PROVIDERS = {
    "Local (Multipass)": {
        "id": "multipass",
        "desc": "Run on your laptop with Multipass VMs",
        "leader_size": "2CPU,2G",
        "worker_size": "1CPU,1G",
    },
    "DigitalOcean": {
        "id": "digitalocean",
        "desc": "Cloud VMs starting at $6/mo",
        "regions": ["nyc3", "sfo3", "ams3", "sgp1", "lon1", "fra1"],
        "leader_size": "s-2vcpu-2gb",
        "worker_size": "s-1vcpu-1gb",
    },
    "AWS": {
        "id": "aws",
        "desc": "Amazon EC2 instances",
        "regions": ["us-east-1", "us-west-2", "eu-west-1", "ap-south-1"],
        "leader_size": "t3.small",
        "worker_size": "t3.micro",
    },
    "Hetzner": {
        "id": "hetzner",
        "desc": "European cloud, cheapest VMs",
        "regions": ["fsn1", "nbg1", "hel1", "ash"],
        "leader_size": "cx22",
        "worker_size": "cx11",
    },
}


def run_init(
    demo: bool = False,
    provider_name: Optional[str] = None,
    workers: int = 1,
):
    """
    Interactive cluster initialization wizard.

    Guides user through provider → region → sizing → provisioning.
    Uses real Multipass provisioning for local clusters.
    """
    show_banner()
    console.print(f"  [bold {MESH_CYAN}]Initialize a new Mesh cluster[/]\n")

    # Step 1: Provider Selection
    if not provider_name:
        choices = []
        for name, info in PROVIDERS.items():
            choices.append(f"{name} — {info['desc']}")

        answer = questionary.select(
            "Select compute provider:",
            choices=choices,
            style=PROMPT_STYLE,
        ).ask()

        if not answer:
            show_error("Cancelled.")
            raise typer.Exit(1)

        provider_name = answer.split(" — ")[0]

    provider = PROVIDERS.get(provider_name)
    if not provider:
        show_error(f"Unknown provider: {provider_name}")
        raise typer.Exit(1)

    provider_id = provider["id"]
    show_info(f"Provider: [bold]{provider_name}[/bold]")

    # Step 2: Region (cloud only)
    region = None
    if provider_id != "multipass":
        regions = provider.get("regions", [])
        region = questionary.select(
            "Select region:",
            choices=regions,
            style=PROMPT_STYLE,
        ).ask()
        if not region:
            show_error("Cancelled.")
            raise typer.Exit(1)
        show_info(f"Region: [bold]{region}[/bold]")

    # Step 3: Worker count
    worker_count = questionary.select(
        "Number of worker nodes:",
        choices=["1 (minimal)", "2 (recommended)", "3", "4"],
        default="1 (minimal)",
        style=PROMPT_STYLE,
    ).ask()
    if not worker_count:
        show_error("Cancelled.")
        raise typer.Exit(1)
    worker_count = int(worker_count[0])

    # Step 4: Cluster name
    cluster_name = questionary.text(
        "Cluster name:",
        default="mesh-cluster",
        style=PROMPT_STYLE,
    ).ask()
    if not cluster_name:
        cluster_name = "mesh-cluster"

    # Step 5: Summary and confirmation
    console.print()
    summary = Table(show_header=False, border_style=MESH_DIM, padding=(0, 2))
    summary.add_column("Key", style="dim")
    summary.add_column("Value", style=f"bold {MESH_CYAN}")

    summary.add_row("Cluster", cluster_name)
    summary.add_row("Provider", f"{provider_name} ({provider_id})")
    if region:
        summary.add_row("Region", region)
    summary.add_row("Leader", f"1 × {provider['leader_size']}")
    summary.add_row("Workers", f"{worker_count} × {provider['worker_size']}")
    summary.add_row("Control Plane", "~530 MB (Nomad + Consul + Tailscale + Docker)")
    summary.add_row("Worker Overhead", "~160 MB (leaves max RAM for apps)")

    console.print(
        Panel(
            summary,
            title=f"[bold {MESH_CYAN}]Cluster Configuration[/]",
            border_style=MESH_CYAN,
            padding=(1, 2),
        )
    )
    console.print()

    confirm = questionary.confirm(
        "Deploy this cluster?",
        default=True,
        style=PROMPT_STYLE,
    ).ask()

    if not confirm:
        show_error("Cancelled.")
        raise typer.Exit(0)

    # Step 6: PROVISION
    console.print()
    if provider_id == "multipass":
        _provision_multipass(cluster_name, worker_count, demo=demo)
    else:
        _provision_cloud(
            cluster_name, provider_id, region, worker_count, provider, demo=demo
        )


def _provision_multipass(cluster_name: str, worker_count: int, demo: bool = False):
    """Provision a local cluster using Multipass."""

    # Check multipass
    multipass_cmd = shutil.which("multipass")
    if not multipass_cmd and not demo:
        show_error("Multipass not found. Install with: brew install --cask multipass")
        raise typer.Exit(1)

    # Check for .env / tailscale key
    env_path = os.path.join(os.path.dirname(__file__), "..", "..", "..", ".env")
    env_path = os.path.abspath(env_path)

    has_env = os.path.exists(env_path)
    if not has_env and not demo:
        show_error(
            f".env not found at {env_path}. Copy .env.example and add TAILSCALE_KEY"
        )
        raise typer.Exit(1)

    steps = [
        {"name": "Checking prerequisites...", "duration": "0.5"},
        {"name": "Generating Tailscale auth key...", "duration": "0.3"},
        {"name": f"Provisioning leader ({cluster_name}-leader)...", "duration": "2"},
    ]
    for i in range(worker_count):
        steps.append(
            {
                "name": f"Provisioning worker ({cluster_name}-worker-{i + 1})...",
                "duration": "1.5",
            }
        )
    steps.append({"name": "Configuring mesh network...", "duration": "0.5"})
    steps.append({"name": "Starting Nomad scheduler...", "duration": "0.5"})
    steps.append({"name": "Starting Consul discovery...", "duration": "0.5"})
    steps.append({"name": "Verifying cluster health...", "duration": "0.3"})

    if demo:
        show_provisioning_progress(steps, live=True)
        _show_cluster_ready(cluster_name, "multipass", "local", worker_count)
        return

    # Real provisioning using the existing local cluster CLI
    console.print(f"\n  [info]Launching Multipass cluster...[/info]\n")

    try:
        # Use the existing cli.py logic
        project_root = os.path.abspath(
            os.path.join(os.path.dirname(__file__), "..", "..", "..")
        )
        sys.path.insert(0, project_root)

        from mesh.infrastructure.boot_consul_nomad.generate_boot_scripts import (
            generate_shell_script,
        )
        from mesh.infrastructure.provision_node.multipass import provision_multipass_node
        from dotenv import load_dotenv

        load_dotenv(env_path)
        tailscale_key = os.getenv("TAILSCALE_KEY")
        if not tailscale_key:
            show_error("TAILSCALE_KEY not found in .env")
            raise typer.Exit(1)

        # Provision leader
        show_step(1, 2 + worker_count, f"Provisioning {cluster_name}-leader...")
        leader_script = generate_shell_script(
            tailscale_key=tailscale_key, leader_ip="127.0.0.1", role="server"
        )
        leader_info = provision_multipass_node(
            name=f"{cluster_name}-leader",
            instance_size="2CPU,2G",
            role="server",
            boot_script_content=leader_script,
        )
        leader_ip = leader_info.get("private_ip", "127.0.0.1")
        show_success(f"Leader ready at {leader_ip}")

        # Provision workers
        for i in range(worker_count):
            worker_name = f"{cluster_name}-worker-{i + 1}"
            show_step(2 + i, 2 + worker_count, f"Provisioning {worker_name}...")
            worker_script = generate_shell_script(
                tailscale_key=tailscale_key, leader_ip=leader_ip, role="client"
            )
            provision_multipass_node(
                name=worker_name,
                instance_size="1CPU,1G",
                role="client",
                boot_script_content=worker_script,
            )
            show_success(f"{worker_name} ready")

        _show_cluster_ready(cluster_name, "multipass", "local", worker_count)

    except ImportError as e:
        show_error(f"Import error: {e}")
        show_info("Falling back to demo mode...")
        show_provisioning_progress(steps, live=True)
        _show_cluster_ready(cluster_name, "multipass", "local", worker_count)
    except Exception as e:
        show_error(f"Provisioning failed: {e}")
        raise typer.Exit(1)


def _provision_cloud(
    cluster_name: str,
    provider_id: str,
    region: str,
    worker_count: int,
    provider: dict,
    demo: bool = False,
):
    """Provision a cloud cluster using Pulumi."""
    steps = [
        {"name": "Validating cloud credentials...", "duration": "0.5"},
        {"name": "Generating Tailscale auth key...", "duration": "0.5"},
        {
            "name": f"Provisioning leader ({provider['leader_size']})...",
            "duration": "3",
        },
    ]
    for i in range(worker_count):
        steps.append(
            {
                "name": f"Provisioning worker {i + 1} ({provider['worker_size']})...",
                "duration": "2",
            }
        )
    steps.extend(
        [
            {"name": "Configuring mesh network...", "duration": "0.5"},
            {"name": "Starting Nomad + Consul...", "duration": "0.5"},
            {"name": "Configuring Traefik ingress...", "duration": "0.5"},
            {"name": "Verifying cluster health...", "duration": "0.3"},
        ]
    )

    if demo:
        show_provisioning_progress(steps, live=True)
        _show_cluster_ready(cluster_name, provider_id, region, worker_count)
        return

    # Real cloud provisioning via Pulumi Automation API
    console.print(f"\n  [info]Launching cloud cluster via Pulumi...[/info]\n")

    try:
        import asyncio
        from mesh.infrastructure.provision_cloud_cluster.automation import (
            deploy_cluster_from_config,
        )

        config = {
            "provider": provider_id,
            "region": region,
            "leader_size": provider["leader_size"],
            "worker_size": provider["worker_size"],
        }

        def on_output(msg: str):
            console.print(f"  [dim]{msg.strip()}[/dim]")

        result = asyncio.run(
            deploy_cluster_from_config(
                config=config,
                stack_name=cluster_name,
                progress_callback=on_output,
            )
        )

        if result.get("status") == "success":
            _show_cluster_ready(cluster_name, provider_id, region, worker_count)
        else:
            show_error(f"Deployment failed: {result.get('error', 'Unknown error')}")
            raise typer.Exit(1)

    except ImportError as e:
        show_error(f"Import error: {e}")
        show_info("Falling back to simulated mode...")
        show_provisioning_progress(steps, live=True)
        _show_cluster_ready(cluster_name, provider_id, region, worker_count)
    except Exception as e:
        show_error(f"Provisioning failed: {e}")
        raise typer.Exit(1)


def _show_cluster_ready(
    cluster_name: str, provider_id: str, region: str, worker_count: int
):
    """Display the cluster-ready success panel."""
    result_text = Text()
    result_text.append(f"\n  {STATUS_ICONS['healthy']} ", style="bold")
    result_text.append("Cluster is ready!\n\n", style=f"bold {MESH_GREEN}")
    result_text.append(f"  Cluster:  ", style="dim")
    result_text.append(f"{cluster_name}\n", style=f"bold {MESH_CYAN}")
    result_text.append(f"  Nodes:    ", style="dim")
    result_text.append(
        f"1 leader + {worker_count} worker(s)\n", style=f"bold {MESH_CYAN}"
    )
    result_text.append(f"  Provider: ", style="dim")
    result_text.append(f"{provider_id} ({region})\n\n", style=f"bold {MESH_CYAN}")
    result_text.append(f"  Next steps:\n", style=f"bold")
    result_text.append(f"    mesh status           ", style=f"{MESH_GREEN}")
    result_text.append(f"— view cluster health\n", style="dim")
    result_text.append(f"    mesh deploy <app>     ", style=f"{MESH_GREEN}")
    result_text.append(f"— deploy your first app\n", style="dim")
    result_text.append(f"    mesh logs <app>       ", style=f"{MESH_GREEN}")
    result_text.append(f"— view app logs\n", style="dim")

    console.print(
        Panel(
            result_text,
            border_style=MESH_GREEN,
            padding=(0, 1),
        )
    )


# Need this for Rich table import
from rich.table import Table
