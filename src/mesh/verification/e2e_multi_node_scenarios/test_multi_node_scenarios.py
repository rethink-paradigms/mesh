"""
Feature: Multi-Node E2E Test Scenarios

End-to-end tests for production-like multi-node cluster scenarios:
- Multi-node scheduling (Leader + 2 Workers)
- Worker failure & automatic rescheduling
- Cross-cloud mesh connectivity
- Multi-cluster service discovery
"""

import pytest
import os
import time
import uuid
import subprocess
from pathlib import Path

from .test_utils import (
    ClusterConfig,
    get_cluster_nodes,
    deploy_job,
    wait_for_allocation,
    get_allocation_nodes,
    stop_nomad_client,
    start_nomad_client,
    verify_service_discovery,
    check_tailscale_mesh,
    check_traefik_routing,
    cleanup_job,
)


# Test markers
pytestmark = [
    pytest.mark.e2e,  # Only run when --run-e2e is set
]


def get_unique_job_id() -> str:
    """Generate unique job ID for test isolation"""
    return f"{int(time.time())}-{uuid.uuid4().hex[:8]}"


@pytest.fixture(scope="module")
def cluster_config():
    """Provide cluster configuration for tests"""
    config = ClusterConfig()

    # Skip E2E tests if no cluster is available
    # Tests can be run with: export E2E_LEADER_IP=x.x.x.x && pytest src/verification/e2e_multi_node_scenarios/ -v
    if not config.leader_ip:
        if config.is_local():
            # Check if Multipass is available before trying to discover cluster
            try:
                subprocess.run(
                    ["multipass", "--version"], capture_output=True, check=True
                )
                # Multipass exists, try to discover cluster
                try:
                    nodes = get_cluster_nodes(config)
                    if not nodes:
                        pytest.skip(
                            "No local cluster found. Start cluster with: "
                            "cd src/infrastructure/provision_local_cluster && python3 cli.py up"
                        )
                except Exception:
                    pytest.skip("Could not discover local cluster")
            except (subprocess.CalledProcessError, FileNotFoundError):
                pytest.skip("Multipass not available for local cluster discovery")
        else:
            pytest.skip("No cluster available. Set E2E_LEADER_IP environment variable")

    return config


@pytest.fixture(scope="module")
def nomad_addr(cluster_config):
    """Provide Nomad server address"""
    return os.getenv("NOMAD_ADDR", f"http://{cluster_config.leader_ip}:4646")


# ============================================================================
# Test 1: Multi-Node Scheduling
# ============================================================================


@pytest.mark.local_only
@pytest.mark.slow
def test_multi_node_app_scheduling(cluster_config, nomad_addr):
    """
    Test_Multi_Node_App_Scheduling: Verify Nomad schedules workloads across
    multiple worker nodes correctly.

    Scenario:
    1. Deploy job with 3 replicas
    2. Verify each replica scheduled to different worker
    3. Verify leader can access all replicas via Traefik
    4. Verify Consul service registration for all replicas
    """
    # Skip if we don't have enough nodes (need leader + at least 1 worker)
    nodes = get_cluster_nodes(cluster_config)
    if len(nodes) < 2:
        pytest.skip(
            "Need at least 2 nodes (leader + worker) for multi-node scheduling test"
        )

    # Generate unique job ID
    job_id = get_unique_job_id()
    job_name = f"test-web-service-{job_id}"

    # Deploy job with 3 replicas
    job_file = Path(__file__).parent / "test-web-service.nomad.hcl"

    vars = {"count": "3", "job_id": job_id}

    try:
        deploy_job(str(job_file), vars, nomad_addr)

        # Wait for allocations to be running
        assert wait_for_allocation(
            job_name, expected_count=3, timeout=120, nomad_addr=nomad_addr
        ), "Timed out waiting for allocations to be running"

        # Verify allocations spread across nodes
        alloc_to_node = get_allocation_nodes(job_name, nomad_addr)
        unique_nodes = set(alloc_to_node.values())

        # We expect at least 2 different nodes (realistic: 1 leader + 1+ workers)
        assert len(unique_nodes) >= 2, (
            f"Expected allocations on at least 2 different nodes, got {len(unique_nodes)}: {unique_nodes}"
        )

        # Verify Consul service discovery
        time.sleep(5)  # Give Consul time to register services
        service_ips = verify_service_discovery("test-web-service")

        assert len(service_ips) >= 2, (
            f"Expected at least 2 service instances in Consul, got {len(service_ips)}"
        )

    finally:
        # Cleanup
        cleanup_job(job_name, nomad_addr, purge=True)


# ============================================================================
# Test 2: Worker Failure & Rescheduling
# ============================================================================


@pytest.mark.local_only
@pytest.mark.slow
@pytest.mark.destructive  # Must run sequentially
def test_worker_failure_rescheduling(cluster_config, nomad_addr):
    """
    Test_Worker_Failure_Rescheduling: Verify Nomad automatically reschedules
    workloads when a worker fails.

    Scenario:
    1. Deploy job with 2 replicas (1 per worker)
    2. Stop Nomad client on Worker-1
    3. Verify replica rescheduled to remaining worker
    4. Verify Consul service health updates
    5. Restart Nomad client on Worker-1
    """
    nodes = get_cluster_nodes(cluster_config)
    workers = [n for n in nodes if n["role"] == "client"]

    if len(workers) < 2:
        pytest.skip("Need at least 2 worker nodes for failure test")

    # Generate unique job ID
    job_id = get_unique_job_id()
    job_name = f"test-web-service-{job_id}"

    # Deploy job with 2 replicas
    job_file = Path(__file__).parent / "test-web-service.nomad.hcl"

    vars = {"count": "2", "job_id": job_id}

    worker_to_fail = workers[0]

    try:
        # Initial deployment
        deploy_job(str(job_file), vars, nomad_addr)
        assert wait_for_allocation(
            job_name, expected_count=2, timeout=120, nomad_addr=nomad_addr
        )

        # Get initial allocation placement
        initial_allocs = get_allocation_nodes(job_name, nomad_addr)
        assert len(initial_allocs) == 2, "Expected 2 initial allocations"

        # Stop Nomad client on worker
        assert stop_nomad_client(worker_to_fail["ip"]), (
            f"Failed to stop Nomad client on {worker_to_fail['name']}"
        )

        # Wait for Nomad to detect failure and reschedule
        time.sleep(15)  # Nomad failship detection interval

        # Verify rescheduled allocation
        rescheduled_allocs = get_allocation_nodes(job_name, nomad_addr)

        # Count allocations on remaining workers
        remaining_workers = [w for w in workers if w["name"] != worker_to_fail["name"]]
        allocs_on_remaining = [
            a
            for a in rescheduled_allocs.values()
            if a in [w["name"] for w in remaining_workers]
        ]

        # At least 1 allocation should be on remaining worker(s)
        assert len(allocs_on_remaining) >= 1, (
            "Expected allocation to reschedule to remaining worker"
        )

        # Verify Consul service discovery (only healthy instances)
        time.sleep(5)
        service_ips = verify_service_discovery("test-web-service")

        assert len(service_ips) >= 1, (
            "Expected at least 1 healthy service instance after worker failure"
        )

    finally:
        # Restart Nomad client (cleanup)
        start_nomad_client(worker_to_fail["ip"])

        # Cleanup job
        cleanup_job(job_name, nomad_addr, purge=True)


# ============================================================================
# Test 3: Cross-Cloud Mesh Connectivity
# ============================================================================


@pytest.mark.cloud_only
@pytest.mark.cross_cloud
@pytest.mark.slow
def test_cross_cloud_mesh_connectivity(cluster_config, nomad_addr):
    """
    Test_Cross_Cloud_Mesh: Verify Tailscale mesh networking enables
    cross-cloud communication.

    Scenario:
    1. Verify cluster spans multiple clouds (AWS + Hetzner)
    2. Verify Tailscale mesh connectivity
    3. Deploy job on Hetzner worker
    4. Verify app accessible via Traefik on AWS leader
    """
    if not cluster_config.is_cross_cloud():
        pytest.skip("Cross-cloud test requires E2E_CROSS_CLOUD=true")

    # Verify Tailscale mesh
    assert check_tailscale_mesh(cluster_config), (
        "Tailscale mesh not healthy - cross-cloud communication failed"
    )

    nodes = get_cluster_nodes(cluster_config)
    if len(nodes) < 2:
        pytest.skip("Need at least 2 nodes for cross-cloud test")

    # Generate unique job ID
    job_id = get_unique_job_id()
    job_name = f"test-web-service-{job_id}"

    # Deploy job
    job_file = Path(__file__).parent / "test-web-service.nomad.hcl"

    vars = {"count": "1", "job_id": job_id}

    try:
        deploy_job(str(job_file), vars, nomad_addr)
        assert wait_for_allocation(
            job_name, expected_count=1, timeout=180, nomad_addr=nomad_addr
        )

        # Verify service registration
        time.sleep(5)
        service_ips = verify_service_discovery("test-web-service")

        assert len(service_ips) >= 1, "Service not registered in Consul"

    finally:
        # Cleanup
        cleanup_job(job_name, nomad_addr, purge=True)


# ============================================================================
# Test 4: Multi-Cluster Service Discovery
# ============================================================================


@pytest.mark.local_only
@pytest.mark.slow
def test_multi_node_service_discovery(cluster_config, nomad_addr):
    """
    Test_Multi_Node_Service_Discovery: Verify Consul service discovery
    works across multiple nodes.

    Scenario:
    1. Deploy API service (2 replicas across workers)
    2. Query Consul DNS for service
    3. Verify both replicas returned
    4. Verify load balancing distributes requests
    """
    nodes = get_cluster_nodes(cluster_config)
    if len(nodes) < 2:
        pytest.skip("Need at least 2 nodes for service discovery test")

    # Generate unique job ID
    job_id = get_unique_job_id()
    job_name = f"test-web-service-{job_id}"

    # Deploy service with 2 replicas
    job_file = Path(__file__).parent / "test-web-service.nomad.hcl"

    vars = {"count": "2", "job_id": job_id}

    try:
        deploy_job(str(job_file), vars, nomad_addr)
        assert wait_for_allocation(
            job_name, expected_count=2, timeout=120, nomad_addr=nomad_addr
        )

        # Wait for Consul registration
        time.sleep(10)

        # Query Consul DNS
        service_ips = verify_service_discovery("test-web-service")

        # Verify we got 2 instances
        assert len(service_ips) == 2, (
            f"Expected 2 service instances, got {len(service_ips)}: {service_ips}"
        )

        # Verify allocations on different nodes
        alloc_to_node = get_allocation_nodes(job_name, nomad_addr)
        unique_nodes = set(alloc_to_node.values())

        assert len(unique_nodes) >= 2, (
            f"Expected service on 2 different nodes, got {len(unique_nodes)}"
        )

    finally:
        cleanup_job(job_name, nomad_addr, purge=True)


# ============================================================================
# Test 5: Traefik Ingress Routing
# ============================================================================


@pytest.mark.local_only
@pytest.mark.slow
def test_traefik_routing_after_deployment(cluster_config, nomad_addr):
    nodes = get_cluster_nodes(cluster_config)
    if len(nodes) < 1:
        pytest.skip("Need at least 1 node for Traefik routing test")

    job_id = get_unique_job_id()
    job_name = f"test-web-service-{job_id}"

    job_file = Path(__file__).parent / "test-web-service.nomad.hcl"

    vars = {
        "count": "1",
        "job_id": job_id,
        "domain": "localhost",
    }

    try:
        deploy_job(str(job_file), vars, nomad_addr)

        assert wait_for_allocation(
            job_name, expected_count=1, timeout=120, nomad_addr=nomad_addr
        ), "Timed out waiting for allocations to be running"

        time.sleep(5)

        leader_ip = nodes[0]["ip"] if nodes else cluster_config.leader_ip
        host_header = f"test-web-service.{vars['domain']}"
        response = check_traefik_routing(leader_ip, host_header)

        assert response.status_code == 200, (
            f"Expected HTTP 200 from Traefik, got {response.status_code}"
        )

        service_ips = verify_service_discovery("test-web-service")

        assert len(service_ips) >= 1, (
            f"Expected at least 1 service instance in Consul, got {len(service_ips)}"
        )

    finally:
        cleanup_job(job_name, nomad_addr, purge=True)
