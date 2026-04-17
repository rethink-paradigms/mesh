"""
Simplified Pulumi Automation API Wrapper

Provides programmatic control over Pulumi operations for CLI integration.
"""

import os
from typing import Dict, Any, Optional, Callable
from pulumi import automation as auto
from pulumi.automation import ProjectSettings, LocalWorkspaceOptions

from mesh.infrastructure.config.env import EnvVars, get_env


class CloudClusterAutomation:
    def __init__(self, stack_name: str = "dev", work_dir: Optional[str] = None):
        self.stack_name = stack_name
        self.work_dir = work_dir or os.path.dirname(__file__)

    def deploy_cluster(
        self,
        config: Dict[str, Any],
        progress_callback: Optional[Callable[[str], None]] = None,
    ) -> Dict[str, Any]:
        try:
            src_dir = os.path.abspath(
                os.path.join(os.path.dirname(__file__), "..", "..", "..")
            )
            workspace_opts = LocalWorkspaceOptions(
                work_dir=self.work_dir,
                project_settings=ProjectSettings(name="mesh-cluster", runtime="python"),
                env_vars={"PYTHONPATH": src_dir},
            )

            stack = auto.create_or_select_stack(
                stack_name=self.stack_name,
                project_name="mesh-cluster",
                program=self._pulumi_program,
                opts=workspace_opts,
            )

            for key, value in config.items():
                stack.set_config(key, auto.ConfigValue(value=value))

            ts_api_key = get_env(EnvVars.TAILSCALE_KEY, default="")
            if ts_api_key:
                stack.set_config(
                    "tailscale:apiKey", auto.ConfigValue(value=ts_api_key, secret=True)
                )
            ts_tailnet = get_env(EnvVars.TAILSCALE_TAILNET, default="")
            if ts_tailnet:
                stack.set_config(
                    "tailscale:tailnet", auto.ConfigValue(value=ts_tailnet)
                )

            if progress_callback:
                progress_callback(f"🚀 Starting deployment")

            result = stack.up(on_output=progress_callback)

            outputs = {key: output.value for key, output in result.outputs.items()}

            duration = None
            if result.summary.start_time and result.summary.end_time:
                duration = (
                    result.summary.end_time - result.summary.start_time
                ).total_seconds()

            return {
                "status": "success",
                "outputs": outputs,
                "summary": {
                    "resource_changes": result.summary.resource_changes,
                    "duration_seconds": duration,
                },
            }

        except Exception as e:
            return {"status": "error", "error": str(e), "outputs": {}}

    def destroy_cluster(
        self, progress_callback: Optional[Callable[[str], None]] = None
    ) -> Dict[str, Any]:
        try:
            src_dir = os.path.abspath(
                os.path.join(os.path.dirname(__file__), "..", "..", "..")
            )
            workspace_opts = LocalWorkspaceOptions(
                work_dir=self.work_dir,
                project_settings=ProjectSettings(name="mesh-cluster", runtime="python"),
                env_vars={"PYTHONPATH": src_dir},
            )
            stack = auto.create_or_select_stack(
                stack_name=self.stack_name,
                project_name="mesh-cluster",
                program=self._pulumi_program,
                opts=workspace_opts,
            )

            if progress_callback:
                progress_callback(f"🗑️  Destroying {self.stack_name}")

            result = stack.destroy(on_output=progress_callback)

            duration = None
            if result.summary.start_time and result.summary.end_time:
                duration = (
                    result.summary.end_time - result.summary.start_time
                ).total_seconds()

            return {
                "status": "success",
                "summary": {
                    "resource_changes": result.summary.resource_changes,
                    "duration_seconds": duration,
                },
            }

        except Exception as e:
            return {"status": "error", "error": str(e)}

    def _pulumi_program(self):
        import pulumi
        from mesh.infrastructure.configure_tailscale import (
            configure as tailscale_feature,
        )
        from mesh.infrastructure.provision_node.provision_node import provision_node

        config = pulumi.Config()
        provider = config.get("provider")
        region = config.get("region")

        if not provider:
            raise ValueError("provider is required in Pulumi config")
        if not region:
            raise ValueError("region is required in Pulumi config")

        tailscale_key_resource = tailscale_feature.create_auth_key("mesh-cluster-key")

        leader = provision_node(
            name="vm-leader",
            provider=provider,
            role="server",
            size=pulumi.Config().get("leader_size") or "t3.small",
            tailscale_auth_key=tailscale_key_resource.key,
            leader_ip="",
            region=region,
        )

        pulumi.export("leader_public_ip", leader["public_ip"])
        pulumi.export("leader_private_ip", leader["private_ip"])


def deploy_cluster_from_config(
    config: Dict[str, str],
    stack_name: str = "dev",
    progress_callback: Optional[Callable[[str], None]] = None,
) -> Dict[str, Any]:
    automation = CloudClusterAutomation(stack_name=stack_name)
    return automation.deploy_cluster(config, progress_callback)


def destroy_cluster_stack(
    stack_name: str = "dev", progress_callback: Optional[Callable[[str], None]] = None
) -> Dict[str, Any]:
    automation = CloudClusterAutomation(stack_name=stack_name)
    return automation.destroy_cluster(progress_callback)
