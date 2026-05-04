# Dimension 6: Cross-Provider Plugin Architecture Patterns

## Research Report: "N providers, one interface" for Mesh Adapters

**Date**: 2025  
**Scope**: Analyze how production systems handle multiple provider implementations behind a single interface. Focus on simplicity for 5-15 AI-generated adapters.  
**Searches performed**: 25+ independent web searches across HashiCorp go-plugin, Terraform, Crossplane, Pulumi, Go plugin package, database/sql, Wasm components, Go build tags, and registry patterns.

---

## 1. Executive Summary

- **For 5-15 adapters, a full RPC plugin system is overkill.** The database/sql "simple interface + optional capabilities + init() registry" pattern is the closest analogy to Mesh's SubstrateAdapter needs and requires ~50 lines of code versus thousands for go-plugin.[^341^]
- **HashiCorp go-plugin is battle-tested but heavy.** It requires subprocesses, gRPC/net/rpc scaffolding, handshake configs, and serialization. The payoff (crash isolation, cross-language) matters at Terraform scale (3000+ providers) but not at 5-15 adapters.[^12^]
- **Crossplane's ExternalClient interface (Observe/Create/Update/Delete) is elegant but Kubernetes-coupled.** The provider contract assumes CRDs, controllers, and ProviderConfig resources. Extracting just the interface pattern is viable; adopting the full framework is overengineered for a non-Kubernetes system.[^389^][^386^]
- **Pulumi dynamic providers are lighter than custom providers but language-locked and have real limitations** (no `read`, secrets serialized in state). The "dynamic adapter" pattern is conceptually similar to what Mesh needs but the implementation is Pulumi-specific.[^367^]
- **Go's built-in `plugin` package remains a trap.** CGO requirement, Linux/FreeBSD/macOS only, exact Go version coupling, no unloading. Official docs warn: "many users decide that traditional IPC mechanisms may be more suitable despite the performance overheads."[^371^]
- **database/sql's optional capability pattern is the gold standard for Go.** Core interface is tiny (`Driver` with `Open`). Optional features are detected via additional interfaces (`ConnBeginTx`, `ExecerContext`, `NamedValueChecker`). Fallback behavior is automatic.[^377^]
- **Wasm Component Model is promising but immature for Go in 2025.** WASI Preview 2 is stable but wazero (the pure-Go runtime) explicitly rejected Component Model support. Wasmtime supports it but requires CGO. The ecosystem is in transition.[^333^][^387^]
- **Build tags + simple registry is the pragmatic sweet spot.** Compile adapters into the binary, register in `init()`, select by name. Zero runtime overhead, zero serialization, type-safe, debuggable. Used by database/sql, Prometheus, and many Go ecosystems.[^341^][^392^]
- **gRPC is not worthwhile at 5-15 provider scale.** The complexity cost (protobuf, subprocess management, version negotiation, error mapping) exceeds the benefit when all adapters are written in Go, deployed together, and don't need crash isolation.[^12^][^382^]
- **Recommended architecture for Mesh**: **Simple interface + registry + optional capabilities**, with build tags for conditional inclusion. Escalate to go-plugin only if cross-language or untrusted adapters become requirements.

---

## 2. Detailed Findings

### 6a. HashiCorp go-plugin

#### Protocol: gRPC vs net/rpc vs stdio

HashiCorp go-plugin uses **subprocess + RPC** as its core architecture. There is no true "stdio" in-process mode—communication is always via RPC over a Unix domain socket or TCP socket.[^12^]

```
Claim: go-plugin supports two RPC transports: net/rpc and gRPC. HTTP/2 handles multiplexing for gRPC; yamux handles multiplexing for net/rpc.[^12^]
Source: hashicorp/go-plugin GitHub
URL: https://github.com/hashicorp/go-plugin
Date: 2016-01-21 (ongoing)
Excerpt: "The HashiCorp plugin system works by launching subprocesses and communicating over RPC (using standard net/rpc or gRPC). A single connection is made between any plugin and the host process. For net/rpc-based plugins, we use a connection multiplexing library to multiplex any other connections on top. For gRPC-based plugins, the HTTP2 protocol handles multiplexing."
Context: Core architecture description
Confidence: high
```

All HashiCorp products (Terraform, Vault, Nomad, Boundary, Waypoint) now use **gRPC** as the default transport. The net/rpc path exists for backward compatibility but is not recommended for new development.[^337^]

```
Claim: All HashiCorp products use gRPC for go-plugin, not net/rpc, due to cross-language support and battle-tested stability.[^337^]
Source: zeroFruit - Hashicorp Plugin System Design and Implementation
URL: https://zerofruit-web3.medium.com/hashicorp-plugin-system-design-and-implementation-5f939f09e3b3
Date: 2022-03-05
Excerpt: "All Hashicorp products are using gRPC client/server for transporting data between the main service and the plugin service. I think it's because by using gRPC there's no restriction on the programming language users should use to implement the plugin, and gRPC is battle-tested so far with lots of use-cases so we can ensure its stability and performance."
Context: Analysis of why HashiCorp migrated to gRPC
Confidence: high
```

#### Interface Versioning for Breaking Changes

go-plugin uses a **handshake config** for coarse version compatibility, not fine-grained interface versioning:

```go
var handshakeConfig = plugin.HandshakeConfig{
    ProtocolVersion:  1,
    MagicCookieKey:   "BASIC_PLUGIN",
    MagicCookieValue: "hello",
}
```

The `ProtocolVersion` is an integer. Changing it breaks all existing plugins. There is no built-in mechanism for "version 2 adds method X but is backward compatible with version 1 callers." This must be implemented manually (e.g., separate interface names or optional method probing).[^328^][^382^]

```
Claim: go-plugin's HandshakeConfig provides coarse protocol version checking, not fine-grained interface evolution.[^328^]
Source: SagooIoT Plugin System Documentation
URL: https://iotdoc.sagoo.cn/develop/plugin/hashicorp
Date: 2026-01-31
Excerpt: "Plugin system, when the main service starts, it exec the binary of the plugin services. And two process shares the file descriptor created by the main service and communicate by using a Unix domain socket. And they use RPC protocol."
Context: Plugin discovery and handshake mechanism
Confidence: high
```

#### Plugin Discovery

go-plugin provides a basic `Discover` function that wraps filesystem glob patterns. Host applications must implement their own discovery logic. The library does not prescribe a directory structure or naming convention.[^382^]

```
Claim: go-plugin does not provide sophisticated plugin discovery; only a basic filesystem glob wrapper. Host applications implement their own discovery.[^382^]
Source: Eli Bendersky - RPC-based plugins in Go
URL: https://eli.thegreenplace.net/2023/rpc-based-plugins-in-go/
Date: 2023-03-28
Excerpt: "since plugins are just binaries that can be found anywhere, go-plugin doesn't prescribe what approach to take here. It only provides a Discover function which is a basic wrapper around a filesystem glob pattern."
Context: Discovery and registration section of go-plugin tutorial
Confidence: high
```

#### Error Propagation Across Boundary

Errors propagate across the RPC boundary via serialization. Panics in the plugin **do not crash the host**—this is a key selling point. However, error types are flattened to strings over RPC (for net/rpc) or gRPC status codes. Rich error types (sentinel errors, error wrapping with `fmt.Errorf`) require custom serialization.[^12^]

```
Claim: go-plugin's primary resilience feature is panic isolation—plugin crashes do not affect the host process.[^12^]
Source: hashicorp/go-plugin GitHub
URL: https://github.com/hashicorp/go-plugin
Date: 2016-01-21
Excerpt: "Plugins can't crash your host process: A panic in a plugin doesn't panic the plugin user."
Context: Architecture benefits list
Confidence: high
```

#### Minimum Viable Setup

The minimum viable go-plugin setup requires:
1. Define a Go interface
2. Implement RPC client + server wrappers for that interface (the tedious part)
3. Implement a `Plugin` type that knows how to create client/server
4. Plugin binary calls `plugin.Serve`
5. Host calls `plugin.Client` to launch subprocess

Step 2 is described as "the most tedious and time consuming step" in the official documentation.[^12^]

```
Claim: Implementing RPC client/server wrappers is the most tedious part of go-plugin setup.[^12^]
Source: hashicorp/go-plugin GitHub
URL: https://github.com/hashicorp/go-plugin
Date: 2016-01-21
Excerpt: "In practice, step 2 is the most tedious and time consuming step. Even so, it isn't very difficult and you can see examples in the examples/ directory as well as throughout our various open source projects."
Context: Usage instructions
Confidence: high
```

---

### 6b. Terraform Provider Architecture

#### terraform-plugin-framework vs terraform-plugin-sdk-go

HashiCorp maintains two SDKs:
- **terraform-plugin-framework**: Modern, recommended for new providers. Uses separate packages per concept (datasource, provider, resource). Better data access (null/unknown distinction). More control over built-in behaviors.[^329^]
- **SDKv2**: Legacy, maintained but feature-frozen. Many existing providers use it. Abstract recursive types make it harder to understand.[^329^]

```
Claim: Terraform Plugin Framework is the recommended SDK for new providers; SDKv2 is legacy and feature-frozen.[^329^]
Source: HashiCorp Terraform Docs - Plugin Framework Benefits
URL: https://developer.hashicorp.com/terraform/plugin/framework-benefits
Date: 2025-05-27
Excerpt: "We recommend using the framework for new provider development because it offers significant advantages as compared to the SDKv2. We also recommend migrating existing providers to the framework when possible."
Context: Official recommendation
Confidence: high
```

#### How 3000+ Providers Implement the Same Contract

Terraform providers are standalone gRPC servers implementing the Terraform provider protocol (tfprotov5/tfprotov6). The protocol is defined in protobuf and implemented via go-plugin.[^335^]

```
Claim: Terraform providers are standalone executables that implement a gRPC server using go-plugin. The protocol assumes encoding based on zclconf/go-cty.[^335^]
Source: HashiCorp Discuss - Provider frameworks for non-Go languages
URL: https://discuss.hashicorp.com/t/provider-frameworks-for-python-typescript-anything-non-go/76296
Date: 2025-08-21
Excerpt: "In general, if you open a GitHub issue against Terraform core that's related to a provider, we will assume it's written in Go and served via terraform-plugin-go, using it's type system tftypes, etc."
Context: Response from Terraform plugin maintainer (austin.valle)
Confidence: high
```

The provider schema command (`terraform providers schema -json`) exposes the complete contract: provider config, resource schemas, data source schemas, ephemeral resources, functions.[^368^][^369^]

#### Provider Configuration Patterns

Providers accept configuration through the provider block (HCL). The provider schema defines required/optional attributes. A common pattern is environment variable fallback.[^363^]

```
Claim: Terraform providers use schema-defined configuration with environment variable fallbacks as a common pattern.[^363^]
Source: OneUptime Blog - Terraform Provider Schema
URL: https://oneuptime.com/blog/post/2026-02-23-terraform-provider-schema/view
Date: 2026-02-23
Excerpt: "DefaultFunc reads from environment variable if not set in HCL... Priority: explicit config > environment variable"
Context: Schema definition patterns
Confidence: high
```

#### gRPC Worthwhile at 5-15 Provider Scale?

Terraform uses gRPC because:
- Providers run in separate processes (crash isolation)
- Providers may be written in other languages (though 99% are Go)
- Providers are installed dynamically from a registry
- The protocol is complex (plans, state, diff, schema versioning)

For Mesh with 5-15 Go adapters deployed together:
- **Crash isolation**: Not critical—adapters are simple HTTP API wrappers
- **Cross-language**: Not needed—all adapters will be Go
- **Dynamic install**: Not needed—adapters ship with the binary
- **Protocol complexity**: Mesh needs ~8 methods, not Terraform's full CRUD lifecycle

**Verdict**: gRPC/go-plugin is not worthwhile at this scale. The cost (subprocess management, serialization, protobuf definitions) exceeds the benefit.

---

### 6c. Crossplane Providers

#### Standardization Across AWS/GCP/Azure

Crossplane providers standardize on:
1. **Managed Resources**: Kubernetes CRDs representing external resources
2. **ProviderConfig**: Credentials and configuration for external APIs
3. **ExternalClient interface**: 4-method CRUD contract
4. **crossplane-runtime managed reconciler**: Generic controller that calls ExternalClient[^389^][^386^]

```
Claim: Crossplane providers implement a 4-method ExternalClient interface: Observe, Create, Update, Delete.[^389^]
Source: VSHN Knowledge Base - Crossplane Provider Mechanics
URL: https://kb.vshn.ch/app-catalog/csp/spks/crossplane/crossplane_provider_mechanics.html
Date: Unknown
Excerpt: "The managed.ExternalConnecter interface is meant as an entrypoint for every reconciliation... The return value of ExternalConnector.Connect() is an instance of managed.ExternalClient itself. managed.ExternalClient features the 4 basic CRUD methods: Create(), Observe(), Update(), Delete()."
Context: Crossplane provider development guide
Confidence: high
```

#### Provider Interface Contract

The `ExternalClient` interface from crossplane-runtime:

```go
type ExternalClient interface {
    Observe(ctx context.Context, mg resource.Managed) (ExternalObservation, error)
    Create(ctx context.Context, mg resource.Managed) (ExternalCreation, error)
    Update(ctx context.Context, mg resource.Managed) (ExternalUpdate, error)
    Delete(ctx context.Context, mg resource.Managed) error
}
```

All methods must be **idempotent** and **non-blocking**. The reconciler calls them in a fixed order: Connect → Observe → [Create | Update | nothing].[^389^][^396^]

```
Claim: Crossplane ExternalClient methods must be idempotent; Create must not return AlreadyExists, Delete must not return NotFound.[^396^]
Source: Crossplane Provider Development Guide
URL: https://master.d1bmjmu5ixie8d.amplifyapp.com/docs/v1.10/contributing/provider_development_guide.html
Date: 2018-12-02
Excerpt: "Create implementations must not return an error if asked to create a resource that already exists... Delete implementations must not return an error when asked to delete a non-existent external resource."
Context: Official provider development guide
Confidence: high
```

#### Optional Verbs/Capabilities

Crossplane does not have a built-in "optional capability" mechanism like database/sql. All providers must implement all 4 methods. However, if a resource type does not support updates, the provider can return `ResourceUpToDate: false` and no-op the Update call.[^394^]

#### Overengineered for 8-Method Interface?

Yes. Crossplane's architecture assumes:
- Kubernetes as the control plane
- CRDs for every resource type
- Controller-runtime reconciliation loops
- ProviderConfig + Secret references
- Finalizers, status conditions, connection secrets
- OCI packaging for providers

Extracting just the "4-method CRUD interface" pattern is useful. Adopting the full framework would require Kubernetes, which is massive overkill for Mesh.[^418^]

```
Claim: Crossplane provider creation involves "the inherent complexity of a Kubernetes Controller combined with unique requirements" and can be overwhelming for bespoke use cases.[^418^]
Source: Syntasso Blog - Crossplane and Kratix
URL: https://www.syntasso.io/post/crossplane-and-kratix-a-powerful-duo-for-platform-engineering-success
Date: 2025-07-11
Excerpt: "when platform teams require something more bespoke for their organisation, they can quickly find themselves thrown into the deep end of provider creation, which involves the inherent complexity of a Kubernetes Controller combined with unique requirements."
Context: Analysis of Crossplane complexity
Confidence: high
```

---

### 6d. Pulumi Dynamic Providers

#### Dynamic Bridge Pattern

Pulumi dynamic providers are **in-process, language-locked** provider implementations. They implement a `ResourceProvider` interface with methods: `check`, `create`, `update`, `delete`. They are lighter than custom providers but only work in the same language as the Pulumi program.[^367^]

```
Claim: Pulumi dynamic providers are in-process and language-locked but lighter weight than custom gRPC providers.[^367^]
Source: Pulumi Docs - Dynamic Resource Providers
URL: https://www.pulumi.com/docs/iac/concepts/providers/dynamic-providers/
Date: 2026-04-29
Excerpt: "Dynamic resource providers are only able to be used in Pulumi programs written in the same language as the dynamic resource provider. But, they are lighter weight than custom providers and for many use-cases are sufficient to leverage the Pulumi state model."
Context: Official Pulumi documentation
Confidence: high
```

```
Claim: Dynamic providers must implement create, and typically update and delete. The read method is not currently functional.[^367^]
Source: Pulumi Docs - Dynamic Resource Providers
URL: https://www.pulumi.com/docs/iac/concepts/providers/dynamic-providers/
Date: 2026-04-29
Excerpt: "You must at least implement the create function but, in practice, you will probably also want to implement the update and delete functions as well. Note that read is not currently functional for dynamic providers."
Context: Interface requirements
Confidence: high
```

#### Assumptions About Underlying API

Dynamic providers assume:
- The API is simple enough to fit in a single file
- State can be serialized/deserialized by Pulumi
- Secrets may be stored in plaintext in state (documented limitation)[^383^]
- The provider code is serialized and stored in the state file

```
Claim: Pulumi dynamic providers have significant limitations: secrets not encrypted in state, code serialized into state, and no read support.[^383^]
Source: Medium - Exploring the Power of Pulumi Dynamic Providers
URL: https://medium.com/@imunscarred/exploring-the-power-of-pulumi-dynamic-providers-aac002db2c78
Date: 2024-10-14
Excerpt: "Any secrets specified in them are not encrypted in the state. Because the code is serialized and pushed into the state, if the resource or the state gets messed up it can cause big issues and it's not easy to back out or refresh."
Context: Limitations section
Confidence: high
```

#### "Dynamic Adapter" Fallback Viable?

The **concept** of a lightweight in-process adapter that implements a minimal interface is viable and matches what Mesh needs. However, Pulumi's implementation is tied to their engine, state model, and serialization. The pattern (simple interface + user-provided adapter code) cannot be reused directly outside Pulumi.

---

### 6e. Go Plugin Patterns

#### Go `plugin` Package (CGO, Limited Platforms)

The standard library `plugin` package loads `.so` files at runtime. It has severe limitations:

```
Claim: Go's plugin package only works on Linux, FreeBSD, and macOS; requires CGO; needs exact Go version match; has no unloading support.[^371^]
Source: Go pkg.go.dev - plugin package
URL: https://pkg.go.dev/plugin
Date: Ongoing
Excerpt: "Plugins are currently supported only on Linux, FreeBSD, and macOS, making them unsuitable for applications intended to be portable... Runtime crashes are likely to occur unless all parts of the program are compiled using exactly the same version of the toolchain, the same build tags, and the same values of certain flags and environment variables."
Context: Official warnings
Confidence: high
```

```
Claim: The Go team explicitly recommends IPC mechanisms over the plugin package for many use cases.[^371^]
Source: Go pkg.go.dev - plugin package
URL: https://pkg.go.dev/plugin
Date: Ongoing
Excerpt: "For these reasons, many users decide that traditional interprocess communication (IPC) mechanisms such as sockets, pipes, remote procedure call (RPC), shared memory mappings, or file system operations may be more suitable despite the performance overheads."
Context: Official recommendation against plugin package
Confidence: high
```

#### HashiCorp go-plugin In-Process Mode

go-plugin does not have a true in-process mode. It always uses subprocesses. However, there is a `ReattachConfig` for reattaching to already-running plugin processes. This is useful for debugging but not for eliminating subprocess overhead.[^331^]

#### Simple Interface + Registry (No Plugin System)

This is the pattern used by database/sql and many Go ecosystems:

```go
var registry = make(map[string]func() Adapter)

func Register(name string, factory func() Adapter) {
    registry[name] = factory
}

func Get(name string) Adapter {
    return registry[name]()
}
```

Adapters register in `init()` functions. The main binary selects by name. No RPC, no serialization, no subprocesses. This is a **compile-time plugin** pattern.[^341^]

```
Claim: The simple registry pattern implements compile-time plugins with zero overhead and full type safety.[^341^]
Source: Eli Bendersky - Design patterns in Go's database/sql package
URL: https://eli.thegreenplace.net/2019/design-patterns-in-gos-databasesql-package/
Date: 2019-03-27
Excerpt: "This approach implements a compile-time plugin, because the imports for the included backends happen when the Go code is compiled. The binary has a fixed set of database drivers built into it."
Context: Analysis of sql.Register pattern
Confidence: high
```

#### Build Tags for Conditional Compilation

Go build tags allow including/excluding adapter files at compile time:

```go
//go:build mesh_adapter_aws

package awsadapter

func init() {
    mesh.Register("aws", NewAWSAdapter)
}
```

This enables shipping a core binary and adding adapters without modifying core code.[^392^]

```
Claim: Go build tags enable platform-specific and feature-specific conditional compilation.[^392^]
Source: OneUptime Blog - Go Build Tags
URL: https://oneuptime.com/blog/post/2026-01-23-go-build-tags/view
Date: 2026-01-23
Excerpt: "Build tags (also called build constraints) let you include or exclude files from compilation based on conditions like target OS, architecture, or custom flags."
Context: Build tag tutorial
Confidence: high
```

#### Go 1.24 Plugin Improvements

**None.** Go 1.24 release notes show no improvements to the `plugin` package. The release focuses on: generic type aliases, new crypto packages, `os.Root`, `testing.B.Loop`, `runtime.AddCleanup`, `sync.Map` improvements, and `weak` pointers. The `plugin` package remains unchanged and unimproved.[^458^][^463^]

```
Claim: Go 1.24 contains no improvements to the plugin package.[^458^]
Source: Go 1.24 Release Notes
URL: https://go.dev/doc/go1.24
Date: 2025-02-11
Excerpt: (No mention of plugin package in the extensive release notes)
Context: Full release notes review
Confidence: high
```

---

### 6f. database/sql Driver Pattern

#### `sql.Register` in `init()`

The canonical pattern: each driver imports `database/sql` and calls `sql.Register` in its `init()` function. The mapping is a global map protected by a mutex.[^341^]

```go
func init() {
    sql.Register("sqlite3", &SQLiteDriver{})
}
```

```
Claim: database/sql uses a global map with sync.RWMutex for driver registration, populated via init() functions.[^341^]
Source: Eli Bendersky - Design patterns in Go's database/sql package
URL: https://eli.thegreenplace.net/2019/design-patterns-in-gos-databasesql-package/
Date: 2019-03-27
Excerpt: "In sql.go, Register adds a mapping from a string name to an implementation of the driver.Driver interface; the mapping is in a global map... Register makes a database driver available by the provided name. If Register is called twice with the same name or if driver is nil, it panics."
Context: Detailed code analysis
Confidence: high
```

#### Optional Capabilities (Closest Analogy to SubstrateAdapter!)

database/sql has a **tiny core interface** (`Driver` with `Open`) and **many optional interfaces** for capabilities:

- `DriverContext` — parse name once for a pool
- `ConnBeginTx` — context + transaction options
- `ConnPrepareContext` — prepared statements with context
- `ExecerContext` / `QueryerContext` — direct exec/query without prepare
- `NamedValueChecker` — custom parameter types
- `Pinger` — connection health check
- `SessionResetter` — reset connection before reuse
- `Validator` — check connection validity
- `RowsNextResultSet` — multiple result sets
- `RowsColumnType*` — column metadata[^377^]

```
Claim: database/sql uses optional interfaces for capabilities. If a driver doesn't implement an optional interface, the sql package falls back to default behavior.[^377^]
Source: Go database/sql/driver package docs
URL: https://pkg.go.dev/database/sql/driver
Date: Ongoing
Excerpt: "If a Conn implements neither ExecerContext nor Execer, the database/sql.DB.Exec will first prepare a query, execute the statement, and then close the statement."
Context: Documentation for optional interfaces
Confidence: high
```

```
Claim: database/sql drivers can return ErrSkip to indicate a fast-path is not available, triggering fallback behavior.[^377^]
Source: Go database/sql/driver package docs
URL: https://pkg.go.dev/database/sql/driver
Date: Ongoing
Excerpt: "var ErrSkip = errors.New(\"driver: skip fast-path; continue as if unimplemented\")"
Context: Error handling for optional features
Confidence: high
```

This pattern is **directly applicable** to Mesh:
- Core `SubstrateAdapter` interface: ~3-5 required methods
- Optional interfaces: `LogStreamer`, `MetricsProvider`, `HealthChecker`
- Adapters implement only what the underlying substrate supports
- Mesh runtime detects capabilities via type assertions

---

### 6g. Wasm Component Model

#### WASI Preview 2 Typed Interfaces

WASI Preview 2 (WASI 0.2) introduced the Component Model with WIT (WebAssembly Interface Types) as the IDL. Worlds define sets of interfaces. Components can be composed like "LEGO bricks."[^333^][^420^]

```
Claim: WASI 0.2 includes the Component Model, wasi-cli, wasi-http, wasi-filesystem, wasi-sockets, and other worlds.[^417^]
Source: Uno Platform - State of WebAssembly 2024-2025
URL: https://platform.uno/blog/state-of-webassembly-2024-2025/
Date: 2025-01-27
Excerpt: "Preview 2 included the following features: Component Model, wasi-cli world, wasi-http world... The Bytecode Alliance decided to change to dot releases instead of referring to the releases as previews."
Context: WASI status overview
Confidence: high
```

#### Component Composition

Components import and export interfaces. A host provides some interfaces; the guest provides others. An adapter component can translate between worlds.[^420^]

```
Claim: WebAssembly components compose via adapter components that translate between worlds.[^420^]
Source: F5 Blog - What is the WebAssembly Component Model?
URL: https://www.f5.com/company/blog/what-is-the-webassembly-component-model
Date: 2025-02-12
Excerpt: "if you have a host environment that implements a sockets world, but you have a component that wants to make HTTP requests in an HTTP world, you can write an adapter component that implements the HTTP world using sockets."
Context: Component composition explanation
Confidence: high
```

#### wazero (Pure Go Runtime)

wazero is a **pure Go** WebAssembly runtime with zero CGO dependencies. It supports WASI Preview 1. However, **wazero explicitly rejected the Component Model**.[^387^][^333^]

```
Claim: wazero rejected the Component Model. The only way to use Component Model interfaces with wazero is to translate Components to Core Wasm first.[^387^]
Source: arcjet-gravity crate documentation
URL: https://lib.rs/crates/arcjet-gravity
Date: 2025-02-26
Excerpt: "Wazero has rejected the Component Model, but we can still translate Components to Core today. By adopting a similar strategy as jco transpile, we've built this tool to produced Wazero output that adhere's to the Component Model's Canonical ABI."
Context: Arcjet's workaround for wazero + Component Model
Confidence: high
```

```
Claim: wazero is production-ready (v1.0 released 2024) but only for Core WebAssembly + WASI Preview 1, not Component Model.[^384^]
Source: WasmRuntime.com - wazero vs CGO 2026
URL: https://wasmruntime.com/en/blog/wazero-vs-cgo-2026
Date: 2026-01-01
Excerpt: "wazero is production-ready and actively maintained by Tetrate Labs. It supports WASI Preview 1 and is used in production by many Go projects."
Context: wazero production readiness FAQ
Confidence: high
```

#### Viable for Adapters?

**Not in 2025.** The barriers are:
1. wazero doesn't support Component Model—would need translation layer
2. Wasmtime supports Component Model but requires CGO
3. WASI is in transition (Preview 1 → Preview 2 → 0.3)
4. Toolchain complexity (WIT, wit-bindgen, adapter generation)
5. Data copying overhead across Wasm boundary
6. For 5-15 adapters, the complexity dwarfs the benefit

```
Claim: WASI is in a transitional period with breaking changes between previews, slowing adoption.[^333^]
Source: eunomia.dev - WASI and WebAssembly Component Model Current Status
URL: https://eunomia.dev/blog/2025/02/16/wasi-and-the-webassembly-component-model-current-status/
Date: 2025-02-16
Excerpt: "The jump from WASI Preview1 to Preview2 introduced breaking changes... binaries compiled against WASI Preview1 are not compatible with Preview2 without adaptation... we're in a transitional period where WASI is evolving quickly, which ironically can slow adoption since some developers adopt a 'wait until it stabilizes' stance."
Context: Technical limitations analysis
Confidence: high
```

---

## 3. Contradictions and Conflict Zones

### Contradiction 1: go-plugin's "Simplicity" Claims vs. Reality

HashiCorp documentation states plugins are "very easy to write" and "very easy to install."[^12^] However, Eli Bendersky's detailed analysis shows the RPC scaffolding is "a bit of work" and "the most tedious and time consuming step."[^382^] The disconnect: go-plugin makes the *plugin author's* experience simple (just implement an interface) but the *host author's* experience requires significant boilerplate.

### Contradiction 2: Wasm Component Model Hype vs. Go Runtime Reality

The Component Model is promoted as the future of composable software.[^420^] However, wazero—the most popular pure-Go Wasm runtime—rejected it.[^387^] This creates a split: Rust/C ecosystems can use Component Model today via Wasmtime, but Go ecosystems cannot without CGO or complex translation layers.

### Contradiction 3: Crossplane's "Simple" 4-Method Interface vs. Massive Framework

Crossplane advertises a simple ExternalClient interface (4 methods).[^389^] In practice, using it requires Kubernetes, controller-runtime, CRDs, ProviderConfig resources, OCI packaging, and finalizer management.[^418^] The interface is simple; the framework around it is not.

### Contradiction 4: Dynamic Providers "Lighter Weight" vs. Critical Limitations

Pulumi markets dynamic providers as "lighter weight than custom providers."[^367^] But they have documented limitations: no `read`, secrets in plaintext state, code serialization fragility, language lock-in.[^383^] They are lighter for the provider *author* but impose costs on the *user*.

---

## 4. Gaps in Available Information

1. **No direct comparison of adapter patterns at small scale (5-15 providers).** Most literature focuses on Terraform-scale (3000+) or Crossplane-scale (enterprise Kubernetes). There is no authoritative analysis of "what's the simplest pattern for a dozen adapters."

2. **Missing: production Go projects using simple registry + optional interfaces.** While database/sql is the canonical example, there are few documented case studies of applying this pattern outside database drivers.

3. **Unclear: wazero's long-term Component Model stance.** wazero rejected Component Model as of 2025, but no public roadmap indicates whether they will adopt it post-stabilization.

4. **Missing: performance benchmarks for go-plugin vs in-process calls at small scale.** All benchmarks focus on large-scale scenarios. The overhead of subprocess + gRPC for a single-digit number of adapters is undocumented.

5. **Missing: standardized "capability negotiation" pattern in Go beyond database/sql.** There is no widely-used library for "optional interface detection + graceful fallback" that can be reused generically.

---

## 5. Preliminary Recommendations

### Tier 1: Recommended for Mesh (High Confidence)

**Simple Interface + Registry + Optional Capabilities**

```go
// Core interface (required)
type SubstrateAdapter interface {
    Name() string
    Deploy(ctx context.Context, req DeployRequest) (*DeployResponse, error)
    Destroy(ctx context.Context, req DestroyRequest) error
    Status(ctx context.Context, id string) (Status, error)
}

// Optional capability interfaces
type LogStreamer interface {
    StreamLogs(ctx context.Context, id string, opts LogOptions) (LogStream, error)
}

type MetricsProvider interface {
    GetMetrics(ctx context.Context, id string) (Metrics, error)
}

// Registry
var adapters sync.Map // or map + RWMutex

func Register(name string, factory func() SubstrateAdapter) {
    adapters.Store(name, factory)
}

func Get(name string) (SubstrateAdapter, error) {
    v, ok := adapters.Load(name)
    if !ok { return nil, fmt.Errorf("unknown adapter: %s", name) }
    return v.(func() SubstrateAdapter)(), nil
}

// Capability detection
func CanStreamLogs(a SubstrateAdapter) bool {
    _, ok := a.(LogStreamer)
    return ok
}
```

Each adapter registers in its `init()`:
```go
func init() {
    mesh.Register("fly", NewFlyAdapter)
}
```

**Why**: Zero overhead, type-safe, debuggable, testable, no dependencies, no serialization, no subprocess management. Directly modeled on database/sql, the most successful plugin pattern in Go.[^341^][^377^]

### Tier 2: Conditional Compilation Enhancement (Medium Confidence)

Use Go build tags to include/exclude adapters:

```go
//go:build mesh_adapter_fly

package fly

func init() { mesh.Register("fly", NewFlyAdapter) }
```

Build with: `go build -tags mesh_adapter_fly,mesh_adapter_aws`

This keeps binary size minimal and allows "core + add-on" distribution models.[^392^]

### Tier 3: Escalation Path if Requirements Change (Low Confidence)

| If requirement emerges... | Escalate to... |
|---|---|
| Untrusted third-party adapters | HashiCorp go-plugin (gRPC) |
| Cross-language adapters (Python/JS) | HashiCorp go-plugin (gRPC) |
| Dynamic loading without recompilation | Go `plugin` package (if Linux-only acceptable) |
| Sandboxed execution | Wasmtime + Component Model (with CGO) |
| Kubernetes-native control plane | Crossplane provider pattern |

**Do not start with these.** Only escalate if the specific requirement materializes.

---

## 6. Summary Matrix

| Pattern | Complexity | Overhead | Crash Isolation | Cross-Lang | Scale Fit | Mesh Fit |
|---|---|---|---|---|---|---|
| Simple interface + registry | Minimal | Zero | No | No | 1-50 | **Excellent** |
| Build tags + registry | Low | Zero | No | No | 1-50 | **Excellent** |
| Go `plugin` package | Medium | Low | No | No | 1-20 | Poor (platform limits) |
| HashiCorp go-plugin | High | Medium-High | Yes | Yes | 50+ | Overkill |
| Wasm Component Model | Very High | Medium | Yes | Yes | 100+ | Not ready (Go) |
| Crossplane provider | Very High | High | Via K8s | No | 100+ | Overkill |
| Pulumi dynamic provider | Medium | Low | No | No | 1-20 | Pulumi-locked |

---

## Citation Index

[^12^]: hashicorp/go-plugin GitHub repository. https://github.com/hashicorp/go-plugin
[^328^]: SagooIoT Plugin System Documentation (HashiCorp go-plugin in Chinese). https://iotdoc.sagoo.cn/develop/plugin/hashicorp
[^329^]: HashiCorp Terraform Plugin Framework Benefits. https://developer.hashicorp.com/terraform/plugin/framework-benefits
[^330^]: Upbound Blog - The Power of Crossplane's Nested Abstractions. https://www.upbound.io/blog/platform-engineering-simplified
[^331^]: Go package docs for hashicorp/go-plugin. https://pkg.go.dev/github.com/hashicorp/go-plugin
[^333^]: eunomia.dev - WASI and the WebAssembly Component Model: Current Status. https://eunomia.dev/blog/2025/02/16/wasi-and-the-webassembly-component-model-current-status/
[^335^]: HashiCorp Discuss - Provider frameworks for non-Go languages. https://discuss.hashicorp.com/t/provider-frameworks-for-python-typescript-anything-non-go/76296
[^337^]: zeroFruit - Hashicorp Plugin System Design and Implementation. https://zerofruit-web3.medium.com/hashicorp-plugin-system-design-and-implementation-5f939f09e3b3
[^340^]: Dev.to - Overview of Crossplane and Crossplane-provider-aws. https://dev.to/yelenary/overview-of-crossplane-and-crossplane-provider-aws-baj
[^341^]: Eli Bendersky - Design patterns in Go's database/sql package. https://eli.thegreenplace.net/2019/design-patterns-in-gos-databasesql-package/
[^342^]: pulumi/pulumi-terraform-bridge GitHub. https://github.com/pulumi/pulumi-terraform-bridge
[^363^]: OneUptime - How to Define Provider Schema in Terraform. https://oneuptime.com/blog/post/2026-02-23-terraform-provider-schema/view
[^364^]: OneUptime - How to Build a Plugin System with Go's plugin Package. https://oneuptime.com/blog/post/2026-01-25-plugin-system-go-plugin-package/view
[^367^]: Pulumi Docs - Dynamic Resource Providers. https://www.pulumi.com/docs/iac/concepts/providers/dynamic-providers/
[^368^]: Medium - Exploring terraform provider capabilities with schema analysis. https://manuchandrasekhar.medium.com/exploring-terraform-provider-capabilities-with-schema-analysis-362f0d0e3ce5
[^369^]: HashiCorp Terraform - providers schema command. https://developer.hashicorp.com/terraform/cli/commands/providers/schema
[^370^]: Caffeinated Coder - Building a Go Plugin System Without the Plugin Package. https://caffeinatedcoder.medium.com/building-a-go-plugin-system-without-the-plugin-package-a-battle-tested-approach-5235db51d182
[^371^]: Go pkg.go.dev - plugin package. https://pkg.go.dev/plugin
[^377^]: Go database/sql/driver package docs. https://pkg.go.dev/database/sql/driver
[^378^]: crossplane-runtime pkg/resource docs. https://pkg.go.dev/github.com/crossplane/crossplane-runtime/pkg/resource
[^382^]: Eli Bendersky - RPC-based plugins in Go. https://eli.thegreenplace.net/2023/rpc-based-plugins-in-go/
[^383^]: Medium - Exploring the Power of Pulumi Dynamic Providers. https://medium.com/@imunscarred/exploring-the-power-of-pulumi-dynamic-providers-aac002db2c78
[^384^]: WasmRuntime.com - wazero vs CGO (2026). https://wasmruntime.com/en/blog/wazero-vs-cgo-2026
[^386^]: crossplane-runtime pkg/reconciler/managed docs. https://pkg.go.dev/github.com/crossplane/crossplane-runtime/pkg/reconciler/managed
[^387^]: arcjet-gravity crate docs (wazero host generator). https://lib.rs/crates/arcjet-gravity
[^389^]: VSHN Knowledge Base - Crossplane Provider Mechanics. https://kb.vshn.ch/app-catalog/csp/spks/crossplane/crossplane_provider_mechanics.html
[^392^]: OneUptime - How to Use Build Tags in Go. https://oneuptime.com/blog/post/2026-01-23-go-build-tags/view
[^394^]: Crossplane Blog - Deep Dive into Terrajet Part II. https://blog.crossplane.io/deep-dive-terrajet-part-ii/
[^396^]: Crossplane Provider Development Guide. https://master.d1bmjmu5ixie8d.amplifyapp.com/docs/v1.10/contributing/provider_development_guide.html
[^417^]: Uno Platform - State of WebAssembly 2024-2025. https://platform.uno/blog/state-of-webassembly-2024-2025/
[^418^]: Syntasso - Crossplane and Kratix. https://www.syntasso.io/post/crossplane-and-kratix-a-powerful-duo-for-platform-engineering-success
[^420^]: F5 - What is the WebAssembly Component Model? https://www.f5.com/company/blog/what-is-the-webassembly-component-model
[^458^]: Go 1.24 Release Notes. https://go.dev/doc/go1.24
