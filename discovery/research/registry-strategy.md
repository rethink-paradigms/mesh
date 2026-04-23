# Research: Registry & Storage Strategy

> Completed: April 23, 2026
> Source: AWS S3 pricing, Cloudflare R2 docs, Docker Hub pricing, OCI spec, Harbor docs, E2B API, Fly.io pricing, IPFS analysis, plugin architecture patterns

## Options Analysis

### Option 1: User-Configured Blob Storage (S3/GCS/R2/MinIO)

**How it works:** User provides object storage credentials, Mesh pushes/pulls tarball snapshots directly to/from the storage service.

#### AWS S3
- **User friction:** Medium. Requires AWS account, IAM setup, credentials management.
- **Cost:** 
  - Standard: $0.023/GB/month
  - Standard-IA: $0.0125/GB/month + $0.01/GB retrieval
  - Glacier Instant: $0.004/GB/month + $0.03/GB retrieval
  - 1GB: ~$0.23/month
  - 10GB: ~$0.23/month
  - 100GB: ~$2.30/month
- **Portability:** High. S3 API is de facto standard, S3-compatible alternatives exist.
- **Reliability:** Very high. 99.999999999% durability, multi-AZ replication.
- **Constraint compliance:** ✅ User owns all compute, keys, network. ✅ No telemetry/login required. ✅ Core can be tiny (plugin). ✅ Compatible with D6 plugin model.
- **Plugin compatibility:** Excellent. S3 SDKs available for all languages, well-documented API.
- **v0 feasibility:** High. S3 is battle-tested, extensive documentation, many examples.

#### Cloudflare R2
- **User friction:** Low-Medium. Requires Cloudflare account, but simpler setup than AWS.
- **Cost:**
  - Standard: $0.015/GB/month
  - Infrequent Access: $0.01/GB/month + $0.01/GB retrieval
  - Free tier: 10GB/month storage, 1M Class A ops, 10M Class B ops
  - **Zero egress fees** (major advantage)
  - 1GB: $0 (within free tier)
  - 10GB: $0 (within free tier)
  - 100GB: $1.50/month
- **Portability:** High. S3-compatible API, can switch backends without code changes.
- **Reliability:** High. Cloudflare's global network, but younger than S3.
- **Constraint compliance:** ✅ User owns all compute, keys, network. ✅ No telemetry/login required. ✅ Core can be tiny (plugin).
- **Plugin compatibility:** Excellent. S3-compatible.
- **v0 feasibility:** High. Simple pricing, great for bandwidth-heavy workloads.

#### Google Cloud Storage
- **User friction:** Medium. Requires GCP account, project setup.
- **Cost:**
  - Standard: $0.02/GB/month
  - Nearline: $0.01/GB/month (30-day minimum)
  - Coldline: $0.004/GB/month (90-day minimum)
  - Archive: $0.0012/GB/month (180-day minimum)
  - 5GB free tier
  - 1GB: $0.02/month
  - 10GB: $0.20/month
  - 100GB: $2.00/month
- **Portability:** High. Google has own API but also S3-compatible access.
- **Reliability:** Very high. Google's infrastructure.
- **Constraint compliance:** ✅ User owns all compute, keys, network.
- **Plugin compatibility:** Good. Official GCS SDKs, also S3-compatible endpoint.
- **v0 feasibility:** High.

#### Azure Blob Storage
- **User friction:** Medium. Requires Azure account.
- **Cost:**
  - Hot: $0.0184/GB/month
  - Cool: $0.01/GB/month
  - Cold: $0.004/GB/month
  - Archive: $0.002/GB/month
  - 1GB: ~$0.018/month
  - 10GB: ~$0.18/month
  - 100GB: ~$1.84/month
- **Portability:** High. Azure Storage REST API is well-documented.
- **Reliability:** Very high. Microsoft's infrastructure.
- **Constraint compliance:** ✅ User owns all compute, keys, network.
- **Plugin compatibility:** Good. Azure SDKs available.
- **v0 feasibility:** High.

#### MinIO (Self-Hosted)
- **User friction:** High. Requires server setup, maintenance, backups.
- **Cost:** 
  - Software: Free (open source)
  - Hardware: Cost of server/storage
  - Operations: Engineering time for maintenance
  - Requirements: 4-8GB RAM, dedicated storage, TLS setup
  - ⚠️ Note: Official MinIO Docker images discontinued October 2025, project archived February 2026. Use with caution.
- **Portability:** High. Full S3 API compatibility, can switch to cloud S3 anytime.
- **Reliability:** Depends on setup. Single-node has no redundancy. Distributed mode (4+ nodes) required for HA.
- **Constraint compliance:** ✅ User owns everything (self-hosted). ✅ No external dependencies. ✅ Core tiny (plugin). ✅ BUT: MinIO project status uncertain (archived).
- **Plugin compatibility:** Excellent. S3-compatible.
- **v0 feasibility:** Medium. More setup overhead, but viable for air-gapped or strict data residency requirements.

---

### Option 2: OCI Registry (Docker Hub, GHCR, Self-Hosted)

**How it works:** Push body as OCI artifact (arbitrary blob) to standard container registry using ORAS or direct OCI manifest.

**Technical detail:** OCI Distribution Spec supports arbitrary artifacts via the `artifactType` field. ORAS provides tooling to push/pull non-image artifacts. This is NOT just stuffing data in image layers — it's a first-class artifact type.

#### Docker Hub
- **User friction:** Low. Everyone has a Docker Hub account.
- **Cost:**
  - Personal: Free, 1 private repo, 100 pulls/hour
  - Pro: $9-11/month, unlimited pulls, unlimited private repos
  - Team: $15-16/month/user
  - Business: $24/month/user
  - Storage pricing not explicitly listed (historically reasonable)
- **Portability:** Medium. Images can be pulled from any registry, but moving repos requires re-pushing.
- **Reliability:** High. Well-maintained service, but rate limits can be problematic.
- **Constraint compliance:** ✅ User owns account. ❌ Rate limits (violates spirit of "no friction"). ✅ No telemetry (Docker doesn't track image contents). ✅ Core tiny (plugin).
- **Plugin compatibility:** Good. OCI distribution API is standard, Docker Registry HTTP API V2.
- **v0 feasibility:** High. Well-known, widely used.

#### GitHub Container Registry (GHCR)
- **User friction:** Low. Uses GitHub account, integrates with Actions.
- **Cost:**
  - Public repos: Unlimited storage and bandwidth (free)
  - Private repos: 500MB storage + 1GB bandwidth (free tier)
  - After free tier: ~$0.25/GB
  - Pro/Team: 2GB storage + 10GB bandwidth for $4-4.50/month
  - Enterprise: 50GB storage + 100GB bandwidth
  - ⚠️ Note: Container registry storage is currently free "until at least one month in advance of any change"
  - 1GB: Free (within 500MB limit, but over)
  - 10GB: ~$2.38/month (9.5GB over free tier × $0.25)
  - 100GB: ~$24.75/month (99.5GB over free tier × $0.25)
- **Portability:** High. Can pull from GHCR and push elsewhere.
- **Reliability:** Very high. GitHub's infrastructure.
- **Constraint compliance:** ✅ User owns account. ✅ No rate limits. ✅ Core tiny (plugin).
- **Plugin compatibility:** Good. OCI-compatible.
- **v0 feasibility:** High. Great for CI/CD integration.

#### Harbor (Self-Hosted)
- **User friction:** High. Requires Kubernetes or Docker Compose setup.
- **Cost:**
  - Software: Free (open source, CNCF-graduated)
  - Infrastructure: Cost of cluster/storage
  - Requirements: ~1.5GB RAM, 2 CPU cores, 2GB+ for images
  - External PostgreSQL and Redis recommended for production
- **Portability:** High. OCI-compatible, can replicate to other registries.
- **Reliability:** High. Replication, vulnerability scanning, content trust.
- **Constraint compliance:** ✅ User owns everything (self-hosted). ✅ No external dependencies. ✅ Core tiny (plugin).
- **Plugin compatibility:** Good. OCI-compatible.
- **v0 feasibility:** Medium. More setup than cloud registries, but feature-rich.

**OCI Artifact Technical Considerations:**
- ORAS (OCI Registry As Storage) provides generic artifact push/pull
- Artifacts use `artifactType` field to distinguish from container images
- Can store arbitrary blobs, not just images
- Layer semantics can be mismatched for flat tarballs (Mesh bodies are flat filesystem exports)
- Config blob required (can be empty per OCI spec)
- Standard registry infrastructure (auth, versioning, distribution)

---

### Option 3: Provider-Native Storage

**How it works:** Each substrate stores snapshots in its own way. Substrate plugin handles storage.

#### E2B
- **User friction:** None. Automatic with E2B usage.
- **Cost:** 
  - Included in E2B pricing
  - Snapshots are E2B's responsibility
  - No separate storage billing
- **Portability:** **Very low**. Snapshots stored in E2B infrastructure, cannot move to other substrates without export.
- **Reliability:** High (E2B's responsibility), but vendor-dependent.
- **Constraint compliance:** ✅ User owns account. ❌ Violates D2 portability. ✅ No telemetry from Mesh.
- **Plugin compatibility:** Good for E2B plugin, bad for cross-substrate portability.
- **v0 feasibility:** High for E2B-only use, but violates portability principle.

#### Fly.io Volumes
- **User friction:** Low. Automatic volume snapshots.
- **Cost:**
  - $0.08/GB/month for snapshots
  - First 10GB free each month
  - Incremental storage (only changed data billed)
  - 1GB: Free
  - 10GB: Free
  - 100GB: $7.20/month (90GB over free tier × $0.08)
- **Portability:** **Low**. Snapshots tied to Fly.io volumes, cannot move to other substrates.
- **Reliability:** High. Daily snapshots, 5-60 day retention.
- **Constraint compliance:** ✅ User owns account. ❌ Violates D2 portability. ✅ No telemetry from Mesh.
- **Plugin compatibility:** Good for Fly plugin, bad for cross-substrate portability.
- **v0 feasibility:** High for Fly-only use, but violates portability principle.

#### Nomad
- **User friction:** Medium. Requires CSI plugin configuration.
- **Cost:** Depends on storage backend (S3, EBS, Ceph, host path).
- **Portability:** **High**. CSI allows pluggable storage backends.
- **Reliability:** Depends on chosen storage backend.
- **Constraint compliance:** ✅ User owns everything. ✅ Highly flexible.
- **Plugin compatibility:** Excellent. CSI plugin model is standard.
- **v0 feasibility:** Medium. Requires Nomad expertise, but flexible.

**Critical Issue:** Provider-native storage **violates D2's portability requirement**. Bodies cannot move between substrates without export/import. This is a fundamental conflict with Mesh's core design.

---

### Option 4: Local Filesystem Only

**How it works:** Snapshots live on the machine where they were created. Manual copy required for portability.

- **User friction:** None. Zero configuration.
- **Cost:** $0. Cost of local disk space.
- **Portability:** **Very low**. Must manually copy tarballs between machines.
- **Reliability:** **Low**. Single point of failure, no redundancy.
- **Constraint compliance:** ✅ User owns everything. ✅ No external dependencies. ✅ Core tiny.
- **Plugin compatibility:** N/A (no plugin needed, just file I/O).
- **v0 feasibility:** High for local-only use (A5 developer agent), but fails portability.

**Use case:** A5 (developer agent) working entirely on local machine. Good for initial development, but not for production multi-substrate use.

---

### Option 5: Mesh-Managed P2P Storage (IPFS, libp2p, Custom)

**How it works:** Snapshots distributed across user's nodes using P2P protocols.

#### IPFS
- **User friction:** Very high. Requires running IPFS nodes, pinning services, gateway management.
- **Cost:**
  - Software: Free
  - Hardware: Multiple nodes for redundancy
  - Bandwidth: Significant cost for serving content
  - Pinning services: $0.01-0.10/GB/month
- **Portability:** High. Content-addressed, globally accessible.
- **Reliability:** **Very low**. IPFS does NOT guarantee persistence. Content disappears if not pinned.
  - Must use pinning services (Filebase, Pinata, Web3.storage)
  - Gateways can fail
  - Retrieval latency unpredictable
  - Network congestion affects performance
- **Constraint compliance:** ✅ Decentralized, no single provider. ❌ Reliability issues. ❌ Complexity violates "core is tiny" (C6).
- **Plugin compatibility:** Medium. IPFS libraries exist, but integration is complex.
- **v0 feasibility:** **Very low**. Too complex, too unreliable for v0.

**IPFS Critical Issues:**
- "Content must be pinned or otherwise stored by nodes that commit to keeping it. Without that, availability can disappear."
- "IPFS fails as a standalone solution when teams need guaranteed persistence, fast global delivery, private access control, or relational querying."
- "If nobody pins it, it can disappear from available peers."
- "Retrieval depends on network topology, peer health, cached availability, and gateway quality."

**Verdict:** Not viable for v0. Maybe for v2+ as an optional backend for distributed teams, but requires significant infrastructure (multiple nodes, pinning services, gateway redundancy).

---

## Comparison Matrix

| Option | User Friction | Cost (1GB) | Cost (10GB) | Cost (100GB) | Portability | Reliability | v0 Feasible |
|--------|--------------|------------|-------------|--------------|-------------|-------------|-------------|
| S3 | Medium | $0.23 | $2.30 | $23.00 | High | Very High | ✅ Yes |
| R2 | Low-Med | $0 | $0 | $1.50 | High | High | ✅ Yes |
| GCS | Medium | $0.02 | $0.20 | $2.00 | High | Very High | ✅ Yes |
| Azure | Medium | $0.018 | $0.18 | $1.84 | High | Very High | ✅ Yes |
| MinIO | High | Hardware only | Hardware only | Hardware only | High | Medium | ⚠️ Maybe (project archived) |
| Docker Hub | Low | Free | Free | ~$5-9 | Medium | High | ✅ Yes |
| GHCR | Low | ~$0.13 | ~$2.38 | ~$24.75 | High | Very High | ✅ Yes |
| Harbor | High | Infrastructure only | Infrastructure only | Infrastructure only | High | High | ⚠️ Maybe |
| E2B Native | None | Included | Included | Included | ❌ Very Low | High | ❌ Violates D2 |
| Fly Native | Low | Free | Free | $7.20 | ❌ Low | High | ❌ Violates D2 |
| Nomad CSI | Medium | Varies | Varies | Varies | High | Varies | ⚠️ Maybe |
| Local FS | None | $0 | $0 | $0 | ❌ Very Low | ❌ Low | ✅ Local only |
| IPFS | Very High | $0.01-0.10 | $0.10-1.00 | $1.00-10.00 | High | ❌ Very Low | ❌ No |

**Notes:**
- Costs are monthly estimates
- "Infrastructure only" means you pay for servers/storage, not per-GB fees
- R2 free tier: 10GB storage, 1M Class A ops, 10M Class B ops, zero egress
- GHCR free tier: 500MB storage + 1GB bandwidth
- Fly free tier: 10GB snapshot storage
- Docker Hub storage cost not explicitly listed, historically reasonable

---

## Hybrid Approach Analysis

**Can options be combined?** Yes, and this is likely the right answer for Mesh.

### Local + Remote Backup
- **How it works:** Primary storage is local filesystem (fast, zero config). Remote storage (S3/R2) as backup/sync.
- **Pros:** Fast local access, portable remote backup, user choice of remote provider.
- **Cons:** Need sync logic, potential conflicts, increased complexity.
- **Implementation:** "local" plugin + "s3" plugin, with a sync daemon.
- **Use case:** A5 developer agent wants fast local access, but wants to sync to S3 for backup.

### Multi-Cloud Redundancy
- **How it works:** Store snapshots in multiple backends simultaneously (e.g., S3 + R2).
- **Pros:** Zero single point of failure, no vendor lock-in.
- **Cons:** Increased cost, complexity, sync challenges.
- **Implementation:** "multi" plugin that wraps multiple storage backends.
- **Use case:** Enterprise user wants redundancy across cloud providers.

### Tiered Storage
- **How it works:** Recent snapshots on local/SSD (fast), older snapshots on cold storage (cheap).
- **Pros:** Cost optimization, fast access to recent data.
- **Cons:** Complexity of tiering logic, migration policies.
- **Implementation:** "tiered" plugin that manages lifecycle.
- **Use case:** User with 100+ bodies, wants to keep recent 10 on fast storage.

### Substrate-Specific + Portable
- **How it works:** Use substrate-native storage for fast access, but also export to portable format (OCI artifact or S3) for portability.
- **Pros:** Best of both worlds: performance + portability.
- **Cons:** Duplicate storage, export overhead.
- **Implementation:** Substrate plugin handles native storage + export to portable format.
- **Use case:** E2B user wants fast snapshots, but also wants to move to Nomad later.

**Plugin Model Compatibility (D6):**
All hybrid approaches work well with D6's plugin model. The core Mesh runtime just calls `storage_plugin.push(body)` and `storage_plugin.pull(body_id)`. The plugin handles the complexity (multi-cloud, tiering, sync).

---

## Plugin Interface Proposal

Based on industry patterns (Mimir AIP, SPIKE, MemPalace), here's a proposal for the Mesh storage plugin interface:

```go
// StoragePlugin interface for body snapshot storage
type StoragePlugin interface {
    // Initialize the plugin with configuration
    Initialize(config *PluginConfig) error
    
    // Push a body snapshot to storage
    // Returns: storage ID, size in bytes, error
    Push(ctx context.Context, body *BodySnapshot) (*PushResult, error)
    
    // Pull a body snapshot from storage
    // Returns: body snapshot, error
    Pull(ctx context.Context, bodyID string) (*BodySnapshot, error)
    
    // Delete a body snapshot
    Delete(ctx context.Context, bodyID string) error
    
    // List all body snapshots
    // Returns: list of body IDs, error
    List(ctx context.Context) ([]string, error)
    
    // Get metadata for a body (size, created_at, etc.)
    GetMetadata(ctx context.Context, bodyID string) (*BodyMetadata, error)
    
    // Check if storage backend is healthy
    HealthCheck(ctx context.Context) (bool, error)
    
    // Close plugin, release resources
    Close() error
}

// BodySnapshot represents a portable Mesh body
type BodySnapshot struct {
    // OCI image reference (base layer)
    ImageRef string
    
    // Volume tarball (docker export | zstd)
    VolumeTarball []byte
    
    // Metadata (labels, annotations, etc.)
    Metadata map[string]string
}

// PushResult result of pushing a body
type PushResult struct {
    // Storage-specific ID (S3 key, OCI digest, etc.)
    StorageID string
    
    // Size in bytes
    Size int64
    
    // ETag or checksum for verification
    ETag string
}

// PluginConfig configuration for storage plugins
type PluginConfig struct {
    // Plugin type: "s3", "r2", "gcs", "oci", "local", "e2b", "fly", etc.
    Type string
    
    // Provider-specific config (credentials, endpoints, etc.)
    Config map[string]interface{}
    
    // Options (compression, encryption, etc.)
    Options *StorageOptions
}

// StorageOptions common options for all storage plugins
type StorageOptions struct {
    // Compression: "none", "gzip", "zstd" (default: "zstd")
    Compression string
    
    // Encryption: enable encryption at rest
    Encryption bool
    
    // Retention: how long to keep snapshots (0 = forever)
    Retention time.Duration
    
    // Tiering: "hot", "cold", "auto" (for tiered plugins)
    Tiering string
}
```

**Plugin Registration:**
```go
// PluginRegistry manages storage plugins
type PluginRegistry interface {
    Register(name string, factory StorageFactory) error
    Get(name string) (StoragePlugin, error)
    List() []string
}

// StorageFactory creates storage plugin instances
type StorageFactory func(config *PluginConfig) (StoragePlugin, error)
```

**Configuration Example:**
```yaml
storage:
  type: "s3"  # or "r2", "gcs", "oci", "local", "multi", "tiered"
  config:
    bucket: "mesh-bodies"
    region: "us-east-1"
    access_key_id: "${AWS_ACCESS_KEY_ID}"
    secret_access_key: "${AWS_SECRET_ACCESS_KEY}"
  options:
    compression: "zstd"
    encryption: true
    retention: "0"  # forever
```

**Multi-Plugin Example:**
```yaml
storage:
  type: "multi"
  config:
    backends:
      - name: "local"
        type: "local"
        config:
          path: "/var/lib/mesh/bodies"
      - name: "s3-backup"
        type: "s3"
        config:
          bucket: "mesh-backup"
          region: "us-east-1"
    strategy: "write_all"  # or "write_primary_sync_secondary"
```

This interface is:
- ✅ Compatible with D6 (plugin model)
- ✅ Supports all storage options
- ✅ Allows hybrid/multi-backend approaches
- ✅ Simple enough for v0 (can start with single backend)
- ✅ Extensible for future features (encryption, tiering, sync)

---

## Key Findings

### F1: Object Storage (S3/R2/GCS/Azure) is Best for Portability
- S3-compatible APIs are de facto standard
- Provider lock-in is minimal (can switch with config change)
- Zero egress on R2 is a major advantage for bandwidth-heavy workloads
- All major providers have reliable, battle-tested infrastructure

### F2: OCI Registries Have Technical Limitations for Flat Tarballs
- OCI artifact spec supports arbitrary blobs via ORAS
- BUT: Layer semantics don't match flat filesystem exports (Mesh bodies are docker export | zstd, not layered images)
- Config blob is required (can be empty, but adds overhead)
- Rate limits on Docker Hub are problematic for CI/CD
- GHCR is great but pricing can be expensive at scale ($0.25/GB after 500MB free)

### F3: Provider-Native Storage Violates Core Portability Requirement (D2)
- E2B snapshots, Fly.io volumes are great for single-substrate use
- BUT: Cannot move bodies between substrates without export/import
- This is a fundamental conflict with Mesh's "portable compute identity" design

### F4: Local Filesystem is Good for Local-Only Use (A5), Bad for Multi-Substrate
- Zero config, fastest access, zero cost
- BUT: No portability, single point of failure
- Good for: Developer agent working on laptop
- Bad for: Production multi-substrate deployment

### F5: IPFS is Not Viable for v0
- Does NOT guarantee persistence (content disappears if not pinned)
- Retrieval reliability is poor (gateways fail, network congestion)
- High operational complexity (multiple nodes, pinning services, gateway management)
- Maybe viable for v2+ as optional backend, but not for v0

### F6: Hybrid Approaches are Powerful
- Local + remote backup gives best of both worlds
- Multi-cloud redundancy eliminates single point of failure
- Tiered storage optimizes cost
- All hybrid approaches work well with D6's plugin model

### F7: Plugin Interface Should Be Simple and Extensible
- Core operations: Push, Pull, Delete, List, GetMetadata, HealthCheck
- Factory pattern for plugin registration
- Config-based initialization
- Support for multi-backend and tiered storage via wrapper plugins

### F8: Cost Analysis at Scale
- **1GB scale:** All options are essentially free (within free tiers or <$1/month)
- **10GB scale:** 
  - R2: Free (10GB free tier)
  - S3: $2.30/month
  - GCS: $0.20/month
  - Azure: $0.18/month
  - GHCR: $2.38/month
- **100GB scale:**
  - R2: $1.50/month (best value)
  - S3: $23.00/month
  - GCS: $2.00/month
  - Azure: $1.84/month
  - GHCR: $24.75/month (most expensive)
- **Egress costs matter:** S3 charges $0.09/GB after first 100GB, R2 charges $0

### F9: MinIO Project Status is Uncertain
- Official Docker images discontinued October 2025
- GitHub repository archived February 2026
- Still works, but no new releases or security patches
- Consider Garage or other alternatives for new deployments

---

## Verdict

### Recommended Approach for v0: **User-Configured Object Storage (S3/R2/GCS/Azure)**

**Rationale:**

1. **Best fit for constraints:**
   - ✅ C3: User owns all compute, keys, network
   - ✅ C4: No telemetry, no login, no mesh-controlled auth
   - ✅ C6: Core is tiny (storage is plugin)
   - ✅ D2: Bodies are portable (standard format, any S3-compatible storage)
   - ✅ D6: Provider integrations are plugins

2. **Portability:** 
   - S3-compatible API is de facto standard
   - Can switch providers with config change
   - Bodies are flat tarballs, universally compatible

3. **Cost:**
   - R2 is best value: $0.015/GB/month, zero egress, 10GB free tier
   - S3 is most battle-tested: $0.023/GB/month, but egress fees add up
   - GCS and Azure are competitive options

4. **Reliability:**
   - All major providers have 99.999999999% durability
   - Multi-AZ replication, built-in redundancy
   - Decades of operational experience

5. **v0 Feasibility:**
   - S3 SDKs available for all languages
   - Extensive documentation and examples
   - Simple plugin interface (Push, Pull, Delete, List)
   - Can ship with 2-3 plugins (S3, R2, Local) and let community add more

### Plugin Strategy for v0:

1. **Core plugins (ship with Mesh):**
   - `local`: Local filesystem (for A5 developer agent, testing)
   - `s3`: AWS S3 and S3-compatible (R2, MinIO, Wasabi, etc.)
   - `oci`: GitHub Container Registry, Docker Hub (for users who prefer registries)

2. **Community plugins (examples for users to build):**
   - `gcs`: Google Cloud Storage
   - `azure`: Azure Blob Storage
   - `e2b`: E2B snapshots (for E2B-only use cases)
   - `fly`: Fly.io volumes (for Fly-only use cases)

3. **Hybrid plugins (future):**
   - `multi`: Multi-cloud redundancy
   - `tiered`: Hot/cold storage tiering
   - `sync`: Local + remote sync

### Configuration Experience:

```bash
# Default: Local filesystem (zero config)
mesh init

# Use R2 (best value, zero egress)
mesh config set storage.type r2
mesh config set storage.config.account_id "${R2_ACCOUNT_ID}"
mesh config set storage.config.access_key_id "${R2_ACCESS_KEY_ID}"
mesh config set storage.config.secret_access_key "${R2_SECRET_ACCESS_KEY}"

# Use S3 (most battle-tested)
mesh config set storage.type s3
mesh config set storage.config.bucket "mesh-bodies"
mesh config set storage.config.region "us-east-1"
# AWS credentials from env or ~/.aws/credentials

# Use GHCR (great for CI/CD)
mesh config set storage.type oci
mesh config set storage.config.registry "ghcr.io"
mesh config set storage.config.username "${GITHUB_USERNAME}"
mesh config set storage.config.password "${GITHUB_TOKEN}"

# Use multi-backend (redundancy)
mesh config set storage.type multi
mesh config set storage.config.backends "[{\"name\":\"local\",\"type\":\"local\",\"config\":{\"path\":\"/var/lib/mesh/bodies\"}},{\"name\":\"s3-backup\",\"type\":\"s3\",\"config\":{\"bucket\":\"mesh-backup\",\"region\":\"us-east-1\"}}]"
mesh config set storage.config.strategy "write_all"
```

### Migration Path:

1. **v0:** Ship with `local`, `s3`, `oci` plugins. Focus on object storage.
2. **v0.x:** Add `gcs`, `azure` plugins based on user demand.
3. **v1:** Add hybrid plugins (`multi`, `tiered`, `sync`) for advanced use cases.
4. **v2:** Consider P2P storage (IPFS) as optional backend, but only if reliability improves.

### Decision Framework for Users:

| Use Case | Recommended Storage | Why? |
|----------|---------------------|------|
| A5 (developer agent, local only) | `local` | Zero config, fastest access |
| A1-A4 (multi-substrate, production) | `s3` or `r2` | Portable, reliable, cost-effective |
| CI/CD integration | `oci` (GHCR) | GitHub Actions integration, familiar workflow |
| Bandwidth-heavy workloads | `r2` | Zero egress fees |
| Cost optimization at 100GB+ scale | `r2` or `azure` | <$2/month vs $23/month (S3) |
| Multi-cloud redundancy | `multi` plugin | No single point of failure |
| Air-gapped / strict data residency | `s3` (MinIO) or self-hosted registry | No external dependencies |
| E2B-only use case | `e2b` plugin | Fastest, but not portable |

### Alternative: Start with Local + S3, Add OCI Later

If OCI artifact storage proves too complex for v0 (layer semantics, config blob, ORAS integration), start with:

1. `local` plugin (for A5)
2. `s3` plugin (for A1-A4, using R2 or S3)
3. Add `oci` plugin in v0.x if users request it

This reduces v0 scope while still providing portable, reliable storage.

---

## Open Questions

1. **OCI vs. S3:** Should we support OCI registries in v0, or defer to v0.x?
   - Pro: Familiar workflow, integrates with existing tools
   - Con: Layer semantics don't match flat tarballs, adds complexity
   - Recommendation: Defer to v0.x, focus on S3-compatible storage for v0

2. **Compression:** What compression format for tarballs?
   - Options: none, gzip, zstd
   - Trade-off: Compression ratio vs. CPU time
   - Recommendation: zstd (fast compression, good ratio), but make it configurable

3. **Encryption:** Should we encrypt at rest?
   - Options: none, AES-256, customer-managed keys
   - Trade-off: Security vs. complexity vs. performance
   - Recommendation: Optional, offload to storage provider (S3 SSE-S3, R2 encryption)

4. **Versioning:** Should we support multiple versions of the same body?
   - Options: Single version (immutable), Multiple versions (versioned storage)
   - Trade-off: Complexity vs. flexibility
   - Recommendation: Single version for v0, add versioning in v1 if needed

5. **Metadata:** What metadata should we store with each body?
   - Options: Minimal (ID, size, created_at), Rich (labels, annotations, tags)
   - Trade-off: Storage overhead vs. queryability
   - Recommendation: Rich metadata for v0 (labels, annotations, creator, substrate)

6. **Garbage Collection:** How do we handle old/deleted bodies?
   - Options: Never delete, Time-based retention, Manual deletion
   - Trade-off: Cost vs. user control
   - Recommendation: Manual deletion for v0, add retention policies in v1

7. **Sync:** Should we support sync between backends?
   - Options: No sync, Async sync, Real-time sync
   - Trade-off: Complexity vs. data safety
   - Recommendation: No sync for v0, add `sync` plugin in v1
