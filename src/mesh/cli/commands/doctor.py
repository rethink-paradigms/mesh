"""
mesh doctor — Environment diagnostics and health checks.

Verifies that all prerequisites are installed and configured
for running the mesh platform.
"""

import os
import sys
import shutil
import subprocess
from typing import List, Tuple

from rich.table import Table
from rich.panel import Panel
from rich.text import Text

from mesh.cli.ui.themes import MESH_CYAN, MESH_GREEN, MESH_RED, MESH_YELLOW, MESH_DIM
from mesh.cli.ui.panels import console, show_banner

MIN_PYTHON = (3, 11)

DEMO_CHECKS = [
    ("Python version", True, "3.12.0"),
    ("Docker installed", True, "v24.0.7"),
    ("Docker daemon running", True, "OK"),
    ("Pulumi installed", True, "v3.100.0"),
    ("Tailscale installed", True, "v1.56.0"),
    ("Environment configured", True, ".env found, TAILSCALE_KEY set"),
    ("Network connectivity", True, "OK"),
]


def _check_python_version() -> Tuple[bool, str]:
    version = (
        f"{sys.version_info.major}.{sys.version_info.minor}.{sys.version_info.micro}"
    )
    if sys.version_info >= MIN_PYTHON:
        return True, version
    return False, f"{version} (need >={MIN_PYTHON[0]}.{MIN_PYTHON[1]})"


def _check_docker_installed() -> Tuple[bool, str]:
    if not shutil.which("docker"):
        return False, "not found"
    try:
        result = subprocess.run(
            ["docker", "--version"],
            capture_output=True,
            text=True,
            timeout=10,
        )
        if result.returncode == 0:
            return True, result.stdout.strip()
        return False, "error running docker"
    except Exception as e:
        return False, str(e)


def _check_docker_running() -> Tuple[bool, str]:
    try:
        result = subprocess.run(
            ["docker", "info"],
            capture_output=True,
            text=True,
            timeout=10,
        )
        if result.returncode == 0:
            return True, "OK"
        return False, "daemon not running"
    except FileNotFoundError:
        return False, "docker not found"
    except Exception:
        return False, "cannot connect to daemon"


def _check_pulumi() -> Tuple[bool, str]:
    if not shutil.which("pulumi"):
        return False, "not found"
    try:
        result = subprocess.run(
            ["pulumi", "version"],
            capture_output=True,
            text=True,
            timeout=10,
        )
        if result.returncode == 0:
            return True, result.stdout.strip()
        return False, "error running pulumi"
    except Exception as e:
        return False, str(e)


def _check_tailscale() -> Tuple[bool, str]:
    if not shutil.which("tailscale"):
        return False, "not found"
    try:
        result = subprocess.run(
            ["tailscale", "version"],
            capture_output=True,
            text=True,
            timeout=10,
        )
        if result.returncode == 0:
            version_line = result.stdout.strip().split("\n")[0]
            return True, version_line
        return False, "error running tailscale"
    except Exception as e:
        return False, str(e)


def _check_env() -> Tuple[bool, str]:
    from mesh.infrastructure.config.env import EnvVars, get_env

    project_root = os.path.abspath(
        os.path.join(os.path.dirname(__file__), "..", "..", "..")
    )
    env_path = os.path.join(project_root, ".env")
    has_env = os.path.exists(env_path)
    has_key = bool(get_env(EnvVars.TAILSCALE_KEY))

    if has_env and has_key:
        return True, ".env found, TAILSCALE_KEY set"
    if has_env:
        return False, ".env found, but TAILSCALE_KEY not set"
    if has_key:
        return True, "TAILSCALE_KEY set via environment"
    return False, ".env not found, TAILSCALE_KEY not set"


def _check_network() -> Tuple[bool, str]:
    import socket

    try:
        socket.create_connection(("1.1.1.1", 53), timeout=5)
        return True, "OK"
    except OSError:
        return False, "no connectivity"


def _run_checks() -> List[Tuple[str, bool, str]]:
    return [
        ("Python version", *_check_python_version()),
        ("Docker installed", *_check_docker_installed()),
        ("Docker daemon running", *_check_docker_running()),
        ("Pulumi installed", *_check_pulumi()),
        ("Tailscale installed", *_check_tailscale()),
        ("Environment configured", *_check_env()),
        ("Network connectivity", *_check_network()),
    ]


def _render_results(checks: List[Tuple[str, bool, str]]):
    table = Table(
        show_header=True,
        border_style=MESH_DIM,
        header_style=f"bold {MESH_CYAN}",
        padding=(0, 1),
    )
    table.add_column("Check", style="bold")
    table.add_column("Status", justify="center")
    table.add_column("Detail", style="dim")

    for name, passed, detail in checks:
        icon = (
            f"[{MESH_GREEN}]✓[/{MESH_GREEN}]"
            if passed
            else f"[{MESH_RED}]✗[/{MESH_RED}]"
        )
        table.add_row(name, icon, detail)

    console.print()
    console.print(table)
    console.print()

    passed_count = sum(1 for _, ok, _ in checks if ok)
    total = len(checks)

    color = (
        MESH_GREEN
        if passed_count == total
        else MESH_YELLOW
        if passed_count > 0
        else MESH_RED
    )
    summary = Text()
    summary.append(f"  {passed_count}/{total} checks passed", style=f"bold {color}")

    console.print(Panel(summary, border_style=color, padding=(0, 1)))
    console.print()


def run_doctor(demo: bool = False):
    """
    Run environment diagnostics and display results.
    """
    show_banner()
    console.print(f"  [bold {MESH_CYAN}]Environment Health Check[/]")
    console.print()

    if demo:
        checks = DEMO_CHECKS
    else:
        checks = _run_checks()

    _render_results(checks)
