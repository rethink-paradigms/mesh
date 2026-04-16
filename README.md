# Mesh

> Deploy containers across any cloud. Zero SSH. Auto-HTTPS. Multi-cloud.

[![Version](https://img.shields.io/badge/version-v0.4-blue)](https://github.com/your-org/mesh)
[![Python](https://img.shields.io/badge/python-3.11+-yellow)](https://github.com/your-org/mesh)
[![License](https://img.shields.io/badge/license-MIT-orange)](https://github.com/your-org/mesh)

Mesh turns any collection of VMs — across AWS, Hetzner, DigitalOcean, and 50+ providers — into a single unified computer. Pulumi provisions. Tailscale connects. Nomad schedules. **Zero SSH required.**

---

## Why Mesh

**10x cheaper** than Heroku. **90% less overhead** than Kubernetes. **Zero SSH.** Multi-cloud by default.

| Metric | Mesh | Kubernetes | Heroku |
|:---|:---|:---|:---|
| 3-node cluster | **$25/mo** | $72+/mo (control plane) | $250+/mo |
| Control plane RAM | **530MB** | 2GB+ | N/A (managed) |
| Setup time | **<5 min** | 2+ hours | <5 min |
| Multi-cloud | **Native** | Complex (federation) | No |
| Auto HTTPS | **Let's Encrypt** | Cert Manager + config | Built-in |
| SSH required | **Never** | Often | Never |

From **$8/month** (single VM) to **$25/month** (3-VM multi-cloud cluster).

---

## Quick Start

```bash
# Install
pip install mesh

# Initialize a cluster (interactive wizard)
mesh init

# Deploy an application
mesh deploy my-app --image nginx:latest --port 80

# View cluster status
mesh status

# View logs
mesh logs my-app --follow
```

### Local Development (5 minutes, $0)

```bash
# Prerequisites: Multipass, Python 3.11+, Tailscale account (free tier)
brew install --cask multipass

# Clone and configure
git clone https://github.com/your-org/mesh.git && cd mesh
cp .env.example .env    # Add your Tailscale auth key

# Launch
mesh init --provider "Local (Multipass)" --workers 2

# Deploy
mesh deploy hello --image nginx:latest
mesh status
```

### Cloud Deployment

```bash
# AWS, Hetzner, DigitalOcean — same workflow
mesh init
# → Select provider, region, sizing via interactive wizard
# → Cluster ready in ~3 minutes
```

---

## Architecture

```
                    ┌─────────────────────┐
                    │   EXTERNAL WORLD    │
                    │   Users / CI/CD     │
                    └──────────┬──────────┘
                               │ HTTP/HTTPS + Pulumi API
                    ┌──────────┴──────────┐
                    │   LEADER NODE (VM-1) │
                    │  Traefik or Caddy    │
                    │  Nomad + Consul      │
                    │  Tailscale + Docker  │
                    └──────────┬──────────┘
                               │
                 Tailscale WireGuard Mesh (100.x.y.z)
                    ┌──────────┴──────────┐
                    │                     │
           ┌────────┴────────┐  ┌─────────┴───────┐
           │  WORKER (VM-2)  │  │  WORKER (VM-N)  │
           │  AWS / Hetzner  │  │  DO / GCP / ... │
           │  App Containers │  │  App Containers │
           └─────────────────┘  └─────────────────┘
```

### How It Works

1. **Pulumi** provisions VMs on any provider (AWS, Hetzner, DO, 50+ others)
2. **Modular boot scripts** install Docker, Nomad, Consul, Tailscale (~2 min)
3. **Tailscale** creates encrypted WireGuard mesh across all VMs
4. **Nomad** schedules containers with resource-aware bin-packing
5. **Consul** provides health-checked service discovery
6. **Traefik or Caddy** handles HTTPS ingress with automatic Let's Encrypt

### Technology Stack

| Layer | Component | RAM | Why |
|:---|:---|:---|:---|
| IaC | Pulumi (Python) | 0MB | Real language vs HCL |
| Mesh | Tailscale | 20MB | Zero-config multi-cloud |
| Scheduler | Nomad | 80MB | K8s needs 1GB+ |
| Discovery | Consul | 50MB | Health-aware DNS |
| Ingress | Traefik | 256MB | Dynamic Consul routing |
| Ingress (Lite) | Caddy | 20MB | Single-VM HTTPS |
| Runtime | Docker | 100MB | Standard isolation |
| Secrets | GitHub Secrets | 0MB | Zero infra overhead |

**Control plane overhead:** ~530MB (full mode) | ~200MB (lite mode)

---

## CLI Commands

| Command | Description |
|:---|:---|
| `mesh init` | Interactive cluster provisioning wizard |
| `mesh deploy <name>` | Deploy a containerized application |
| `mesh status` | View cluster health, nodes, and running apps |
| `mesh logs <app>` | Stream application logs |
| `mesh ssh <node>` | Connect to a cluster node |
| `mesh destroy` | Tear down a cluster |
| `mesh compare` | Show resource comparison vs Kubernetes |

### Extensible via Plugins

Mesh supports a plugin architecture via Python `entry_points`. Third-party and enterprise extensions can add commands without modifying the core:

```toml
# In your plugin's pyproject.toml:
[project.entry-points."mesh.plugins"]
my-command = "my_package.cli:register"
```

---

## Deployment Tiers

The platform automatically activates services based on cluster topology. No manual configuration required.

| | **Lite** | **Standard** | **Ingress** | **Production** |
|:---|:---|:---|:---|:---|
| **Topology** | 1 VM | 2+ VMs, 1 region | 2+ VMs, 1 region | 3+ VMs, multi-region |
| **Ingress** | Caddy (20MB) | Caddy (20MB) | Traefik (256MB) | Traefik (256MB) |
| **Service Discovery** | -- | Consul | Consul | Consul + WAN |
| **RAM Overhead** | ~200MB | ~400MB | ~530MB | ~530MB+ |
| **Cost** | ~$8/mo | ~$15/mo | ~$25/mo | ~$50/mo |

---

## Project Structure

```
src/
├── infrastructure/            # Domain: Compute, Network, OS
│   ├── provision_node/        #   Multi-provider VM provisioning (50+ providers)
│   ├── boot_consul_nomad/     #   Modular boot scripts (Jinja2)
│   ├── configure_tailscale/   #   Auth key generation
│   ├── providers/             #   Libcloud provider implementations
│   └── progressive_activation/ #  Tier detection and configuration
├── workloads/                 # Domain: Application Deployment
│   ├── deploy_app/            #   Tier-aware unified deployment API
│   ├── deploy_web_service/    #   Nomad web app templates (Traefik)
│   ├── deploy_lite_web_service/ # Lite web service (Caddy routing)
│   ├── deploy_lite_ingress/   #   Caddy HTTPS ingress controller
│   ├── deploy_traefik/        #   Traefik TLS ingress controller
│   └── manage_secrets/        #   GitHub Secrets to Nomad sync
├── verification/              # Domain: System Testing
│   ├── e2e_app_deployment/    #   Full cluster deployment tests
│   ├── e2e_lite_mode/         #   Lite mode E2E validation
│   └── e2e_multi_node_scenarios/ # Multi-node fault tolerance
└── cli/                       # Domain: mesh CLI Tool
    ├── commands/              #   init, status, deploy, destroy, logs, ssh
    ├── plugins.py             #   Plugin discovery (entry_points)
    └── ui/                    #   Rich panels and themes
```

Each feature directory contains a `CONTEXT.md` (interface contract), implementation code, and co-located tests.

---

## Testing

```bash
pytest src/mesh -v -m "not e2e"       # Unit + integration
pytest src/mesh/verification/ -v        # End-to-end (requires running cluster)
```

---

## Security

- WireGuard encryption on all mesh traffic (Tailscale)
- TLS/HTTPS on all external endpoints (Let's Encrypt)
- Docker container isolation with resource limits
- Zero SSH access — all configuration is declarative

---

## License

MIT — see [pyproject.toml](pyproject.toml).
