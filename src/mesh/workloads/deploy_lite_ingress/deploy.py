import argparse
import os
import subprocess
import sys
from dataclasses import dataclass, field
from typing import Optional


@dataclass
class LiteIngressConfig:
    acme_email: str
    caddy_image: str = "caddy:2"
    memory: int = 25
    cpu: int = 100
    datacenter: str = "dc1"
    log_level: str = "INFO"
    nomad_addr: Optional[str] = None


def deploy_lite_ingress(config: LiteIngressConfig) -> bool:
    script_dir = os.path.dirname(os.path.abspath(__file__))
    job_file = os.path.join(script_dir, "lite_ingress.nomad.hcl")

    if not os.path.exists(job_file):
        print(f"Error: Job file not found: {job_file}")
        return False

    cmd = ["nomad", "job", "run"]

    if config.nomad_addr:
        cmd.extend(["-address", config.nomad_addr])

    cmd.extend(
        [
            "-var",
            f"acme_email={config.acme_email}",
            "-var",
            f"caddy_image={config.caddy_image}",
            "-var",
            f"memory={config.memory}",
            "-var",
            f"cpu={config.cpu}",
            "-var",
            f"datacenter={config.datacenter}",
            "-var",
            f"log_level={config.log_level}",
            job_file,
        ]
    )

    try:
        print("=" * 60)
        print("Deploying Caddy Lite Ingress")
        print("=" * 60)
        print(f"ACME Email: {config.acme_email}")
        print(f"Caddy Image: {config.caddy_image}")
        print(f"Memory: {config.memory} MB")
        print(f"CPU: {config.cpu} MHz")
        print(f"Datacenter: {config.datacenter}")
        print("=" * 60)
        print()

        result = subprocess.run(cmd, capture_output=True, text=True, check=True)
        print(result.stdout)

        print()
        print("=" * 60)
        print("Caddy deployed successfully!")
        print("=" * 60)
        print()
        print("Next steps:")
        print("  1. Verify Caddy is running:")
        print("     nomad job status caddy")
        print("  2. Check Caddy logs:")
        print("     nomad alloc logs -job caddy")
        print("  3. Configure DNS for your domain")
        print("  4. Add routes via RouteManager or Caddyfile")
        print()
        print("Important:")
        print("  - Ensure ports 80 and 443 are accessible from internet")
        print("  - DNS must point to server public IP")
        print("  - Admin API available on port 2019")
        print()
        print("=" * 60)

        return True
    except subprocess.CalledProcessError as e:
        print(f"Error deploying Caddy:")
        print(e.stderr)
        return False


def main():
    parser = argparse.ArgumentParser(description="Deploy Caddy as lite HTTPS ingress")
    parser.add_argument(
        "--acme-email",
        required=True,
        help="Email for Let's Encrypt certificate notifications",
    )
    parser.add_argument(
        "--caddy-image", default="caddy:2", help="Caddy Docker image (default: caddy:2)"
    )
    parser.add_argument(
        "--memory", type=int, default=25, help="Memory allocation in MB (default: 25)"
    )
    parser.add_argument("--cpu", type=int, default=100, help="CPU allocation in MHz (default: 100)")
    parser.add_argument("--datacenter", default="dc1", help="Nomad datacenter name (default: dc1)")
    parser.add_argument(
        "--log-level",
        default="INFO",
        choices=["DEBUG", "INFO", "WARN", "ERROR"],
        help="Caddy log level (default: INFO)",
    )
    parser.add_argument(
        "--nomad-addr", help="Nomad server address (default: http://127.0.0.1:4646)"
    )

    args = parser.parse_args()

    config = LiteIngressConfig(
        acme_email=args.acme_email,
        caddy_image=args.caddy_image,
        memory=args.memory,
        cpu=args.cpu,
        datacenter=args.datacenter,
        log_level=args.log_level,
        nomad_addr=args.nomad_addr,
    )

    success = deploy_lite_ingress(config)
    sys.exit(0 if success else 1)


if __name__ == "__main__":
    main()
