"""
Tests for Integration-Style Scenarios

Validates complete provisioning scenarios:
- Leader and worker provisioning
- Multiple workers with leader
- GPU worker with spot handling
- Full stack with dependencies
"""

import pytest
import pulumi
import os
import sys

# Add src to path for imports
sys.path.insert(0, os.path.join(os.path.dirname(__file__), "../.."))

from mesh.infrastructure.provision_node.provision_node import provision_node, GPUConfig, SpotConfig

created_resources = {}


class MyMocks(pulumi.runtime.Mocks):
    def new_resource(self, args: pulumi.runtime.MockResourceArgs):
        created_resources[args.name] = args.inputs
        state = args.inputs.copy()
        if args.typ == "aws:ec2/instance:Instance":
            state["publicIp"] = "1.2.3.4"
            state["privateIp"] = "10.0.0.1"
        return [args.name + "_id", state]

    def call(self, args: pulumi.runtime.MockCallArgs):
        if args.token == "aws:ec2/getAmi:getAmi":
            return {"id": "ami-0abcdef1234567890"}
        return {}


@pytest.fixture(autouse=True)
def setup_mocks():
    pulumi.runtime.set_mocks(MyMocks(), preview=False)


@pulumi.runtime.test
def test_leader_and_worker_provisioning():
    """
    Test_LeaderAndWorker_Provisioning: Verify leader + worker scenario.

    Validates that a complete leader + worker scenario works correctly,
    simulating the typical cluster deployment pattern.
    """
    created_resources.clear()

    # Provision leader (server role)
    leader = provision_node(
        name="integration-leader",
        provider="aws",
        role="server",
        size="t3.small",
        tailscale_auth_key=pulumi.Output.secret("integration-ts-key"),
        leader_ip="127.0.0.1",
    )

    # Provision worker (client role)
    worker = provision_node(
        name="integration-worker",
        provider="aws",
        role="client",
        size="t3.micro",
        tailscale_auth_key=pulumi.Output.secret("integration-ts-key"),
        leader_ip="integration-leader",
    )

    def check_both_provisioned(args):
        leader_id, worker_id, leader_ip, worker_ip = args
        # Verify leader
        assert "integration-leader_id" in leader_id
        assert leader_ip == "1.2.3.4"

        # Verify worker
        assert "integration-worker_id" in worker_id
        assert worker_ip == "1.2.3.4"

        # Verify resources were created
        assert "integration-leader" in created_resources
        assert "integration-worker" in created_resources

        # Verify roles in tags
        leader_tags = created_resources["integration-leader"]["tags"]
        worker_tags = created_resources["integration-worker"]["tags"]
        assert leader_tags["Role"] == "server"
        assert worker_tags["Role"] == "client"

    return pulumi.Output.all(
        leader["instance_id"], worker["instance_id"], leader["public_ip"], worker["public_ip"]
    ).apply(check_both_provisioned)


@pulumi.runtime.test
def test_multiple_workers_with_leader():
    """
    Test_MultipleWorkers_WithLeader: Verify multiple workers can work with same leader.

    Validates that a single leader can support multiple worker nodes,
    simulating a real cluster with multiple workers.
    """
    created_resources.clear()

    # Provision leader
    leader = provision_node(
        name="multi-leader",
        provider="aws",
        role="server",
        size="t3.small",
        tailscale_auth_key=pulumi.Output.secret("multi-ts-key"),
        leader_ip="127.0.0.1",
    )

    # Provision multiple workers
    worker1 = provision_node(
        name="multi-worker-1",
        provider="aws",
        role="client",
        size="t3.micro",
        tailscale_auth_key=pulumi.Output.secret("multi-ts-key"),
        leader_ip="multi-leader",
    )

    worker2 = provision_node(
        name="multi-worker-2",
        provider="aws",
        role="client",
        size="t3.micro",
        tailscale_auth_key=pulumi.Output.secret("multi-ts-key"),
        leader_ip="multi-leader",
    )

    worker3 = provision_node(
        name="multi-worker-3",
        provider="aws",
        role="client",
        size="t3.micro",
        tailscale_auth_key=pulumi.Output.secret("multi-ts-key"),
        leader_ip="multi-leader",
    )

    def check_all_provisioned(args):
        leader_id, w1_id, w2_id, w3_id = args
        # Verify all nodes created
        assert "multi-leader_id" in leader_id
        assert "multi-worker-1_id" in w1_id
        assert "multi-worker-2_id" in w2_id
        assert "multi-worker-3_id" in w3_id

        # Verify all node resources created (may include additional AWS resources)
        assert "multi-leader" in created_resources
        assert "multi-worker-1" in created_resources
        assert "multi-worker-2" in created_resources
        assert "multi-worker-3" in created_resources

    return pulumi.Output.all(
        leader["instance_id"],
        worker1["instance_id"],
        worker2["instance_id"],
        worker3["instance_id"],
    ).apply(check_all_provisioned)


@pulumi.runtime.test
def test_gpu_worker_with_spot_handling():
    """
    Test_GPUWorker_WithSpotHandling: Verify GPU + spot handling combination.

    Validates that a worker node can have both GPU support and spot
    instance interruption handling enabled simultaneously.
    """
    created_resources.clear()

    # Provision leader
    leader = provision_node(
        name="gpu-spot-leader",
        provider="aws",
        role="server",
        size="t3.small",
        tailscale_auth_key=pulumi.Output.secret("gpu-spot-key"),
        leader_ip="127.0.0.1",
    )

    # Provision GPU worker with spot handling
    gpu_worker = provision_node(
        name="gpu-spot-worker",
        provider="aws",
        role="client",
        size="g4dn.xlarge",
        tailscale_auth_key=pulumi.Output.secret("gpu-spot-key"),
        leader_ip="gpu-spot-leader",
        gpu_config=GPUConfig(enable_gpu=True, cuda_version="12.1", nvidia_driver_version="535"),
        spot_config=SpotConfig(
            enable_spot_handling=True, spot_check_interval=5, spot_grace_period=90
        ),
    )

    def check_gpu_spot_config(worker_id):
        # Verify worker created
        assert "gpu-spot-worker_id" in worker_id

        # Check boot script has both GPU and spot handling
        inputs = created_resources["gpu-spot-worker"]
        user_data = inputs.get("userData", {})

        # Handle different user_data formats
        if isinstance(user_data, dict) and "value" in user_data:
            user_data = user_data["value"]

        # Verify GPU support
        assert "04-install-gpu-drivers.sh" in user_data
        assert 'CUDA_VERSION="12.1"' in user_data

        # Verify spot handling
        assert "09-handle-spot-interruption.sh" in user_data
        assert 'SPOT_CHECK_INTERVAL="5"' in user_data

    return gpu_worker["instance_id"].apply(check_gpu_spot_config)


@pulumi.runtime.test
def test_full_stack_with_dependencies():
    """
    Test_FullStack_WithDependencies: Verify complete provisioning stack.

    Validates a realistic full-stack deployment scenario:
    - 1 leader node
    - 2 worker nodes
    - 1 GPU worker with spot handling
    - All outputs accessible
    """
    created_resources.clear()

    # Leader
    leader = provision_node(
        name="full-leader",
        provider="aws",
        role="server",
        size="t3.small",
        tailscale_auth_key=pulumi.Output.secret("full-stack-key"),
        leader_ip="127.0.0.1",
    )

    # Regular worker 1
    worker1 = provision_node(
        name="full-worker-1",
        provider="aws",
        role="client",
        size="t3.micro",
        tailscale_auth_key=pulumi.Output.secret("full-stack-key"),
        leader_ip="full-leader",
    )

    # Regular worker 2
    worker2 = provision_node(
        name="full-worker-2",
        provider="aws",
        role="client",
        size="t3.micro",
        tailscale_auth_key=pulumi.Output.secret("full-stack-key"),
        leader_ip="full-leader",
    )

    # GPU worker with spot handling
    gpu_worker = provision_node(
        name="full-gpu-worker",
        provider="aws",
        role="client",
        size="g4dn.xlarge",
        tailscale_auth_key=pulumi.Output.secret("full-stack-key"),
        leader_ip="full-leader",
        gpu_config=GPUConfig(enable_gpu=True),
        spot_config=SpotConfig(enable_spot_handling=True),
    )

    def check_full_stack(args):
        leader_id, w1_id, w2_id, gpu_id = args
        # Verify all instance IDs
        assert "full-leader_id" in leader_id
        assert "full-worker-1_id" in w1_id
        assert "full-worker-2_id" in w2_id
        assert "full-gpu-worker_id" in gpu_id

        # Verify all node resources created (may include additional AWS resources)
        assert "full-leader" in created_resources
        assert "full-worker-1" in created_resources
        assert "full-worker-2" in created_resources
        assert "full-gpu-worker" in created_resources

    return pulumi.Output.all(
        leader["instance_id"],
        worker1["instance_id"],
        worker2["instance_id"],
        gpu_worker["instance_id"],
    ).apply(check_full_stack)


@pulumi.runtime.test
def test_cluster_provisioning_sequence():
    """
    Test_Cluster_ProvisioningSequence: Verify sequential cluster provisioning.

    Validates that cluster nodes can be provisioned in the correct sequence.
    """
    created_resources.clear()

    # Step 1: Leader
    leader = provision_node(
        name="seq-leader",
        provider="aws",
        role="server",
        size="t3.small",
        tailscale_auth_key=pulumi.Output.secret("seq-key"),
        leader_ip="127.0.0.1",
    )

    # Step 2: First worker
    worker1 = provision_node(
        name="seq-worker-1",
        provider="aws",
        role="client",
        size="t3.micro",
        tailscale_auth_key=pulumi.Output.secret("seq-key"),
        leader_ip="seq-leader",
    )

    # Step 3: Second worker
    worker2 = provision_node(
        name="seq-worker-2",
        provider="aws",
        role="client",
        size="t3.micro",
        tailscale_auth_key=pulumi.Output.secret("seq-key"),
        leader_ip="seq-leader",
    )

    def check_sequence(args):
        leader_id, w1_id, w2_id = args
        # All should be created
        assert "seq-leader_id" in leader_id
        assert "seq-worker-1_id" in w1_id
        assert "seq-worker-2_id" in w2_id

    return pulumi.Output.all(
        leader["instance_id"], worker1["instance_id"], worker2["instance_id"]
    ).apply(check_sequence)


@pulumi.runtime.test
def test_cross_region_cluster_simulation():
    """
    Test_CrossRegion_ClusterSimulation: Simulate cross-region cluster.

    Validates that multiple nodes can be created that simulate a cross-region
    deployment (all in same region for test, but different IPs would apply).
    """
    created_resources.clear()

    # All nodes in same region for test, but simulating cross-region pattern
    region1_leader = provision_node(
        name="region1-leader",
        provider="aws",
        role="server",
        size="t3.small",
        tailscale_auth_key=pulumi.Output.secret("cross-region-key"),
        leader_ip="127.0.0.1",
    )

    region1_worker = provision_node(
        name="region1-worker",
        provider="aws",
        role="client",
        size="t3.micro",
        tailscale_auth_key=pulumi.Output.secret("cross-region-key"),
        leader_ip="region1-leader",
    )

    def cross_region_check(args):
        leader_id, worker_id = args
        # Verify both created
        assert "region1-leader_id" in leader_id
        assert "region1-worker_id" in worker_id

    return pulumi.Output.all(region1_leader["instance_id"], region1_worker["instance_id"]).apply(
        cross_region_check
    )
