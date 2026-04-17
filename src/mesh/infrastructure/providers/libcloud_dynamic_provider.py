"""
Pulumi Dynamic Resource Provider for Multi-Cloud Node Provisioning

This module implements a universal cloud node provider using Apache Libcloud and
Pulumi's Dynamic Resource API. It enables provisioning nodes across multiple cloud
providers (AWS, DigitalOcean, Hetzner, GCP, Azure, and 50+ more) through a unified interface.

Key Features:
- Dynamic provisioning through Apache Libcloud drivers
- Automatic credential resolution from environment variables
- Exact provider values (no size tier abstractions)
- Ubuntu image auto-discovery or explicit image selection
- Boot script injection via cloud-init/userdata
- Real-time validation against provider APIs

Architecture:
    UniversalCloudNodeProvider (pulumi.dynamic.ResourceProvider)
        ├── create() - Provisions new node instances
        ├── delete() - Destroys existing node instances
        ├── read()  - Fetches current node state
        └── _get_driver() - Lazy initializes Libcloud drivers

    UniversalCloudNode (pulumi.dynamic.Resource)
        ├── public_ip: pulumi.Output[str]
        ├── private_ip: pulumi.Output[str]
        ├── instance_id: pulumi.Output[str]
        └── status: pulumi.Output[str]

 Example:
    # Import the provider
    from mesh.infrastructure.providers.libcloud_dynamic_provider import UniversalCloudNode

    # Provision a node on DigitalOcean
    node = UniversalCloudNode(
        "my-worker",
        cloud_provider="digitalocean",
        region="nyc3",
        size_id="s-2vcpu-4gb",
        boot_script=boot_script_content,
        opts=pulumi.ResourceOptions(depends_on=[network_setup])
    )

    # Access outputs
    pulumi.Output.all(node.public_ip, node.private_ip).apply(
        lambda args: print(f"Node public IP: {args[0]}, private IP: {args[1]}")
    )

To see available options for a provider:
    from mesh.infrastructure.providers.discovery import list_sizes, list_regions

    sizes = list_sizes("digitalocean")
    for s in sizes:
        print(f"{s.id}: {s.name} - {s.ram}MB RAM")

    regions = list_regions("digitalocean")
    for r in regions:
        print(f"{r.id}: {r.name}")
"""

import pulumi
import time
import logging
from pulumi.dynamic import ResourceProvider, Resource, CreateResult, ReadResult
from typing import Dict, Any, Optional
from dataclasses import dataclass

logger = logging.getLogger(__name__)

# Apache Libcloud imports
from libcloud.compute.base import NodeDriver

# Import provider utilities
from mesh.infrastructure.providers import get_driver, is_provider_supported
from mesh.infrastructure.providers.discovery import (
    get_size,
    get_region,
    is_region_available,
    get_image,
    find_ubuntu_image,
)


@dataclass
class UniversalCloudNodeInputs:
    """
    Input properties for UniversalCloudNode resource.

    Attributes:
        cloud_provider: Cloud provider identifier (e.g., "aws", "digitalocean", "hetzner")
        region: Target region/zone for node provisioning (e.g., "us-east-1", "nyc3")
        size_id: Exact instance size ID from provider (e.g., "t3.medium", "s-2vcpu-4gb")
        image_id: Optional exact image ID (auto-discovers Ubuntu 22.04 if not specified)
        boot_script: Cloud-init/userdata script to execute on boot
        credentials: Optional dictionary of credentials (overrides env vars)
        node_name: Optional explicit name for the node (defaults to resource name)
    """

    cloud_provider: str
    region: str
    size_id: str
    boot_script: str
    image_id: Optional[str] = None
    credentials: Optional[Dict[str, str]] = None
    node_name: Optional[str] = None


class UniversalCloudNodeProvider(ResourceProvider):
    """
    Pulumi Dynamic Resource Provider for multi-cloud node provisioning.

    This provider implements the CRUD operations for cloud nodes using Apache
    Libcloud drivers. It validates all inputs against provider APIs in real-time.

    Lifecycle:
        1. create() - Provisions new node with boot script
        2. read()  - Fetches current state from provider API
        3. delete() - Destroys node and releases resources
        4. update() - Not supported (requires node replacement)

    Error Handling:
        - Missing credentials: Raises ValueError with clear message
        - Invalid provider: Raises ValueError
        - Invalid region/size/image: Raises ValueError with available options
        - Provisioning failure: Propagates Libcloud exception
    """

    def __init__(self):
        """Initialize the provider with driver cache."""
        super().__init__()
        self._drivers: Dict[tuple, NodeDriver] = {}

    def create(self, inputs: Dict[str, Any]) -> CreateResult:
        """
        Provision a new cloud node.

        Steps:
            1. Validate provider exists
            2. Validate region exists in provider
            3. Validate size_id exists in provider
            4. Resolve credentials from inputs or environment
            5. Get or create Libcloud driver
            6. Find or validate image
            7. Create node with boot script
            8. Return node ID and outputs

        Args:
            inputs: Dictionary containing provider, region, size_id, boot_script,
                    image_id (optional), credentials (optional), node_name (optional)

        Returns:
            CreateResult with node ID and output dictionary containing:
                - public_ip: Public IP address (or None if no public IP)
                - private_ip: Private IP address
                - instance_id: Provider-specific instance ID
                - status: Node status (e.g., "RUNNING")
                - provider: Cloud provider name
                - region: Provider region/zone
                - size_id: Size ID used

        Raises:
            ValueError: If provider invalid, credentials missing, or inputs invalid
            Exception: If Libcloud provisioning fails
        """
        # Support both new cloud_provider key and legacy provider key for backward compatibility
        provider_id = inputs.get("cloud_provider") or inputs.get("provider")
        region = inputs.get("region")
        size_id = inputs.get("size_id")
        image_id = inputs.get("image_id")
        boot_script = inputs.get("boot_script")
        credentials = inputs.get("credentials") or {}
        node_name = (
            inputs.get("node_name")
            or f"mesh-node-{pulumi.get_stack()}-{pulumi.get_project()}"
        )

        # Validate required inputs
        if not provider_id:
            raise ValueError("provider is required")
        if not region:
            raise ValueError("region is required")
        if not size_id:
            raise ValueError("size_id is required")
        if not boot_script:
            raise ValueError("boot_script is required")

        # Validate provider
        if not is_provider_supported(provider_id):
            raise ValueError(
                f"Unknown provider '{provider_id}'. "
                f"Use one of the supported providers from Libcloud. "
                f"See: https://libcloud.readthedocs.io/en/stable/compute/supported_providers.html"
            )

        # Validate region exists and resolve to NodeLocation object
        location = get_region(provider_id, region)
        if not location:
            raise ValueError(
                f"Invalid region '{region}' for provider {provider_id}. "
                f"Query list_regions('{provider_id}') to see available regions."
            )

        # Validate size_id exists
        size = get_size(provider_id, size_id, region=region)
        if not size:
            raise ValueError(
                f"Invalid size_id '{size_id}' for provider {provider_id}. "
                f"Query list_sizes('{provider_id}', '{region}') to see available sizes."
            )

        # Get image (auto-discover Ubuntu if not specified)
        if image_id:
            image = get_image(provider_id, image_id)
            if not image:
                raise ValueError(
                    f"Invalid image_id '{image_id}' for provider {provider_id}. "
                    f"Query list_images('{provider_id}') to see available images."
                )
        else:
            image = find_ubuntu_image(provider_id, version="22.04")
            if not image:
                raise ValueError(
                    f"Could not find Ubuntu 22.04 image for provider {provider_id}. "
                    f"Specify image_id explicitly or check provider image catalog."
                )

        # Get Libcloud driver
        driver = self._get_driver(provider_id, region, credentials)

        # Create node
        try:
            node = driver.create_node(
                name=node_name,
                size=size,
                image=image,
                location=location,
                ex_user_data=boot_script,
            )
        except Exception as e:
            raise RuntimeError(f"Failed to create node on {provider_id}: {str(e)}")

        # Providers like DigitalOcean assign IPs asynchronously after create returns.
        public_ip = node.public_ips[0] if node.public_ips else None
        private_ip = node.private_ips[0] if node.private_ips else None

        if public_ip is None:
            max_wait = 120
            poll_interval = 5
            elapsed = 0
            logger.info(
                "Node %s created (id=%s) but no public IP yet — polling...",
                node_name,
                node.id,
            )
            while elapsed < max_wait:
                time.sleep(poll_interval)
                elapsed += poll_interval
                try:
                    refreshed = driver.ex_get_node_details(node.id)
                    if refreshed and refreshed.public_ips:
                        public_ip = refreshed.public_ips[0]
                        private_ip = (
                            refreshed.private_ips[0]
                            if refreshed.private_ips
                            else private_ip
                        )
                        logger.info(
                            "Node %s got public IP %s after %ds",
                            node_name,
                            public_ip,
                            elapsed,
                        )
                        break
                except Exception:
                    break

        return CreateResult(
            id_=node.id,
            outs={
                "public_ip": public_ip,
                "private_ip": private_ip,
                "instance_id": node.id,
                "status": node.state,
                "provider": provider_id,
                "region": region,
                "size_id": size_id,
            },
        )

    def delete(self, id: str, inputs: Dict[str, Any]) -> None:
        """
        Destroy a cloud node.

        Args:
            id: The node ID (instance_id) to destroy
            inputs: Dictionary containing provider, region, credentials

        Raises:
            ValueError: If provider invalid or credentials missing
            Exception: If Libcloud deletion fails
        """
        # Support both new cloud_provider key and legacy provider key for backward compatibility
        provider_id = inputs.get("cloud_provider") or inputs.get("provider")
        region = inputs.get("region")
        credentials = inputs.get("credentials") or {}

        # Validate provider
        if not is_provider_supported(provider_id):
            raise ValueError(f"Unknown provider '{provider_id}'")

        # Get Libcloud driver
        driver = self._get_driver(provider_id, region, credentials)

        # Get node details and destroy
        try:
            # Try to get node by ID
            node = None
            try:
                node = driver.ex_get_node_details(id)
            except Exception:
                # Node may not exist or method not supported
                # Try listing all nodes and finding by ID
                all_nodes = driver.list_nodes()
                for n in all_nodes:
                    if n.id == id:
                        node = n
                        break

            if node:
                driver.destroy_node(node)
            else:
                # Node doesn't exist, consider it deleted
                pass

        except Exception as e:
            raise RuntimeError(f"Failed to delete node {id} on {provider_id}: {str(e)}")

    def read(self, id: str, inputs: Dict[str, Any]) -> Optional[ReadResult]:
        """
        Fetch the current state of a cloud node.

        Args:
            id: The node ID (instance_id) to read
            inputs: Dictionary containing provider, region, credentials

        Returns:
            ReadResult with current node state, or None if node doesn't exist

        Raises:
            ValueError: If provider invalid or credentials missing
            Exception: If Libcloud read fails
        """
        # Support both new cloud_provider key and legacy provider key for backward compatibility
        provider_id = inputs.get("cloud_provider") or inputs.get("provider")
        region = inputs.get("region")
        credentials = inputs.get("credentials") or {}

        # Validate provider
        if not is_provider_supported(provider_id):
            raise ValueError(f"Unknown provider '{provider_id}'")

        # Get Libcloud driver
        driver = self._get_driver(provider_id, region, credentials)

        # Get node details
        try:
            node = None
            try:
                node = driver.ex_get_node_details(id)
            except Exception:
                # Node may not exist or method not supported
                all_nodes = driver.list_nodes()
                for n in all_nodes:
                    if n.id == id:
                        node = n
                        break

            if not node:
                # Node doesn't exist
                return None

            # Extract current state
            public_ip = node.public_ips[0] if node.public_ips else None
            private_ip = node.private_ips[0] if node.private_ips else None

            return ReadResult(
                id_=node.id,
                outs={
                    "public_ip": public_ip,
                    "private_ip": private_ip,
                    "instance_id": node.id,
                    "status": node.state,
                    "provider": provider_id,
                    "region": region,
                    "size_id": inputs.get("size_id", "unknown"),
                },
            )

        except Exception as e:
            raise RuntimeError(f"Failed to read node {id} on {provider_id}: {str(e)}")

    def _get_driver(
        self, provider_id: str, region: str, credentials: Dict[str, str]
    ) -> NodeDriver:
        """
        Get or create a cached Libcloud driver.

        Args:
            provider_id: The provider identifier
            region: The provider region
            credentials: Credential dictionary

        Returns:
            Initialized Libcloud NodeDriver instance

        Raises:
            ValueError: If credentials are missing
            Exception: If driver initialization fails
        """
        # Create cache key
        cache_key = (provider_id, region)

        # Return cached driver if available
        if cache_key in self._drivers:
            return self._drivers[cache_key]

        # Get new driver (credentials will be auto-resolved if empty)
        try:
            creds = credentials if credentials else None
            driver = get_driver(provider_id, credentials=creds, region=region)
            self._drivers[cache_key] = driver
            return driver
        except ValueError as e:
            raise ValueError(f"Failed to initialize driver for {provider_id}: {str(e)}")


class UniversalCloudNode(Resource):
    """
    Pulumi Dynamic Resource for multi-cloud node provisioning.

    This resource represents a cloud node that can be provisioned across
    multiple cloud providers using Apache Libcloud.

    Properties:
        cloud_provider: Cloud provider identifier
        region: Provider region/zone
        size_id: Exact instance size ID
        image_id: Optional image ID (auto-discovers Ubuntu if not specified)
        boot_script: Cloud-init/userdata script
        credentials: Optional credential overrides
        node_name: Optional node name

    Outputs:
        public_ip: pulumi.Output[str] - Public IP address
        private_ip: pulumi.Output[str] - Private IP address
        instance_id: pulumi.Output[str] - Provider instance ID
        status: pulumi.Output[str] - Node status
    """

    public_ip: pulumi.Output[str]
    private_ip: pulumi.Output[str]
    instance_id: pulumi.Output[str]
    status: pulumi.Output[str]

    def __init__(
        self,
        name: str,
        cloud_provider: str = None,
        region: str = None,
        size_id: str = None,
        boot_script: str = None,
        image_id: Optional[str] = None,
        credentials: Optional[Dict[str, str]] = None,
        node_name: Optional[str] = None,
        opts: Optional["pulumi.ResourceOptions"] = None,
    ):
        _PROVIDER_INSTANCE = UniversalCloudNodeProvider()
        props: Dict[str, Any] = {
            "cloud_provider": cloud_provider,
            "region": region,
            "size_id": size_id,
            "boot_script": boot_script,
            "public_ip": None,
            "private_ip": None,
            "instance_id": None,
            "status": None,
        }
        if image_id is not None:
            props["image_id"] = image_id
        if credentials is not None:
            props["credentials"] = credentials
        if node_name is not None:
            props["node_name"] = node_name
        super().__init__(_PROVIDER_INSTANCE, name, props, opts)


def provision_cloud_node(
    name: str,
    provider: str,
    region: str,
    size_id: str,
    boot_script: str,
    image_id: str = None,
    credentials: Dict[str, str] = None,
    opts=None,
) -> Dict[str, Any]:
    """
    Convenience function to provision a cloud node.

    This is a simpler wrapper around UniversalCloudNode for common use cases.

    Args:
        name: Resource name (used for node naming if node_name not specified)
        provider: Cloud provider identifier (e.g., "aws", "digitalocean")
        region: Provider region (e.g., "us-east-1", "nyc3")
        size_id: Exact size ID (e.g., "t3.medium", "s-2vcpu-4gb")
        boot_script: Cloud-init/userdata script content
        image_id: Optional image ID (auto-discovers Ubuntu 22.04 if not specified)
        credentials: Optional credential dictionary
        opts: Optional Pulumi resource options

    Returns:
        Dictionary with keys:
        - public_ip: pulumi.Output[str]
        - private_ip: pulumi.Output[str]
        - instance_id: pulumi.Output[str]

    Example:
        >>> from mesh.infrastructure.providers.libcloud_dynamic_provider import provision_cloud_node
        >>>
        >>> node = provision_cloud_node(
        ...     name="my-worker",
        ...     provider="digitalocean",
        ...     region="nyc3",
        ...     size_id="s-2vcpu-4gb",
        ...     boot_script=script_content
        ... )
        >>>
        >>> pulumi.Output.all(node["public_ip"]).apply(
        ...     lambda ip: print(f"Node IP: {ip}")
        ... )
    """
    # Validate inputs
    if not is_provider_supported(provider):
        raise ValueError(f"Unknown provider '{provider}'")

    # Create the resource
    node = UniversalCloudNode(
        name,
        cloud_provider=provider,
        region=region,
        size_id=size_id,
        image_id=image_id,
        boot_script=boot_script,
        credentials=credentials,
        node_name=name,
        opts=opts,
    )

    # Return outputs as dictionary
    return {
        "public_ip": node.public_ip,
        "private_ip": node.private_ip,
        "instance_id": node.instance_id,
    }
