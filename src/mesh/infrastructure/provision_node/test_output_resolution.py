"""
Tests for Output Resolution

Validates that Pulumi Outputs resolve correctly:
- public_ip Output resolves
- private_ip Output resolves
- instance_id Output resolves
- .apply() chains work correctly
- Output.all() for multiple values
"""

import pytest
import pulumi
import os
import sys

# Add src to path for imports
sys.path.insert(0, os.path.join(os.path.dirname(__file__), "../.."))

from mesh.infrastructure.provision_node.provision_node import provision_node

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
def test_public_ip_output_resolves():
    """
    Test_PublicIP_OutputResolves: Verify public_ip Output resolves correctly.

    Validates that the public_ip output returns the mocked IP address.
    """
    created_resources.clear()

    outputs = provision_node(
        name="test-public-ip",
        provider="aws",
        role="client",
        size="t3.micro",
        tailscale_auth_key=pulumi.Output.secret("test-key"),
        leader_ip="127.0.0.1",
    )

    def check_public_ip(public_ip):
        assert public_ip == "1.2.3.4", f"Expected '1.2.3.4', got '{public_ip}'"

    return outputs["public_ip"].apply(check_public_ip)


@pulumi.runtime.test
def test_private_ip_output_resolves():
    """
    Test_PrivateIP_OutputResolves: Verify private_ip Output resolves correctly.

    Validates that the private_ip output returns the mocked IP address.
    """
    created_resources.clear()

    outputs = provision_node(
        name="test-private-ip",
        provider="aws",
        role="client",
        size="t3.micro",
        tailscale_auth_key=pulumi.Output.secret("test-key"),
        leader_ip="127.0.0.1",
    )

    def check_private_ip(private_ip):
        assert private_ip == "10.0.0.1", f"Expected '10.0.0.1', got '{private_ip}'"

    return outputs["private_ip"].apply(check_private_ip)


@pulumi.runtime.test
def test_instance_id_output_resolves():
    """
    Test_InstanceID_OutputResolves: Verify instance_id Output resolves correctly.

    Validates that the instance_id output contains the resource name.
    """
    created_resources.clear()

    outputs = provision_node(
        name="test-instance-id",
        provider="aws",
        role="client",
        size="t3.micro",
        tailscale_auth_key=pulumi.Output.secret("test-key"),
        leader_ip="127.0.0.1",
    )

    def check_instance_id(instance_id):
        assert (
            "test-instance-id_id" in instance_id
        ), f"Expected 'test-instance-id_id' in '{instance_id}'"

    return outputs["instance_id"].apply(check_instance_id)


@pulumi.runtime.test
def test_output_chaining_with_apply():
    """
    Test_OutputChaining_WithApply: Verify .apply() chains work correctly.

    Validates that we can chain multiple .apply() operations on outputs.
    """
    created_resources.clear()

    outputs = provision_node(
        name="test-chaining",
        provider="aws",
        role="client",
        size="t3.micro",
        tailscale_auth_key=pulumi.Output.secret("test-key"),
        leader_ip="127.0.0.1",
    )

    # Chain multiple .apply() operations
    def add_suffix(instance_id):
        return instance_id + "-suffix"

    def check_chained(result):
        assert "test-chaining_id-suffix" in result

    return outputs["instance_id"].apply(add_suffix).apply(check_chained)


@pulumi.runtime.test
def test_output_all_combination():
    """
    Test_OutputAll_Combination: Verify Output.all() for multiple values.

    Validates that we can combine multiple outputs using Output.all().
    """
    created_resources.clear()

    outputs = provision_node(
        name="test-output-all",
        provider="aws",
        role="server",
        size="t3.small",
        tailscale_auth_key=pulumi.Output.secret("test-key"),
        leader_ip="127.0.0.1",
    )

    def check_all_outputs(args):
        public_ip, private_ip, instance_id = args
        assert public_ip == "1.2.3.4"
        assert private_ip == "10.0.0.1"
        assert "test-output-all_id" in instance_id

    return pulumi.Output.all(
        outputs["public_ip"], outputs["private_ip"], outputs["instance_id"]
    ).apply(check_all_outputs)


@pulumi.runtime.test
def test_outputs_are_pulumi_outputs():
    """
    Test_Outputs_ArePulumiOutputs: Verify outputs are Pulumi Output objects.

    Validates that the returned values are actual Pulumi Output objects.
    """
    created_resources.clear()

    outputs = provision_node(
        name="test-output-types",
        provider="aws",
        role="client",
        size="t3.micro",
        tailscale_auth_key=pulumi.Output.secret("test-key"),
        leader_ip="127.0.0.1",
    )

    def check_output_types(instance_id):
        # All outputs should be Pulumi Output objects
        assert isinstance(
            outputs["public_ip"], pulumi.Output
        ), "public_ip should be a Pulumi Output"
        assert isinstance(
            outputs["private_ip"], pulumi.Output
        ), "private_ip should be a Pulumi Output"
        assert isinstance(
            outputs["instance_id"], pulumi.Output
        ), "instance_id should be a Pulumi Output"

    return outputs["instance_id"].apply(check_output_types)


@pulumi.runtime.test
def test_output_values_accessible():
    """
    Test_OutputValues_Accessible: Verify output values are accessible.

    Validates that all expected output keys are present in the result.
    """
    created_resources.clear()

    outputs = provision_node(
        name="test-output-keys",
        provider="aws",
        role="client",
        size="t3.micro",
        tailscale_auth_key=pulumi.Output.secret("test-key"),
        leader_ip="127.0.0.1",
    )

    def check_keys_present(instance_id):
        assert "public_ip" in outputs, "public_ip key should be present"
        assert "private_ip" in outputs, "private_ip key should be present"
        assert "instance_id" in outputs, "instance_id key should be present"

    return outputs["instance_id"].apply(check_keys_present)


@pulumi.runtime.test
def test_multiple_nodes_distinct_outputs():
    """
    Test_MultipleNodes_DistinctOutputs: Verify multiple nodes have distinct outputs.

    Validates that provisioning multiple nodes results in distinct output values.
    """
    created_resources.clear()

    node1 = provision_node(
        name="test-node-1",
        provider="aws",
        role="client",
        size="t3.micro",
        tailscale_auth_key=pulumi.Output.secret("test-key"),
        leader_ip="127.0.0.1",
    )

    node2 = provision_node(
        name="test-node-2",
        provider="aws",
        role="client",
        size="t3.micro",
        tailscale_auth_key=pulumi.Output.secret("test-key"),
        leader_ip="127.0.0.1",
    )

    def check_distinct(args):
        node1_id, node2_id = args
        assert "test-node-1_id" in node1_id
        assert "test-node-2_id" in node2_id
        assert node1_id != node2_id, "Instance IDs should be distinct"

    return pulumi.Output.all(node1["instance_id"], node2["instance_id"]).apply(check_distinct)


@pulumi.runtime.test
def test_output_resolution_with_gpu():
    """
    Test_OutputResolution_WithGPU: Verify outputs resolve correctly with GPU config.

    Validates that outputs work correctly when GPU configuration is provided.
    """
    created_resources.clear()

    from mesh.infrastructure.provision_node.provision_node import GPUConfig

    outputs = provision_node(
        name="test-gpu-outputs",
        provider="aws",
        role="client",
        size="g4dn.xlarge",
        tailscale_auth_key=pulumi.Output.secret("test-key"),
        leader_ip="vm-leader",
        gpu_config=GPUConfig(enable_gpu=True),
    )

    def check_gpu_outputs(args):
        public_ip, private_ip, instance_id = args
        assert public_ip == "1.2.3.4"
        assert private_ip == "10.0.0.1"
        assert "test-gpu-outputs_id" in instance_id

    return pulumi.Output.all(
        outputs["public_ip"], outputs["private_ip"], outputs["instance_id"]
    ).apply(check_gpu_outputs)


@pulumi.runtime.test
def test_output_resolution_with_spot():
    """
    Test_OutputResolution_WithSpot: Verify outputs resolve correctly with spot config.

    Validates that outputs work correctly when spot configuration is provided.
    """
    created_resources.clear()

    from mesh.infrastructure.provision_node.provision_node import SpotConfig

    outputs = provision_node(
        name="test-spot-outputs",
        provider="aws",
        role="client",
        size="t3.micro",
        tailscale_auth_key=pulumi.Output.secret("test-key"),
        leader_ip="vm-leader",
        spot_config=SpotConfig(enable_spot_handling=True),
    )

    def check_spot_outputs(args):
        public_ip, private_ip, instance_id = args
        assert public_ip == "1.2.3.4"
        assert private_ip == "10.0.0.1"
        assert "test-spot-outputs_id" in instance_id

    return pulumi.Output.all(
        outputs["public_ip"], outputs["private_ip"], outputs["instance_id"]
    ).apply(check_spot_outputs)
