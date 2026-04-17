"""
Deploy Traefik - Python Helper
Deploys Traefik ingress controller with Let's Encrypt ACME to the Nomad cluster.
"""

import argparse
import subprocess
import os
import sys
from typing import Optional

# ACME server URLs
ACME_SERVERS = {
    "letsencrypt": "https://acme-v02.api.letsencrypt.org/directory",
    "letsencrypt-staging": "https://acme-staging-v02.api.letsencrypt.org/directory",
}


def deploy_traefik(
    acme_email: str,
    acme_ca_server: str = "letsencrypt",
    use_tls_challenge: bool = True,
    use_http_challenge: bool = False,
    acme_storage_path: str = "/letsencrypt/acme.json",
    memory: int = 256,
    cpu: int = 200,
    dashboard_enabled: bool = True,
    log_level: str = "INFO",
    nomad_addr: Optional[str] = None,
) -> bool:
    """
    Deploy Traefik ingress controller with Let's Encrypt ACME.

    Args:
        acme_email: Email for Let's Encrypt certificate notifications
        acme_ca_server: ACME server name or URL (default: "letsencrypt")
        use_tls_challenge: Use TLS-ALPN-01 challenge (default: True)
        use_http_challenge: Use HTTP-01 challenge as fallback (default: False)
        acme_storage_path: Path to store ACME certificates (default: "/letsencrypt/acme.json")
        memory: Memory allocation in MB (default: 256)
        cpu: CPU allocation in MHz (default: 200)
        dashboard_enabled: Enable Traefik dashboard (default: True, set False in production)
        log_level: Traefik log level (default: "INFO")
        nomad_addr: Nomad server address (default: http://127.0.0.1:4646)

    Returns:
        True if deployment succeeded, False otherwise
    """
    script_dir = os.path.dirname(os.path.abspath(__file__))
    job_file = os.path.join(script_dir, "traefik.nomad.hcl")

    if not os.path.exists(job_file):
        print(f"Error: Job file not found: {job_file}")
        return False

    # Resolve ACME server URL
    acme_server_url = ACME_SERVERS.get(acme_ca_server, acme_ca_server)

    # Build nomad job run command
    cmd = ["nomad", "job", "run"]

    if nomad_addr:
        cmd.extend(["-address", nomad_addr])

    cmd.extend(
        [
            "-var",
            f"acme_email={acme_email}",
            "-var",
            f"acme_ca_server={acme_server_url}",
            "-var",
            f"use_tls_challenge={str(use_tls_challenge).lower()}",
            "-var",
            f"use_http_challenge={str(use_http_challenge).lower()}",
            "-var",
            f"acme_storage_path={acme_storage_path}",
            "-var",
            f"memory={memory}",
            "-var",
            f"cpu={cpu}",
            "-var",
            f"dashboard_enabled={str(dashboard_enabled).lower()}",
            "-var",
            f"log_level={log_level}",
            job_file,
        ]
    )

    try:
        print("=" * 60)
        print("Deploying Traefik with Let's Encrypt")
        print("=" * 60)
        print(f"ACME Email: {acme_email}")
        print(f"ACME Server: {acme_server_url}")
        print(f"TLS Challenge: {use_tls_challenge}")
        print(f"HTTP Challenge: {use_http_challenge}")
        print(f"Memory: {memory} MB")
        print(f"CPU: {cpu} MHz")
        print("=" * 60)
        print()

        result = subprocess.run(cmd, capture_output=True, text=True, check=True)
        print(result.stdout)

        print()
        print("=" * 60)
        print("Traefik deployed successfully!")
        print("=" * 60)
        print()
        print("Next steps:")
        print("  1. Verify Traefik is running:")
        print("     nomad job status traefik")
        print("  2. Check Traefik logs:")
        print("     nomad alloc logs -job traefik")
        print("  3. Access dashboard (if enabled):")
        print("     http://<leader-ip>:8080")
        print("  4. Configure DNS for your domain")
        print("  5. Update web services to use HTTPS")
        print()
        print("Important:")
        print("  - Ensure ports 80 and 443 are accessible from internet")
        print("  - DNS must point to Traefik public IP")
        print("  - Use staging environment for testing first")
        print("  - Certificates auto-generate on first HTTPS request")
        print()
        print("=" * 60)

        return True
    except subprocess.CalledProcessError as e:
        print(f"Error deploying Traefik:")
        print(e.stderr)
        return False


def main():
    """CLI entry point for deploying Traefik."""
    parser = argparse.ArgumentParser(
        description="Deploy Traefik ingress controller with Let's Encrypt"
    )
    parser.add_argument(
        "--acme-email", required=True, help="Email for Let's Encrypt certificate notifications"
    )
    parser.add_argument(
        "--acme-ca-server",
        default="letsencrypt",
        choices=["letsencrypt", "letsencrypt-staging"],
        help="ACME server (default: letsencrypt, use staging for testing)",
    )
    parser.add_argument(
        "--use-http-challenge",
        action="store_true",
        help="Use HTTP-01 challenge (fallback, requires port 80)",
    )
    parser.add_argument(
        "--no-tls-challenge", action="store_true", help="Disable TLS-ALPN-01 challenge"
    )
    parser.add_argument(
        "--acme-storage-path",
        default="/letsencrypt/acme.json",
        help="Path to store ACME certificates (default: /letsencrypt/acme.json)",
    )
    parser.add_argument(
        "--memory", type=int, default=256, help="Memory allocation in MB (default: 256)"
    )
    parser.add_argument("--cpu", type=int, default=200, help="CPU allocation in MHz (default: 200)")
    parser.add_argument(
        "--no-dashboard",
        action="store_true",
        help="Disable Traefik dashboard (recommended for production)",
    )
    parser.add_argument(
        "--log-level",
        default="INFO",
        choices=["DEBUG", "INFO", "WARN", "ERROR"],
        help="Traefik log level (default: INFO)",
    )
    parser.add_argument(
        "--nomad-addr", help="Nomad server address (default: http://127.0.0.1:4646)"
    )

    args = parser.parse_args()

    success = deploy_traefik(
        acme_email=args.acme_email,
        acme_ca_server=args.acme_ca_server,
        use_tls_challenge=not args.no_tls_challenge,
        use_http_challenge=args.use_http_challenge,
        acme_storage_path=args.acme_storage_path,
        memory=args.memory,
        cpu=args.cpu,
        dashboard_enabled=not args.no_dashboard,
        log_level=args.log_level,
        nomad_addr=args.nomad_addr,
    )

    sys.exit(0 if success else 1)


if __name__ == "__main__":
    main()
