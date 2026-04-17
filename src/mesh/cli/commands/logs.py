"""
mesh logs — Stream and view Nomad job logs.

View stdout/stderr from any job running on the mesh cluster.
Supports real-time streaming (follow), tail, and allocation targeting.
"""

import json
import subprocess
import sys

import typer
from typing import Optional

from mesh.cli.commands.helpers import get_nomad_addr
from mesh.cli.ui.themes import (
    MESH_CYAN,
    MESH_DIM,
    MESH_PURPLE,
    STATUS_ICONS,
)
from mesh.cli.ui.panels import console, show_error, show_info

# Mock data for demo mode
DEMO_JOBS = [
    {"id": "web-api", "type": "service", "status": "running", "summary": "2 allocated"},
    {
        "id": "frontend",
        "type": "service",
        "status": "running",
        "summary": "1 allocated",
    },
    {"id": "worker", "type": "batch", "status": "running", "summary": "3 allocated"},
    {"id": "redis", "type": "service", "status": "running", "summary": "1 allocated"},
]

DEMO_LOG_LINES = [
    "2026-04-17T10:23:01Z [info] Server started on :8080",
    "2026-04-17T10:23:02Z [info] Connected to redis://100.64.0.3:6379",
    "2026-04-17T10:23:03Z [info] Health check passed",
    "2026-04-17T10:23:15Z [info] GET /api/v1/status 200 12ms",
    "2026-04-17T10:23:16Z [info] GET /api/v1/apps 200 45ms",
    "2026-04-17T10:23:20Z [info] POST /api/v1/deploy 201 230ms",
    "2026-04-17T10:23:21Z [info] Deployment hello-mesh scheduled on mesh-worker-1",
    "2026-04-17T10:23:22Z [info] Container pulling nginx:latest...",
    "2026-04-17T10:23:35Z [info] Container started successfully",
    "2026-04-17T10:23:36Z [info] Health check endpoint /health returned 200",
    "2026-04-17T10:24:01Z [info] GET /api/v1/status 200 8ms",
    "2026-04-17T10:24:15Z [info] Autoscaler: 1/3 CPU, 256/512 MB RAM",
    "2026-04-17T10:24:30Z [info] TLS certificate renewed for *.mesh.local",
    "2026-04-17T10:24:45Z [info] Consul service registration updated",
    "2026-04-17T10:25:00Z [info] Nomad allocation health: healthy",
    "2026-04-17T10:25:15Z [info] GET /api/v1/metrics 200 15ms",
    "2026-04-17T10:25:30Z [info] Batch job completed: data-sync (exit 0)",
    "2026-04-17T10:25:45Z [info] Leader election: mesh-leader is current leader",
    "2026-04-17T10:26:00Z [info] Mesh network: 3 peers connected",
    "2026-04-17T10:26:15Z [info] GET /api/v1/apps 200 32ms",
]


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


def _list_running_jobs():
    nomad_addr = get_nomad_addr()
    cmd = ["nomad", "job", "status", "-address", nomad_addr, "-json"]
    try:
        result = subprocess.run(cmd, capture_output=True, text=True, timeout=10)
        if result.returncode != 0:
            return []
        jobs_raw = json.loads(result.stdout)
        jobs = []
        for j in jobs_raw:
            jobs.append(
                {
                    "id": j.get("ID", ""),
                    "type": j.get("Type", ""),
                    "status": j.get("Status", ""),
                    "summary": j.get("JobSummary", {}).get("Summary", ""),
                }
            )
        return jobs
    except (
        FileNotFoundError,
        subprocess.TimeoutExpired,
        json.JSONDecodeError,
        KeyError,
    ):
        return []


def run_logs(
    job_name: Optional[str] = None,
    follow: bool = False,
    tail: int = 20,
    alloc: Optional[str] = None,
    stderr: bool = False,
    demo: bool = False,
):
    if demo:
        _run_logs_demo(job_name=job_name, follow=follow, tail=tail, stderr=stderr)
        return

    if not _check_cluster():
        show_error(
            "No cluster available. Set NOMAD_ADDR or start a local Nomad server."
        )
        raise typer.Exit(1)

    if not job_name:
        jobs = _list_running_jobs()
        if not jobs:
            show_info("No running jobs found on the cluster.")
            return
        from rich.table import Table

        table = Table(
            title=f"[bold {MESH_PURPLE}]{STATUS_ICONS['app']} Running Jobs[/]",
            border_style=MESH_DIM,
            show_header=True,
            header_style=f"bold {MESH_CYAN}",
            padding=(0, 1),
        )
        table.add_column("Job ID", style=f"bold {MESH_PURPLE}")
        table.add_column("Type", style="dim")
        table.add_column("Status", justify="center")
        table.add_column("Summary", style="dim")

        for job in jobs:
            status = job["status"]
            status_display = f"{STATUS_ICONS.get(status, '🔵')} {status}"
            table.add_row(job["id"], job["type"], status_display, str(job["summary"]))

        console.print(table)
        console.print()
        show_info("Usage: mesh logs <job_name>")
        return

    nomad_addr = get_nomad_addr()

    cmd = ["nomad", "logs", "-address", nomad_addr]

    if follow:
        cmd.append("-f")

    cmd.extend(["-tail", str(tail)])

    if alloc:
        cmd.extend(["-alloc", alloc])

    if stderr:
        cmd.append("-stderr")

    cmd.append(job_name)

    console.print(
        f"  {STATUS_ICONS['app']} [bold {MESH_CYAN}]Logs for "
        f"[bold {MESH_PURPLE}]{job_name}[/][/]"
    )
    if follow:
        console.print(f"  [dim]Streaming (Ctrl+C to stop)...[/dim]")
    console.print()

    process = None
    try:
        process = subprocess.Popen(
            cmd,
            stdout=sys.stdout,
            stderr=sys.stderr,
        )
        process.wait()
    except KeyboardInterrupt:
        if process is not None:
            process.terminate()
            process.wait()
    except FileNotFoundError:
        show_error(
            "nomad CLI not found. Install it: https://developer.hashicorp.com/nomad/install"
        )
        raise typer.Exit(1)


def _run_logs_demo(
    job_name: Optional[str] = None,
    follow: bool = False,
    tail: int = 20,
    stderr: bool = False,
):
    if not job_name:
        from rich.table import Table

        table = Table(
            title=f"[bold {MESH_PURPLE}]{STATUS_ICONS['app']} Running Jobs (demo)[/]",
            border_style=MESH_DIM,
            show_header=True,
            header_style=f"bold {MESH_CYAN}",
            padding=(0, 1),
        )
        table.add_column("Job ID", style=f"bold {MESH_PURPLE}")
        table.add_column("Type", style="dim")
        table.add_column("Status", justify="center")
        table.add_column("Summary", style="dim")

        for job in DEMO_JOBS:
            status = job["status"]
            status_display = f"{STATUS_ICONS.get(status, '🔵')} {status}"
            table.add_row(job["id"], job["type"], status_display, job["summary"])

        console.print(table)
        console.print()
        show_info("Usage: mesh logs <job_name> --demo")
        return

    console.print(
        f"  {STATUS_ICONS['app']} [bold {MESH_CYAN}]Logs for "
        f"[bold {MESH_PURPLE}]{job_name}[/] (demo)[/]"
    )
    if stderr:
        console.print(f"  [dim]Showing stderr[/dim]")
    console.print()

    lines_to_show = DEMO_LOG_LINES[-tail:]
    for line in lines_to_show:
        console.print(f"  [dim]{line}[/dim]")

    if follow:
        console.print()
        show_info("Use --follow without --demo for real-time streaming.")

    console.print()
