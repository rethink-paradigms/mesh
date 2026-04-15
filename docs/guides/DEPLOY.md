# Deployment & CLI Guide

**Last Updated:** 2026-04-02
**Status:** Production Ready (Pulumi/CLI) | Design Phase (`mesh` CLI)

---

## Part 1: Current Deployment Methods

### Lite Mode (Single VM)

For solo projects and minimal budgets (~$8/month), deploy everything on a single VM with Caddy ingress.

```bash
cd src/infrastructure/provision_cloud_cluster
pulumi stack init lite-cluster

pulumi config set provider aws
pulumi config set region us-east-1
pulumi config set leader_size t3.micro
pulumi config set worker_node_count 0

pulumi up
```

This automatically triggers **lite mode** (0 workers). Caddy deploys as a Nomad system job with automatic HTTPS via Let's Encrypt. No Consul or Traefik required.

You can also explicitly set the tier:

```bash
pulumi config set cluster_tier lite
```

Deploy an app in lite mode:

```python
from src.workloads.deploy_app import deploy_app

deploy_app(
    app_name="myapp",
    image="nginx:latest",
    cluster_tier="lite",
    domain="myapp.example.com"
)
```

### Cloud Deployment (Pulumi)

The platform supports 50+ cloud providers through Apache Libcloud.

#### Quickstart: DigitalOcean

```bash
cd src/infrastructure/provision_cloud_cluster
pulumi stack init do-cluster

pulumi config set provider digitalocean
pulumi config set region nyc3
pulumi config set leader_size s-2vcpu-4gb
pulumi config set worker_size s-1vcpu-1gb

pulumi config set --secret tailscale:apiKey sk_your_tailscale_key
pulumi config set --secret digitalocean:token dop_v1_your_do_token

pulumi up
```

#### Quickstart: AWS

```bash
cd src/infrastructure/provision_cloud_cluster
pulumi stack init aws-cluster

pulumi config set provider aws
pulumi config set region us-east-1
pulumi config set leader_size t3.small
pulumi config set worker_size t3.micro

pulumi config set --secret tailscale:apiKey sk_your_tailscale_key
pulumi config set --secret aws:accessKey AKIA...
pulumi config set --secret aws:secretKey your_secret_key

pulumi up
```

#### Quickstart: Google Cloud

```bash
pulumi config set provider gcp
pulumi config set region us-central1
pulumi config set leader_size n1-standard-2
pulumi config set worker_size n1-standard-1
pulumi config set --secret gcp:credentials '{"type": "service_account", ...}'
```

#### Quickstart: Azure

```bash
pulumi config set provider azure
pulumi config set region eastus
pulumi config set leader_size Standard_B2s
pulumi config set worker_size Standard_B1s
pulumi config set --secret azure:clientId your_client_id
pulumi config set --secret azure:clientSecret your_client_secret
```

### Supported Providers

| Provider | Config Value | Credential Environment Variables |
|----------|--------------|----------------------------------|
| **AWS** | `aws` | `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY` |
| **DigitalOcean** | `digitalocean` | `DIGITALOCEAN_API_TOKEN` |
| **Google Cloud** | `gcp` | `GOOGLE_CREDENTIALS` |
| **Azure** | `azure` | `AZURE_CLIENT_ID`, `AZURE_CLIENT_SECRET`, `AZURE_TENANT_ID` |
| **Linode** | `linode` | `LINODE_API_KEY` |
| **Vultr** | `vultr` | `VULTR_API_KEY` |
| **Hetzner** | `hetzner` | `HZCLOUD_API_TOKEN` |
| **UpCloud** | `upcloud` | `UPCLOUD_USERNAME`, `UPCLOUD_PASSWORD` |
| **Scaleway** | `scaleway` | `SCALEWAY_ACCESS_KEY`, `SCALEWAY_SECRET_KEY` |
| **Equinix Metal** | `equinixmetal` | `EQUINIXMETAL_API_KEY`, `EQUINIXMETAL_PROJECT_ID` |

Full list: https://libcloud.readthedocs.io/en/stable/compute/supported_providers.html

### Configuration Reference

**Required:**

| Parameter | Description | Default |
|-----------|-------------|---------|
| `provider` | Cloud provider | `aws` |
| `region` | Provider region | `us-east-1` |
| `leader_size` | Leader node size | `t3.small` |
| `worker_size` | Worker node size | `t3.micro` |

**Optional:**

| Parameter | Description | Default |
|-----------|-------------|---------|
| `leader_node_count` | Number of leader nodes | `1` |
| `worker_node_count` | Number of worker nodes | `1` |
| `cluster_tier` | Cluster tier: `lite`, `standard`, `ingress`, `production` | Auto-detected from node count |

### Size Reference

**DigitalOcean:**

| Size Slug | CPU | RAM | Price/month |
|-----------|-----|-----|-------------|
| `s-1vcpu-1gb` | 1 | 1 GB | $6 |
| `s-2vcpu-2gb` | 2 | 2 GB | $12 |
| `s-2vcpu-4gb` | 2 | 4 GB | $24 |
| `s-4vcpu-8gb` | 4 | 8 GB | $48 |

**AWS:**

| Instance Type | CPU | RAM | Price/month |
|---------------|-----|-----|-------------|
| `t3.nano` | 2 | 0.5 GB | $4 |
| `t3.micro` | 1 | 1 GB | $8 |
| `t3.small` | 2 | 2 GB | $15 |
| `t3.medium` | 2 | 4 GB | $30 |

### Local Development (Multipass)

```bash
# Start local cluster
cd src/infrastructure/provision_local_cluster
python3 cli.py up

# Check status
python3 cli.py status

# Destroy
python3 cli.py down
```

**Requirements:** Multipass (macOS), 3GB free RAM, Tailscale auth key in `.env`.

---

## Part 2: Deploying Applications

### Deploy a Web Service

```bash
nomad job run src/workloads/deploy_web_service/web_service.nomad.hcl \
  -var="app_name=myapp" \
  -var="image=nginx" \
  -var="image_tag=latest" \
  -var="port=80" \
  -var="host_rule=myapp.localhost" \
  -var="domain=example.com" \
  -var="count=1"
```

### Deploy Traefik (TLS/HTTPS)

```python
from src.workloads.deploy_traefik import deploy_traefik

deploy_traefik(
    acme_email="admin@example.com",
    acme_ca_server="letsencrypt-production"  # or "letsencrypt-staging"
)
```

### Deploy Monitoring

```python
from src.workloads.deploy_monitoring import deploy_monitoring_job

deploy_monitoring_job(
    output_type="datadog",  # or "influxdb", "prometheus"
    api_key="your-datadog-key",
    site="us"
)
```

### Deploy GPU Workload

```bash
nomad job run src/workloads/deploy_gpu_service/gpu_service.nomad.hcl \
  -var="app_name=pytorch-training" \
  -var="image=pytorch/pytorch:2.1.0-cuda12.1-runtime" \
  -var="gpu_count=1" \
  -var="memory=16384"
```

### Manage Secrets

```bash
# Sync GitHub Secrets to Nomad Variables
python -m src.workloads.manage_secrets.manage --job myapp --secrets '{"DB_URL": "..."}'
```

---

## Part 3: Future `mesh` CLI (Design Phase)

The `mesh` CLI is a planned interactive tool to reduce onboarding from 2-4 hours to 15 minutes. It is **not yet implemented**.

### Planned Commands

| Command | Purpose | Interactive? |
|---------|---------|-------------|
| `mesh init` | Interactive cluster creation | Yes |
| `mesh deploy` | Application deployment | Partial |
| `mesh status` | Cluster health monitoring | No |
| `mesh logs` | Tail service logs | No |
| `mesh scale` | Add/remove nodes | Partial |
| `mesh doctor` | Health diagnostics | No |
| `mesh check` | Prerequisites check | No |
| `mesh destroy` | Cluster teardown | Yes (confirmation) |

### Planned Onboarding Flow (`mesh init`)

1. **Prerequisites validation** — Check Python, Docker, Pulumi installed
2. **Provider selection** — Interactive menu with 50+ providers
3. **Credential collection** — Validate API tokens, open browser to console
4. **Region selection** — Test latency to each region
5. **Size selection** — Show pricing alongside sizes
6. **Worker count** — How many workers
7. **Tailscale configuration** — Auth key validation
8. **Cluster name** — Name your cluster
9. **Confirmation** — Summary with estimated monthly cost
10. **Provisioning** — Progress bar through all steps
11. **Success** — Show IPs, UIs, next steps

### Planned Architecture

```
src/cli/
├── main.py                    # Typer app definition
├── commands/
│   ├── init.py                # mesh init
│   ├── deploy.py              # mesh deploy
│   ├── status.py              # mesh status
│   ├── logs.py                # mesh logs
│   ├── scale.py               # mesh scale
│   ├── doctor.py              # mesh doctor
│   └── destroy.py             # mesh destroy
├── prompts/
│   ├── provider.py            # Provider selection
│   ├── credentials.py         # Credential collection
│   ├── region.py              # Region selection with latency
│   └── size.py                # Size selection with pricing
└── utils/
    ├── browser.py             # Browser opener
    ├── latency.py             # Region latency testing
    └── pricing.py             # Cost estimation
```

### Planned Dependencies

| Package | Purpose |
|---------|---------|
| **typer** | CLI argument parsing |
| **questionary** | Interactive prompts |
| **rich** | Progress bars, colored output |
| **pulumi-automation** | Programmatic pulumi up |

### Target Metrics

| Metric | Current | Target |
|--------|---------|--------|
| Time to first deploy | 2-4 hours | <15 min |
| Onboarding success rate | ~40% | >95% |
| Documentation reads before deploy | 2000+ words | <500 words |

### Feasibility Assessment

**Verdict: Feasible, 3 weeks to MVP**

The CLI is technically feasible because:
- All infrastructure APIs already exist (`provision_node`, `configure_tailscale`, provider discovery)
- Libcloud provides unified API for all 50+ providers
- Pulumi Automation API enables programmatic provisioning
- Rich ecosystem of CLI tools (Typer, Questionary, Rich) available

**Key Risk:** Pulumi Automation API may have limitations for complex stacks.

**Recommended Approach:** Build MVP with `mesh init` + `mesh deploy hello-world` first (1 week), then expand.

---

*Deployment guide reflects v0.3 capabilities and planned v0.4 `mesh` CLI design.*
