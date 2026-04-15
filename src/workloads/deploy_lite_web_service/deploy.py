import argparse
import os
import subprocess
import sys
from typing import Optional

from src.workloads.deploy_lite_ingress.route_manager import RouteManager


def deploy_lite_web_service(
    app_name: str,
    image: str,
    image_tag: str = "latest",
    port: int = 8080,
    domain: Optional[str] = None,
    cpu: int = 100,
    memory: int = 128,
    datacenter: str = "dc1",
    nomad_addr: Optional[str] = None,
    caddy_admin_addr: str = "http://127.0.0.1:2019",
) -> bool:
    script_dir = os.path.dirname(os.path.abspath(__file__))
    job_file = os.path.join(script_dir, "lite_web_service.nomad.hcl")

    if not os.path.exists(job_file):
        print(f"Error: Job file not found: {job_file}")
        return False

    cmd = ["nomad", "job", "run"]

    if nomad_addr:
        cmd.extend(["-address", nomad_addr])

    cmd.extend(
        [
            "-var",
            f"app_name={app_name}",
            "-var",
            f"image={image}",
            "-var",
            f"image_tag={image_tag}",
            "-var",
            f"port={port}",
            "-var",
            f"domain={domain or ''}",
            "-var",
            f"cpu={cpu}",
            "-var",
            f"memory={memory}",
            "-var",
            f"datacenter={datacenter}",
            job_file,
        ]
    )

    try:
        print("=" * 60)
        print(f"Deploying lite web service: {app_name}")
        print("=" * 60)
        print(f"Image: {image}:{image_tag}")
        print(f"Port: {port}")
        print(f"Domain: {domain or '(none)'}")
        print(f"CPU: {cpu} MHz")
        print(f"Memory: {memory} MB")
        print(f"Datacenter: {datacenter}")
        print("=" * 60)

        result = subprocess.run(cmd, capture_output=True, text=True, check=True)
        print(result.stdout)

        if domain:
            print(f"\nRegistering route with Caddy: {domain}")
            manager = RouteManager(caddy_admin_addr=caddy_admin_addr)
            route_ok = manager.add_route(domain, "127.0.0.1", port)
            if route_ok:
                print(f"Route registered: {domain} -> 127.0.0.1:{port}")
            else:
                print(f"Warning: Failed to register Caddy route for {domain}")

        print()
        print("=" * 60)
        print(f"Lite web service '{app_name}' deployed successfully!")
        print("=" * 60)
        print()
        print("Next steps:")
        print(f"  1. Verify: nomad job status {app_name}")
        print(f"  2. Logs: nomad alloc logs -job {app_name}")
        print()

        return True
    except subprocess.CalledProcessError as e:
        print(f"Error deploying lite web service:")
        print(e.stderr)
        return False


def main():
    parser = argparse.ArgumentParser(
        description="Deploy a lite web service to Nomad (no Consul/Traefik)"
    )
    parser.add_argument("--app-name", required=True, help="Application name")
    parser.add_argument("--image", required=True, help="Docker image")
    parser.add_argument(
        "--image-tag", default="latest", help="Image tag (default: latest)"
    )
    parser.add_argument(
        "--port", type=int, default=8080, help="Application port (default: 8080)"
    )
    parser.add_argument("--domain", default=None, help="Domain for Caddy routing")
    parser.add_argument(
        "--cpu", type=int, default=100, help="CPU in MHz (default: 100)"
    )
    parser.add_argument(
        "--memory", type=int, default=128, help="Memory in MB (default: 128)"
    )
    parser.add_argument(
        "--datacenter", default="dc1", help="Nomad datacenter (default: dc1)"
    )
    parser.add_argument("--nomad-addr", default=None, help="Nomad server address")
    parser.add_argument(
        "--caddy-admin-addr",
        default="http://127.0.0.1:2019",
        help="Caddy admin address",
    )

    args = parser.parse_args()

    success = deploy_lite_web_service(
        app_name=args.app_name,
        image=args.image,
        image_tag=args.image_tag,
        port=args.port,
        domain=args.domain,
        cpu=args.cpu,
        memory=args.memory,
        datacenter=args.datacenter,
        nomad_addr=args.nomad_addr,
        caddy_admin_addr=args.caddy_admin_addr,
    )

    sys.exit(0 if success else 1)


if __name__ == "__main__":
    main()
