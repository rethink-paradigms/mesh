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
| `auth_token` | string | **Yes** | -- | Bearer token for REST API authentication |

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
bodies, and plugin loading.

```yaml
daemon:
  socket_path: "/tmp/mesh.sock"
  pid_file: "/var/run/mesh.pid"
  log_level: "info"
  listen_addr: "0.0.0.0:8080"
  auth_token: "${DAEMON_TOKEN}"

store:
  path: "/var/lib/mesh/state.db"

orchestrators:
  nomad:
    address: "http://nomad.service.consul:4646"
    region: "us-east-1"
    namespace: "mesh"

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
