# Dimension 12: Testing, Observability & Error Handling for Adapters

## Executive Summary

- **Interface-based mocking** is the idiomatic Go approach for testing SubstrateAdapters without real credentials. The `SubstrateAdapter` interface itself enables hand-written fakes or generated mocks (gomock/mockery) for unit tests[^1^][^2^].
- **HTTP recording/replay** via `go-vcr` or `govcr` provides deterministic integration-style tests without network calls or credentials, with support for request matchers, hooks to redact auth headers, and cassette-based fixtures[^3^][^4^].
- **Testcontainers-go with LocalStack** enables real AWS service emulation in Docker containers for Go integration tests, providing a consistent local and CI environment without cloud costs[^5^][^6^].
- **Provider SDKs (AWS SDK v2)** have built-in retry middleware with exponential jitter backoff and adaptive rate-limiting; adapter-level retry should complement, not duplicate, SDK behavior[^7^].
- **OpenTelemetry** provides the unified observability backbone: `slog` + `otelslog` bridge for structured logging with trace correlation, histograms for Create/Start/Stop latency, and manual spans for provider-call tracing[^8^][^9^].
- **Circuit breakers** (`sony/gobreaker`) and **exponential backoff** (`cenkalti/backoff`) are production-proven patterns for provider resilience; combine them with layered retry for transient failures and breaker for systemic ones[^10^][^11^].
- **Validation tooling** for generated adapters should use `golangci-lint` with `generated: lax` exclusion policy, `staticcheck` for code quality, `gosec` for security anti-patterns, and `govulncheck` for dependency vulnerabilities[^12^][^13^].
- **Coverage exclusion** for generated mock files is best achieved with the `_test.go` file suffix, which Go tools automatically exclude from coverage calculations[^14^].
- **Provider free tiers** exist across AWS (12-month), Azure (12-month + $200 credit), and GCP ($300 credit + always-free), making real-cloud integration testing feasible but requiring parallel execution limits to avoid rate-limiting[^15^].
- **Timeout strategy** should use `context.WithTimeout` per adapter verb (Create/Start/Stop/Delete), with different durations per verb's expected latency, propagated from the caller through the adapter to the SDK[^16^].

---

## 1. Testing Without Credentials

### 1.1 Interface-Based Mocking (Test Against SubstrateAdapter)

The core principle of testing adapters without credentials is to depend on interfaces, not concrete SDK clients. Since `SubstrateAdapter` is itself an interface, any code consuming adapters can be tested by passing a hand-written fake or generated mock.

**Hand-written fakes** are the most idiomatic Go approach for small interfaces:

```go
type FakeComputeAdapter struct {
    CreateFunc func(context.Context, ComputeSpec) (ResourceID, error)
    // ...
}

func (f *FakeComputeAdapter) Create(ctx context.Context, spec ComputeSpec) (ResourceID, error) {
    return f.CreateFunc(ctx, spec)
}
```

Claim: Hand-written fakes are preferred in Go for small interfaces because they offer compile-time type safety, transparent IDE assistance, and zero generation overhead[^1^].
Source: Harrison Cramer blog
URL: https://harrisoncramer.me/mocking-interfaces-in-go
Date: 2024-08-18
Excerpt: "When testing, we can write a different translator that satisfies the Translator interface, without having to hit that external API."
Context: Article demonstrates interface-based mocking without external dependencies.
Confidence: high

**gomock** (from `go.uber.org/mock`) generates mock implementations from interfaces and provides a rich DSL for expectations:

```go
mockAdapter := mock_main.NewMockSubstrateAdapter(ctrl)
mockAdapter.EXPECT().Create(gomock.Any(), gomock.Any()).Return("id-123", nil)
```

Claim: gomock generates strongly-typed mock implementations with call verification, argument matchers (`gomock.Any()`, `gomock.Eq()`), and call count expectations (`Times()`, `InOrder()`)[^2^].
Source: Leapcell blog
URL: https://leapcell.io/blog/mastering-mocking-in-go-gomock-vs-interface-based-fakes
Date: 2025-08-05
Excerpt: "gomock generates code, so type mismatches or incorrect method calls are caught at compile-time... offers a powerful and expressive API for defining expectations."
Context: Comparison of gomock vs hand-written fakes.
Confidence: high

**Mockery** (`vektra/mockery`) is an alternative that generates one file per interface. A common complaint is directory bloat, though it offers auto-generation on file change[^17^].

Claim: mockery generates one file per interface, which can clutter packages; mockgen allows generating multiple mocks in a single file via `//go:generate mockgen -package foo -source=models.go -destination=mocks_test.go *`[^17^].
Source: Medium - Matteo Pampana
URL: https://medium.com/@matteopampana/how-i-generate-mocks-in-go-like-a-boss-710663749f06
Date: 2024-03-14
Excerpt: "mockery is great, but — at the time of writing — it generates a file for each interface to be mocked. This will bloat your package directory."
Context: Author's preferred solution using mockgen with `_test.go` suffix for coverage exclusion.
Confidence: high

### 1.2 HTTP Recording/Replay (go-vcr, recorder, govcr)

VCR-style testing records real HTTP interactions to "cassettes" and replays them in subsequent test runs. This tests the complete HTTP stack without network calls.

**go-vcr** (`gopkg.in/dnaeon/go-vcr.v3`) is the most popular Go VCR library:

```go
rec, err := recorder.New("fixtures/matchers")
defer rec.Stop()
client := rec.GetDefaultClient()
```

Claim: go-vcr v3 provides custom request matching via `MatcherFunc`, replayable interactions, and cassette-based fixtures; note that v3 cassettes are not backward-compatible with older releases[^3^].
Source: pkg.go.dev - go-vcr v3
URL: https://pkg.go.dev/gopkg.in/dnaeon/go-vcr.v3
Date: 2024-03-13
Excerpt: "go-vcr simplifies testing by recording your HTTP interactions and replaying them in future runs... as of go-vcr v3 there is a new format of the cassette, which is not backwards-compatible with older releases."
Context: Official package documentation for go-vcr v3.
Confidence: high

**Critical feature for adapter testing**: Hooks to redact sensitive data before saving cassettes:

```go
hook := func(i *cassette.Interaction) error {
    delete(i.Request.Headers, "Authorization")
    return nil
}
opts := []recorder.Option{
    recorder.WithHook(hook, recorder.BeforeSaveHook),
}
```

Claim: go-vcr supports `BeforeSaveHook` to remove or replace sensitive data (e.g., `Authorization` headers) before it is stored on disk, preventing credential leakage in test fixtures[^4^].
Source: GitHub - dnaeon/go-vcr
URL: https://github.com/dnaeon/go-vcr
Date: 2015-12-14
Excerpt: "Removing or replacing data before it is stored can be done by adding one or more Hooks to your Recorder... BeforeSaveHook: modify the recorded interactions right before they are saved on disk."
Context: README documentation on hooks and passthrough features.
Confidence: high

**govcr** (`github.com/seborama/govcr`) is another mature option with cassette encryption, cloud storage support (AWS S3), and multiple playback modes (Normal, Live-only, Offline, Read-only)[^18^].

Claim: govcr supports operation modes including Offline mode (playback only, transport error if no track matches) and cassette encryption for sensitive recorded data[^18^].
Source: GitHub - seborama/govcr
URL: https://github.com/seborama/govcr
Date: 2025-04-19
Excerpt: "Offline: playback from cassette only, return a transport error if no track matches... Recipe: VCR with encrypted cassette."
Context: README with cookbook recipes for various configurations.
Confidence: high

### 1.3 Provider Sandboxes (Free Tiers)

All major cloud providers offer free tiers or credits for testing:

| Provider | Free Tier | Credits | Duration |
|----------|-----------|---------|----------|
| AWS | 12-month free tier (750 hrs/mo EC2, S3, etc.) | - | 12 months |
| Azure | 12-month free tier + popular services always free | $200 | 30 days |
| GCP | Always-free tier (1 f1-micro VM, etc.) | $300 | 90 days |
| Oracle Cloud | Always-free tier | - | Unlimited |

Claim: AWS, Azure, GCP, and OCI all offer free trials, always-free services, and initial credits to attract new customers, allowing testing without incurring costs[^15^].
Source: EffectiveSoft blog
URL: https://www.effectivesoft.com/blog/cloud-pricing-comparison.html
Date: 2026-01-23
Excerpt: "AWS, Azure, GCP, and OCI all offer free trials, always-free services, and an initial credit to attract new customers, allowing them to test services without incurring costs."
Context: Cloud pricing comparison for 2026.
Confidence: high

### 1.4 Fake/Test Substrates for CI

**LocalStack** provides a fully functional local AWS cloud stack emulator. Combined with **testcontainers-go**, it runs in Docker during tests:

```go
localstackContainer, err := localstack.Run(ctx, "localstack/localstack:1.4.0")
```

Claim: Testcontainers-go's LocalStack module provides a fully functional local AWS cloud stack for testing without real AWS credentials; supports S3, DynamoDB, Lambda, SQS, and more[^5^].
Source: Testcontainers for Go docs
URL: https://golang.testcontainers.org/modules/localstack/
Date: Unknown
Excerpt: "The Testcontainers module for LocalStack is 'a fully functional local AWS cloud stack', to develop and test your cloud and serverless apps without actually using the cloud."
Context: Official testcontainers-go module documentation.
Confidence: high

Claim: LocalStack with testcontainers-go enables running integration tests for AWS Lambda in CI by copying ZIP artifacts into the container and invoking via HTTP, with tests passing in seconds[^6^].
Source: CONF42 Golang 2025 talk transcript
URL: https://www.conf42.com/Golang_2025_Manuel_de_la_Pena_testing_go_integration
Date: 2025-04-03
Excerpt: "We are able to run locally AWS Lambdas and verify that our code is working as expected... in just a few seconds we are able to run locally AWS Lambdas."
Context: Talk on delightful integration tests in Go using testcontainers-go.
Confidence: high

---

## 2. Unit Testing Patterns

### 2.1 Mocking SDK Clients (gomock, mockery)

AWS SDK for Go v2 officially recommends mocking the interface for testing[^19^]. However, creating an interface for every SDK client can be tedious. Two approaches exist:

**Approach A: Define narrow interfaces for only the methods you use**

```go
type EC2Client interface {
    RunInstances(ctx context.Context, params *ec2.RunInstancesInput, optFns ...func(*ec2.Options)) (*ec2.RunInstancesOutput, error)
    TerminateInstances(ctx context.Context, params *ec2.TerminateInstancesInput, optFns ...func(*ec2.Options)) (*ec2.TerminateInstancesOutput, error)
}
```

**Approach B: Use AWS SDK v2 middleware to mock responses without interfaces**

```go
middlewareFunc := func(ctx context.Context, in middleware.FinalizeInput, next middleware.FinalizeHandler) (middleware.FinalizeOutput, middleware.Metadata, error) {
    return middleware.FinalizeOutput{
        Result: &cloudformation.DeleteStackOutput{},
    }, middleware.Metadata{}, nil
}
```

Claim: AWS SDK for Go v2 allows mocking responses using middleware (FinalizeMiddlewareFunc) without creating interface mocks, though the official documentation recommends mocking the interface[^19^].
Source: dev.to - AWS Builders
URL: https://dev.to/aws-builders/testing-with-aws-sdk-for-go-v2-without-interface-mocks-55de
Date: 2023-08-09
Excerpt: "The method is 'Mock the SDK response using the middleware provided by AWS SDK for Go V2'... you can use the Client of the SDK as is for testing... in the next chapter of the official documentation ('Testing'), it is recommended that testing be done by mocking the interface."
Context: Article documenting both middleware-based and interface-based testing approaches.
Confidence: high

### 2.2 Table-Driven Tests for Adapter Methods

Table-driven tests are the canonical Go pattern for testing adapter methods exhaustively:

```go
func TestAdapterCreate(t *testing.T) {
    tests := []struct {
        name      string
        spec      ComputeSpec
        mockSetup func(*mock.MockSubstrateAdapter)
        wantID    string
        wantErr   error
    }{
        {
            name: "successful creation",
            spec: ComputeSpec{Image: "ubuntu-22.04"},
            mockSetup: func(m *mock.MockSubstrateAdapter) {
                m.EXPECT().Create(gomock.Any(), gomock.Any()).Return("vm-123", nil)
            },
            wantID:  "vm-123",
            wantErr: nil,
        },
        {
            name: "provider quota exceeded",
            spec: ComputeSpec{Image: "ubuntu-22.04"},
            mockSetup: func(m *mock.MockSubstrateAdapter) {
                m.EXPECT().Create(gomock.Any(), gomock.Any()).Return("", ErrQuotaExceeded)
            },
            wantID:  "",
            wantErr: ErrQuotaExceeded,
        },
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            ctrl := gomock.NewController(t)
            mock := mock.NewMockSubstrateAdapter(ctrl)
            tt.mockSetup(mock)
            
            got, err := mock.Create(context.Background(), tt.spec)
            if !errors.Is(err, tt.wantErr) {
                t.Errorf("Create() error = %v, wantErr %v", err, tt.wantErr)
            }
            if got != tt.wantID {
                t.Errorf("Create() = %v, want %v", got, tt.wantID)
            }
        })
    }
}
```

### 2.3 Error Path Testing

Error path testing should cover:
- SDK-level errors (network, auth, rate-limiting)
- Adapter-level validation errors
- Context cancellation / timeout errors
- Retry exhaustion errors

Claim: VCR testing can simulate error scenarios by modifying recorded fixtures to inject rate limiting (HTTP 429), server errors (HTTP 503), or increased response times[^20^].
Source: dev.to - Calvin McLean
URL: https://dev.to/calvinmclean/effortless-http-client-testing-in-go-4d75
Date: 2024-07-11
Excerpt: "You can take real-world responses and modify the generated fixtures to increase response time, cause rate limiting, etc. to test error scenarios that don't often occur organically."
Context: Article on HTTP client testing with VCR in Go.
Confidence: high

---

## 3. Integration Testing

### 3.1 Which Providers Have Free Tiers for Testing?

| Provider | Free Testing Option | Key Limitations |
|----------|--------------------|-----------------|
| AWS | 12-month free tier; LocalStack for local | Free tier expires; some services not in LocalStack |
| Azure | 12-month free tier + $200 credit; Dev/test pricing | Credit expires after 30 days |
| GCP | $300 credit + always-free tier | Credit expires after 90 days |
| Oracle Cloud | Always-free tier (no expiration) | Smaller ecosystem |
| DigitalOcean | $200 credit for 60 days | Credit expires |

Claim: GCP gives new customers a $300 credit valid for 90 days, plus always-free services including 1 f1-micro VM instance per month; AWS offers 12-month free tier with 750 hours/month of EC2[^15^].
Source: Medium - Cloud Platform Engineering
URL: https://medium.com/cloudplatformengineering/which-cloud-is-cheaper-aws-azure-gcp-and-stackit-bc2bc14c083f
Date: 2024-10-15
Excerpt: "Google Cloud offers always-free services with limited usage (e.g., 1 f1-micro VM instance per month), and a $300 credit for new customers valid for the first 90 days."
Context: In-depth cloud pricing comparison.
Confidence: high

### 3.2 Test Account Provisioning

For CI integration tests:
1. Use LocalStack/testcontainers for PR-level tests (no credentials needed)
2. Use real cloud accounts for nightly/weekly integration tests
3. Use unique resource naming prefixes per CI run to avoid collisions:
   ```go
   prefix := fmt.Sprintf("ci-%d-%s", os.Getpid(), time.Now().Format("20060102-150405"))
   ```

### 3.3 Cost of Integration Test Suite

Cost can be minimized by:
- Running unit + local integration tests on every PR (free)
- Running real-cloud integration tests on schedule (nightly) or on-demand
- Using spot/preemptible instances where possible
- Cleaning up resources aggressively (defer-based cleanup)

### 3.4 Parallel Test Execution Risks

Parallel test execution risks hitting provider rate limits. Best practices:

```yaml
# GitHub Actions example
integration-tests:
  strategy:
    fail-fast: false
    max-parallel: 3  # Limit to avoid API rate limits
```

Claim: The sweet spot for parallelism depends on the cloud provider's rate limits; for AWS, 4-6 parallel integration tests usually works well; start with 3-4 and increase until hitting rate limits[^21^].
Source: OneUptime blog
URL: https://oneuptime.com/blog/post/2026-02-23-how-to-use-parallel-testing-for-terraform/view
Date: 2026-02-23
Excerpt: "The sweet spot for parallelism depends on your cloud provider's rate limits and the types of resources your tests create. Start with 3-4 parallel tests and increase until you start hitting rate limits. For AWS, 4-6 parallel integration tests usually works well."
Context: Article on parallel testing strategies for Terraform (applicable to any cloud API testing).
Confidence: high

Additional mitigations:
- Add retry logic for rate-limited operations
- Use unique resource naming to avoid cross-test collisions
- Consider using `t.Parallel()` only for unit tests, not integration tests

---

## 4. Observability

### 4.1 Structured Logging Per Adapter

Go 1.21's `log/slog` is the standard structured logging library. For OpenTelemetry integration, use the `otelslog` bridge:

```go
import "go.opentelemetry.io/contrib/bridges/otelslog"
slog.SetDefault(otelslog.NewLogger("substrate-adapter"))
```

Claim: The otelslog bridge turns slog into an OpenTelemetry-native log source, automatically injecting trace ID and span ID into log records, enabling correlation between logs and distributed traces[^8^].
Source: Dash0 guide
URL: https://www.dash0.com/guides/opentelemetry-logging-in-go
Date: 2026-04-24
Excerpt: "The otelslog bridge turns slog into an OpenTelemetry-native log source. It implements slog.Handler, so your existing logging calls don't change, but under the hood every record flows through the OTel Logs SDK alongside your traces and metrics."
Context: Guide on OpenTelemetry-native logging in Go.
Confidence: high

### 4.2 Metrics: Create/Start/Stop Latency

OpenTelemetry histograms are the appropriate instrument for measuring adapter operation latency:

```go
meter := otel.Meter("substrate-adapter")
opHistogram, _ := meter.Int64Histogram(
    "substrate.adapter.operation.duration",
    metric.WithDescription("Duration of adapter operations"),
    metric.WithUnit("ms"),
)

// Record latency
start := time.Now()
result, err := adapter.Create(ctx, spec)
opHistogram.Record(ctx, time.Since(start).Milliseconds(),
    metric.WithAttributes(
        attribute.String("operation", "create"),
        attribute.String("provider", "aws"),
    ),
)
```

Claim: OpenTelemetry histograms with custom bucket boundaries should be tuned for the application's latency characteristics; for fast operations use fine-grained buckets (e.g., 10ms-1s), for slow operations use sparse buckets at the tails[^9^].
Source: OneUptime blog
URL: https://oneuptime.com/blog/post/2026-02-09-otel-histogram-buckets-latency/view
Date: 2026-02-09
Excerpt: "Define custom bucket boundaries tailored to your application's latency characteristics... Dense around typical web response times (10ms-1s), sparse at the tails for outlier capture."
Context: Guide on configuring OpenTelemetry histogram buckets for latency tracking.
Confidence: high

### 4.3 Health Checks for Provider Connectivity

A health check pattern for adapters should verify both connectivity and basic operations:

```go
type AdapterHealthChecker struct {
    name    string
    adapter SubstrateAdapter
}

type CheckResult struct {
    Name      string
    Status    Status // Healthy, Degraded, Unhealthy
    Duration  time.Duration
    Message   string
}

func (c *AdapterHealthChecker) Check(ctx context.Context) CheckResult {
    start := time.Now()
    // Perform a lightweight operation (e.g., list or describe)
    _, err := c.adapter.List(ctx)
    if err != nil {
        return CheckResult{Status: StatusUnhealthy, Message: err.Error(), Duration: time.Since(start)}
    }
    return CheckResult{Status: StatusHealthy, Message: "operational", Duration: time.Since(start)}
}
```

Claim: Health checks should verify both connectivity and the ability to perform basic operations, with status categories: Healthy, Degraded, and Unhealthy, and should always measure and report duration[^22^].
Source: OneUptime blog
URL: https://oneuptime.com/blog/post/2026-01-07-go-health-checks-kubernetes/view
Date: 2026-01-07
Excerpt: "Redis health checks should verify both connectivity and the ability to perform basic operations... Test basic connectivity... Test write operation... Test read operation... result.Status = StatusHealthy"
Context: Article on implementing health checks in Go for Kubernetes.
Confidence: high

### 4.4 Distributed Tracing Across Adapter Calls

Distributed tracing should create spans for each adapter verb call:

```go
func (a *AWSAdapter) Create(ctx context.Context, spec ComputeSpec) (ResourceID, error) {
    ctx, span := a.tracer.Start(ctx, "aws-adapter.create")
    defer span.End()
    span.SetAttributes(
        attribute.String("provider", "aws"),
        attribute.String("region", a.region),
    )
    
    result, err := a.ec2Client.RunInstances(ctx, input)
    if err != nil {
        span.RecordError(err)
        span.SetStatus(codes.Error, err.Error())
        return "", err
    }
    return ResourceID(*result.Instances[0].InstanceId), nil
}
```

Claim: Wrapping the HTTP client with `otelhttp.NewTransport` automatically injects traceparent headers and creates client-side spans for every outgoing HTTP call; always pass `ctx` through to maintain trace chain continuity[^23^].
Source: dev.to - young_gao
URL: https://dev.to/young_gao/distributed-tracing-with-opentelemetry-a-practical-guide-for-go-services-pep
Date: 2026-03-21
Excerpt: "Wrap your HTTP client with otelhttp.NewTransport... Now every outgoing HTTP call automatically injects traceparent headers and creates a client-side span... always pass ctx through."
Context: Practical guide on distributed tracing with OpenTelemetry in Go.
Confidence: high

---

## 5. Error Handling

### 5.1 Provider Errors vs Adapter Errors

Adapter errors should wrap provider SDK errors to preserve the error chain:

```go
var (
    ErrNotFound     = errors.New("resource not found")
    ErrUnauthorized = errors.New("unauthorized")
    ErrRateLimited  = errors.New("rate limited")
)

func (a *AWSAdapter) Create(ctx context.Context, spec ComputeSpec) (ResourceID, error) {
    result, err := a.ec2Client.RunInstances(ctx, input)
    if err != nil {
        var apiErr smithy.APIError
        if errors.As(err, &apiErr) {
            switch apiErr.ErrorCode() {
            case "UnauthorizedOperation":
                return "", fmt.Errorf("AWS adapter create: %w: %v", ErrUnauthorized, err)
            }
        }
        return "", fmt.Errorf("AWS adapter create: %w", err)
    }
    return ResourceID(*result.Instances[0].InstanceId), nil
}
```

Claim: Go 1.13's `%w` verb preserves the original error in the chain, enabling `errors.Is` and `errors.As` inspection; `%v` does not preserve the chain[^24^].
Source: OneUptime blog
URL: https://oneuptime.com/blog/post/2026-01-23-go-error-wrapping/view
Date: 2026-01-23
Excerpt: "Using %v - loses the original error... errors.Is(errV, ErrNotFound) // false. Using %w - preserves the original error... errors.Is(errW, ErrNotFound) // true."
Context: Guide on error wrapping in Go with %w vs %v.
Confidence: high

### 5.2 Retry with Backoff (Provider Rate Limits)

The `cenkalti/backoff` library is the de facto standard for exponential backoff in Go:

```go
operation := func() (string, error) {
    resp, err := http.Get("http://api.provider.com/resource")
    if err != nil {
        return "", err
    }
    if resp.StatusCode == 429 {
        seconds, _ := strconv.ParseInt(resp.Header.Get("Retry-After"), 10, 64)
        return "", backoff.RetryAfter(int(seconds))
    }
    if resp.StatusCode >= 500 {
        return "", errors.New("server error")
    }
    if resp.StatusCode == 400 {
        return "", backoff.Permanent(errors.New("bad request"))
    }
    return "success", nil
}

result, err := backoff.Retry(context.TODO(), operation,
    backoff.WithBackOff(backoff.NewExponentialBackOff()),
    backoff.WithMaxTries(5),
)
```

Claim: cenkalti/backoff v5 provides `Retry`, `RetryWithData`, `Permanent` errors (non-retryable), and `RetryAfter` errors (server-specified wait time) with exponential backoff and jitter[^10^].
Source: pkg.go.dev - cenkalti/backoff/v5
URL: https://pkg.go.dev/github.com/cenkalti/backoff/v5
Date: 2025-07-23
Excerpt: "Retry attempts the operation until success, a permanent error, or backoff completion... Permanent wraps the given err in a *PermanentError... RetryAfter returns a RetryAfter error that specifies how long to wait before retrying."
Context: Official package documentation for backoff v5.
Confidence: high

**AWS SDK v2 already has built-in retry middleware** with exponential jitter backoff. Adapter-level retry should complement this for cross-provider consistency:

Claim: AWS SDK v2 includes built-in retry middleware with `Standard` retry strategy (exponential jitter backoff), `AdaptiveMode` (with client attempt rate limiting), and configurable `MaxAttempts` and `MaxBackoffDelay`[^7^].
Source: pkg.go.dev - AWS SDK retry package
URL: https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/aws/retry
Date: Unknown
Excerpt: "AdaptiveMode provides an experimental retry strategy that expands on the Standard retry strategy, adding client attempt rate limits... Standard is the standard retry pattern for the SDK. It uses a set of retryable checks to determine of the failed attempt should be retried."
Context: Official AWS SDK for Go v2 retry package documentation.
Confidence: high

### 5.3 Circuit Breaker Pattern

Circuit breakers prevent cascading failures when a provider is systemically down. `sony/gobreaker` is the most popular Go implementation:

```go
st := gobreaker.Settings{
    Name:        "substrate-adapter",
    MaxRequests: 5,
    Interval:    0,
    Timeout:     10 * time.Second,
    ReadyToTrip: func(counts gobreaker.Counts) bool {
        return counts.ConsecutiveFailures > 3
    },
}
cb := gobreaker.NewCircuitBreaker(st)

result, err := cb.Execute(func() (interface{}, error) {
    return adapter.Create(ctx, spec)
})
```

Claim: sony/gobreaker is a mature, well-tested circuit breaker implementation for Go with configurable failure thresholds, half-open state, and timeout support; integrates well with Prometheus/OpenTelemetry for observability[^11^].
Source: OneUptime blog
URL: https://oneuptime.com/blog/post/2026-01-07-go-circuit-breaker/view
Date: 2026-01-07
Excerpt: "The sony/gobreaker library is a mature, well-tested implementation of the circuit breaker pattern for Go... errors.Is(err, gobreaker.ErrOpenState)... errors.Is(err, gobreaker.ErrTooManyRequests)."
Context: Guide on implementing circuit breakers in Go.
Confidence: high

**Layered resilience**: Combine retry for transient errors + circuit breaker for systemic failures:

```go
func resilientCall(cb *gobreaker.CircuitBreaker, fn func() error) error {
    for i := 0; i < 3; i++ {
        err := cb.Call(fn)
        if err == nil {
            return nil
        }
        time.Sleep(time.Duration(i+1) * 200 * time.Millisecond)
    }
    return errors.New("all retries failed")
}
```

Claim: Circuit breakers shine when combined with retry and backoff patterns, providing layered resilience: retry with exponential backoff for transient errors, circuit breaker for systemic failures[^25^].
Source: dev.to - serifcolakel
URL: https://dev.to/serifcolakel/circuit-breaker-patterns-in-go-microservices-n3
Date: 2025-11-09
Excerpt: "This gives you layered resilience: Retry with exponential backoff for transient errors; Circuit breaker for systemic failures."
Context: Article on circuit breaker patterns in Go microservices.
Confidence: high

### 5.4 Timeout Handling Per Verb

Different adapter verbs have different expected latencies. Use per-verb timeouts:

```go
type VerbTimeout struct {
    Create time.Duration
    Start  time.Duration
    Stop   time.Duration
    Delete time.Duration
    Get    time.Duration
    List   time.Duration
}

var DefaultTimeouts = VerbTimeout{
    Create: 5 * time.Minute,  // VM provisioning takes time
    Start:  30 * time.Second,
    Stop:   30 * time.Second,
    Delete: 2 * time.Minute,
    Get:    10 * time.Second,
    List:   10 * time.Second,
}

func (a *AWSAdapter) Create(ctx context.Context, spec ComputeSpec) (ResourceID, error) {
    ctx, cancel := context.WithTimeout(ctx, a.timeouts.Create)
    defer cancel()
    // ... perform creation
}
```

Claim: Go context timeouts should be set per operation with separate contexts for each operation, so one slow operation does not eat into the budget of the next; avoid timeouts smaller than 1 second[^16^].
Source: Uptrace blog
URL: https://uptrace.dev/blog/golang-context-timeout
Date: 2026-02-06
Excerpt: "You can use a separate context timeout for each operation so that one slow operation does not eat into the budget of the next... You should also avoid timeouts smaller than 1 second."
Context: Article on Go context timeout best practices.
Confidence: high

---

## 6. Validation Tooling

### 6.1 staticcheck, go vet, golangci-lint

`golangci-lint` is the recommended aggregator that combines multiple linters including `staticcheck`, `govet`, `errcheck`, and many others:

```yaml
# .golangci.yml
linters:
  enable:
    - staticcheck
    - gosec
    - errcheck
    - govet
    - ineffassign
run:
  timeout: 5m
  issues-exit-code: 1
```

Claim: golangci-lint aggregates multiple analysers (including gosec and staticcheck) into one consistent report; supports presets for bugs, error handling, performance, and style[^12^].
Source: Deployflow blog
URL: https://deployflow.co/blog/go-simplifies-pci-dss-psd2-compliance-code/
Date: 2025-11-05
Excerpt: "golangci-lint: Combining Security and Style Rules in One Pass... Aggregates multiple analysers (including gosec and staticcheck) into one consistent report."
Context: Article on Go security and compliance tooling.
Confidence: high

### 6.2 gosec for Security Scanning

`gosec` finds security anti-patterns: weak crypto, hardcoded secrets, unsafe file handling, SQL injection patterns:

```bash
gosec -exclude=G104,G306 -fmt=json -out=reports/gosec.json ./...
```

Claim: gosec detects security risks like weak encryption, hardcoded secrets, unsafe file handling, and SQL injection patterns; can be mapped to compliance requirements like PCI DSS 3.2[^26^].
Source: Deployflow blog
URL: https://deployflow.co/blog/go-simplifies-pci-dss-psd2-compliance-code/
Date: 2025-11-05
Excerpt: "gosec: Detecting Insecure Patterns and Weak Crypto... Detects security risks like weak encryption, hardcoded secrets, unsafe file handling, and SQL injection patterns."
Context: Article on Go security tooling for compliance.
Confidence: high

### 6.3 govulncheck for Vulnerability Scanning

`govulncheck` scans dependencies against the Go vulnerability database:

```bash
govulncheck ./...
```

Claim: govulncheck scans all dependencies for known vulnerabilities (CVEs), making it a perfect fit for ensuring third-party packages used in transaction flows stay patched and trusted[^27^].
Source: Medium - Jesse Corson
URL: https://medium.com/@jessecorson/golang-and-security-best-practices-4f6e2d96834e
Date: 2025-05-11
Excerpt: "govulncheck: Scans for known vulnerabilities in dependencies... govulncheck ./..."
Context: Article on Go security best practices.
Confidence: high

### 6.4 Which Tools to Run on Generated Code?

Generated code should be excluded from certain lint checks. golangci-lint supports generated file exclusion:

```yaml
# .golangci.yml
exclusions:
  generated: strict  # or lax
  rules:
    - path: _test\.go
      linters:
        - gocyclo
        - errcheck
        - dupl
        - gosec
```

Claim: golangci-lint supports `generated: strict` mode (excludes files matching `^// Code generated .* DO NOT EDIT\.$`) and `generated: lax` mode (broader matching including `autogenerated file`, `code generated`, `do not edit`)[^28^].
Source: golangci-lint docs
URL: https://golangci-lint.run/docs/configuration/file/
Date: 2026-04-29
Excerpt: "Mode of the generated files analysis... strict: sources are excluded by strictly following the Go generated file convention... lax: sources are excluded if they contain lines like autogenerated file, code generated, do not edit, etc."
Context: Official golangci-lint configuration documentation.
Confidence: high

For mock files specifically, naming them with `_test.go` suffix ensures Go automatically excludes them from coverage calculations:

Claim: Files with `_test` at the end of the name are ignored by default for Go code coverage calculations; this is the idiomatic approach for generated mock files[^14^].
Source: Stack Overflow
URL: https://stackoverflow.com/questions/50065448/how-to-ignore-generated-files-from-go-test-coverage
Date: 2020-02-12
Excerpt: "By using '_test' in the name definitions. If you would like to ignore files which are used in the tests then make sense use _test at the end of the name."
Context: Q&A on excluding generated files from Go test coverage.
Confidence: high

---

## 7. Contradictions and Conflict Zones

### 7.1 Interface Mocking vs Middleware Mocking for AWS SDK

The AWS SDK for Go v2 official documentation recommends mocking interfaces for testing. However, community practitioners find creating interfaces for every SDK client tedious and advocate for middleware-based mocking[^19^]. The middleware approach avoids interface proliferation but is less portable across SDK versions.

**Resolution**: For a multi-provider adapter generator, interface-based mocking is the correct abstraction because it generalizes across all providers (AWS, Azure, GCP, etc.). AWS-specific middleware mocking is useful only for AWS-specific internal tests.

### 7.2 Retry at SDK Level vs Adapter Level

AWS SDK v2 has built-in retry with exponential backoff. Adding adapter-level retry risks double-retry amplification. However, other providers may not have built-in retry, and a uniform adapter-level retry policy ensures consistency.

**Resolution**: The adapter should configure SDK-level retry where available (AWS, Azure SDKs have it) and implement a thin adapter-level retry only for providers lacking it. The adapter should expose retry configuration (max attempts, backoff) per provider.

### 7.3 Testcontainers vs Real Cloud for Integration Tests

LocalStack/testcontainers provide fast, deterministic, credential-free tests but don't perfectly emulate all cloud behaviors (especially IAM policies, some edge-case errors). Real cloud tests catch provider-specific quirks but are slower, cost money, and require credentials.

**Resolution**: Use a layered testing pyramid:
- Base: Unit tests with interface mocks (fast, no credentials)
- Middle: Testcontainers/LocalStack integration tests (medium speed, no credentials, PR-level)
- Top: Real cloud integration tests (slow, credentials required, nightly/weekly)

### 7.4 Hand-Written Fakes vs Generated Mocks

Go community is divided: some advocate hand-written fakes for readability and type safety; others prefer gomock for large interfaces. One author notes gomock has downsides including "maintenance cost to (re)generate mocks, reduced compile-time type safety due to interface{} arguments, poor IDE assistance, and potential negative impact on project code coverage"[^29^].

**Resolution**: For small interfaces (SubstrateAdapter has ~6 methods), hand-written fakes are preferable. For large provider SDK interfaces, use gomock with `_test.go` suffix to exclude from coverage.

---

## 8. Gaps in Available Information

1. **Provider SDK retry behavior**: Limited documentation on Azure SDK for Go and GCP SDK retry defaults; most research focused on AWS SDK v2.
2. **Testcontainers-go performance**: No benchmarks found on LocalStack container startup time in CI environments; empirical testing needed.
3. **Adapter-specific tracing patterns**: No published patterns specifically for "adapter layer" distributed tracing; general OTel patterns must be adapted.
4. **Rate limit behavior across providers**: Provider-specific rate limit headers and retry-after behavior not comprehensively documented in Go context.
5. **Cost tracking for generated code validation**: No data on CI execution time impact of running full golangci-lint + gosec + govulncheck on large generated adapter codebases.
6. **Multi-provider error taxonomy**: No standard error classification (transient vs permanent) across cloud providers; each SDK has different error types.

---

## 9. Preliminary Recommendations

| Recommendation | Confidence | Priority |
|----------------|-----------|----------|
| **R1**: Define `SubstrateAdapter` as a Go interface and generate hand-written fakes in `_test.go` files for unit testing | High | P0 |
| **R2**: Use `go-vcr` v3 for HTTP-level provider testing; commit cassettes to repo after redacting auth headers | High | P0 |
| **R3**: Adopt testcontainers-go + LocalStack for AWS integration tests; use container lifecycle in `TestMain` | High | P1 |
| **R4**: Use `slog` + `otelslog` bridge for structured logging with trace correlation per adapter | High | P1 |
| **R5**: Record OTel histogram metrics for `substrate.adapter.operation.duration` with `provider` and `operation` attributes | High | P1 |
| **R6**: Implement per-verb timeouts via `context.WithTimeout` in each adapter method, with configurable durations | High | P1 |
| **R7**: Wrap provider SDK errors with `fmt.Errorf("...: %w", err)` and define adapter-level sentinel errors (`ErrNotFound`, `ErrRateLimited`, etc.) | High | P0 |
| **R8**: Configure SDK-level retry where available; implement thin adapter-level retry with `cenkalti/backoff` only for providers lacking it | Medium | P1 |
| **R9**: Use `sony/gobreaker` circuit breaker at the adapter manager level (not per-adapter) to prevent cascading failures | Medium | P2 |
| **R10**: Run `golangci-lint` with `generated: strict` exclusion, `staticcheck`, `gosec`, `govulncheck` in CI; skip mocks via `_test.go` naming | High | P1 |
| **R11**: Limit real-cloud integration test parallelism to 3-4 concurrent tests to avoid rate limiting | Medium | P2 |
| **R12**: Use unique resource naming prefixes per CI run (`ci-<run-id>-<timestamp>`) to prevent cross-test collisions | High | P1 |
| **R13**: Create `TelemetryProvider` interface with `NoopTelemetry` implementation for tests | Medium | P2 |
| **R14**: Implement health checks via lightweight `List()` or `Describe()` calls per adapter, reporting Healthy/Degraded/Unhealthy | Medium | P2 |
| **R15**: Run unit + testcontainer tests on every PR; run real-cloud integration tests nightly with dedicated test accounts | High | P1 |

---

## Citations

[^1^]: Harrison Cramer, "Mocking Interfaces in Go", https://harrisoncramer.me/mocking-interfaces-in-go, 2024-08-18
[^2^]: Leapcell, "Mastering Mocking in Go gomock vs. Interface-Based Fakes", https://leapcell.io/blog/mastering-mocking-in-go-gomock-vs-interface-based-fakes, 2025-08-05
[^3^]: pkg.go.dev, "go-vcr module - gopkg.in/dnaeon/go-vcr.v3", https://pkg.go.dev/gopkg.in/dnaeon/go-vcr.v3, 2024-03-13
[^4^]: GitHub - dnaeon/go-vcr, "Record and replay your HTTP interactions for fast, deterministic and accurate tests", https://github.com/dnaeon/go-vcr, 2015-12-14
[^5^]: Testcontainers for Go, "LocalStack module", https://golang.testcontainers.org/modules/localstack/, Accessed 2025
[^6^]: Manuel de la Pena, "Delightful integration tests in Go applications" (CONF42 Golang 2025), https://www.conf42.com/Golang_2025_Manuel_de_la_Pena_testing_go_integration, 2025-04-03
[^7^]: pkg.go.dev, "retry package - github.com/aws/aws-sdk-go-v2/aws/retry", https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/aws/retry, Accessed 2025
[^8^]: Dash0, "OpenTelemetry-Native Logging in Go with the Slog Bridge", https://www.dash0.com/guides/opentelemetry-logging-in-go, 2026-04-24
[^9^]: OneUptime, "How to configure OpenTelemetry histogram buckets for latency tracking", https://oneuptime.com/blog/post/2026-02-09-otel-histogram-buckets-latency/view, 2026-02-09
[^10^]: pkg.go.dev, "backoff package - github.com/cenkalti/backoff/v5", https://pkg.go.dev/github.com/cenkalti/backoff/v5, 2025-07-23
[^11^]: OneUptime, "How to Implement Circuit Breakers in Go with sony/gobreaker", https://oneuptime.com/blog/post/2026-01-07-go-circuit-breaker/view, 2026-01-07
[^12^]: Deployflow, "How Go Automates PCI DSS & PSD2 Compliance in FinTech", https://deployflow.co/blog/go-simplifies-pci-dss-psd2-compliance-code/, 2025-11-05
[^13^]: IMTI, "AI on a Leash: Complete Go Project Configuration", https://imti.co/go-ai-verified-development/, 2026-02-09
[^14^]: Stack Overflow, "How to ignore generated files from Go test coverage", https://stackoverflow.com/questions/50065448/how-to-ignore-generated-files-from-go-test-coverage, 2020-02-12
[^15^]: EffectiveSoft, "Cloud Pricing Comparison 2026: AWS, Azure, GCP, Oracle", https://www.effectivesoft.com/blog/cloud-pricing-comparison.html, 2026-01-23
[^16^]: Uptrace, "Go Context timeouts can be harmful", https://uptrace.dev/blog/golang-context-timeout, 2026-02-06
[^17^]: Matteo Pampana, "How I Generate Mocks In Go Like A Boss", https://medium.com/@matteopampana/how-i-generate-mocks-in-go-like-a-boss-710663749f06, 2024-03-14
[^18^]: GitHub - seborama/govcr, "HTTP mock for Golang", https://github.com/seborama/govcr, 2025-04-19
[^19^]: dev.to - AWS Builders, "Testing with AWS SDK for Go V2 without interface mocks", https://dev.to/aws-builders/testing-with-aws-sdk-for-go-v2-without-interface-mocks-55de, 2023-08-09
[^20^]: dev.to - Calvin McLean, "Effortless HTTP Client Testing in Go", https://dev.to/calvinmclean/effortless-http-client-testing-in-go-4d75, 2024-07-11
[^21^]: OneUptime, "How to Use Parallel Testing for Terraform", https://oneuptime.com/blog/post/2026-02-23-how-to-use-parallel-testing-for-terraform/view, 2026-02-23
[^22^]: OneUptime, "How to Implement Health Checks in Go for Kubernetes", https://oneuptime.com/blog/post/2026-01-07-go-health-checks-kubernetes/view, 2026-01-07
[^23^]: dev.to - young_gao, "Distributed Tracing with OpenTelemetry in Go: A Practical Guide", https://dev.to/young_gao/distributed-tracing-with-opentelemetry-a-practical-guide-for-go-services-pep, 2026-03-21
[^24^]: OneUptime, "How to Wrap Errors with %w in Go", https://oneuptime.com/blog/post/2026-01-23-go-error-wrapping/view, 2026-01-23
[^25^]: dev.to - serifcolakel, "Circuit Breaker Patterns in Go Microservices", https://dev.to/serifcolakel/circuit-breaker-patterns-in-go-microservices-n3, 2025-11-09
[^26^]: Deployflow, "How Go Automates PCI DSS & PSD2 Compliance in FinTech", https://deployflow.co/blog/go-simplifies-pci-dss-psd2-compliance-code/, 2025-11-05
[^27^]: Jesse Corson, "Golang and Security Best Practices", https://medium.com/@jessecorson/golang-and-security-best-practices-4f6e2d96834e, 2025-05-11
[^28^]: golangci-lint docs, "Configuration File", https://golangci-lint.run/docs/configuration/file/, 2026-04-29
[^29^]: vearutop, "Mocking interfaces with typed functions in Go", https://dev.to/vearutop/mocking-interfaces-in-go-4nfn, 2020-12-05
[^30^]: pkg.go.dev, "backoff package - github.com/cenkalti/backoff/v4", https://pkg.go.dev/github.com/cenkalti/backoff/v4, 2024-01-02
