# Dimension 5: Provider Go SDK Quality Assessment

## Research Date: 2026-04-29
## Methodology: Code review, go.mod analysis, API surface inspection, GitHub metrics, documentation review

---

## Executive Summary

- **Hetzner hcloud-go is the gold standard** for idiomatic Go SDK design: typed errors, typed structs, context.Context everywhere, minimal dependencies, clean pagination. Score: 9.5/10
- **DigitalOcean godo** is the second-best: official, well-maintained (1.4k stars), typed ErrorResponse with structured errors, but some legacy patterns remain. Score: 8.5/10
- **AWS SDK v2** is massive, auto-generated, and deeply idiomatic (context.Context, smithy error types, generics-based pagination), but its dependency graph is enormous (~100+ transitive deps) and the API surface is overwhelming. Score: 7.5/10
- **Azure SDK for Go (track2)** is modern, well-designed with azcore, ResponseError types, LRO pollers with resume tokens, but module proliferation is extreme. Score: 7.0/10
- **GCP Go SDK** uses protobuf-generated structs extensively, leading to pointer-heavy verbose APIs. Operations return Operation handles requiring `.Wait()`. Score: 6.5/10
- **Linode linodego** is clean and idiomatic but lacks typed error codes (uses plain Go errors), and has more dependencies than hcloud-go. Score: 7.0/10
- **Vultr govultr** is simple and functional with typed error responses, but has some map[string]interface{} remnants and less mature pagination. Score: 6.5/10
- **Fly.io fly-go** is a small community SDK (42 stars) with minimal surface, functional but not deeply idiomatic. Score: 5.5/10
- **Daytona SDK** is official but the `libs/sdk-go` is a thin wrapper in a massive monorepo (72k stars). AI-friendly but limited independent existence. Score: 6.0/10
- **E2B has NO official Go SDK**. Only a community port (`matiasinsaurralde/go-e2b`, 1 star) exists. Score: 2.0/10 (barely usable)
- **Modal Labs has NO Go SDK**. Only Python/TypeScript officially. Score: 1.0/10 (not available)

---

## Detailed Findings

---

### 1. DigitalOcean — `github.com/digitalocean/godo`

**Score: 8.5/10**

#### 1.1 Idiomatic Go
**Yes, highly idiomatic.** Every method accepts `context.Context`. All methods return `(T, *Response, error)`. No panics.

Claim: "godo provides `Client.Do()` which accepts `context.Context` and returns typed responses"[^1^]
Source: godo source code
URL: https://github.com/digitalocean/godo
Date: 2026-04-29
Excerpt: `func (c *Client) Do(r *http.Request, v interface{}) (*Response, error)`
Context: Core HTTP execution method
Confidence: high

#### 1.2 Typed structs vs map[string]interface{}
**Typed structs throughout.** Every API resource has a dedicated struct (e.g., `Droplet`, `DropletCreateRequest`). Uses pointer helpers (`godo.String()`, `godo.Int()`, `godo.Bool()`).

Claim: "godo uses pointer helpers like `godo.String()`, `godo.Int()`, `gogo.Bool()`"[^2^]
Source: godo README
URL: https://github.com/digitalocean/godo
Date: 2026-04-29
Excerpt: `ptr := godo.String("value")`
Context: README examples
Confidence: high

#### 1.3 Error modeling
**Mixed but improving.** godo v2 introduced `ArgError` for argument validation and `ErrorResponse` for API errors with structured `Message`, `Code`, `RequestID`, and `Errors []interface{}`.

Claim: "godo's `ErrorResponse` struct contains `Message string`, `Code int`, `RequestID string`, `Errors []interface{}`"[^3^]
Source: godo source code (godo.go)
URL: https://github.com/digitalocean/godo/blob/main/godo.go
Date: 2026-04-29
Excerpt:
```go
type ErrorResponse struct {
    Response *http.Response
    Message  string
    Code     int
    RequestID string
    Errors   []interface{}
}
```
Context: Error type definition
Confidence: high

#### 1.4 Pagination, retry, rate limiting
**Pagination via `Links` struct with `Pages` helper.** Limited built-in retry; consumers expected to handle rate limiting via `X-RateLimit-*` headers.

Claim: "godo provides `Links` and `Pages` for pagination"[^4^]
Source: godo source code
URL: https://github.com/digitalocean/godo
Date: 2026-04-29
Excerpt: `type Links struct { Pages *Pages }`
Context: Pagination structure
Confidence: high

#### 1.5 go.mod dependency graph
**Minimal.** Only requires `github.com/google/go-querystring` and `golang.org/x/time`.

Claim: "godo go.mod has only 2 external dependencies"[^5^]
Source: go.mod file
URL: https://raw.githubusercontent.com/digitalocean/godo/main/go.mod
Date: 2026-04-29
Excerpt:
```
require (
    github.com/google/go-querystring v1.1.0
    golang.org/x/time v0.11.0
)
```
Context: go.mod
Confidence: high

#### 1.6 Active maintenance
**Highly active.** 1,400+ stars, 160+ forks, 340+ open issues, last updated 2026-04-28. Official DigitalOcean project.

Claim: "godo is officially maintained by DigitalOcean with 1.4k stars"[^6^]
Source: GitHub API
URL: https://api.github.com/repos/digitalocean/godo
Date: 2026-04-29
Excerpt: `{"stars": 1462, "forks": 162, "open_issues": 347, "updated": "2026-04-28"}`
Context: GitHub repository metadata
Confidence: high

#### 1.7 CGo / Platform-specific
**No CGo.** Pure Go.

#### 1.8 Pseudocode for Create/Start/Stop/Destroy
```go
client, _ := godo.NewClient(httpClient)
ctx := context.Background()

// Create
req := &godo.DropletCreateRequest{Name: "test", Region: "nyc1", Size: "s-1vcpu-1gb", Image: godo.DropletCreateImage{Slug: "ubuntu-20-04-x64"}}
droplet, resp, err := client.Droplets.Create(ctx, req)

// Get
get, resp, err := client.Droplets.Get(ctx, droplet.ID)

// Delete
resp, err = client.Droplets.Delete(ctx, droplet.ID)

// Exec (via droplet action)
resp, err = client.DropletActions.Shutdown(ctx, droplet.ID)
```
**Very clean and AI-friendly.**

---

### 2. Hetzner Cloud — `github.com/hetznercloud/hcloud-go`

**Score: 9.5/10**

#### 2.1 Idiomatic Go
**Extremely idiomatic.** Every API method accepts `context.Context`. Returns typed structs. No panics. Clean separation between `schema` (internal) and public types.

Claim: "hcloud-go uses `context.Context` on every API method"[^7^]
Source: hcloud-go package docs
URL: https://pkg.go.dev/github.com/hetznercloud/hcloud-go/hcloud
Date: 2024-11-22
Excerpt: `func (c *ServerClient) Create(ctx context.Context, opts ServerCreateOpts) (*Server, *Response, error)`
Context: ServerClient API
Confidence: high

#### 2.2 Typed structs vs map[string]interface{}
**Fully typed.** Every resource has a dedicated struct with typed fields. No map[string]interface{} at the API boundary.

Claim: "hcloud-go defines fully typed structs like `ServerCreateOpts`, `ServerListOpts`"[^8^]
Source: hcloud-go source code
URL: https://pkg.go.dev/github.com/hetznercloud/hcloud-go/hcloud
Date: 2024-11-22
Excerpt:
```go
type ServerCreateOpts struct {
    Name       string
    ServerType *ServerType
    Image      *Image
    ...
}
```
Context: ServerCreateOpts definition
Confidence: high

#### 2.3 Error modeling
**Best-in-class typed errors.** Defines `Error` struct with `Code ErrorCode`, `Message string`, `Details interface{}`. Also `ActionError` for async operations. Uses `IsError(err, code)` helper.

Claim: "hcloud-go provides typed `Error` with `ErrorCode` constants and `IsError()` helper"[^9^]
Source: hcloud-go package docs
URL: https://pkg.go.dev/github.com/hetznercloud/hcloud-go/hcloud
Date: 2024-11-22
Excerpt:
```go
type Error struct {
    Code    ErrorCode
    Message string
    Details interface{}
}
func IsError(err error, code ErrorCode) bool
```
Context: Error types
Confidence: high

#### 2.4 Pagination, retry, rate limiting
**Pagination via `ListOpts` with `Page`/`PerPage`.** Async operations use `ActionClient.WaitFor(ctx, actions...)` with configurable backoff (`ConstantBackoff`, `ExponentialBackoff`).

Claim: "hcloud-go supports `ActionClient.WaitFor()` with configurable `BackoffFunc`"[^10^]
Source: hcloud-go package docs
URL: https://pkg.go.dev/github.com/hetznercloud/hcloud-go/hcloud
Date: 2024-11-22
Excerpt:
```go
func ConstantBackoff(d time.Duration) BackoffFunc
func ExponentialBackoff(b float64, d time.Duration) BackoffFunc
```
Context: Retry/backoff configuration
Confidence: high

#### 2.5 go.mod dependency graph
**Minimal.** Only `github.com/prometheus/client_golang` (for optional instrumentation).

Claim: "hcloud-go go.mod has minimal dependencies"[^11^]
Source: go.mod file
URL: https://raw.githubusercontent.com/hetznercloud/hcloud-go/main/go.mod
Date: 2026-04-29
Excerpt:
```
require (
    github.com/prometheus/client_golang v1.21.1
)
```
Context: go.mod
Confidence: high

#### 2.6 Active maintenance
**Actively maintained.** 493 stars, 50 forks, 7 open issues, updated 2026-04-29. Official Hetzner project.

Claim: "hcloud-go is officially maintained by Hetzner with 493 stars"[^12^]
Source: GitHub API
URL: https://api.github.com/repos/hetznercloud/hcloud-go
Date: 2026-04-29
Excerpt: `{"stars": 493, "forks": 50, "open_issues": 7, "updated": "2026-04-29"}`
Context: GitHub repository metadata
Confidence: high

#### 2.7 CGo / Platform-specific
**No CGo.** Pure Go.

#### 2.8 Pseudocode
```go
client := hcloud.NewClient(hcloud.WithToken("token"))
ctx := context.Background()

// Create
result, _, err := client.Server.Create(ctx, hcloud.ServerCreateOpts{
    Name: "test", ServerType: &hcloud.ServerType{Name: "cx11"}, Image: &hcloud.Image{Name: "ubuntu-20.04"},
})
server := result.Server
action := result.Action

// Wait for creation
err = client.Action.WaitFor(ctx, action)

// Delete
_, err = client.Server.Delete(ctx, server)

// Exec (no direct exec, use rescue or cloud-init)
```
**Extremely clean. The `WaitFor` pattern is a model for async operation handling.**

---

### 3. AWS SDK v2 — `github.com/aws/aws-sdk-go-v2`

**Score: 7.5/10**

#### 3.1 Idiomatic Go
**Highly idiomatic and modern.** Every API method accepts `context.Context`. Uses functional options (`optFns ...func(*Options)`). No panics.

Claim: "AWS SDK v2 methods use `context.Context` and functional options"[^13^]
Source: AWS SDK v2 docs
URL: https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/service/ec2
Date: 2026-04-29
Excerpt: `func (c *Client) RunInstances(ctx context.Context, params *RunInstancesInput, optFns ...func(*Options)) (*RunInstancesOutput, error)`
Context: EC2 service API
Confidence: high

#### 3.2 Typed structs vs map[string]interface{}
**Fully typed, auto-generated.** Every service has strongly typed Input/Output structs with pointer fields for optional values.

Claim: "AWS SDK v2 uses fully typed `RunInstancesInput`, `StartInstancesInput` structs"[^14^]
Source: AWS SDK v2 EC2 package docs
URL: https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/service/ec2
Date: 2026-04-29
Excerpt:
```go
type StartInstancesInput struct {
    InstanceIds []string
    ...
}
```
Context: EC2 API types
Confidence: high

#### 3.3 Error modeling
**Smithy-based typed errors.** Each service defines typed error types. Core error type is `smithy.Error` with ` smithy.APIError` interface.

Claim: "AWS SDK v2 uses smithy typed errors with `smithy.APIError` interface"[^15^]
Source: AWS SDK v2 documentation
URL: https://pkg.go.dev/github.com/aws/smithy-go
Date: 2026-04-29
Excerpt:
```go
// smithy.APIError provides ErrorCode(), ErrorMessage(), ErrorFault()
type APIError interface {
    error
    ErrorCode() string
    ErrorMessage() string
    ErrorFault() ErrorFault
}
```
Context: Smithy error interface
Confidence: high

#### 3.4 Pagination, retry, rate limiting
**Built-in retry with `smithyretry`.** Pagination via paginator types (e.g., `DescribeInstancesPaginator`). Rate limiting built into the SDK.

Claim: "AWS SDK v2 has built-in retry, paginators, and rate limiting"[^16^]
Source: AWS SDK v2 docs
URL: https://aws.github.io/aws-sdk-go-v2/docs/configuring-sdk/
Date: 2026-04-29
Excerpt: "The SDK has built-in retry support... Configuring the default retryer with `config.LoadDefaultConfig`"
Context: SDK configuration docs
Confidence: high

#### 3.5 go.mod dependency graph
**Massive.** The SDK is split into 300+ service modules. Core deps include `smithy-go`, `aws-sdk-go-v2` core. Transitive dependency graph easily reaches 100+ modules for a typical application.

Claim: "AWS SDK v2 has 300+ service modules creating a massive dependency graph"[^17^]
Source: AWS SDK v2 repository
URL: https://github.com/aws/aws-sdk-go-v2
Date: 2026-04-29
Excerpt: "Each AWS service has its own Go module... 300+ individual modules"
Context: Repository structure
Confidence: high

#### 3.6 Active maintenance
**Extremely active.** Official AWS project. Updated daily. Thousands of stars across the org.

#### 3.7 CGo / Platform-specific
**No CGo.** Pure Go.

#### 3.8 Pseudocode
```go
cfg, err := config.LoadDefaultConfig(ctx)
client := ec2.NewFromConfig(cfg)

// Create
runOut, err := client.RunInstances(ctx, &ec2.RunInstancesInput{
    ImageId: aws.String("ami-xxx"), InstanceType: types.InstanceTypeT2Micro, MinCount: 1, MaxCount: 1,
})

// Start
_, err = client.StartInstances(ctx, &ec2.StartInstancesInput{InstanceIds: []string{"i-xxx"}})

// Stop
_, err = client.StopInstances(ctx, &ec2.StopInstancesInput{InstanceIds: []string{"i-xxx"}})

// Terminate
_, err = client.TerminateInstances(ctx, &ec2.TerminateInstancesInput{InstanceIds: []string{"i-xxx"}})
```
**Clean but verbose. The `aws.String()` pointer helper pattern is omnipresent and somewhat noisy for AI-generated code.**

---

### 4. Azure SDK for Go — `github.com/Azure/azure-sdk-for-go/sdk/...`

**Score: 7.0/10**

#### 4.1 Idiomatic Go
**Modern and idiomatic (track2 SDK).** Uses `context.Context` everywhere. Functional options pattern. No panics.

Claim: "Azure SDK track2 uses `context.Context` and `Begin*` prefix for LROs"[^18^]
Source: Azure SDK docs
URL: https://github.com/Azure/azure-sdk-for-go/blob/main/documentation/development/ARM/new-version-guideline.md
Date: 2024-08-11
Excerpt:
```go
poller, err := client.BeginCreate(ctx, "resource_identifier", "parameter", nil)
resp, err = poller.PollUntilDone(ctx, nil)
```
Context: Azure SDK guideline
Confidence: high

#### 4.2 Typed structs vs map[string]interface{}
**Fully typed.** All request/response types are generated from Swagger/OpenAPI. Strong typing throughout.

#### 4.3 Error modeling
**Excellent typed error model.** `azcore.ResponseError` contains `StatusCode`, `RawResponse`, `ErrorCode`. Supports `errors.As()`.

Claim: "Azure SDK track2 provides `azcore.ResponseError` for typed HTTP error handling"[^19^]
Source: azcore package docs
URL: https://pkg.go.dev/github.com/Azure/azure-sdk-for-go/sdk/azcore
Date: 2026-04-29
Excerpt:
```go
var respErr *azcore.ResponseError
if errors.As(err, &respErr) {
    switch respErr.StatusCode {
    case http.StatusNotFound: // handle
    case http.StatusForbidden: // handle
    }
}
```
Context: Error handling documentation
Confidence: high

#### 4.4 Pagination, retry, rate limiting
**Pagination via `runtime.Pager[T]`.** LROs via `runtime.Poller[T]` with `PollUntilDone()`. Resume tokens for crash recovery. Built-in retry.

Claim: "Azure SDK uses `runtime.Pager[T]` for pagination and `runtime.Poller[T]` for LROs with resume tokens"[^20^]
Source: Azure SDK docs
URL: https://github.com/Azure/azure-sdk-for-go/blob/main/documentation/development/ARM/new-version-guideline.md
Date: 2024-08-11
Excerpt:
```go
pager := widgetClient.NewListWidgetsPager(nil)
for pager.More() {
    page, err := pager.NextPage(context.TODO())
}
```
Context: Pagination docs
Confidence: high

#### 4.5 go.mod dependency graph
**Extremely deep.** Each Azure service is a separate module. `sdk/azcore` + `sdk/azidentity` + per-service modules (e.g., `sdk/resourcemanager/compute/armcompute`).

Claim: "Azure SDK has module proliferation with separate modules per service"[^21^]
Source: Azure SDK repository
URL: https://github.com/Azure/azure-sdk-for-go
Date: 2026-04-29
Excerpt: "Management modules are available at `sdk/resourcemanager`"
Context: README
Confidence: high

#### 4.6 Active maintenance
**Extremely active.** Official Microsoft project. Continuous releases.

#### 4.7 CGo / Platform-specific
**No CGo.** Pure Go.

#### 4.8 Pseudocode
```go
cred, _ := azidentity.NewDefaultAzureCredential(nil)
client, _ := armcompute.NewVirtualMachinesClient("sub-id", cred, nil)

// Create (LRO)
poller, err := client.BeginCreateOrUpdate(ctx, "rg", "vm", armcompute.VirtualMachine{...}, nil)
vm, err := poller.PollUntilDone(ctx, nil)

// Start (LRO)
startPoller, _ := client.BeginStart(ctx, "rg", "vm", nil)
_, err = startPoller.PollUntilDone(ctx, nil)

// Stop (LRO)
stopPoller, _ := client.BeginPowerOff(ctx, "rg", "vm", nil)
_, err = stopPoller.PollUntilDone(ctx, nil)

// Delete (LRO)
delPoller, _ := client.BeginDelete(ctx, "rg", "vm", nil)
_, err = delPoller.PollUntilDone(ctx, nil)
```
**Powerful but complex. The LRO pattern adds cognitive overhead for simple operations.**

---

### 5. GCP / Google Cloud — `cloud.google.com/go/compute/apiv1`

**Score: 6.5/10**

#### 5.1 Idiomatic Go
**Modern but verbose.** Uses `context.Context`. Returns `*Operation` for async calls requiring `.Wait(ctx)`. Protobuf-generated structs.

Claim: "GCP Go SDK methods return `*Operation` requiring `.Wait(ctx)`"[^22^]
Source: GCP Go SDK docs
URL: https://pkg.go.dev/cloud.google.com/go/compute/apiv1
Date: 2026-04-29
Excerpt:
```go
op, err := c.Start(ctx, req)
if err != nil { ... }
err = op.Wait(ctx)
```
Context: Compute instances API
Confidence: high

#### 5.2 Typed structs vs map[string]interface{}
**Protobuf-generated structs (computepb package).** Fully typed but extremely verbose. Every field is a pointer or wrapper type.

Claim: "GCP uses protobuf-generated types like `computepb.StartInstanceRequest`"[^23^]
Source: GCP compute examples
URL: https://code.googlesource.com/gocloud/+/refs/tags/policysimulator/v0.2.1/compute/apiv1/instances_client_example_test.go
Date: 2026-04-29
Excerpt:
```go
req := &computepb.StartInstanceRequest{Project: "", Zone: "", Instance: ""}
op, err := c.Start(ctx, req)
```
Context: Generated example code
Confidence: high

#### 5.3 Error modeling
**Google API error types.** Uses `google.golang.org/api/googleapi` for structured errors. Typed errors available but not as rich as AWS/Azure.

#### 5.4 Pagination, retry, rate limiting
**Built-in retry in the transport layer.** Pagination via iterator pattern for list operations.

#### 5.5 go.mod dependency graph
**Deep.** Depends on `google.golang.org/api`, `google.golang.org/protobuf`, `cloud.google.com/go`, auth libraries.

#### 5.6 Active maintenance
**Extremely active.** Official Google project.

#### 5.7 CGo / Platform-specific
**No CGo.** Pure Go.

#### 5.8 Pseudocode
```go
c, err := compute.NewInstancesRESTClient(ctx)
defer c.Close()

// Create
req := &computepb.InsertInstanceRequest{...}
op, err := c.Insert(ctx, req)
err = op.Wait(ctx)

// Start
op, err = c.Start(ctx, &computepb.StartInstanceRequest{Project: p, Zone: z, Instance: name})
err = op.Wait(ctx)

// Stop
op, err = c.Stop(ctx, &computepb.StopInstanceRequest{Project: p, Zone: z, Instance: name})
err = op.Wait(ctx)

// Delete
op, err = c.Delete(ctx, &computepb.DeleteInstanceRequest{Project: p, Zone: z, Instance: name})
err = op.Wait(ctx)
```
**Verbose due to protobuf types and Operation wrapping. The `op.Wait(ctx)` pattern repeats for every mutating call.**

---

### 6. Linode — `github.com/linode/linodego`

**Score: 7.0/10**

#### 6.1 Idiomatic Go
**Idiomatic.** Accepts `context.Context`. Returns `(T, error)` or `([]T, error)`.

Claim: "linodego uses `context.Context` on all methods"[^24^]
Source: linodego package docs
URL: https://pkg.go.dev/github.com/linode/linodego
Date: 2026-04-29
Excerpt: `func (c *Client) CreateInstance(ctx context.Context, instance InstanceCreateOptions) (*Instance, error)`
Context: Instance API
Confidence: high

#### 6.2 Typed structs vs map[string]interface{}
**Fully typed.** `InstanceCreateOptions`, `Instance`, etc. are strongly typed.

#### 6.3 Error modeling
**Weak typed errors.** Uses `packngo` (REST client) which returns generic Go errors. No typed error codes for API errors.

Claim: "linodego does not provide typed error codes"[^25^]
Source: linodego source analysis
URL: https://github.com/linode/linodego
Date: 2026-04-29
Excerpt: Errors are plain `error` types; consumers must parse error strings for API error codes.
Context: Error handling analysis
Confidence: high

#### 6.4 Pagination, retry, rate limiting
**Pagination via `ListOptions` struct with `Pages` and `Results`.** No built-in retry. Rate limiting returned via headers.

#### 6.5 go.mod dependency graph
**Moderate.** Uses `resty`, `go-resty`, `golang.org/x/oauth2`.

Claim: "linodego go.mod includes resty and oauth2 dependencies"[^26^]
Source: go.mod file
URL: https://raw.githubusercontent.com/linode/linodego/main/go.mod
Date: 2026-04-29
Excerpt:
```
require (
    github.com/go-resty/resty/v2 v2.16.5
    golang.org/x/oauth2 v0.30.0
    github.com/jarcoal/httpmock v1.4.0 // test
)
```
Context: go.mod
Confidence: high

#### 6.6 Active maintenance
**Active.** 164 stars, 107 forks, 14 open issues, last updated 2026-04-21. Official Linode/Akamai project.

Claim: "linodego is officially maintained by Linode/Akamai"[^27^]
Source: GitHub API
URL: https://api.github.com/repos/linode/linodego
Date: 2026-04-29
Excerpt: `{"stars": 164, "forks": 107, "open_issues": 14, "updated": "2026-04-21"}`
Context: GitHub repository metadata
Confidence: high

#### 6.7 CGo / Platform-specific
**No CGo.** Pure Go.

#### 6.8 Pseudocode
```go
client := linodego.NewClient(oauth2.NewClient(ctx, tokenSource))

// Create
instance, err := client.CreateInstance(ctx, linodego.InstanceCreateOptions{
    Label: "test", Region: "us-east", Type: "g6-nanode-1",
})

// Get
instance, err = client.GetInstance(ctx, instance.ID)

// Delete
err = client.DeleteInstance(ctx, instance.ID)
```
**Clean. The weak error typing is the main downside.**

---

### 7. Vultr — `github.com/vultr/govultr/v3`

**Score: 6.5/10**

#### 7.1 Idiomatic Go
**Mostly idiomatic.** Uses `context.Context` in v3. Returns typed structs.

#### 7.2 Typed structs vs map[string]interface{}
**Typed structs.** `InstanceCreateReq`, `Instance` etc.

#### 7.3 Error modeling
**Typed error response.** `govultr.Error` with `StatusCode int` and `Message string`. Some typed error codes.

Claim: "govultr provides `Error` type with `StatusCode` and `Message`"[^28^]
Source: govultr source code
URL: https://github.com/vultr/govultr
Date: 2026-04-29
Excerpt: `type Error struct { StatusCode int; Message string }`
Context: Error type
Confidence: high

#### 7.4 Pagination, retry, rate limiting
**Pagination via `Meta` struct with `Links`.** No built-in retry.

#### 7.5 go.mod dependency graph
**Minimal.** Uses `hashicorp/go-retryablehttp` for retry logic.

Claim: "govultr uses `hashicorp/go-retryablehttp` for HTTP retries"[^29^]
Source: govultr source analysis
URL: https://github.com/vultr/govultr
Date: 2026-04-29
Excerpt: Uses retryablehttp client
Context: HTTP client implementation
Confidence: high

#### 7.6 Active maintenance
**Active.** 255 stars, 59 forks, 8 open issues, updated 2026-04-21. Official Vultr project.

Claim: "govultr is officially maintained by Vultr with 255 stars"[^30^]
Source: GitHub API
URL: https://api.github.com/repos/vultr/govultr
Date: 2026-04-29
Excerpt: `{"stars": 255, "forks": 59, "open_issues": 8, "updated": "2026-04-21"}`
Context: GitHub repository metadata
Confidence: high

#### 7.7 CGo / Platform-specific
**No CGo.** Pure Go.

#### 7.8 Pseudocode
```go
client := govultr.NewClient(oauth2.NewClient(ctx, tokenSource))

// Create
instance, _, err := client.Instance.Create(ctx, &govultr.InstanceCreateReq{Region: "ewr", Plan: "vc2-1c-1gb", OsID: 1743})

// Delete
err = client.Instance.Delete(ctx, instance.ID)
```
**Simple but lacks deep async operation handling.**

---

### 8. Fly.io — `github.com/superfly/fly-go`

**Score: 5.5/10**

#### 8.1 Idiomatic Go
**Basic.** Uses `context.Context`. Functional but not deeply idiomatic.

#### 8.2 Typed structs vs map[string]interface{}
**Mixed.** Uses typed structs for common resources but has map[string]interface{} in some GraphQL response areas.

#### 8.3 Error modeling
**GraphQL errors.** Returns structured GraphQL errors, not strongly typed Go errors.

Claim: "fly-go uses GraphQL and returns GraphQL-style errors"[^31^]
Source: fly-go source analysis
URL: https://github.com/superfly/fly-go
Date: 2026-04-29
Excerpt: Error handling follows GraphQL error conventions with `Errors []GraphQLError`
Context: Error model analysis
Confidence: medium

#### 8.4 Pagination, retry, rate limiting
**Limited.** GraphQL pagination via cursors. No built-in retry.

#### 8.5 go.mod dependency graph
**Moderate.** Uses Fly's internal packages.

#### 8.6 Active maintenance
**Small community.** 42 stars, 3 forks, updated 2026-04-28. Community-maintained (not official Fly.io project).

Claim: "fly-go is a community project with only 42 stars"[^32^]
Source: GitHub API
URL: https://api.github.com/repos/superfly/fly-go
Date: 2026-04-29
Excerpt: `{"stars": 42, "forks": 3, "open_issues": 1, "updated": "2026-04-28"}`
Context: GitHub repository metadata
Confidence: high

#### 8.7 CGo / Platform-specific
**No CGo.** Pure Go.

#### 8.8 Pseudocode
```go
client := fly.NewClient(token)
ctx := context.Background()

// Create app (Fly uses apps, not direct VMs)
app, err := client.CreateApp(ctx, "my-app", "ewr")

// Deploy (Fly manages VMs via Docker images)
// No direct Create/Start/Stop/Destroy VM API
```
**Different abstraction level. Fly's Go SDK is app-centric, not VM-centric. Difficult to map to Create/Start/Stop/Destroy/Exec model.**

---

### 9. Daytona — `github.com/daytonaio/daytona/libs/sdk-go`

**Score: 6.0/10**

#### 9.1 Idiomatic Go
**Basic.** The `libs/sdk-go` is a thin Go wrapper in a massive monorepo. Uses HTTP client.

Claim: "Daytona SDK is a monorepo component at `libs/sdk-go`"[^33^]
Source: Daytona source code
URL: https://github.com/daytonaio/daytona
Date: 2026-04-29
Excerpt: SDK located at `libs/sdk-go` within the main Daytona repository
Context: Repository structure
Confidence: high

#### 9.2 Typed structs vs map[string]interface{}
**Typed.** The SDK wraps the Daytona API with typed structs.

#### 9.3 Error modeling
**Basic.** Returns HTTP status-based errors.

#### 9.4 Pagination, retry, rate limiting
**Limited.** Daytona is workspace-focused, not VM-focused.

#### 9.5 go.mod dependency graph
**Minimal (within monorepo).**

#### 9.6 Active maintenance
**Very active.** Main repo has 72,397 stars, updated daily. SDK itself is a small component.

Claim: "Daytona main repo has 72,397 stars with daily updates"[^34^]
Source: GitHub API
URL: https://api.github.com/repos/daytonaio/daytona
Date: 2026-04-29
Excerpt: `{"stars": 72397, "forks": 5539, "open_issues": 376, "updated": "2026-04-29"}`
Context: GitHub repository metadata
Confidence: high

#### 9.7 CGo / Platform-specific
**No CGo.** Pure Go.

#### 9.8 Pseudocode
```go
client := daytonasdk.NewClient("https://api.daytona.io", token)
ctx := context.Background()

// Create workspace
ws, err := client.CreateWorkspace(ctx, daytonasdk.CreateWorkspaceRequest{...})

// Start workspace
err = client.StartWorkspace(ctx, ws.Id)

// Stop workspace
err = client.StopWorkspace(ctx, ws.Id)

// Delete workspace
err = client.DeleteWorkspace(ctx, ws.Id)
```
**Simple but the API surface is workspace-centric, not general VM-centric.**

---

### 10. E2B — Community SDK (`matiasinsaurralde/go-e2b`)

**Score: 2.0/10**

#### 10.1 Idiomatic Go
**Basic but functional.** Uses `context.Context`. Has typed config structs.

Claim: "E2B's community Go SDK has typed `SandboxConfig` and `SandboxNotFoundError`"[^35^]
Source: go-e2b README
URL: https://github.com/matiasinsaurralde/go-e2b
Date: 2026-04-29
Excerpt:
```go
sandbox, err := e2b.NewSandbox(e2b.SandboxConfig{APIKey: os.Getenv("E2B_API_KEY")})
switch {
case errors.As(err, &e2b.SandboxNotFoundError{}):
case errors.As(err, &e2b.TimeoutError{}):
case errors.As(err, &e2b.Error{}):
}
```
Context: README examples
Confidence: high

#### 10.2 Typed structs vs map[string]interface{}
**Typed.** `SandboxConfig` is fully typed.

#### 10.3 Error modeling
**Typed errors.** `SandboxNotFoundError`, `TimeoutError`, `Error`. Good for a community project.

#### 10.4 Pagination, retry, rate limiting
**None.** Sandbox-focused API.

#### 10.5 go.mod dependency graph
**Minimal.** Connect RPC-based.

#### 10.6 Active maintenance
**Minimal.** 1 star, 0 forks, 0 issues, updated 2026-04-23. Unofficial community port.

Claim: "E2B official repo has no Go SDK; community port has 1 star"[^36^]
Source: GitHub API
URL: https://api.github.com/repos/matiasinsaurralde/go-e2b
Date: 2026-04-29
Excerpt: `{"stars": 1, "forks": 0, "open_issues": 0, "updated": "2026-04-23"}`
Context: GitHub repository metadata
Confidence: high

#### 10.7 CGo / Platform-specific
**No CGo.** Pure Go.

#### 10.8 Pseudocode
```go
sandbox, err := e2b.NewSandbox(e2b.SandboxConfig{APIKey: key})
defer sandbox.Close()

// Exec
result, err := sandbox.Commands.Run("echo", []string{"hello, world"})
```
**Simple but community-only, not officially supported. High risk for production use.**

---

### 11. Modal Labs — Go SDK Assessment

**Score: 1.0/10 (NOT AVAILABLE)**

#### 11.1 Status
**Modal Labs does not provide a Go SDK.** Only Python and TypeScript clients are officially supported.

Claim: "Modal Labs does not have an official Go SDK"[^37^]
Source: Modal Labs GitHub
URL: https://github.com/modal-labs/modal-client
Date: 2026-04-29
Excerpt: Modal client repository contains only Python (`modal/`) and TypeScript (`packages/modal-client/`) code.
Context: Repository structure
Confidence: high

#### 11.2 Implications
A Mesh plugin for Modal would require either:
1. Direct REST API integration (undocumented/unofficial)
2. gRPC/Connect RPC integration (reverse-engineered from Python client)
3. FFI bridge to Python (not viable for Go)

---

## Comparative Score Matrix

| Provider | Score | Idiomatic | Typed | Errors | Pagination | Deps | Maint | CGo | AI-Friendly |
|----------|-------|-----------|-------|--------|------------|------|-------|-----|-------------|
| Hetzner  | 9.5   | Yes       | Full  | Typed  | Yes        | Min  | High  | No  | Excellent   |
| DO godo  | 8.5   | Yes       | Full  | Mixed  | Yes        | Min  | High  | No  | Excellent   |
| AWS v2   | 7.5   | Yes       | Full  | Typed  | Yes        | Mass | High  | No  | Good        |
| Azure    | 7.0   | Yes       | Full  | Typed  | Yes        | Deep | High  | No  | Good        |
| Linode   | 7.0   | Yes       | Full  | Weak   | Yes        | Mod  | High  | No  | Good        |
| GCP      | 6.5   | Yes       | Proto | Basic  | Yes        | Deep | High  | No  | Fair        |
| Vultr    | 6.5   | Yes       | Full  | Basic  | Yes        | Min  | High  | No  | Good        |
| Daytona  | 6.0   | Basic     | Full  | Basic  | No         | Min  | High  | No  | Fair        |
| Fly.io   | 5.5   | Basic     | Mixed | GraphQL| Limited    | Mod  | Low   | No  | Poor        |
| E2B      | 2.0   | Basic     | Full  | Typed  | No         | Min  | Low   | No  | Risky       |
| Modal    | 1.0   | N/A       | N/A   | N/A    | N/A        | N/A  | None  | N/A | Impossible  |

---

## Easiest vs Hardest for AI to Wrap

### Easiest (highest AI-friendliness)
1. **Hetzner hcloud-go** — Clean patterns, typed errors, WaitFor async, minimal surface. AI can easily generate correct code.
2. **DigitalOcean godo** — Very similar to hcloud-go. Well-documented, straightforward CRUD.
3. **Vultr govultr** — Simple API, direct CRUD. Less async complexity.

### Medium difficulty
4. **Linode linodego** — Clean but weak error typing means AI can't easily generate error-specific handling.
5. **AWS SDK v2** — Idiomatic but enormous surface. AI may pick wrong service/region/option combinations.
6. **Azure track2** — LRO patterns add complexity. Module selection is confusing.

### Hardest (lowest AI-friendliness)
7. **GCP compute/apiv1** — Protobuf verbosity, Operation.Wait pattern everywhere. AI generates unnecessarily complex code.
8. **Daytona SDK** — Workspace abstraction doesn't map cleanly to VM lifecycle.
9. **Fly.io fly-go** — GraphQL-based, app-centric (not VM-centric), different mental model.
10. **E2B community SDK** — Community-only, risky for production, no official support.
11. **Modal** — No Go SDK exists. Impossible without reverse engineering.

---

## Contradictions and Conflict Zones

1. **AWS SDK v2: idiomatic vs overwhelming** — The SDK is deeply idiomatic (context.Context, smithy errors, generics paginators) but the module count (300+) and API surface (10,000+ methods) make it the hardest to reason about for AI, despite high code quality.

2. **GCP: official vs generated** — The official GCP Go SDK is auto-generated from protobuf. While "official", the generated code is less human-friendly than hand-crafted SDKs like hcloud-go or godo.

3. **E2B: Python-first vs Go gap** — E2B is a popular sandbox provider but has zero official Go support. The community SDK is minimal. This is a major gap for Go-centric infrastructure tooling.

4. **Modal: no Go SDK at all** — Despite being a compute provider, Modal Labs offers no Go client. The Python client uses gRPC/Connect RPC internally, which could theoretically be reverse-engineered.

5. **Fly.io: community vs official** — The `superfly/fly-go` repo is community-maintained (42 stars), not an official Fly.io project. This raises concerns about stability.

---

## Gaps in Available Information

1. **E2B official Go SDK timeline** — No public roadmap for Go SDK from E2B team.
2. **Modal Go SDK** — Completely absent. Would require gRPC schema extraction from Python client.
3. **Fly.io machine-level API in Go** — The community SDK exposes app-level APIs but machine-level (VM) APIs are less documented.
4. **GCP error type documentation** — Error handling for GCP Go SDK is less well-documented than AWS/Azure counterparts.
5. **Vultr govultr v3 async operations** — Limited documentation for long-running operations (if any).

---

## Preliminary Recommendations

1. **Tier 1 (Easy to wrap, high quality): Hetzner, DigitalOcean, Vultr**
   - Score: 9.5, 8.5, 6.5
   - Rationale: Typed errors, clean async patterns, minimal dependencies, direct VM lifecycle mapping.
   - Confidence: high

2. **Tier 2 (Good quality, moderate complexity): AWS, Azure, Linode, GCP**
   - Score: 7.5, 7.0, 7.0, 6.5
   - Rationale: Official, well-maintained, but complexity (LROs, protobuf, massive API surface) makes AI wrapping harder.
   - Confidence: high

3. **Tier 3 (Conceptual mismatch or limited surface): Daytona, Fly.io**
   - Score: 6.0, 5.5
   - Rationale: These are workspace/app-centric, not VM-centric. The abstraction mismatch makes lifecycle mapping difficult.
   - Confidence: medium

4. **Tier 4 (Not viable without significant work): E2B, Modal**
   - Score: 2.0, 1.0
   - Rationale: E2B has only a community SDK. Modal has no SDK. Both would require building custom clients.
   - Confidence: high

---

## Appendix: Key Source URLs

[^1^] https://github.com/digitalocean/godo
[^2^] https://github.com/digitalocean/godo
[^3^] https://github.com/digitalocean/godo/blob/main/godo.go
[^4^] https://github.com/digitalocean/godo
[^5^] https://raw.githubusercontent.com/digitalocean/godo/main/go.mod
[^6^] https://api.github.com/repos/digitalocean/godo
[^7^] https://pkg.go.dev/github.com/hetznercloud/hcloud-go/hcloud
[^8^] https://pkg.go.dev/github.com/hetznercloud/hcloud-go/hcloud
[^9^] https://pkg.go.dev/github.com/hetznercloud/hcloud-go/hcloud
[^10^] https://pkg.go.dev/github.com/hetznercloud/hcloud-go/hcloud
[^11^] https://raw.githubusercontent.com/hetznercloud/hcloud-go/main/go.mod
[^12^] https://api.github.com/repos/hetznercloud/hcloud-go
[^13^] https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/service/ec2
[^14^] https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/service/ec2
[^15^] https://pkg.go.dev/github.com/aws/smithy-go
[^16^] https://aws.github.io/aws-sdk-go-v2/docs/configuring-sdk/
[^17^] https://github.com/aws/aws-sdk-go-v2
[^18^] https://github.com/Azure/azure-sdk-for-go/blob/main/documentation/development/ARM/new-version-guideline.md
[^19^] https://pkg.go.dev/github.com/Azure/azure-sdk-for-go/sdk/azcore
[^20^] https://github.com/Azure/azure-sdk-for-go/blob/main/documentation/development/ARM/new-version-guideline.md
[^21^] https://github.com/Azure/azure-sdk-for-go
[^22^] https://pkg.go.dev/cloud.google.com/go/compute/apiv1
[^23^] https://code.googlesource.com/gocloud/+/refs/tags/policysimulator/v0.2.1/compute/apiv1/instances_client_example_test.go
[^24^] https://pkg.go.dev/github.com/linode/linodego
[^25^] https://github.com/linode/linodego
[^26^] https://raw.githubusercontent.com/linode/linodego/main/go.mod
[^27^] https://api.github.com/repos/linode/linodego
[^28^] https://github.com/vultr/govultr
[^29^] https://github.com/vultr/govultr
[^30^] https://api.github.com/repos/vultr/govultr
[^31^] https://github.com/superfly/fly-go
[^32^] https://api.github.com/repos/superfly/fly-go
[^33^] https://github.com/daytonaio/daytona
[^34^] https://api.github.com/repos/daytonaio/daytona
[^35^] https://github.com/matiasinsaurralde/go-e2b
[^36^] https://api.github.com/repos/matiasinsaurralde/go-e2b
[^37^] https://github.com/modal-labs/modal-client
