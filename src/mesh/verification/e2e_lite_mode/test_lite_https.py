import pytest
import os
from unittest.mock import patch, MagicMock
from mesh.workloads.deploy_lite_ingress.route_manager import RouteManager
from mesh.workloads.deploy_lite_ingress.deploy import (
    LiteIngressConfig,
    deploy_lite_ingress,
)


class TestLiteHTTPS:
    def test_caddy_route_manager_add_route(self):
        manager = RouteManager("http://127.0.0.1:2019")
        with patch.object(manager, "_request", return_value=[]):
            assert manager.add_route("app.example.com", "127.0.0.1", 8080) is True

    def test_caddy_route_manager_remove_route(self):
        manager = RouteManager("http://127.0.0.1:2019")
        with patch.object(manager, "_request", return_value=[]):
            assert manager.remove_route("app.example.com") is True

    def test_lite_ingress_config_has_acme_email(self):
        config = LiteIngressConfig(acme_email="test@example.com")
        assert config.acme_email == "test@example.com"
        assert config.memory == 25
        assert config.cpu == 100

    def test_caddy_template_has_auto_https(self):
        hcl_path = os.path.join(
            os.path.dirname(__file__),
            "..",
            "..",
            "workloads",
            "deploy_lite_ingress",
            "lite_ingress.nomad.hcl",
        )
        hcl_path = os.path.normpath(hcl_path)
        with open(hcl_path, "r") as f:
            content = f.read()
        assert "443" in content
        assert "80" in content
        assert "redir" in content.lower() or "https" in content

    def test_lite_web_service_deploy_with_domain(self):
        with patch("subprocess.run") as mock_run:
            mock_run.return_value = MagicMock(returncode=0, stdout="Job registered", stderr="")
            with patch.object(RouteManager, "add_route", return_value=True):
                from mesh.workloads.deploy_lite_web_service.deploy import (
                    deploy_lite_web_service,
                )

                result = deploy_lite_web_service(
                    app_name="test-app",
                    image="nginx",
                    domain="test.example.com",
                )
                assert result is True
