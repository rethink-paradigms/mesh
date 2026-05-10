# Mesh Daemon Configuration

## Overview

The Mesh daemon reads a YAML configuration file on startup. By default it
looks for `~/.mesh/config.yaml`. You can override the path with the
`MESH_CONFIG` environment variable. There is no `.env` file support.

The file defines daemon runtime settings, state store location, orchestrator
connections, provisioners, body definitions, artifact registry access, and
plugin loading.

## Top-Level Schema

### `daemon:` (DaemonConfig)

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `socket_path` | string | No | `/tmp/mesh.sock` | Unix domain socket path for IPC |
| `pid_file` | string | No | `~/.mesh/mesh.pid` | PID file path |
| `log_level` | string | No | `info` | One of: debug, info, warn, error |
| `listen_addr` | string | No | `127.0.0.1:8080` | HTTP REST API listen address |
| `auth_token` | string | **Yes** | -- | Bearer token for REST API authentication and heartbeat authentication to agent-bodies gateway |
| `auth_mode` | string | No | `token` | Authentication mode: `token`, `jwt`, or `both` |
| `auth0_domain` | string | Conditional | -- | Auth0 tenant domain (required when `auth_mode` is `jwt` or `both`) |
| `auth0_audience` | string | Conditional | -- | Auth0 API audience identifier (required when `auth_mode` is `jwt` or `both`) |
| `cluster_owner_id` | string | No | -- | Auth0 user ID that owns this cluster |
| `cluster_id` | string | No | -- | Cluster UUID assigned by the agent-bodies gateway |
| `gateway_url` | string | No | (empty) | URL of the agent-bodies gateway for heartbeat and registration. Empty string disables heartbeat. Example: `https://gateway.example.com` |
| `heartbeat_interval_seconds` | int | No | `30` | Seconds between heartbeats to the gateway. Set to `0` to disable heartbeat (also disables when `gateway_url` is empty) |

### `store:` (StoreConfig)

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `path` | string | No | `~/.mesh/state.db` | SQLite database path for persistent state |

### `orchestrators:` (map[string]map[string]string)

Orchestrator-specific key-value pairs. Defaults to a single `nomad` entry with
`address: http://127.0.0.1:4646`. Optional. Add additional orchestrators by
name, each with its own set of keys.

### `provisioners:` (map[string]map[string]string)

Provisioner-specific configuration. Optional. Same structure as orchestrators.

### `bodies:` ([]BodyConfig)

A list of static body definitions the daemon should know about at startup.

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `name` | string | **Yes** | -- | Human-readable body name |
| `image` | string | **Yes** | -- | OCI container image reference |
| `workdir` | string | No | -- | Working directory inside the container |
| `env` | map[string]string | No | -- | Environment variables to inject |
| `cmd` | []string | No | -- | Command to run on container start |
| `memory_mb` | int | No | -- | Memory limit in megabytes |
| `cpu_shares` | int | No | -- | CPU shares (relative weight) |
| `substrate` | string | No | `docker` | Substrate adapter to use |

### `registry:` (RegistryConfig)

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `type` | string | No | `s3` | Registry type (currently only `s3`) |
| `bucket` | string | **Yes** (if type=s3) | -- | S3 bucket name |
| `region` | string | No | -- | AWS region |
| `endpoint` | string | No | -- | Custom S3-compatible endpoint URL |
| `access_key_id` | string | No | -- | AWS access key ID |
| `secret_access_key` | string | No | -- | AWS secret access key |

### `plugin:` (PluginConfig)

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `dir` | string | No | `~/.mesh/plugins` | Directory to load plugins from |
| `enabled` | []string | No | -- | List of plugin names to enable |

### `ingress:` (IngressConfig)

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `adapter` | string | No | `noop` (auto-detected) | Ingress adapter to use. `caddy` for Caddy-based HTTP routing, `noop` for no routing, or empty to auto-detect |
| `admin_url` | string | No | `http://127.0.0.1:2019` | Caddy admin API URL (only used when adapter is `caddy`) |
| `port_pool_start` | int | No | `9000` | Start of the port pool range for exposed body ports |
| `port_pool_end` | int | No | `9999` | End of the port pool range for exposed body ports |
| `domain_suffix` | string | No | `.mesh.local` | Domain suffix appended to exposed body routes |

**Caddy auto-detection behavior:**

When `adapter` is empty or set to `"noop"`, the daemon automatically probes the
Caddy admin API at `http://127.0.0.1:2019/config/`. If Caddy responds with
HTTP 200, the adapter is automatically upgraded to `"caddy"`. If Caddy does not
respond (not installed, not running, or on a different port), the adapter stays
as `"noop"` and no HTTP routing is configured.

When `adapter` is explicitly set to `"caddy"`, no auto-detection runs. The
daemon uses Caddy regardless of whether it is actually reachable.

To force no routing without auto-detection probing, set `adapter: "noop"`.

### `agents_dir`

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `agents_dir` | string | No | `~/.mesh/agents` | Directory containing agent manifest YAML files loaded at startup |

## REST API: Agent Install Manifest

The `POST /api/v1/agents/install` endpoint accepts an optional `manifest` field
in the request body. This field contains inline YAML that defines or overrides
the agent manifest for the installed agent.

```json
{
  "agent_type": "hermes",
  "name": "my-agent",
  "env": { "LOG_LEVEL": "debug" },
  "manifest": "name: agent\nimage: hermes:latest\n"
}
```

When provided, the manifest is parsed by the daemon to extract agent
configuration (name, image, etc.) and takes precedence over any manifest
loaded from the agents directory. When omitted, the daemon uses the manifest
from the configured `agents_dir`.

## Examples

### LITE mode (Docker adapter, no Nomad)

Minimal config for development or single-machine use with the Docker
substrate and a local store. No Nomad required.

```yaml
daemon:
  socket_path: "/tmp/mesh.sock"
  listen_addr: "127.0.0.1:8080"
  auth_token: "dev-token"

store:
  path: "/home/user/.mesh/state.db"

orchestrators:
  docker:
    address: "unix:///var/run/docker.sock"

registry:
  type: s3
  bucket: "mesh-artifacts-dev"
  region: "us-east-1"

plugin:
  dir: "/home/user/.mesh/plugins"
```

### STANDARD mode (Nomad adapter, full fleet)

Full production config with Nomad orchestrator, S3 artifact registry, static
bodies, plugin loading, gateway heartbeat, and ingress routing.

```yaml
daemon:
  socket_path: "/tmp/mesh.sock"
  pid_file: "/var/run/mesh.pid"
  log_level: "info"
  listen_addr: "0.0.0.0:8080"
  auth_token: "${DAEMON_TOKEN}"
  gateway_url: "https://gateway.example.com"
  heartbeat_interval_seconds: 30

store:
  path: "/var/lib/mesh/state.db"

orchestrators:
  nomad:
    address: "http://nomad.service.consul:4646"
    region: "us-east-1"
    namespace: "mesh"

ingress:
  adapter: "caddy"
  admin_url: "http://127.0.0.1:2019"
  port_pool_start: 9000
  port_pool_end: 9999
  domain_suffix: ".mesh.example.com"

bodies:
  - name: "build-worker"
    image: "mesh/build-worker:latest"
    workdir: "/workspace"
    env:
      LOG_LEVEL: "debug"
    memory_mb: 2048
    cpu_shares: 512
    substrate: "docker"

  - name: "sandbox-runner"
    image: "mesh/sandbox:latest"
    cmd: ["/entrypoint.sh", "--sandbox"]
    memory_mb: 1024
    substrate: "nomad"

registry:
  type: s3
  bucket: "mesh-artifacts-prod"
  region: "us-west-2"
  endpoint: "https://s3.us-west-2.amazonaws.com"

plugin:
  dir: "/etc/mesh/plugins"
  enabled:
    - "healthcheck"
    - "metrics"
```

## Warnings

### `auth_token` dual purpose

The `auth_token` field serves two roles:
1. **REST API authentication** -- sent as a Bearer token in `Authorization` headers
2. **Heartbeat authentication** -- sent to the agent-bodies gateway at `gateway_url` to authenticate heartbeat requests

When `gateway_url` is configured and heartbeat is enabled, the daemon sends
periodic heartbeats using the same `auth_token` for authentication. Keep this
token secure. If compromised, an attacker can impersonate the daemon to both
the REST API and the gateway.

### Ingress `adapter` port binding

When using the `caddy` adapter, bodies with exposed HTTP ports are routed via
Caddy reverse proxy. The port pool (`port_pool_start` to `port_pool_end`)
defines the range of host ports available for port mapping. Ensure this range
does not conflict with other services on the host.

### `auth_token` nesting

The `auth_token` field **must** be nested under `daemon:`:

```yaml
# Correct:
daemon:
  auth_token: "some-token"

# Wrong (daemon will refuse to start):
auth_token: "some-token"
```

A top-level `auth_token` is silently ignored by the parser. The daemon
checks this field on startup and refuses to start with an unprotected API,
so a misconfigured file produces a hard failure with no indication that the
format is wrong.

### Legacy `[nomad]` section

The legacy flat `nomad:` section is supported for backward compatibility:

```yaml
nomad:
  address: "http://nomad:4646"
  token: "abc"
  region: "global"
  namespace: "default"
```

It auto-migrates to `orchestrators.nomad` at load time. Prefer writing
`orchestrators.nomad` directly in new configs.

## Validation

The daemon validates the following at startup:

- **Body entries**: `name` and `image` must be non-empty. Missing either
  produces a `config: body has empty name` or `config: body "X": image must
  not be empty` error.
- **Plugin directory**: If `plugin.dir` is set, the directory must exist on
  disk. If missing, the daemon returns a `config: plugin dir "X" does not
  exist` error (unless `MESH_TESTING` is set, in which case it creates the
  directory).
- **Nomad address**: Must be a valid HTTP or HTTPS URL. Invalid URLs produce
  a `config: nomad address "X" is not a valid URL` error.
- **S3 registry**: When `registry.type` is `s3`, the `bucket` field is
  required. Missing it produces `config: registry bucket is required when
  type is s3`.
- **Auth mode**: `auth_mode` must be one of `"token"`, `"jwt"`, or `"both"`.
  Invalid values produce `config: auth_mode "X" is invalid`.
- **Auth0 fields**: When `auth_mode` is `"jwt"` or `"both"`, both
  `auth0_domain` and `auth0_audience` are required. Missing either produces
  a `config: auth0_domain is required when auth_mode is "X"` error.
