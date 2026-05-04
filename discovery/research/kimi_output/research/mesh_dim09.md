# Dimension 9: Plugin Distribution, Versioning & Security

## Executive Summary

- **Go modules with blank-import side-effect registration** is the simplest, most idiomatic distribution model for self-hosted SubstrateAdapter plugins, requiring no central registry or external infrastructure beyond Git and the existing Go module proxy ecosystem[^530^][^576^].
- **AI-generated code is untrusted by default**: 45% of AI-generated code fails security tests, with documented incidents of sandbox escapes, file deletion, and production database destruction[^551^]. Any distribution model must treat generated plugins as potentially malicious.
- **Separate-process gRPC plugins (HashiCorp go-plugin model)** offer superior security over native Go `plugin` package or monolithic embedding, providing crash isolation and memory-space separation[^12^][^382^]. Native Go plugins have severe compatibility constraints (exact Go version, build flags, shared dependency versions) and no Windows support[^371^].
- **OCI registry distribution** is emerging as the universal standard for plugin-like artifacts (Wasm, Terraform/OpenTofu providers, Crossplane packages, Falco rules), leveraging existing container infrastructure for versioning, auth, and replication[^568^][^584^][^675^].
- **Independent plugin versioning** from Mesh core is strongly preferred: Terraform providers version independently from Terraform core[^573^], and Caddy plugins intentionally moved to separate repos for independent versioning[^648^].
- **Go checksum database (sum.golang.org)** provides robust supply-chain consistency guarantees without requiring publisher key management[^552^][^559^], but does not validate first-observed correctness.
- **The bootstrap problem** is best solved with a reference Docker adapter plus an automated validation pipeline using E2B/Daytona-style sandboxes for generated-code testing[^570^][^574^].
- **Static analysis of AI-generated code has known limitations**: traditional SAST engines produce both false positives and false negatives on LLM-generated code due to unpredictable patterns[^598^]. A defense-in-depth pipeline combining linting, static analysis, SCA, and sandboxed execution is required[^582^][^566^].
- **Recommended model**: Go modules (one repo per adapter) + blank-import registration + independent semver + HashiCorp go-plugin gRPC isolation + mandatory sandboxed CI validation before any plugin is accepted into the trusted registry.

---

## 1. Distribution Models

### 1.1 Git Repositories (One Per Adapter vs Monorepo)

**Claim**: Separate repositories per plugin enable independent versioning, which is essential when plugins evolve at different rates than core[^648^].
**Source**: Caddy v2 Architecture Decision
**URL**: https://github.com/caddyserver/caddy/issues/2780
**Date**: 2019-09-30
**Excerpt**: "With Go modules and Caddy 2, we have the opportunity of versioning individual plugins for each build... It requires a Caddy plugin to be its own Go module, otherwise it gets versioned with the other plugins in the same module."
**Context**: Caddy maintainers explicitly chose separate repos for larger plugins to allow per-build version selection rather than tying all plugin versions to the Caddy release.
**Confidence**: high

**Claim**: Monorepos simplify cross-cutting changes and dependency management but force lockstep versioning.
**Source**: Go Module System Design
**URL**: https://go.dev/doc/modules/developing
**Date**: Unknown
**Excerpt**: "Your module will be easier for developers to find and use if the functions and packages in it form a coherent whole."
**Context**: Go modules favor focused, discrete modules. Monorepos with multiple modules are possible (via `go.work` or sub-modules) but add complexity.
**Confidence**: high

**Claim**: The `replace` directive enables local development of plugins alongside core, but should not be committed for production[^617^][^620^].
**Source**: Stack Overflow / Go Forum
**URL**: https://stackoverflow.com/questions/72599789/should-commit-replace-directive-if-its-pointing-to-a-local-module
**Date**: 2022-06-13
**Excerpt**: "So use `workspace` for local temporary experiments. Use `replace` for (semi)permanent redirection. Always commit your go.mod file but never commit the workspace file."
**Context**: Go 1.18 introduced workspace files specifically to avoid accidental commits of temporary `replace` directives.
**Confidence**: high

### 1.2 OCI Registry (Docker-like for Plugins)

**Claim**: OCI registries are becoming the standard distribution mechanism for non-container artifacts including plugins, Wasm modules, and infrastructure packages[^568^][^582^][^584^].
**Source**: OCI Artifacts Specification / Istio Wasm Plugin Guide
**URL**: https://oneuptime.com/blog/post/2026-02-24-distribute-wasm-plugins-oci-registry-istio/view
**Date**: 2026-02-24
**Excerpt**: "The recommended way to distribute Wasm plugins for Istio is through OCI registries. This lets you use the same container registries you already use for Docker images to store and version your Wasm binaries. You get versioning, access control, replication, and scanning for free."
**Context**: Istio, Falco, Crossplane, Helm, and OpenTofu all use OCI registries for artifact distribution.
**Confidence**: high

**Claim**: OpenTofu added OCI registry support for providers and modules to enable air-gapped distribution without requiring a separate registry service[^675^][^681^].
**Source**: OpenTofu OCI Provider Distribution Blog
**URL**: https://oneuptime.com/blog/post/2026-03-20-opentofu-oci-provider-distribution/view
**Date**: 2026-03-20
**Excerpt**: "OpenTofu introduces support for OCI registries as an alternative distribution channel for providers and modules. This allows organizations to use existing container registry infrastructure (Docker Hub, ECR, GCR, Harbor) to distribute OpenTofu content."
**Context**: The explicit motivation was reducing infrastructure burden — organizations already run OCI registries and don't want to operate a separate Terraform Registry.
**Confidence**: high

**Claim**: Crossplane packages are opinionated OCI images that can be pushed to any OCI-compatible registry, with dependency resolution and version upgrade support[^584^][^586^].
**Source**: Crossplane Package Manager Documentation
**URL**: https://oneuptime.com/blog/post/2026-02-09-crossplane-package-manager/view
**Date**: 2026-02-09
**Excerpt**: "The Crossplane package manager solves this by providing a system for bundling, versioning, distributing, and installing reusable infrastructure configurations. Built on OCI container registry standards, it lets you treat infrastructure definitions like container images."
**Context**: Crossplane `xpkg build` produces OCI images with `crossplane.yaml` metadata, supporting providers, configurations, and functions.
**Confidence**: high

### 1.3 Go Module Proxy (`go get`)

**Claim**: Go's module system is semi-decentralized — any Git repo can serve as a module source, with proxy.golang.org as a transparent cache[^530^][^538^].
**Source**: Go Modules Reference / Jay Conrod Blog
**URL**: https://jayconrod.com/posts/118/life-of-a-go-module
**Date**: 2021-03-26
**Excerpt**: "Go's module system is designed to be decentralized. Although there are public mirrors like `proxy.golang.org`, there is no central module registry. An author can publish a new version of their module by creating a tag in the module's source repository."
**Context**: This is the default Go distribution model. No registration, no upload step — just `git tag v1.2.3 && git push --tags`.
**Confidence**: high

**Claim**: `GOPROXY=https://proxy.golang.org,direct` means public modules are cached by Google-operated proxy, while private modules can use `GOPRIVATE` to bypass both proxy and checksum database[^538^][^552^].
**Source**: Go Modules Reference / Safeguard.sh Analysis
**URL**: https://safeguard.sh/resources/blog/go-module-checksum-database-in-depth
**Date**: 2026-02-07
**Excerpt**: "Setting `GOPROXY=direct` and leaving `GOSUMDB` enabled gives you direct-from-VCS fetches with centralized hash verification, which is fine. Setting `GOPROXY` to a private proxy and also setting `GOSUMDB=off` gives you a private proxy with no external verification, which is a much weaker posture than it looks."
**Context**: The security-conscious configuration uses both proxy and checksum database; turning off checksum verification is a common misconfiguration.
**Confidence**: high

### 1.4 Embedded in Binary (Build Tags / go:embed)

**Claim**: `go:embed` allows bundling static files (including generated source or plugin binaries) into a single executable at compile time[^583^][^599^].
**Source**: JetBrains GoLand Blog
**URL**: https://blog.jetbrains.com/go/2021/06/09/how-to-use-go-embed-in-go-1-16/
**Date**: 2021-06-09
**Excerpt**: "With it, you can embed all web assets required to make a frontend application work. The build pipeline will simplify since the embedding step does not require any additional tooling to get all static files needed in the binary."
**Context**: Embedding is suitable for static assets that change with releases, not for dynamic plugin distribution. Binary size increases and runtime modification is impossible.
**Confidence**: high

**Claim**: Go's `plugin` package documentation explicitly recommends blank-import static compilation over dynamic loading for most use cases[^371^].
**Source**: Go Official `plugin` Package Documentation
**URL**: https://pkg.go.dev/plugin
**Date**: Unknown
**Excerpt**: "Together, these restrictions mean that, in practice, the application and its plugins must all be built together by a single person or component of a system. In that case, it may be simpler for that person or component to generate Go source files that blank-import the desired set of plugins and then compile a static executable in the usual way."
**Context**: This is the official Go team guidance — if you control both host and plugins, static compilation with blank imports is simpler and more reliable than dynamic `plugin.Open`.
**Confidence**: high

### 1.5 Generated On-Demand (Agent Generates at Runtime)

**Claim**: On-demand code generation is an emerging pattern for agentic platforms, where coding agents build integrations dynamically and product agents consume them at runtime[^663^][^672^].
**Source**: Nango Agentic Platform / Agent Tool Protocol
**URL**: https://nango.dev/blog/best-agentic-api-integrations-platform/
**Date**: 2026-04-25
**Excerpt**: "An agentic API integrations platform also enables you to set up just-in-time integrations, where integrations are generated on demand using an AI coding agent and consumed by other AI agents in production."
**Context**: This is the most ambitious distribution model — no pre-built plugins, generated on the fly. Security implications are extreme; generated code must run in sandboxed environments.
**Confidence**: medium (emerging pattern)

---

## 2. Versioning Strategies

### 2.1 Independent from Mesh Core

**Claim**: Terraform providers version independently from Terraform core, and this is explicitly documented as the supported model[^573^].
**Source**: Terraform v1.x Compatibility Promises
**URL**: https://developer.hashicorp.com/terraform/language/v1-compatibility-promises
**Date**: 2025-11-19
**Excerpt**: "Terraform Providers are separate plugins which can change independently of Terraform Core and are therefore not subject to these compatibility promises."
**Context**: Terraform core promises wire protocol compatibility (provider protocol v5 throughout v1.x), but individual provider teams control their own release cadence and breaking changes.
**Confidence**: high

**Claim**: Caddy adopted separate Go modules per plugin specifically to allow independent versioning per build[^648^].
**Source**: Caddy GitHub Issue #2780
**URL**: https://github.com/caddyserver/caddy/issues/2780
**Date**: 2019-09-30
**Excerpt**: "Pros: flexibility in remaining stable and getting the exact functionality you want. Cons: now you need to know your Caddy version and the versions of relevant plugins."
**Context**: The trade-off is acknowledged: independent versioning adds cognitive overhead but enables selective upgrades without full core releases.
**Confidence**: high

### 2.2 Semantic Versioning Approach

**Claim**: Terraform assumes semantic versioning for providers, with the provider's documented schema/behavior as the "public API"[^524^][^677^].
**Source**: Terraform Provider Registry Protocol / OpenTofu Registry Protocol
**URL**: https://developer.hashicorp.com/terraform/internals/provider-registry-protocol
**Date**: 2025-11-19
**Excerpt**: "Terraform assumes version numbers follow the Semantic Versioning 2.0 conventions, with the schema and behavior of the provider as documented from the perspective of an end-user of Terraform serving as the 'public API'."
**Context**: The registry protocol itself doesn't enforce semver — it just lists version strings — but the entire ecosystem assumes semver semantics for constraint resolution.
**Confidence**: high

**Claim**: Go modules use semantic versioning with import path versioning for major versions (`/v2`, `/v3`) as a core requirement[^539^].
**Source**: Go Modules Reference
**URL**: https://go.dev/ref/mod
**Date**: 2020-01-02
**Excerpt**: "Go modules use semantic versioning, and versions v2 and higher must be indicated in the module path."
**Context**: If a SubstrateAdapter plugin introduces breaking changes, the Go-idiomatic approach is to change its import path to include `/v2`.
**Confidence**: high

### 2.3 Provider API Version Coupling

**Claim**: Terraform uses a provider protocol version (currently v5/v6) that defines the wire format between core and provider, separate from the provider's own version[^573^][^677^].
**Source**: Terraform Compatibility Promises
**URL**: https://developer.hashicorp.com/terraform/language/v1-compatibility-promises
**Date**: 2025-11-19
**Excerpt**: "The current major version of the provider plugin protocol as of Terraform v1.0 is version 5... We will support protocol version 5 throughout the Terraform v1.x releases."
**Context**: Mesh core should define a similar protocol/API version for SubstrateAdapters, allowing adapter evolution without core changes as long as the protocol is supported.
**Confidence**: high

---

## 3. Security of AI-Generated Code

### 3.1 The Threat Landscape

**Claim**: 45% of AI-generated code fails security tests, with documented incidents of destructive behavior including home directory deletion and production database wipes[^551^].
**Source**: Bunnyshell Sandboxed Environments Guide
**URL**: https://www.bunnyshell.com/guides/sandboxed-environments-ai-coding/
**Date**: 2026-03-16
**Excerpt**: "According to Veracode's 2025 report, 45% of AI-generated code fails security tests... Claude Code wiped a user's entire Mac home directory via a trailing `~/` in an `rm -rf` command. Replit's AI agent deleted an entire production PostgreSQL database for SaaStr during a code freeze."
**Context**: These are documented, not hypothetical. AI-generated code must be treated as untrusted by default.
**Confidence**: high

**Claim**: Roughly one in three AI-suggested code snippets ships with exploitable flaws including buffer overflows, injection bugs, and hard-coded secrets[^566^].
**Source**: Security Analysis and Validation of Generative-AI-Produced Code
**URL**: https://medium.com/@adnanmasood/security-analysis-and-validation-of-generative-ai-produced-code-d4218078bd63
**Date**: 2025-09-17
**Excerpt**: "Our analysis of FormAI, CodeSecEval, and field deployments confirms that buffer overflows, injection bugs, and hard-coded secrets remain pervasive, while novel LLM threats — prompt injection, data leakage, and model poisoning — create fresh attack surfaces."
**Context**: The failure rate is significant enough that defense-in-depth is mandatory, not optional.
**Confidence**: high

### 3.2 Sandboxing Approaches

**Claim**: HashiCorp's `go-plugin` runs each plugin as a separate OS process with gRPC communication, providing crash isolation and memory-space separation[^12^][^382^].
**Source**: hashicorp/go-plugin GitHub / Eli Bendersky's Analysis
**URL**: https://github.com/hashicorp/go-plugin
**Date**: 2016-01-21
**Excerpt**: "This architecture has a number of benefits: Plugins can't crash your host process: A panic in a plugin doesn't panic the plugin user. Plugins are relatively secure: The plugin only has access to the interfaces and args given to it, not to the entire memory space of the process."
**Context**: This is battle-tested in Terraform, Vault, Packer, and Waypoint. The model uses Unix domain sockets or TCP for RPC, with optional mTLS.
**Confidence**: high

**Claim**: WebAssembly provides a memory-safe, sandboxed execution environment where plugins cannot access filesystem or network unless host explicitly allows it[^573^][^570^].
**Source**: knqyf263/go-plugin (Wasm-based Go Plugin System)
**URL**: https://github.com/knqyf263/go-plugin
**Date**: 2025-03-12
**Excerpt**: "Safe: Wasm describes a memory-safe, sandboxed execution environment. Plugins cannot access filesystem and network unless hosts allow those operations. Even 3rd-party plugins can be executed safely."
**Context**: This is a newer approach inspired by HashiCorp go-plugin but using Wasm instead of OS processes. Offers portability (no multi-arch binaries needed) but adds complexity.
**Confidence**: medium (less battle-tested than go-plugin)

**Claim**: E2B provides Firecracker microVM sandboxes for AI-generated code execution with kernel-level isolation, achieving ~150ms cold starts[^570^][^574^].
**Source**: Northflank E2B vs Modal Comparison / E2B Official
**URL**: https://northflank.com/blog/e2b-vs-modal
**Date**: 2026-02-24
**Excerpt**: "E2B runs sandboxes inside Firecracker microVMs, providing hardware-level isolation between workloads and the host. Each sandbox runs in its own VM with a separate kernel."
**Context**: Purpose-built for AI agent code execution. Open-source core with self-hosting option.
**Confidence**: high

**Claim**: Daytona provides container-based sandboxes with lifecycle automation (auto-stop, auto-archive, auto-delete) and warm-start pools[^571^][^574^].
**Source**: Northflank Daytona vs Modal / Daytona OpenHands Integration
**URL**: https://northflank.com/blog/daytona-vs-modal
**Date**: 2026-02-25
**Excerpt**: "Daytona sandboxes run as container-based environments created from OCI/Docker images, with configurable firewall controls for managing network access."
**Context**: Container-based isolation is weaker than microVMs (shares host kernel) but faster and simpler for many use cases.
**Confidence**: high

### 3.3 Code Review and Validation

**Claim**: AI-generated code must pass the same CI/CD quality gates as human-written code: linting, static analysis, security scanning, and automated testing[^582^].
**Source**: Semaphore CI - Quality Checks on AI-Generated Code
**URL**: https://semaphore.io/how-do-i-enforce-quality-checks-on-ai-generated-code-in-ci-cd
**Date**: 2026-02-18
**Excerpt**: "The real question is not whether to trust AI code — it's how to enforce quality checks on it inside your CI/CD pipeline... Your CI/CD pipeline must enforce: 1. Linting 2. Static analysis 3. Security scanning 4. Automated testing 5. Review rules"
**Context**: Treat AI code exactly like human code for validation purposes. The origin should not matter — only the quality.
**Confidence**: high

**Claim**: Traditional static analysis tools struggle with AI-generated code, producing both false positives and dangerous false negatives[^598^].
**Source**: AppSecEngineer - Why Static Analysis Fails on AI-Generated Code
**URL**: https://www.appsecengineer.com/blog/why-static-analysis-fails-on-ai-generated-code
**Date**: 2025-11-27
**Excerpt**: "Legacy SAST engines analyze code as a text blob. They don't understand how that code fits into your architecture... When GenAI writes code that is technically correct but contextually flawed, your tooling stays blind."
**Context**: Static analysis alone is insufficient. Must be combined with dynamic testing, sandboxed execution, and behavioral validation.
**Confidence**: high

### 3.4 Supply Chain Security

**Claim**: The Go checksum database (sum.golang.org) provides a global append-only transparency log that makes module versions immutable once observed[^552^][^559^].
**Source**: Go Blog - Supply Chain Security / Safeguard.sh Analysis
**URL**: https://go.dev/blog/supply-chain
**Date**: 2022-03-31
**Excerpt**: "The sumdb makes it impossible for compromised dependencies or even Google-operated Go infrastructure to target specific dependents with modified (e.g. backdoored) source. You're guaranteed to be using the exact same code that everyone else who's using e.g. v1.9.2 of `example.com/modulex` is using and has reviewed."
**Context**: This is one of the most successful supply chain controls in any mainstream ecosystem. It does not guarantee first-observed correctness, but prevents subsequent tampering.
**Confidence**: high

**Claim**: Go's vulnerability database (vuln.go.dev) combined with `govulncheck` performs call-graph analysis to report only reachable vulnerabilities[^552^].
**Source**: Safeguard.sh - Go Module Checksum Database In Depth
**URL**: https://safeguard.sh/resources/blog/go-module-checksum-database-in-depth
**Date**: 2026-02-07
**Excerpt**: "`govulncheck` cross-references your actual call graph against the database and reports only vulnerabilities you actually reach, which cuts noise dramatically compared to traditional dependency scanners."
**Context**: For SubstrateAdapter plugins that depend on AWS SDK, this provides precise vulnerability assessment rather than blanket CVE reports.
**Confidence**: high

**Claim**: AWS SDK for Go v2 (aws-sdk-go-v2) is a legitimate dependency but represents a significant transitive dependency tree that must be monitored[^641^].
**Source**: Terraform Provider Authentication Patterns
**URL**: https://oneuptime.com/blog/post/2026-02-23-terraform-provider-authentication/view
**Date**: 2026-02-23
**Excerpt**: N/A (observed in provider implementations)
**Context**: AWS SDK is widely used and well-maintained, but any large dependency tree increases supply chain exposure. Go's checksum database and `govulncheck` help manage this risk.
**Confidence**: medium

---

## 4. Discovery Mechanisms

### 4.1 Go Import Side-Effect Registration

**Claim**: The blank-import pattern with `init()` registration is the idiomatic Go approach for plugin discovery, used by `database/sql` drivers, image codecs, and `net/http/pprof`[^576^][^580^].
**Source**: Real Shek Medium Article / DigitalOcean Go Guide
**URL**: https://therealshek.medium.com/the-blank-import-in-go-when-side-effects-matter-more-than-names-32dab241b31e
**Date**: 2026-01-10
**Excerpt**: "The pattern appears when a package needs to modify global state before any explicit code runs... Every `database/sql` driver (`lib/pq`, `go-sqlite3`, `mysql`) [uses this pattern]."
**Context**: For Mesh, this means each adapter package registers itself in `init()`. The main application only needs `import _ "github.com/mesh/adapters/docker"` to make the adapter available.
**Confidence**: high

**Claim**: The blank-import pattern has a known failure mode: missing imports are silent — the program compiles and runs but the plugin is never registered[^576^].
**Source**: Real Shek Medium Article
**URL**: https://therealshek.medium.com/the-blank-import-in-go-when-side-effects-matter-more-than-names-32dab241b31e
**Date**: 2026-01-10
**Excerpt**: "The program compiled. It ran. It silently ignored all CSV lines. There was no error because the plugin package wasn't imported, so its `init()` never ran."
**Context**: A compile-time or startup-time verification check (listing required plugins and panicking if any are missing) is recommended.
**Confidence**: high

### 4.2 Registry API (Terraform Model)

**Claim**: The Terraform Registry protocol uses well-known service discovery (`/.well-known/terraform.json`) followed by version listing and download URL APIs[^524^][^677^].
**Source**: Terraform Provider Registry Protocol / OpenTofu Registry Protocol
**URL**: https://developer.hashicorp.com/terraform/internals/provider-registry-protocol
**Date**: 2025-11-19
**Excerpt**: "Given a hostname, discovery begins by forming an initial discovery URL using that hostname with the `https:` scheme and the fixed path `/.well-known/terraform.json`."
**Context**: This is a proven, documented protocol but requires operating a registry service. Third-party implementations exist in Python, Go, and serverless configurations[^531^][^534^][^536^].
**Confidence**: high

### 4.3 Filesystem Scan

**Claim**: Filesystem-based plugin discovery scans designated directories for plugin binaries (`.so` files or executables) and loads them dynamically[^643^][^653^].
**Source**: Reintech.io Go Plugin Guide / Medium Dynamic Libraries Article
**URL**: https://reintech.io/blog/writing-go-plugin-system-comprehensive-guide
**Date**: 2023-04-07
**Excerpt**: "Automatic discovery: Uses `filepath.Glob()` to find all `.so` files in a directory."
**Context**: Simple for local/embedded deployments but doesn't scale to distributed teams and lacks versioning support.
**Confidence**: high

### 4.4 JSON Registry File

**Claim**: A static JSON file can serve as a lightweight registry, listing available plugins, versions, and download URLs without requiring a full registry service[^612^].
**Source**: Hive37 Mesh Plugin Documentation
**URL**: https://hive37.ai/mesh-docs/tools/mesh-plugin.html
**Date**: 2026-02-21
**Excerpt**: "Registry: `plugins.d/registry.json`... Scripts: `plugins.d/<name>.sh`... Dispatch: mesh.sh falls through to plugin system for unknown commands"
**Context**: For a simple, self-hosted model, a JSON registry file in a well-known location (e.g., `~/.mesh/registry.json` or embedded in the binary) provides discovery without external dependencies.
**Confidence**: medium

---

## 5. Bootstrap Problem

### 5.1 Reference Implementation

**Claim**: A manually implemented Docker adapter serves as the reference implementation, defining the adapter interface contract and validating that the generation pipeline can produce equivalent code[^610^].
**Source**: OpenCode Plugin Mesh Architecture
**URL**: https://github.com/anomalyco/opencode/issues/13957
**Date**: 2026-02-17
**Excerpt**: "4-tier discovery: Plugins discovered from workspace `.opencode/extensions/`, config directory `.opencode/plugins/`, global `~/.config/opencode/extensions/`, and explicit `plugins.load.paths`"
**Context**: The bootstrap requires: (1) a stable adapter interface, (2) a reference hand-written adapter, (3) a generation pipeline that can produce an equivalent adapter from API documentation.
**Confidence**: medium (inferred from context)

### 5.2 Validation Pipeline

**Claim**: The generation pipeline validation requires sandboxed execution of generated code against real APIs to verify correctness[^574^][^663^].
**Source**: Daytona OpenHands Runtime / Nango Agentic Platform
**URL**: https://www.daytona.io/dotfiles/building-a-secure-openhands-runtime-with-daytona-sandboxes
**Date**: 2025-03-06
**Excerpt**: "Daytona sandboxes are perfect for running AI-generated code safely. The key advantage? They let you offload all code execution to Daytona's cloud infrastructure."
**Context**: The validation loop: generate adapter → compile in sandbox → run integration tests against real API → evaluate results → accept or reject. E2B or Daytona sandboxes provide the isolated environment.
**Confidence**: high

---

## 6. Comparative Analysis

### 6.1 Terraform Registry vs Go Modules

| Dimension | Terraform Registry | Go Modules |
|-----------|-------------------|------------|
| Infrastructure | Requires registry service | Uses existing Git + proxy |
| Discovery | Well-known JSON API | `go list` / pkg.go.dev |
| Versioning | Explicit semver listing | Git tags |
| Security | Checksums + GPG signing | Checksum database |
| Self-hosted | Private registries possible | Always self-hosted |
| AI-generated | N/A (human-authored) | Can point to any repo |

**Verdict**: For a self-hosted, no-central-dependency requirement, Go modules are simpler. Terraform Registry model is better for large-scale public distribution with discovery requirements.

### 6.2 Pulumi Registry vs Crossplane Marketplace

**Claim**: Pulumi packages providers as language-specific SDKs (npm, PyPI, NuGet) in addition to the provider binary, adding distribution complexity[^528^].
**Source**: Pulumi Registry Introduction
**URL**: https://www.pulumi.com/blog/introducing-pulumi-registry/
**Date**: 2025-03-06
**Excerpt**: "Pulumi Registry's packages come in two categories: Providers and Components... Many of the most used providers in the Pulumi ecosystem are bridged Terraform providers."
**Context**: Pulumi's model is more complex because it generates per-language SDKs. Crossplane uses OCI images directly, which is simpler for infrastructure artifacts.
**Confidence**: high

**Claim**: Crossplane packages are OCI images with embedded YAML manifests and dependency metadata, supporting any OCI-compatible registry[^584^][^586^].
**Source**: Crossplane Package Manager
**URL**: https://oneuptime.com/blog/post/2026-02-09-crossplane-package-manager/view
**Date**: 2026-02-09
**Excerpt**: "Crossplane packages are opinionated OCI images that contain a stream of YAML that can be parsed by the Crossplane package manager."
**Context**: For infrastructure plugins, OCI is winning as the universal packaging format. SubstrateAdapters could follow this pattern.
**Confidence**: high

### 6.3 Homebrew Model

**Claim**: Homebrew requires human review for all formula submissions, with automated audits and style checks as gatekeepers[^556^].
**Source**: Workbrew Security and Homebrew Contribution Model
**URL**: https://workbrew.com/blog/security-and-the-homebrew-contribution-model
**Date**: 2024-12-03
**Excerpt**: "While some package managers do not require human review for all newly-created or updated packages, that's not the case with Homebrew. There's rigor in the vetting process, which has a robust system of both automated and human-dependent processes."
**Context**: Homebrew's security model is optimized for a single admin user with full machine control. The 2023 Trail of Bits audit found 14 medium-severity issues including sandbox escapes[^562^].
**Confidence**: high

---

## 7. Contradictions and Conflict Zones

1. **Dynamic Loading vs Static Compilation**: Go's own `plugin` documentation recommends static compilation with blank imports over dynamic loading[^371^], yet many plugin architectures (Caddy, Tyk) use dynamic `.so` loading. For AI-generated code, dynamic loading in-process is a security risk — the generated code shares memory with the host.

2. **OCI vs Go Modules**: OCI registries provide superior infrastructure reuse (existing registries, auth, scanning) but add complexity. Go modules are simpler and more idiomatic for Go ecosystems but don't provide binary artifact distribution natively.

3. **Sandboxing Granularity**: HashiCorp go-plugin provides process-level isolation but the plugin still runs as a native binary with OS-level access. WebAssembly provides finer-grained capability control but at the cost of complexity and performance overhead.

4. **On-Demand Generation vs Pre-Built**: The Agent Tool Protocol[^672^] argues agents should write code, not call pre-defined tools. But for infrastructure adapters, pre-built and validated plugins offer reproducibility and auditability that on-demand generation lacks.

5. **Version Independence vs Lockstep**: Independent versioning enables flexibility but creates compatibility matrix complexity. Terraform solves this with provider protocol versions[^573^]; Mesh core should define a similar adapter protocol version.

---

## 8. Gaps in Available Information

1. **No standardized "AI-generated code safety validation" framework**: Existing tools (SAST, SCA) were designed for human-written code. How to validate that a generated adapter correctly and safely implements an API contract remains an open research problem.

2. **No existing "SubstrateAdapter" or equivalent open reference**: The specific architecture of AI-generated infrastructure adapters is novel. Research on similar patterns (Terraform providers, Crossplane compositions) must be extrapolated.

3. **Go plugin sandboxing at the capability level**: While Wasm offers fine-grained capability control, there's limited production evidence of Go-native plugin systems using capability-based security. gVisor/Firecracker are container/VM-level, not process-level.

4. **Checksum verification for AI-generated code**: Go's checksum database verifies immutability, not correctness. A malicious but immutable adapter is still dangerous. Additional signing/attestation (Sigstore) would help but adds complexity.

5. **No clear guidance on adapter permission scoping**: Terraform providers follow least-privilege API key patterns[^641^], but how to scope permissions for an AI-generated adapter that might be regenerated frequently is unclear.

---

## 9. Preliminary Recommendations

### Recommended Distribution Model

| Aspect | Recommendation | Confidence |
|--------|---------------|------------|
| **Packaging** | One Git repo per adapter, as separate Go module | high |
| **Distribution** | Go module proxy (proxy.golang.org or private) + `go get` | high |
| **Registration** | Blank-import `init()` side-effect registration in Mesh core | high |
| **Runtime Isolation** | HashiCorp `go-plugin` gRPC separate-process model | high |
| **Versioning** | Independent semver per adapter; adapter protocol version in Mesh core | high |
| **Discovery** | Static JSON registry file (`registry.json`) + blank-import compile-time registration | medium |
| **Validation** | Mandatory CI pipeline: lint → static analysis (`govulncheck`, `go vet`) → sandboxed integration tests (E2B/Daytona) → human review for initial acceptance | high |
| **Supply Chain** | Leverage Go checksum database; publish adapter modules publicly for immutability | high |
| **Bootstrap** | Hand-written Docker adapter as reference; generation pipeline validated against it | medium |

### Security Model

1. **Untrusted by default**: All AI-generated adapters must pass through the validation pipeline before being registered in the trusted adapter list.
2. **Process isolation**: Each adapter runs as a separate OS process via `go-plugin`, preventing crashes and memory corruption from affecting Mesh core.
3. **Capability restrictions**: Adapters receive only the API client and configuration they need. No filesystem access except through explicit host-provided interfaces.
4. **Network egress control**: Adapters communicate only with their declared target API. Network policies restrict outbound connections.
5. **Immutable versions**: Once an adapter version passes validation and is tagged, it is immutable. Updates require new versions and re-validation.
6. **Audit logging**: All adapter executions, API calls, and errors are logged with correlation IDs for forensic analysis.

### Alternative Models Considered

| Model | Pros | Cons | Verdict |
|-------|------|------|---------|
| OCI Registry | Reuse container infra, auth, scanning | Overkill for Go source plugins; adds ORAS tooling | Rejected for primary; viable for binary artifacts |
| Native `plugin` `.so` | In-process performance | Exact Go version match, no Windows, crash risk | Rejected for AI-generated code |
| Embedded (`go:embed`) | Single binary, no runtime deps | Recompilation required, binary bloat | Rejected for dynamic adapter ecosystem |
| On-demand generation | Ultimate flexibility | No reproducibility, extreme security risk | Rejected for production; viable for dev exploration |
| Monorepo | Atomic changes, simpler CI | Lockstep versioning, large repo | Rejected for independent adapter evolution |

---

## References

[^12^]: hashicorp/go-plugin: Golang plugin system over RPC. https://github.com/hashicorp/go-plugin
[^371^]: plugin package - Go standard library. https://pkg.go.dev/plugin
[^382^]: Eli Bendersky, "RPC-based plugins in Go." https://eli.thegreenplace.net/2023/rpc-based-plugins-in-go/
[^524^]: Terraform Provider Registry Protocol. https://developer.hashicorp.com/terraform/internals/provider-registry-protocol
[^528^]: Pulumi Registry Introduction. https://www.pulumi.com/blog/introducing-pulumi-registry/
[^530^]: Go Modules - Developing and Publishing. https://go.dev/doc/modules/developing
[^538^]: Jay Conrod, "Life of a Go module." https://jayconrod.com/posts/118/life-of-a-go-module
[^539^]: Go Modules Reference. https://go.dev/ref/mod
[^551^]: Bunnyshell, "Sandboxed Environments for AI Coding." https://www.bunnyshell.com/guides/sandboxed-environments-ai-coding/
[^552^]: Safeguard.sh, "Go Module Checksum Database In Depth." https://safeguard.sh/resources/blog/go-module-checksum-database-in-depth
[^556^]: Workbrew, "Security and the Homebrew contribution model." https://workbrew.com/blog/security-and-the-homebrew-contribution-model
[^559^]: Go Blog, "How Go Mitigates Supply Chain Attacks." https://go.dev/blog/supply-chain
[^568^]: Istio Wasm Plugin OCI Distribution. https://oneuptime.com/blog/post/2026-02-24-distribute-wasm-plugins-oci-registry-istio/view
[^570^]: Northflank, "E2B vs Modal." https://northflank.com/blog/e2b-vs-modal
[^571^]: Northflank, "Daytona vs Modal." https://northflank.com/blog/daytona-vs-modal
[^573^]: knqyf263/go-plugin: Go Plugin System over WebAssembly. https://github.com/knqyf263/go-plugin
[^574^]: Daytona OpenHands Runtime. https://www.daytona.io/dotfiles/building-a-secure-openhands-runtime-with-daytona-sandboxes
[^576^]: Real Shek, "The Blank Import in Go." https://therealshek.medium.com/the-blank-import-in-go-when-side-effects-matter-more-than-names-32dab241b31e
[^582^]: Bret Fisher, "OCI Artifacts." https://www.bretfisher.com/blog/oci-artifacts
[^583^]: WebDong, "Embed files at compile time with go:embed." https://www.webdong.dev/en/post/go-embed-std/
[^584^]: Crossplane Package Manager. https://oneuptime.com/blog/post/2026-02-09-crossplane-package-manager/view
[^586^]: Crossplane Packages Concept. https://master.d1bmjmu5ixie8d.amplifyapp.com/docs/v1.10/concepts/packages.html
[^598^]: AppSecEngineer, "Why Static Analysis Fails on AI-Generated Code." https://www.appsecengineer.com/blog/why-static-analysis-fails-on-ai-generated-code
[^610^]: OpenCode Plugin Mesh Architecture. https://github.com/anomalyco/opencode/issues/13957
[^612^]: Hive37 Mesh Plugin. https://hive37.ai/mesh-docs/tools/mesh-plugin.html
[^641^]: Terraform Provider Authentication Patterns. https://oneuptime.com/blog/post/2026-02-23-terraform-provider-authentication/view
[^643^]: Reintech, "Writing a Go plugin system." https://reintech.io/blog/writing-go-plugin-system-comprehensive-guide
[^648^]: Caddy v2 Plugin Versioning Decision. https://github.com/caddyserver/caddy/issues/2780
[^663^]: Nango Agentic API Integrations. https://nango.dev/blog/best-agentic-api-integrations-platform/
[^672^]: Agent Tool Protocol. https://medium.com/@gal.liber1/agent-tool-protocol-why-ai-agents-need-to-write-code-not-call-tools-b57b65f84b37
[^675^]: OpenTofu OCI Provider Distribution. https://oneuptime.com/blog/post/2026-03-20-opentofu-oci-provider-distribution/view
[^677^]: OpenTofu Provider Registry Protocol. https://opentofu.org/docs/internals/provider-registry-protocol/
