"""
mesh — Infrastructure Platform CLI

Deploy containerized applications across any compute, from a local
laptop to a multi-cloud cluster. Zero SSH. Auto-HTTPS. Multi-cloud.

Usage:
    mesh init                    Initialize a new cluster
    mesh status                  View cluster health
    mesh deploy <name>           Deploy an application
    mesh logs                    View application logs
    mesh ssh                     Connect to a node
    mesh destroy                 Tear down cluster

Quick start:
    mesh init                    # Interactive wizard
    mesh deploy my-app --image nginx:latest
    mesh status
"""

import typer
from typing import Optional

from src.cli.commands.init_cmd import run_init
from src.cli.commands.status import run_status
from src.cli.commands.destroy import run_destroy
from src.cli.commands.logs import run_logs
from src.cli.commands.ssh import run_ssh
from src.cli.commands.deploy import run_deploy
from src.cli.ui.panels import (
    show_banner,
    show_resource_comparison,
    show_vision_roadmap,
)
from src.cli.plugins import discover_plugins

# Create the main app
app = typer.Typer(
    name="mesh",
    help="Infrastructure Platform — Deploy containers across any cloud. Zero SSH.",
    no_args_is_help=True,
    pretty_exceptions_enable=True,
    pretty_exceptions_show_locals=False,
    add_completion=False,
)


@app.command("init")
def init(
    demo: bool = typer.Option(
        False, "--demo", help="Run in demo mode (simulated provisioning)"
    ),
    provider: Optional[str] = typer.Option(
        None, "--provider", "-p", help="Skip provider prompt"
    ),
    workers: int = typer.Option(1, "--workers", "-w", help="Number of worker nodes"),
):
    """
    Initialize a new mesh cluster.

    Interactive wizard that guides you through provider selection,
    region, sizing, and cluster provisioning.

    Example:
        mesh init
        mesh init --demo
        mesh init --provider "Local (Multipass)" --workers 2
    """
    run_init(demo=demo, provider_name=provider, workers=workers)


@app.command("status")
def status(
    demo: bool = typer.Option(False, "--demo", help="Show demo data"),
    compare: bool = typer.Option(False, "--compare", help="Show K8s comparison"),
    roadmap: bool = typer.Option(False, "--roadmap", help="Show capability roadmap"),
):
    """
    View cluster health, node topology, and running apps.

    Shows a tree view of your mesh with nodes, apps, and their status.
    Use --compare to see how the platform compares to Kubernetes.
    Use --roadmap to see the capability timeline.

    Example:
        mesh status
        mesh status --demo --compare --roadmap
    """
    run_status(demo=demo, show_comparison=compare, show_roadmap=roadmap)


@app.command("destroy")
def destroy(
    cluster: str = typer.Option("mesh-cluster", "--cluster", "-c", help="Cluster name"),
    demo: bool = typer.Option(False, "--demo", help="Run in demo mode"),
):
    """
    Tear down a mesh cluster.

    Stops all running apps, then terminates all nodes.
    Requires confirmation.

    Example:
        mesh destroy
        mesh destroy --cluster my-cluster
    """
    run_destroy(cluster_name=cluster, demo=demo)


@app.command("logs")
def logs(
    job_name: Optional[str] = typer.Argument(None, help="Job name to fetch logs for"),
    follow: bool = typer.Option(
        False, "--follow", "-f", help="Stream logs in real-time"
    ),
    tail: int = typer.Option(20, "--tail", "-n", help="Number of lines to show"),
    alloc: Optional[str] = typer.Option(
        None, "--alloc", "-a", help="Specific allocation ID"
    ),
    stderr: bool = typer.Option(
        False, "--stderr", help="Show stderr instead of stdout"
    ),
):
    """
    View logs from jobs running on the mesh cluster.

    Without a job name, lists all running jobs.
    Use --follow to stream logs in real-time.

    Example:
        mesh logs
        mesh logs my-app
        mesh logs my-app --follow
        mesh logs my-app --tail 50 --stderr
    """
    run_logs(job_name=job_name, follow=follow, tail=tail, alloc=alloc, stderr=stderr)


@app.command("ssh")
def ssh(
    node_name: Optional[str] = typer.Argument(None, help="Node name to connect to"),
    user: str = typer.Option("ubuntu", "--user", "-u", help="SSH user"),
):
    """
    SSH into a cluster node.

    Without a node name, lists all available nodes.
    Tries Tailscale IPs when available.

    Example:
        mesh ssh
        mesh ssh mesh-leader
        mesh ssh mesh-worker-1 --user admin
    """
    run_ssh(node_name=node_name, user=user)


@app.command("deploy")
def deploy(
    name: str = typer.Argument(..., help="Application name"),
    image: str = typer.Option(..., "--image", "-i", help="Docker image"),
    image_tag: str = typer.Option("latest", "--tag", "-t", help="Image tag"),
    port: int = typer.Option(8080, "--port", "-p", help="Container port"),
    domain: Optional[str] = typer.Option(None, "--domain", "-d", help="Domain for HTTPS"),
    memory: int = typer.Option(128, "--memory", "-m", help="Memory in MB"),
    cpu: int = typer.Option(100, "--cpu", "-c", help="CPU in MHz"),
    count: int = typer.Option(1, "--count", "-n", help="Number of replicas"),
    datacenter: str = typer.Option("dc1", "--datacenter", help="Nomad datacenter"),
    cluster_tier: Optional[str] = typer.Option(None, "--tier", help="Force cluster tier"),
    demo: bool = typer.Option(False, "--demo", hidden=True),
):
    """
    Deploy a containerized application to the mesh.

    Auto-detects cluster tier and routes to the appropriate deployment path.

    Example:
        mesh deploy my-app --image nginx:latest
        mesh deploy api --image python:3.11 --port 5000 --domain api.example.com
    """
    run_deploy(
        name=name, image=image, image_tag=image_tag, port=port,
        domain=domain, cpu=cpu, memory=memory, count=count,
        datacenter=datacenter, cluster_tier=cluster_tier, demo=demo,
    )

@app.command("compare")
def compare():
    """Show resource comparison: Mesh Platform vs Kubernetes."""
    show_banner()
    show_resource_comparison()


@app.command("roadmap")
def roadmap():
    """Show the capability roadmap and future vision."""
    show_banner()
    show_vision_roadmap()


# Discover and register plugins from installed packages
# Enterprise features (GPU, monitoring, backups, etc.) register here
for _plugin_app, _plugin_name in discover_plugins():
    app.add_typer(_plugin_app, name=_plugin_name)


def main():
    """Entry point for the mesh CLI."""
    app()


if __name__ == "__main__":
    main()
