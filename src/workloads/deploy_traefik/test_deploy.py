"""
Unit tests for Traefik deployment functionality
"""

import subprocess
from unittest.mock import patch, MagicMock
from src.workloads.deploy_traefik.deploy import deploy_traefik, ACME_SERVERS


class TestACMEServers:
    """Test ACME server URL mapping"""

    def test_letsencrypt_production_url(self):
        """Test Let's Encrypt production URL"""
        assert ACME_SERVERS["letsencrypt"] == "https://acme-v02.api.letsencrypt.org/directory"

    def test_letsencrypt_staging_url(self):
        """Test Let's Encrypt staging URL"""
        assert ACME_SERVERS["letsencrypt-staging"] == "https://acme-staging-v02.api.letsencrypt.org/directory"


class TestDeployTraefik:
    """Test Traefik deployment"""

    @patch('src.workloads.deploy_traefik.deploy.subprocess.run')
    @patch('src.workloads.deploy_traefik.deploy.os.path.exists')
    def test_deploy_traefik_success(self, mock_exists, mock_run):
        """Test successful Traefik deployment"""
        mock_exists.return_value = True
        mock_run.return_value = MagicMock(stdout="Deployment successful", returncode=0)

        result = deploy_traefik(
            acme_email="admin@example.com",
            acme_ca_server="letsencrypt-staging"
        )

        assert result is True
        mock_run.assert_called_once()

    @patch('src.workloads.deploy_traefik.deploy.subprocess.run')
    @patch('src.workloads.deploy_traefik.deploy.os.path.exists')
    def test_deploy_traefik_job_file_not_found(self, mock_exists, mock_run):
        """Test Traefik deployment with missing job file"""
        mock_exists.return_value = False

        result = deploy_traefik(acme_email="admin@example.com")

        assert result is False
        mock_run.assert_not_called()

    @patch('src.workloads.deploy_traefik.deploy.subprocess.run')
    @patch('src.workloads.deploy_traefik.deploy.os.path.exists')
    def test_deploy_traefik_command_failure(self, mock_exists, mock_run):
        """Test Traefik deployment with command failure"""
        mock_exists.return_value = True
        mock_run.side_effect = subprocess.CalledProcessError(1, "nomad", stderr="Deployment failed")

        result = deploy_traefik(acme_email="admin@example.com")

        assert result is False

    def test_deploy_traefik_default_parameters(self):
        """Test Traefik deployment with default parameters"""
        with patch('src.workloads.deploy_traefik.deploy.subprocess.run') as mock_run, \
             patch('src.workloads.deploy_traefik.deploy.os.path.exists', return_value=True):
            mock_run.return_value = MagicMock(stdout="Success", returncode=0)

            deploy_traefik(acme_email="admin@example.com")

            call_args = mock_run.call_args[0][0]
            assert "acme_email=admin@example.com" in call_args
            assert "dashboard_enabled=true" in call_args
            assert "log_level=INFO" in call_args

    def test_deploy_traefik_custom_parameters(self):
        """Test Traefik deployment with custom parameters"""
        with patch('src.workloads.deploy_traefik.deploy.subprocess.run') as mock_run, \
             patch('src.workloads.deploy_traefik.deploy.os.path.exists', return_value=True):
            mock_run.return_value = MagicMock(stdout="Success", returncode=0)

            deploy_traefik(
                acme_email="admin@example.com",
                acme_ca_server="letsencrypt-staging",
                use_tls_challenge=False,
                use_http_challenge=True,
                memory=512,
                cpu=500,
                dashboard_enabled=False,
                log_level="DEBUG"
            )

            call_args = mock_run.call_args[0][0]
            assert "acme_email=admin@example.com" in call_args
            assert "use_tls_challenge=false" in call_args
            assert "use_http_challenge=true" in call_args
            assert "memory=512" in call_args
            assert "cpu=500" in call_args
            assert "dashboard_enabled=false" in call_args
            assert "log_level=DEBUG" in call_args

    def test_deploy_traefik_acme_server_resolution(self):
        """Test that named ACME servers are resolved to URLs"""
        with patch('src.workloads.deploy_traefik.deploy.subprocess.run') as mock_run, \
             patch('src.workloads.deploy_traefik.deploy.os.path.exists', return_value=True):
            mock_run.return_value = MagicMock(stdout="Success", returncode=0)

            deploy_traefik(
                acme_email="admin@example.com",
                acme_ca_server="letsencrypt-staging"
            )

            call_args = mock_run.call_args[0][0]
            assert "acme_ca_server=https://acme-staging-v02.api.letsencrypt.org/directory" in call_args

    def test_deploy_traefik_custom_acme_url(self):
        """Test that custom ACME server URLs are passed through"""
        custom_url = "https://custom-acme.example.com/directory"

        with patch('src.workloads.deploy_traefik.deploy.subprocess.run') as mock_run, \
             patch('src.workloads.deploy_traefik.deploy.os.path.exists', return_value=True):
            mock_run.return_value = MagicMock(stdout="Success", returncode=0)

            deploy_traefik(
                acme_email="admin@example.com",
                acme_ca_server=custom_url
            )

            call_args = mock_run.call_args[0][0]
            assert f"acme_ca_server={custom_url}" in call_args


class TestTraefikConfiguration:
    """Test Traefik configuration options"""

    def test_tls_challenge_enabled_by_default(self):
        """Test that TLS challenge is enabled by default"""
        with patch('src.workloads.deploy_traefik.deploy.subprocess.run') as mock_run, \
             patch('src.workloads.deploy_traefik.deploy.os.path.exists', return_value=True):
            mock_run.return_value = MagicMock(stdout="Success", returncode=0)

            deploy_traefik(acme_email="admin@example.com")

            call_args = mock_run.call_args[0][0]
            assert "use_tls_challenge=true" in call_args

    def test_http_challenge_disabled_by_default(self):
        """Test that HTTP challenge is disabled by default"""
        with patch('src.workloads.deploy_traefik.deploy.subprocess.run') as mock_run, \
             patch('src.workloads.deploy_traefik.deploy.os.path.exists', return_value=True):
            mock_run.return_value = MagicMock(stdout="Success", returncode=0)

            deploy_traefik(acme_email="admin@example.com")

            call_args = mock_run.call_args[0][0]
            assert "use_http_challenge=false" in call_args

    def test_dashboard_enabled_by_default(self):
        """Test that dashboard is enabled by default"""
        with patch('src.workloads.deploy_traefik.deploy.subprocess.run') as mock_run, \
             patch('src.workloads.deploy_traefik.deploy.os.path.exists', return_value=True):
            mock_run.return_value = MagicMock(stdout="Success", returncode=0)

            deploy_traefik(acme_email="admin@example.com")

            call_args = mock_run.call_args[0][0]
            assert "dashboard_enabled=true" in call_args

    def test_default_memory_and_cpu(self):
        """Test default resource allocations"""
        with patch('src.workloads.deploy_traefik.deploy.subprocess.run') as mock_run, \
             patch('src.workloads.deploy_traefik.deploy.os.path.exists', return_value=True):
            mock_run.return_value = MagicMock(stdout="Success", returncode=0)

            deploy_traefik(acme_email="admin@example.com")

            call_args = mock_run.call_args[0][0]
            assert "memory=256" in call_args
            assert "cpu=200" in call_args
