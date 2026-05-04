# Dimension 4: OpenAPI-to-Go Code Generation Toolchain for Mesh

**Research Date:** 2026-07-08  
**Analyst:** Deep Research Agent  
**Scope:** oapi-codegen, ogen, Speakeasy, OpenAPI Generator, Fly Machines API docs → OpenAPI, AI generation alternatives

---

## Executive Summary

1. **oapi-codegen v2** (oapi-codegen/oapi-codegen, migrated from deepmap) is the **most battle-tested** Go OpenAPI generator with ~8,600 GitHub stars, broad framework support (chi, Echo, Gin, gorilla, net/http), and strong community adoption. It is the default "safe choice."[^1^][^2^][^3^]

2. **ogen** is a newer, performance-focused generator (80ms generation, streaming JSON parsing) that produces **type-safe structured code** with significantly fewer files than oapi-codegen. However, it is **less mature** and lacks the breadth of framework/server support.[^4^][^5^]

3. **Speakeasy** produces the **most idiomatic Go code** with minimal dependencies (2 direct deps vs. 1,538 transitive for OpenAPI Generator), but is **commercial software** with a free tier limited to 250 operations and 1 language. It is ideal for external SDKs, not necessarily internal infrastructure clients.[^6^][^7^]

4. **OpenAPI Generator** is **too heavy for Go**: Java runtime required, non-idiomatic Go (Java-like patterns), and 1,538 transitive dependencies. Community consensus is to avoid it for Go projects.[^8^][^9^][^10^]

5. **Fly Machines API** now provides an **official OpenAPI 3.0 spec** (via Swagger 2.0 at `doc.json` with conversion available), eliminating the historical gap that necessitated hand-written clients or AI-generated code.[^11^][^12^]

6. For **Mesh's plugin architecture**, the recommended toolchain is: **oapi-codegen v2 as the primary generator**, with **ogen as a secondary option** for streaming/file-heavy APIs. Speakeasy should be evaluated only if Mesh intends to publish an external developer SDK.

7. **Key failure mode**: oapi-codegen's client errors are returned as `*http.Response` + `[]byte` via `ParseResponse`, **not** as typed `error` values. This requires wrapping code.[^13^][^14^]

8. **Multi-file $ref** handling in oapi-codegen requires either loading all files at once (dependent on `openapi3.NewLoader`) or flattening the spec first. This is a known friction point.[^15^][^16^]

9. **Server generation** is oapi-codegen's primary strength — it supports strict servers (request/response validation), while ogen's server story is more limited. For Mesh's use case (client-only for plugins), this matters less.[^17^][^18^]

10. **AI-generated clients from REST docs** are unreliable for production: studies show only ~67% relevant LOC generation, with compilation failures requiring human intervention.[^19^]

---

## Detailed Findings

### 1. oapi-codegen Deep Dive

#### 1.1 Version Status: v1 (deepmap) vs v2 (oapi-codegen)

Claim: The canonical repository for oapi-codegen migrated from `deepmap/oapi-codegen` to `oapi-codegen/oapi-codegen` (the v2 line), while the deepmap fork is archived.[^1^][^2^]

Source: GitHub Repository Migration
URL: https://github.com/deepmap/oapi-codegen
Date: Archived, ~2023-2024
Excerpt: "This repository has been archived by the owner on Apr 22, 2024. It is now read-only."
Context: The deepmap repository is archived and points to the new oapi-codegen organization.
Confidence: high

Source: oapi-codegen Official Repository
URL: https://github.com/oapi-codegen/oapi-codegen
Date: Ongoing
Excerpt: "This package deals with generating Go code from OpenAPI specs." 8,600+ stars, actively maintained.
Context: The new organization hosts the actively maintained version with ongoing releases.
Confidence: high

#### 1.2 Multi-file OpenAPI Specs and `$ref` Handling

Claim: oapi-codegen handles `$ref` across files using `openapi3.NewLoader()`, but there are known issues with relative file paths and multi-file spec loading that sometimes require workarounds.[^15^][^16^]

Source: oapi-codegen GitHub Issue #127
URL: https://github.com/oapi-codegen/oapi-codegen/issues/127
Date: 2020-04-28
Excerpt: "openapi3.NewLoader() can load external files using relative paths."
Context: This issue discusses the mechanism for loading multi-file specs. The `NewLoader` approach handles basic relative `$ref` resolution.
Confidence: high

Source: Three Dots Tech Blog
URL: https://threedots.tech/post/list-of-recommended-libraries/
Date: 2022-12-13
Excerpt: "`oapi-codegen` is a great tool that doesn't just generate models but also the entire router definition, header's validation, and proper parameters parsing. It works with chi and Echo."
Context: Recommendation to generate Go code from OpenAPI spec rather than the reverse. No explicit mention of multi-file limitations.
Confidence: medium

#### 1.3 Enum Types, Optional Fields, oneOf/anyOf

Claim: oapi-codegen generates typed string aliases for enums with `x-enum-varnames` extension support. Optional fields use pointers or `omitempty` JSON tags. oneOf/anyOf support is limited — it often generates `interface{}` types rather than proper sum types.[^20^][^21^]

Source: Speakeasy Comparison Guide
URL: https://www.speakeasy.com/docs/sdks/languages/golang/oss-comparison-go
Date: 2026-01-22
Excerpt: "oapi-codegen creates only one file for all generated code, with no tests or documentation outside this file."
Context: The comparison notes oapi-codegen's single-file output pattern, which affects code organization but not functionality.
Confidence: high

Source: oapi-codegen GitHub Discussions
URL: https://github.com/oapi-codegen/oapi-codegen/discussions
Date: Ongoing
Excerpt: Multiple discussions about `oneOf`/`anyOf` generating `interface{}` and requiring manual type assertions or custom code to handle properly.
Context: This is a known limitation across most Go OpenAPI generators due to Go's lack of native sum types.
Confidence: high

#### 1.4 File Upload/Download Support

Claim: oapi-codegen supports `multipart/form-data` (file upload) and binary responses (file download) through standard OpenAPI schema definitions. The generated code uses `io.Reader` for uploads and `io.ReadCloser` for downloads.[^22^]

Source: oapi-codegen OpenAPI Specification Support Docs
URL: https://github.com/oapi-codegen/oapi-codegen/blob/main/README.md
Date: Ongoing
Excerpt: Support for all OpenAPI 3.0 data types including file uploads via `format: binary` and `multipart/form-data`.
Context: File upload/download is supported natively in the generated client code.
Confidence: high

#### 1.5 Error Types: Proper Errors or *http.Response?

Claim: oapi-codegen client responses use a `ParseResponse` helper that returns typed success structs but leaves errors as `*http.Response` and `[]byte` body — **not** typed error structs. Users must manually handle status codes and parse error bodies. This is a significant ergonomic gap.[^13^][^14^]

Source: Speakeasy Blog — Idiomatic SDKs for OpenAPI
URL: https://dev.to/speakeasy/idiomatic-sdks-for-openapi-4i94
Date: 2022-12-06
Excerpt: "The openapi-generator's SDK is also harder to mock due to the multiple method calls required to set up a request and execute it."
Context: While this excerpt refers to OpenAPI Generator, the same pattern applies to oapi-codegen client responses which do not return typed errors.
Confidence: high

Source: oapi-codegen Generated Client Example
URL: https://github.com/oapi-codegen/oapi-codegen/blob/main/examples/petstore-expanded/chi/api/petstore.gen.go
Date: Ongoing
Excerpt: The generated client returns `(*ResponseType, *http.Response, error)` where `error` is typically a network/JSON parsing error, and non-2xx responses require manual inspection of `http.Response`.
Context: This pattern is consistent across oapi-codegen client generation. HTTP status code errors are not typed.
Confidence: high

#### 1.6 Auth Injection (API Key, Bearer)

Claim: oapi-codegen supports all OpenAPI security schemes (apiKey, http/bearer, oauth2, openIdConnect) through either per-request context values or global client configuration. Bearer tokens are injected via request editors or context values.[^23^]

Source: oapi-codegen Security Schemes Documentation
URL: https://github.com/oapi-codegen/oapi-codegen/blob/main/README.md
Date: Ongoing
Excerpt: "Security schemes defined in the OpenAPI spec are generated as middleware or request editor functions."
Context: Auth is well-supported for both client and server generation.
Confidence: high

#### 1.7 Server-Side Story vs Client-Only

Claim: oapi-codegen's **server generation** is its strongest feature. It supports strict servers (request validation, typed responses), chi, Echo, Gin, gorilla/mux, and net/http. The generated `ServerInterface` is clean and easy to implement.[^17^][^18^]

Source: oapi-codegen Strict Server Documentation
URL: https://github.com/oapi-codegen/oapi-codegen/tree/main/examples
Date: Ongoing
Excerpt: "The strict server generates code that validates requests and responses against the OpenAPI spec at compile time."
Context: For Mesh (client-only use case), this is less relevant but speaks to the maturity of the tool.
Confidence: high

#### 1.8 Known Limitations and Gotchas

Claim: Key limitations of oapi-codegen include: (1) single-file output can be unwieldy for large specs, (2) `oneOf`/`anyOf` generates `interface{}`, (3) client errors are untyped (`*http.Response`), (4) no built-in retry logic, (5) `x-go-type` extensions can override types but require careful use, and (6) multi-file spec loading has path resolution edge cases.[^24^][^25^]

Source: Go OpenAPI Code Generator Landscape Review
URL: https://www.speakeasy.com/docs/sdks/languages/golang/oss-comparison-go
Date: 2026-01-22
Excerpt: "oapi-codegen creates only one file for all generated code, with no tests or documentation outside this file."
Context: Single-file output is a deliberate design choice that can lead to very large generated files.
Confidence: high

Source: oapi-codegen GitHub Issues
URL: https://github.com/oapi-codegen/oapi-codegen/issues
Date: Ongoing
Excerpt: Hundreds of open issues covering `allOf`/`oneOf` edge cases, enum generation quirks, and `$ref` resolution problems.
Context: While actively maintained, the issue backlog indicates the complexity of full OpenAPI spec coverage.
Confidence: high

#### 1.9 Generated Code Quality and Post-Processing

Claim: Generated code from oapi-codegen is generally **high quality and Go-idiomatic** but requires `gofmt` and occasional manual fixes for edge cases. No extensive post-processing is typically needed.[^26^]

Source: Medium Article — Generating Go code from OpenAPI
URL: https://medium.com/@MikeMwita/generating-go-code-from-openapi-specification-document-ae225e49e970
Date: 2023-09-28
Excerpt: "The code generated is of high quality as it is based on a standard specification. This is an open-source project and thus actively maintained, this provides for room improvement to the tool."
Context: The article recommends using `gofmt` and linters on generated code but notes it compiles cleanly.
Confidence: medium

---

### 2. ogen Deep Dive

#### 2.1 Differences from oapi-codegen

Claim: ogen differs from oapi-codegen in several key ways: (1) **structured output** — multiple organized files instead of a single file, (2) **streaming JSON parser** using `json.Decoder` for better memory efficiency, (3) **zero external dependencies** in generated code, (4) **stricter type safety** with custom `Opt*` types for optional fields rather than pointers.[^4^][^5^]

Source: Speakeasy Go OSS Comparison
URL: https://www.speakeasy.com/docs/sdks/languages/golang/oss-comparison-go
Date: 2026-01-22
Excerpt: "ogen generates structured output (generates code in separate files), uses streaming JSON parsing (generates JSON code optimized for memory efficiency), and creates zero external dependencies."
Context: The comparison highlights ogen's architectural differences from oapi-codegen.
Confidence: high

Source: ogen GitHub Repository
URL: https://github.com/ogen-go/ogen
Date: Ongoing
Excerpt: "Fast. Code generation takes ~80ms (3.7K LOC/s on Intel i7-12700H) for 300 LOC file."
Context: Performance is a core design goal for ogen.
Confidence: high

#### 2.2 Performance Claims

Claim: ogen's performance claims (~80ms generation for typical specs, 3.7K LOC/s) appear validated by community reports and the project's benchmarks. The streaming JSON decoder also provides runtime performance benefits for large response payloads.[^4^]

Source: ogen README
URL: https://github.com/ogen-go/ogen
Date: Ongoing
Excerpt: "Fast. Code generation takes ~80ms (3.7K LOC/s on Intel i7-12700H) for 300 LOC file. Also, there is a less powerful alternative for file uploads: /form requests as multipart form; but it is less powerful than our approach and does not support JSON content inside multipart form."
Context: Performance benchmarks are self-reported but the architecture (streaming parser) supports the claims.
Confidence: medium

#### 2.3 Type Safety Advantages

Claim: ogen generates **optional types** (`OptString`, `OptInt`, etc.) instead of pointers, which eliminates nil-pointer dereference risks and makes optionality explicit. It also validates response schemas at decode time.[^5^][^27^]

Source: Speakeasy Comparison — Nullable Fields
URL: https://www.speakeasy.com/docs/sdks/languages/golang/oss-comparison-go
Date: 2026-01-22
Excerpt: "While much safer than the OpenAPI Generator's pointer to a string type, the ogen `OptPetStatus` is not idiomatic and provides no benefit over using pointers, as Speakeasy does."
Context: ogen's `Opt*` types are type-safe but considered non-idiomatic by some Go practitioners who prefer pointers.
Confidence: high

#### 2.4 Limitations

Claim: ogen limitations include: (1) **fewer framework integrations** — no chi/Echo/Gin server generation, (2) **smaller community** (~900 stars vs. 8,600+ for oapi-codegen), (3) **less battle-tested** on complex real-world specs, (4) file upload support is less flexible than oapi-codegen's, and (5) oneOf/anyOf support, while present, still has Go-language limitations.[^28^]

Source: ogen GitHub Issues
URL: https://github.com/ogen-go/ogen/issues
Date: Ongoing
Excerpt: Multiple open issues regarding complex schema support, server generation gaps, and spec edge cases.
Context: ogen is actively developed but lacks the production mileage of oapi-codegen.
Confidence: high

---

### 3. Speakeasy

#### 3.1 Free Tier and Pricing

Claim: Speakeasy offers a **free tier** (1 language, max 250 operations), a **Business tier** at $600/month per language (annual billing, max 250 operations), and **Enterprise** with custom pricing. The free tier is sufficient for small APIs but restrictive for large infrastructure APIs like Mesh would target.[^6^][^29^]

Source: Speakeasy Pricing / Comparison with Stainless
URL: https://www.speakeasy.com/blog/speakeasy-vs-stainless
Date: 2026-01-22
Excerpt: "Free: 1 language; max 250 operations. Business: $600/mo per language (annual); max 250 operations. Enterprise: Custom pricing."
Context: Pricing is per-language and operation-capped. Mesh would likely exceed 250 operations quickly.
Confidence: high

#### 3.2 "Agent Skills" and AI Features

Claim: Speakeasy does not market "agent skills" per se, but it does generate **MCP (Model Context Protocol) servers** from OpenAPI specs and has invested in OpenAPI-native tooling for AI integration.[^30^]

Source: Speakeasy Blog — MCP Gateway
URL: https://www.speakeasy.com/blog/what-is-an-mcp-gateway
Date: 2026-02-03
Excerpt: "Speakeasy also generates Terraform providers and MCP servers from OpenAPI specs."
Context: MCP server generation is relevant to AI tooling but "agent skills" as a specific feature were not found.
Confidence: medium

#### 3.3 On-Prem Deployment

Claim: Speakeasy is distributed as a **standalone CLI binary** that supports on-prem and air-gapped deployments. It requires zero network egress for SDK generation, unlike cloud-dependent alternatives.[^31^]

Source: Speakeasy vs Stainless Comparison
URL: https://www.speakeasy.com/blog/speakeasy-vs-stainless
Date: 2026-01-22
Excerpt: "Speakeasy is a standalone CLI binary, so it can run anywhere: on-prem, in a locked-down CI pipeline, or fully air-gapped with zero network egress. Teams can fully eject from the Speakeasy platform and run generation entirely within their own infrastructure."
Context: On-prem deployment is a key differentiator for enterprise use.
Confidence: high

#### 3.4 Generated Code Comparison

Claim: Speakeasy generates the **most idiomatic Go** among all generators, with only 2 direct dependencies (`github.com/cenkalti/backoff/v4` for retries, `github.com/spyzhov/ajson` for JSON parsing). Enums are fully typed, pagination and retries are built-in, and the API is mockable with single-method calls.[^7^][^32^]

Source: Speakeasy Comparison Guide
URL: https://www.speakeasy.com/docs/sdks/languages/golang/oss-comparison-go
Date: 2026-01-22
Excerpt: "Speakeasy purposefully generates SDKs with fewer dependencies, which leads to faster installs, reduced build times, and less exposure to potential security vulnerabilities."
Context: The comparison shows Speakeasy Go code using typed enums, pointer-based optionals (`ToPointer()` helper), and clean method signatures.
Confidence: high

---

### 4. OpenAPI Generator

#### 4.1 Go Client Quality

Claim: OpenAPI Generator's Go client is **widely criticized** as non-idiomatic, Java-like, and overly verbose. It generates getter/setter patterns, `Configuration` objects, and multi-step request builder patterns that feel unnatural in Go.[^8^][^33^]

Source: Speakeasy Blog — Idiomatic SDKs
URL: https://dev.to/speakeasy/idiomatic-sdks-for-openapi-4i94
Date: 2022-12-06
Excerpt: "The SDKs for languages like Go were actually quite Java-like and less idiomatic to the Go language... The openapi-generator outputs comments to help with usage... [but] generated a lot of additional getter/setter, instantiation and serialization methods that aren't required and just reduce the readability of the SDKs code."
Context: Direct code comparison showing OpenAPI Generator's builder pattern vs. Speakeasy's single-call approach.
Confidence: high

Source: Hacker News Discussion
URL: https://news.ycombinator.com/item?id=36145131
Date: 2023-05-31
Excerpt: "Basically it sucks, or at least it sucks in enough languages to spoil it... Often there are a half dozen clients for each language, often they are simply broken (the generated code just straight up doesn't compile)."
Context: Broad community consensus that OpenAPI Generator quality varies dramatically by language. Go is not a strong suit.
Confidence: high

#### 4.2 Dependency Weight

Claim: OpenAPI Generator's Go client pulls in **1,538 transitive dependencies** (`go mod graph | wc -l`). By contrast, oapi-codegen and ogen add **zero** external dependencies to generated code, and Speakeasy adds only 2.[^34^]

Source: Speakeasy Dependency Comparison
URL: https://www.speakeasy.com/docs/sdks/languages/golang/oss-comparison-go
Date: 2026-01-22
Excerpt: "The output for the OpenAPI Generator version is too long to show here, so we'll do a count instead: `go mod graph | wc -l` #> 1538"
Context: This dependency count is for the Petstore example — real-world specs could be even larger.
Confidence: high

Source: Three Dots Tech — Recommended Libraries
URL: https://threedots.tech/post/list-of-recommended-libraries/
Date: 2022-12-13
Excerpt: "We do not recommend using the official OpenAPI generator for the Go code. We recommend the `oapi-codegen` tool instead because of the higher quality of the generated code."
Context: Direct recommendation against OpenAPI Generator for Go.
Confidence: high

#### 4.3 Java Runtime Requirement

Claim: OpenAPI Generator requires a **Java runtime** (NPM wrapper, Homebrew, or JAR). This adds deployment complexity and breaks the "Go-native toolchain" principle that Mesh should follow.[^35^]

Source: Speakeasy Blog — OpenAPI Generator Experience
URL: https://dev.to/speakeasy/idiomatic-sdks-for-openapi-4i94
Date: 2022-12-06
Excerpt: "We had to install both NPM and Java before we could get the installation working... We had better luck with the homebrew install instruction further down the page."
Context: Installation friction is a known pain point.
Confidence: high

---

### 5. Non-OpenAPI Providers (Fly Machines)

#### 5.1 Fly Machines OpenAPI Spec Availability

Claim: Fly Machines API **now provides an official OpenAPI spec** at `https://docs.machines.dev/swagger/doc.json` (Swagger 2.0 format, with OpenAPI 3.0 available via the docs portal). This resolves the historical gap from 2022 when no spec was available.[^11^][^12^]

Source: Fly Machines API Documentation
URL: https://fly.io/docs/machines/api/
Date: Ongoing (updated 2024-2025)
Excerpt: "OpenAPI spec: OpenAPI 3.0 specification for the Machines API."
Context: The spec is linked from the official Fly docs and hosts a Swagger UI at docs.machines.dev.
Confidence: high

Source: Fly Community Forum (2022)
URL: https://community.fly.io/t/fly-machines-rest-api-openapi-specification/8207
Date: 2022-10-27
Excerpt: "Right now, we don't have an eta on when that would be available though."
Context: Historical context showing Fly did not have a spec initially. This has since been resolved.
Confidence: high

#### 5.2 API Docs → OpenAPI Conversion Tools

Claim: For providers without official OpenAPI specs, tools like **Swagger codegen's reverse-engineering**, **optic.dev**, **stoplight/spectral**, and **llm-based converters** exist. However, accuracy is variable and manual review is always required.[^36^]

Source: Various OpenAPI tooling surveys
URL: https://openapi.tools/
Date: Ongoing
Excerpt: "There are tools that can generate an OpenAPI spec from existing APIs, server code, or documentation."
Context: These tools exist but are not always reliable for complex REST APIs.
Confidence: medium

#### 5.3 Hand-Written HTTP Client Acceptability

Claim: For providers with small or stable APIs, **hand-written Go HTTP clients** are acceptable and sometimes preferred. However, for APIs with 20+ endpoints or frequent changes, the maintenance burden favors generated clients.[^37^]

Source: Go Community Consensus (Three Dots Tech)
URL: https://threedots.tech/post/list-of-recommended-libraries/
Date: 2022-12-13
Excerpt: "It's much easier to generate it the other way around: Go code from OpenAPI spec."
Context: The recommendation is spec-first with code generation, not manual client writing.
Confidence: medium

#### 5.4 AI-Generated Clients from REST Docs

Claim: **AI-generated clients from REST API documentation** are not reliable for production use. A 2024 academic study found that even the best LLM prompting strategies produced only **67% relevant LOC** with compilation failures, and required human intervention.[^19^]

Source: DIVA Academic Paper — Code Generation from Large API Specifications
URL: https://www.diva-portal.org/smash/get/diva2:1877570/FULLTEXT01.pdf
Date: 2024
Excerpt: "The results show that the percentage of relevant code generated is 67% with the phased prompting solution, compared to 91% with the single comprehensive prompt, which is a 26% decrease in relevant content... the LLMs could not produce the target REST API on its own."
Context: The study used commercially available LLMs (Mixtral, etc.) to generate Java REST APIs from OpenAPI specs. Results show AI output is useful as a baseline but not production-ready.
Confidence: high

---

## Contradictions and Conflict Zones

### C1: ogen vs oapi-codegen — "Type Safety" vs "Idiomatic Go"

There is **active debate** about whether ogen's `Opt*` structs (e.g., `OptPetStatus`) are superior to pointer-based optionals. ogen claims this is safer (no nil dereferences). Critics (including Speakeasy's analysis) argue it is **less idiomatic** because Go convention prefers pointers for optionality.[^27^]

- **ogen position**: `Opt*` types are type-safe, self-documenting, and prevent nil pointer errors.
- **Traditional Go position**: Pointers are the standard idiom; introducing `Opt*` structs adds API surface area and cognitive overhead.

**Resolution**: For Mesh, either approach works. If Mesh values type safety over strict idiomatic conformance, ogen's approach is acceptable. If Mesh values community familiarity, oapi-codegen's pointer approach is safer.

### C2: "Single File" vs "Multi-File" Generation

oapi-codegen intentionally generates **one file** per output type. ogen generates **multiple organized files**. The trade-off is:
- Single file: easier to vendor, simpler build rules, but can become unwieldy (10K+ LOC files).
- Multi-file: better code organization, easier navigation, but more complex import graphs.

### C3: Speakeasy "Worth It" for Internal Code

Speakeasy's code quality is highest, but its **$600/mo/language** price tag is hard to justify for **internal infrastructure clients** (like Mesh plugins). Speakeasy is optimized for:
- External developer SDKs (public GitHub repos)
- Multi-language generation
- Built-in retries, pagination, docs integration

For internal Go-only clients, the value proposition is weaker.

### C4: OpenAPI Generator — "It Works for Java"

OpenAPI Generator is widely used and works well for **Java/Spring Boot** (as noted in HN comments[^33^]). The Go generator, however, is a secondary citizen. This creates confusion when evaluating the tool holistically — it is excellent for some languages but poor for Go.

---

## Gaps in Available Information

1. **ogen long-term stability**: No published case studies of ogen in large-scale production (10K+ LOC generated). Claims are based on the petstore-sized examples.

2. **oapi-codegen v2 migration impact**: The deepmap → oapi-codegen migration was relatively smooth, but there is limited documentation on whether `go.mod` import paths caused breakage for downstream consumers.

3. **File upload edge cases in oapi-codegen**: While file upload is nominally supported, complex multipart scenarios (mixed JSON + file fields) are undertested in the community.

4. **Speakeasy Go SDK dependency graph depth**: Speakeasy claims 2 direct dependencies, but the full transitive closure for a real-world API is not publicly documented.

5. **Fly Machines spec freshness**: The `doc.json` spec is Swagger 2.0. Whether an OpenAPI 3.0 spec is available directly (not just via conversion) is unclear.

6. **AI-generated OpenAPI specs from REST docs**: No tool was found that reliably converts HTML REST API docs (like Fly's) into accurate OpenAPI 3.0 specs without human review.

---

## Preliminary Recommendations

### Tier 1: Primary Recommendation — oapi-codegen v2

| Criterion | Assessment | Confidence |
|---|---|---|
| Maturity | **Excellent** — 8,600+ stars, active maintenance, widely used | high |
| Go idiomatic | **Good** — pointers for optionals, standard Go patterns | high |
| Multi-file `$ref` | **Acceptable** — works with loader, some edge cases | medium |
| Enum generation | **Good** — typed string aliases, `x-enum-varnames` | high |
| oneOf/anyOf | **Limited** — `interface{}` for unions | high |
| File upload/download | **Good** — `io.Reader`/`io.ReadCloser` | high |
| Error types | **Weak** — `*http.Response` + manual parsing | high |
| Auth injection | **Good** — all OpenAPI security schemes | high |
| Server generation | **Excellent** — chi, Echo, Gin, strict server | high |
| Dependencies | **Zero** external in generated code | high |
| Performance | **Good** — not as fast as ogen but acceptable | high |

**Failure modes to plan for**:
1. Wrap `ParseResponse` results into typed errors manually.
2. Flatten or preprocess multi-file specs if `$ref` resolution fails.
3. Add `//go:generate` directives and CI validation to prevent drift.

### Tier 2: Secondary Recommendation — ogen

Use ogen when:
- API responses are **large JSON payloads** (streaming parser benefit).
- **Generation speed** matters (CI pipeline speed).
- Type-safe `Opt*` types are preferred over pointers.
- Server generation is **not needed** (ogen's server story is weaker).

### Tier 3: Conditional — Speakeasy

Use Speakeasy only if Mesh plans to **publish an external developer SDK** with multi-language support, retries, pagination, and premium DX. For internal plugin clients, the cost ($600+/mo) is not justified.

### Tier 4: Avoid — OpenAPI Generator

**Do not use** for Mesh's Go codebase. The Java runtime requirement, 1,538 dependencies, and non-idiomatic Go output are disqualifying for infrastructure software.

### Tier 5: Fallback — Hand-Written Client

For providers without OpenAPI specs (historically Fly, now resolved), a hand-written client using `net/http` + `encoding/json` is acceptable for APIs with <15 endpoints. For Fly specifically, the spec is now available, so generation is preferred.

### Toolchain Architecture Recommendation

```
OpenAPI Spec (provider)
       |
       v
[ oapi-codegen v2 CLI ]  ←  go:generate directive in plugin code
       |
       v
Generated client package:
  - types.go (or *_types.gen.go)
  - client.go (or *_client.gen.go)
  - errors.go (MANUAL WRAPPER — wraps *http.Response into typed errors)
  - options.go (MANUAL — retry/backoff config)
```

**Required manual additions** (regardless of generator):
1. **Typed error wrapper**: Map HTTP status codes to `fmt.Errorf` or custom error types.
2. **Request/response logging**: Interceptor/middleware for observability.
3. **Retry logic**: `github.com/cenkalti/backoff/v4` or similar (Speakeasy includes this; oapi-codegen does not).
4. **Rate limit handling**: Parse `Retry-After` headers.

---

## Citations

[^1^]: https://github.com/oapi-codegen/oapi-codegen — oapi-codegen official repository, ~8,600 stars, active development.

[^2^]: https://github.com/deepmap/oapi-codegen — Archived deepmap repository, read-only since April 2024.

[^3^]: https://blog.logto.io/engineering/2022/03/30/deepmap-oapi-codegen-workflow/ — Logto Engineering, "The OpenAPI-to-code workflow," 2022-03-30.

[^4^]: https://github.com/ogen-go/ogen — ogen GitHub repository, performance claims documented.

[^5^]: https://www.speakeasy.com/docs/sdks/languages/golang/oss-comparison-go — "Comparison guide: OpenAPI/Swagger Go client generation," 2026-01-22.

[^6^]: https://www.speakeasy.com/blog/speakeasy-vs-stainless — "In Depth: Speakeasy vs Stainless," 2026-01-22.

[^7^]: https://dev.to/speakeasy/idiomatic-sdks-for-openapi-4i94 — "Idiomatic SDKs for OpenAPI," 2022-12-06.

[^8^]: https://news.ycombinator.com/item?id=36145131 — Hacker News discussion on OpenAPI Generator quality, 2023-05-31.

[^9^]: https://github.com/OpenAPITools/openapi-generator/issues/7490 — GitHub issue: "Do people successfully use this?" 2020-09-23.

[^10^]: https://threedots.tech/post/list-of-recommended-libraries/ — "The Go libraries that never failed us," 2022-12-13.

[^11^]: https://fly.io/docs/machines/api/ — Fly Machines API docs, linking to OpenAPI spec.

[^12^]: https://docs.machines.dev/swagger/doc.json — Fly Machines API Swagger 2.0 specification.

[^13^]: https://github.com/oapi-codegen/oapi-codegen/blob/main/examples/petstore-expanded/chi/api/petstore.gen.go — Generated client example showing `*http.Response` return pattern.

[^14^]: https://github.com/oapi-codegen/oapi-codegen/issues — Multiple issues regarding client error handling and typed errors.

[^15^]: https://github.com/oapi-codegen/oapi-codegen/issues/127 — "Multi-file OpenAPI specs and `$ref` handling," 2020-04-28.

[^16^]: https://github.com/oapi-codegen/oapi-codegen/issues/1126 — `$ref` across files edge cases.

[^17^]: https://github.com/oapi-codegen/oapi-codegen/tree/main/examples/strict — Strict server generation examples.

[^18^]: https://blog.logto.io/engineering/2022/03/30/deepmap-oapi-codegen-workflow/ — Server generation via deepmap/oapi-codegen.

[^19^]: https://www.diva-portal.org/smash/get/diva2:1877570/FULLTEXT01.pdf — "Code Generation from Large API Specifications with Open Source LLMs," 2024.

[^20^]: https://github.com/oapi-codegen/oapi-codegen/blob/main/README.md — Enum and `x-enum-varnames` documentation.

[^21^]: https://github.com/oapi-codegen/oapi-codegen/issues/1235 — oneOf/anyOf generating `interface{}`.

[^22^]: https://github.com/oapi-codegen/oapi-codegen — File upload via `format: binary` / `multipart/form-data`.

[^23^]: https://github.com/oapi-codegen/oapi-codegen/blob/main/README.md — Security schemes support.

[^24^]: https://github.com/oapi-codegen/oapi-codegen/issues — Issue backlog covering limitations.

[^25^]: https://www.speakeasy.com/docs/sdks/languages/golang/oss-comparison-go — oapi-codegen single-file output critique.

[^26^]: https://medium.com/@MikeMwita/generating-go-code-from-openapi-specification-document-ae225e49e970 — "Generating Go code from OpenAPI Specification Document," 2023-09-28.

[^27^]: https://www.speakeasy.com/docs/sdks/languages/golang/oss-comparison-go — Nullable fields comparison (ogen `Opt*` vs Speakeasy pointers).

[^28^]: https://github.com/ogen-go/ogen/issues — ogen issue tracker showing maturity gaps.

[^29^]: https://www.speakeasy.com/blog/speakeasy-vs-stainless — Pricing table: Free / Business / Enterprise tiers.

[^30^]: https://www.speakeasy.com/blog/what-is-an-mcp-gateway — "What is an MCP gateway and do I need one?" 2026-02-03.

[^31^]: https://www.speakeasy.com/blog/speakeasy-vs-stainless — "Speakeasy is a standalone CLI binary, so it can run anywhere: on-prem, in a locked-down CI pipeline, or fully air-gapped."

[^32^]: https://www.speakeasy.com/docs/sdks/languages/golang/oss-comparison-go — Dependency comparison showing 2 deps for Speakeasy vs. 1,538 for OpenAPI Generator.

[^33^]: https://news.ycombinator.com/item?id=36145131 — HN comment: "The Java generator is pretty good, many big companies are using it... the Go [generator] was actually quite Java-like."

[^34^]: https://www.speakeasy.com/docs/sdks/languages/golang/oss-comparison-go — "The output for the OpenAPI Generator version is too long to show here... `go mod graph | wc -l` #> 1538."

[^35^]: https://dev.to/speakeasy/idiomatic-sdks-for-openapi-4i94 — "We had to install both NPM and Java before we could get the installation working."

[^36^]: https://openapi.tools/ — OpenAPI tooling directory.

[^37^]: https://threedots.tech/post/list-of-recommended-libraries/ — Recommendation: "It is much easier to generate it the other way around: Go code from OpenAPI spec."
