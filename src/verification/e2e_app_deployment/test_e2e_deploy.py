"""
Feature: E2E App Deployment Verification
Implementation of end-to-end testing for application deployment.
"""

import pytest
import os
import time
import requests
import subprocess
import json

# Configuration (KISS: Load from Env)
LEADER_IP = os.getenv("E2E_LEADER_IP")
TARGET_ENV = os.getenv("E2E_TARGET_ENV", "local")

def get_leader_ip():
    """
    Retrieves the leader IP based on the target environment.
    If E2E_LEADER_IP is set, use it. Otherwise, try to discover it.
    """
    if LEADER_IP:
        return LEADER_IP
    
    if TARGET_ENV == "local":
        try:
            # Legacy support for multipass local dev
            res = subprocess.run(
                ["multipass", "info", "local-leader", "--format", "json"],
                capture_output=True, text=True, check=True
            )
            return json.loads(res.stdout)["info"]["local-leader"]["ipv4"][0]
        except (subprocess.CalledProcessError, FileNotFoundError):
            pytest.skip("Multipass not found or cluster not running.")
    
    # TODO: Add AWS/Pulumi stack output lookup here for cloud envs
    pytest.fail("Could not determine Leader IP. Set E2E_LEADER_IP or ensure local cluster is running.")

def test_marketing_site_deployment():
    """
    Scenario: Deploy & Reach
    1. Triggers deployment (simulated or real).
    2. Verifies HTTP access via Traefik Ingress.
    """
    leader_ip = get_leader_ip()
    print(f"\n🚀 Testing against Leader IP: {leader_ip}")

    # In a real pipeline, the app might already be deployed by a previous stage.
    # If we need to trigger deploy, we would call the src/workloads tools here.
    # For this E2E, we assume the 'marketing-site' job is intended to be running.
    
    url = f"http://{leader_ip}:80" 
    headers = {"Host": "marketing-site.localhost"} # Matches Traefik rule
    
    print(f"⏳ Verification Loop: GET {url} with Host={headers['Host']}")
    
    start_time = time.time()
    timeout = 60 # seconds
    
    while time.time() - start_time < timeout:
        try:
            resp = requests.get(url, headers=headers, timeout=2)
            if resp.status_code == 200:
                print("✅ App returned HTTP 200")
                # Verify content
                if "Hello World" in resp.text:
                    print("✅ Content verified: 'Hello World' found.")
                    return
                else:
                    print(f"⚠️  HTTP 200 received, but content mismatch. Body preview: {resp.text[:50]}...")
            else:
                print(f"⏳ Service returned HTTP {resp.status_code}")
        except requests.exceptions.RequestException as e:
            print(f"⏳ Connection failed: {e}")
            
        time.sleep(3)
        
    pytest.fail(f"Marketing site failed to pass verification within {timeout}s")
