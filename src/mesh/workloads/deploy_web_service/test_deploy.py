"""
Tests for Feature: Deploy Web Service
"""

import os


def test_nomad_template_variables():
    """
    Test_NomadTemplate_Variables: Verify all required variables are defined in the HCL.
    """
    feature_dir = os.path.dirname(__file__)
    hcl_path = os.path.join(feature_dir, "web_service.nomad.hcl")

    with open(hcl_path, "r") as f:
        content = f.read()

    required_vars = ["app_name", "image", "host_rule", "count", "port"]
    for var in required_vars:
        assert f'variable "{var}"' in content, f"Missing variable definition: {var}"


def test_nomad_template_traefik_tags():
    """
    Test_NomadTemplate_TraefikTags: Verify Traefik tags are correctly interpolated.
    """
    feature_dir = os.path.dirname(__file__)
    hcl_path = os.path.join(feature_dir, "web_service.nomad.hcl")

    with open(hcl_path, "r") as f:
        content = f.read()

    assert "traefik.enable=true" in content
    assert "traefik.http.routers.${var.app_name}.rule=Host(`${var.host_rule}`)" in content


def test_nomad_template_secret_block():
    """
    Test_NomadTemplate_SecretTemplate: Verify the secret template block is present.
    """
    feature_dir = os.path.dirname(__file__)
    hcl_path = os.path.join(feature_dir, "web_service.nomad.hcl")

    with open(hcl_path, "r") as f:
        content = f.read()

    assert 'destination = "secrets/app.env"' in content
    assert "env" not in content.split("template {")[-1].split("}")[0] or "env = true" not in content
    assert "nomadVar" in content
    assert "SECRETS_FILE" in content
    assert "perms" in content
