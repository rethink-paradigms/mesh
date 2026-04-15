"""
Feature: Multi-Node E2E Test Scenarios

End-to-end tests for production-like multi-node cluster scenarios.

Usage:
    pytest src/verification/e2e_multi_node_scenarios/ -m e2e --run-e2e

Test Scenarios:
    - test_multi_node_app_scheduling: Verify workload distribution across workers
    - test_worker_failure_rescheduling: Verify automatic rescheduling on failure
    - test_cross_cloud_mesh_connectivity: Verify Tailscale cross-cloud mesh
    - test_multi_node_service_discovery: Verify Consul service discovery
"""

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
    cleanup_job
)

__all__ = [
    "ClusterConfig",
    "get_cluster_nodes",
    "deploy_job",
    "wait_for_allocation",
    "get_allocation_nodes",
    "stop_nomad_client",
    "start_nomad_client",
    "verify_service_discovery",
    "check_tailscale_mesh",
    "cleanup_job"
]
