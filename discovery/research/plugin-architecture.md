# Research: Plugin & Provider Architecture

> Completed: April 23, 2026
> Source: Pulumi Neo/Agent Skills docs, Terraform provider architecture, HashiCorp go-plugin, Pulumi provider architecture, Nomad task drivers, Docker CLI plugins

## Pulumi AI / Skill Analysis

### How Pulumi AI Code Generation Works

**Pulumi Neo** is an AI agent built to execute, govern, and optimize cloud automation. Unlike generic AI tools, Neo understands infrastructure dependencies, respects policies, and works within existing Pulumi governance frameworks.

**Core capabilities:**
- Natural language automation: "Create an AWS Lambda function that processes S3 events"
- End-to-end execution: Understands dependencies, executes changes, monitors outcomes
- Human-in-the-loop controls: Configurable approval workflows
- Complete audit trail: Every action is previewed, logged, and reversible

**Integration points:**
- Available in Pulumi Cloud, VS Code, Cursor, Claude Code, and Windsurf through **MCP server**
- Pulumi Agent Skills are reusable knowledge packages following the `agentskills.io` standard
- Works with: Claude Code, OpenAI Codex, Cursor, GitHub Copilot, Google Gemini, JetBrains Junie

### Pulumi Agent Skills

**Skills are organized into two categories:**

**Migration Skills** (convert from other tools to Pulumi):
- `pulumi-terraform-to-pulumi` - Migrate Terraform projects
- `cloudformation-to-pulumi` - Migrate AWS CloudFormation
- `pulumi-cdk-to-pulumi` - Convert AWS CDK applications
- `pulumi-arm-to-pulumi` - Convert Azure ARM templates and Bicep

**Authoring Skills** (generate Pulumi code):
- ComponentResource patterns
- Infrastructure best practices
- Multi-cloud workflow guidance
- Stack reference generation

**Installation (Claude Code):**
```bash
claude plugin marketplace add pulumi/agent-skills
claude plugin install pulumi-migration
claude plugin install pulumi-authoring
```

**Installation (General):**
```bash
npx skills add pulumi/agent-skills --skill '*'
```

### Pulumi Automation API (Programmatic Interface)

Pulumi Automation API exposes the full power of infrastructure as code through a programmatic interface, enabling:

- **Strongly-typed SDK**: Use Pulumi engine as a library in your application
- **Custom cloud interfaces**: Build REST, gRPC, or Custom Resource APIs
- **Developer portals**: Self-serve platforms for infrastructure teams
- **Orchestration**: Multi-stack deployments, dependency tracking, incremental updates
- **Tooling**: CLIs, higher-level frameworks, CI/CD workflows, desktop apps

**Key operations:**
```typescript
// Create/select stack
const stack = await workspace.createStack("dev");

// Install plugins
await stack.workspace.installPlugin("aws", "v5.0.0");

// Update stack
const result = await stack.up({ onOutput: console.log });

// Get outputs
const url = result.outputs.websiteUrl.value;
```

### Code Generation Capabilities

**Languages supported:**
- TypeScript
- Python
- Go
- C#
- Java
- Any language with gRPC support

**Cloud providers supported:**
- AWS
- Azure
- Google Cloud
- DigitalOcean
- Hetzner
- Any cloud with a Terraform or Pulumi provider

**Code generation patterns:**
- Correct import paths (`@pulumi/*` packages)
- ComponentResource pattern with `registerOutputs()`
- `pulumi.interpolate` for string templates with `Output<T>`
- `pulumi.Config` for configuration values
- Stack references for cross-stack dependencies
- Provider-specific knowledge (accurate for major clouds)

### Limitations

1. **Requires Pulumi Cloud or self-hosted backend**: State management requires a Pulumi backend (can be self-hosted with open-source Pulumi)
2. **AI hallucinations**: Like all LLMs, can generate incorrect code, especially for obscure cloud resources
3. **Context window limits**: Large infrastructure stacks may exceed context capacity
4. **Requires cloud credentials**: User must provide AWS/Azure/GCP credentials
5. **No direct plugin generation**: Generates Pulumi programs, not provider/plugin code itself
6. **Dependent on provider availability**: Can only generate code for providers that exist in Terraform or Pulumi ecosystems

### Can Pulumi AI Generate Mesh Provider Plugins?

**Directly? No.** Pulumi AI generates infrastructure-as-code (Pulumi programs), not plugin code.

**However, it can help generate infrastructure provisioning code:**
1. Generate Pulumi programs to provision DigitalOcean droplets, AWS EC2 instances, etc.
2. Generate SDK calls to cloud provider APIs
3. Generate Terraform HCL that can be converted to Pulumi

**For Mesh plugin generation:**
- Mesh skill would need to use Pulumi AI as a sub-component
- Mesh skill provides the plugin interface/template
- Pulumi AI generates the cloud API interaction code
- Mesh skill wraps the generated code in the plugin interface

**Workflow:**
```
User: "Generate a DigitalOcean provider for Mesh"
  ↓
Mesh Skill: Extracts DigitalOcean API requirements
  ↓
Mesh Skill: Asks Pulumi AI: "Generate Pulumi code to create/destroy/manage DO droplets"
  ↓
Pulumi AI: Generates TypeScript/Python/Go code using @pulumi/digitalocean
  ↓
Mesh Skill: Wraps generated code in SubstrateAdapter interface
  ↓
Mesh Skill: Outputs complete plugin code
```

## Existing Plugin Architecture Patterns

### 1. Terraform Providers

**Architecture:**
- gRPC-based plugin system
- Each provider is a separate OS process
- Protocol versioning (v5, v6) for backward compatibility
- Schema-based resource definitions

**Discovery and Loading:**
```go
// Provider discovery
type GRPCProviderPlugin struct {
    plugin.Plugin
    GRPCProvider func() proto.ProviderServer
}

// Provider loading
pluginClient := plugin.NewClient(&plugin.ClientConfig{
    Cmd:              exec.Command("terraform-provider-aws"),
    HandshakeConfig:  plugin.HandshakeConfig{
        ProtocolVersion:  5,
        MagicCookieKey:   "TERRAFORM_PLUGIN_MAGIC_COOKIE",
        MagicCookieValue: "d602bf8394d30e894b74718db058527d84e30998bf3f25a442e4575fad419932",
    },
    AllowedProtocols: []plugin.Protocol{plugin.ProtocolGRPC},
})
```

**Interface Contract:**
```protobuf
// Core provider interface
service Provider {
    rpc GetSchema(GetProviderSchemaRequest) returns (GetProviderSchemaResponse);
    rpc PrepareProviderConfig(PrepareProviderConfigRequest) returns (PrepareProviderConfigResponse);
    rpc ValidateProviderConfig(ValidateProviderConfigRequest) returns (ValidateProviderConfigResponse);
    rpc Configure(ConfigureRequest) returns (ConfigureResponse);
    rpc Stop(StopRequest) returns (StopResponse);
    rpc PlanResourceChange(PlanResourceChangeRequest) returns (PlanResourceChangeResponse);
    rpc ApplyResourceChange(ApplyResourceChangeRequest) returns (ApplyResourceChangeResponse);
    rpc ReadResource(ReadResourceRequest) returns (ReadResourceResponse);
    rpc DeleteResource(DeleteResourceRequest) returns (DeleteResourceResponse);
}
```

**Communication Protocol:**
- **gRPC over Unix domain sockets or TCP**
- **Multiplexing**: HTTP/2 handles multiple streams over single connection
- **Protocol handshake**: Line-based handshake on stdout:
  ```
  5|1|unix|/tmp/plugin12345|grpc
  ```
  Format: `CORE-PROTOCOL-VERSION|APP-PROTOCOL-VERSION|NETWORK-TYPE|NETWORK-ADDR|PROTOCOL`

**Plugin Capabilities Declaration:**
- Provider schema contains resource and data source definitions
- Each resource defines its schema (inputs, outputs, computed values)
- Schema validation happens before API calls
- Diff operations performed at schema level

**Error Handling:**
- gRPC status codes for errors
- Detailed error messages in response payloads
- Provider can signal retryable vs. fatal errors
- Validation errors returned before state changes

**Plugin Lifecycle:**
```
1. Discovery → Find provider binary in PATH or local directory
2. Launch → Start provider subprocess
3. Handshake → Protocol version negotiation
4. Schema Load → Fetch provider schema (cached)
5. Configure → Pass provider configuration (API keys, regions)
6. Plan/Apply → Execute resource operations
7. Stop → Graceful shutdown
8. Cleanup → Kill process if needed
```

### 2. HashiCorp go-plugin

**Used by:** Terraform, Nomad, Vault, Boundary, Waypoint

**Architecture:**
- RPC-based plugin system (net/rpc or gRPC)
- Subprocess isolation (plugin crash doesn't crash host)
- Cross-language support via gRPC
- Binary handshake protocol

**Key Features:**
```go
type Client struct {
    // AllowedProtocols restricts which protocols can be used
    AllowedProtocols []Protocol
    
    // GRPCDialOptions for custom gRPC configuration
    GRPCDialOptions []grpc.DialOption
    
    // GRPCBrokerMultiplex for multiplexed gRPC streams
    GRPCBrokerMultiplex bool
}

type Protocol string

const (
    ProtocolNetRPC Protocol = "netrpc"
    ProtocolGRPC   Protocol = "grpc"
)
```

**Interface Definition:**
```go
// Plugin interface
type Plugin interface {
    // Server returns the RPC server implementation
    Server(*MuxBroker) (interface{}, error)
    
    // Client returns the RPC client implementation
    Client(*MuxBroker, *RPCClient) (interface{}, error)
}

// GRPCPlugin interface for gRPC-based plugins
type GRPCPlugin interface {
    // GRPCServer returns a gRPC server implementation
    GRPCServer(*GRPCBroker) *grpc.Server
    
    // GRPCClient returns a gRPC client implementation
    GRPCClient(context.Context, *GRPCBroker, *grpc.ClientConn) (interface{}, error)
}
```

**Handshake Protocol:**
1. Plugin writes handshake line to stdout
2. Format: `CORE-PROTOCOL-VERSION|APP-PROTOCOL-VERSION|NETWORK-TYPE|NETWORK-ADDR|PROTOCOL`
3. Example: `1|5|unix|/tmp/plugin12345|grpc`
4. Host reads handshake, establishes connection

**Plugin Registration:**
```go
func main() {
    plugin.Serve(&plugin.ServeConfig{
        HandshakeConfig: plugin.HandshakeConfig{
            ProtocolVersion:  1,
            MagicCookieKey:   "BASIC_PLUGIN",
            MagicCookieValue: "hello",
        },
        Plugins: map[string]plugin.Plugin{
            "kv": &KVPlugin{},
        },
        GRPCServer: plugin.DefaultGRPCServer,
    })
}
```

### 3. Pulumi Providers

**Architecture:**
- Layered architecture (protocol → bindings → SDK)
- gRPC-based provider protocol
- Two implementation paths: bridged (from Terraform) or native

**Provider Types:**

**A. Bridged Providers (Pulumi Terraform Bridge)**
- Wraps existing Terraform providers
- Majority of Pulumi providers (4000+ Terraform providers available)
- No code generation required for new providers

**Bridge Workflow:**
```
1. Build-time:
   - Inspect Terraform provider schema
   - Generate Pulumi schema/code
   - Generate TypeScript types

2. Runtime:
   - Pulumi engine calls bridged provider
   - Bridge translates to Terraform provider calls
   - Terraform provider makes API calls
   - Bridge translates responses back to Pulumi
```

**B. Native Providers (pulumi-go-provider)**
- Written in Go using pulumi-go-provider SDK
- Schema inference from Go structs and tags
- Automatic multi-language SDK generation
- Built-in testing framework

**Interface Contract (Go SDK):**
```go
type Provider interface {
    // Name returns the provider name
    Name() string
    
    // Version returns the provider version
    Version() string
    
    // Configure configures the provider with user inputs
    Configure(context.Context, ConfigureRequest) (ConfigureResponse, error)
    
    // Schema returns the provider schema
    Schema(context.Context, SchemaRequest) (SchemaResponse, error)
    
    // Resources returns the resources provided
    Resources() []Resource
}

type Resource interface {
    // Name returns the resource name
    Name() string
    
    // Schema returns the resource schema
    Schema(context.Context, SchemaRequest) (SchemaResponse, error)
    
    // Create creates a new resource
    Create(context.Context, CreateRequest) (CreateResponse, error)
    
    // Read reads the current state
    Read(context.Context, ReadRequest) (ReadResponse, error)
    
    // Update updates an existing resource
    Update(context.Context, UpdateRequest) (UpdateResponse, error)
    
    // Delete deletes a resource
    Delete(context.Context, DeleteRequest) (DeleteResponse, error)
}
```

**Discovery and Loading:**
- Providers are binaries named `pulumi-resource-<name>`
- Discovered via:
  - `PULUMI_HOME` directory
  - Language-specific package managers (npm, pip, go get)
  - `pulumi plugin install <name> <version>`
- Loaded dynamically at runtime

**Protocol:**
- gRPC-based provider protocol
- Protocol Buffers define the interface
- Language bindings generated from `.proto` files
- Any language with gRPC support can implement a provider

### 4. Nomad Task Drivers

**Architecture:**
- Pluggable task execution system
- Uses HashiCorp go-plugin
- Long-lived processes (not bound to Nomad client lifecycle)
- Reattach support for crash recovery

**Interface Contract:**
```go
// BasePlugin interface
type BasePlugin interface {
    // PluginInfo returns plugin metadata
    PluginInfo() *base.PluginInfo
    
    // SetConfig sets the plugin configuration
    SetConfig(*base.Config) error
}

// DriverPlugin interface
type DriverPlugin interface {
    BasePlugin
    
    // TaskConfigSchema returns the task configuration schema
    TaskConfigSchema() (*hclspec.Spec, error)
    
    // TaskSchema returns the task schema
    TaskSchema() (*hclspec.Spec, error)
    
    // Capabilities returns driver capabilities
    Capabilities() *DriverCapabilities
    
    // Fingerprint returns driver fingerprint
    Fingerprint(*DriverContext) (bool, error)
    
    // Prestart prepares for task start
    Prestart(*DriverContext, *TaskConfig) (*PrestartResponse, error)
    
    // Start starts a task
    Start(*DriverContext, *TaskConfig) (*TaskHandle, error)
    
    // Wait waits for task to complete
    Wait(*DriverContext, *TaskConfig) (*TaskStatus, error)
    
    // Stop stops a task
    Stop(*DriverContext, *TaskConfig) error
    
    // Destroy destroys a task
    Destroy(*DriverContext, *TaskConfig) error
    
    // Inspect inspects task state
    Inspect(*DriverContext, *TaskConfig) (*TaskStatus, error)
    
    // Exec executes a command in task
    Exec(*DriverContext, *TaskConfig, ExecRequest) (*ExecResponse, error)
    
    // Signal sends a signal to task
    Signal(*DriverContext, *TaskConfig, string) error
}
```

**Discovery and Loading:**
- Configured in `client.hcl`:
  ```hcl
  plugin_dir = "/opt/nomad/plugins"
  ```
- Plugin binary: `nomad-driver-<name>`
- Auto-discovered on startup
- Versioned plugin interface

**State Management:**
- TaskHandle contains reattachment state
- DriverState allows custom state encoding
- Nomad manages task lifecycle
- Plugin restart doesn't kill tasks

**Error Handling:**
- Plugin crashes → Nomad restarts plugin
- Task crashes → Plugin reports status
- Reattach support via DriverState
- Graceful degradation on plugin failure

### 5. Docker CLI Plugins

**Architecture:**
- Extension architecture (not just CLI)
- Three components: frontend, backend, executables
- Hook mechanism for contextual actions
- Distributed via Docker Hub as images

**Plugin Components:**
```json
{
  "name": "my-extension",
  "version": "0.1.0",
  "description": "My Docker extension",
  "icon": "icon.png",
  "vm": {
    "image": "my-extension/backend:latest",
    "cpus": 2,
    "memory": 2048
  },
  "ui": {
    "dashboard-tab": {
      "title": "My Extension",
      "root": "/ui",
      "src": "index.html"
    }
  }
}
```

**Hook Mechanism:**
```json
{
  "auths": {},
  "plugins": {
    "hints": {
      "hooks": "pull,build"
    }
  }
}
```

- Plugins declare which commands they hook into
- Only configured plugins are invoked (performance optimization)
- Templating approach for output
- Plugin receives command context

**Discovery and Loading:**
- Scanned from CLI plugin directory
- Configured in `~/.docker/config.json`
- Can be installed via Marketplace or CLI
- Versioned metadata

**Communication:**
- Frontend ↔ Backend: Socket-based HTTP
- Frontend ↔ Executables: SDK invocation
- No direct process communication

### Comparison Summary

| Aspect | Terraform | go-plugin | Pulumi | Nomad | Docker |
|--------|-----------|-----------|--------|-------|--------|
| **Protocol** | gRPC | net/rpc or gRPC | gRPC | go-plugin (gRPC) | HTTP/Socket |
| **Language** | Go (multi-lang via gRPC) | Go (multi-lang via gRPC) | Go, TS, Python, C#, Java | Go | Any |
| **Isolation** | Subprocess | Subprocess | Subprocess | Subprocess | Container |
| **Discovery** | Binary in PATH | Binary in PATH | PULUMI_HOME or pkg manager | plugin_dir | config.json |
| **Handshake** | Line-based stdout | Line-based stdout | gRPC handshake | go-plugin handshake | Metadata file |
| **Versioning** | Protocol v5/v6 | Protocol version | Provider version | Driver API version | Extension version |
| **State** | Terraform state | Plugin-managed | Pulumi state | DriverState | Extension-managed |
| **Error Handling** | gRPC status | RPC error | gRPC error | Plugin restart | Exception handling |
| **Reattach** | No | No | No | Yes (DriverState) | N/A |

## Proposed Mesh Plugin Interface

### Design Principles

Based on research and constraints:

1. **Use gRPC for protocol** - Industry standard, cross-language support, proven at scale
2. **Subprocess isolation** - Plugin crash doesn't crash Mesh core
3. **Capability declaration** - Plugins declare what they support (optional features)
4. **HashiCorp go-plugin for lifecycle** - Proven, battle-tested, handles handshake, restart, cleanup
5. **Go for plugin SDK** - Compiles to standalone binary, no runtime dependencies
6. **Multi-language plugin support** - Users can write plugins in Python, TypeScript, Rust, etc.

### Plugin Interface Definition

**Protocol Buffers Definition (`mesh_plugin.proto`):**

```protobuf
syntax = "proto3";

package mesh.v1;

// Core plugin interface
service SubstrateAdapter {
    // Plugin metadata
    rpc GetPluginInfo(Empty) returns (PluginInfo);
    rpc GetCapabilities(Empty) returns (Capabilities);
    
    // Core lifecycle (required)
    rpc Create(CreateRequest) returns (CreateResponse);
    rpc Start(StartRequest) returns (Empty);
    rpc Stop(StopRequest) returns (Empty);
    rpc Destroy(DestroyRequest) returns (Empty);
    
    // State inspection (required)
    rpc GetStatus(StatusRequest) returns (StatusResponse);
    rpc GetLogs(LogsRequest) returns (stream LogEntry);
    
    // Command execution (required)
    rpc Exec(ExecRequest) returns (ExecResponse);
    
    // Filesystem operations (optional)
    rpc ReadFile(ReadFileRequest) returns (ReadFileResponse);
    rpc WriteFile(WriteFileRequest) returns (Empty);
    rpc CopyFiles(CopyFilesRequest) returns (Empty);
    
    // Snapshot/export (optional)
    rpc ExportFilesystem(ExportRequest) returns (stream FileChunk);
    rpc ImportFilesystem(ImportRequest) returns (CreateResponse);
    rpc CreateSnapshot(SnapshotRequest) returns (SnapshotResponse);
    rpc RestoreFromSnapshot(RestoreRequest) returns (CreateResponse);
    
    // Suspend/resume (optional)
    rpc Suspend(SuspendRequest) returns (Empty);
    rpc Resume(ResumeRequest) returns (Empty);
    
    // Resource queries (optional)
    rpc GetAllocatedResources(ResourceRequest) returns (Resources);
    rpc GetAvailableResources(Empty) returns (Resources);
}

// Types
message PluginInfo {
    string name = 1;
    string version = 2;
    string description = 3;
}

message Capabilities {
    // Required capabilities
    bool create = 1;
    bool start = 2;
    bool stop = 3;
    bool destroy = 4;
    bool get_status = 5;
    bool exec = 6;
    
    // Optional capabilities
    bool read_file = 10;
    bool write_file = 11;
    bool copy_files = 12;
    
    bool export_filesystem = 20;
    bool import_filesystem = 21;
    bool create_snapshot = 22;
    bool restore_from_snapshot = 23;
    
    bool suspend = 30;
    bool resume = 31;
    
    // Snapshot type
    enum SnapshotType {
        NONE = 0;
        FILESYSTEM_ONLY = 1;
        MEMORY_AND_FILESYSTEM = 2;
        READ_ONLY_TEMPLATE = 3;
    }
    SnapshotType snapshot_type = 40;
    
    // Resource limits
    Resources min_resources = 50;
    Resources max_resources = 51;
    
    // Platform characteristics
    bool persistent_disk = 60;
    bool portable_snapshots = 61;
    bool memory_snapshots = 62;
    bool auto_resume = 63;
    bool scale_to_zero = 64;
    
    // GPU support
    bool gpu = 70;
    repeated string gpu_types = 71;
}

message Resources {
    double cpu = 1;        // vCPUs
    int64 memory = 2;     // MiB
    int64 disk = 3;       // GiB
    GPU gpu = 4;
}

message GPU {
    string type = 1;
    int32 count = 2;
}

message NetworkConfig {
    enum Mode {
        BRIDGE = 0;
        HOST = 1;
        NONE = 2;
        PRIVATE = 3;
    }
    Mode mode = 1;
    repeated PortMapping ports = 2;
    repeated string dns = 3;
}

message PortMapping {
    int32 container = 1;
    int32 host = 2;
    string protocol = 3; // "tcp" or "udp"
}

message CreateRequest {
    string image = 1;
    Resources resources = 2;
    NetworkConfig network = 3;
    map<string, string> env = 4;
    map<string, string> labels = 5;
}

message CreateResponse {
    string instance_id = 1;
    map<string, string> metadata = 2;
}

message StartRequest {
    string instance_id = 1;
}

message StopRequest {
    string instance_id = 1;
}

message DestroyRequest {
    string instance_id = 1;
}

message StatusRequest {
    string instance_id = 1;
}

message StatusResponse {
    enum State {
        CREATED = 0;
        STARTING = 1;
        RUNNING = 2;
        PAUSED = 3;
        STOPPED = 4;
        SUSPENDED = 5;
        TERMINATING = 6;
        TERMINATED = 7;
        ERROR = 8;
    }
    State state = 1;
    string message = 2;
    Resources allocated_resources = 3;
    int64 created_at = 4;
}

message LogsRequest {
    string instance_id = 1;
    bool follow = 2;
    int64 since = 3;  // Unix timestamp
}

message LogEntry {
    string line = 1;
    int64 timestamp = 2;
    string stream = 3; // "stdout" or "stderr"
}

message ExecRequest {
    string instance_id = 1;
    repeated string command = 2;
    map<string, string> env = 3;
}

message ExecResponse {
    int32 exit_code = 1;
    string stdout = 2;
    string stderr = 3;
}

message ReadFileRequest {
    string instance_id = 1;
    string path = 2;
}

message ReadFileResponse {
    bytes data = 1;
}

message WriteFileRequest {
    string instance_id = 1;
    string path = 2;
    bytes data = 3;
}

message CopyFilesRequest {
    string instance_id = 1;
    string src = 2;
    string dst = 3;
}

message ExportRequest {
    string instance_id = 1;
    enum Format {
        TAR = 0;
        TAR_ZSTD = 1;
    }
    Format format = 2;
}

message FileChunk {
    bytes data = 1;
    bool done = 2;
}

message ImportRequest {
    string image = 1;
    Format format = 2;
    // Stream data via client streaming
}

message SnapshotRequest {
    string instance_id = 1;
    string name = 2;
}

message SnapshotResponse {
    string snapshot_id = 1;
    int64 size_bytes = 2;
    int64 created_at = 3;
}

message RestoreRequest {
    string snapshot_id = 1;
    Resources resources = 2;
}

message SuspendRequest {
    string instance_id = 1;
}

message ResumeRequest {
    string instance_id = 1;
}

message ResourceRequest {
    string instance_id = 1;
}

message Empty {}
```

### Go Plugin SDK

**Plugin interface (Go):**

```go
package meshplugin

import (
    context "context"
    "github.com/hashicorp/go-plugin"
)

// SubstrateAdapter is the interface all substrate plugins must implement
type SubstrateAdapter interface {
    // PluginInfo returns plugin metadata
    PluginInfo() *PluginInfo
    
    // GetCapabilities returns plugin capabilities
    GetCapabilities() *Capabilities
    
    // Core lifecycle (required)
    Create(ctx context.Context, req *CreateRequest) (*CreateResponse, error)
    Start(ctx context.Context, req *StartRequest) error
    Stop(ctx context.Context, req *StopRequest) error
    Destroy(ctx context.Context, req *DestroyRequest) error
    
    // State inspection (required)
    GetStatus(ctx context.Context, req *StatusRequest) (*StatusResponse, error)
    GetLogs(ctx context.Context, req *LogsRequest) (<-chan LogEntry, error)
    
    // Command execution (required)
    Exec(ctx context.Context, req *ExecRequest) (*ExecResponse, error)
    
    // Filesystem operations (optional)
    ReadFile(ctx context.Context, req *ReadFileRequest) (*ReadFileResponse, error)
    WriteFile(ctx context.Context, req *WriteFileRequest) error
    CopyFiles(ctx context.Context, req *CopyFilesRequest) error
    
    // Snapshot/export (optional)
    ExportFilesystem(ctx context.Context, req *ExportRequest) (<-chan FileChunk, error)
    ImportFilesystem(ctx context.Context, req *ImportRequest) (*CreateResponse, error)
    CreateSnapshot(ctx context.Context, req *SnapshotRequest) (*SnapshotResponse, error)
    RestoreFromSnapshot(ctx context.Context, req *RestoreRequest) (*CreateResponse, error)
    
    // Suspend/resume (optional)
    Suspend(ctx context.Context, req *SuspendRequest) error
    Resume(ctx context.Context, req *ResumeRequest) error
    
    // Resource queries (optional)
    GetAllocatedResources(ctx context.Context, req *ResourceRequest) (*Resources, error)
    GetAvailableResources(ctx context.Context) (*Resources, error)
}

// Plugin implementation
type MeshPlugin struct {
    impl SubstrateAdapter
}

func (p *MeshPlugin) Server(*plugin.MuxBroker) (interface{}, error) {
    return &GRPCServer{impl: p.impl}, nil
}

func (p *MeshPlugin) Client(b *plugin.MuxBroker, c *plugin.RPCClient) (interface{}, error) {
    return &GRPCClient{client: NewPluginClient(c.Conn())}, nil
}

// Plugin main
func Serve(impl SubstrateAdapter) {
    plugin.Serve(&plugin.ServeConfig{
        HandshakeConfig: plugin.HandshakeConfig{
            ProtocolVersion:  1,
            MagicCookieKey:   "MESH_PLUGIN",
            MagicCookieValue: "e4f7a3b2c1d0e9f8a7b6c5d4e3f2a1b0",
        },
        Plugins: map[string]plugin.Plugin{
            "substrate": &MeshPlugin{impl: impl},
        },
        GRPCServer: plugin.DefaultGRPCServer,
    })
}
```

### Plugin Discovery and Loading

**Discovery locations:**
```bash
~/.mesh/plugins/substrate-<name>/
~/.mesh/plugins/substrate-<name>@<version>/
/opt/mesh/plugins/substrate-<name>/
```

**Plugin naming convention:**
- Binary: `mesh-substrate-<name>`
- Example: `mesh-substrate-digitalocean`, `mesh-substrate-e2b`

**Loading process:**
```go
type PluginManager struct {
    pluginDir string
    plugins   map[string]*plugin.Client
    mu        sync.RWMutex
}

func (pm *PluginManager) LoadPlugin(name string) (*SubstrateAdapterClient, error) {
    pluginPath := filepath.Join(pm.pluginDir, "mesh-substrate-"+name)
    
    client := plugin.NewClient(&plugin.ClientConfig{
        Cmd: exec.Command(pluginPath),
        HandshakeConfig: plugin.HandshakeConfig{
            ProtocolVersion:  1,
            MagicCookieKey:   "MESH_PLUGIN",
            MagicCookieValue: "e4f7a3b2c1d0e9f8a7b6c5d4e3f2a1b0",
        },
        AllowedProtocols: []plugin.Protocol{plugin.ProtocolGRPC},
    })
    
    rpcClient, err := client.Client()
    if err != nil {
        return nil, err
    }
    
    raw, err := rpcClient.Dispense("substrate")
    if err != nil {
        return nil, err
    }
    
    adapter := raw.(*GRPCClient)
    
    pm.mu.Lock()
    pm.plugins[name] = client
    pm.mu.Unlock()
    
    return &SubstrateAdapterClient{impl: adapter}, nil
}
```

### Plugin vs Core Boundary

**Core responsibilities (IN core):**

1. **Body lifecycle management**
   - Body creation, deletion, state tracking
   - Body ID generation and persistence
   - Snapshot lifecycle (create, list, delete)
   - Cold migration orchestration

2. **Substrate adapter interface**
   - gRPC protocol definitions
   - Plugin discovery and loading
   - Plugin lifecycle (start, stop, restart, cleanup)
   - Capability querying and validation

3. **MCP server**
   - Tool definitions (spawn, snapshot, migrate, destroy)
   - Request routing and validation
   - Error handling and reporting
   - Authentication/authorization (if any)

4. **Networking**
   - Tailscale integration
   - Port forwarding
   - Network policy enforcement

5. **Snapshot pipeline**
   - OCI image management
   - Tarball compression/decompression
   - Registry interface (Docker Hub, user S3, provider-native)

6. **Scheduler integration** (Fleet substrate)
   - Nomad job submission
   - Resource allocation tracking
   - Scheduling policies

**Plugin responsibilities (NOT in core):**

1. **Provider-specific implementation**
   - DigitalOcean droplet API calls
   - AWS EC2/ECS/Fargate integration
   - E2B sandbox API
   - Fly.io Machines API
   - Modal API
   - Cloudflare Workers

2. **Registry backends**
   - Docker Hub connector
   - AWS ECR connector
   - Google GCR connector
   - S3-compatible storage

3. **Scheduler policies** (optional plugins)
   - Cost-based scheduling
   - Latency-based scheduling
   - Geo-distribution policies

**The line:**

- **Core** if:
  - Required for ALL substrates (body lifecycle, MCP server)
  - Universal abstraction (SubstrateAdapter interface)
  - Platform-independent (Tailscale, OCI images)
  - Single correct implementation (no provider variations)

- **Plugin** if:
  - Provider-specific (DigitalOcean, AWS, E2B)
  - Multiple valid implementations (registry backends, scheduling policies)
  - User-choice (which provider to use, which scheduler policy)
  - Fast-evolving or third-party APIs

### Pulumi Skill Integration

**How the Pulumi skill generates plugins:**

```typescript
// Mesh skill: generate-provider.ts
import { PulumiAI } from '@pulumi/ai';

async function generateProvider(providerName: string, providerSpec: any) {
    // 1. Extract provider requirements
    const requirements = extractRequirements(providerSpec);
    
    // 2. Generate infrastructure provisioning code using Pulumi AI
    const pulumiAI = new PulumiAI({
        apiKey: process.env.OPENAI_API_KEY,
        model: 'gpt-4'
    });
    
    const prompt = `
Generate Pulumi code in TypeScript to:
1. Create ${providerName} instances
2. Start/stop instances
3. Execute commands in instances
4. Export/import filesystem (if supported)
5. Get instance status and logs

Requirements:
- Provider: ${providerName}
- API docs: ${requirements.apiDocs}
- Example API calls: ${requirements.examples}
- Constraints: ${requirements.constraints}
    `;
    
    const pulumiCode = await pulumiAI.generate(prompt);
    
    // 3. Wrap generated code in SubstrateAdapter interface
    const pluginCode = wrapInAdapter(pulumiCode, providerName, requirements);
    
    // 4. Add error handling, retries, timeouts
    const finalPlugin = addErrorHandling(pluginCode);
    
    // 5. Output complete plugin code
    return finalPlugin;
}

function wrapInAdapter(pulumiCode: string, name: string, req: any): string {
    return `
package main

import (
    ${name}pulumi "${name}-pulumi"
    "github.com/mesh/mesh-plugin-sdk"
)

type ${capitalize(name)}Adapter struct {
    client *${name}pulumi.Provider
}

func (a *${capitalize(name)}Adapter) PluginInfo() *meshplugin.PluginInfo {
    return &meshplugin.PluginInfo{
        Name:        "${name}",
        Version:     "1.0.0",
        Description: "Mesh substrate adapter for ${name}",
    }
}

func (a *${capitalize(name)}Adapter) GetCapabilities() *meshplugin.Capabilities {
    return &meshplugin.Capabilities{
        // ... based on provider capabilities
    }
}

// Embed generated Pulumi code here
${pulumiCode}

func main() {
    meshplugin.Serve(&${capitalize(name)}Adapter{})
}
    `;
}
```

**Skill invocation:**
```bash
# Via MCP tool
mesh generate-provider digitalocean --spec do-spec.yaml

# Via CLI
mesh plugin generate --name digitalocean --spec do-spec.yaml
```

**Output:**
- Complete Go plugin code
- Compiled binary
- Installation instructions
- Example usage
- Test suite scaffold

### Plugin Repo Structure

```
mesh-plugins/
├── providers/
│   ├── digitalocean/
│   │   ├── main.go           # Plugin entry point
│   │   ├── adapter.go        # SubstrateAdapter implementation
│   │   ├── pulumi.go         # Generated Pulumi code
│   │   ├── go.mod
│   │   ├── go.sum
│   │   ├── README.md
│   │   ├── plugin.yaml       # Plugin metadata
│   │   └── examples/
│   │       └── basic/
│   │           └── main.go
│   ├── aws/
│   │   ├── ...
│   ├── e2b/
│   │   ├── ...
│   └── fly/
│       └── ...
├── registries/
│   ├── dockerhub/
│   │   └── ...
│   ├── s3/
│   │   └── ...
│   └── gcr/
│       └── ...
├── schedulers/
│   ├── cost-aware/
│   │   └── ...
│   └── latency-aware/
│       └── ...
└── README.md
```

**Plugin metadata (`plugin.yaml`):**
```yaml
name: digitalocean
version: 1.0.0
description: Mesh substrate adapter for DigitalOcean droplets
author: Mesh Community
license: MIT
homepage: https://github.com/mesh-plugins/providers/digitalocean

# Plugin binary
binary: mesh-substrate-digitalocean

# Capabilities
capabilities:
  required:
    - create
    - start
    - stop
    - destroy
    - get_status
    - exec
  optional:
    - read_file
    - write_file
    - copy_files
    - export_filesystem
    - import_filesystem
  snapshot_type: filesystem-only
  portable_snapshots: true
  auto_resume: false

# Resource limits
min_resources:
  cpu: 0.1
  memory: 512
  disk: 1

max_resources:
  cpu: 64
  memory: 131072
  disk: 1000

# GPU support
gpu: false

# Dependencies
dependencies:
  - name: pulumi-digitalocean
    version: ^4.0.0

# Installation
install:
  pre:
    - go install github.com/pulumi/pulumi-digitalocean/cmd/pulumi-resource-digitalocean@latest
  post:
    - mesh plugin register digitalocean --version 1.0.0
```

### Plugin Installation

**Via CLI:**
```bash
# From GitHub
mesh plugin install github.com/mesh-plugins/providers/digitalocean

# From local directory
mesh plugin install ./digitalocean

# From plugin registry
mesh plugin install digitalocean --version 1.0.0

# Generate new plugin using Pulumi AI
mesh plugin generate digitalocean --spec do-spec.yaml
```

**Via MCP tool:**
```json
{
  "tool": "mesh_plugin_install",
  "arguments": {
    "name": "digitalocean",
    "version": "1.0.0"
  }
}
```

**Installation process:**
1. Download plugin binary
2. Verify checksum
3. Place in `~/.mesh/plugins/`
4. Run pre-install hooks (if any)
5. Load plugin and verify capabilities
6. Run post-install hooks (if any)
7. Register plugin in Mesh config

**Plugin configuration:**
```yaml
# ~/.mesh/config.yaml
plugins:
  digitalocean:
    enabled: true
    version: "1.0.0"
    config:
      api_token: ${DO_API_TOKEN}
      region: nyc3
  e2b:
    enabled: true
    version: "0.5.0"
    config:
      api_key: ${E2B_API_KEY}
```

### Error Handling and Status Reporting

**Error codes (protobuf enum):**
```protobuf
enum ErrorCode {
    UNKNOWN = 0;
    INSTANCE_NOT_FOUND = 1;
    INSTANCE_ALREADY_EXISTS = 2;
    INVALID_STATE = 3;
    INSUFFICIENT_RESOURCES = 4;
    NETWORK_ERROR = 5;
    AUTHENTICATION_ERROR = 6;
    NOT_SUPPORTED = 7;
    TIMEOUT = 8;
    QUOTA_EXCEEDED = 9;
    RATE_LIMITED = 10;
}
```

**Status reporting:**
- Plugins return gRPC status codes
- Detailed error messages in response payloads
- Plugins signal retryable vs. fatal errors
- Mesh core implements retry logic for retryable errors

**Plugin health checks:**
```go
// Periodic health check
func (pm *PluginManager) HealthCheck(ctx context.Context) error {
    client := pm.plugins["digitalocean"]
    if client == nil {
        return errors.New("plugin not loaded")
    }
    
    rpcClient, err := client.Client()
    if err != nil {
        return err
    }
    
    adapter := rpcClient.Dispense("substrate")
    if err != nil {
        return err
    }
    
    // Ping plugin
    info, err := adapter.(*GRPCClient).GetPluginInfo(ctx, &Empty{})
    if err != nil {
        return err
    }
    
    // Check version compatibility
    if !isCompatible(info.Version) {
        return fmt.Errorf("incompatible plugin version: %s", info.Version)
    }
    
    return nil
}
```

### Plugin Lifecycle

```
1. Discovery
   - Scan plugin directories
   - Read plugin metadata
   - Validate version compatibility

2. Load
   - Launch plugin subprocess
   - Perform handshake
   - Verify capabilities
   - Cache plugin client

3. Use
   - Call plugin methods via gRPC
   - Handle errors and retries
   - Stream logs and file data

4. Health Check
   - Periodic ping
   - Capabilities verification
   - Resource monitoring

5. Unload
   - Stop accepting new requests
   - Wait for in-flight requests
   - Graceful shutdown
   - Kill process if needed

6. Update
   - Unload old version
   - Install new version
   - Load new version
   - Migrate state if needed
```

## Key Findings

### F1: gRPC is the de facto standard for plugin protocols
Terraform, Pulumi, and HashiCorp go-plugin all use gRPC. It provides:
- Cross-language support via Protocol Buffers
- Streaming support (logs, file transfers)
- Built-in error handling and status codes
- Bi-directional streaming
- Performance and efficiency

**Implication for Mesh:** Use gRPC for the plugin protocol. It's proven, well-documented, and familiar to infrastructure developers.

### F2: Subprocess isolation is critical
All studied systems (Terraform, Pulumi, Nomad) run plugins as separate OS processes. This provides:
- Crash isolation (plugin bug doesn't crash core)
- Resource isolation (CPU, memory limits)
- Security boundary (limited access to host)
- Language independence (plugin can be any language)

**Implication for Mesh:** Run substrate plugins as separate processes. Use HashiCorp go-plugin for lifecycle management.

### F3: Capability declaration enables optional features
Terraform providers, Pulumi providers, and Nomad drivers all declare capabilities at load time. This allows:
- Runtime feature detection
- Graceful degradation when features missing
- Clear documentation of supported operations
- Validation before operation

**Implication for Mesh:** SubstrateAdapter must declare capabilities. Mesh core queries capabilities before using optional features.

### F4: Go is the preferred plugin language
Pulumi and Terraform both recommend Go for plugin development because:
- Compiles to standalone binary (no runtime dependencies)
- Excellent gRPC support
- Strong typing and tooling
- Can be consumed from any Pulumi language

**Implication for Mesh:** Provide Go SDK for plugin development. Allow other languages via gRPC but optimize for Go.

### F5: Plugin discovery is directory-based
All studied systems discover plugins by scanning directories:
- Terraform: Binary in PATH or configured path
- Pulumi: `PULUMI_HOME` directory or package managers
- Nomad: `plugin_dir` configuration
- Docker: CLI plugin directory or Marketplace

**Implication for Mesh:** Use `~/.mesh/plugins/` for user-installed plugins. Support plugin registry for discovery.

### F6: Pulumi AI generates infrastructure code, not plugin code
Pulumi Neo and Pulumi AI generate Pulumi programs (TypeScript, Python, Go) that call Pulumi providers. They don't generate the providers themselves.

**Implication for Mesh:** Mesh skill must use Pulumi AI as a sub-component to generate infrastructure provisioning code, then wrap it in the SubstrateAdapter interface.

### F7: Plugin versioning is critical
All studied systems have robust versioning:
- Terraform: Protocol versioning (v5, v6)
- Pulumi: Provider versioning
- Nomad: Driver API versioning
- Docker: Extension versioning

**Implication for Mesh:** Implement protocol versioning and plugin versioning. Support multiple plugin versions simultaneously.

### F8: Error handling varies by system
- Terraform: gRPC status codes with detailed messages
- Pulumi: gRPC errors with rich metadata
- Nomad: Plugin restart on crash, reattach support
- Docker: Exception handling in containers

**Implication for Mesh:** Define error code enum. Distinguish retryable errors (rate limits, network) from fatal errors (auth, quota). Implement plugin restart on crash.

### F9: Plugin state management is system-specific
- Terraform: No plugin state (state in Terraform state file)
- Pulumi: No plugin state (state in Pulumi state file)
- Nomad: DriverState for reattach after crash
- Docker: Extension-managed state

**Implication for Mesh:** SubstrateAdapter instances should be stateless (per-request). Mesh core manages all state (body state, snapshot metadata).

### F10: Plugin testing is framework-dependent
- Terraform: acceptance test framework
- Pulumi: Go SDK testing framework
- Nomad: Test utilities in go-plugin

**Implication for Mesh:** Provide testing framework in Go SDK. Mock SubstrateAdapter interface for unit tests.

## Verdict

### Recommended Plugin System Design

**1. Protocol: gRPC over HashiCorp go-plugin**

**Rationale:**
- Proven at scale (Terraform 4000+ providers, Pulumi 100+ providers)
- Cross-language support via Protocol Buffers
- Subprocess isolation for security and stability
- Built-in handshake, versioning, and lifecycle management
- Streaming support for logs and file transfers

**Implementation:**
```go
// Plugin serves via go-plugin
plugin.Serve(&plugin.ServeConfig{
    HandshakeConfig: plugin.HandshakeConfig{
        ProtocolVersion:  1,
        MagicCookieKey:   "MESH_PLUGIN",
        MagicCookieValue: "e4f7a3b2c1d0e9f8a7b6c5d4e3f2a1b0",
    },
    Plugins: map[string]plugin.Plugin{
        "substrate": &MeshPlugin{impl: adapterImpl},
    },
    GRPCServer: plugin.DefaultGRPCServer,
})
```

**2. Plugin Language: Go (primary), any language via gRPC (secondary)**

**Rationale:**
- Go compiles to standalone binary (no runtime dependencies)
- Excellent gRPC support
- Rich ecosystem for infrastructure tools
- Proven by Terraform and Pulumi
- Users can write plugins in Python, TypeScript, Rust, etc. via gRPC bindings

**3. Plugin Interface: SubstrateAdapter with capability declaration**

**Rationale:**
- Clear separation between required and optional methods
- Runtime feature detection enables graceful degradation
- Capability queries validate requirements before operation
- Supports diverse substrate capabilities (Docker, E2B, Fly, Modal)

**4. Plugin Discovery: Directory-based with plugin registry**

**Rationale:**
- Simple and predictable (`~/.mesh/plugins/`)
- Supports manual installation (drop binary in directory)
- Supports plugin registry for discovery
- Versioned directories (`mesh-substrate-digitalocean@1.0.0/`)

**5. Pulumi Skill Integration: Sub-component approach**

**Rationale:**
- Pulumi AI generates infrastructure provisioning code (not plugin code)
- Mesh skill wraps generated code in SubstrateAdapter interface
- Enables users to generate plugins for any cloud with a Pulumi or Terraform provider
- Leverages Pulumi's 4000+ provider ecosystem

**Workflow:**
```
1. User asks Mesh skill: "Generate a DigitalOcean provider"
2. Mesh skill extracts requirements from DigitalOcean API docs
3. Mesh skill asks Pulumi AI: "Generate Pulumi code for DigitalOcean droplets"
4. Pulumi AI generates TypeScript/Go code using @pulumi/digitalocean
5. Mesh skill wraps code in SubstrateAdapter interface
6. Mesh skill outputs complete Go plugin code
7. User compiles and installs plugin
```

### Critical Decisions

**D11: Use gRPC for plugin protocol**
All communication between Mesh core and substrate plugins is via gRPC. Protocol defined in `mesh_plugin.proto`. Handshake via HashiCorp go-plugin.

**D12: Provide Go SDK as primary plugin development interface**
Go SDK includes: SubstrateAdapter interface, base implementation, testing framework, logging utilities, error handling helpers. Compile to standalone binary with no runtime dependencies.

**D13: Plugin capabilities are declared at load time**
SubstrateAdapter.GetCapabilities() returns capability set. Mesh core queries capabilities before using optional features. Enables graceful degradation and feature detection.

**D14: Plugins are stateless**
SubstrateAdapter instances are stateless. All state (body state, snapshot metadata, instance IDs) is managed by Mesh core. Enables plugin restart without data loss.

**D15: Plugin errors distinguish retryable vs. fatal**
ErrorCode enum includes: NETWORK_ERROR, TIMEOUT, RATE_LIMITED (retryable) vs. AUTHENTICATION_ERROR, QUOTA_EXCEEDED, NOT_SUPPORTED (fatal). Mesh core implements retry logic for retryable errors.

**D16: Pulumi skill generates infrastructure code, not plugin code**
Mesh skill uses Pulumi AI as a sub-component to generate cloud API interaction code, then wraps it in SubstrateAdapter interface. Does NOT generate the plugin interface itself (provided by Mesh skill).

**D17: Plugin repo structure follows Terraform/Pulumi patterns**
Mesh plugins follow same structure as Terraform/Pulumi providers: provider binary, metadata file, examples, test suite. Enables familiarity for infrastructure developers.

### Next Steps

1. **Implement plugin protocol** - Define `mesh_plugin.proto` with all SubstrateAdapter methods
2. **Implement Go SDK** - Create `mesh-plugin-sdk` package with base implementation, testing framework
3. **Implement plugin manager** - Plugin discovery, loading, lifecycle management, health checks
4. **Implement Pulumi skill** - Mesh skill that uses Pulumi AI to generate infrastructure code
5. **Create reference plugin** - DigitalOcean provider as example implementation
6. **Document plugin development** - Getting started guide, examples, best practices
7. **Implement plugin registry** - Central registry for plugin discovery and distribution

### Risks and Mitigations

**Risk 1: Plugin ecosystem fragmentation**
- **Mitigation:** Provide clear plugin interface, comprehensive SDK, reference implementations
- **Mitigation:** Establish plugin registry with quality standards

**Risk 2: Pulumi AI generates incorrect code**
- **Mitigation:** Mesh skill validates generated code against interface
- **Mitigation:** Provide test scaffold and examples
- **Mitigation:** Community review and plugin certification

**Risk 3: Plugin performance overhead**
- **Mitigation:** gRPC is efficient; subprocess isolation minimal overhead
- **Mitigation:** Cache plugin clients; reuse connections
- **Mitigation:** Benchmark plugin communication

**Risk 4: Plugin security vulnerabilities**
- **Mitigation:** Subprocess isolation limits plugin access
- **Mitigation:** Plugin signature verification
- **Mitigation:** Plugin sandboxing (optional, advanced)

**Risk 5: Plugin version incompatibility**
- **Mitigation:** Protocol versioning with backward compatibility
- **Mitigation:** Capability declaration enables feature detection
- **Mitigation:** Support multiple plugin versions simultaneously

### Alternative Considered and Rejected

**Alternative: HTTP/REST API for plugins**
- **Rejected:** gRPC is more efficient for binary data (file transfers), supports streaming, has better tooling, is industry standard for infrastructure plugins

**Alternative: In-process plugins (shared libraries)**
- **Rejected:** No crash isolation, language coupling, security risk, not proven at scale

**Alternative: Plugin generates full plugin code (including interface)**
- **Rejected:** Pulumi AI doesn't understand Mesh-specific requirements; better for Mesh skill to provide interface and use Pulumi AI for infrastructure code

**Alternative: WebAssembly (WASM) for plugins**
- **Rejected:** Immature for infrastructure plugins, limited ecosystem, no clear benefits over gRPC, potential performance overhead

### Success Criteria

1. **Plugin can be written in < 100 lines of Go** (using SDK) for simple substrates
2. **Plugin compiles to single binary** with no runtime dependencies
3. **Plugin can be installed via single command** (`mesh plugin install <name>`)
4. **Plugin capabilities are discoverable at runtime** via GetCapabilities()
5. **Plugin crash doesn't affect Mesh core** (subprocess isolation)
6. **Pulumi skill can generate working plugin** for any cloud with Pulumi or Terraform provider
7. **Plugin performance overhead < 10ms** per operation (excluding substrate API calls)
8. **Plugin supports streaming** for logs and file transfers
9. **Plugin can be updated without Mesh restart** (dynamic loading/unloading)
10. **Plugin error handling distinguishes retryable vs. fatal** errors

The proposed plugin architecture is sound, follows industry best practices, and enables Mesh to scale without bloating core.
