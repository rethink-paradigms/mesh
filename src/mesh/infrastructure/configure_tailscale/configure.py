"""
Feature: Configure Tailscale
Implementation details for generating Tailscale auth keys.
"""

import pulumi_tailscale as tailscale
from typing import List, Optional


def create_auth_key(
    key_name: str,
    ephemeral: bool = True,
    reusable: bool = True,
    tags: Optional[List[str]] = None,
) -> tailscale.TailnetKey:
    """
    Generates a Tailscale Authentication Key for joining nodes to the mesh.

    This function wraps the Pulumi Tailscale provider to create a standardized
    auth key. It defaults to 'tag:mesh' to ensure all nodes are auto-grouped
    into the mesh ACLs.

    Args:
        key_name (str): The logical name for the key resource.
        ephemeral (bool): If True, nodes are automatically removed from the
                          Tailnet when they go offline. Default: True.
        reusable (bool): If True, the key can be used to authenticate multiple
                         nodes. Default: True.
        tags (List[str], optional): ACL tags to apply. Defaults to ["tag:mesh"].

    Returns:
        tailscale.TailnetKey: The created Tailscale key resource. The 'key'
                              property of this object is a Secret.
    """

    # Build key arguments — only include tags if explicitly provided.
    # Tailscale requires tags to be pre-defined in the tailnet ACL policy.
    # Omitting tags lets nodes join untagged; users can add ACL tags later.
    key_args = dict(
        ephemeral=ephemeral,
        reusable=reusable,
        description=f"Auto-generated key for {key_name}",
    )
    if tags:
        key_args["tags"] = tags

    # Create the key resource using Pulumi
    auth_key = tailscale.TailnetKey(
        key_name,
        **key_args,
    )

    return auth_key
