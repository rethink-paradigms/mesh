## 4. Plugin Architecture Recommendation

### 4.1 Pattern Analysis

#### 4.1.1 HashiCorp go-plugin: battle-tested but heavy; justified at 3000+ providers, not 5-15

HashiCorp go-plugin launches each plugin as a subprocess and communicates over gRPC (all HashiCorp products now default to gRPC; net/rpc remains only for backward compatibility) [^12^][^337^]. It provides panic isolation and cross-language support. For Mesh, neither benefit is currently relevant: all adapters are Go code wrapping Go SDKs. Terraform's 3,000+ providers justify the overhead; 5-15 HTTP wrappers do not. Eli Bendersky's analysis identifies the RPC client/server wrappers as "the most tedious and time consuming step" of setup [^12^][^382^]. The `HandshakeConfig` also provides only coarse version checking — a single integer that, when changed, breaks all existing plugins [^328^].

**Steal this**: Panic isolation as an escalation path for untrusted adapters. **Avoid this**: go-plugin as the default for a small, trusted adapter set.

#### 4.1.2 Terraform/Crossplane: K8s-coupled, overengineered for non-orchestrator use

Terraform providers are standalone gRPC servers implementing the tfprotov5/tfprotov6 protocol, with full schema definitions, plan diffing, state versioning, and lifecycle management [^335^]. Mesh needs approximately eight methods; the Terraform protocol is two orders of magnitude more complex than required.

Crossplane's `ExternalClient` interface is minimal — four methods: Observe, Create, Update, Delete [^389^]. Yet it is embedded in a framework assuming Kubernetes as the control plane: CRDs, controller-runtime, ProviderConfig resources, finalizers, and OCI packaging [^418^]. Extracting the four-method pattern is useful; adopting the framework is overkill for a standalone runtime.

#### 4.1.3 database/sql: gold standard — tiny core interface + init() registry + extension interfaces

The `database/sql` package is the most successful plugin architecture in Go. First, the core interface is vanishingly small: a driver implements only `Driver` with a single `Open` method [^341^]. Second, drivers self-register via `init()` side effects, calling `sql.Register("drivername", &Driver{})` at initialization time [^341^]. Third, optional capabilities are expressed through extension interfaces detected at runtime via type assertion, with automatic fallback when absent [^377^].

The Go CDK applies the same portable-type pattern to cloud abstractions [^713^], and the Go team formalized extension interfaces in the `io/fs` proposal [^677^]. For Mesh, this pattern requires approximately 50 lines of registry code versus thousands for go-plugin [^341^]. The SubstrateAdapter core interface defines required verbs; extension interfaces (`FilesystemExporter`, `FilesystemImporter`) advertise optional capabilities. Adapters register in `init()`, and the runtime selects by name. Zero overhead, zero serialization, full type safety.

#### 4.1.4 Go plugin package: CGO trap, universally rejected

Go's standard library `plugin` package loads `.so` files at runtime. The Go documentation warns: supported only on Linux, FreeBSD, and macOS; requires CGO; crashes unless all binaries use exactly the same Go toolchain version, build tags, and flags; no unloading support [^371^]. The Go team recommends IPC mechanisms over this package [^371^]. Go 1.24 contained no improvements to it [^458^]. No major Go project relies on it in production.

#### 4.1.5 Wasm Component Model: promising but wazero rejected; not viable for Go in 2026

The WebAssembly Component Model (WASI Preview 2) introduces typed interfaces via WIT and component composition [^333^][^420^]. For Go, two blockers remain. wazero — the production-ready pure-Go Wasm runtime — explicitly rejected Component Model support [^387^]. The only workaround is translating components to Core Wasm first [^387^]. Wasmtime supports it natively but requires CGO [^384^]. WASI is in a transitional period with breaking changes between previews [^333^]. For 5-15 adapters, the overhead dwarfs any benefit.

### 4.2 Recommended Architecture

#### 4.2.1 Phase 1: Simple interface + runtime registry + optional extension interfaces

Apply the `database/sql` pattern directly: minimal core interface, global registry populated by `init()`, and extension interfaces for optional capabilities. The user-facing API should be a concrete struct that hides type assertions from callers, following the Go CDK model [^713^]. Every generated adapter must include a compile-time interface check (`var _ SubstrateAdapter = (*ProviderAdapter)(nil)`) to prevent signature drift [^639^]. Capability detection follows the `io/fs` extension interface pattern, with graceful fallback for unimplemented capabilities [^677^].

**Confidence**: High. Validated by `database/sql`, the Go CDK, and the Go team's design documents.

#### 4.2.2 Phase 2: Escalate to HashiCorp go-plugin gRPC only for untrusted/AI-generated adapters

Go's implicit interface satisfaction means the `SubstrateAdapter` contract is stable across deployment models. An adapter can begin in-process and later be replaced with a gRPC proxy without changing caller code. Adapters start in-process (simple, debuggable, zero overhead), and escalate to go-plugin gRPC only after passing security review, the certifier suite, and load testing. Escalation is per-adapter, not all-or-nothing.

**Confidence**: Medium. The interface stability claim is mechanically true in Go, but the gRPC proxy implementation has not been prototyped for Mesh's specific interface.

#### 4.2.3 Build tags for conditional compilation of specific adapters

Go build tags enable including or excluding adapter packages at compile time, useful for excluding experimental adapters or producing minimal binaries that omit heavy provider SDKs [^392^]. Build tags should not be the primary selection mechanism — runtime `init()` registration (like `database/sql`) is more idiomatic. Build tags are a compile-time filter layered on top of the runtime registry [^703^].

### 4.3 What NOT To Do

#### 4.3.1 Avoid gRPC for all adapters at small scale

The complexity cost of gRPC — protobuf, code generation, subprocess management, version negotiation, and error mapping — is not amortized over 5-15 adapters. When all adapters are Go code deployed together, gRPC adds overhead without adding value. Reserve gRPC for Phase 2 escalation when specific adapters require isolation.

#### 4.3.2 Avoid Go plugin package entirely

Do not use the `plugin` package under any circumstance. It requires CGO, fails on Windows, demands exact Go version matching, offers no unloading, and is explicitly discouraged by the Go team [^371^].

#### 4.3.3 Avoid K8s-coupled patterns (Crossplane) for standalone runtime

Crossplane's framework assumes Kubernetes as the control plane: CRDs, controller-runtime, ProviderConfig resources, finalizers, and OCI packaging [^418^]. Do not import Kubernetes client libraries into a standalone runtime designed for 2GB VMs. If the four-method CRUD pattern is inspiring, extract only the interface shape; reject the framework entirely.

---

**Table 4.1**: Plugin architecture pattern comparison for 5-15 Go adapters

| Pattern | Setup Complexity | Runtime Overhead | Crash Isolation | Cross-Language | Scale Fit | Mesh Recommendation |
|---|---|---|---|---|---|---|
| database/sql + registry | Minimal (~50 LOC) [^341^] | Zero | No | No | 1-50 | **Adopt immediately** |
| HashiCorp go-plugin (gRPC) | High (RPC stubs, protobuf) [^12^][^382^] | Medium (subprocess + serialization) | Yes | Yes | 50+ | Escalate only for untrusted adapters |
| Terraform provider protocol | Very high (schema, plan, diff, state) [^335^] | High (gRPC + complex protocol) | Yes | Yes (99% Go) | 1000+ | Reject — 2 orders of magnitude too complex |
| Crossplane provider | Very high (K8s + CRDs + controllers) [^418^] | High (reconciler overhead) | Via K8s | No | 100+ | Reject — framework is overkill |
| Go `plugin` package (CGO) | Medium (.so compilation) [^371^] | Low | No | No | 1-20 | **Reject entirely** — platform trap, no unloading |
| Wasm Component Model | Very high (WIT, wit-bindgen, translation) [^387^] | Medium (boundary copying) | Yes | Yes | 100+ | Reject for Go — wazero rejected Component Model |

The comparison reveals a bimodal distribution. In-process patterns cluster at low complexity, but only the database/sql pattern avoids platform traps. Out-of-process patterns provide crash isolation at complexity justified only by large scale. For 5-15 Go adapters, the database/sql pattern is the sole option that delivers zero overhead, full type safety, and trivial debuggability without architectural baggage.
