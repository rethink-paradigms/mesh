"""
Simplified Pulumi Automation API Wrapper

Provides programmatic control over Pulumi operations for CLI integration.
"""

import os
from typing import Dict, Any, Optional, Callable
from pulumi import automation as auto
from pulumi.automation import ProjectSettings, LocalWorkspaceOptions


class CloudClusterAutomation:
    """
    Simplified wrapper for Pulumi Automation API.
    """

    def __init__(self, stack_name: str = "dev", work_dir: Optional[str] = None):
        self.stack_name = stack_name
        self.work_dir = work_dir or os.path.dirname(__file__)

    async def deploy_cluster(
        self,
        config: Dict[str, Any],
        progress_callback: Optional[Callable[[str], None]] = None,
    ) -> Dict[str, Any]:
        """Deploy cluster using automation API."""
        try:
            # Create workspace options
            workspace_opts = LocalWorkspaceOptions(
                work_dir=self.work_dir,
                project_settings=ProjectSettings(name="mesh-cluster", runtime="python"),
            )

            # Create or select stack
            stack = await auto.create_or_select_stack(
                stack_name=self.stack_name,
                project_name="mesh-cluster",
                program=self._pulumi_program,
                opts=workspace_opts,
            )

            # Set configuration values
            for key, value in config.items():
                await stack.set_config(key, auto.ConfigValue(value=value))

            # Deploy with progress tracking
            if progress_callback:
                progress_callback(f"🚀 Starting deployment")

            result = await stack.up(on_output=progress_callback)

            # Extract outputs
            outputs = {key: output.value for key, output in result.outputs.items()}

            return {
                "status": "success",
                "outputs": outputs,
                "summary": {
                    "resource_changes": result.summary.resource_changes,
                    "duration_seconds": result.summary.duration_seconds,
                },
            }

        except Exception as e:
            return {"status": "error", "error": str(e), "outputs": {}}

    async def destroy_cluster(
        self, progress_callback: Optional[Callable[[str], None]] = None
    ) -> Dict[str, Any]:
        """Destroy cluster using automation API."""
        try:
            workspace_opts = LocalWorkspaceOptions(work_dir=self.work_dir)
            stack = await auto.select_stack(
                stack_name=self.stack_name, opts=workspace_opts
            )

            if progress_callback:
                progress_callback(f"🗑️  Destroying {self.stack_name}")

            result = await stack.destroy(on_output=progress_callback)

            return {
                "status": "success",
                "summary": {
                    "resource_changes": result.summary.resource_changes,
                    "duration_seconds": result.summary.duration_seconds,
                },
            }

        except Exception as e:
            return {"status": "error", "error": str(e)}

    def _pulumi_program(self):
        """Pulumi program function that defines the infrastructure."""
        import pulumi
        from src.infrastructure.configure_tailscale import (
            configure as tailscale_feature,
        )
        from src.infrastructure.provision_node.provision_node import provision_node

        # Load configuration from stack config
        provider = pulumi.Config().get("provider") or "aws"
        region = pulumi.Config().get("region") or "us-east-1"

        # Generate Tailscale auth key
        tailscale_auth_key = tailscale_feature.generate_auth_key()

        # Provision leader node
        leader = provision_node(
            name="vm-leader",
            provider=provider,
            role="server",
            size=pulumi.Config().get("leader_size") or "t3.small",
            tailscale_auth_key=tailscale_auth_key,
            leader_ip="",
        )

        # Export outputs
        pulumi.export("leader_public_ip", leader["public_ip"])
        pulumi.export("leader_private_ip", leader["private_ip"])


# Convenience functions for CLI usage
async def deploy_cluster_from_config(
    config: Dict[str, str],
    stack_name: str = "dev",
    progress_callback: Optional[Callable[[str], None]] = None,
) -> Dict[str, Any]:
    """Convenience function to deploy cluster with given configuration."""
    automation = CloudClusterAutomation(stack_name=stack_name)
    return await automation.deploy_cluster(config, progress_callback)


async def destroy_cluster_stack(
    stack_name: str = "dev", progress_callback: Optional[Callable[[str], None]] = None
) -> Dict[str, Any]:
    """Convenience function to destroy cluster stack."""
    automation = CloudClusterAutomation(stack_name=stack_name)
    return await automation.destroy_cluster(progress_callback)
