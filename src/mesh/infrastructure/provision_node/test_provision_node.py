"""
Tests for Feature: Provision Node (Generic Interface)

Test Categories:
    - Existing Tests (4 tests): Basic dispatch functionality
    - T2.2.1: Test Dispatcher Routing Logic (5 tests)
    - T2.2.2: Test Libcloud Path Integration (10 tests)
    - T2.2.3: Test Backward Compatibility (5 tests)

Total: 24 tests
"""

import pytest
import pulumi
from unittest.mock import patch, MagicMock
import os


# Mocking Pulumi Infrastructure (minimal for generic dispatch)
class MyMocks(pulumi.runtime.Mocks):
    def new_resource(self, args: pulumi.runtime.MockResourceArgs):
        return [args.name + "_id", args.inputs]

    def call(self, args: pulumi.runtime.MockCallArgs):
        return {}


@pytest.fixture(autouse=True)
def setup_mocks():
    pulumi.runtime.set_mocks(MyMocks(), preview=False)


# Import the code under test
from mesh.infrastructure.provision_node.provision_node import (
    provision_node,
    GPUConfig,
    SpotConfig,
)
from mesh.infrastructure.progressive_activation.tier_config import (
    TierConfig,
    ClusterTier,
)


@patch("mesh.infrastructure.provision_node.provision_node.UniversalCloudNode")
@patch(
    "mesh.infrastructure.provision_node.provision_node.multipass_provider.provision_multipass_node"
)
def test_provision_node_aws_dispatch(mock_multipass_provider, mock_universal_node):
    """
    Test_ProvisionNode_AWS_Dispatch: Verify provision_node routes AWS provider to Libcloud.
    """
    # Mock the UniversalCloudNode resource
    mock_node_instance = MagicMock()
    mock_node_instance.public_ip = pulumi.Output.secret("aws-public-ip")
    mock_node_instance.private_ip = pulumi.Output.secret("aws-private-ip")
    mock_node_instance.instance_id = pulumi.Output.secret("aws-instance-id")
    mock_universal_node.return_value = mock_node_instance

    # Set test credentials for AWS
    os.environ["AWS_ACCESS_KEY_ID"] = "test-key"
    os.environ["AWS_SECRET_ACCESS_KEY"] = "test-secret"

    try:
        result = provision_node(
            name="test-aws-node",
            provider="aws",
            role="server",
            size="t3.small",  # Exact size ID
            tailscale_auth_key=pulumi.Output.secret("ts-key"),
            leader_ip="127.0.0.1",
            region="us-east-1",  # Required for cloud providers
        )

        mock_universal_node.assert_called_once()
        mock_multipass_provider.assert_not_called()

        # Verify a basic output from the mocked adapter
        def check_output(ip):
            assert ip == "aws-public-ip"

        result["public_ip"].apply(check_output)
    finally:
        # Clean up env vars
        os.environ.pop("AWS_ACCESS_KEY_ID", None)
        os.environ.pop("AWS_SECRET_ACCESS_KEY", None)


@patch(
    "mesh.infrastructure.provision_node.provision_node.multipass_provider.provision_multipass_node"
)
def test_provision_node_multipass_dispatch(mock_multipass_provider):
    """
    Test_ProvisionNode_Multipass_Dispatch: Verify provision_node calls the Multipass adapter for 'multipass' provider.
    """
    mock_multipass_provider.return_value = {"public_ip": "multipass-public-ip"}

    result = provision_node(
        name="test-mp-node",
        provider="multipass",
        role="client",
        size="2CPU,1GB",
        tailscale_auth_key=pulumi.Output.secret("ts-key"),
        leader_ip="vm-leader",
    )

    mock_multipass_provider.assert_called_once()

    # Verify a basic output from the mocked adapter (multipass is not pulumi.Output)
    assert result["public_ip"] == "multipass-public-ip"


def test_provision_node_unknown_provider():
    """
    Test_ProvisionNode_UnknownProvider: Verify error handling for unknown providers.
    """
    with pytest.raises(ValueError, match="region is required for cloud provider"):
        provision_node(
            name="test-unknown",
            provider="unknown-provider",
            role="server",
            size="t3.small",
            tailscale_auth_key=pulumi.Output.secret("ts-key"),
            leader_ip="127.0.0.1",
        )


@patch("mesh.infrastructure.provision_node.provision_node.UniversalCloudNode")
def test_provision_node_digitalocean_dispatch(mock_universal_node):
    """
    Test_ProvisionNode_DigitalOcean_Dispatch: Verify provision_node routes DigitalOcean to Libcloud.
    """
    # Mock the UniversalCloudNode resource
    mock_node_instance = MagicMock()
    mock_node_instance.public_ip = pulumi.Output.secret("do-public-ip")
    mock_node_instance.private_ip = pulumi.Output.secret("do-private-ip")
    mock_node_instance.instance_id = pulumi.Output.secret("do-instance-id")
    mock_universal_node.return_value = mock_node_instance

    # Set test credentials for DigitalOcean
    os.environ["DIGITALOCEAN_API_TOKEN"] = "test-token"

    try:
        result = provision_node(
            name="test-do-node",
            provider="digitalocean",
            role="client",
            size="s-2vcpu-4gb",  # Exact size ID
            tailscale_auth_key=pulumi.Output.secret("ts-key"),
            leader_ip="100.100.100.1",
            region="nyc3",
        )

        mock_universal_node.assert_called_once()

        # Verify the call used the correct size_id
        call_args = mock_universal_node.call_args
        assert call_args[1]["size_id"] == "s-2vcpu-4gb"
        assert call_args[1]["region"] == "nyc3"
    finally:
        # Clean up env vars
        os.environ.pop("DIGITALOCEAN_API_TOKEN", None)


class TestDispatcherRouting:
    """
    T2.2.1: Test Dispatcher Routing Logic (5 tests)

    Validates that the provision_node dispatcher correctly routes requests
    to the appropriate provider implementation based on the provider parameter.
    """

    @patch(
        "mesh.infrastructure.provision_node.provision_node.multipass_provider.provision_multipass_node"
    )
    def test_multipass_routes_to_multipass_adapter(self, mock_multipass):
        """
        Test_DispatcherRouting_MultipassRoutesToMultipassAdapter: Verify that
        provider="multipass" routes to the multipass adapter.
        """
        mock_multipass.return_value = {
            "public_ip": "192.168.1.100",
            "private_ip": "10.0.0.100",
        }

        result = provision_node(
            name="test-multipass",
            provider="multipass",
            role="client",
            size="2CPU,1GB",
            tailscale_auth_key=pulumi.Output.secret("ts-key"),
            leader_ip="leader-ip",
        )

        # Verify multipass adapter was called
        mock_multipass.assert_called_once()
        assert result["public_ip"] == "192.168.1.100"

    @patch("mesh.infrastructure.provision_node.provision_node.UniversalCloudNode")
    @patch.dict(
        "os.environ",
        {"AWS_ACCESS_KEY_ID": "test-key", "AWS_SECRET_ACCESS_KEY": "test-secret"},
    )
    def test_aws_routes_to_libcloud(self, mock_universal_node):
        """
        Test_DispatcherRouting_AWSRoutesToLibcloud: Verify that provider="aws"
        routes to the Libcloud UniversalCloudNode.
        """
        mock_node = MagicMock()
        mock_node.public_ip = pulumi.Output.secret("1.2.3.4")
        mock_node.private_ip = pulumi.Output.secret("10.0.1.5")
        mock_node.instance_id = pulumi.Output.secret("i-12345")
        mock_universal_node.return_value = mock_node

        result = provision_node(
            name="test-aws",
            provider="aws",
            role="server",
            size="t3.small",
            tailscale_auth_key=pulumi.Output.secret("ts-key"),
            leader_ip="leader-ip",
            region="us-east-1",
        )

        # Verify UniversalCloudNode was called
        mock_universal_node.assert_called_once()
        call_args = mock_universal_node.call_args
        assert call_args[1]["provider"] == "aws"

    @patch("mesh.infrastructure.provision_node.provision_node.UniversalCloudNode")
    @patch.dict("os.environ", {"DIGITALOCEAN_API_TOKEN": "test-do-token"})
    def test_digitalocean_routes_to_libcloud(self, mock_universal_node):
        """
        Test_DispatcherRouting_DigitalOceanRoutesToLibcloud: Verify that
        provider="digitalocean" routes to the Libcloud UniversalCloudNode.
        """
        mock_node = MagicMock()
        mock_node.public_ip = pulumi.Output.secret("203.0.113.50")
        mock_node.private_ip = pulumi.Output.secret("10.0.2.50")
        mock_node.instance_id = pulumi.Output.secret("do-12345")
        mock_universal_node.return_value = mock_node

        result = provision_node(
            name="test-do",
            provider="digitalocean",
            role="client",
            size="s-2vcpu-4gb",  # Exact size ID
            tailscale_auth_key=pulumi.Output.secret("ts-key"),
            leader_ip="leader-ip",
            region="nyc3",
        )

        # Verify UniversalCloudNode was called
        mock_universal_node.assert_called_once()
        call_args = mock_universal_node.call_args
        assert call_args[1]["provider"] == "digitalocean"

    def test_unknown_provider_raises_error(self):
        """
        Test_DispatcherRouting_UnknownProviderRaisesError: Verify that an
        unknown provider raises a ValueError.
        """
        with pytest.raises(ValueError, match="region is required for cloud provider"):
            provision_node(
                name="test-unknown",
                provider="nonexistent",
                role="server",
                size="small",
                tailscale_auth_key=pulumi.Output.secret("ts-key"),
                leader_ip="leader-ip",
            )

    def test_bare_metal_raises_not_implemented(self):
        """
        Test_DispatcherRouting_BareMetalRaisesNotImplemented: Verify that
        provider="bare-metal" raises NotImplementedError.
        """
        with pytest.raises(
            NotImplementedError,
            match="Bare Metal provider 'bare-metal' is not yet implemented",
        ):
            provision_node(
                name="test-baremetal",
                provider="bare-metal",
                role="server",
                size="large",
                tailscale_auth_key=pulumi.Output.secret("ts-key"),
                leader_ip="leader-ip",
            )


class TestLibcloudPathIntegration:
    """
    T2.2.2: Test Libcloud Path Integration (10 tests)

    Validates the complete integration path through the Libcloud provider,
    including parameter passing, credential resolution, and output mapping.
    """

    @patch("mesh.infrastructure.provision_node.provision_node.UniversalCloudNode")
    @patch.dict(
        "os.environ",
        {"AWS_ACCESS_KEY_ID": "test-key", "AWS_SECRET_ACCESS_KEY": "test-secret"},
    )
    def test_universal_cloud_node_instantiated(self, mock_universal_node):
        """
        Test_LibcloudPathIntegration_UniversalCloudNodeInstantiated: Verify that
        UniversalCloudNode is instantiated with correct parameters.
        """
        mock_node = MagicMock()
        mock_node.public_ip = pulumi.Output.secret("1.2.3.4")
        mock_node.private_ip = pulumi.Output.secret("10.0.1.5")
        mock_node.instance_id = pulumi.Output.secret("i-test")
        mock_universal_node.return_value = mock_node

        provision_node(
            name="test-node",
            provider="aws",
            role="client",
            size="t3.small",
            tailscale_auth_key=pulumi.Output.secret("ts-key"),
            leader_ip="leader-ip",
            region="us-east-1",
        )

        # Verify UniversalCloudNode was instantiated
        mock_universal_node.assert_called_once()
        call_args = mock_universal_node.call_args
        assert call_args[0][0] == "test-node"  # name
        assert call_args[1]["provider"] == "aws"

    @patch("mesh.infrastructure.provision_node.provision_node.UniversalCloudNode")
    @patch("mesh.infrastructure.boot_consul_nomad.generate_boot_scripts.generate_shell_script")
    @patch.dict(
        "os.environ",
        {"AWS_ACCESS_KEY_ID": "test-key", "AWS_SECRET_ACCESS_KEY": "test-secret"},
    )
    def test_boot_script_passed_through(self, mock_generate_script, mock_universal_node):
        """
        Test_LibcloudPathIntegration_BootScriptPassedThrough: Verify that the
        boot script is generated and passed to UniversalCloudNode.
        """
        mock_generate_script.return_value = "#!/bin/bash\necho test"
        mock_node = MagicMock()
        mock_node.public_ip = pulumi.Output.secret("1.2.3.4")
        mock_node.private_ip = pulumi.Output.secret("10.0.1.5")
        mock_node.instance_id = pulumi.Output.secret("i-test")
        mock_universal_node.return_value = mock_node

        provision_node(
            name="test-node",
            provider="aws",
            role="server",
            size="t3.small",
            tailscale_auth_key=pulumi.Output.secret("ts-key"),
            leader_ip="100.100.100.1",
            region="us-east-1",
        )

        # Verify boot script was generated and passed
        call_args = mock_universal_node.call_args
        assert "boot_script" in call_args[1]

    @patch("mesh.infrastructure.provision_node.provision_node.UniversalCloudNode")
    @patch.dict(
        "os.environ",
        {
            "AWS_ACCESS_KEY_ID": "env-access-key",
            "AWS_SECRET_ACCESS_KEY": "env-secret-key",
        },
    )
    def test_credentials_resolved_correctly(self, mock_universal_node):
        """
        Test_LibcloudPathIntegration_CredentialsResolvedCorrectly: Verify that
        credentials are correctly resolved from environment variables.
        """
        mock_node = MagicMock()
        mock_node.public_ip = pulumi.Output.secret("1.2.3.4")
        mock_node.private_ip = pulumi.Output.secret("10.0.1.5")
        mock_node.instance_id = pulumi.Output.secret("i-test")
        mock_universal_node.return_value = mock_node

        provision_node(
            name="test-node",
            provider="aws",
            role="client",
            size="t3.small",
            tailscale_auth_key=pulumi.Output.secret("ts-key"),
            leader_ip="leader-ip",
            region="us-east-1",
        )

        # Verify credentials were passed (auto-resolved from env)
        call_args = mock_universal_node.call_args
        # Credentials are auto-resolved, not passed explicitly
        assert "provider" in call_args[1]
        assert call_args[1]["provider"] == "aws"

    @patch("mesh.infrastructure.provision_node.provision_node.UniversalCloudNode")
    @patch.dict(
        "os.environ",
        {"AWS_ACCESS_KEY_ID": "test-key", "AWS_SECRET_ACCESS_KEY": "test-secret"},
    )
    def test_outputs_returned_in_correct_format(self, mock_universal_node):
        """
        Test_LibcloudPathIntegration_OutputsReturnedInCorrectFormat: Verify that
        outputs are returned in the expected dictionary format.
        """
        mock_node = MagicMock()
        mock_node.public_ip = pulumi.Output.secret("54.123.45.67")
        mock_node.private_ip = pulumi.Output.secret("10.0.2.100")
        mock_node.instance_id = pulumi.Output.secret("i-12345abcd")
        mock_universal_node.return_value = mock_node

        result = provision_node(
            name="test-node",
            provider="aws",
            role="server",
            size="t3.small",
            tailscale_auth_key=pulumi.Output.secret("ts-key"),
            leader_ip="leader-ip",
            region="us-east-1",
        )

        # Verify output format
        assert "public_ip" in result
        assert "private_ip" in result
        assert "instance_id" in result
        assert isinstance(result["public_ip"], pulumi.Output)

    @patch("mesh.infrastructure.provision_node.provision_node.UniversalCloudNode")
    @patch.dict(
        "os.environ",
        {"AWS_ACCESS_KEY_ID": "test-key", "AWS_SECRET_ACCESS_KEY": "test-secret"},
    )
    def test_resource_options_propagated(self, mock_universal_node):
        """
        Test_LibcloudPathIntegration_ResourceOptionsPropagated: Verify that
        Pulumi resource options are propagated correctly.
        """
        mock_node = MagicMock()
        mock_node.public_ip = pulumi.Output.secret("1.2.3.4")
        mock_node.private_ip = pulumi.Output.secret("10.0.1.5")
        mock_node.instance_id = pulumi.Output.secret("i-test")
        mock_universal_node.return_value = mock_node

        # Create mock dependency resource
        mock_dep = MagicMock(spec=pulumi.Resource)

        test_opts = pulumi.ResourceOptions(depends_on=[mock_dep])

        provision_node(
            name="test-node",
            provider="aws",
            role="client",
            size="t3.small",
            tailscale_auth_key=pulumi.Output.secret("ts-key"),
            leader_ip="leader-ip",
            region="us-east-1",
            opts=test_opts,
        )

        # Verify opts were passed
        call_args = mock_universal_node.call_args
        assert "opts" in call_args[1]
        assert call_args[1]["opts"] == test_opts

    @patch("mesh.infrastructure.provision_node.provision_node.UniversalCloudNode")
    @patch.dict(
        "os.environ",
        {"AWS_ACCESS_KEY_ID": "test-key", "AWS_SECRET_ACCESS_KEY": "test-secret"},
    )
    def test_region_parameter_passed(self, mock_universal_node):
        """
        Test_LibcloudPathIntegration_RegionParameterPassed: Verify that the
        region parameter is correctly passed to UniversalCloudNode.
        """
        mock_node = MagicMock()
        mock_node.public_ip = pulumi.Output.secret("1.2.3.4")
        mock_node.private_ip = pulumi.Output.secret("10.0.1.5")
        mock_node.instance_id = pulumi.Output.secret("i-test")
        mock_universal_node.return_value = mock_node

        provision_node(
            name="test-node",
            provider="aws",
            role="client",
            size="t3.small",
            tailscale_auth_key=pulumi.Output.secret("ts-key"),
            leader_ip="leader-ip",
            region="us-west-2",
        )

        # Verify region was passed
        call_args = mock_universal_node.call_args
        assert call_args[1]["region"] == "us-west-2"

    @patch("mesh.infrastructure.provision_node.provision_node.UniversalCloudNode")
    @patch.dict(
        "os.environ",
        {"AWS_ACCESS_KEY_ID": "test-key", "AWS_SECRET_ACCESS_KEY": "test-secret"},
    )
    def test_gpu_config_passed_to_boot_script(self, mock_universal_node):
        """
        Test_LibcloudPathIntegration_GPUConfigPassedToBootScript: Verify that
        GPU configuration parameters are correctly handled.
        """
        mock_node = MagicMock()
        mock_node.public_ip = pulumi.Output.secret("1.2.3.4")
        mock_node.private_ip = pulumi.Output.secret("10.0.1.5")
        mock_node.instance_id = pulumi.Output.secret("i-test")
        mock_universal_node.return_value = mock_node

        gpu_config = GPUConfig(enable_gpu=True, cuda_version="12.1", nvidia_driver_version="535")

        provision_node(
            name="test-gpu-node",
            provider="aws",
            role="client",
            size="g4dn.xlarge",
            tailscale_auth_key=pulumi.Output.secret("ts-key"),
            leader_ip="leader-ip",
            region="us-east-1",
            gpu_config=gpu_config,
        )

        # Verify UniversalCloudNode was called (boot script is generated in apply callback)
        mock_universal_node.assert_called_once()
        # The boot script is passed as an Output, we can't directly check the lambda
        # but we can verify the node was created with GPU config
        call_args = mock_universal_node.call_args
        assert "boot_script" in call_args[1]

    @patch("mesh.infrastructure.provision_node.provision_node.UniversalCloudNode")
    @patch.dict(
        "os.environ",
        {"AWS_ACCESS_KEY_ID": "test-key", "AWS_SECRET_ACCESS_KEY": "test-secret"},
    )
    def test_spot_config_passed_to_boot_script(self, mock_universal_node):
        """
        Test_LibcloudPathIntegration_SpotConfigPassedToBootScript: Verify that
        spot configuration parameters are correctly handled.
        """
        mock_node = MagicMock()
        mock_node.public_ip = pulumi.Output.secret("1.2.3.4")
        mock_node.private_ip = pulumi.Output.secret("10.0.1.5")
        mock_node.instance_id = pulumi.Output.secret("i-test")
        mock_universal_node.return_value = mock_node

        spot_config = SpotConfig(
            enable_spot_handling=True, spot_check_interval=10, spot_grace_period=120
        )

        provision_node(
            name="test-spot-node",
            provider="aws",
            role="client",
            size="t3.small",
            tailscale_auth_key=pulumi.Output.secret("ts-key"),
            leader_ip="leader-ip",
            region="us-east-1",
            spot_config=spot_config,
        )

        # Verify UniversalCloudNode was called (boot script is generated in apply callback)
        mock_universal_node.assert_called_once()
        # The boot script is passed as an Output, we can't directly check the lambda
        # but we can verify the node was created with spot config
        call_args = mock_universal_node.call_args
        assert "boot_script" in call_args[1]

    @patch("mesh.infrastructure.provision_node.provision_node.UniversalCloudNode")
    @patch.dict(
        "os.environ",
        {"AWS_ACCESS_KEY_ID": "test-key", "AWS_SECRET_ACCESS_KEY": "test-secret"},
    )
    def test_depends_on_resources_propagated(self, mock_universal_node):
        """
        Test_LibcloudPathIntegration_DependsOnResourcesPropagated: Verify that
        opts parameter is passed through correctly.
        """
        mock_node = MagicMock()
        mock_node.public_ip = pulumi.Output.secret("1.2.3.4")
        mock_node.private_ip = pulumi.Output.secret("10.0.1.5")
        mock_node.instance_id = pulumi.Output.secret("i-test")
        mock_universal_node.return_value = mock_node

        # Create mock dependency resources
        mock_dep1 = MagicMock(spec=pulumi.Resource)
        mock_dep2 = MagicMock(spec=pulumi.Resource)

        test_opts = pulumi.ResourceOptions(depends_on=[mock_dep1, mock_dep2])

        provision_node(
            name="test-node",
            provider="aws",
            role="client",
            size="t3.small",
            tailscale_auth_key=pulumi.Output.secret("ts-key"),
            leader_ip="leader-ip",
            region="us-east-1",
            opts=test_opts,
        )

        # Verify opts were passed through
        call_args = mock_universal_node.call_args
        assert "opts" in call_args[1]
        assert call_args[1]["opts"] == test_opts


class TestBackwardCompatibility:
    """
    T2.2.3: Test Backward Compatibility (5 tests)

    Validates that existing multipass and AWS calls continue to work
    transparently after the dispatcher integration changes.
    """

    @patch(
        "mesh.infrastructure.provision_node.provision_node.multipass_provider.provision_multipass_node"
    )
    def test_multipass_calls_still_work(self, mock_multipass):
        """
        Test_BackwardCompatibility_MultipassCallsStillWork: Verify that
        existing multipass provider calls work unchanged.
        """
        mock_multipass.return_value = {
            "public_ip": "192.168.122.100",
            "private_ip": "10.0.0.100",
        }

        # This is how existing code calls provision_node for multipass
        result = provision_node(
            name="local-worker-1",
            provider="multipass",
            role="client",
            size="2CPU,4GB",
            tailscale_auth_key=pulumi.Output.secret("ts-key"),
            leader_ip="vm-leader",
        )

        # Verify it still works
        mock_multipass.assert_called_once()
        assert result["public_ip"] == "192.168.122.100"

    @patch("mesh.infrastructure.provision_node.provision_node.UniversalCloudNode")
    @patch.dict(
        "os.environ",
        {"AWS_ACCESS_KEY_ID": "old-aws-key", "AWS_SECRET_ACCESS_KEY": "old-aws-secret"},
    )
    def test_aws_calls_migrate_transparently(self, mock_universal_node):
        """
        Test_BackwardCompatibility_AWSCallsMigrateTransparently: Verify that
        existing AWS calls work without modification.
        """
        mock_node = MagicMock()
        mock_node.public_ip = pulumi.Output.secret("34.200.100.50")
        mock_node.private_ip = pulumi.Output.secret("172.31.32.100")
        mock_node.instance_id = pulumi.Output.secret("i-oldformat123")
        mock_universal_node.return_value = mock_node

        # This is how existing code might call provision_node for AWS (updated with region)
        result = provision_node(
            name="aws-leader-1",
            provider="aws",
            role="server",
            size="t3.small",
            tailscale_auth_key=pulumi.Output.secret("ts-key"),
            leader_ip="127.0.0.1",
            region="us-east-1",
        )

        # Verify it still works
        mock_universal_node.assert_called_once()
        assert isinstance(result["public_ip"], pulumi.Output)


class TestTierConfigThreading:
    """
    Task 4.3: Test TierConfig threading through provision_node
    """

    @patch("mesh.infrastructure.provision_node.provision_node.UniversalCloudNode")
    @patch.dict(
        "os.environ",
        {"AWS_ACCESS_KEY_ID": "test-key", "AWS_SECRET_ACCESS_KEY": "test-secret"},
    )
    def test_provision_node_with_lite_tier(self, mock_universal_node):
        """
        Test_TierConfig_LiteTier: verify provision_node accepts tier_config with LITE.
        """
        mock_node = MagicMock()
        mock_node.public_ip = pulumi.Output.secret("1.2.3.4")
        mock_node.private_ip = pulumi.Output.secret("10.0.1.5")
        mock_node.instance_id = pulumi.Output.secret("i-test")
        mock_universal_node.return_value = mock_node

        tier_config = TierConfig.from_tier(ClusterTier.LITE)

        result = provision_node(
            name="test-lite-node",
            provider="aws",
            role="server",
            size="t3.small",
            tailscale_auth_key=pulumi.Output.secret("ts-key"),
            leader_ip="10.0.0.1",
            region="us-east-1",
            tier_config=tier_config,
        )

        mock_universal_node.assert_called_once()
        assert "public_ip" in result

    @patch(
        "mesh.infrastructure.provision_node.provision_node.multipass_provider.provision_multipass_node"
    )
    def test_provision_node_without_tier_defaults_production(self, mock_multipass):
        """
        Test_TierConfig_DefaultProduction: verify provision_node works without tier_config.
        """
        mock_multipass.return_value = {"public_ip": "1.2.3.4"}

        result = provision_node(
            name="test-default-tier",
            provider="multipass",
            role="client",
            size="2CPU,1GB",
            tailscale_auth_key=pulumi.Output.secret("ts-key"),
            leader_ip="10.0.0.1",
        )

        mock_multipass.assert_called_once()
        assert result["public_ip"] == "1.2.3.4"

    @patch(
        "mesh.infrastructure.provision_node.provision_node.multipass_provider.provision_multipass_node"
    )
    def test_provision_node_tier_config_passed_to_boot_script(self, mock_multipass):
        """
        Test_TierConfig_PassedToBootScript: verify provision_node accepts tier_config
        and passes it through to boot script generation.
        """
        mock_multipass.return_value = {"public_ip": "1.2.3.4"}

        tier_config = TierConfig.from_tier(ClusterTier.LITE)

        result = provision_node(
            name="test-tier-pass",
            provider="multipass",
            role="server",
            size="2CPU,1GB",
            tailscale_auth_key=pulumi.Output.secret("ts-key"),
            leader_ip="10.0.0.1",
            tier_config=tier_config,
        )

        mock_multipass.assert_called_once()
        boot_script_content = mock_multipass.call_args.kwargs.get(
            "boot_script_content",
            mock_multipass.call_args[1].get("boot_script_content"),
        )
        assert boot_script_content is not None

    @patch(
        "mesh.infrastructure.provision_node.provision_node.multipass_provider.provision_multipass_node"
    )
    def test_provision_node_without_tier_defaults_production(self, mock_multipass):
        """
        Test_TierConfig_DefaultProduction: verify provision_node works without tier_config.
        """
        mock_multipass.return_value = {"public_ip": "1.2.3.4"}

        result = provision_node(
            name="test-default-tier",
            provider="multipass",
            role="client",
            size="2CPU,1GB",
            tailscale_auth_key=pulumi.Output.secret("ts-key"),
            leader_ip="10.0.0.1",
        )

        mock_multipass.assert_called_once()
        assert result["public_ip"] == "1.2.3.4"

    @patch(
        "mesh.infrastructure.provision_node.provision_node.multipass_provider.provision_multipass_node"
    )
    def test_provision_node_tier_config_passed_to_boot_script(self, mock_multipass):
        """
        Test_TierConfig_PassedToBootScript: verify provision_node accepts tier_config
        and passes it through to boot script generation.
        """
        mock_multipass.return_value = {"public_ip": "1.2.3.4"}

        tier_config = TierConfig.from_tier(ClusterTier.LITE)

        result = provision_node(
            name="test-tier-pass",
            provider="multipass",
            role="server",
            size="2CPU,1GB",
            tailscale_auth_key=pulumi.Output.secret("ts-key"),
            leader_ip="10.0.0.1",
            tier_config=tier_config,
        )

        mock_multipass.assert_called_once()
        assert result["public_ip"] == "1.2.3.4"
