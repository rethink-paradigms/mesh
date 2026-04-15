"""
Rich UI panels and formatted outputs for the mesh CLI.
Reusable components for beautiful terminal output.
"""

import time
from typing import List, Dict, Any
from rich.console import Console
from rich.panel import Panel
from rich.table import Table
from rich.tree import Tree
from rich.text import Text
from rich.progress import Progress, SpinnerColumn, TextColumn, BarColumn

from src.cli.ui.themes import (
    MESH_THEME, BANNER, STATUS_ICONS,
    MESH_CYAN, MESH_GREEN, MESH_PURPLE, MESH_YELLOW, MESH_RED,
    MESH_ORANGE, MESH_DIM,
)

# Global console with theme
console = Console(theme=MESH_THEME)


def show_banner():
    """Display the mesh CLI banner."""
    console.print(BANNER)


def show_error(message: str):
    """Display a styled error message."""
    console.print(f"\n  {STATUS_ICONS['error']} [error]{message}[/error]\n")


def show_success(message: str):
    """Display a styled success message."""
    console.print(f"\n  [success]✓ {message}[/success]\n")


def show_warning(message: str):
    """Display a styled warning message."""
    console.print(f"\n  [warning]⚠ {message}[/warning]\n")


def show_info(message: str):
    """Display a styled info message."""
    console.print(f"  [info]→ {message}[/info]")


def show_step(step_num: int, total: int, message: str):
    """Display a numbered step."""
    console.print(f"  [dim][{step_num}/{total}][/dim] [info]{message}[/info]")


def show_cluster_status(
    cluster_name: str,
    provider: str,
    region: str,
    nodes: List[Dict[str, Any]],
    apps: List[Dict[str, Any]],
):
    """
    Display a beautiful cluster status overview with node tree and app table.
    """
    # Header panel
    header_text = Text()
    header_text.append(f"  {STATUS_ICONS['mesh']} Cluster: ", style="bold")
    header_text.append(f"{cluster_name}", style=f"bold {MESH_CYAN}")
    header_text.append(f"  │  ", style="dim")
    header_text.append(f"Provider: ", style="dim")
    header_text.append(f"{provider}", style=f"bold {MESH_ORANGE}")
    header_text.append(f"  │  ", style="dim")
    header_text.append(f"Region: ", style="dim")
    header_text.append(f"{region}", style=f"bold {MESH_ORANGE}")

    console.print(Panel(header_text, border_style=MESH_CYAN, padding=(0, 1)))

    # Node topology tree
    tree = Tree(
        f"[bold {MESH_CYAN}]{STATUS_ICONS['mesh']} Mesh Network[/]",
        guide_style=MESH_DIM,
    )

    for node in nodes:
        role = node.get("role", "worker")
        icon = STATUS_ICONS["leader"] if role == "server" else STATUS_ICONS["worker"]
        status_icon = STATUS_ICONS.get(node.get("status", "running"), "🔵")
        ip = node.get("ip", "100.x.x.x")
        name = node.get("name", "unknown")
        mem = node.get("memory", "?")
        cpu = node.get("cpu", "?")

        node_label = (
            f"{icon} [bold {MESH_ORANGE}]{name}[/]  "
            f"{status_icon}  "
            f"[dim]{ip}[/dim]  "
            f"[dim]RAM: {mem} │ CPU: {cpu}[/dim]"
        )
        node_branch = tree.add(node_label)

        # Add apps running on this node
        node_apps = [a for a in apps if a.get("node") == name]
        for app in node_apps:
            app_status = STATUS_ICONS.get(app.get("status", "running"), "🔵")
            app_name = app.get("name", "unknown")
            app_image = app.get("image", "unknown")
            app_mem = app.get("memory", "?")
            uptime = app.get("uptime", "?")

            app_label = (
                f"{STATUS_ICONS['app']} [bold {MESH_PURPLE}]{app_name}[/]  "
                f"{app_status}  "
                f"[dim]{app_image}[/dim]  "
                f"[dim]RAM: {app_mem} │ Up: {uptime}[/dim]"
            )
            node_branch.add(app_label)

    console.print()
    console.print(tree)
    console.print()

    # App summary table
    if apps:
        table = Table(
            title=f"[bold {MESH_PURPLE}]{STATUS_ICONS['app']} Running Apps[/]",
            border_style=MESH_DIM,
            show_header=True,
            header_style=f"bold {MESH_CYAN}",
            padding=(0, 1),
        )
        table.add_column("App", style=f"bold {MESH_PURPLE}")
        table.add_column("Image", style="dim")
        table.add_column("Node", style=MESH_ORANGE)
        table.add_column("Status", justify="center")
        table.add_column("Memory", justify="right", style="dim")
        table.add_column("Uptime", justify="right", style="dim")

        for app in apps:
            status = app.get("status", "running")
            status_display = f"{STATUS_ICONS.get(status, '🔵')} {status}"

            table.add_row(
                app.get("name", "?"),
                app.get("image", "?"),
                app.get("node", "?"),
                status_display,
                app.get("memory", "?"),
                app.get("uptime", "?"),
            )

        console.print(table)
        console.print()


def show_provisioning_progress(steps: List[Dict[str, str]], live: bool = True):
    """
    Display an animated provisioning progress.
    
    Args:
        steps: List of dicts with 'name' and 'duration' (seconds) keys
        live: If True, animate in real-time
    """
    with Progress(
        SpinnerColumn(style=f"bold {MESH_CYAN}"),
        TextColumn("[progress.description]{task.description}"),
        BarColumn(bar_width=30, style=MESH_DIM, complete_style=MESH_GREEN, finished_style=MESH_GREEN),
        TextColumn("[progress.percentage]{task.percentage:>3.0f}%"),
        console=console,
    ) as progress:
        overall = progress.add_task(
            f"[bold {MESH_CYAN}]Provisioning cluster...", total=len(steps)
        )

        for step in steps:
            step_task = progress.add_task(
                f"  [dim]→[/dim] {step['name']}", total=100
            )

            if live:
                duration = float(step.get("duration", 1))
                increments = 20
                for i in range(increments):
                    time.sleep(duration / increments)
                    progress.update(step_task, advance=100 / increments)
            else:
                progress.update(step_task, completed=100)

            progress.update(overall, advance=1)

    console.print()


def show_resource_comparison():
    """Display K8s vs Mesh resource comparison table."""
    table = Table(
        title=f"[bold {MESH_CYAN}]Control Plane Memory Comparison[/]",
        border_style=MESH_DIM,
        show_header=True,
        header_style=f"bold {MESH_CYAN}",
        padding=(0, 1),
    )
    table.add_column("VM Size", style="bold")
    table.add_column("Kubernetes", justify="center", style=MESH_RED)
    table.add_column("Mesh Platform", justify="center", style=MESH_GREEN)
    table.add_column("Free for Apps", justify="center", style=f"bold {MESH_GREEN}")

    table.add_row("16 GB", "Works (1GB+ ctrl plane)", "Works (530MB ctrl plane)", "15.4 GB")
    table.add_row("8 GB", "Barely functional", "Works perfectly", "7.4 GB")
    table.add_row("4 GB", "[bold red]Unusable[/]", "Works", "3.4 GB")
    table.add_row("2 GB", "[bold red]Impossible[/]", "[bold green]Leader runs here[/]", "1.4 GB")
    table.add_row("Raspberry Pi", "[bold red]Impossible[/]", "[bold green]Worker runs here[/]", "~840 MB")

    console.print(table)
    console.print()


def show_vision_roadmap():
    """Display the roadmap of capabilities."""
    tree = Tree(
        f"[bold {MESH_CYAN}]🗺️  Capability Roadmap[/]",
        guide_style=MESH_DIM,
    )

    today = tree.add(f"[bold {MESH_GREEN}]✅ Today (Built)[/]")
    today.add("[dim]Deploy containers on 2GB+ machines[/dim]")
    today.add("[dim]Mesh networking (encrypted, zero-config)[/dim]")
    today.add("[dim]Service discovery (apps find each other)[/dim]")
    today.add("[dim]50+ cloud provider support[/dim]")
    today.add("[dim]Auto-HTTPS with Let's Encrypt[/dim]")

    sprint = tree.add(f"[bold {MESH_YELLOW}]🚧 This Sprint[/]")
    sprint.add("[dim]Interactive CLI (mesh init/deploy/status)[/dim]")
    sprint.add("[dim]Application deployment commands[/dim]")
    sprint.add("[dim]Cluster health dashboard[/dim]")

    future = tree.add(f"[bold {MESH_PURPLE}]🔮 Future[/]")
    future.add("[dim]Autoscaling (scale based on load)[/dim]")
    future.add("[dim]Multi-region federation[/dim]")
    future.add("[dim]HA control plane (multi-server)[/dim]")
    future.add("[dim]Policy engine (resource limits, placement)[/dim]")
    future.add("[dim]Observability dashboard[/dim]")

    console.print(tree)
    console.print()
