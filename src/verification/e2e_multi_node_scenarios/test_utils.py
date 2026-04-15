"""
E2E Test Utilities

Helper functions for multi-node end-to-end testing.
Provides cluster operations, job deployment, and validation utilities.
"""

import subprocess
import time
import json
import os
from typing import List, Dict, Optional
from pathlib import Path

import requests


class ClusterConfig:
    """Configuration for E2E test cluster"""

    def __init__(self):
        self.target_env = os.getenv("E2E_TARGET_ENV", "local")
        self.leader_ip = os.getenv("E2E_LEADER_IP")
        self.worker_ips = self._parse_worker_ips()
        self.cross_cloud = os.getenv("E2E_CROSS_CLOUD", "false").lower() == "true"

        # Cross-cloud specific
        self.aws_leader_ip = os.getenv("E2E_AWS_LEADER")
        self.hetzner_worker_ip = os.getenv("E2E_HETZNER_WORKER")

    def _parse_worker_ips(self) -> List[str]:
        """Parse E2E_WORKER_IPS env var (comma-separated)"""
        ips_str = os.getenv("E2E_WORKER_IPS", "")
        return [ip.strip() for ip in ips_str.split(",") if ip.strip()]

    def is_local(self) -> bool:
        """Check if testing against local Multipass cluster"""
        return self.target_env == "local"

    def is_cloud(self) -> bool:
        """Check if testing against cloud cluster"""
        return self.target_env in ["aws", "hetzner", "cloud"]

    def is_cross_cloud(self) -> bool:
        """Check if testing cross-cloud scenario"""
        return self.cross_cloud


def get_cluster_nodes(config: Optional[ClusterConfig] = None) -> List[Dict[str, str]]:
    """
    Get list of all cluster nodes (leader + workers).

    Returns:
        List of dicts with keys: name, ip, role, status
    """
    if config is None:
        config = ClusterConfig()

    nodes = []

    # Get leader node
    leader_ip = config.leader_ip
    if not leader_ip and config.is_local():
        leader_ip = get_multipass_ip("local-leader")

    if leader_ip:
        nodes.append(
            {"name": "leader", "ip": leader_ip, "role": "server", "status": "running"}
        )

    # Get worker nodes
    if config.worker_ips:
        for idx, ip in enumerate(config.worker_ips):
            nodes.append(
                {
                    "name": f"worker-{idx + 1}",
                    "ip": ip,
                    "role": "client",
                    "status": "running",
                }
            )
    elif config.is_local():
        # Try to discover local Multipass workers
        try:
            result = subprocess.run(
                ["multipass", "list", "--format", "json"],
                capture_output=True,
                text=True,
                check=True,
            )
            vms = json.loads(result.stdout)
            # Multipass returns a list of VM objects
            if isinstance(vms, list):
                for vm in vms:
                    name = vm.get("name", "")
                    state = vm.get("state", "")
                    ipv4 = vm.get("ipv4", [])
                    if name.startswith("local-worker") and state == "Running" and ipv4:
                        nodes.append(
                            {
                                "name": name,
                                "ip": ipv4[0],
                                "role": "client",
                                "status": "running",
                            }
                        )
            else:
                # Fallback: dict format (older Multipass versions)
                for name, info in vms.get("list", {}).items():
                    if (
                        name.startswith("local-worker")
                        and info.get("state") == "Running"
                    ):
                        ipv4 = info.get("ipv4", [])
                        if ipv4:
                            nodes.append(
                                {
                                    "name": name,
                                    "ip": ipv4[0],
                                    "role": "client",
                                    "status": "running",
                                }
                            )
        except (
            subprocess.CalledProcessError,
            FileNotFoundError,
            KeyError,
            json.JSONDecodeError,
        ):
            # Local worker auto-discovery via Multipass is best-effort;
            # failures are non-fatal — tests that require workers will skip.
            return nodes

    return nodes


def get_multipass_ip(vm_name: str) -> Optional[str]:
    """
    Get IP address of a Multipass VM.

    Args:
        vm_name: Name of the Multipass VM

    Returns:
        IP address or None if not found
    """
    try:
        result = subprocess.run(
            ["multipass", "info", vm_name, "--format", "json"],
            capture_output=True,
            text=True,
            check=True,
        )
        info = json.loads(result.stdout)
        return info["info"][vm_name]["ipv4"][0]
    except (subprocess.CalledProcessError, FileNotFoundError, KeyError):
        return None


def deploy_job(
    job_file: str, vars: Dict[str, str], nomad_addr: Optional[str] = None
) -> str:
    """
    Deploy Nomad job and return job ID.

    Args:
        job_file: Path to Nomad job file
        vars: Dictionary of job variables
        nomad_addr: Nomad server address (default: localhost:4646)

    Returns:
        Job ID

    Raises:
        subprocess.CalledProcessError: If deployment fails
    """
    if nomad_addr is None:
        nomad_addr = os.getenv("NOMAD_ADDR", "http://localhost:4646")

    cmd = ["nomad", "job", "run", "-addr", nomad_addr]

    # Add variables
    for key, value in vars.items():
        cmd.extend(["-var", f"{key}={value}"])

    cmd.append(job_file)

    result = subprocess.run(cmd, capture_output=True, text=True, check=True)

    # Parse job ID from output (format: "Job 'my-job' in state: running")
    for line in result.stdout.splitlines():
        if "Job" in line and "in state" in line:
            # Extract job ID from "Job 'my-job'"
            parts = line.split("'")
            if len(parts) >= 2:
                return parts[1]

    # Fallback: derive from job filename
    return Path(job_file).stem


def wait_for_allocation(
    job_id: str,
    expected_count: int = 1,
    timeout: int = 120,
    nomad_addr: Optional[str] = None,
) -> bool:
    """
    Wait for job allocations to be running.

    Args:
        job_id: Nomad job ID
        expected_count: Expected number of running allocations
        timeout: Timeout in seconds
        nomad_addr: Nomad server address

    Returns:
        True if all allocations are running, False on timeout
    """
    if nomad_addr is None:
        nomad_addr = os.getenv("NOMAD_ADDR", "http://localhost:4646")

    start_time = time.time()

    while time.time() - start_time < timeout:
        try:
            result = subprocess.run(
                ["nomad", "job", "status", "-address", nomad_addr, "-json", job_id],
                capture_output=True,
                text=True,
                check=True,
            )

            status = json.loads(result.stdout)

            # Count running allocations
            running_count = 0
            for alloc in status.get("Status", {}).get("Allocations", []):
                if alloc.get("ClientStatus") == "running":
                    running_count += 1

            if running_count >= expected_count:
                return True

        except (subprocess.CalledProcessError, json.JSONDecodeError):
            pass

        time.sleep(2)

    return False


def get_allocation_nodes(
    job_id: str, nomad_addr: Optional[str] = None
) -> Dict[str, str]:
    """
    Return mapping of allocation ID to node name.

    Args:
        job_id: Nomad job ID
        nomad_addr: Nomad server address

    Returns:
        Dict mapping allocation IDs to node names
    """
    if nomad_addr is None:
        nomad_addr = os.getenv("NOMAD_ADDR", "http://localhost:4646")

    try:
        result = subprocess.run(
            ["nomad", "job", "allocations", "-address", nomad_addr, "-json", job_id],
            capture_output=True,
            text=True,
            check=True,
        )

        allocations = json.loads(result.stdout)

        alloc_to_node = {}
        for alloc in allocations:
            alloc_id = alloc.get("ID")
            node_name = alloc.get("NodeName")
            if alloc_id and node_name:
                alloc_to_node[alloc_id] = node_name

        return alloc_to_node

    except (subprocess.CalledProcessError, json.JSONDecodeError):
        return {}


def stop_nomad_client(node_ip: str) -> bool:
    """
    Stop Nomad client on specific node (simulate failure).

    Args:
        node_ip: IP address of the node

    Returns:
        True if successful, False otherwise
    """
    try:
        subprocess.run(
            ["ssh", f"ubuntu@{node_ip}", "sudo", "systemctl", "stop", "nomad-client"],
            capture_output=True,
            text=True,
            check=True,
            timeout=30,
        )
        return True
    except (subprocess.CalledProcessError, subprocess.TimeoutExpired):
        return False


def start_nomad_client(node_ip: str) -> bool:
    """
    Start Nomad client on specific node (recovery).

    Args:
        node_ip: IP address of the node

    Returns:
        True if successful, False otherwise
    """
    try:
        subprocess.run(
            ["ssh", f"ubuntu@{node_ip}", "sudo", "systemctl", "start", "nomad-client"],
            capture_output=True,
            text=True,
            check=True,
            timeout=30,
        )
        return True
    except (subprocess.CalledProcessError, subprocess.TimeoutExpired):
        return False


def verify_service_discovery(
    service_name: str, consul_addr: Optional[str] = None
) -> List[str]:
    """
    Query Consul DNS and return list of service IPs.

    Args:
        service_name: Name of the service (without .service.consul)
        consul_addr: Consul address (default: localhost:8500)

    Returns:
        List of service IP addresses
    """
    if consul_addr is None:
        consul_addr = os.getenv("CONSUL_ADDR", "http://localhost:8500")

    try:
        # Query Consul API for service
        url = f"{consul_addr}/v1/catalog/service/{service_name}"
        result = subprocess.run(
            ["curl", "-s", url], capture_output=True, text=True, check=True
        )

        services = json.loads(result.stdout)

        # Extract service addresses
        ips = []
        for service in services:
            if "ServiceAddress" in service and service["ServiceAddress"]:
                ips.append(service["ServiceAddress"])
            elif "Address" in service:
                ips.append(service["Address"])

        return ips

    except (subprocess.CalledProcessError, json.JSONDecodeError):
        return []


def check_tailscale_mesh(config: Optional[ClusterConfig] = None) -> bool:
    """
    Verify Tailscale mesh connectivity.

    Args:
        config: Cluster configuration

    Returns:
        True if mesh is healthy, False otherwise
    """
    nodes = get_cluster_nodes(config)

    if len(nodes) < 2:
        return False  # Need at least 2 nodes for mesh

    # Check if leader can ping workers
    leader_ip = nodes[0]["ip"] if nodes else None
    if not leader_ip:
        return False

    for node in nodes[1:]:
        try:
            # Ping Tailscale IP (100.x.x.x range)
            subprocess.run(
                [
                    "ssh",
                    f"ubuntu@{leader_ip}",
                    "ping",
                    "-c",
                    "1",
                    "-W",
                    "2",
                    node["ip"],
                ],
                capture_output=True,
                check=True,
                timeout=5,
            )
        except (subprocess.CalledProcessError, subprocess.TimeoutExpired):
            return False

    return True


def check_traefik_routing(
    leader_ip: str, host_header: str, port: int = 80, timeout: int = 10
) -> requests.Response:
    url = f"http://{leader_ip}:{port}/"
    response = requests.get(url, headers={"Host": host_header}, timeout=timeout)
    return response


def cleanup_job(
    job_id: str, nomad_addr: Optional[str] = None, purge: bool = True
) -> bool:
    """
    Stop and optionally purge a Nomad job.

    Args:
        job_id: Nomad job ID
        nomad_addr: Nomad server address
        purge: Whether to purge job from history

    Returns:
        True if successful, False otherwise
    """
    if nomad_addr is None:
        nomad_addr = os.getenv("NOMAD_ADDR", "http://localhost:4646")

    try:
        cmd = ["nomad", "job", "stop", "-address", nomad_addr]
        if purge:
            cmd.append("-purge")
        cmd.append(job_id)

        subprocess.run(cmd, capture_output=True, text=True, check=True)
        return True
    except subprocess.CalledProcessError:
        return False
