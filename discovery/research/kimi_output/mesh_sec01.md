# 1. Provider Ecosystem Map

The Mesh SubstrateAdapter interface requires every provider to expose eight imperative verbs: Create, Start, Stop, Destroy, GetStatus, Exec, ExportFilesystem, and ImportFilesystem. Not every cloud API maps cleanly. Providers with machine-readable OpenAPI specifications enable mechanical adapter generation; those without force the AI agent to read REST docs and hand-write HTTP clients — a pattern with a documented 33% compilation failure rate [^19^]. This chapter catalogs VM, sandbox, and self-hosted options, then ranks them into Wave 1 and Wave 2.

## 1.1 VM Provider Matrix

Eight public-cloud VM providers were evaluated on six criteria: official Go SDK, public OpenAPI specification, full lifecycle coverage, filesystem export capability, authentication simplicity, and fully-loaded monthly cost of a 2GB instance.

| Provider | Go SDK (Stars) | OpenAPI Spec | Lifecycle | Filesystem Export | Auth Model | Free Tier | Cost (2GB/mo) |
|----------|---------------|--------------|-----------|-------------------|------------|-----------|---------------|
| AWS EC2 | aws-sdk-go-v2 (~2,500) [^1^][^2^] | No (Query API) [^25^] | Full + SSM RunCommand [^3^][^26^] | AMI export to VMDK/VHD [^27^] | IAM / access key [^28^] | t4g.small until Dec 2026 [^29^] | ~$18.75 [^31^] |
| Hetzner Cloud | hcloud-go (654) [^4^] | Unofficial (community) [^32^] | Full (no native exec) [^33^] | Snapshot→new server only [^34^] | Bearer token [^35^] | None [^36^] | ~$4.51 [^37^] |
| DigitalOcean | godo (1,100) [^7^] | **Yes** (official) [^38^] | Full + cloud-init [^39^] | No direct download [^40^] | Bearer PAT [^41^] | None [^42^] | ~$24 [^44^] |
| Vultr | govultr (150) [^10^] | No [^45^] | Full + cloud-init [^46^] | No direct download [^47^] | Bearer API key [^48^] | None [^49^] | ~$10 [^50^] |
| Google Compute Engine | cloud.google.com/go/compute (4,400*) [^13^] | No (Discovery doc) [^51^] | Full + cloud-init [^52^] | Disk image to GCS [^53^] | Service account / OAuth2 [^54^] | e2-micro only (1GB) [^55^] | ~$12.23 [^57^] |
| Azure VMs | armcompute v7 (track2) [^16^] | No (ARM JSON) [^58^] | Full + Run Command [^59^] | Snapshot→VHD via SAS [^60^] | Service Principal [^61^] | B1s 12 mo (1GB) [^62^] | ~$30.37 [^64^] |
| Linode/Akamai | linodego (401) [^19^] | **Yes** (official) [^65^] | Full + StackScripts [^66^] | No direct export [^67^] | PAT / OAuth2 [^68^] | None [^69^] | ~$12 [^71^] |
| OVH Cloud | go-ovh (149) [^22^] | No [^72^] | Suspend/resume semantics [^73^] | **QCOW2 download** [^74^] | 3-legged / OAuth2 [^75^] | None [^76^] | ~$6–8 [^78^] |

*Stars for entire google-cloud-go monorepo.

The table reveals a stark OpenAPI split: only DigitalOcean and Linode publish official machine-readable specifications [^38^][^65^]. Every other provider documents its API in human-readable formats, forcing the generation pipeline to infer types from prose. Boot latency is uneven — Hetzner provisions in 15–30 seconds [^5^], Azure in 2–5 minutes [^63^] — and cost spans nearly a 7× range. Only AWS offers a no-strings-attached free 2GB tier, ending December 2026 [^29^].

**Wave 1** — AWS EC2, Hetzner, DigitalOcean, and Vultr — each has an actively maintained official Go SDK and covers the full lifecycle. AWS uniquely offers native SSM RunCommand, eliminating SSH-key management [^26^]. Hetzner is cheapest and fastest-booting [^37^]. DigitalOcean's official OpenAPI spec is the only Wave 1 member enabling deterministic `oapi-codegen` [^38^]. Vultr rounds out the quartet at $10/mo with sub-60-second boots, though its 150-star SDK increases AI wrapping risk [^10^].

**Wave 2** — GCP, Azure, Linode, and OVH — are deferred because their ecosystems introduce complexity better tackled after the pipeline is proven. GCP lacks OpenAPI and caps free tier at 1GB [^55^]. Azure is the most expensive ($30.37/mo) and slowest to boot [^64^][^63^]. Linode is mechanically sound but offers no differentiating advantage. OVH is the outlier: it alone provides direct QCOW2 download [^74^], yet its 149-star SDK [^22^] and multi-legged authentication [^75^] make adapter generation fragile.

## 1.2 Sandbox Provider Matrix

Sandbox providers differ from VM providers in two critical dimensions: cold-start latency (milliseconds versus minutes) and filesystem access (file-level APIs versus disk-image export).

| Provider | Go SDK | OpenAPI | Isolation | Filesystem API | Cold Start | Free Tier | Managed Only |
|----------|--------|---------|-----------|---------------|------------|-----------|--------------|
| Daytona | **Official** (`sdk-go`) [^159^] | **Yes** (generated clients) [^159^] | Docker / optional Kata [^184^] | Full (list, upload, download, permissions) [^165^] | ~90ms [^184^] | $200 credits [^184^] | No (self-hostable) |
| Modal | **Beta** v0.5 (`modal-go`) [^136^][^137^] | No | gVisor [^30^] | Full + snapshots [^91^] | Sub-second [^32^] | $30/mo credits [^31^] | Yes |
| E2B | **None** (Py/JS only) [^86^] | No | Firecracker microVMs [^33^] | Isolated per sandbox [^38^] | ~1s | $100 credit [^125^] | No (OSS Apache-2.0) |
| Fly.io Machines | None (community WIP) [^83^] | No | Firecracker [^35^] | Volume mounts [^89^] | Sub-second [^35^] | None ($5 trial) [^108^][^139^] | Yes |

Daytona is the only sandbox provider that satisfies all four generation-friendly properties: official Go SDK, OpenAPI spec, generous free tier, and self-hosting option [^159^][^184^]. Its ~90ms cold start and "archived" state are purpose-built for ephemeral agent workloads [^156^]. Modal's beta Go SDK is the second strongest; it uniquely offers filesystem, directory, and memory snapshots [^91^], but the platform is managed-only, gVisor-only, and Python-centric [^30^]. E2B offers the strongest isolation — Firecracker microVMs with a dedicated kernel per sandbox [^33^] — yet the complete absence of a Go SDK makes it the most expensive adapter to build [^86^]. Fly.io Machines exposes a low-level REST API with an elegant state machine, but no official Go SDK and no free tier push it to the trailing position [^83^][^108^].

## 1.3 Self-Hosted Options

For operators who must satisfy C3 (user-owned compute) and C4 (no central dependency), three self-hosted substrates are viable.

**Incus/LXD** is the strongest all-around candidate. The official Go client supports `CreateInstance`, `ExecInstance`, `DeleteInstance`, `GetInstanceFileSFTP`, and full backup export to compressed tarballs [^1^][^18^]. The REST API is versioned and documented with Swagger annotations, and the daemon runs independently of Kubernetes [^2^]. One daemon manages both system containers and QEMU-backed VMs.

**Podman** is the simplest Docker-compatible option. Its official Go bindings cover `CreateWithSpec`, `Start`, `Stop`, `Remove`, `ExecCreate`, and `Export` [^3^]. The `podman system service` exposes a REST API over a Unix socket compatible with Docker API v1.40+ [^4^]. The core engine is daemon-less and rootless-capable, but container-only.

**Firecracker** delivers the best snapshot speed — ~28ms restore from a saved microVM state [^6^] — and hardware-level isolation via KVM. The official Go SDK wraps the Unix-socket REST API [^19^]. The catch is operational complexity: the operator must supply kernels and rootfs images, configure TAP networking manually, and build a guest agent for in-VM exec [^22^]. It is a strong fit only when isolation and warm-start latency are non-negotiable.

## 1.4 Ranking Criteria and Selection Framework

Wave assignment follows a weighted scoring model: Go SDK (official) at 25%, OpenAPI spec at 25%, full lifecycle support at 20%, filesystem export at 15%, free tier at 10%, and cost efficiency at 5%. Cross-dimensional analysis shows OpenAPI availability is the single strongest predictor of adapter generation success — stronger than SDK maturity or provider popularity [^19^]. Cost receives minimal weight because per-second billing makes operating expense less critical than generation reliability.

| Provider | Go SDK (25%) | OpenAPI (25%) | Lifecycle (20%) | FS Export (15%) | Free Tier (10%) | Cost (5%) | Weighted Score | Wave |
|----------|-------------|---------------|-----------------|-----------------|-----------------|-----------|----------------|------|
| Daytona | 10 | 10 | 9 | 9 | 9 | 8 | **9.1** | **Wave 1** |
| Hetzner | 9 | 6 | 8 | 5 | 3 | 10 | **7.2** | **Wave 1** |
| DigitalOcean | 9 | 10 | 8 | 4 | 3 | 6 | **7.2** | **Wave 1** |
| AWS EC2 | 9 | 3 | 10 | 8 | 8 | 6 | **7.1** | **Wave 1** |
| Modal | 7 | 3 | 9 | 9 | 6 | 7 | **6.9** | **Wave 2** |
| Linode | 8 | 10 | 8 | 3 | 3 | 7 | **6.8** | **Wave 2** |
| GCP | 9 | 3 | 8 | 7 | 4 | 7 | **6.5** | **Wave 2** |
| Azure | 8 | 3 | 9 | 8 | 4 | 4 | **6.3** | **Wave 2** |
| Vultr | 7 | 3 | 8 | 4 | 3 | 9 | **5.9** | **Wave 2** |
| OVH | 5 | 3 | 6 | 9 | 3 | 8 | **5.4** | **Wave 2** |
| E2B | 1 | 3 | 8 | 5 | 5 | 7 | **4.6** | **Wave 2** |
| Fly.io | 3 | 3 | 8 | 5 | 2 | 6 | **4.6** | **Wave 2** |

**Recommended Wave 1 adapters**: Docker (reference implementation), Daytona, Hetzner, DigitalOcean, and AWS EC2. This five-provider set covers all three substrate types and offers a zero-cost bootstrap stack: AWS t4g.small (free until December 2026) or Hetzner CPX11 ($4.51/mo) for VMs, Daytona ($200 free credits, no credit card required) for sandboxes, and Docker (free, local) for the reference [^29^][^184^][^37^].

**Wave 2 candidates** — Modal, Vultr, GCP, Linode, E2B, Fly.io, Azure, and OVH — should be deferred until the generation pipeline is validated against Wave 1. Modal and E2B are the highest-priority Wave 2 items: Modal for its unmatched snapshotting primitives [^91^], and E2B for its Firecracker isolation [^33^], once a custom Go REST client is justified by security requirements. The framework is designed to be re-run quarterly; as providers publish OpenAPI specs or launch official Go SDKs, scores and wave assignments shift automatically.
