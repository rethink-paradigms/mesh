## 7. Distribution Model Recommendation

Selecting a distribution model for AI-generated adapters requires balancing three constraints that rarely align: the user must retain full ownership (no central registry dependency), the system must remain self-hosted, and generated code must be treated as untrusted by default. This chapter evaluates five distribution models against six operational criteria and recommends a phased approach that starts with Go modules and reserves OCI registries for a future enterprise tier.

### 7.1 Distribution Models Evaluated

The evaluation scope covers five candidate models deployed in comparable ecosystems: Go modules (one repository per adapter), a monolithic single-repository layout, OCI registry distribution, embedding via `go:embed` or build tags, and fully on-demand generation at runtime.

| Model | Versioning Granularity | Security Audit Trail | Discovery Mechanism | Self-hosted Feasibility | Runtime Flexibility | Bootstrap Complexity |
|-------|----------------------|---------------------|--------------------|------------------------|--------------------|---------------------|
| Go modules (one repo per adapter) | Independent semver via git tags [^648^] | Go checksum database (`sum.golang.org`) provides immutability [^552^][^559^] | Blank-import `init()` registration [^576^] | High — any Git host works; no central registry [^530^][^538^] | High — add adapter with single import [^371^] | Low — reference Docker adapter + `go mod init` |
| Monorepo (single repo) | Lockstep with Mesh core release | Same as Go modules but all-or-nothing | Same as Go modules | High | Low — forces inclusion of all adapters [^648^] | Low but scales poorly |
| OCI registry | Explicit semver tags; reuse container infrastructure [^568^][^584^] | Registry-native scanning + Sigstore cosign [^675^] | Registry API or well-known JSON [^524^] | Medium — requires OCI-compatible registry service | Medium — binary artifacts need ORAS tooling | Medium — must package adapter as OCI image |
| Embedded binary (`go:embed`) | Tied to Mesh core binary version [^583^][^599^] | Build-time only; no post-ship audit | Compile-time build tags or static list | Very high — zero external deps | None — recompilation required for any change [^583^] | Low for initial build; high for maintenance |
| On-demand generation | Ephemeral; no reproducible version [^663^][^672^] | None — code vanishes after execution | N/A (agent generates locally) | Very high — no external distribution | Maximum — any provider, any time | High — requires full generation pipeline at runtime |

The Caddy project explicitly rejected monorepo lockstep for this reason, adopting separate Go modules per plugin so that "a Caddy plugin [can] be its own Go module, otherwise it gets versioned with the other plugins in the same module" [^648^]. Embedding and on-demand generation sit at opposite extremes — the former eliminates flexibility entirely, while the latter sacrifices reproducibility and auditability. OCI registries occupy the middle ground but impose infrastructure that exceeds Mesh's Phase 1 requirements. **Recommendation**: Go modules (one repo per adapter). **Confidence**: high.

### 7.2 Versioning Strategy

Adapters must version independently from Mesh core. Terraform providers are explicitly documented as "separate plugins which can change independently of Terraform Core" [^573^], and Caddy v2 adopted separate Go modules specifically to allow per-build version selection [^648^]. Go modules enforce this through import-path versioning: breaking changes require a `/v2` suffix in the module path, signaling downstream consumers without ambiguity [^539^]. Mesh core should define an adapter protocol version (analogous to Terraform's provider protocol v5/v6 [^573^]) that remains stable across core releases, preventing the "compatibility matrix explosion" that would otherwise occur if N adapters each had M core version combinations to validate.

### 7.3 Security Model

AI-generated code is untrusted by default. Veracode's 2025 analysis reports that 45% of AI-generated code fails security tests, with documented incidents ranging from sandbox escapes to production database destruction [^551^]. A separate study finds roughly one in three AI-suggested snippets ships with exploitable flaws including buffer overflows, injection bugs, and hard-coded secrets [^566^].

The defense-in-depth pipeline for adapter admission combines static analysis, supply-chain verification, and sandboxed execution. `go vet`, `gosec`, and `govulncheck` form the baseline; the latter cross-references the actual call graph against the Go vulnerability database, reporting only reachable flaws [^552^]. Generated adapters must then compile and pass integration tests inside an isolated sandbox — E2B provides Firecracker microVMs with ~150 ms cold starts [^570^][^574^], while Daytona offers container-based environments with warm-start pools [^571^][^574^]. Once validated, the Go checksum database (`sum.golang.org`) makes the tagged version immutable for all subsequent fetches [^552^][^559^].

**Steal this**: HashiCorp `go-plugin` runs each adapter as a separate OS process communicating over gRPC, providing crash isolation and memory-space separation [^12^][^382^]. Because Go uses implicit interface satisfaction, the same `SubstrateAdapter` interface can be satisfied by both an in-process struct (for trusted, hand-written adapters) and a gRPC client proxy (for AI-generated adapters). Adapters start in-process for debuggability and transparently escalate to gRPC isolation after passing the certifier suite.

### 7.4 Discovery Mechanism

Discovery follows the `database/sql` blank-import pattern. Each adapter package registers itself in `init()` — for example, `func init() { RegisterAdapter("docker", NewDockerAdapter) }` — so that a user only needs `import _ "github.com/mesh/adapters/docker"` to make the adapter available [^576^][^580^]. The known failure mode is the silent missing import: if the user forgets the blank import, the program compiles but the adapter is never registered [^576^]. Mitigate this with a startup-time verification step that panics if a configured adapter is absent from the registry.

For metadata — names, capabilities, version strings — a static JSON registry file (`registry.json`) at a well-known location provides discovery without requiring a registry service [^612^]. The file is optional; the blank-import mechanism alone is sufficient for compile-time registration, but the JSON file improves CLI listing and dashboard rendering.

### 7.5 Bootstrap Problem

The bootstrap requires a stable adapter interface, a reference hand-written adapter, and a validation pipeline that can confirm generated equivalents. The Docker adapter serves as the reference implementation: it validates the interface contract and provides a copyable template for the AI generation skill [^610^].

The validation stack for the first wave of adapters can operate at zero cost. AWS EC2 `t4g.small` instances remain free-tier eligible through December 2026, Daytona offers $200 in sandbox credits with no credit card required, and Docker runs locally without cloud spend [^570^][^574^]. These three providers — VM, sandbox, and container — cover the full substrate taxonomy. The recommended path is: (1) commit the Docker adapter as reference; (2) manually implement a second adapter (Daytona is recommended due to its official Go SDK) to prove the interface across a non-container substrate; (3) automate the generation pipeline against these two references; and (4) gate every generated adapter through the sandboxed CI pipeline before it is admitted to the trusted adapter list.
