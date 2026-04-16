"""
Provider Discovery Module

This module provides real-time discovery of cloud provider capabilities including
available sizes, regions, and images. All data is queried directly from the provider
via Libcloud, ensuring it's always up-to-date.

Key Features:
    - List all available sizes with RAM/CPU details
    - List all available regions/locations
    - List all available images
    - Auto-discover Ubuntu images
    - Validate provider-specific IDs (size_id, region, image_id)

Usage:
    from mesh.infrastructure.providers.discovery import list_sizes, list_regions, list_images

    # Query provider for available options
    sizes = list_sizes("aws", "us-east-1")
    for size in sizes:
        print(f"{size.id}: {size.ram}MB RAM, {getattr(size, 'vcpu', '?')} vCPU")

    regions = list_regions("digitalocean")
    for region in regions:
        print(f"{region.id}: {region.name}")

    images = list_images("aws")
    ubuntu = find_ubuntu_image("aws", version="22.04")
"""

from typing import List, Optional, Dict, Any
from libcloud.compute.base import NodeSize, NodeImage, NodeLocation

from mesh.infrastructure.providers import get_driver, is_provider_supported


# =============================================================================
# Size Discovery
# =============================================================================

def list_sizes(
    provider_id: str,
    region: str = None,
    credentials: Dict[str, str] = None
) -> List[NodeSize]:
    """
    List all available instance sizes for a provider.

    This queries the provider in real-time to get all available instance types
    with their RAM, CPU, disk, and pricing information.

    Args:
        provider_id: The provider identifier (e.g., "aws", "digitalocean")
        region: Optional region to filter sizes (some providers have region-specific sizes)
        credentials: Optional credential dict (auto-resolved from env if None)

    Returns:
        List of NodeSize objects with id, name, ram, cpu, disk, and price attributes

    Raises:
        ValueError: If provider is not supported or credentials are missing

    Examples:
        >>> sizes = list_sizes("aws", "us-east-1")
        >>> for s in sizes[:5]:  # Show first 5
        ...     print(f"{s.id}: {s.ram}MB RAM, ${s.price}/hr")

        >>> sizes = list_sizes("digitalocean")
        >>> small = [s for s in sizes if "2gb" in s.id][0]
        >>> print(f"{small.id}: {small.name} - {small.ram}MB RAM")
    """
    if not is_provider_supported(provider_id):
        raise ValueError(f"Provider '{provider_id}' is not supported")

    driver = get_driver(provider_id, credentials=credentials, region=region)

    try:
        all_sizes = driver.list_sizes()

        # Some providers support region-specific size filtering
        if region and hasattr(driver, 'list_sizes_in_location'):
            try:
                all_sizes = driver.list_sizes_in_location(region)
            except Exception:
                # If region-specific filtering fails, return all sizes
                pass

        return all_sizes

    except Exception as e:
        raise RuntimeError(
            f"Failed to list sizes for {provider_id}: {str(e)}"
        )


def get_size(
    provider_id: str,
    size_id: str,
    region: str = None,
    credentials: Dict[str, str] = None
) -> Optional[NodeSize]:
    """
    Get a specific size by ID from a provider.

    This validates that the specified size_id exists and returns its details.

    Args:
        provider_id: The provider identifier
        size_id: The exact size ID (e.g., "t3.medium", "s-2vcpu-4gb")
        region: Optional region for providers with region-specific sizes
        credentials: Optional credential dict

    Returns:
        NodeSize object if found, None otherwise

    Raises:
        ValueError: If provider is not supported or credentials are missing

    Examples:
        >>> size = get_size("aws", "t3.medium")
        >>> print(f"{size.name}: {size.ram}MB RAM, {size.vcpu} vCPU")

        >>> size = get_size("digitalocean", "s-2vcpu-4gb")
        >>> print(f"${size.price * 730:.2f}/month")
    """
    all_sizes = list_sizes(provider_id, region=region, credentials=credentials)

    for size in all_sizes:
        if size.id == size_id:
            return size

    return None


def find_size_by_specs(
    provider_id: str,
    min_ram_mb: int,
    min_cpu: int = 1,
    max_cost: float = None,
    region: str = None,
    credentials: Dict[str, str] = None
) -> Optional[NodeSize]:
    """
    Find the smallest size that meets minimum RAM and CPU requirements.

    This is useful for finding a cost-effective size for a given workload.

    Args:
        provider_id: The provider identifier
        min_ram_mb: Minimum RAM in megabytes
        min_cpu: Minimum CPU cores
        max_cost: Optional maximum hourly price in USD
        region: Optional region for providers with region-specific pricing
        credentials: Optional credential dict

    Returns:
        NodeSize object that meets requirements, or None if none found

    Examples:
        >>> # Find smallest size with at least 4GB RAM and 2 CPUs
        >>> size = find_size_by_specs("aws", min_ram_mb=4096, min_cpu=2)
        >>> print(f"Selected: {size.id} ({size.ram}MB RAM)")

        >>> # Find cheapest size under $50/month
        >>> size = find_size_by_specs("digitalocean", min_ram_mb=2048, max_cost=0.07)
        >>> print(f"Selected: {size.id}")
    """
    all_sizes = list_sizes(provider_id, region=region, credentials=credentials)

    # Filter by requirements
    candidates = [
        s for s in all_sizes
        if s.ram >= min_ram_mb
        and getattr(s, 'vcpu', 1) >= min_cpu
    ]

    # Filter by cost if specified
    if max_cost is not None:
        candidates = [s for s in candidates if s.price and s.price <= max_cost]

    if not candidates:
        return None

    # Sort by RAM (ascending) to get smallest that meets requirements
    candidates.sort(key=lambda s: s.ram)

    return candidates[0]


# =============================================================================
# Region Discovery
# =============================================================================

def list_regions(
    provider_id: str,
    credentials: Dict[str, str] = None
) -> List[NodeLocation]:
    """
    List all available regions/locations for a provider.

    This queries the provider in real-time to get all available regions
    with their geographic and availability zone information.

    Args:
        provider_id: The provider identifier (e.g., "aws", "digitalocean")
        credentials: Optional credential dict (auto-resolved from env if None)

    Returns:
        List of NodeLocation objects with id, name, country, and availability zone info

    Raises:
        ValueError: If provider is not supported or credentials are missing

    Examples:
        >>> regions = list_regions("aws")
        >>> for r in regions[:5]:
        ...     print(f"{r.id}: {r.country} ({r.name})")

        >>> regions = list_regions("digitalocean")
        >>> for r in regions:
        ...     print(f"{r.id}: {r.name}")
    """
    if not is_provider_supported(provider_id):
        raise ValueError(f"Provider '{provider_id}' is not supported")

    driver = get_driver(provider_id, credentials=credentials)

    try:
        return driver.list_locations()

    except Exception as e:
        raise RuntimeError(
            f"Failed to list regions for {provider_id}: {str(e)}"
        )


def get_region(
    provider_id: str,
    region_id: str,
    credentials: Dict[str, str] = None
) -> Optional[NodeLocation]:
    """
    Get a specific region by ID from a provider.

    This validates that the specified region_id exists and returns its details.

    Args:
        provider_id: The provider identifier
        region_id: The exact region ID (e.g., "us-east-1", "nyc3")
        credentials: Optional credential dict

    Returns:
        NodeLocation object if found, None otherwise

    Examples:
        >>> region = get_region("aws", "us-east-1")
        >>> print(f"{region.name}: {region.country}")

        >>> region = get_region("digitalocean", "nyc3")
        >>> print(f"{region.name}")
    """
    all_regions = list_regions(provider_id, credentials=credentials)

    for region in all_regions:
        if region.id == region_id:
            return region

    return None


def is_region_available(
    provider_id: str,
    region_id: str,
    credentials: Dict[str, str] = None
) -> bool:
    """
    Check if a region is available for a provider.

    Args:
        provider_id: The provider identifier
        region_id: The region ID to check
        credentials: Optional credential dict

    Returns:
        True if the region exists and is available, False otherwise
    """
    return get_region(provider_id, region_id, credentials=credentials) is not None


# =============================================================================
# Image Discovery
# =============================================================================

def list_images(
    provider_id: str,
    credentials: Dict[str, str] = None
) -> List[NodeImage]:
    """
    List all available images for a provider.

    This queries the provider in real-time to get all available images.
    Note: Some providers have hundreds or thousands of images.

    Args:
        provider_id: The provider identifier
        credentials: Optional credential dict (auto-resolved from env if None)

    Returns:
        List of NodeImage objects with id, name, and other metadata

    Raises:
        ValueError: If provider is not supported or credentials are missing

    Examples:
        >>> images = list_images("aws")
        >>> ubuntu_images = [i for i in images if "ubuntu" in i.name.lower()]
        >>> for img in ubuntu_images[:5]:
        ...     print(f"{img.id}: {img.name}")
    """
    if not is_provider_supported(provider_id):
        raise ValueError(f"Provider '{provider_id}' is not supported")

    driver = get_driver(provider_id, credentials=credentials)

    try:
        return driver.list_images()

    except Exception as e:
        raise RuntimeError(
            f"Failed to list images for {provider_id}: {str(e)}"
        )


def find_ubuntu_image(
    provider_id: str,
    version: str = "22.04",
    arch: str = "x86_64",
    credentials: Dict[str, str] = None
) -> Optional[NodeImage]:
    """
    Find the latest Ubuntu image for a provider.

    This searches for Ubuntu images matching the specified version and architecture.
    It tries to find the most recent image by date.

    Args:
        provider_id: The provider identifier
        version: Ubuntu version (e.g., "22.04", "20.04", "18.04")
        arch: Architecture (e.g., "x86_64", "arm64", "amd64")
        credentials: Optional credential dict

    Returns:
        NodeImage object if found, None otherwise

    Examples:
        >>> img = find_ubuntu_image("aws", version="22.04")
        >>> print(f"Ubuntu 22.04: {img.id}")

        >>> img = find_ubuntu_image("digitalocean", version="20.04")
        >>> print(f"Ubuntu 20.04: {img.name}")
    """
    all_images = list_images(provider_id, credentials=credentials)

    # Filter for Ubuntu images matching version and architecture
    version_aliases = {
        "22.04": ["22.04", "jammy", "ubuntu-jammy"],
        "20.04": ["20.04", "focal", "ubuntu-focal"],
        "18.04": ["18.04", "bionic", "ubuntu-bionic"],
    }

    search_terms = version_aliases.get(version, [version])

    # Add architecture patterns
    arch_patterns = {
        "x86_64": ["amd64", "x86_64", "x64"],
        "arm64": ["arm64", "aarch64"],
    }

    arch_terms = arch_patterns.get(arch, [arch])

    # Filter images
    candidates = []
    for img in all_images:
        name_lower = img.name.lower()

        # Check if it's Ubuntu
        if "ubuntu" not in name_lower:
            continue

        # Check version
        if not any(term in name_lower for term in search_terms):
            continue

        # Check architecture
        if any(term in name_lower for term in arch_terms):
            candidates.append(img)

    if not candidates:
        return None

    # Try to find the most recent by looking for date patterns in the name
    # Images often have dates like "20240101" in their names
    import re

    def extract_date(img):
        match = re.search(r'(\d{4})[-/]?(\d{2})[-/]?(\d{2})', img.name)
        if match:
            year, month, day = match.groups()
            return int(year), int(month), int(day)
        return (0, 0, 0)

    candidates.sort(key=extract_date, reverse=True)

    return candidates[0]


def get_image(
    provider_id: str,
    image_id: str,
    credentials: Dict[str, str] = None
) -> Optional[NodeImage]:
    """
    Get a specific image by ID from a provider.

    This validates that the specified image_id exists and returns its details.

    Args:
        provider_id: The provider identifier
        image_id: The exact image ID (e.g., "ami-0abcdef123456")
        credentials: Optional credential dict

    Returns:
        NodeImage object if found, None otherwise

    Examples:
        >>> img = get_image("aws", "ami-0c02fb55956c7d3b6")
        >>> print(f"{img.name}")
    """
    all_images = list_images(provider_id, credentials=credentials)

    for img in all_images:
        if img.id == image_id:
            return img

    return None


# =============================================================================
# Summary/Overview
# =============================================================================

def get_provider_summary(
    provider_id: str,
    region: str = None,
    credentials: Dict[str, str] = None
) -> Dict[str, Any]:
    """
    Get a summary of a provider's capabilities.

    This provides a quick overview of what's available from a provider.

    Args:
        provider_id: The provider identifier
        region: Optional region to filter results
        credentials: Optional credential dict

    Returns:
        Dictionary with provider information including:
        - provider: Provider ID
        - regions: List of region IDs
        - region_count: Number of regions
        - size_count: Number of available sizes
        - image_count: Number of available images
        - has_ubuntu: Whether Ubuntu images are available

    Examples:
        >>> summary = get_provider_summary("digitalocean")
        >>> print(f"{summary['region_count']} regions")
        >>> print(f"{summary['size_count']} sizes available")
    """
    regions = list_regions(provider_id, credentials=credentials)
    sizes = list_sizes(provider_id, region=region, credentials=credentials)
    images = list_images(provider_id, credentials=credentials)

    ubuntu_img = find_ubuntu_image(provider_id, credentials=credentials)

    return {
        "provider": provider_id,
        "regions": [r.id for r in regions],
        "region_count": len(regions),
        "size_count": len(sizes),
        "image_count": len(images),
        "has_ubuntu": ubuntu_img is not None,
        "ubuntu_image": ubuntu_img.id if ubuntu_img else None,
    }
