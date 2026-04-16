"""
Feature: Provision Node
Orchestrates node provisioning across different providers.
"""

import pulumi
from typing import Dict, Any, Optional, List
from dataclasses import dataclass

from mesh.infrastructure.progressive_activation.tier_config import TierConfig

from mesh.infrastructure.provision_node import multipass as multipass_provider

from mesh.infrastructure.boot_consul_nomad.generate_boot_scripts import (
    generate_cloud_init_yaml,
)

from mesh.infrastructure.providers.libcloud_dynamic_provider import UniversalCloudNode
from mesh.infrastructure.providers import is_provider_supported


@dataclass
class GPUConfig:
    """GPU configuration for node provisioning.

    Uses HashiCorp's nomad-device-nvidia plugin for automatic GPU detection.
    The plugin detects available NVIDIA GPUs via NVML, so no gpu_type/gpu_count
    specifications are needed - the plugin handles this automatically.

    Args:
        enable_gpu: Enable GPU support on the node (default: True)
        cuda_version: CUDA runtime version to install (default: "12.1")
        nvidia_driver_version: NVIDIA driver version (default: "535")
    """

    enable_gpu: bool = True
    cuda_version: str = "12.1"
    nvidia_driver_version: str = "535"


@dataclass
class SpotConfig:
    """Spot instance interruption handling configuration.

    Enables graceful handling of AWS spot instance termination warnings.
    When enabled, installs a systemd service that polls EC2 metadata for
    termination notices and triggers Nomad node drain before shutdown.

    Args:
        enable_spot_handling: Enable spot interruption handling (default: False)
        spot_check_interval: Polling interval in seconds (default: 5)
        spot_grace_period: Grace period for workload migration in seconds (default: 90)
    """

    enable_spot_handling: bool = False
    spot_check_interval: int = 5
    spot_grace_period: int = 90


def _provision_via_libcloud(
    name: str,
    provider: str,
    region: str,
    size_id: str,
    role: str,
    tailscale_auth_key: pulumi.Output[str],
    leader_ip: str,
    has_gpu: bool,
    cuda_version: Optional[str],
    driver_version: Optional[str],
    enable_spot_handling: bool,
    spot_check_interval: int,
    spot_grace_period: int,
    depends_on_resources: Optional[List[pulumi.Resource]],
    opts: Optional[pulumi.ResourceOptions],
    cluster_tier: str = "production",
) -> Dict[str, Any]:
    """
    Provision a node via Libcloud dynamic provider.

    This helper function handles provisioning for all non-multipass providers
    through the UniversalCloudNode dynamic resource.

    Args:
        name: Node name
        provider: Provider ID (e.g., "aws", "digitalocean")
        region: Cloud region
        size_id: Exact instance size ID (e.g., "t3.medium", "s-2vcpu-4gb")
        role: Node role ("server" or "client")
        tailscale_auth_key: Tailscale auth key
        leader_ip: Leader IP for boot script
        has_gpu: Whether GPU is enabled
        cuda_version: CUDA version if GPU enabled
        driver_version: NVIDIA driver version if GPU enabled
        enable_spot_handling: Whether spot handling is enabled
        spot_check_interval: Spot check interval
        spot_grace_period: Spot grace period
        depends_on_resources: Resource dependencies
        opts: Pulumi resource options
        cluster_tier: Cluster tier string (default: "production")

    Returns:
        Dict with public_ip, private_ip, instance_id

    Raises:
        ValueError: If provider not supported
    """
    if not is_provider_supported(provider):
        raise ValueError(
            f"Unknown provider: {provider}. "
            f"See https://libcloud.readthedocs.io/en/stable/compute/supported_providers.html"
        )

    boot_script_content_output = tailscale_auth_key.apply(
        lambda ts_key: generate_cloud_init_yaml(
            tailscale_key=ts_key,
            leader_ip=leader_ip,
            role=role,
            has_gpu=has_gpu,
            cuda_version=cuda_version,
            driver_version=driver_version,
            enable_spot_handling=enable_spot_handling,
            spot_check_interval=spot_check_interval,
            spot_grace_period=spot_grace_period,
            validate=False,
            cluster_tier=cluster_tier,
        )
    )

    node = UniversalCloudNode(
        name,
        provider=provider,
        region=region,
        size_id=size_id,
        boot_script=boot_script_content_output,
        opts=opts,
    )

    return {
        "public_ip": node.public_ip,
        "private_ip": node.private_ip,
        "instance_id": node.instance_id,
    }


def provision_node(
    name: str,
    provider: str,
    role: str,
    size: str,
    tailscale_auth_key: pulumi.Output[str],
    leader_ip: str,
    region: Optional[str] = None,
    gpu_config: Optional[GPUConfig] = None,
    spot_config: Optional[SpotConfig] = None,
    tier_config: Optional[TierConfig] = None,
    depends_on_resources: Optional[List[pulumi.Resource]] = None,
    opts: Optional[pulumi.ResourceOptions] = None,
) -> Dict[str, Any]:
    """
    Provisions a generic compute node on a specified provider.

    This function acts as an abstraction layer, dispatching to provider-specific
    implementations based on the 'provider' argument. It also handles the generation
    of the appropriate boot script format for the chosen provider.

    Supported providers:
        - "multipass": Local virtual machines (via Multipass CLI)
        - "aws": Amazon Web Services (via Libcloud)
        - "digitalocean": DigitalOcean (via Libcloud)
        - "hetzner": Hetzner Cloud (via Libcloud)
        - "gcp": Google Cloud Platform (via Libcloud)
        - "azure": Microsoft Azure (via Libcloud)
        - And 50+ more providers supported by Libcloud

    Args:
        name (str): The unique name/identifier for the node.
        provider (str): The compute provider to use.
        role (str): The role of the node ("server" or "client").
        size (str): The exact instance size ID (e.g., "t3.small", "s-2vcpu-4gb").
        tailscale_auth_key (pulumi.Output[str]): The ephemeral auth key for joining the Tailscale mesh.
        leader_ip (str): IP or Hostname of the Leader node for joining the cluster.
        region (str, optional): Cloud region. Required for cloud providers.
        gpu_config (GPUConfig, optional): GPU configuration for GPU worker nodes.
        spot_config (SpotConfig, optional): Spot instance interruption handling configuration.
        tier_config (TierConfig, optional): Progressive activation tier configuration.
        depends_on_resources (List[pulumi.Resource], optional): Resources this node depends on.
        opts (pulumi.ResourceOptions, optional): Optional Pulumi resource options.

    Returns:
        Dict[str, Any]: A dictionary containing provider-specific outputs like public_ip,
                        private_ip, and instance_id.

    Raises:
        ValueError: If provider is unknown, credentials are missing, or region is required.
        NotImplementedError: If provider is not yet implemented.
    """

    has_gpu = gpu_config is not None and gpu_config.enable_gpu
    cuda_version = gpu_config.cuda_version if gpu_config else None
    driver_version = gpu_config.nvidia_driver_version if gpu_config else None

    enable_spot_handling = spot_config is not None and spot_config.enable_spot_handling
    spot_check_interval = spot_config.spot_check_interval if spot_config else 5
    spot_grace_period = spot_config.spot_grace_period if spot_config else 90

    cluster_tier = tier_config.tier.value if tier_config else "production"

    if provider == "multipass":
        boot_script_content_output = tailscale_auth_key.apply(
            lambda ts_key: generate_cloud_init_yaml(
                tailscale_key=ts_key,
                leader_ip=leader_ip,
                role=role,
                has_gpu=has_gpu,
                cuda_version=cuda_version,
                driver_version=driver_version,
                enable_spot_handling=enable_spot_handling,
                spot_check_interval=spot_check_interval,
                spot_grace_period=spot_grace_period,
                cluster_tier=cluster_tier,
            )
        )

        return multipass_provider.provision_multipass_node(
            name=name,
            instance_size=size,
            role=role,
            boot_script_content=boot_script_content_output,
            opts=None,
        )

    elif provider == "bare-metal":
        raise NotImplementedError(
            f"Bare Metal provider '{provider}' is not yet implemented."
        )

    else:
        if not region:
            raise ValueError(
                f"region is required for cloud provider '{provider}'. "
                f"Use list_regions('{provider}') to see available regions."
            )

        return _provision_via_libcloud(
            name=name,
            provider=provider,
            region=region,
            size_id=size,
            role=role,
            tailscale_auth_key=tailscale_auth_key,
            leader_ip=leader_ip,
            has_gpu=has_gpu,
            cuda_version=cuda_version,
            driver_version=driver_version,
            enable_spot_handling=enable_spot_handling,
            spot_check_interval=spot_check_interval,
            spot_grace_period=spot_grace_period,
            cluster_tier=cluster_tier,
            depends_on_resources=depends_on_resources,
            opts=opts,
        )
