#!/usr/bin/env python3
"""
provision_do_cluster.py — Spin up a real mesh cluster on DigitalOcean.

Uses Libcloud directly (no Pulumi) to create droplets with the existing
boot script that installs Nomad, Consul, Tailscale, and Docker.

Usage:
    python3 provision_do_cluster.py
"""

import os
import sys
import time
import json
from pathlib import Path
from datetime import datetime

# Load .env
env_path = Path(__file__).parent / ".env"
if env_path.exists():
    for line in env_path.read_text().splitlines():
        line = line.strip()
        if line and not line.startswith("#") and "=" in line:
            k, v = line.split("=", 1)
            os.environ.setdefault(k.strip(), v.strip())

# Now import project modules
from src.infrastructure.providers import get_credentials, get_driver
from src.infrastructure.boot_consul_nomad.generate_boot_scripts import generate_cloud_init_yaml


# ── Configuration ──────────────────────────────────────────────────────────
CLUSTER_NAME = "mesh"
REGION = "blr1"  # Bangalore

NODES = [
    {"name": f"{CLUSTER_NAME}-leader",   "role": "server", "size_id": "s-2vcpu-4gb"},
    {"name": f"{CLUSTER_NAME}-worker-1", "role": "client", "size_id": "s-1vcpu-2gb"},
    {"name": f"{CLUSTER_NAME}-worker-2", "role": "client", "size_id": "s-1vcpu-2gb"},
]

# State file to track created nodes
STATE_FILE = Path(__file__).parent / ".cluster-state.json"


def load_state():
    if STATE_FILE.exists():
        return json.loads(STATE_FILE.read_text())
    return {"nodes": [], "created_at": None}


def save_state(state):
    STATE_FILE.write_text(json.dumps(state, indent=2))


def find_ubuntu_image(driver):
    """Find exact Ubuntu 22.04 x86_64 image on DigitalOcean."""
    print("  Fetching Ubuntu 22.04 base image...")
    img = driver.get_image("ubuntu-22-04-x64")
    if not img:
        print("  ERROR: Could not find base image ubuntu-22-04-x64!")
        sys.exit(1)
    
    print(f"  Image: {img.name} ({img.id})")
    return img


def find_size(driver, size_id):
    """Find exact size by ID."""
    sizes = driver.list_sizes()
    for s in sizes:
        if s.id == size_id:
            return s
    print(f"  ERROR: Size '{size_id}' not found!")
    sys.exit(1)


def find_location(driver, region_id):
    """Find location by ID."""
    regions = driver.list_locations()
    for r in regions:
        if r.id == region_id:
            return r
    print(f"  ERROR: Region '{region_id}' not found!")
    available = [r.id for r in regions]
    print(f"  Available: {', '.join(available)}")
    sys.exit(1)


def main():
    print("=" * 60)
    print("  MESH CLUSTER — DigitalOcean Provisioning")
    print("=" * 60)
    print()

    # Check for existing state
    state = load_state()
    if state["nodes"]:
        print("⚠️  Existing cluster state found:")
        for n in state["nodes"]:
            print(f"  {n['name']:25s}  {n.get('public_ip', '?'):16s}  {n.get('status', '?')}")
        print()
        resp = input("Continue with new cluster? Existing nodes will NOT be affected. [y/N]: ")
        if resp.lower() != "y":
            print("Aborted.")
            return

    # ── Step 1: Credentials ──────────────────────────────────────────────
    print("[1/6] Resolving credentials...")
    try:
        creds = get_credentials("digitalocean")
        masked = creds["key"][:8] + "..." + creds["key"][-4:]
        print(f"  DO API Key: {masked}")
    except ValueError as e:
        print(f"  ERROR: {e}")
        sys.exit(1)

    ts_auth_key = os.environ.get("TAILSCALE_KEY", "")
    if not ts_auth_key:
        print("  ERROR: TAILSCALE_KEY not set in .env")
        sys.exit(1)
    print(f"  Tailscale Auth Key: {ts_auth_key[:14]}...{ts_auth_key[-4:]}")
    print()

    # ── Step 2: Connect to DigitalOcean ──────────────────────────────────
    print("[2/6] Connecting to DigitalOcean API...")
    driver = get_driver("digitalocean", credentials=creds)
    image = find_ubuntu_image(driver)
    location = find_location(driver, REGION)
    print(f"  Region: {location.name} ({location.id})")
    
    # Fetch SSH keys to attach
    ssh_keys = driver.list_key_pairs()
    ssh_key_fps = [k.fingerprint for k in ssh_keys]
    if ssh_key_fps:
        print(f"  Will attach {len(ssh_key_fps)} SSH key(s) to droplets.")
    else:
        print("  WARNING: No SSH keys found. You won't be able to SSH into the droplets.")

    print()

    # ── Step 3: Generate boot scripts ────────────────────────────────────
    print("[3/6] Generating boot scripts...")

    # Leader's Tailscale IP won't be known until it boots.
    # The boot script uses LEADER_IP for workers to join the Consul/Nomad cluster.
    # For the leader, leader_ip is "self" (it IS the leader).
    # For workers, we'll need the leader's Tailscale IP.
    # Strategy: Create leader first, wait for its IP, then create workers.

    leader_boot = generate_cloud_init_yaml(
        tailscale_key=ts_auth_key,
        leader_ip="127.0.0.1",  # Leader bootstraps to itself
        role="server",
        validate=True,
    )
    print(f"  Leader boot script: {len(leader_boot)} bytes")
    print()

    # ── Step 4: Provision Leader ─────────────────────────────────────────
    leader_node_cfg = NODES[0]
    print(f"[4/6] Provisioning LEADER: {leader_node_cfg['name']}...")
    leader_size = find_size(driver, leader_node_cfg["size_id"])
    print(f"  Size: {leader_size.id} ({leader_size.ram} MB RAM)")

    try:
        leader_node = driver.create_node(
            name=leader_node_cfg["name"],
            size=leader_size,
            image=image,
            location=location,
            ex_user_data=leader_boot,
            ex_ssh_key_ids=ssh_key_fps,
        )
        print(f"  ✓ Leader created! ID: {leader_node.id}")
    except Exception as e:
        print(f"  ✗ FAILED: {e}")
        sys.exit(1)

    # Wait for leader to get a public IP
    print("  Waiting for leader IP assignment...", end="", flush=True)
    for i in range(60):
        time.sleep(5)
        print(".", end="", flush=True)
        try:
            # Refresh node info
            nodes = driver.list_nodes()
            for n in nodes:
                if n.id == leader_node.id:
                    leader_node = n
                    break
            if leader_node.public_ips:
                break
        except Exception:
            pass
    print()

    leader_public_ip = leader_node.public_ips[0] if leader_node.public_ips else "UNKNOWN"
    leader_private_ip = leader_node.private_ips[0] if leader_node.private_ips else "UNKNOWN"
    print(f"  Leader Public IP:  {leader_public_ip}")
    print(f"  Leader Private IP: {leader_private_ip}")
    print()

    # Save leader to state
    state = {
        "cluster_name": CLUSTER_NAME,
        "region": REGION,
        "created_at": datetime.now().isoformat(),
        "nodes": [{
            "name": leader_node_cfg["name"],
            "role": "server",
            "droplet_id": leader_node.id,
            "public_ip": leader_public_ip,
            "private_ip": leader_private_ip,
            "size_id": leader_node_cfg["size_id"],
            "status": str(leader_node.state),
        }],
    }
    save_state(state)

    # ── Step 5: Provision Workers ────────────────────────────────────────
    # Workers need the leader's IP to join the cluster.
    # They'll use the leader's public IP (or Tailscale IP once connected).
    print(f"[5/6] Provisioning WORKERS...")

    for worker_cfg in NODES[1:]:
        print(f"\n  Creating {worker_cfg['name']}...")
        worker_size = find_size(driver, worker_cfg["size_id"])
        print(f"  Size: {worker_size.id} ({worker_size.ram} MB RAM)")

        # Generate worker boot script pointing to leader via Tailscale MagicDNS
        worker_boot = generate_cloud_init_yaml(
            tailscale_key=ts_auth_key,
            leader_ip=f"node-{CLUSTER_NAME}-leader",  # Workers connect to leader via MagicDNS
            role="client",
            validate=True,
        )

        try:
            worker_node = driver.create_node(
                name=worker_cfg["name"],
                size=worker_size,
                image=image,
                location=location,
                ex_user_data=worker_boot,
                ex_ssh_key_ids=ssh_key_fps,
            )
            print(f"  ✓ {worker_cfg['name']} created! ID: {worker_node.id}")

            # Wait briefly for IP
            print("  Waiting for IP...", end="", flush=True)
            for i in range(30):
                time.sleep(3)
                print(".", end="", flush=True)
                try:
                    nodes = driver.list_nodes()
                    for n in nodes:
                        if n.id == worker_node.id:
                            worker_node = n
                            break
                    if worker_node.public_ips:
                        break
                except Exception:
                    pass
            print()

            worker_public_ip = worker_node.public_ips[0] if worker_node.public_ips else "PENDING"
            worker_private_ip = worker_node.private_ips[0] if worker_node.private_ips else "PENDING"
            print(f"  Public IP:  {worker_public_ip}")
            print(f"  Private IP: {worker_private_ip}")

            # Add to state
            state["nodes"].append({
                "name": worker_cfg["name"],
                "role": "client",
                "droplet_id": worker_node.id,
                "public_ip": worker_public_ip,
                "private_ip": worker_private_ip,
                "size_id": worker_cfg["size_id"],
                "status": str(worker_node.state),
            })
            save_state(state)

        except Exception as e:
            print(f"  ✗ FAILED: {e}")
            # Continue with other workers

    # ── Step 6: Summary ──────────────────────────────────────────────────
    print()
    print("=" * 60)
    print("  CLUSTER READY")
    print("=" * 60)
    print()
    print(f"  Cluster: {CLUSTER_NAME}")
    print(f"  Region:  {REGION} ({location.name})")
    print(f"  Nodes:   {len(state['nodes'])}")
    print()
    for n in state["nodes"]:
        role_icon = "👑" if n["role"] == "server" else "⚙️ "
        print(f"  {role_icon} {n['name']:25s}  {n['public_ip']:16s}  {n['size_id']}")
    print()
    print(f"  State saved to: {STATE_FILE}")
    print()
    print("  Boot scripts are running. Services will be ready in ~3-5 minutes:")
    print("    • Tailscale (mesh network)")
    print("    • Consul (service discovery)")
    print("    • Nomad (container scheduler)")
    print("    • Docker (container runtime)")
    print()
    print("  To check status, SSH to leader or check DO console:")
    print(f"    ssh root@{state['nodes'][0].get('public_ip', '?')}")
    print()
    print("  To destroy the cluster:")
    print("    python3 destroy_do_cluster.py")
    print()


if __name__ == "__main__":
    main()
