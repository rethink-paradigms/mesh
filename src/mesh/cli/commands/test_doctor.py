"""
Tests for mesh doctor command.
"""

from unittest.mock import patch, MagicMock
from typer.testing import CliRunner

from mesh.cli.main import app

runner = CliRunner()


class TestDoctorDemoMode:
    def test_demo_mode_passes(self):
        result = runner.invoke(app, ["doctor", "--demo"])
        assert result.exit_code == 0
        assert "7/7 checks passed" in result.output

    def test_demo_mode_shows_banner(self):
        result = runner.invoke(app, ["doctor", "--demo"])
        assert result.exit_code == 0
        assert (
            "Infrastructure Platform" in result.output
            or "infrastructure platform" in result.output.lower()
        )


class TestDoctorPythonCheck:
    @patch("mesh.cli.commands.doctor._check_python_version", return_value=(True, "3.12.0"))
    @patch(
        "mesh.cli.commands.doctor._check_docker_installed",
        return_value=(False, "not found"),
    )
    @patch(
        "mesh.cli.commands.doctor._check_docker_running",
        return_value=(False, "docker not found"),
    )
    @patch("mesh.cli.commands.doctor._check_pulumi", return_value=(False, "not found"))
    @patch("mesh.cli.commands.doctor._check_tailscale", return_value=(False, "not found"))
    @patch("mesh.cli.commands.doctor._check_env", return_value=(False, ".env not found"))
    @patch("mesh.cli.commands.doctor._check_network", return_value=(True, "OK"))
    def test_python_check_passes(
        self,
        mock_net,
        mock_env,
        mock_ts,
        mock_pulumi,
        mock_dock_run,
        mock_dock,
        mock_py,
    ):
        result = runner.invoke(app, ["doctor"])
        assert result.exit_code == 0
        assert "2/7 checks passed" in result.output

    @patch(
        "mesh.cli.commands.doctor._check_python_version",
        return_value=(False, "3.10.0 (need >=3.11)"),
    )
    @patch(
        "mesh.cli.commands.doctor._check_docker_installed",
        return_value=(True, "v24.0.7"),
    )
    @patch("mesh.cli.commands.doctor._check_docker_running", return_value=(True, "OK"))
    @patch("mesh.cli.commands.doctor._check_pulumi", return_value=(True, "v3.100.0"))
    @patch("mesh.cli.commands.doctor._check_tailscale", return_value=(True, "v1.56.0"))
    @patch(
        "mesh.cli.commands.doctor._check_env",
        return_value=(True, ".env found, TAILSCALE_KEY set"),
    )
    @patch("mesh.cli.commands.doctor._check_network", return_value=(True, "OK"))
    def test_python_check_fails(
        self,
        mock_net,
        mock_env,
        mock_ts,
        mock_pulumi,
        mock_dock_run,
        mock_dock,
        mock_py,
    ):
        result = runner.invoke(app, ["doctor"])
        assert result.exit_code == 0
        assert "6/7 checks passed" in result.output


class TestDoctorAllChecks:
    @patch("mesh.cli.commands.doctor._check_python_version", return_value=(True, "3.12.0"))
    @patch(
        "mesh.cli.commands.doctor._check_docker_installed",
        return_value=(True, "v24.0.7"),
    )
    @patch("mesh.cli.commands.doctor._check_docker_running", return_value=(True, "OK"))
    @patch("mesh.cli.commands.doctor._check_pulumi", return_value=(True, "v3.100.0"))
    @patch("mesh.cli.commands.doctor._check_tailscale", return_value=(True, "v1.56.0"))
    @patch(
        "mesh.cli.commands.doctor._check_env",
        return_value=(True, ".env found, TAILSCALE_KEY set"),
    )
    @patch("mesh.cli.commands.doctor._check_network", return_value=(True, "OK"))
    def test_all_pass(
        self,
        mock_net,
        mock_env,
        mock_ts,
        mock_pulumi,
        mock_dock_run,
        mock_dock,
        mock_py,
    ):
        result = runner.invoke(app, ["doctor"])
        assert result.exit_code == 0
        assert "7/7 checks passed" in result.output

    @patch("mesh.cli.commands.doctor._check_python_version", return_value=(False, "3.10.0"))
    @patch(
        "mesh.cli.commands.doctor._check_docker_installed",
        return_value=(False, "not found"),
    )
    @patch(
        "mesh.cli.commands.doctor._check_docker_running",
        return_value=(False, "not found"),
    )
    @patch("mesh.cli.commands.doctor._check_pulumi", return_value=(False, "not found"))
    @patch("mesh.cli.commands.doctor._check_tailscale", return_value=(False, "not found"))
    @patch("mesh.cli.commands.doctor._check_env", return_value=(False, ".env not found"))
    @patch(
        "mesh.cli.commands.doctor._check_network",
        return_value=(False, "no connectivity"),
    )
    def test_all_fail(
        self,
        mock_net,
        mock_env,
        mock_ts,
        mock_pulumi,
        mock_dock_run,
        mock_dock,
        mock_py,
    ):
        result = runner.invoke(app, ["doctor"])
        assert result.exit_code == 0
        assert "0/7 checks passed" in result.output


class TestDoctorIndividualChecks:
    @patch("mesh.cli.commands.doctor.shutil.which", return_value=None)
    def test_docker_not_installed(self, mock_which):
        from mesh.cli.commands.doctor import _check_docker_installed

        ok, detail = _check_docker_installed()
        assert ok is False
        assert "not found" in detail

    @patch("mesh.cli.commands.doctor.shutil.which", return_value="/usr/bin/docker")
    @patch("mesh.cli.commands.doctor.subprocess.run")
    def test_docker_installed(self, mock_run, mock_which):
        mock_run.return_value = MagicMock(returncode=0, stdout="Docker version 24.0.7")
        from mesh.cli.commands.doctor import _check_docker_installed

        ok, detail = _check_docker_installed()
        assert ok is True
        assert "24.0.7" in detail

    @patch("mesh.cli.commands.doctor.shutil.which", return_value=None)
    def test_pulumi_not_installed(self, mock_which):
        from mesh.cli.commands.doctor import _check_pulumi

        ok, detail = _check_pulumi()
        assert ok is False

    @patch("mesh.cli.commands.doctor.shutil.which", return_value=None)
    def test_tailscale_not_installed(self, mock_which):
        from mesh.cli.commands.doctor import _check_tailscale

        ok, detail = _check_tailscale()
        assert ok is False

    @patch("mesh.cli.commands.doctor.os.path.exists", return_value=True)
    @patch("mesh.cli.commands.doctor.os.getenv", return_value="tskey-abc123")
    def test_env_check_passes(self, mock_getenv, mock_exists):
        from mesh.cli.commands.doctor import _check_env

        ok, detail = _check_env()
        assert ok is True

    @patch("mesh.cli.commands.doctor.os.path.exists", return_value=False)
    @patch("mesh.cli.commands.doctor.os.getenv", return_value=None)
    def test_env_check_fails(self, mock_getenv, mock_exists):
        from mesh.cli.commands.doctor import _check_env

        ok, detail = _check_env()
        assert ok is False
