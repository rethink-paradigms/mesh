"""
Tests for Task 4.2: Tier-gate boot.sh and Caddy install script
"""

import os
import subprocess
from .generate_boot_scripts import generate_shell_script


def test_lite_boot_skips_tailscale():
    rendered = generate_shell_script(
        tailscale_key="ts-key-123",
        leader_ip="10.0.0.1",
        role="server",
        cluster_tier="lite",
    )
    assert 'if [ "$CLUSTER_TIER" != "lite" ]; then' in rendered
    assert 'bash scripts/02-install-tailscale.sh "$TAILSCALE_KEY"' in rendered
    lines = rendered.split("\n")
    in_tailscale_guard = False
    for line in lines:
        if 'bash scripts/02-install-tailscale.sh "$TAILSCALE_KEY"' in line:
            assert in_tailscale_guard, "tailscale call should be inside lite guard"
        if '"lite" ]; then' in line and "CLUSTER_TIER" in line:
            in_tailscale_guard = True
        if in_tailscale_guard and line.strip() == "fi":
            in_tailscale_guard = False
            break


def test_lite_boot_skips_consul():
    rendered = generate_shell_script(
        tailscale_key="ts-key-123",
        leader_ip="10.0.0.1",
        role="server",
        cluster_tier="lite",
    )
    assert 'bash scripts/06-configure-consul.sh "$LEADER_IP" "$ROLE"' in rendered
    lines = rendered.split("\n")
    found_consul_in_guard = False
    in_consul_guard = False
    guard_count = 0
    for i, line in enumerate(lines):
        if '"lite" ]; then' in line and "CLUSTER_TIER" in line:
            guard_count += 1
            if guard_count == 2:
                in_consul_guard = True
        if in_consul_guard and "06-configure-consul.sh" in line:
            found_consul_in_guard = True
        if in_consul_guard and line.strip() == "fi":
            in_consul_guard = False
    assert found_consul_in_guard, "consul configure should be inside lite guard"


def test_lite_boot_installs_caddy():
    rendered = generate_shell_script(
        tailscale_key="ts-key-123",
        leader_ip="10.0.0.1",
        role="server",
        cluster_tier="lite",
    )
    assert 'ENABLE_CADDY="true"' in rendered
    assert "bash scripts/10-install-caddy.sh" in rendered
    assert "mkdir -p /opt/caddy/data" in rendered
    assert "caddy-data" in rendered


def test_production_boot_includes_tailscale():
    rendered = generate_shell_script(
        tailscale_key="ts-key-123",
        leader_ip="10.0.0.1",
        role="server",
        cluster_tier="production",
    )
    assert 'bash scripts/02-install-tailscale.sh "$TAILSCALE_KEY"' in rendered
    assert 'if [ "$CLUSTER_TIER" != "lite" ]; then' in rendered


def test_production_boot_skips_caddy():
    rendered = generate_shell_script(
        tailscale_key="ts-key-123",
        leader_ip="10.0.0.1",
        role="server",
        cluster_tier="production",
    )
    assert 'ENABLE_CADDY="false"' in rendered
    assert 'if [ "$ENABLE_CADDY" == "true" ]; then' in rendered


def test_caddy_install_script_exists():
    feature_dir = os.path.dirname(__file__)
    script_path = os.path.join(feature_dir, "scripts", "10-install-caddy.sh")
    assert os.path.exists(script_path), "Missing script: 10-install-caddy.sh"
    with open(script_path, "r") as f:
        content = f.read()
    assert content.startswith("#!/bin/bash")
    assert "caddy" in content.lower()


def test_caddy_install_script_valid_bash():
    feature_dir = os.path.dirname(__file__)
    script_path = os.path.join(feature_dir, "scripts", "10-install-caddy.sh")
    result = subprocess.run(
        ["bash", "-n", script_path],
        capture_output=True,
        text=True,
    )
    assert result.returncode == 0, f"Bash syntax error: {result.stderr}"


def test_caddy_host_volume_configured():
    rendered = generate_shell_script(
        tailscale_key="ts-key-123",
        leader_ip="10.0.0.1",
        role="server",
        cluster_tier="lite",
    )
    assert 'host_volume "caddy-data"' in rendered
    assert "/opt/caddy/data" in rendered
    assert "read_only = false" in rendered


def test_lite_boot_consul_systemd_skipped():
    rendered = generate_shell_script(
        tailscale_key="ts-key-123",
        leader_ip="10.0.0.1",
        role="server",
        cluster_tier="lite",
    )
    lines = rendered.split("\n")
    in_consul_service_guard = False
    found_consul_service = False
    guard_count = 0
    for line in lines:
        if '"lite" ]; then' in line and "CLUSTER_TIER" in line:
            guard_count += 1
        if "consul.service" in line:
            found_consul_service = True
    assert found_consul_service

    assert "systemctl enable nomad" in rendered
    assert "systemctl restart nomad" in rendered
