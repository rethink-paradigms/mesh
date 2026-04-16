import os
import subprocess
from unittest.mock import patch, MagicMock

import pytest

TEMPLATE_DIR = os.path.join(os.path.dirname(__file__), "lite_web_service.nomad.hcl")


def _read_template():
    with open(TEMPLATE_DIR, "r") as f:
        return f.read()


class TestNomadJobTemplate:
    def test_nomad_job_template_valid_syntax(self):
        content = _read_template()
        assert 'job "${var.app_name}"' in content
        assert 'variable "app_name"' in content
        assert 'variable "image"' in content
        assert 'variable "image_tag"' in content
        assert 'variable "port"' in content
        assert 'variable "domain"' in content
        assert 'variable "cpu"' in content
        assert 'variable "memory"' in content
        assert 'variable "datacenter"' in content

    def test_template_has_no_consul_tags(self):
        content = _read_template()
        assert "consul" not in content.lower() or "datacenter" in content

    def test_template_has_no_traefik_tags(self):
        content = _read_template()
        assert "traefik" not in content.lower()

    def test_template_has_nomad_native_service(self):
        content = _read_template()
        assert "service {" in content
        assert "name = var.app_name" in content
        assert 'port = "http"' in content
        assert "traefik.enable" not in content

    def test_template_variables_present(self):
        content = _read_template()
        assert 'variable "app_name"' in content
        assert 'variable "image"' in content
        assert 'variable "image_tag"' in content
        assert 'variable "port"' in content
        assert 'variable "domain"' in content
        assert 'variable "cpu"' in content
        assert 'variable "memory"' in content
        assert 'variable "datacenter"' in content

    def test_template_uses_host_network(self):
        content = _read_template()
        assert 'mode = "host"' in content


class TestDeploy:
    @patch("mesh.workloads.deploy_lite_web_service.deploy.os.path.exists")
    def test_deploy_missing_job_file(self, mock_exists):
        mock_exists.return_value = False
        from mesh.workloads.deploy_lite_web_service.deploy import deploy_lite_web_service

        result = deploy_lite_web_service(
            app_name="test-app",
            image="nginx",
        )
        assert result is False

    @patch("mesh.workloads.deploy_lite_web_service.deploy.RouteManager")
    @patch("mesh.workloads.deploy_lite_web_service.deploy.subprocess.run")
    @patch("mesh.workloads.deploy_lite_web_service.deploy.os.path.exists")
    def test_deploy_success(self, mock_exists, mock_run, mock_rm_cls):
        mock_exists.return_value = True
        mock_run.return_value = MagicMock(stdout="Success", returncode=0)
        mock_manager = MagicMock()
        mock_manager.add_route.return_value = True
        mock_rm_cls.return_value = mock_manager

        from mesh.workloads.deploy_lite_web_service.deploy import deploy_lite_web_service

        result = deploy_lite_web_service(
            app_name="test-app",
            image="nginx",
            image_tag="latest",
            port=8080,
            domain="test.example.com",
        )
        assert result is True
        mock_run.assert_called_once()
        mock_manager.add_route.assert_called_once_with(
            "test.example.com", "127.0.0.1", 8080
        )

    @patch("mesh.workloads.deploy_lite_web_service.deploy.RouteManager")
    @patch("mesh.workloads.deploy_lite_web_service.deploy.subprocess.run")
    @patch("mesh.workloads.deploy_lite_web_service.deploy.os.path.exists")
    def test_deploy_success_no_domain(self, mock_exists, mock_run, mock_rm_cls):
        mock_exists.return_value = True
        mock_run.return_value = MagicMock(stdout="Success", returncode=0)

        from mesh.workloads.deploy_lite_web_service.deploy import deploy_lite_web_service

        result = deploy_lite_web_service(
            app_name="test-app",
            image="nginx",
        )
        assert result is True
        mock_rm_cls.assert_not_called()

    @patch("mesh.workloads.deploy_lite_web_service.deploy.subprocess.run")
    @patch("mesh.workloads.deploy_lite_web_service.deploy.os.path.exists")
    def test_deploy_command_failure(self, mock_exists, mock_run):
        mock_exists.return_value = True
        mock_run.side_effect = subprocess.CalledProcessError(
            1, "nomad", stderr="Deployment failed"
        )

        from mesh.workloads.deploy_lite_web_service.deploy import deploy_lite_web_service

        result = deploy_lite_web_service(
            app_name="test-app",
            image="nginx",
        )
        assert result is False

    @patch("mesh.workloads.deploy_lite_web_service.deploy.RouteManager")
    @patch("mesh.workloads.deploy_lite_web_service.deploy.subprocess.run")
    @patch("mesh.workloads.deploy_lite_web_service.deploy.os.path.exists")
    def test_deploy_passes_all_vars(self, mock_exists, mock_run, mock_rm_cls):
        mock_exists.return_value = True
        mock_run.return_value = MagicMock(stdout="Success", returncode=0)

        from mesh.workloads.deploy_lite_web_service.deploy import deploy_lite_web_service

        deploy_lite_web_service(
            app_name="myapp",
            image="myimage",
            image_tag="v1",
            port=3000,
            cpu=200,
            memory=256,
            datacenter="dc2",
            nomad_addr="http://10.0.0.1:4646",
        )

        call_args = mock_run.call_args[0][0]
        assert "app_name=myapp" in call_args
        assert "image=myimage" in call_args
        assert "image_tag=v1" in call_args
        assert "port=3000" in call_args
        assert "cpu=200" in call_args
        assert "memory=256" in call_args
        assert "datacenter=dc2" in call_args
        assert "-address" in call_args
        assert "http://10.0.0.1:4646" in call_args
