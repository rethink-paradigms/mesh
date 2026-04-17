"""
Tests for Feature: E2E App Deployment Verification (Logic Check)
"""

import pytest
from unittest.mock import patch, MagicMock
from _pytest.outcomes import Failed
from test_e2e_deploy import test_marketing_site_deployment


@patch("test_e2e_deploy.get_leader_ip")
@patch("test_e2e_deploy.requests.get")
@patch("test_e2e_deploy.time.sleep")  # Don't actually sleep
def test_marketing_site_success(mock_sleep, mock_get, mock_ip):
    """
    Verify the test passes when the app returns 200 OK with correct content.
    """
    mock_ip.return_value = "1.2.3.4"

    # Mock successful response
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.text = "<html>Hello World</html>"
    mock_get.return_value = mock_response

    # Should not raise exception
    test_marketing_site_deployment()


@patch("test_e2e_deploy.get_leader_ip")
@patch("test_e2e_deploy.requests.get")
@patch("test_e2e_deploy.time.sleep")
def test_marketing_site_failure_timeout(mock_sleep, mock_get, mock_ip):
    """
    Verify the test fails if the app never becomes healthy.
    """
    mock_ip.return_value = "1.2.3.4"

    # Mock failure response (503 Service Unavailable)
    mock_response = MagicMock()
    mock_response.status_code = 503
    mock_get.return_value = mock_response

    # Reduce timeout loop for test speed
    with patch("test_e2e_deploy.time.time", side_effect=[0, 10, 20, 70, 80, 90]):  # Jump past 60s
        with pytest.raises(Failed):
            test_marketing_site_deployment()
