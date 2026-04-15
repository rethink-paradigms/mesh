"""
Tests for Adapter: Multipass Provisioner for Nodes
"""
from unittest.mock import patch, MagicMock
import json
import yaml

# Import from the adapter module itself
from src.infrastructure.provision_node.multipass import provision_multipass_node, generate_cloud_init_yaml

@patch("src.infrastructure.provision_node.multipass.subprocess.run")
@patch("src.infrastructure.provision_node.multipass.os.remove")
@patch("src.infrastructure.provision_node.multipass.click.echo") # Mock click.echo to prevent output during tests
def test_generate_cloud_init_yaml(mock_click_echo, mock_os_remove, mock_subprocess_run):
    """
    Verify cloud-init YAML generation is correct.
    """
    boot_script = "echo HELLO WORLD\nTAILSCALE_KEY=ABC"
    cloud_init_content = generate_cloud_init_yaml(boot_script)
    
    assert cloud_init_content.startswith("#cloud-config")
    
    loaded_yaml = yaml.safe_load(cloud_init_content.replace("#cloud-config\n", ""))
    
    assert loaded_yaml["package_update"] is True
    assert "/opt/ops-platform/startup.sh" in loaded_yaml["runcmd"]
    assert {"path": "/opt/ops-platform/startup.sh", "permissions": "0755", "content": boot_script} in loaded_yaml["write_files"]

@patch("src.infrastructure.provision_node.multipass.subprocess.run")
@patch("src.infrastructure.provision_node.multipass.os.remove")
@patch("src.infrastructure.provision_node.multipass.click.echo")
def test_provision_multipass_node_launch(mock_click_echo, mock_os_remove, mock_subprocess_run):
    """
    Verify multipass launch command is correctly executed.
    """
    # Mock multipass info to say node doesn't exist
    mock_subprocess_run.side_effect = [
        MagicMock(returncode=1), # 1. info check (fails -> not exists)
        MagicMock(returncode=0), # 2. launch command
        MagicMock(returncode=0, stdout=json.dumps({"info": {"test-mp-node": {"ipv4": ["10.0.0.5"]}}})), # 3. info after launch (get IP)
    ]

    boot_script = "echo hello"
    node_name = "test-mp-node"
    instance_size = "2CPU,1G"
    role = "server"

    result = provision_multipass_node(
        name=node_name,
        instance_size=instance_size,
        role=role,
        boot_script_content=boot_script
    )
    
    # Verify launch command
    expected_launch_cmd_prefix = [
        "multipass", "launch", "lts",
        "--name", node_name,
        "--cpus", "2",
        "--memory", "1G",
        "--cloud-init"
    ]
    # Check that the launch call was made
    launch_called = False
    for call in mock_subprocess_run.call_args_list:
        args = call.args[0]
        # Check if command starts with our expected prefix (ignoring the temp file path at the end)
        if len(args) > len(expected_launch_cmd_prefix) and args[:len(expected_launch_cmd_prefix)] == expected_launch_cmd_prefix:
            launch_called = True
            # Verify the last arg is a cloud-init path in tmp
            assert args[-1].startswith("/tmp/cloud-init-")
            break
            
    assert launch_called, f"Launch command not found. Calls: {mock_subprocess_run.call_args_list}"

    assert result["public_ip"] == "10.0.0.5"
    assert result["instance_id"] == node_name

@patch("src.infrastructure.provision_node.multipass.subprocess.run")
@patch("src.infrastructure.provision_node.multipass.os.remove")
@patch("src.infrastructure.provision_node.multipass.click.echo")
def test_provision_multipass_node_existing_running(mock_click_echo, mock_os_remove, mock_subprocess_run):
    """
    Verify if node is already running, no launch command is executed.
    """
    # Mock multipass info to say node exists and is running
    mock_subprocess_run.side_effect = [
        MagicMock(returncode=0, stdout=json.dumps({"info": {"existing-node": {"state": "Running", "ipv4": ["10.0.0.6"]}}})), # 1. info check
        # No launch, no start
        MagicMock(returncode=0, stdout=json.dumps({"info": {"existing-node": {"ipv4": ["10.0.0.6"]}}})), # 2. info get IP
    ]

    boot_script = "echo hello"
    node_name = "existing-node"
    instance_size = "1CPU,512M"
    role = "client"

    result = provision_multipass_node(
        name=node_name,
        instance_size=instance_size,
        role=role,
        boot_script_content=boot_script
    )
    
    # Check that 'launch' was NOT called
    for call in mock_subprocess_run.call_args_list:
        assert "launch" not in call.args[0]
    
    assert result["public_ip"] == "10.0.0.6"
    
@patch("src.infrastructure.provision_node.multipass.subprocess.run")
@patch("src.infrastructure.provision_node.multipass.os.remove")
@patch("src.infrastructure.provision_node.multipass.click.echo")
def test_provision_multipass_node_existing_stopped(mock_click_echo, mock_os_remove, mock_subprocess_run):
    """
    Verify if node is stopped, it's started and then its info is retrieved.
    """
    # Mock multipass info to say node exists but is stopped
    mock_subprocess_run.side_effect = [
        MagicMock(returncode=0, stdout=json.dumps({"info": {"stopped-node": {"state": "Stopped", "ipv4": []}}})), # 1. info check
        MagicMock(returncode=0), # 2. multipass start
        MagicMock(returncode=0, stdout=json.dumps({"info": {"stopped-node": {"ipv4": ["10.0.0.7"]}}})), # 3. info get IP
    ]

    boot_script = "echo hello"
    node_name = "stopped-node"
    instance_size = "1CPU,512M"
    role = "client"

    result = provision_multipass_node(
        name=node_name,
        instance_size=instance_size,
        role=role,
        boot_script_content=boot_script
    )
    
    # Verify 'start' command was called
    mock_subprocess_run.assert_any_call(["multipass", "start", node_name], check=True)
    assert result["public_ip"] == "10.0.0.7"
