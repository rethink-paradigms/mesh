# Research: Daytona OSS Architecture

> Completed: April 23, 2026
> Source: daytonaio/daytona GitHub repo (v0.154.0, latest Mar 20, 2026), official documentation at daytona.io/docs, architecture documentation, API reference, and pricing/limits pages

## Architecture Overview

Daytona is a **control plane + compute plane** architecture designed for running AI-generated code in isolated sandboxes.

### Control Plane Components
- **API Server**: NestJS-based RESTful service (primary entry point for all platform operations)
  - Manages authentication, sandbox lifecycle, snapshots, volumes, and resource allocation
  - Internal processes: snapshot builder, sandbox manager
- **PostgreSQL**: Primary persistent store for metadata and configuration
- **Redis**: Caching, session management, and distributed locking
- **Auth0/OIDC Provider**: Authenticates users and services via OpenID Connect
- **SMTP Server**: Handles email delivery for invitations and notifications
- **PostHog**: Platform analytics and usage metrics
- **Sandbox Manager**: Schedules sandboxes onto runners, reconciles states, enforces lifecycle policies

### Compute Plane Components
- **Runners**: Compute nodes that host multiple sandboxes with dedicated resources
  - Poll control plane API for jobs
  - Execute sandbox operations: create, start, stop, destroy, resize, backup
  - Interact with S3-compatible object storage for snapshot/volume data
- **Sandbox Daemon**: Code execution agent running inside each sandbox
- **Snapshot Store**: Stores sandbox snapshot images (S3-compatible)
- **Volumes**: Persistent storage shared across sandboxes, backed by S3-compatible object storage

### Networking Model
- **Native**: Each sandbox has its own network stack with per-sandbox firewall rules
- **VPN Integration**: Supports Tailscale and OpenVPN for connecting sandboxes to private networks
  - Tailscale: Sandbox becomes part of private tailnet with dedicated IP
  - OpenVPN: Client-server model for corporate VPN integration
- **SSH Gateway**: Standalone service for secure SSH access
  - Token-based authentication
  - Validates sandbox state before forwarding connections
  - Per-region SSH gateway support with dedicated API keys

### OSS Deployment
- Docker Compose configuration includes all services
- Single-node deployment possible
- No K8s requirement for OSS (K8s only via separate Helm charts for enterprise)

## Snapshot & Migration

### Snapshot Format
- **OCI-compliant container images** (not Docker export format)
- Stored in S3-compatible object storage (MinIO in OSS deployment)
- Can be created from:
  - Public images (any container registry)
  - Local images
  - Local Dockerfiles (Daytona builds them)
  - Private registries (Docker Hub, GCR, GHCR)
  - Declarative builder (programmatic definition via SDKs)

### State Capture
- **Filesystem-only**: Snapshots capture container filesystem state
- **No memory state**: Like Docker export, no running process state
- **Metadata stored separately**: Workspace metadata, labels, provider metadata in PostgreSQL

### Migration Capabilities
- **Export/Import** exists but limited:
  - `daytona pc export` / `daytona pc import` for project configurations (JSON format)
  - Copies to clipboard by default, not file
  - Excludes user-specific data (GitProviderConfigId)
- **No direct snapshot export**: Snapshots stored in S3, not exportable as tarballs
- **Cross-region support**: Snapshots can be available in multiple regions

### Portability
- **Snapshot images are portable** (OCI standard)
- **Migration requires**: S3 storage access, control plane coordination
- **No standalone migration format**: Unlike `docker export | zstd`, Daytona requires full platform

## Resource Requirements

### Minimum System Requirements
**Default Runner Configuration** (from docker-compose.yaml):
- **CPU**: 4 cores (`DEFAULT_RUNNER_CPU=4`)
- **RAM**: 8 GB (`DEFAULT_RUNNER_MEMORY=8`)
- **Disk**: 50 GB (`DEFAULT_RUNNER_DISK=50`)

**Build Resources**:
- **CPU**: 4 cores (`BUILD_CPU_CORES=4`, minimum 1)
- **RAM**: 8 GB (`BUILD_MEMORY_GB=8`, minimum 1)

### Single-Node Deployment
- **Docker Compose setup**: Runs all services on one machine
- **Total services**: 11 containers (API, Proxy, Runner, SSH Gateway, PostgreSQL, Redis, Dex, Registry, MinIO, MailDev, Jaeger, PgAdmin)
- **Estimated minimum**: 8-16 GB RAM (not suitable for 2GB VMs)
- **Default quotas** (configurable):
  - Total CPU: 10,000 units
  - Total Memory: 10,000 units
  - Total Disk: 100,000 units
  - Max per sandbox: 100 CPU, 100 Memory, 1000 Disk

### Sandbox Resource Limits
| Resource | Default | Minimum | Maximum (Organization) |
|----------|---------|---------|------------------------|
| CPU | 1 vCPU | 1 vCPU | 4-100 vCPU (tier-dependent) |
| Memory | 1 GiB | 1 GiB | 8-100 GiB (tier-dependent) |
| Disk | 3 GiB | 1 GiB | 10-1000 GiB (tier-dependent) |

### 2GB VM Feasibility
**NOT FEASIBLE** - Default configuration requires 8GB+ RAM for runner alone.

## Plugin/Provider Model

### Provider Interface
- **Standard interface exists**: `daytonaio/daytona-provider-sample` repo demonstrates pattern
- **HashiCorp go-plugin**: Uses plugin system for provider extensibility
- **Configuration schema**:
  - Target options (required/optional strings, ints, file paths)
  - Default targets
  - Input validation (masked, disabled predicates)
  - Suggestions for UX enhancement

### Custom Providers
- **Can be created**: Fork sample provider, implement required methods
- **Runtime**: Go-based providers that implement Daytona's provider interface
- **Integration**: Plug into Daytona's runner system

### Provider Capabilities
- **Compute resource provisioning**: Allocate VMs/containers as runners
- **Network configuration**: Set up networking for sandboxes
- **Storage management**: Interface with storage backends
- **Lifecycle management**: Start/stop/destroy infrastructure

## Agent-Facing Interface

### Primary Interface: MCP Server
Daytona has **native Model Context Protocol (MCP) support**:
```bash
daytona mcp init [claude|cursor|windsurf]
daytona mcp start
```

**Available MCP Tools**:
- Sandbox management (create, destroy)
- File operations (upload, download)
- Git operations
- Command execution
- Computer use
- Preview link generation

**MCP Configuration**:
```json
{
  "mcpServers": {
    "daytona-mcp": {
      "command": "daytona",
      "args": ["mcp", "start"],
      "env": { "HOME": "${HOME}", "PATH": "${HOME}:..." }
    }
  }
}
```

### Alternative Interfaces
- **REST API**: Full REST API with base URL access
- **SDKs**: Python, TypeScript, Ruby, Go
  - Programmatic sandbox lifecycle management
  - Filesystem operations
  - Git operations
  - Process execution
- **CLI**: Go-based command-line tool

### API Characteristics
- RESTful, not gRPC
- Authentication via API keys
- Organization-level multi-tenancy
- Rate limits (tier-based)

## State Model

### Workspace Representation
- **UUID-based identity**: Workspaces have UUIDs (not just names)
- **Metadata fields**:
  - `id` (UUID)
  - `name` (string)
  - `image` (OCI image reference)
  - `env` (map of environment variables)
  - `repository` (Git repository info)
  - `target` (deployment target)
  - `state` (ResourceState)
  - `labels` (map[string]string) - added in 2025
  - `providerMetadata` (string)
  - `metadata` (workspace metadata)

### Persistence
- **Metadata**: PostgreSQL (relational)
- **Filesystem state**: Container filesystem (ephemeral unless backed up)
- **Snapshots**: S3-compatible object storage (persistent)
- **Volumes**: S3-compatible object storage (persistent, shareable)

### Identity Model
- **No persistent "body" abstraction**: Workspaces are not first-class persistent entities
- **Snapshots are templates**: Base images, not running instances
- **Runtime state**: Stored in control plane (PostgreSQL), not portable
- **Migration limitation**: Cannot move a running workspace without coordination with control plane

### Lifecycle States
- Created, Starting, Started, Stopping, Stopped, Destroyed
- Auto-stop/archive/delete intervals configurable
- State reconciliation by sandbox manager

## OSS vs Commercial Boundary

### What's in OSS (AGPL 3.0)
✅ **Full platform**:
- All core services (API, Runner, Proxy, SSH Gateway)
- Complete feature set
- MCP server
- All SDKs
- Docker-based runner (container isolation via Sysbox)
- Tailscale and OpenVPN integration
- Snapshot creation and management
- Volumes and persistent storage
- Multi-region support
- REST API

### What's Commercial-Only
❓ **Managed hosting**:
- SaaS platform at app.daytona.io
- Dedicated support (SLAs)
- HIPAA/SOC 2 compliance (as add-on)
- Custom enterprise terms
- Volume discounts

❓ **Potential proprietary features** (not clearly documented):
- Advanced security features
- Enterprise SSO beyond OIDC
- Custom domains for agent endpoints
- Dedicated IP addresses for outbound traffic
- Zero Data Retention options
- GPU support at scale

### Licensing Implications
**AGPL 3.0**:
- Can use unmodified Daytona commercially without sharing code
- **IF you modify Daytona AND make it available over a network**, must release modifications under AGPL
- "Remote Network Interaction" clause (Section 13) triggered by user interaction
- Workaround: Purchase commercial license to avoid copyleft requirements

### Self-Hosting
- **Fully supported** via Docker Compose
- Community support (GitHub, Slack)
- No license fee for self-hosting unmodified code
- AGPL obligations only if you modify the platform

## Key Findings

**F1**: Daytona is **resource-heavy**. Default runner requires 4 CPU / 8GB RAM. Single-node OSS deployment with all services estimated at 8-16GB RAM minimum. **Cannot run on 2GB VMs** without significant reconfiguration.

**F2**: Daytona uses **OCI container images** for snapshots, not Docker export format. Snapshots stored in S3-compatible object storage, requiring full platform coordination for migration. No `docker export | zstd` equivalent for standalone state capture.

**F3**: Daytona has **native MCP support**, making it highly agent-friendly. Primary interface is already MCP, which aligns with Mesh's "primary interface is MCP" constraint.

**F4**: **No Kubernetes dependency** for OSS. Daytona OSS uses Docker Compose. K8s only via separate enterprise Helm charts. Mesh's "no K8s" constraint is compatible.

**F5**: Daytona lacks a **persistent "body" abstraction**. Workspaces have metadata in PostgreSQL and filesystem in containers, but no concept of a portable persistent identity like Mesh's "body" that survives substrate changes.

**F6**: **Migration is platform-bound**. Cannot cold-migrate (stop, export, destroy, instantiate elsewhere, import) a workspace without Daytona control plane coordination. State is distributed across PostgreSQL, S3, and container runtime.

**F7**: Daytona provides **VPN networking** (Tailscale, OpenVPN) and **SSH gateway** with token auth. Mesh would need to integrate with or duplicate this networking model.

**F8**: **AGPL 3.0 license** imposes copyleft requirements if Mesh modifies Daytona and makes it available over a network. Commercial license available but may cost money.

**F9**: **No clear "body = container + filesystem snapshot" primitive**. Daytona's snapshot model is closer to "image template" rather than "running state capture".

**F10**: Provider model exists (Go-based plugins), but Daytona is **monolithic platform**, not a portable runtime that can be embedded. Mesh would need to run Daytona as a dependency, not integrate at a component level.

## Mesh Implications

### Can Mesh Run ON TOP of Daytona?
**Maybe, but problematic**:
- ✅ MCP alignment: Daytona has native MCP server
- ✅ Agent-friendly: Designed for AI code execution
- ❌ Resource mismatch: Daytona too heavy for 2GB VMs
- ❌ No body abstraction: Daytona doesn't expose a portable "body" primitive
- ❌ Migration constraint: Mesh can't cold-migrate Daytona workspaces
- ❌ AGPL concerns: Modifying Daytona triggers copyleft
- ❌ Platform dependency: Mesh would depend on Daytona control plane

**Verdict**: Running Mesh ON TOP of Daytona means Mesh becomes a thin wrapper around Daytona's MCP server, losing Mesh's core value proposition (portable body runtime).

### Can Mesh Run ALONGSIDE Daytona?
**Technically possible, no clear benefit**:
- Could use Daytona as one substrate provider
- Would need Daytona provider implementation
- Heavy weight compared to other substrates
- Adds complexity without solving Mesh's core needs
- Better to focus on lighter substrates (Docker directly, Nomad, etc.)

**Verdict**: Parallel deployment adds unnecessary complexity.

### Must Mesh Be SEPARATE from Daytona?
**Yes, for Mesh's core goals**:
- ✅ Mesh's 2GB VM constraint incompatible with Daytona
- ✅ Mesh's "no central dependency" conflicts with Daytona's monolithic control plane
- ✅ Mesh's "body = filesystem snapshot" doesn't match Daytona's snapshot model
- ✅ Mesh's "cold migration" not supported by Daytona
- ✅ AGPL license risk if Mesh modifies Daytona
- ✅ Mesh is simpler: Nomad + Docker + Tailscale vs. Daytona's 11-service stack

**Verdict**: Separation is the right approach. Mesh and Daytona solve different problems:
- **Daytona**: Managed AI code execution platform (SaaS focus, heavy, full-featured)
- **Mesh**: Portable agent-body runtime (self-hosted, lightweight, minimal)

### Potential Integration Points
Even if separate, Mesh could learn from Daytona:
- **MCP implementation**: Study Daytona's MCP server design
- **Provider model**: Use similar plugin pattern for substrate providers
- **VPN networking**: Implement Tailscale/OpenVPN integration like Daytona
- **Snapshot format**: Consider OCI images (with export capability)
- **SSH gateway**: Implement token-based SSH access similar to Daytona

## Verdict

**Mesh should NOT build on top of or integrate deeply with Daytona.**

### Rationale

1. **Fundamental mismatch in goals**:
   - Daytona: Managed platform for AI code execution (SaaS, commercial focus)
   - Mesh: Self-hosted portable agent-body runtime (user-owned, decentralized)

2. **Incompatible constraints**:
   - Mesh's 2GB VM requirement vs. Daytona's 8GB+ minimum
   - Mesh's "no central dependency" vs. Daytona's monolithic control plane

3. **Different primitives**:
   - Mesh: Body = persistent identity across substrates, cold-migration via filesystem snapshots
   - Daytona: Workspace = ephemeral instance, platform-bound state, no portable body abstraction

4. **License and complexity concerns**:
   - AGPL 3.0 triggers if Mesh modifies Daytona
   - Daytona's 11-service stack is overkill for Mesh's minimal runtime needs

5. **No clear benefit to integration**:
   - Mesh's MCP server would be simpler to build directly
   - Mesh's snapshot primitive (docker export | zstd) is simpler and more portable than Daytona's OCI+S3 model
   - Mesh's Nomad substrate is lighter than Daytona's Docker-based runner

### Recommendation

**Proceed with Mesh as a separate, independent runtime.** Use Daytona as a reference for:
- MCP server implementation patterns
- VPN networking approaches (Tailscale integration)
- Provider plugin architecture
- SSH gateway security model

But **do not depend on Daytona as a substrate or platform component.** Mesh's value proposition is simplicity, portability, and user-ownership—things Daytona doesn't provide at the scale Mesh needs.

### Strategic Positioning

**Daytona** = "Heroku for AI code execution" (managed, feature-rich, commercial)
**Mesh** = "Nomad + Docker + Tailscale for AI agents" (self-hosted, minimal, portable)

Different markets, different users, different constraints. They're complementary, not competing, and Mesh should remain independent.
