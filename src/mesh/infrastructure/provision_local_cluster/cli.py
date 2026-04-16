#!/usr/bin/env python3

import os
import sys
import subprocess
import json
import click
import shutil
from dotenv import load_dotenv

# --- PATH SETUP ---
# Add project root to sys.path to import from 'platform'
# Current: ops-platform/local/cli.py
# Root:    ../..
sys.path.insert(
    0, os.path.abspath(os.path.join(os.path.dirname(__file__), "..", "..", ".."))
)

# --- NEW IMPORTS ---
from mesh.infrastructure.boot_consul_nomad.generate_boot_scripts import (
    generate_shell_script,
)
from mesh.infrastructure.provision_node.multipass import provision_multipass_node
from mesh.workloads.manage_secrets.manage import SecretsManager

# Paths
BASE_DIR = os.path.dirname(os.path.abspath(__file__))
ENV_FILE = os.path.join(os.path.dirname(__file__), "..", "..", "..", ".env")

# Configuration
NODES = {
    "local-leader": {"cpus": "2", "mem": "1G"},
    "local-worker": {"cpus": "1", "mem": "512M"},
}

MULTIPASS_CMD = "multipass"


def check_multipass():
    """Ensure multipass is installed and return the command path."""
    cmd = shutil.which("multipass")
    if cmd:
        return cmd

    common_paths = ["/usr/local/bin/multipass", "/opt/homebrew/bin/multipass"]
    for path in common_paths:
        if os.path.exists(path) and os.access(path, os.X_OK):
            return path

    click.echo(
        click.style("❌ 'multipass' not found in PATH or standard locations.", fg="red")
    )
    click.echo("Please run: brew install --cask multipass")
    sys.exit(1)


def load_secrets():
    """Load and return secrets from .env."""
    if not os.path.exists(ENV_FILE):
        click.echo(click.style(f"❌ .env file not found at {ENV_FILE}", fg="red"))
        sys.exit(1)

    load_dotenv(ENV_FILE)
    secrets = {
        "TAILSCALE_AUTH_KEY": os.getenv("TAILSCALE_KEY"),
        # Load other secrets as needed for syncing
        "DATABASE_URL": os.getenv("DATABASE_URL", ""),
    }

    if not secrets["TAILSCALE_AUTH_KEY"]:
        click.echo(click.style("❌ TAILSCALE_KEY not found in .env", fg="red"))
        sys.exit(1)

    return secrets


@click.group()
def cli():
    """A CLI for managing the local development environment (Scavenger Mesh)."""
    pass


@cli.command()
@click.pass_context
def up(ctx):
    """Brings up the local development environment."""
    check_multipass()
    secrets = load_secrets()

    click.echo("🚀 Launching Cloud Cluster (Multipass)...")

    leader_ip = "127.0.0.1"  # Initial bootstrap assumption for leader

    # 1. Provision Leader
    leader_name = "local-leader"
    leader_specs = NODES[leader_name]
    size_str = f"{leader_specs['cpus']}CPU,{leader_specs['mem']}"

    # Generate boot script for leader
    leader_script = generate_shell_script(
        tailscale_key=secrets["TAILSCALE_AUTH_KEY"], leader_ip=leader_ip, role="server"
    )

    click.echo(f"   - Provisioning {leader_name}...")
    leader_info = provision_multipass_node(
        name=leader_name,
        instance_size=size_str,
        role="server",
        boot_script_content=leader_script,
    )

    # Update leader_ip with actual IP
    leader_ip = leader_info["private_ip"]  # In multipass, private == public usually
    click.echo(f"     ✅ Leader is at {leader_ip}")

    # 2. Provision Workers
    for name, specs in NODES.items():
        if name == "local-leader":
            continue

        size_str = f"{specs['cpus']}CPU,{specs['mem']}"
        worker_script = generate_shell_script(
            tailscale_key=secrets["TAILSCALE_AUTH_KEY"],
            leader_ip=leader_ip,  # Point to actual leader IP
            role="client",
        )

        click.echo(f"   - Provisioning {name}...")
        provision_multipass_node(
            name=name,
            instance_size=size_str,
            role="client",
            boot_script_content=worker_script,
        )

    click.echo("\n✅ Cluster Up!")
    ctx.invoke(status)

    # 3. Sync Secrets
    click.echo("\n🔄 Syncing secrets to Nomad...")
    nomad_addr = f"http://{leader_ip}:4646"
    nomad_token = (
        "dev-token"  # Hardcoded in boot.sh for dev envs? Or we need to fetch it?
    )
    # In boot.sh logic (not visible here), ideally we set a dev token or ACL is disabled.
    # For now, assuming ACL might be enabled but we use anonymous or a known token if set in boot.sh

    # Filter secrets to only those needed for apps (exclude infra keys if desired)
    app_secrets = {k: v for k, v in secrets.items() if k not in ["TAILSCALE_AUTH_KEY"]}

    manager = SecretsManager(nomad_addr=nomad_addr, nomad_token=nomad_token)
    # Sync for a default job, e.g., 'marketing-site' or 'example-app'
    manager.sync_secrets("marketing-site", app_secrets)


@cli.command()
def down():
    """Tears down the local development environment."""
    check_multipass()
    click.echo("🗑️  Tearing down Cloud Cluster...")
    for name in NODES.keys():
        click.echo(f"   - Deleting {name}...")
        subprocess.run([MULTIPASS_CMD, "delete", name, "--purge"], capture_output=True)
    click.echo("✅ Cluster Destroyed.")


@cli.command()
def status():
    """Shows the status of the local development environment."""
    check_multipass()
    subprocess.run([MULTIPASS_CMD, "list"])


@cli.command()
@click.argument("name", required=False)
def provision(name):
    """Refreshes the configuration (re-runs boot script) on a running node."""
    check_multipass()
    secrets = load_secrets()

    nodes_to_provision = [name] if name else NODES.keys()

    # Get leader IP first if we are provisioning workers
    leader_ip = ""
    res = subprocess.run(
        [MULTIPASS_CMD, "info", "local-leader", "--format", "json"],
        capture_output=True,
        text=True,
    )
    if res.returncode == 0:
        info = json.loads(res.stdout)
        leader_ip = info.get("info", {}).get("local-leader", {}).get("ipv4", [None])[0]

    if not leader_ip:
        click.echo(
            click.style(
                "⚠️  Could not find Leader IP. Assuming 127.0.0.1 (might fail for workers).",
                fg="yellow",
            )
        )
        leader_ip = "127.0.0.1"

    for node_name in nodes_to_provision:
        if node_name not in NODES:
            click.echo(
                click.style(
                    f"❌ Node '{node_name}' not found in configuration.", fg="red"
                )
            )
            continue

        click.echo(f"🔄 Re-Provisioning {node_name}...")

        role = "server" if node_name == "local-leader" else "client"

        # Generate the script using the Feature module
        script_content = generate_shell_script(
            tailscale_key=secrets["TAILSCALE_AUTH_KEY"], leader_ip=leader_ip, role=role
        )

        temp_script_path = os.path.join(BASE_DIR, f"temp_startup_{node_name}.sh")
        with open(temp_script_path, "w") as f:
            f.write(script_content)

        # Transfer the script
        click.echo(f"   - Transferring startup script to {node_name}...")
        dest_path = f"{node_name}:/tmp/startup.sh"
        subprocess.run(
            [MULTIPASS_CMD, "transfer", temp_script_path, dest_path],
            check=True,
            capture_output=True,
        )

        # Execute the script
        click.echo(f"   - Executing startup script on {node_name}...")
        subprocess.run(
            [MULTIPASS_CMD, "exec", node_name, "--", "sudo", "bash", "/tmp/startup.sh"],
            check=True,
            capture_output=True,
        )

        # Clean up
        os.remove(temp_script_path)

        click.echo(f"✅ {node_name} re-provisioned successfully.")


if __name__ == "__main__":
    cli()
