## 3. SDK Quality Scores

### 3.1 Scoring Methodology

Each provider Go SDK was scored on a 1–10 composite derived from six axes: idiomatic Go (`context.Context`, no panics), typed structs (no `map[string]interface{}` at the boundary), error modeling (typed codes with `errors.As()` support), pagination and async primitives, dependency graph depth, and active maintenance. A 9.5 does not mean "best engineered in absolute terms"; it means "best engineered *and* easiest for an AI agent to wrap into a `SubstrateAdapter` without hallucination." Conversely, AWS SDK v2 is deeply idiomatic yet scores 7.5 because its 300+ modules and 10,000+ method surface overwhelm generation context windows [^17^]. Confidence for all scores is **high** based on source-code inspection, `go.mod` analysis, and package documentation review.

### 3.2 Tier 1: Easiest for AI (8.0–9.5/10)

#### 3.2.1 Hetzner hcloud-go (9.5)

`github.com/hetznercloud/hcloud-go` is the reference. Every method accepts `context.Context` [^7^], every resource has a dedicated struct (`ServerCreateOpts`, `ServerListOpts`) [^8^], and errors are fully typed via `Error` with `Code ErrorCode` and the `IsError(err, code)` helper [^9^]. Async operations use `ActionClient.WaitFor(ctx, actions…)` with configurable `ConstantBackoff` or `ExponentialBackoff` [^10^]. The `go.mod` lists a single external dependency [^11^], and the repository is actively maintained by Hetzner with 493 stars and only 7 open issues [^12^].

**Steal this:** The `WaitFor` pattern. When a mutating call returns `*Action`, a single `client.Action.WaitFor(ctx, action)` blocks with backoff until completion. This is the model Mesh adapters should emulate for any provider with long-running operations.

```go
client := hcloud.NewClient(hcloud.WithToken(token))
ctx := context.Background()

result, _, err := client.Server.Create(ctx, hcloud.ServerCreateOpts{
    Name: "mesh-node", ServerType: &hcloud.ServerType{Name: "cx11"},
    Image: &hcloud.Image{Name: "ubuntu-22.04"},
})
server := result.Server
err = client.Action.WaitFor(ctx, result.Action) // blocks until ready

startAction, _, err := client.Server.Poweron(ctx, server)
err = client.Action.WaitFor(ctx, startAction)

stopAction, _, err := client.Server.Shutdown(ctx, server)
err = client.Action.WaitFor(ctx, stopAction)

_, err = client.Server.Delete(ctx, server)
```

#### 3.2.2 DigitalOcean godo (8.5)

`github.com/digitalocean/godo` mirrors Hetzner with slightly more legacy weight. Methods return `(T, *Response, error)` [^1^], structs are fully typed with documented pointer helpers (`godo.String`, `godo.Int`) [^2^], and pagination uses a `Links` struct with `Pages` [^4^]. Error modeling is mixed: `ErrorResponse` carries typed `Message`, `Code`, and `RequestID`, but the `Errors` field is `[]interface{}`, forcing consumers into type assertions [^3^]. Dependencies are minimal—only `github.com/google/go-querystring` and `golang.org/x/time` [^5^]. Maintenance is strong: 1,462 stars, 162 forks, and active daily updates [^6^].

```go
client, _ := godo.NewClient(httpClient)
ctx := context.Background()

droplet, _, err := client.Droplets.Create(ctx, &godo.DropletCreateRequest{
    Name: "mesh-node", Region: "nyc1", Size: "s-1vcpu-1gb",
    Image: godo.DropletCreateImage{Slug: "ubuntu-22-04-x64"},
})

_, err = client.DropletActions.Shutdown(ctx, droplet.ID) // Stop
_, err = client.DropletActions.PowerOn(ctx, droplet.ID)  // Start
_, err = client.Droplets.Delete(ctx, droplet.ID)          // Destroy
```

### 3.3 Tier 2: Medium Complexity (6.5–7.5/10)

#### 3.3.1 AWS SDK v2 (7.5)

`github.com/aws/aws-sdk-go-v2` is the most idiomatic enterprise SDK: `context.Context`, functional options, Smithy-based typed errors (`smithy.APIError` with `ErrorCode()` / `ErrorMessage()` / `ErrorFault()`) [^13^][^15^], and built-in paginators and retry [^16^]. The flaw is scale: 300+ modules [^17^] and 10,000+ methods. For an AI agent, selecting the correct service module and option permutation is a documented failure mode—SWE-bench Go resolve rates sit near 30% on comparable tasks.

```go
cfg, _ := config.LoadDefaultConfig(ctx)
client := ec2.NewFromConfig(cfg)

runOut, _ := client.RunInstances(ctx, &ec2.RunInstancesInput{
    ImageId: aws.String("ami-xxx"), InstanceType: types.InstanceTypeT2Micro,
    MinCount: 1, MaxCount: 1,
})

_, err = client.StartInstances(ctx, &ec2.StartInstancesInput{InstanceIds: []string{"i-xxx"}})
_, err = client.StopInstances(ctx, &ec2.StopInstancesInput{InstanceIds: []string{"i-xxx"}})
_, err = client.TerminateInstances(ctx, &ec2.TerminateInstancesInput{InstanceIds: []string{"i-xxx"}})
// Exec: use SSM SendCommand or user-data scripts
```

#### 3.3.2 Azure track2 (7.0)

`github.com/Azure/azure-sdk-for-go/sdk/…` uses `runtime.Pager[T]` for pagination and `runtime.Poller[T]` with `PollUntilDone()` for LROs [^18^][^20^]. `azcore.ResponseError` exposes `StatusCode` and `ErrorCode`, compatible with `errors.As()` [^19^]. The cost is module proliferation: each service is a separate module [^21^], and the LRO pattern (`BeginCreateOrUpdate` → poller → `PollUntilDone`) adds boilerplate that simpler SDKs hide.

```go
cred, _ := azidentity.NewDefaultAzureCredential(nil)
client, _ := armcompute.NewVirtualMachinesClient("sub-id", cred, nil)

poller, _ := client.BeginCreateOrUpdate(ctx, "rg", "vm", armcompute.VirtualMachine{…}, nil)
vm, _ := poller.PollUntilDone(ctx, nil)

startPoller, _ := client.BeginStart(ctx, "rg", "vm", nil)
_, _ = startPoller.PollUntilDone(ctx, nil)

stopPoller, _ := client.BeginPowerOff(ctx, "rg", "vm", nil)
_, _ = stopPoller.PollUntilDone(ctx, nil)

delPoller, _ := client.BeginDelete(ctx, "rg", "vm", nil)
_, _ = delPoller.PollUntilDone(ctx, nil)
// Exec: use RunCommandExtensions or SSH after provisioning
```

#### 3.3.3 Linode linodego (7.0)

`github.com/linode/linodego` is clean and idiomatic (`context.Context`, `InstanceCreateOptions`) [^24^], but its error modeling is weak: the `packngo` REST client returns plain Go errors; consumers must parse strings to extract API codes [^25^]. This is a material risk for AI-generated adapters because the agent cannot generate `errors.As` switches and must instead fall back to fragile string matching. Dependencies include `go-resty/resty/v2` and `golang.org/x/oauth2` [^26^]. Officially maintained by Akamai with 164 stars [^27^].

#### 3.3.4 GCP compute (6.5)

`cloud.google.com/go/compute/apiv1` uses protobuf-generated structs (`computepb.InsertInstanceRequest`), making the API fully typed but verbose [^23^]. Every mutating call returns `*Operation` requiring `op.Wait(ctx)` [^22^]. The dependency graph is deep, and error types under `google.golang.org/api/googleapi` are less structured than AWS Smithy or Azure `azcore` errors. The protobuf pointer-heavy style and mandatory `op.Wait(ctx)` repetition increase token consumption and hallucination probability.

#### 3.3.5 Vultr govultr (6.5)

`github.com/vultr/govultr/v3` is simple and functional: `context.Context`, typed structs, typed `Error` with `StatusCode` and `Message` [^28^], and `hashicorp/go-retryablehttp` for built-in retry [^29^]. Officially maintained by Vultr with 255 stars [^30^]. The score is capped at 6.5 because pagination helpers are less mature than Hetzner's and async operation support is limited.

### 3.4 Tier 3: Harder or Beta (5.5–6.5/10)

#### 3.4.1 Modal beta Go SDK (6.5)

Modal launched an official beta Go SDK in October 2025 (`github.com/modal-labs/modal-client/go`). It supports Sandboxes, Functions, Images, and Volumes, but the surface is narrower than the Python client. The beta status means APIs may shift between minor versions, and several Python-only filesystem primitives are not yet exposed. For AI wrapping, the recommendation is to pin to a specific beta version and regenerate the adapter on each SDK release.

#### 3.4.2 Daytona SDK (6.0)

`github.com/daytonaio/daytona/libs/sdk-go` is an official SDK nested inside a 72,397-star monorepo [^33^][^34^]. It is typed and functional, but the surface is workspace-centric (`CreateWorkspace`, `StartWorkspace`) rather than VM-centric. The abstraction mismatch means concepts like "instance type" are opaque, and pagination is limited. Viable for Mesh, but the agent must understand Daytona's workspace model.

#### 3.4.3 Fly.io fly-go (5.5)

`github.com/superfly/fly-go` is a community project (42 stars, 3 forks), not official [^32^]. It uses GraphQL internally, so errors arrive as `[]GraphQLError` [^31^]. The SDK is app-centric (`CreateApp`), and direct machine lifecycle is less exposed. For Mesh, this creates a mental-model mismatch—`Create` maps to `CreateApp + Deploy`, and `Exec` requires SSH fallbacks. The low star count and unofficial status raise maintenance-risk flags. **Confidence: medium**.

### 3.5 Tier 4: Not Viable (<3.0/10)

#### 3.5.1 E2B: no official Go SDK

E2B maintains Python and JavaScript clients officially. The only Go option is a community port (`github.com/matiasinsaurralde/go-e2b`) with 1 star and 0 forks [^36^]. The port exposes typed structs and typed errors (`SandboxNotFoundError`, `TimeoutError`) [^35^], but it is unofficial, receives no security patches, and tracks an API that may diverge without notice. For production Mesh adapters, this is a supply-chain liability. **Avoid this** unless Mesh itself forks and maintains the Go client.

### 3.6 Key Gotchas by Provider

**AWS Smithy middleware.** All errors travel through the Smithy pipeline as `smithy.OperationError` wrappers. An AI agent must unwrap with `errors.As(err, new(*smithy.OperationError))` before asserting to service-specific errors. Agents that treat AWS errors like Hetzner errors generate non-compiling code.

**GCP `Operation.Wait()`.** Every mutating call returns `*Operation`; the resource is not ready until `op.Wait(ctx)` completes. Agents frequently omit the wait, leading to race conditions where subsequent `Get` or `Exec` runs against a still-provisioning instance.

**Azure LRO pollers.** Azure track2 prefixes mutating methods with `Begin`: `BeginCreateOrUpdate`, `BeginStart`, `BeginPowerOff`, `BeginDelete`. Each returns `(Poller[T], error)`, and the agent must call `PollUntilDone(ctx, nil)`. Forgetting the `Begin` prefix or the poller dance produces compile failures or hung goroutines.

**Linode string errors.** Because `linodego` returns plain errors, an AI cannot generate robust `errors.As` handling. The adapter must use string-match fallbacks, which are brittle across API versions.

**Fly.io app-centric model.** There is no `CreateInstance`. The agent must chain `CreateApp` → `CreateMachine` → `WaitForMachine`. This three-step mapping exceeds the token budget of most single-shot generation prompts.

The matrix below consolidates all eleven providers.

| Provider | Score | Tier | Idiomatic | Typed | Errors | Pagination | Deps | Maint | AI-Friendly |
|----------|-------|------|-----------|-------|--------|------------|------|-------|-------------|
| Hetzner | 9.5 | 1 | Yes [^7^] | Full [^8^] | Typed [^9^] | Yes [^10^] | Minimal [^11^] | High [^12^] | Excellent |
| DO godo | 8.5 | 1 | Yes [^1^] | Full [^2^] | Mixed [^3^] | Yes [^4^] | Minimal [^5^] | High [^6^] | Excellent |
| AWS v2 | 7.5 | 2 | Yes [^13^] | Full [^14^] | Typed [^15^] | Yes [^16^] | Massive [^17^] | High | Good |
| Azure | 7.0 | 2 | Yes [^18^] | Full | Typed [^19^] | Yes [^20^] | Deep [^21^] | High | Good |
| Linode | 7.0 | 2 | Yes [^24^] | Full | Weak [^25^] | Yes | Mod [^26^] | High [^27^] | Good |
| GCP | 6.5 | 2 | Yes [^22^] | Proto [^23^] | Basic | Yes | Deep | High | Fair |
| Vultr | 6.5 | 2 | Yes | Full | Basic [^28^] | Yes | Minimal [^29^] | High [^30^] | Good |
| Modal | 6.5 | 3 | Yes | Full | Basic | No | Minimal | Beta | Fair |
| Daytona | 6.0 | 3 | Basic [^33^] | Full | Basic | No | Minimal | High [^34^] | Fair |
| Fly.io | 5.5 | 3 | Basic | Mixed [^31^] | GraphQL | Limited | Mod | Low [^32^] | Poor |
| E2B | 2.0 | 4 | Basic [^35^] | Full | Typed | No | Minimal | Low [^36^] | Risky |

The outlier pattern is the **enterprise penalty**: AWS and Azure score higher on raw engineering quality than Vultr or Daytona, yet they are harder for AI agents to wrap because abstraction depth increases faster than interface clarity. For Mesh's generation pipeline, this implies a counterintuitive roadmap: validate the skill template on Tier 1 providers, then escalate to Tier 2 only after the agent demonstrates reliable handling of `WaitFor`, `Poller`, and Smithy patterns.
