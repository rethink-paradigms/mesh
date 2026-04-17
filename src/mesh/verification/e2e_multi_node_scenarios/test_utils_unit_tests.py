"""
Unit tests for E2E test utilities
"""

import pytest
import os
import requests
import subprocess
from unittest.mock import patch, MagicMock

from mesh.verification.e2e_multi_node_scenarios.test_utils import (
    ClusterConfig,
    get_cluster_nodes,
    get_multipass_ip,
    deploy_job,
    wait_for_allocation,
    get_allocation_nodes,
    verify_service_discovery,
    check_tailscale_mesh,
    check_traefik_routing,
    cleanup_job,
)


class TestClusterConfig:
    """Test ClusterConfig class"""

    def test_cluster_config_defaults(self):
        """Test default ClusterConfig values"""
        with patch.dict(os.environ, {}, clear=True):
            config = ClusterConfig()

            assert config.target_env == "local"
            assert config.leader_ip is None
            assert config.worker_ips == []
            assert config.is_cross_cloud() is False

    def test_cluster_config_from_env(self):
        """Test ClusterConfig loading from environment variables"""
        env_vars = {
            "E2E_TARGET_ENV": "aws",
            "E2E_LEADER_IP": "1.2.3.4",
            "E2E_WORKER_IPS": "5.6.7.8,9.10.11.12",
            "E2E_CROSS_CLOUD": "true",
        }

        with patch.dict(os.environ, env_vars, clear=True):
            config = ClusterConfig()

            assert config.target_env == "aws"
            assert config.leader_ip == "1.2.3.4"
            assert config.worker_ips == ["5.6.7.8", "9.10.11.12"]
            assert config.is_cross_cloud() is True

    def test_is_local(self):
        """Test is_local detection"""
        with patch.dict(os.environ, {"E2E_TARGET_ENV": "local"}, clear=True):
            config = ClusterConfig()
            assert config.is_local() is True
            assert config.is_cloud() is False

    def test_is_cloud(self):
        """Test is_cloud detection"""
        for env in ["aws", "hetzner", "cloud"]:
            with patch.dict(os.environ, {"E2E_TARGET_ENV": env}, clear=True):
                config = ClusterConfig()
                assert config.is_local() is False
                assert config.is_cloud() is True

    def test_parse_worker_ips_empty(self):
        """Test parsing empty worker IPs"""
        with patch.dict(os.environ, {"E2E_WORKER_IPS": ""}, clear=True):
            config = ClusterConfig()
            assert config.worker_ips == []

    def test_parse_worker_ips_single(self):
        """Test parsing single worker IP"""
        with patch.dict(os.environ, {"E2E_WORKER_IPS": "1.2.3.4"}, clear=True):
            config = ClusterConfig()
            assert config.worker_ips == ["1.2.3.4"]

    def test_parse_worker_ips_multiple(self):
        """Test parsing multiple worker IPs"""
        with patch.dict(os.environ, {"E2E_WORKER_IPS": "1.2.3.4, 5.6.7.8, 9.10.11.12"}, clear=True):
            config = ClusterConfig()
            assert config.worker_ips == ["1.2.3.4", "5.6.7.8", "9.10.11.12"]


class TestGetClusterNodes:
    """Test get_cluster_nodes function"""

    @patch.dict(os.environ, {"E2E_LEADER_IP": "1.2.3.4"}, clear=True)
    def test_get_cluster_nodes_with_leader(self):
        """Test getting cluster nodes with leader IP"""
        nodes = get_cluster_nodes()

        assert len(nodes) >= 1
        assert nodes[0]["name"] == "leader"
        assert nodes[0]["ip"] == "1.2.3.4"
        assert nodes[0]["role"] == "server"

    @patch.dict(
        os.environ,
        {"E2E_LEADER_IP": "1.2.3.4", "E2E_WORKER_IPS": "5.6.7.8,9.10.11.12"},
        clear=True,
    )
    def test_get_cluster_nodes_with_workers(self):
        """Test getting cluster nodes with workers"""
        nodes = get_cluster_nodes()

        assert len(nodes) >= 3
        assert nodes[0]["name"] == "leader"
        assert nodes[1]["name"] == "worker-1"
        assert nodes[2]["name"] == "worker-2"

    @patch("mesh.verification.e2e_multi_node_scenarios.test_utils.get_multipass_ip")
    @patch.dict(os.environ, {"E2E_TARGET_ENV": "local"}, clear=True)
    def test_get_cluster_nodes_local_discovery(self, mock_get_ip):
        """Test local cluster discovery via Multipass"""
        mock_get_ip.side_effect = lambda name: "10.0.0.1" if name == "local-leader" else None

        nodes = get_cluster_nodes()

        # Should find leader
        assert len(nodes) >= 1
        assert nodes[0]["name"] == "leader"


class TestGetMultipassIP:
    """Test get_multipass_ip function"""

    @patch("subprocess.run")
    def test_get_multipass_ip_success(self, mock_run):
        """Test successful Multipass IP retrieval"""
        mock_run.return_value = MagicMock(
            stdout='{"info": {"local-leader": {"ipv4": ["10.0.0.1"]}}}', returncode=0
        )

        ip = get_multipass_ip("local-leader")
        assert ip == "10.0.0.1"

    @patch("subprocess.run")
    def test_get_multipass_ip_failure(self, mock_run):
        """Test Multipass IP retrieval failure"""
        mock_run.side_effect = subprocess.CalledProcessError(1, "multipass")

        ip = get_multipass_ip("local-leader")
        assert ip is None

    @patch("subprocess.run")
    def test_get_multipass_ip_not_found(self, mock_run):
        """Test Multipass VM not found"""
        mock_run.side_effect = FileNotFoundError()

        ip = get_multipass_ip("local-leader")
        assert ip is None


class TestDeployJob:
    """Test deploy_job function"""

    @patch("subprocess.run")
    @patch.dict(os.environ, {"NOMAD_ADDR": "http://localhost:4646"}, clear=True)
    def test_deploy_job_success(self, mock_run):
        """Test successful job deployment"""
        mock_run.return_value = MagicMock(stdout="Job 'test-job' in state: running\n", returncode=0)

        job_id = deploy_job("/path/to/job.nomad", {"app_name": "test"})

        assert job_id == "test-job"
        mock_run.assert_called_once()

    @patch("subprocess.run")
    def test_deploy_job_failure(self, mock_run):
        """Test job deployment failure"""
        mock_run.side_effect = subprocess.CalledProcessError(1, "nomad")

        with pytest.raises(subprocess.CalledProcessError):
            deploy_job("/path/to/job.nomad", {})

    @patch("subprocess.run")
    @patch.dict(os.environ, {}, clear=True)
    def test_deploy_job_default_nomad_addr(self, mock_run):
        """Test default Nomad address"""
        mock_run.return_value = MagicMock(stdout="Job 'test-job' in state: running\n", returncode=0)

        deploy_job("/path/to/job.nomad", {})

        # Verify default address used
        call_args = mock_run.call_args[0][0]
        assert "http://localhost:4646" in call_args


class TestWaitForAllocation:
    """Test wait_for_allocation function"""

    @patch("subprocess.run")
    @patch("time.sleep")
    @patch.dict(os.environ, {}, clear=True)
    def test_wait_for_allocation_success(self, mock_sleep, mock_run):
        """Test successful allocation wait"""
        mock_run.return_value = MagicMock(
            stdout='{"Status": {"Allocations": [{"ClientStatus": "running"}]}}',
            returncode=0,
        )

        result = wait_for_allocation("test-job", expected_count=1, timeout=30)

        assert result is True

    @patch("subprocess.run")
    @patch("time.sleep")
    @patch("time.time", side_effect=[0, 10, 20, 30, 40])
    def test_wait_for_allocation_timeout(self, mock_time, mock_sleep, mock_run):
        """Test allocation wait timeout"""
        # Return non-running status
        mock_run.return_value = MagicMock(
            stdout='{"Status": {"Allocations": [{"ClientStatus": "pending"}]}}',
            returncode=0,
        )

        result = wait_for_allocation("test-job", expected_count=1, timeout=30)

        assert result is False


class TestGetAllocationNodes:
    """Test get_allocation_nodes function"""

    @patch("subprocess.run")
    @patch.dict(os.environ, {}, clear=True)
    def test_get_allocation_nodes_success(self, mock_run):
        """Test successful allocation node mapping"""
        mock_run.return_value = MagicMock(
            stdout="["
            '{"ID": "alloc-1", "NodeName": "worker-1"}, '
            '{"ID": "alloc-2", "NodeName": "worker-2"}'
            "]",
            returncode=0,
        )

        alloc_nodes = get_allocation_nodes("test-job")

        assert alloc_nodes == {"alloc-1": "worker-1", "alloc-2": "worker-2"}

    @patch("subprocess.run")
    def test_get_allocation_nodes_failure(self, mock_run):
        """Test allocation node retrieval failure"""
        mock_run.side_effect = subprocess.CalledProcessError(1, "nomad")

        alloc_nodes = get_allocation_nodes("test-job")

        assert alloc_nodes == {}


class TestVerifyServiceDiscovery:
    """Test verify_service_discovery function"""

    @patch("subprocess.run")
    @patch.dict(os.environ, {}, clear=True)
    def test_verify_service_discovery_success(self, mock_run):
        """Test successful service discovery"""
        mock_run.return_value = MagicMock(
            stdout="["
            '{"ServiceAddress": "10.0.0.1", "ServiceName": "api"}, '
            '{"ServiceAddress": "10.0.0.2", "ServiceName": "api"}'
            "]",
            returncode=0,
        )

        ips = verify_service_discovery("api")

        assert ips == ["10.0.0.1", "10.0.0.2"]

    @patch("subprocess.run")
    def test_verify_service_discovery_failure(self, mock_run):
        """Test service discovery failure"""
        mock_run.side_effect = subprocess.CalledProcessError(1, "curl")

        ips = verify_service_discovery("api")

        assert ips == []


class TestCheckTailscaleMesh:
    """Test check_tailscale_mesh function"""

    @patch("subprocess.run")
    def test_check_tailscale_mesh_success(self, mock_run):
        """Test healthy Tailscale mesh"""
        with patch.dict(
            os.environ,
            {"E2E_LEADER_IP": "10.0.0.1", "E2E_WORKER_IPS": "10.0.0.2"},
            clear=True,
        ):
            mock_run.return_value = MagicMock(returncode=0)

            result = check_tailscale_mesh()

            assert result is True

    @patch("mesh.verification.e2e_multi_node_scenarios.test_utils.get_cluster_nodes")
    def test_check_tailscale_mesh_insufficient_nodes(self, mock_get_nodes):
        """Test mesh check with insufficient nodes"""
        # Return only leader (insufficient for mesh)
        mock_get_nodes.return_value = [{"name": "leader", "ip": "10.0.0.1", "role": "server"}]

        result = check_tailscale_mesh()

        assert result is False

    @patch("subprocess.run")
    def test_check_tailscale_mesh_failure(self, mock_run):
        """Test Tailscale mesh failure"""
        with patch.dict(
            os.environ,
            {"E2E_LEADER_IP": "10.0.0.1", "E2E_WORKER_IPS": "10.0.0.2"},
            clear=True,
        ):
            mock_run.side_effect = subprocess.CalledProcessError(1, "ssh")

            result = check_tailscale_mesh()

            assert result is False


class TestCleanupJob:
    """Test cleanup_job function"""

    @patch("subprocess.run")
    @patch.dict(os.environ, {}, clear=True)
    def test_cleanup_job_success(self, mock_run):
        """Test successful job cleanup"""
        mock_run.return_value = MagicMock(returncode=0)

        result = cleanup_job("test-job", purge=True)

        assert result is True

    @patch("subprocess.run")
    def test_cleanup_job_failure(self, mock_run):
        """Test job cleanup failure"""
        mock_run.side_effect = subprocess.CalledProcessError(1, "nomad")

        result = cleanup_job("test-job")

        assert result is False

    @patch("subprocess.run")
    @patch.dict(os.environ, {}, clear=True)
    def test_cleanup_job_no_purge(self, mock_run):
        """Test job cleanup without purge"""
        mock_run.return_value = MagicMock(returncode=0)

        cleanup_job("test-job", purge=False)

        # Verify no -purge flag in command
        call_args = mock_run.call_args[0][0]
        assert "-purge" not in call_args


class TestJobTemplate:
    """Test job template validation"""

    def test_test_job_template_exists(self):
        """Test that test job template exists"""
        from pathlib import Path

        template_path = Path(__file__).parent / "test-web-service.nomad.hcl"
        assert template_path.exists(), "Test job template not found"

    def test_test_job_template_content(self):
        """Test that test job template contains required sections"""
        from pathlib import Path

        template_path = Path(__file__).parent / "test-web-service.nomad.hcl"
        content = template_path.read_text()

        # Check for job definition
        assert 'job "test-web-service"' in content
        assert 'variable "count"' in content
        assert 'variable "job_id"' in content

        # Check for spread stanza
        assert "spread {" in content
        assert 'attribute = "${node.unique.name}"' in content

        # Check for service registration
        assert "service {" in content
        assert "check {" in content

        # Check for task configuration
        assert 'task "server"' in content
        assert 'driver = "docker"' in content


class TestCheckTraefikRouting:
    @patch("requests.get")
    def test_check_traefik_routing_success(self, mock_get):
        mock_response = MagicMock()
        mock_response.status_code = 200
        mock_response.text = "<html>E2E Test</html>"
        mock_get.return_value = mock_response

        response = check_traefik_routing("10.0.0.1", "test-web-service.localhost")

        assert response.status_code == 200
        mock_get.assert_called_once_with(
            "http://10.0.0.1:80/",
            headers={"Host": "test-web-service.localhost"},
            timeout=10,
        )

    @patch("requests.get")
    def test_check_traefik_routing_custom_port(self, mock_get):
        mock_response = MagicMock()
        mock_response.status_code = 200
        mock_get.return_value = mock_response

        check_traefik_routing("10.0.0.1", "svc.example.com", port=8080, timeout=30)

        mock_get.assert_called_once_with(
            "http://10.0.0.1:8080/",
            headers={"Host": "svc.example.com"},
            timeout=30,
        )

    @patch("requests.get")
    def test_check_traefik_routing_non_200(self, mock_get):
        mock_response = MagicMock()
        mock_response.status_code = 502
        mock_get.return_value = mock_response

        response = check_traefik_routing("10.0.0.1", "test-web-service.localhost")

        assert response.status_code == 502

    @patch("requests.get")
    def test_check_traefik_routing_timeout(self, mock_get):
        mock_get.side_effect = requests.Timeout("connection timed out")

        with pytest.raises(requests.Timeout):
            check_traefik_routing("10.0.0.1", "test-web-service.localhost")
