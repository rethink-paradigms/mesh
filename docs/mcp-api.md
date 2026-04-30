# MCP API Reference

Mesh exposes a Model Context Protocol (MCP) server over stdio transport using JSON-RPC 2.0. This is the primary interface for AI agents to manage bodies, snapshots, and migrations (D5). AI agents communicate with Mesh via MCP tools, not CLI commands.

## Transport

The MCP server listens on **stdin/stdout** (stdio). Each request is a single JSON line. Each response is a single JSON line. Messages are newline-delimited. There is no HTTP layer.

The server is started by `mesh serve`. The daemon registers all tools at startup and processes requests in a loop until EOF or context cancellation.

### Connection Lifecycle

1. Agent starts `mesh serve` (or connects to a running daemon)
2. Agent sends `initialize` request to negotiate protocol version
3. Agent sends `tools/list` to discover available tools
4. Agent sends `tools/call` with tool name and arguments to execute operations
5. Agent closes stdin to signal end of session

## JSON-RPC 2.0

### Request Format

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "create_body",
    "arguments": {
      "name": "my-agent",
      "image": "ubuntu:22.04"
    }
  }
}
```

### Success Response

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "content": [
      {
        "type": "text",
        "text": "{\"id\":\"uuid-here\",\"name\":\"my-agent\",\"state\":\"Running\",\"handle\":\"container-id\"}"
      }
    ]
  }
}
```

### Error Response

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "error": {
    "code": -32602,
    "message": "missing required parameter: id"
  }
}
```

### Error Codes

| Code | Meaning |
|------|---------|
| -32700 | Parse error (invalid JSON) |
| -32600 | Invalid request |
| -32601 | Method not found |
| -32602 | Invalid params (missing required fields) |
| -32603 | Internal error (daemon or adapter failure) |

## Lifecycle Methods

### `initialize`

Negotiate protocol version and discover server capabilities.

**Parameters:** none (or standard MCP initialize params)

**Response:**

```json
{
  "protocolVersion": "2024-11-05",
  "capabilities": {
    "tools": {}
  },
  "serverInfo": {
    "name": "mesh",
    "version": "0.1.0"
  }
}
```

### `tools/list`

List all registered tools with their names, descriptions, and input schemas.

**Parameters:** none

**Response:**

```json
{
  "tools": [
    {
      "name": "create_body",
      "description": "Create and start a new body on the substrate.",
      "inputSchema": {
        "type": "object",
        "properties": {
          "name": { "type": "string" },
          "image": { "type": "string" }
        },
        "required": ["name", "image"]
      }
    }
  ]
}
```

### `tools/call`

Execute a tool by name with the given arguments.

**Parameters:**

| Field | Type | Description |
|-------|------|-------------|
| `name` | `string` | Tool name to invoke |
| `arguments` | `object` | Tool-specific parameters |

## Tool Reference

### `ping`

Health check. Returns `{"pong": true}` if the daemon is running.

**Parameters:** none

**Response:** `{"pong": true}`

---

### `list_bodies`

List all managed bodies in the store. Returns an array of body records with ID, name, state, substrate, and timestamps.

**Parameters:** none

---

### `get_body`

Get detailed information about a specific body by ID.

**Parameters:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `id` | `string` | Yes | Body ID to retrieve |

---

### `create_body`

Create and start a new body on the substrate. Provisions a container from the specified image, starts it, and returns the body ID, name, state, and handle.

**Parameters:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | `string` | Yes | Human-readable name for the body |
| `image` | `string` | Yes | OCI container image (e.g., `ubuntu:22.04`) |
| `workdir` | `string` | No | Working directory inside the container |
| `env` | `object` | No | Environment variables (key-value pairs) |
| `cmd` | `array[string]` | No | Command to run on start |
| `memory_mb` | `integer` | No | Memory limit in megabytes |
| `cpu_shares` | `integer` | No | CPU shares (relative weight) |

**Response:**

```json
{
  "id": "abc123",
  "name": "my-agent",
  "state": "Running",
  "handle": "container-id-xyz"
}
```

---

### `delete_body`

Destroy a stopped or errored body. Removes the container and deletes the store record.

**Parameters:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `id` | `string` | Yes | Body ID to destroy |

**Response:** `{"deleted": true}`

---

### `start_body`

Start a stopped body. Transitions the body from Stopped to Starting to Running.

**Parameters:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `body_id` | `string` | Yes | Body ID to start |

**Response:**

```json
{
  "id": "abc123",
  "name": "my-agent",
  "state": "Running"
}
```

---

### `stop_body`

Stop a running body. Sends SIGTERM to the container process with a 30-second timeout.

**Parameters:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `body_id` | `string` | Yes | Body ID to stop |

**Response:**

```json
{
  "id": "abc123",
  "name": "my-agent",
  "state": "Stopped"
}
```

---

### `get_body_status`

Get runtime status of a body including state, uptime, memory, and CPU usage.

**Parameters:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `body_id` | `string` | Yes | Body ID to query |

**Response:**

```json
{
  "id": "abc123",
  "name": "my-agent",
  "state": "Running",
  "uptime_sec": 3600,
  "memory_mb": 256,
  "cpu_usage": 12.5
}
```

---

### `get_body_logs`

Get log output from a running body. Executes `tail -n N /var/log/mesh.log` inside the container.

**Parameters:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `body_id` | `string` | Yes | Body ID to fetch logs from |
| `tail` | `integer` | No | Number of lines from the end (default: 100) |

**Response:**

```json
{
  "body_id": "abc123",
  "logs": "[info] agent started\n[info] processing task...\n",
  "tail": 100
}
```

---

### `execute_command`

Execute a command inside a running body. Returns stdout, stderr, and exit code.

**Parameters:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `body_id` | `string` | Yes | Body ID to run the command in |
| `command` | `array[string]` | Yes | Command and arguments (e.g., `["ls", "-la"]`) |
| `timeout_seconds` | `integer` | No | Maximum execution time (default: 30) |

**Response:**

```json
{
  "stdout": "total 64\ndrwxr-xr-x ...",
  "stderr": "",
  "exit_code": 0
}
```

---

### `create_snapshot`

Create a filesystem snapshot of a running body. Exports the container filesystem, compresses with zstd, computes SHA-256, and stores the result with a manifest.

**Parameters:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `body_id` | `string` | Yes | Body ID to snapshot |
| `label` | `string` | No | Optional label for the snapshot (appended to ID) |

**Response:**

```json
{
  "id": "abc123-20260427-143000",
  "body_id": "abc123",
  "created_at": "2026-04-27T14:30:00Z",
  "size_bytes": 47350000,
  "sha256": "a1b2c3d4e5f6..."
}
```

---

### `list_snapshots`

List snapshots, optionally filtered by body ID.

**Parameters:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `body_id` | `string` | No | Filter to snapshots for a specific body |

---

### `get_snapshot`

Get details for a specific snapshot by ID.

**Parameters:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `id` | `string` | Yes | Snapshot ID to retrieve |

---

### `restore_body`

Restore a body from a snapshot. Extracts the snapshot tarball to a restore directory.

**Parameters:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `snapshot_id` | `string` | Yes | Snapshot ID to restore from |
| `target_substrate` | `string` | No | Optional target substrate for the restored body |

**Response:**

```json
{
  "restored": true,
  "snapshot_id": "abc123-20260427-143000",
  "body_id": "abc123",
  "target_dir": "/tmp/mesh-restore-abc123",
  "target_substrate": ""
}
```

---

### `migrate_body`

Migrate a body to a different substrate. Initiates the 7-step cold migration coordinator (export, provision, transfer, import, verify, switch, cleanup).

**Parameters:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `body_id` | `string` | Yes | Body ID to migrate |
| `target_substrate` | `string` | Yes | Target substrate name (e.g., `"docker"`, `"nomad"`) |

**Response:**

```json
{
  "migration_id": "mig-uuid-here"
}
```

---

### `list_plugins`

List all loaded plugins with name, version, state, and health status.

**Parameters:** none

**Response:**

```json
[
  {
    "name": "nomad-adapter",
    "version": "0.1.0",
    "state": "healthy",
    "healthy": true
  }
]
```

---

### `plugin_health`

Get detailed health information for a specific plugin.

**Parameters:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `plugin_name` | `string` | Yes | Plugin name to query |

**Response:**

```json
{
  "name": "nomad-adapter",
  "version": "0.1.0",
  "state": "healthy",
  "healthy": true,
  "fail_count": 0,
  "retry_count": 0,
  "description": "Nomad substrate adapter plugin",
  "author": "mesh-team"
}
```

## Tool Summary

| Tool | Purpose | Requires body_id |
|------|---------|------------------|
| `ping` | Health check | No |
| `list_bodies` | List all bodies | No |
| `get_body` | Get body details | Yes |
| `create_body` | Create and start a body | No |
| `delete_body` | Destroy a body | Yes |
| `start_body` | Start a stopped body | Yes |
| `stop_body` | Stop a running body | Yes |
| `get_body_status` | Get runtime status | Yes |
| `get_body_logs` | Fetch container logs | Yes |
| `execute_command` | Run a command | Yes |
| `create_snapshot` | Create filesystem snapshot | Yes |
| `list_snapshots` | List snapshots | No (optional filter) |
| `get_snapshot` | Get snapshot details | No |
| `restore_body` | Restore from snapshot | No |
| `migrate_body` | Migrate to a different substrate | Yes |
| `list_plugins` | List loaded plugins | No |
| `plugin_health` | Plugin health details | No |
