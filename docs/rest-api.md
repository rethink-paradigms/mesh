# REST API Reference

Mesh exposes an HTTP REST API for human operators and external tools to manage bodies and query the control plane. All endpoints return JSON. The REST API is designed for scripting, CI/CD, and debugging. AI agents use the MCP API instead (see [MCP API Reference](mcp-api.md)).

## Base URL

The daemon binds to the address and port configured in `mesh.yaml`. Default:

```
http://localhost:8080
```

## Authentication

The REST API uses Bearer token authentication. The token is configured in `mesh.yaml` under `auth_token`.

**Authenticated endpoints** require an `Authorization` header:

```
Authorization: Bearer <your-auth-token>
```

**Unauthenticated endpoints:**

| Endpoint | Reason |
|----------|--------|
| `GET /healthz` | Health check must work without auth for load balancers and monitoring |

Requests to authenticated endpoints without a valid token return `401 Unauthorized`.

## Common Error Format

Every error response follows this structure:

```json
{
  "error": {
    "code": "error_code_string",
    "message": "Human-readable description of what went wrong",
    "status": 400
  }
}
```

### Error Codes

| Code | HTTP Status | Description |
|------|-------------|-------------|
| `unauthorized` | 401 | Missing, malformed, or invalid bearer token |
| `body_not_found` | 404 | The specified body ID does not exist |
| `node_not_found` | 404 | The specified node ID does not exist |
| `body_conflict` | 409 | The body is not in the correct state for the requested action |
| `bad_request` | 400 | Malformed request payload or missing required fields |
| `nomad_unreachable` | 502 | The orchestrator backend (Nomad) is unreachable |
| `resource_exhausted` | 503 | No substrate capacity available for the request |
| `internal` | 500 | Unexpected server error |

---

### GET /healthz

Health check. Returns the daemon status, version, and aggregate counts. Bypasses authentication so load balancers and monitoring tools can check liveness without a token.

**Auth:** No

**Response schema:** `HealthzResponse`

| Field | Type | Description |
|-------|------|-------------|
| `status` | `string` | `"healthy"` or `"degraded"` (degraded when the orchestrator is unreachable) |
| `version` | `string` | Daemon version string |
| `nomad_connected` | `boolean` | Whether the orchestrator adapter reports healthy |
| `consul_connected` | `boolean` | Whether Consul is connected (reserved, always `false`) |
| `bodies_count` | `integer` | Number of bodies in the store |
| `nodes_count` | `integer` | Number of orchestrator nodes |

**Status codes:**

| Code | Description |
|------|-------------|
| 200 | Daemon is running |

**Example:**

```bash
curl http://localhost:8080/healthz
```

Response:

```json
{
  "status": "healthy",
  "version": "0.1.0",
  "nomad_connected": true,
  "consul_connected": false,
  "bodies_count": 3,
  "nodes_count": 5
}
```

---

### GET /api/v1/bodies

List all managed bodies. Returns an array of body records with ID, name, state, image, resource allocation, and timestamps.

**Auth:** Bearer token

**Response schema:** `ListBodiesResponse`

| Field | Type | Description |
|-------|------|-------------|
| `bodies` | `array[BodyResponse]` | List of bodies |

**BodyResponse fields:**

| Field | Type | Description |
|-------|------|-------------|
| `id` | `string` | Unique body identifier |
| `name` | `string` | Human-readable name |
| `image` | `string` | OCI container image (e.g., `ubuntu:22.04`) |
| `state` | `string` | Current body state |
| `node_id` | `string` | ID of the orchestrator node the body runs on (omitted if not running) |
| `ports` | `object` | Map of port name to `PortInfo` (omitted if not running) |
| `resources` | `ResourceSpec` | CPU and memory allocation |
| `health` | `HealthCheckSpec` | Health check configuration (omitted if not configured) |
| `uptime_seconds` | `integer` | Seconds since the body started (omitted if not running) |
| `created_at` | `string` | RFC 3339 timestamp of creation |
| `started_at` | `string` | RFC 3339 timestamp of last start (omitted if never started) |

**PortInfo fields:**

| Field | Type | Description |
|-------|------|-------------|
| `host_port` | `integer` | Host-side port number |
| `domain` | `string` | Public domain or subdomain (if exposed) |

**Status codes:**

| Code | Description |
|------|-------------|
| 200 | Success |

**Example:**

```bash
curl http://localhost:8080/api/v1/bodies \
  -H "Authorization: Bearer <your-auth-token>"
```

Response:

```json
{
  "bodies": [
    {
      "id": "a1b2c3d4",
      "name": "my-agent",
      "image": "ubuntu:22.04",
      "state": "Running",
      "node_id": "node-01",
      "ports": {
        "http": {
          "host_port": 8081,
          "domain": "my-agent.example.com"
        }
      },
      "resources": {
        "cpu_mhz": 1024,
        "memory_mb": 512
      },
      "uptime_seconds": 3600,
      "created_at": "2026-05-01T12:00:00Z",
      "started_at": "2026-05-01T12:05:00Z"
    }
  ]
}
```

---

### POST /api/v1/bodies

Create a new body. Provisions a container from the specified image and returns the body ID and initial state.

**Auth:** Bearer token

**Request schema:** `CreateBodyRequest`

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | `string` | Yes | Human-readable name for the body |
| `image` | `string` | Yes | OCI container image (e.g., `ubuntu:22.04`) |
| `ports` | `array[PortSpec]` | No | Port mappings to expose |
| `volume_mount` | `VolumeMountSpec` | No | Volume mount configuration |
| `command` | `array[string]` | No | Command to run on start (overrides image entrypoint) |
| `env` | `object` | No | Environment variables as key-value pairs |
| `resources` | `ResourceSpec` | No | CPU and memory limits |
| `health_check` | `HealthCheckSpec` | No | Health check configuration |

**PortSpec fields:**

| Field | Type | Description |
|-------|------|-------------|
| `name` | `string` | Port name (e.g., `"http"`, `"ssh"`) |
| `container_port` | `integer` | Port number inside the container |
| `expose` | `boolean` | Whether to expose publicly via ingress |
| `protocol` | `string` | Protocol: `"http"` or `"tcp"` |

**VolumeMountSpec fields:**

| Field | Type | Description |
|-------|------|-------------|
| `container_path` | `string` | Path inside the container to mount the volume |

**ResourceSpec fields:**

| Field | Type | Description |
|-------|------|-------------|
| `cpu_mhz` | `integer` | CPU limit in MHz |
| `memory_mb` | `integer` | Memory limit in megabytes |

**HealthCheckSpec fields:**

| Field | Type | Description |
|-------|------|-------------|
| `type` | `string` | `"http"` or `"tcp"` |
| `path` | `string` | HTTP path for `http` type checks (e.g., `"/health"`) |
| `port` | `string` | Port name or number to check |
| `interval_seconds` | `integer` | Check interval in seconds |

**Response schema:** `CreateBodyResponse`

| Field | Type | Description |
|-------|------|-------------|
| `id` | `string` | Unique body identifier |
| `state` | `string` | Initial body state |
| `message` | `string` | Status message (e.g., `"Body created"`) |

**Status codes:**

| Code | Description |
|------|-------------|
| 201 | Body created successfully |
| 400 | Bad request (missing required fields, malformed JSON) |

**Example:**

```bash
curl -X POST http://localhost:8080/api/v1/bodies \
  -H "Authorization: Bearer <your-auth-token>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "my-agent",
    "image": "ubuntu:22.04",
    "command": ["sleep", "infinity"],
    "env": {
      "LOG_LEVEL": "debug"
    },
    "resources": {
      "cpu_mhz": 1024,
      "memory_mb": 512
    },
    "ports": [
      {
        "name": "http",
        "container_port": 8080,
        "expose": true,
        "protocol": "http"
      }
    ],
    "health_check": {
      "type": "http",
      "path": "/health",
      "port": "http",
      "interval_seconds": 30
    }
  }'
```

Response:

```json
{
  "id": "a1b2c3d4",
  "state": "Created",
  "message": "Body created"
}
```

---

### GET /api/v1/bodies/{id}

Get detailed information about a specific body by ID. Returns the full `BodyResponse` for that body.

**Auth:** Bearer token

**Path parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `id` | `string` | Body ID to retrieve |

**Response schema:** `BodyResponse`

(See [GET /api/v1/bodies](#get-apiv1bodies) for `BodyResponse` field documentation.)

**Status codes:**

| Code | Description |
|------|-------------|
| 200 | Success |
| 404 | Body not found |

**Example:**

```bash
curl http://localhost:8080/api/v1/bodies/a1b2c3d4 \
  -H "Authorization: Bearer <your-auth-token>"
```

Response:

```json
{
  "id": "a1b2c3d4",
  "name": "my-agent",
  "image": "ubuntu:22.04",
  "state": "Running",
  "node_id": "node-01",
  "ports": {
    "http": {
      "host_port": 8081,
      "domain": "my-agent.example.com"
    }
  },
  "resources": {
    "cpu_mhz": 1024,
    "memory_mb": 512
  },
  "uptime_seconds": 3600,
  "created_at": "2026-05-01T12:00:00Z",
  "started_at": "2026-05-01T12:05:00Z"
}
```

---

### POST /api/v1/bodies/{id}/stop

Stop a running or starting body. Sends a stop signal to the container with a 30-second timeout. The body transitions to `"stopping"` and eventually to `"stopped"`.

**Auth:** Bearer token

**Path parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `id` | `string` | Body ID to stop |

**Prerequisite:** Body state must be `"Running"` or `"Starting"`.

**Response schema:** `ActionResponse`

| Field | Type | Description |
|-------|------|-------------|
| `id` | `string` | Body ID |
| `state` | `string` | New body state (`"stopping"`) |

**Status codes:**

| Code | Description |
|------|-------------|
| 200 | Stop initiated |
| 404 | Body not found |
| 409 | Body is not in `"Running"` or `"Starting"` state |

**Example:**

```bash
curl -X POST http://localhost:8080/api/v1/bodies/a1b2c3d4/stop \
  -H "Authorization: Bearer <your-auth-token>"
```

Response:

```json
{
  "id": "a1b2c3d4",
  "state": "stopping"
}
```

---

### POST /api/v1/bodies/{id}/start

Start a stopped body. The body transitions from `"Stopped"` to `"starting"` and then to `"Running"`.

**Auth:** Bearer token

**Path parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `id` | `string` | Body ID to start |

**Prerequisite:** Body state must be `"Stopped"`.

**Response schema:** `ActionResponse`

| Field | Type | Description |
|-------|------|-------------|
| `id` | `string` | Body ID |
| `state` | `string` | New body state (`"starting"`) |

**Status codes:**

| Code | Description |
|------|-------------|
| 200 | Start initiated |
| 404 | Body not found |
| 409 | Body is not in `"Stopped"` state |

**Example:**

```bash
curl -X POST http://localhost:8080/api/v1/bodies/a1b2c3d4/start \
  -H "Authorization: Bearer <your-auth-token>"
```

Response:

```json
{
  "id": "a1b2c3d4",
  "state": "starting"
}
```

---

### DELETE /api/v1/bodies/{id}

Destroy a body. Removes the container and deletes the store record. The body state is set to `"destroyed"`.

**Auth:** Bearer token

**Path parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `id` | `string` | Body ID to destroy |

**Response schema:** `ActionResponse`

| Field | Type | Description |
|-------|------|-------------|
| `id` | `string` | Body ID |
| `state` | `string` | Final body state (`"destroyed"`) |

**Status codes:**

| Code | Description |
|------|-------------|
| 200 | Body destroyed |
| 404 | Body not found |

**Example:**

```bash
curl -X DELETE http://localhost:8080/api/v1/bodies/a1b2c3d4 \
  -H "Authorization: Bearer <your-auth-token>"
```

Response:

```json
{
  "id": "a1b2c3d4",
  "state": "destroyed"
}
```

---

### GET /api/v1/nodes

List orchestrator nodes in the substrate pool. Returns node identity, capacity, and status. This endpoint depends on the orchestrator adapter supporting `NodeLister`; returns `501 Not Implemented` if the adapter does not support it.

**Auth:** Bearer token

**Response schema:** `ListNodesResponse`

| Field | Type | Description |
|-------|------|-------------|
| `nodes` | `array[NodeResponse]` | List of orchestrator nodes |

**NodeResponse fields:**

| Field | Type | Description |
|-------|------|-------------|
| `id` | `string` | Unique node identifier |
| `name` | `string` | Human-readable node name |
| `address` | `string` | Network address of the node |
| `state` | `string` | Node state (e.g., `"ready"`, `"down"`) |
| `capacity` | `CapacityInfo` | Available compute resources |
| `bodies_count` | `integer` | Number of bodies running on this node |
| `provider` | `string` | Substrate provider name (omitted if unknown) |
| `region` | `string` | Geographic region (omitted if unknown) |
| `last_seen_at` | `string` | RFC 3339 timestamp of last contact |

**CapacityInfo fields:**

| Field | Type | Description |
|-------|------|-------------|
| `cpu_mhz` | `integer` | Total CPU capacity in MHz |
| `memory_mb` | `integer` | Total memory capacity in megabytes |
| `disk_gb` | `integer` | Total disk capacity in gigabytes |

**Status codes:**

| Code | Description |
|------|-------------|
| 200 | Success |
| 501 | Node listing not supported by this orchestrator adapter |
| 502 | Orchestrator backend unreachable |

**Example:**

```bash
curl http://localhost:8080/api/v1/nodes \
  -H "Authorization: Bearer <your-auth-token>"
```

Response:

```json
{
  "nodes": [
    {
      "id": "node-01",
      "name": "nomad-client-01",
      "address": "10.0.1.10:4646",
      "state": "ready",
      "capacity": {
        "cpu_mhz": 4096,
        "memory_mb": 8192,
        "disk_gb": 100
      },
      "bodies_count": 2,
      "provider": "nomad",
      "region": "us-east-1",
      "last_seen_at": "2026-05-01T14:30:00Z"
    }
  ]
}
```

## Body State Machine

Bodies transition through the following states. Each state transition is validated before execution.

```
    +---------+
    | Created |  (initial state after POST /api/v1/bodies)
    +----+----+
         |
    +----v----+
    | Starting|  (after start, or on first provision)
    +----+----+
         |
    +----v----+
    | Running |  (container is running and healthy)
    +----+----+
         |
    +----v----+
    | Stopping|  (after POST /api/v1/bodies/{id}/stop)
    +----+----+
         |
    +----v----+
    | Stopped |  (container has exited)
    +----+----+
         |
         | (POST /start -> back to Starting)
         v
    (cycle)

Other states:
  - Error   : Recoverable failure state
  - Migrating : Snapshot-transfer-import cycle active
  - Destroyed : Terminal state (after DELETE)
```

## Endpoint Summary

| Method | Route | Auth | Description |
|--------|-------|------|-------------|
| `GET` | `/healthz` | No | Daemon health check |
| `GET` | `/api/v1/bodies` | Bearer | List all bodies |
| `POST` | `/api/v1/bodies` | Bearer | Create a new body |
| `GET` | `/api/v1/bodies/{id}` | Bearer | Get body by ID |
| `POST` | `/api/v1/bodies/{id}/stop` | Bearer | Stop a running/starting body |
| `POST` | `/api/v1/bodies/{id}/start` | Bearer | Start a stopped body |
| `DELETE` | `/api/v1/bodies/{id}` | Bearer | Destroy a body |
| `GET` | `/api/v1/nodes` | Bearer | List orchestrator nodes |
