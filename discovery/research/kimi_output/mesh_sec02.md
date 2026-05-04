## 2. Codegen Toolchain Recommendation

Mesh's value proposition — generating provider adapters from machine-readable API specifications — depends on a code generation toolchain that is deterministic, fast, and produces Go-idiomatic output. This chapter evaluates four candidate generators and issues a verdict with explicit failure modes to avoid.

### 2.1 oapi-codegen v2 Deep Dive

The canonical repository migrated from the archived `deepmap/oapi-codegen` to `oapi-codegen/oapi-codegen` (the v2 line) in April 2024.[^1^][^2^] The new organization hosts an actively maintained project with approximately 8,600 GitHub stars and framework support spanning chi, Echo, Gin, gorilla/mux, and net/http.[^1^][^3^]

**Multi-file specs and `$ref` handling.** oapi-codegen resolves external `$ref` references through `openapi3.NewLoader()`, which supports relative file paths across split specifications.[^15^] Community reports document edge cases where nested relative paths fail, sometimes requiring a preprocessing flattening step.[^16^] The workflow: attempt direct generation, then fall back to `swagger-cli bundle` if resolution fails.

**Enum types, optional fields, and oneOf behavior.** The generator produces typed string aliases for `enum` schemas and respects the `x-enum-varnames` extension.[^20^] Optional fields render as pointers or `omitempty` JSON tags. The critical limitation is `oneOf`/`anyOf`: because Go lacks native sum types, polymorphic schemas collapse to `interface{}`, requiring manual type assertions in the mapping layer.[^21^] This is a systemic Go constraint.

**Error types: the untyped `*http.Response` gap.** oapi-codegen's `ParseResponse` helper yields typed success structs but leaves non-2xx responses as raw `*http.Response` and `[]byte` body content.[^13^][^14^] There is no typed error hierarchy; callers must manually inspect status codes. This is the largest ergonomic gap. Mesh's generator skill must emit a wrapper mapping HTTP status codes to domain-specific errors.

**Auth injection.** All OpenAPI security schemes — apiKey, http/bearer, oauth2, and openIdConnect — are supported via generated request editor functions or context-based injection.[^23^] oapi-codegen's strict server generation enforces compile-time validation across major Go routers, indirectly validating the quality of the client path.[^17^][^18^]

### 2.2 Alternative Generators

**ogen: faster, structured output, smaller community.** ogen (github.com/ogen-go/ogen) produces type-safe structured code across multiple organized files, contrasting with oapi-codegen's single-file output.[^4^][^5^] It uses a streaming JSON parser and generates code in approximately 80 ms.[^4^] Optional fields use `OptString`, `OptInt` rather than pointers — safer against nil dereferences but debated as non-idiomatic.[^27^] Its server generation is weaker, acceptable for Mesh's client-only use case.[^28^]

**Speakeasy: commercial, idiomatic, free-tier limited.** Speakeasy generates the most idiomatic Go among evaluated tools, with only two direct dependencies and built-in pagination, retry, and mockability.[^7^][^32^] The free tier is limited to one language and 250 operations; the Business tier costs $600 per month per language.[^6^][^29^] Distributed as a standalone CLI binary supporting on-prem deployments, it is optimized for external developer SDKs, not internal infrastructure clients.[^31^]

**OpenAPI Generator: avoid for Go.** The Java-based generator requires a Java runtime and produces Go code widely criticized as non-idiomatic — getter/setter patterns, multi-step request builders, and `Configuration` objects.[^8^][^33^] The generated Go client pulls in 1,538 transitive dependencies for a Petstore-sized spec, versus zero for oapi-codegen or ogen.[^34^] Community consensus is to avoid it for Go projects.[^10^]

> **Avoid this:** OpenAPI Generator for Go output. The Java runtime dependency, 1,538 transitive dependencies, and Java-like code patterns make it unsuitable for infrastructure software.

### 2.3 Non-OpenAPI Providers

The presence of an OpenAPI spec is the single strongest predictor of adapter generation success. Providers bifurcate into "auto-generatable" (spec available) and "hand-written" (no spec).

**Fly Machines now has a spec; E2B and AWS require hand-written clients.** Fly Machines resolved its historical gap and now publishes an official spec at `https://docs.machines.dev/swagger/doc.json` (Swagger 2.0, with OpenAPI 3.0 via the docs portal).[^11^][^12^] E2B and AWS EC2 do not expose machine-readable OpenAPI specifications; AWS adapters wrap the official `aws-sdk-go-v2`, while E2B requires manual HTTP client construction.

**AI-generated clients from REST docs: 67% compilation failure rate.** A 2024 academic study evaluating LLM-based REST API client generation from unstructured documentation found that even optimized prompting strategies produced only 67% relevant lines of code, with compilation failures requiring human intervention.[^19^] AI generation is viable for the 200-line mapping layer but cannot reliably produce the HTTP client itself — the deterministic generator must handle that part.

> **Steal this:** For providers without OpenAPI specs, use the upstream SDK if one exists (AWS, GCP, Azure), or write a minimal `net/http` client (~150–300 lines) for small APIs (<15 endpoints). Do not attempt AI generation of the HTTP client from REST documentation.

### 2.4 Toolchain Verdict

#### 2.4.1 Generator Comparison

| Feature | oapi-codegen v2 | ogen | Speakeasy | OpenAPI Generator |
|---------|-----------------|------|-----------|-------------------|
| GitHub Stars | ~8,600 [^1^] | ~900 [^4^] | Commercial [^6^] | ~22,000 (Java) [^8^] |
| Generated Deps | 0 [^25^] | 0 [^5^] | 2 [^32^] | 1,538 [^34^] |
| Generation Speed | Good | ~80 ms [^4^] | Fast | Slow (Java) [^35^] |
| Multi-file Output | No (single file) [^25^] | Yes [^5^] | Yes [^7^] | Yes [^8^] |
| oneOf/anyOf | `interface{}` [^21^] | Limited | Typed unions [^7^] | `interface{}` [^8^] |
| Error Types | `*http.Response` [^13^] | Typed | Typed [^7^] | Typed |
| Auth Injection | All schemes [^23^] | Standard | All schemes [^7^] | All schemes |
| Server Generation | Excellent [^17^] | Limited [^28^] | No | Yes |
| Retry / Pagination | Manual | Manual | Built-in [^7^] | Manual |
| Go Idiomatic | Good | Good (debated `Opt*`) | Best [^7^] | Poor [^33^] |
| Runtime Required | None | None | None | Java [^35^] |
| Cost | Free | Free | Free tier: 250 ops [^29^] | Free |

oapi-codegen v2 and ogen occupy the open-source, zero-dependency tier, differing on output structure and maturity. Speakeasy is the quality leader but gated by pricing. OpenAPI Generator is an outlier in the wrong direction.

**Primary recommendation: oapi-codegen v2.** The most battle-tested open-source generator, with the largest community and zero dependencies in generated code. The untyped error limitation is addressable with a thin manual wrapper.

**Secondary: ogen.** Use when streaming large JSON payloads, when CI generation speed is critical, or when type-safe `Opt*` types are preferred over pointers.

**Conditional: Speakeasy.** Evaluate only if Mesh publishes an external developer SDK where built-in retries, pagination, and premium documentation justify the $600+/month cost.

**Avoid: OpenAPI Generator for Go.** The dependency weight and runtime requirement disqualify it for infrastructure software.

> **Steal this:** Structure each adapter package as: `types.gen.go` (oapi-codegen output), `client.gen.go` (oapi-codegen output), `errors.go` (manual wrapper mapping `*http.Response` to typed errors), and `adapter.go` (AI-generated mapping layer). Add `//go:generate` directives to keep generation reproducible in CI.

#### 2.4.2 Failure Modes to Avoid

Mesh should plan for four specific failure modes:

1. **oneOf/anyOf schemas.** Polymorphic responses generate `interface{}` fields. The mapping layer must emit safe type assertions with fallback error handling. Confidence: High.

2. **Multi-file `$ref` path resolution.** If a provider distributes its spec across multiple files, `$ref` resolution may fail on nested relative paths. The pipeline should attempt direct loading first, then fall back to flattening. Confidence: High.

3. **Untyped HTTP errors.** oapi-codegen returns non-2xx responses as raw `*http.Response` objects. Every adapter must include a standardized error wrapper mapping status codes to domain errors so callers can distinguish transient from permanent failures. Confidence: High.

4. **Missing retry and observability.** None of the open-source generators emit retry logic, request logging, or metrics. Mesh must inject these via interceptors (`github.com/cenkalti/backoff/v4` for retries, `go.opentelemetry.io/otel` for tracing). Confidence: High.

