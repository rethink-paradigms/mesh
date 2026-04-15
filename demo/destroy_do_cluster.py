#!/usr/bin/env python3
"""
destroy_do_cluster.py — Destroy the mesh cluster on DigitalOcean.

Reads .cluster-state.json and destroys all droplets listed there.

Usage:
    python3 destroy_do_cluster.py
"""

import os
import sys
import json
from pathlib import Path

# Load .env
env_path = Path(__file__).parent / ".env"
if env_path.exists():
    for line in env_path.read_text().splitlines():
        line = line.strip()
        if line and not line.startswith("#") and "=" in line:
            k, v = line.split("=", 1)
            os.environ.setdefault(k.strip(), v.strip())

from src.infrastructure.providers import get_credentials, get_driver

STATE_FILE = Path(__file__).parent / ".cluster-state.json"


def main():
    if not STATE_FILE.exists():
        print("No cluster state found (.cluster-state.json missing).")
        sys.exit(0)

    state = json.loads(STATE_FILE.read_text())
    nodes = state.get("nodes", [])

    if not nodes:
        print("No nodes in cluster state.")
        sys.exit(0)

    print("=" * 60)
    print("  DESTROY MESH CLUSTER")
    print("=" * 60)
    print()
    print(f"  Cluster: {state.get('cluster_name', '?')}")
    print(f"  Region:  {state.get('region', '?')}")
    print(f"  Nodes:   {len(nodes)}")
    print()
    for n in nodes:
        print(f"  {'👑' if n['role'] == 'server' else '⚙️ '} {n['name']:25s}  {n.get('public_ip', '?')}")
    print()

    resp = input("Destroy all nodes? This CANNOT be undone. [y/N]: ")
    if resp.lower() != "y":
        print("Aborted.")
        return

    # Connect
    creds = get_credentials("digitalocean")
    driver = get_driver("digitalocean", credentials=creds)

    # Get all current droplets
    all_droplets = driver.list_nodes()
    droplet_map = {n.id: n for n in all_droplets}

    destroyed = 0
    for node_state in nodes:
        droplet_id = node_state.get("droplet_id")
        name = node_state.get("name", "?")
        print(f"  Destroying {name} (ID: {droplet_id})...", end=" ")

        if droplet_id in droplet_map:
            try:
                driver.destroy_node(droplet_map[droplet_id])
                print("✓ Destroyed")
                destroyed += 1
            except Exception as e:
                print(f"✗ Error: {e}")
        else:
            print("⚠ Not found (already destroyed?)")
            destroyed += 1

    # Clean up state file
    STATE_FILE.unlink(missing_ok=True)
    print()
    print(f"  {destroyed}/{len(nodes)} nodes destroyed.")
    print("  State file removed.")
    print()


if __name__ == "__main__":
    main()
