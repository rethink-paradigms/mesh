# Domain: Application Deployment

**Description:**
Focuses on the deployment, scaling, and operational aspects of applications running on the mesh, including CI/CD integration and ingress management.

## 🧩 Public Interface

| Feature | Input | Output | Description |
|:---|:---|:---|:---|
| `DeployWebService` | app_name, image, [image_tag], [count], [port], [host_rule], [cpu], [memory], [datacenter], [domain] | service_name | Standardized web application deployment via Nomad |
| `DeployTraefik` | acme_email, [acme_ca_server], [acme_tls_challenge], [acme_http_challenge], [memory], [cpu], [nomad_addr] | job_id | Deploys Traefik with Let's Encrypt ACME for TLS/HTTPS |
| `ManageSecrets` | job_name, secrets_json | status | Syncs secrets from GitHub to Nomad Variables API |
| `DeployLiteIngress` | acme_email, [caddy_image], [memory], [cpu], [datacenter], [nomad_addr] | job_id | Deploys Caddy as lightweight HTTPS ingress for lite/standard tiers |
| `DeployLiteWebService` | app_name, image, [image_tag], [port], [domain], [cpu], [memory], [datacenter], [nomad_addr] | service_name | Deploys web services without Consul/Traefik using Nomad native service discovery |
| `DeployApp` | app_name, image, [image_tag], [port], [domain], [cpu], [memory], [datacenter], [cluster_tier], [nomad_addr] | bool | Tier-aware unified deployment dispatcher (auto-detects cluster tier) |

## 📦 Dependencies

- **Nomad (HCL/API)** - Workload scheduler and job management
- **Consul** - Service discovery and health checks
- **Traefik** - HTTP ingress with dynamic Consul catalog integration
- **GitHub Actions** - CI/CD workflows (reusable workflows in `.github/workflows/`)
- **Docker** - Container runtime for all workloads
- **Let's Encrypt ACME** - TLS certificate provisioning
- **Caddy** - Lightweight HTTPS server (lite/standard tiers, ADR-014)

## 🏗 Features

### `deploy_web_service/` - Standard Web Application Deployment
**Purpose:** Provides a standardized Nomad job template for deploying containerized web applications with automatic service discovery and ingress routing.

**Features:**
- Consul service registration with health checks
- Traefik dynamic routing via Consul catalog
- Nomad Variables integration for secrets
- Configurable resource limits (CPU, memory, replicas)

### `deploy_traefik/` - TLS/HTTPS Ingress Controller
**Purpose:** Deploys Traefik as an ingress controller with automatic TLS certificate provisioning via Let's Encrypt ACME.

**Features:**
- Let's Encrypt ACME integration (TLS-ALPN-01 and HTTP-01 challenges)
- Automatic certificate generation and renewal
- HTTP → HTTPS redirect middleware
- Dynamic route configuration from Consul catalog

### `manage_secrets/` - Secret Synchronization
**Purpose:** Handles the secure injection and storage of application secrets via Nomad Variables API.

**Flow:** GitHub Secrets → Nomad Variables → Nomad template stanza → container

### `deploy_lite_ingress/` - Caddy-based Lite HTTPS Ingress
**Purpose:** Lightweight HTTPS ingress using Caddy for single-VM and standard deployments.

**Features:**
- Automatic HTTPS via Caddy
- HTTP→HTTPS redirect
- Caddy admin API for dynamic routes
- 25MB RAM

### `deploy_lite_web_service/` - Lite Mode Web Service Deployment
**Purpose:** Deploys web services without Consul/Traefik dependencies.

**Features:**
- Nomad native service registration
- Caddy route integration

### `deploy_app/` - Tier-Aware Unified Deployment
**Purpose:** Single API for deploying apps regardless of cluster tier.

**Features:**
- Auto-detects cluster tier from Nomad
- Routes to lite or full deployment path

## 🧪 Test Coverage

- **13 tests** in `deploy_traefik/test_deploy.py`
- **Template validation** for all .nomad.hcl files

## 🔗 Dependencies on Other Domains

- **infrastructure/provision_node** - All workloads run on nodes provisioned by this feature
- **infrastructure/boot_consul_nomad** - Depends on Nomad/Consul installed by boot scripts
- **infrastructure/configure_tailscale** - Mesh networking for multi-node deployments

## 📝 Design Decisions

- **Nomad over Kubernetes:** 90% less memory usage (ADR-001)
- **Traefik over NGINX:** Dynamic Consul catalog integration (ADR-003)
- **Caddy for Lite Ingress (ADR-014):** 63% RAM savings over Traefik for single-VM deployments

## 🚀 CI/CD Integration

Reusable GitHub Actions in `.github/workflows/`:
- `reusable-docker-build.yml` - Build and push Docker images
- `reusable-nomad-deploy.yml` - Deploy Nomad jobs, sync secrets

These workflows are designed to be called from application repositories, not from this infra repository.
