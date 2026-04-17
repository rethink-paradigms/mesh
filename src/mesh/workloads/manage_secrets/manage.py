"""
Feature: Manage Secrets
Implementation details for syncing secrets to Nomad.
"""

import os
import requests
import argparse
from typing import Dict, Optional
from dotenv import dotenv_values

from mesh.infrastructure.config.env import EnvVars, get_env


class SecretsManager:
    """
    Manages the interaction with the Nomad Variables API to securely store application secrets.
    """

    def __init__(
        self, nomad_addr: Optional[str] = None, nomad_token: Optional[str] = None
    ):
        """
        Initialize the manager with connection details.

        Args:
            nomad_addr (str, optional): The base URL of the Nomad server. Defaults to env NOMAD_ADDR.
            nomad_token (str, optional): The authentication token. Defaults to env NOMAD_TOKEN.
        """
        # KISS: Load from env if not provided, strict dependency on configuration.
        self.nomad_addr = nomad_addr or get_env(EnvVars.NOMAD_ADDR)
        self.nomad_token = nomad_token or get_env(EnvVars.NOMAD_TOKEN)

    def sync_secrets(self, job_name: str, secrets: Dict[str, str]) -> bool:
        """
        Syncs a dictionary of secrets to the Nomad variables path for a specific job.

        Path used: v1/var/nomad/jobs/{job_name}

        Args:
            job_name (str): The name of the Nomad job (used as variable path).
            secrets (Dict[str, str]): Key-value map of secrets.

        Returns:
            bool: True if successful, False otherwise.
        """
        if not self.nomad_addr:
            print("❌ Error: NOMAD_ADDR is not set.")
            return False

        # Prepare request headers with auth token
        headers = {}
        if self.nomad_token:
            headers["X-Nomad-Token"] = self.nomad_token

        # Construct the API endpoint
        url = f"{self.nomad_addr}/v1/var/nomad/jobs/{job_name}"

        # Payload format for Nomad Variables
        payload = {"Items": secrets}

        try:
            # KISS: Simple PUT request to upsert variables
            response = requests.put(url, headers=headers, json=payload, timeout=5)

            if response.status_code == 200:
                print(f"✅ Secrets synced successfully for job '{job_name}'.")
                return True
            else:
                print(
                    f"❌ Failed to sync secrets. Status: {response.status_code}, Body: {response.text}"
                )
                return False

        except requests.exceptions.RequestException as e:
            print(f"❌ Network error connecting to Nomad: {e}")
            return False


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="Sync secrets to Nomad Variables.")
    parser.add_argument(
        "--env-file", required=True, help="Path to .env file containing secrets."
    )
    parser.add_argument(
        "--job", required=True, help="Nomad job name (used as variable path)."
    )
    parser.add_argument("--address", help="Nomad server address.")
    parser.add_argument("--token", help="Nomad ACL token.")
    # Legacy argument support (ignored but allowed to prevent breaking old calls if any)
    parser.add_argument("--source", help="Ignored (legacy compatibility).")

    args = parser.parse_args()

    # Load secrets from file
    if not os.path.exists(args.env_file):
        print(f"❌ Error: Env file not found at {args.env_file}")
        exit(1)

    secrets = dotenv_values(args.env_file)
    # Filter out None values just in case
    clean_secrets = {k: v for k, v in secrets.items() if v is not None}

    if not clean_secrets:
        print("⚠️  Warning: No secrets found in the provided file.")

    manager = SecretsManager(nomad_addr=args.address, nomad_token=args.token)
    success = manager.sync_secrets(args.job, clean_secrets)

    if not success:
        exit(1)
