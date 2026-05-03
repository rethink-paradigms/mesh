# 10. Key Decisions Pending

Research across twelve dimensions resolved most architectural questions with high confidence, but five categories remain unproven without empirical data from a working generation pipeline.

## Decision Framework

| Category | Option A | Option B | Recommendation |
|-----------|----------|----------|----------------|
| Architecture | In-process registry (database/sql pattern) [^341^] | gRPC isolation (go-plugin) [^12^] | Start in-process; graduate adapters to gRPC after passing security review |
| Provider priority | Hetzner (simple SDK, low cost) [^648^] | AWS (enterprise demand, massive SDK) [^1^] | Hetzner first to validate pipeline; AWS as Wave 2 with human-in-the-loop |
| Codegen validation | Custom benchmark for Go adapter generation | Reuse SWE-bench Multilingual (30.95% Go resolve rate) [^491^] | Build custom benchmark — no published baseline for "AI wrapping Go SDKs" |
| Security review | Review all AI-generated adapters | Review only first *N*, then auto-accept | Review first 3 adapters + random 10% sampling; measure defect rate before scaling |
| Ecosystem | Separate repo per adapter (Caddy precedent) [^648^] | Monorepo with sub-modules | Separate repos with `go.work` for local development; evaluate after 5 adapters |

## 10.1 Architecture Decisions

### 10.1.1 In-process registry versus gRPC isolation for untrusted adapters

Go's implicit interface satisfaction lets the same `SubstrateAdapter` contract be satisfied by both a local struct and a gRPC client proxy.[^12^][^382^] What remains unproven is the graduation trigger: go-plugin adds ~5–10 ms per RPC call and subprocess startup latency.[^12^] Only a running pipeline can measure this overhead.

### 10.1.2 Capability tiers versus single optional interface for filesystem operations

Insight 3 identified a three-tier filesystem export taxonomy — "fast" (seconds), "slow" (minutes-hours), and "impossible" — and proposed capability tiers (`FastExporter`, `SlowExporter`, `NoExporter`) over a single optional `ExportFilesystem` method.[^377^] If the first three adapters fall into different tiers, the tier system proves its value; otherwise the added API surface is unjustified.

## 10.2 Provider Selection

### 10.2.1 E2B: invest in Go REST client or deprioritize due to no official SDK?

E2B offers the strongest isolation (Firecracker microVMs, ~150 ms cold starts) but has no official Go SDK.[^86^][^570^] The choice is between writing a Go REST client for E2B or deprioritizing E2B and using Daytona (official Go SDK).[^159^] A spike is needed: can an AI agent generate a compilable E2B Go client from REST docs matching oapi-codegen quality?

### 10.2.2 AWS versus Hetzner as first VM adapter

Hetzner's `hcloud-go` scores 9.5/10 for idiomatic Go and costs $4.51/month.[^648^] AWS's `aws-sdk-go-v2` spans 300+ modules with a smithy middleware pipeline that creates an "enterprise penalty": mature SDKs are hardest for AI agents to wrap.[^1^] Generate both and measure which passes the certifier first.

## 10.3 Code Generation

### 10.3.1 Custom benchmark needed

No published benchmark measures "AI generating Go interface implementations from SDK + OpenAPI spec." SWE-bench Multilingual reports a 30.95% resolve rate for Go tasks,[^491^] but these are general bug-fix tasks, not adapter-wrapping work. Mesh needs a custom benchmark: compilation rate, `go vet`, interface satisfaction, and integration test passage.

### 10.3.2 oneOf/anyOf handling strategy across polymorphic responses

oapi-codegen generates `interface{}` for `oneOf`/`anyOf` schemas,[^20^][^21^] and Go lacks sum types, so every provider API with polymorphic responses requires manual type assertions. The open question is whether to (a) embed a standard `SafeUnmarshal` helper in every adapter, or (b) preprocess OpenAPI specs to flatten polymorphic fields. The latter is more reliable but requires a spec-transformation pipeline.

## 10.4 Security and Trust

### 10.4.1 Human review requirement: all adapters or only first N?

Research documents a 45% security failure rate for AI-generated code.[^551^] Reviewing every adapter is safe but defeats the purpose of automation. The provisional rule — review the first three adapters, then sample 10% randomly — needs validation against real defect rates. If failures cluster by SDK pattern, the skill spec needs targeted constraints rather than blanket review.

### 10.4.2 Sandboxed CI validation provider choice: Daytona versus E2B versus Modal

The validation pipeline must execute generated adapter code against live APIs in isolation. Daytona offers container-based sandboxes with ~90 ms cold starts and a $200 free tier.[^571^] E2B provides Firecracker microVMs with hardware-level isolation but no Go SDK.[^570^] Modal offers a beta Go SDK and gVisor isolation but no self-hosting path.[^136^] A side-by-side trial with the same generated adapter through both sandboxes is the only valid next step.

## 10.5 Ecosystem

### 10.5.1 Monorepo versus separate repos per adapter

Caddy moved to separate repositories per plugin for independent versioning.[^648^] Monorepos simplify cross-cutting changes but force lockstep releases. The recommended path is separate repositories with a `go.work` file for local development, re-evaluated after five adapters exist.

### 10.5.2 Registry file format: JSON versus YAML versus Go package metadata

A `registry.json` file listing adapters requires no external infrastructure,[^612^] but duplicates information in Go module paths and tags. YAML is more human-editable but adds a parser dependency; Go package metadata is the most native but lacks provider-specific annotations. The decision is deferred until the discovery UX is prototyped.