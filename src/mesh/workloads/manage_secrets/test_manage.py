"""
Tests for Feature: Manage Secrets
"""
from unittest.mock import patch, MagicMock
from manage import SecretsManager

@patch("manage.requests.put")
def test_sync_secrets_success(mock_put):
    """
    Test_SyncSecrets_Success: Verify secrets are successfully sent to Nomad Variables API (HTTP 200).
    """
    # Setup
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_put.return_value = mock_response

    manager = SecretsManager(nomad_addr="http://localhost:4646", nomad_token="secret")
    secrets = {"API_KEY": "12345"}
    
    # Execute
    result = manager.sync_secrets("my-job", secrets)

    # Verify
    assert result is True
    mock_put.assert_called_once_with(
        "http://localhost:4646/v1/var/nomad/jobs/my-job",
        headers={"X-Nomad-Token": "secret"},
        json={"Items": secrets},
        timeout=5
    )

def test_sync_secrets_missing_nomad_addr():
    """
    Test_SyncSecrets_MissingNomadAddr: Verify failure when NOMAD_ADDR env var is missing.
    """
    # Ensure no env var is set for this test instance
    manager = SecretsManager(nomad_addr=None)
    result = manager.sync_secrets("job", {"k": "v"})
    assert result is False

@patch("manage.requests.put")
def test_sync_secrets_api_failure(mock_put):
    """
    Test_SyncSecrets_ApiFailure: Verify error handling when Nomad API returns non-200 status.
    """
    mock_response = MagicMock()
    mock_response.status_code = 500
    mock_response.text = "Internal Server Error"
    mock_put.return_value = mock_response

    manager = SecretsManager(nomad_addr="http://localhost:4646")
    result = manager.sync_secrets("job", {"k": "v"})
    
    assert result is False

@patch("manage.requests.put")
def test_sync_secrets_connection_error(mock_put):
    """
    Test_SyncSecrets_ConnectionError: Verify error handling for network connection issues.
    """
    import requests
    mock_put.side_effect = requests.exceptions.ConnectionError("Connection refused")

    manager = SecretsManager(nomad_addr="http://localhost:4646")
    result = manager.sync_secrets("job", {"k": "v"})
    
    assert result is False

@patch("manage.requests.put")
def test_sync_secrets_headers(mock_put):
    """
    Test_SyncSecrets_Headers: Verify X-Nomad-Token is included in the request headers.
    """
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_put.return_value = mock_response

    token = "my-secret-token"
    manager = SecretsManager(nomad_addr="http://localhost:4646", nomad_token=token)
    manager.sync_secrets("job", {})

    call_args = mock_put.call_args
    headers = call_args[1]["headers"]
    assert headers["X-Nomad-Token"] == token
