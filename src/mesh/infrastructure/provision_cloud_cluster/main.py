import pulumi
import os
import sys
from dotenv import load_dotenv

# --- PATH SETUP ---
# Add project root to sys.path to import from 'platform'
# Current: ops-platform/pulumi/__main__.py
# Root:    ../../..
sys.path.insert(
    0, os.path.abspath(os.path.join(os.path.dirname(__file__), "..", "..", ".."))
)

# Load secrets from project root .env
load_dotenv(os.path.join(os.path.dirname(__file__), "..", "..", "..", "..", ".env"))

# --- NEW IMPORTS ---
from mesh.infrastructure.configure_tailscale import configure as tailscale_feature
from mesh.infrastructure.provision_node.provision_node import provision_node

# ==============================================================================
# CONFIG
# ==============================================================================
# Read configuration from Pulumi config
# Usage:
#   pulumi config set provider digitalocean
#   pulumi config set region nyc3
#   pulumi config set size s-2vcpu-4gb
#   pulumi config set leader_node_count 1
#   pulumi config set worker_node_count 1

provider = pulumi.config.get("provider") or "aws"
region = pulumi.config.get("region") or "us-east-1"
leader_size = pulumi.config.get("leader_size") or "t3.small"
worker_size = pulumi.config.get("worker_size") or "t3.micro"
leader_node_count = pulumi.config.get_int("leader_node_count") or 1
worker_node_count = pulumi.config.get_int("worker_node_count") or 1

# Validate provider is supported
from mesh.infrastructure.providers import is_provider_supported

if not is_provider_supported(provider):
    raise ValueError(
        f"Unsupported provider '{provider}'. "
        f"Supported providers: {', '.join(sorted(['aws', 'digitalocean', 'gcp', 'azure', 'linode', 'vultr']))}"
    )

# tailscale_api_key needs to be set via `pulumi config set --secret tailscale:apiKey ...`

# ==============================================================================
# 1. NETWORK (Tailscale)
# ==============================================================================
# Create a reusable key for the cluster.
# Feature: Configure Tailscale
common_auth_key = tailscale_feature.create_auth_key("scavenger-mesh-key")

# ==============================================================================
# 2. INFRASTRUCTURE (Compute)
# ==============================================================================

# --- LEADER NODE (VM-1) ---
# Feature: Provision Node (Multi-Cloud via Libcloud)
leader = provision_node(
    name="vm-leader",
    provider=provider,
    role="server",
    size=leader_size,
    region=region,
    tailscale_auth_key=common_auth_key.key,
    leader_ip="127.0.0.1",  # Bootstrap leader
)

# --- WORKER NODE (VM-2) ---
# Feature: Provision Node (Multi-Cloud via Libcloud)
# Depends on Leader to exist.
# We pass "vm-leader" as the LEADER_IP. Consuls DNS/Tailscale DNS handles resolution.
wrapper_worker = provision_node(
    name="vm-worker-01",
    provider=provider,
    role="client",
    size=worker_size,
    region=region,
    tailscale_auth_key=common_auth_key.key,
    leader_ip="vm-leader",
    opts=pulumi.ResourceOptions(depends_on=[leader["instance_id"]]),  # Wait for leader
)

# ==============================================================================
# OUTPUTS
# ==============================================================================
pulumi.export("provider", provider)
pulumi.export("region", region)
pulumi.export("leader_public_ip", leader["public_ip"])
pulumi.export("worker_public_ip", wrapper_worker["public_ip"])
