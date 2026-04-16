"""
Tests for mesh ssh command.
"""

from unittest.mock import patch, MagicMock
from typer.testing import CliRunner

from mesh.cli.main import app

runner = CliRunner()


class TestSshNoCluster:
    @patch("mesh.cli.commands.ssh._check_cluster", return_value=False)
    def test_no_cluster_shows_error(self, mock_check):
        result = runner.invoke(app, ["ssh"])
        assert result.exit_code == 1
        assert "No cluster available" in result.output

    @patch("mesh.cli.commands.ssh._check_cluster", return_value=False)
    def test_no_cluster_with_node_name(self, mock_check):
        result = runner.invoke(app, ["ssh", "mesh-leader"])
        assert result.exit_code == 1
        assert "No cluster available" in result.output


class TestSshListNodes:
    @patch("mesh.cli.commands.ssh._get_tailscale_ips", return_value={})
    @patch(
        "mesh.cli.commands.ssh._get_nodes",
        return_value=[
            {
                "id": "a1",
                "name": "mesh-leader",
                "status": "ready",
                "address": "10.0.0.1",
                "datacenter": "dc1",
                "role": "server",
            },
            {
                "id": "b2",
                "name": "mesh-worker-1",
                "status": "ready",
                "address": "10.0.0.2",
                "datacenter": "dc1",
                "role": "client",
            },
        ],
    )
    @patch("mesh.cli.commands.ssh._check_cluster", return_value=True)
    def test_lists_nodes_when_no_node_name(self, mock_check, mock_nodes, mock_ts):
        result = runner.invoke(app, ["ssh"])
        assert result.exit_code == 0
        assert "mesh-leader" in result.output
        assert "mesh-worker-1" in result.output

    @patch("mesh.cli.commands.ssh._get_tailscale_ips", return_value={})
    @patch("mesh.cli.commands.ssh._get_nodes", return_value=[])
    @patch("mesh.cli.commands.ssh._check_cluster", return_value=True)
    def test_no_nodes_found(self, mock_check, mock_nodes, mock_ts):
        result = runner.invoke(app, ["ssh"])
        assert result.exit_code == 0
        assert "No nodes found" in result.output

    @patch(
        "mesh.cli.commands.ssh._get_tailscale_ips",
        return_value={"mesh-leader": "100.64.0.1"},
    )
    @patch(
        "mesh.cli.commands.ssh._get_nodes",
        return_value=[
            {
                "id": "a1",
                "name": "mesh-leader",
                "status": "ready",
                "address": "10.0.0.1",
                "datacenter": "dc1",
                "role": "server",
            },
        ],
    )
    @patch("mesh.cli.commands.ssh._check_cluster", return_value=True)
    def test_tailscale_ip_shown_in_list(self, mock_check, mock_nodes, mock_ts):
        result = runner.invoke(app, ["ssh"])
        assert result.exit_code == 0
        assert "100.64.0.1" in result.output


class TestSshConnect:
    @patch("mesh.cli.commands.ssh.subprocess.Popen")
    @patch(
        "mesh.cli.commands.ssh._get_tailscale_ips",
        return_value={"mesh-leader": "100.64.0.1"},
    )
    @patch(
        "mesh.cli.commands.ssh._get_nodes",
        return_value=[
            {
                "id": "a1",
                "name": "mesh-leader",
                "status": "ready",
                "address": "10.0.0.1",
                "datacenter": "dc1",
                "role": "server",
            },
        ],
    )
    @patch("mesh.cli.commands.ssh._check_cluster", return_value=True)
    def test_connect_with_tailscale_ip(
        self, mock_check, mock_nodes, mock_ts, mock_popen
    ):
        mock_proc = MagicMock()
        mock_proc.wait.return_value = 0
        mock_popen.return_value = mock_proc
        result = runner.invoke(app, ["ssh", "mesh-leader"])
        assert result.exit_code == 0
        cmd_called = mock_popen.call_args[0][0]
        assert "ubuntu@100.64.0.1" in cmd_called

    @patch("mesh.cli.commands.ssh.subprocess.Popen")
    @patch("mesh.cli.commands.ssh._get_tailscale_ips", return_value={})
    @patch(
        "mesh.cli.commands.ssh._get_nodes",
        return_value=[
            {
                "id": "a1",
                "name": "mesh-leader",
                "status": "ready",
                "address": "10.0.0.1",
                "datacenter": "dc1",
                "role": "server",
            },
        ],
    )
    @patch("mesh.cli.commands.ssh._check_cluster", return_value=True)
    def test_connect_fallback_to_node_address(
        self, mock_check, mock_nodes, mock_ts, mock_popen
    ):
        mock_proc = MagicMock()
        mock_proc.wait.return_value = 0
        mock_popen.return_value = mock_proc
        result = runner.invoke(app, ["ssh", "mesh-leader"])
        assert result.exit_code == 0
        cmd_called = mock_popen.call_args[0][0]
        assert "ubuntu@10.0.0.1" in cmd_called

    @patch("mesh.cli.commands.ssh.subprocess.Popen")
    @patch("mesh.cli.commands.ssh._get_tailscale_ips", return_value={})
    @patch(
        "mesh.cli.commands.ssh._get_nodes",
        return_value=[
            {
                "id": "a1",
                "name": "mesh-leader",
                "status": "ready",
                "address": "10.0.0.1",
                "datacenter": "dc1",
                "role": "server",
            },
        ],
    )
    @patch("mesh.cli.commands.ssh._check_cluster", return_value=True)
    def test_custom_user(self, mock_check, mock_nodes, mock_ts, mock_popen):
        mock_proc = MagicMock()
        mock_proc.wait.return_value = 0
        mock_popen.return_value = mock_proc
        result = runner.invoke(app, ["ssh", "mesh-leader", "--user", "admin"])
        assert result.exit_code == 0
        cmd_called = mock_popen.call_args[0][0]
        assert "admin@10.0.0.1" in cmd_called

    @patch("mesh.cli.commands.ssh._get_tailscale_ips", return_value={})
    @patch(
        "mesh.cli.commands.ssh._get_nodes",
        return_value=[
            {
                "id": "a1",
                "name": "mesh-leader",
                "status": "ready",
                "address": "10.0.0.1",
                "datacenter": "dc1",
                "role": "server",
            },
        ],
    )
    @patch("mesh.cli.commands.ssh._check_cluster", return_value=True)
    def test_node_not_found(self, mock_check, mock_nodes, mock_ts):
        result = runner.invoke(app, ["ssh", "nonexistent"])
        assert result.exit_code == 1
        assert "not found" in result.output

    @patch("mesh.cli.commands.ssh.subprocess.Popen", side_effect=FileNotFoundError)
    @patch("mesh.cli.commands.ssh._get_tailscale_ips", return_value={})
    @patch(
        "mesh.cli.commands.ssh._get_nodes",
        return_value=[
            {
                "id": "a1",
                "name": "mesh-leader",
                "status": "ready",
                "address": "10.0.0.1",
                "datacenter": "dc1",
                "role": "server",
            },
        ],
    )
    @patch("mesh.cli.commands.ssh._check_cluster", return_value=True)
    def test_ssh_not_found(self, mock_check, mock_nodes, mock_ts, mock_popen):
        result = runner.invoke(app, ["ssh", "mesh-leader"])
        assert result.exit_code == 1
        assert "ssh not found" in result.output
