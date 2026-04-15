"""
mesh ssh — Connect to cluster nodes via SSH.

Lists nodes from Nomad and resolves Tailscale IPs when available.
Falls back to node addresses from the Nomad API.
"""

import json
import subprocess
import sys

import typer
from typing import Optional

from src.cli.commands.helpers import get_nomad_addr
from src.cli.ui.themes import (
    MESH_CYAN,
    MESH_DIM,
    MESH_ORANGE,
    STATUS_ICONS,
)
from src.cli.ui.panels import console, show_error, show_info


def _check_cluster() -> bool:
    nomad_addr = get_nomad_addr()
    try:
        result = subprocess.run(
            ["nomad", "node", "status", "-address", nomad_addr],
            capture_output=True,
            text=True,
            timeout=5,
        )
        return result.returncode == 0
    except (FileNotFoundError, subprocess.TimeoutExpired):
        return False


def _get_nodes():
    nomad_addr = get_nomad_addr()
    cmd = ["nomad", "node", "status", "-address", nomad_addr, "-json"]
    try:
        result = subprocess.run(cmd, capture_output=True, text=True, timeout=10)
        if result.returncode != 0:
            return []
        nodes_raw = json.loads(result.stdout)
        nodes = []
        for n in nodes_raw:
            nodes.append(
                {
                    "id": n.get("ID", ""),
                    "name": n.get("Name", "unknown"),
                    "status": n.get("Status", "unknown"),
                    "address": n.get("Address", ""),
                    "datacenter": n.get("Datacenter", ""),
                    "role": "server" if n.get("Server", False) else "client",
                }
            )
        return nodes
    except (
        FileNotFoundError,
        subprocess.TimeoutExpired,
        json.JSONDecodeError,
        KeyError,
    ):
        return []


def _get_tailscale_ips():
    try:
        result = subprocess.run(
            ["tailscale", "status", "--json"],
            capture_output=True,
            text=True,
            timeout=5,
        )
        if result.returncode != 0:
            return {}
        data = json.loads(result.stdout)
        ips = {}
        for peer in data.get("Peer", {}).values():
            name = peer.get("HostName", "")
            addrs = peer.get("TailscaleIPs", [])
            if name and addrs:
                ips[name] = addrs[0]
        return ips
    except (
        FileNotFoundError,
        subprocess.TimeoutExpired,
        json.JSONDecodeError,
        KeyError,
    ):
        return {}


def run_ssh(
    node_name: Optional[str] = None,
    user: str = "ubuntu",
):
    if not _check_cluster():
        show_error("No cluster available. Set NOMAD_ADDR or start a local Nomad server.")
        raise typer.Exit(1)

    nodes = _get_nodes()
    if not nodes:
        show_info("No nodes found in the cluster.")
        return

    tailscale_ips = _get_tailscale_ips()

    if not node_name:
        from rich.table import Table

        table = Table(
            title=f"[bold {MESH_ORANGE}]{STATUS_ICONS['mesh']} Cluster Nodes[/]",
            border_style=MESH_DIM,
            show_header=True,
            header_style=f"bold {MESH_CYAN}",
            padding=(0, 1),
        )
        table.add_column("Node", style=f"bold {MESH_ORANGE}")
        table.add_column("Address", style="dim")
        table.add_column("Tailscale IP", style=f"bold {MESH_CYAN}")
        table.add_column("Status", justify="center")
        table.add_column("Datacenter", style="dim")

        for node in nodes:
            status = node["status"]
            status_display = f"{STATUS_ICONS.get(status, '🔵')} {status}"
            ts_ip = tailscale_ips.get(node["name"], "[dim]—[/dim]")
            table.add_row(
                node["name"],
                node["address"],
                ts_ip,
                status_display,
                node["datacenter"],
            )

        console.print(table)
        console.print()
        show_info("Usage: mesh ssh <node_name> [--user ubuntu]")
        return

    target = None
    for node in nodes:
        if node["name"] == node_name:
            target = node
            break

    if not target:
        show_error(
            f"Node '{node_name}' not found. Use 'mesh ssh' to list available nodes."
        )
        raise typer.Exit(1)

    ip = tailscale_ips.get(node_name, target["address"])

    console.print(
        f"  {STATUS_ICONS['mesh']} [bold {MESH_CYAN}]Connecting to "
        f"[bold {MESH_ORANGE}]{node_name}[/] ({ip})[/]"
    )
    console.print()

    ssh_cmd = ["ssh", "-o", "StrictHostKeyChecking=accept-new", f"{user}@{ip}"]

    process = None
    try:
        process = subprocess.Popen(ssh_cmd, stdout=sys.stdout, stderr=sys.stderr)
        process.wait()
    except KeyboardInterrupt:
        if process is not None:
            process.terminate()
            process.wait()
    except FileNotFoundError:
        show_error("ssh not found. Install an SSH client.")
        raise typer.Exit(1)
