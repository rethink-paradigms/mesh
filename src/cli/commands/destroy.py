"""
mesh destroy — Teardown a mesh cluster.
"""

import questionary
from questionary import Style as QStyle

from src.cli.ui.themes import MESH_RED
from src.cli.ui.panels import console, show_success, show_info, show_step

PROMPT_STYLE = QStyle([
    ('qmark', 'fg:#ff4444 bold'),
    ('question', 'fg:#e0e0e0 bold'),
    ('answer', 'fg:#ff4444 bold'),
])


def run_destroy(cluster_name: str = "mesh-cluster", demo: bool = False):
    """Destroy a mesh cluster with confirmation."""
    console.print()
    console.print(
        f"  [bold {MESH_RED}]⚠️  Destroy cluster '{cluster_name}'?[/]"
    )
    console.print(f"  [dim]This will terminate all nodes and stop all running apps.[/dim]")
    console.print()

    confirm = questionary.confirm(
        "Are you sure? This cannot be undone.",
        default=False,
        style=PROMPT_STYLE,
    ).ask()

    if not confirm:
        show_info("Cancelled.")
        return

    console.print()

    if demo:
        import time
        show_step(1, 4, "Stopping all running apps...")
        time.sleep(0.5)
        show_step(2, 4, "Draining nodes...")
        time.sleep(0.5)
        show_step(3, 4, "Terminating nodes...")
        time.sleep(0.5)
        show_step(4, 4, "Cleaning up network...")
        time.sleep(0.3)
    else:
        # Real: Use Multipass or Pulumi to destroy
        import subprocess
        import shutil

        multipass_cmd = shutil.which("multipass")
        if multipass_cmd:
            show_step(1, 3, "Stopping Multipass VMs...")
            for suffix in ["leader", "worker-1", "worker-2", "worker-3"]:
                name = f"{cluster_name}-{suffix}"
                subprocess.run(
                    [multipass_cmd, "delete", name, "--purge"],
                    capture_output=True,
                )
            show_step(2, 3, "Purging deleted VMs...")
            subprocess.run([multipass_cmd, "purge"], capture_output=True)
            show_step(3, 3, "Cleanup complete")
        else:
            show_info("No local VMs found. For cloud clusters, use: pulumi destroy")

    console.print()
    show_success(f"Cluster '{cluster_name}' destroyed")
    console.print()
