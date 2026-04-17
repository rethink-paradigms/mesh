# Mesh

Lightweight infrastructure orchestration platform for multi-cloud container deployment.

[![PyPI Version](https://img.shields.io/pypi/v/rethink-mesh)](https://pypi.org/project/rethink-mesh/)
[![Python](https://img.shields.io/pypi/pyversions/rethink-mesh)](https://pypi.org/project/rethink-mesh/)
[![License](https://img.shields.io/pypi/l/rethink-mesh)](https://github.com/rethink-paradigms/mesh)

**Deploy containers across any cloud. Zero SSH required. Auto-HTTPS. Multi-cloud.**

Mesh turns any collection of VMs into a single unified computer. Deploy to AWS, DigitalOcean, Google Cloud, or 13+ cloud providers with one command.

---

## Quick Start

```bash
# Install
pip install rethink-mesh

# Initialize a cluster
mesh init

# Deploy an application
mesh deploy my-app --image nginx:latest

# Check status
mesh status

# View logs
mesh logs my-app --follow
```

**From install to running:** ~5 minutes

---

## What It Does

* **Multi-cloud deployment** — Run on AWS, DigitalOcean, Google Cloud, and 10+ other providers from one CLI
* **Zero-SSH deployment** — Declarative infrastructure means no manual server access required
* **Auto-HTTPS** — Let's Encrypt certificates provisioned automatically for all services
* **Lightweight control plane** — ~530MB RAM vs 2GB+ for Kubernetes
* **Cost-effective** — From $8/month (single VM) to $25/month (3-node cluster)

---

## Installation

### Prerequisites

* **Python 3.11 or later**
* **Docker** (for local development, optional for cloud deployments)
* **Cloud account** — AWS, DigitalOcean, Google Cloud, or any supported provider
* **Tailscale account** — Free tier sufficient for mesh networking

### Install

```bash
pip install rethink-mesh
```

### Verify Installation

```bash
mesh doctor
```

---

## Configuration

Mesh reads configuration from environment variables. Copy the example file and add your credentials:

```bash
cp .env.example .env
```

### Required Variables

```bash
# Tailscale authentication key (required for all providers)
# Generate at: https://login.tailscale.com/admin/settings/keys
TAILSCALE_KEY=tskey-auth-example-yyyyy

# Choose one cloud provider:
# DigitalOcean
DIGITALOCEAN_API_TOKEN=do_token_xxxxx

# AWS
AWS_ACCESS_KEY_ID=AKIA...
AWS_SECRET_ACCESS_KEY=wJalr...

# Google Cloud
GOOGLE_CREDENTIALS=path/to/service-account.json
GOOGLE_PROJECT=my-project-id
```

See `.env.example` for all 13+ supported providers.

---

## Usage

### Core Commands

**Initialize a cluster**

```bash
mesh init
# Interactive wizard guides you through:
#   • Provider selection (cloud or local)
#   • Region configuration
#   • Instance sizing
#   • Worker count
```

```bash
# Skip prompts with flags
mesh init --provider "Local (Multipass)" --workers 2
mesh init --provider "AWS" --workers 3
```

**Deploy an application**

```bash
# Basic deployment
mesh deploy my-app --image nginx:latest

# With custom configuration
mesh deploy api --image python:3.11 \
  --port 5000 \
  --memory 256 \
  --cpu 200 \
  --domain api.example.com

# Multiple replicas
mesh deploy worker --image my-worker \
  --count 3
```

**Check cluster status**

```bash
mesh status
# Shows:
#   • Cluster health
#   • Node topology
#   • Running applications
#   • Resource utilization

mesh status --compare   # Compare vs Kubernetes
mesh status --roadmap   # Show capability timeline
```

**View logs**

```bash
# List all running jobs
mesh logs

# Stream logs for a specific app
mesh logs my-app --follow

# Last 50 lines, stderr only
mesh logs my-app --tail 50 --stderr
```

**SSH into nodes**

```bash
# List all nodes
mesh ssh

# Connect to a specific node
mesh ssh mesh-leader
mesh ssh mesh-worker-1 --user admin
```

**Destroy a cluster**

```bash
mesh destroy
# → Stops all apps
# → Terminates all nodes
# → Requires confirmation

mesh destroy --cluster my-cluster
```

### Utility Commands

| Command | Description |
|:---|:---|
| `mesh doctor` | Check system prerequisites |
| `mesh demo` | Run full experience in demo mode (no infrastructure) |
| `mesh version` | Show installed version |
| `mesh compare` | Show resource comparison vs Kubernetes |
| `mesh roadmap` | Show capability roadmap |

---

## How It Works

Mesh orchestrates infrastructure across cloud providers with these components:

```
┌─────────────────────────────────────────────┐
│          User / CI/CD                  │
└───────────────────┬─────────────────────┘
                    │ mesh CLI
┌───────────────────┴─────────────────────┐
│  LEADER NODE                            │
│  • Pulumi (provisioning)               │
│  • Tailscale (mesh networking)           │
│  • Nomad (scheduler)                    │
│  • Consul (service discovery)            │
│  • Traefik/Caddy (HTTPS ingress)        │
└───────────────────┬─────────────────────┘
                    │ Tailscale WireGuard mesh
┌───────────────────┴─────────────────────┐
│  WORKER NODES (VMs on any provider)  │
│  • Nomad client                       │
│  • Consul agent                       │
│  • Docker runtime                     │
│  • Application containers             │
└──────────────────────────────────────────┘
```

### Architecture

1. **Pulumi** provisions VMs on any provider via Apache Libcloud (50+ providers supported)
2. **Tailscale** creates encrypted WireGuard mesh across all VMs
3. **Nomad** schedules containers with resource-aware bin-packing
4. **Consul** provides health-checked service discovery
5. **Traefik or Caddy** handles HTTPS ingress with automatic Let's Encrypt

### Technology Stack

| Layer | Component | RAM |
|:---|:---|:---|
| Infrastructure | Pulumi (Python) | 0MB |
| Networking | Tailscale | 20MB |
| Scheduler | Nomad | 80MB |
| Service Discovery | Consul | 50MB |
| Ingress | Traefik | 256MB |
| Ingress (Lite) | Caddy | 20MB |
| Runtime | Docker | 100MB |
| Secrets | Nomad Variables | 0MB |

**Control plane overhead:** ~530MB (full mode) | ~200MB (lite mode)

---

## Supported Providers

| Provider | Status | Regions |
|:---|:---:|:---|
| AWS | ✅ | us-east-1, us-west-2, eu-west-1, ap-south-1 |
| DigitalOcean | ✅ | nyc3, sfo3, ams3, sgp1, lon1, fra1 |
| Google Cloud | ✅ | us-central1, us-east1, europe-west1 |
| Azure | ✅ | eastus, westus2, westeurope |
| Linode | ✅ | us-east, us-central, eu-west |
| Vultr | ✅ | ewr, lax, ams, sgp |
| +7 more providers | ✅ | Varies |

See `.env.example` for complete list of 13+ supported providers.

---

## Deployment Tiers

Mesh automatically activates services based on cluster topology.

| Tier | Topology | Ingress | Service Discovery | RAM | Cost |
|:---|:---|:---|:---|:---:|:---|
| **Lite** | 1 VM | Caddy | Native | ~200MB | ~$8/mo |
| **Standard** | 2+ VMs, 1 region | Caddy | Consul | ~350MB | ~$15/mo |
| **Ingress** | 2+ VMs, 1 region | Traefik | Consul | ~530MB | ~$25/mo |
| **Production** | 3+ VMs, multi-region | Traefik | Consul + WAN | ~530MB+ | ~$50/mo |

---

## Comparison

| Metric | Mesh | Kubernetes | Heroku |
|:---|:---|:---|:---|
| 3-node cluster cost | $25/mo | $72+/mo | $250+/mo |
| Control plane RAM | 530MB | 2GB+ | N/A |
| Setup time | <15 min | 2+ hours | <5 min |
| Multi-cloud | Native | Complex | No |
| Auto HTTPS | Built-in | Manual config | Built-in |
| SSH required | Optional | Often | Never |

---

## Demo Mode

Try Mesh without creating real infrastructure:

```bash
mesh demo
# Simulates:
#   • Cluster initialization
#   • Application deployment
#   • Status viewing
#   No cloud resources created
```

All commands support `--demo` flag for testing without real infrastructure.

---

## Development

### Running Tests

```bash
# Unit and integration tests (fast, no cluster required)
pytest src/mesh -m "not e2e"

# Full test suite (requires running cluster)
pytest src/mesh

# E2E tests only
RUN_E2E=1 ./run_tests.sh
```

### Project Structure

```
src/mesh/
├── cli/                    # CLI commands and UI
│   ├── commands/            # init, deploy, status, logs, ssh, etc.
│   └── ui/                 # Rich-formatted panels and themes
├── infrastructure/          # VM provisioning and networking
│   ├── provision_node/      # Multi-provider VM provisioning
│   ├── boot_consul_nomad/  # Modular boot scripts
│   ├── configure_tailscale/  # Tailscale auth key generation
│   └── providers/           # Libcloud provider implementations
├── workloads/               # Application deployment
│   ├── deploy_app/          # Tier-aware unified deployment
│   ├── deploy_web_service/   # Nomad web app templates
│   ├── deploy_traefik/      # Traefik TLS ingress
│   └── manage_secrets/      # Secret synchronization
└── verification/            # E2E test suites
```

Each directory contains a `CONTEXT.md` with interface contracts and design decisions.

---

## Contributing

Contributions welcome. See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

**Quick start:**

```bash
# Clone and install
git clone https://github.com/rethink-paradigms/mesh.git
cd mesh
pip install -e ".[dev]"

# Run tests
pytest src/mesh -m "not e2e"
```

---

## Plugin Architecture

Extend Mesh with custom commands via Python entry points:

```toml
# In your plugin's pyproject.toml:
[project.entry-points."mesh.plugins"]
my-command = "my_package.cli:register"
```

Enterprise features (GPU, monitoring, backups, AI agent orchestration) register as plugins in the separate `mesh-enterprise` package.

---

## Security

* WireGuard encryption on all mesh traffic via Tailscale
* TLS/HTTPS on all external endpoints via Let's Encrypt
* Docker container isolation with resource limits
* Declarative infrastructure — SSH optional for deployment

---

## License

MIT — see [LICENSE](LICENSE) for details.

---

## Links

* [Documentation](https://github.com/rethink-paradigms/mesh/tree/main/docs)
* [Deployment Guide](https://rethink-paradigms.github.io/mesh/guides/deploy/)
* [Architecture Overview](https://rethink-paradigms.github.io/mesh/architecture/overview/)
* [Comparisons](https://rethink-paradigms.github.io/mesh/comparisons/)
* [FAQ](https://rethink-paradigms.github.io/mesh/faq/)
