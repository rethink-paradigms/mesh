"""
Tests for mesh logs command.
"""

from unittest.mock import patch, MagicMock
from typer.testing import CliRunner

from mesh.cli.main import app

runner = CliRunner()


class TestLogsNoCluster:
    @patch("mesh.cli.commands.logs._check_cluster", return_value=False)
    def test_no_cluster_shows_error(self, mock_check):
        result = runner.invoke(app, ["logs"])
        assert result.exit_code == 1
        assert "No cluster available" in result.output

    @patch("mesh.cli.commands.logs._check_cluster", return_value=False)
    def test_no_cluster_with_job_name(self, mock_check):
        result = runner.invoke(app, ["logs", "my-job"])
        assert result.exit_code == 1
        assert "No cluster available" in result.output


class TestLogsListJobs:
    @patch(
        "mesh.cli.commands.logs._list_running_jobs",
        return_value=[
            {
                "id": "researcher",
                "type": "service",
                "status": "running",
                "summary": "1/1",
            },
            {
                "id": "code-writer",
                "type": "service",
                "status": "running",
                "summary": "1/1",
            },
        ],
    )
    @patch("mesh.cli.commands.logs._check_cluster", return_value=True)
    def test_lists_jobs_when_no_job_name(self, mock_check, mock_list):
        result = runner.invoke(app, ["logs"])
        assert result.exit_code == 0
        assert "researcher" in result.output
        assert "code-writer" in result.output

    @patch("mesh.cli.commands.logs._list_running_jobs", return_value=[])
    @patch("mesh.cli.commands.logs._check_cluster", return_value=True)
    def test_no_jobs_found(self, mock_check, mock_list):
        result = runner.invoke(app, ["logs"])
        assert result.exit_code == 0
        assert "No running jobs" in result.output


class TestLogsStreaming:
    @patch("mesh.cli.commands.logs.subprocess.Popen")
    @patch("mesh.cli.commands.logs._check_cluster", return_value=True)
    def test_follow_flag_passes_f(self, mock_check, mock_popen):
        mock_proc = MagicMock()
        mock_proc.wait.return_value = 0
        mock_popen.return_value = mock_proc
        result = runner.invoke(app, ["logs", "my-job", "--follow"])
        assert result.exit_code == 0
        cmd_called = mock_popen.call_args[0][0]
        assert "-f" in cmd_called

    @patch("mesh.cli.commands.logs.subprocess.Popen")
    @patch("mesh.cli.commands.logs._check_cluster", return_value=True)
    def test_tail_flag_passes_n(self, mock_check, mock_popen):
        mock_proc = MagicMock()
        mock_proc.wait.return_value = 0
        mock_popen.return_value = mock_proc
        result = runner.invoke(app, ["logs", "my-job", "--tail", "50"])
        assert result.exit_code == 0
        cmd_called = mock_popen.call_args[0][0]
        assert "50" in cmd_called

    @patch("mesh.cli.commands.logs.subprocess.Popen")
    @patch("mesh.cli.commands.logs._check_cluster", return_value=True)
    def test_stderr_flag(self, mock_check, mock_popen):
        mock_proc = MagicMock()
        mock_proc.wait.return_value = 0
        mock_popen.return_value = mock_proc
        result = runner.invoke(app, ["logs", "my-job", "--stderr"])
        assert result.exit_code == 0
        cmd_called = mock_popen.call_args[0][0]
        assert "-stderr" in cmd_called

    @patch("mesh.cli.commands.logs.subprocess.Popen")
    @patch("mesh.cli.commands.logs._check_cluster", return_value=True)
    def test_alloc_flag(self, mock_check, mock_popen):
        mock_proc = MagicMock()
        mock_proc.wait.return_value = 0
        mock_popen.return_value = mock_proc
        result = runner.invoke(app, ["logs", "my-job", "--alloc", "abc123"])
        assert result.exit_code == 0
        cmd_called = mock_popen.call_args[0][0]
        assert "abc123" in cmd_called

    @patch("mesh.cli.commands.logs.subprocess.Popen", side_effect=FileNotFoundError)
    @patch("mesh.cli.commands.logs._check_cluster", return_value=True)
    def test_nomad_not_found(self, mock_check, mock_popen):
        result = runner.invoke(app, ["logs", "my-job"])
        assert result.exit_code == 1
        assert "nomad CLI not found" in result.output

    @patch("mesh.cli.commands.logs.subprocess.Popen")
    @patch("mesh.cli.commands.logs._check_cluster", return_value=True)
    def test_job_name_in_command(self, mock_check, mock_popen):
        mock_proc = MagicMock()
        mock_proc.wait.return_value = 0
        mock_popen.return_value = mock_proc
        result = runner.invoke(app, ["logs", "my-app"])
        assert result.exit_code == 0
        cmd_called = mock_popen.call_args[0][0]
        assert "my-app" in cmd_called
