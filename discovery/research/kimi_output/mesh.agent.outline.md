# Mesh Plugin Architecture: Research Synthesis

## 1. Provider Ecosystem Map
### 1.1 VM Provider Matrix
#### 1.1.1 Wave 1: AWS EC2, Hetzner, DigitalOcean, Vultr — full lifecycle, Go SDK, cost analysis
#### 1.1.2 Wave 2: GCP, Azure, Linode, OVH — enterprise or secondary options
### 1.2 Sandbox Provider Matrix
#### 1.2.1 Daytona: official Go SDK, OpenAPI, 90ms start, $200 free tier
#### 1.2.2 Modal: beta Go SDK (v0.5), gVisor, advanced snapshots
#### 1.2.3 E2B: Firecracker microVMs, OSS, no Go SDK (Py/JS only)
#### 1.2.4 Fly.io: Machines API, Firecracker, no free tier
### 1.3 Self-Hosted Options
#### 1.3.1 Incus/LXD: native Go client, full REST API, backup/restore
#### 1.3.2 Podman: Docker-compatible, daemon-less, Go bindings
#### 1.3.3 Firecracker: high isolation, DIY orchestration complexity
### 1.4 Ranking Criteria and Selection Framework
#### 1.4.1 Go SDK + OpenAPI + free tier + lifecycle support as weighted factors
#### 1.4.2 Recommended Wave 1 adapters: Docker (reference), Hetzner, Daytona, DigitalOcean

## 2. Codegen Toolchain Recommendation
### 2.1 oapi-codegen v2 Deep Dive
#### 2.1.1 Multi-file specs, $ref handling, enum/optional/oneOf behavior
#### 2.1.2 Error types: untyped *http.Response limitation and wrapper strategy
#### 2.1.3 Auth injection, server-side story, known gotchas
### 2.2 Alternative Generators
#### 2.2.1 ogen: faster, structured output, smaller community
#### 2.2.2 Speakeasy: commercial, idiomatic, free tier limits
#### 2.2.3 OpenAPI Generator: avoid for Go (Java dependency, 1,538 transitive deps)
### 2.3 Non-OpenAPI Providers
#### 2.3.1 Fly Machines now has OpenAPI spec; E2B/AWS require hand-written clients
#### 2.3.2 AI-generated clients from REST docs: 67% compilation failure rate
### 2.4 Toolchain Verdict
#### 2.4.1 Primary: oapi-codegen v2; Secondary: ogen; Conditional: Speakeasy
#### 2.4.2 Failure modes to avoid: oneOf/anyOf, multi-file path issues, untyped errors

## 3. SDK Quality Scores
### 3.1 Scoring Methodology
#### 3.1.1 Idiomatic Go, typed structs, error modeling, pagination, dependencies, maintenance
### 3.2 Tier 1: Easiest for AI (8.0–9.5/10)
#### 3.2.1 Hetzner hcloud-go (9.5): typed errors, WaitFor, minimal deps
#### 3.2.2 DigitalOcean godo (8.5): official, mature, clean API, 2 deps
### 3.3 Tier 2: Medium Complexity (6.5–7.5/10)
#### 3.3.1 AWS SDK v2 (7.5): idiomatic but 300+ modules, overwhelming surface
#### 3.3.2 Azure track2 (7.0): modern, LRO pollers, module proliferation
#### 3.3.3 Linode linodego (7.0): clean but weak typed errors
#### 3.3.4 GCP compute (6.5): protobuf verbosity, pointer-heavy APIs
#### 3.3.5 Vultr govultr (6.5): simple, functional, less mature pagination
### 3.4 Tier 3: Harder or Beta (5.5–6.5/10)
#### 3.4.1 Daytona SDK (6.0): monorepo wrapper, workspace-centric
#### 3.4.2 Modal beta Go SDK (6.5): official but evolving, limited vs Python SDK
#### 3.4.3 Fly.io fly-go (5.5): community-only, 42 stars, app-centric
### 3.5 Tier 4: Not Viable (<3.0/10)
#### 3.5.1 E2B: no official Go SDK; only 1-star community port
### 3.6 Key Gotchas by Provider
#### 3.6.1 AWS smithy middleware, GCP Operation.Wait(), Azure LRO pollers
#### 3.6.2 Simple SDKs outperform for AI wrapping despite fewer features

## 4. Plugin Architecture Recommendation
### 4.1 Pattern Analysis
#### 4.1.1 HashiCorp go-plugin: battle-tested but heavy; justified at 3000+ providers, not 5-15
#### 4.1.2 Terraform/Crossplane: K8s-coupled, overengineered for non-orchestrator use
#### 4.1.3 database/sql: gold standard — tiny core interface + init() registry + extension interfaces
#### 4.1.4 Go plugin package: CGO trap, universally rejected
#### 4.1.5 Wasm Component Model: promising but wazero rejected; not viable for Go in 2026
### 4.2 Recommended Architecture
#### 4.2.1 Phase 1: Simple interface + runtime registry + optional extension interfaces
#### 4.2.2 Phase 2: Escalate to HashiCorp go-plugin gRPC only for untrusted/AI-generated adapters
#### 4.2.3 Build tags for conditional compilation of specific adapters
### 4.3 What NOT To Do
#### 4.3.1 Avoid gRPC for all adapters at small scale
#### 4.3.2 Avoid Go plugin package entirely
#### 4.3.3 Avoid K8s-coupled patterns (Crossplane) for standalone runtime

## 5. Agent Skill Specification
### 5.1 Input Format
#### 5.1.1 Composite provider manifest: OpenAPI spec + target interface + reference adapters + constraints
#### 5.1.2 Constitutional boundaries: max lines, forbidden patterns (no net/http.Client, no json.Marshal)
### 5.2 Skill Structure
#### 5.2.1 Anthropic Agent Skills standard (SKILL.md with YAML frontmatter)
#### 5.2.2 Progressive disclosure: metadata → instructions → references on demand
### 5.3 Reliability Patterns
#### 5.3.1 4-phase workflow: Structured CoT Analyze → Test-First Plan → Few-Shot Write → Validation Verify
#### 5.3.2 Few-shot with 2-3 working adapters is the sweet spot
### 5.4 Boundary Enforcement
#### 5.4.1 Agent MUST NOT generate: HTTP client, serialization, retry logic
#### 5.4.1 Agent ONLY writes mapping layer (~200 lines)
#### 5.4.3 4-layer defense: negative triggers + constitutional rules + scaffold template + validation script
### 5.5 Validation Gates
#### 5.5.1 go build → go vet → interface satisfaction (var _ = ...) → unit tests → boundary check
#### 5.5.2 Automated via Claude Code hooks or CI script
### 5.6 AI Codegen Research
#### 5.6.1 Go SWE-bench resolve rate ~30%; Claude 4.5 Sonnet 46-75% with tool creation
#### 5.6.2 No published benchmark on Go interface implementation tasks — custom benchmark needed

## 6. Filesystem Strategy Matrix
### 6.1 VM Providers: Slow Export (Minutes to Hours)
#### 6.1.1 AWS: export-image to S3 (VMDK/VHD); EBS Direct APIs for block-level access
#### 6.1.2 GCP: disk.raw in tar.gz to Cloud Storage
#### 6.1.3 Azure: disk export SAS URL → VHD download
#### 6.1.4 DigitalOcean/Hetzner: NO snapshot download API; tar-over-SSH required
### 6.2 Sandbox Providers: Fast Export (Seconds)
#### 6.2.1 Daytona: upload_file/download_file with batch support
#### 6.2.2 Modal: filesystem snapshots (diff-based), directory snapshots, memory snapshots
#### 6.2.3 E2B: read/write but no directory upload/download
#### 6.2.4 Fly.io: rootfs ephemeral; only Volume snapshots (block-level)
### 6.3 Self-Hosted: Gold Standard
#### 6.3.1 Docker: docker export → flat tar in seconds (reference implementation)
#### 6.3.2 Incus: incus export → backup.tar.gz
#### 6.3.3 Firecracker: manual rootfs construction; no native export
### 6.4 Universal Fallbacks
#### 6.4.1 tar-over-SSH for VMs; tar-over-exec for sandboxes
#### 6.4.2 Cloud-init + object storage (S3/GCS) for import
### 6.5 Migration Design Implications
#### 6.5.1 Fast exporters enable live migration; slow exporters require scheduled downtime
#### 6.5.2 Capability tiers: FastExporter / SlowExporter / NoExporter extension interfaces

## 7. Distribution Model Recommendation
### 7.1 Distribution Models Evaluated
#### 7.1.1 Go modules (one repo per adapter): simplest, decentralized, versioned
#### 7.1.2 OCI registry: emerging standard but overkill for Phase 1
#### 7.1.3 Embedded in binary: go:embed or build tags limit flexibility
#### 7.1.4 Generated on-demand: loses reproducibility and auditability
### 7.2 Versioning Strategy
#### 7.2.1 Independent semantic versioning per adapter (Terraform/Caddy precedent)
#### 7.2.2 Not locked to Mesh core version
### 7.3 Security Model
#### 7.3.1 AI-generated code is untrusted by default: 45% fail security tests
#### 7.3.2 Sandboxed CI validation (E2B/Daytona) before trusted registry admission
#### 7.3.3 Static analysis + gosec + govulncheck on generated code
### 7.4 Discovery Mechanism
#### 7.4.1 Blank-import side-effect registration (import _ "mesh/plugin/aws")
#### 7.4.2 Optional JSON registry file for metadata (name, version, capabilities)
### 7.5 Bootstrap Problem
#### 7.5.1 Docker adapter exists as reference implementation
#### 7.5.2 Zero-cost validation stack: AWS free tier + Daytona credits

## 8. Competitive Landscape
### 8.1 Directly Relevant Systems
#### 8.1.1 Daytona: "composable computer" for AI agents; three-plane architecture
#### 8.1.2 E2B: OSS Firecracker sandboxes; template/registry model
#### 8.1.3 Modal: serverless containers; beta Go SDK; advanced snapshots
#### 8.1.4 Fly.io Machines: explicit state machine API (created/started/stopped/destroyed)
### 8.2 Adjacent/Competitive Systems
#### 8.2.1 Kubernetes + Knative: enterprise approach, overengineered for Mesh scale
#### 8.2.2 HashiCorp Nomad: driver model (Docker, exec, qemu, podman)
#### 8.2.3 OpenFaaS: certifier pattern for provider compliance — steal this
#### 8.2.4 GitHub Codespaces / Coder / DevPod: dev environment provisioning models
### 8.3 Research/Experimental
#### 8.3.1 WebAssembly: Envoy/Shopify/Figma plugins — not for general compute adapters
#### 8.3.2 Agent-as-a-Service platforms: LangChain, AutoGPT — no compute abstraction
### 8.4 Mesh's Unique Position
#### 8.4.1 No competitor generates adapters automatically from API specs
#### 8.4.2 Mesh occupies the whitespace: self-hosted + AI-generated + portable + simple

## 9. Go Implementation Patterns
### 9.1 Interface Design
#### 9.1.1 Compile-time satisfaction check: var _ SubstrateAdapter = (*ProviderAdapter)(nil)
#### 9.1.2 Mandatory for all generated adapters
### 9.2 Optional Capabilities
#### 9.2.1 Extension interface pattern: SubstrateAdapter + FilesystemAdapter + Snapshotter
#### 9.2.2 Type assertion at runtime: if fa, ok := adapter.(FilesystemAdapter); ok { ... }
#### 9.2.3 Avoid ErrNotSupported as primary pattern; use it only at application boundary
### 9.3 Context and Error Handling
#### 9.3.1 context.Context as first parameter on all methods; never store in structs
#### 9.3.2 Error wrapping: fmt.Errorf("verb: %w", err) at adapter boundary
#### 9.3.3 Sentinel errors for known conditions: ErrNotSupported, ErrNotFound
### 9.4 Code Generation Integration
#### 9.4.1 //go:generate directives for triggering adapter generation
#### 9.4.2 Generated files must include "Code generated by ...; DO NOT EDIT."
#### 9.4.3 CI verifies with go generate ./... && git diff --exit-code
### 9.5 Generics and Build Tags
#### 9.5.1 Generics unnecessary for 8 concrete methods — no major Go project uses them for adapters
#### 9.5.2 Build tags only for excluding adapters from specific builds, not for selection
### 9.6 Prescriptive Rules for AI Agents
#### 9.6.1 Explicit do/don't list: DO use context.Context, DO use typed structs, DON'T use map[string]interface{}
#### 9.6.2 Pattern enforcement via static analysis (staticcheck, go vet)

## 10. Key Decisions Pending
### 10.1 Architecture Decisions
#### 10.1.1 In-process registry vs gRPC isolation for untrusted adapters: start simple, measure security needs
#### 10.1.2 Capability tiers vs single optional interface for filesystem operations
### 10.2 Provider Selection
#### 10.2.1 E2B: invest in Go REST client or deprioritize due to no official SDK?
#### 10.2.2 AWS vs Hetzner as first VM adapter: enterprise demand vs simplicity for pipeline validation
### 10.3 Code Generation
#### 10.3.1 Custom benchmark needed: "AI generating Go interface implementations" has no published baseline
#### 10.3.2 oneOf/anyOf handling strategy across all providers with polymorphic responses
### 10.4 Security and Trust
#### 10.4.1 Human review requirement for AI-generated adapters: all adapters or only first N?
#### 10.4.2 Sandboxed CI validation provider choice: Daytona vs E2B vs Modal
### 10.5 Ecosystem
#### 10.5.1 Monorepo vs separate repos per adapter: Caddy precedent favors separate
#### 10.5.2 Registry file format: JSON vs YAML vs Go package metadata

# References
## Research Dimension Files
- **Type**: research artifacts
- **Path**: /mnt/agents/output/research/mesh_dim01.md through mesh_dim12.md
- **Description**: 12 dimension deep-dive reports

## Cross-Verification
- **Type**: synthesis
- **Path**: /mnt/agents/output/research/mesh_cross_verification.md
- **Description**: Confidence tiers and contradiction analysis

## Insights
- **Type**: synthesis
- **Path**: /mnt/agents/output/research/mesh_insight.md
- **Description**: 10 cross-dimension strategic insights
