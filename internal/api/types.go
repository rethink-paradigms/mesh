// Package api defines request/response DTOs and error contracts
// for the Mesh daemon REST API.
package api

// CreateBodyRequest is the request payload for POST /api/v1/bodies.
type CreateBodyRequest struct {
	Name        string            `json:"name"`
	Image       string            `json:"image"`
	Ports       []PortSpec        `json:"ports,omitempty"`
	VolumeMount *VolumeMountSpec  `json:"volume_mount,omitempty"`
	Command     []string          `json:"command,omitempty"`
	Env         map[string]string `json:"env,omitempty"`
	Resources   ResourceSpec      `json:"resources,omitempty"`
	HealthCheck *HealthCheckSpec  `json:"health_check,omitempty"`
}

// PortSpec describes a single port mapping for a body.
type PortSpec struct {
	Name          string `json:"name"`
	ContainerPort int    `json:"container_port"`
	Expose        bool   `json:"expose"`
	Protocol      string `json:"protocol"` // "http" or "tcp"
}

// VolumeMountSpec describes a volume mount for a body.
type VolumeMountSpec struct {
	ContainerPath string `json:"container_path"`
}

// ResourceSpec describes compute resource limits for a body.
type ResourceSpec struct {
	CPUMHZ   int `json:"cpu_mhz"`
	MemoryMB int `json:"memory_mb"`
}

// HealthCheckSpec describes a health check configuration for a body.
type HealthCheckSpec struct {
	Type            string `json:"type"` // "http" or "tcp"
	Path            string `json:"path,omitempty"`
	Port            string `json:"port"`
	IntervalSeconds int    `json:"interval_seconds"`
}

// BodyResponse is the response payload for a single body.
type BodyResponse struct {
	ID            string             `json:"id"`
	Name          string             `json:"name"`
	Image         string             `json:"image"`
	State         string             `json:"state"`
	NodeID        string             `json:"node_id,omitempty"`
	Ports         map[string]PortInfo `json:"ports,omitempty"`
	Resources     ResourceSpec       `json:"resources"`
	Health        *HealthCheckSpec   `json:"health,omitempty"`
	UptimeSeconds int64              `json:"uptime_seconds,omitempty"`
	CreatedAt     string             `json:"created_at"`
	StartedAt     string             `json:"started_at,omitempty"`
}

// PortInfo describes port information for a running body.
type PortInfo struct {
	HostPort int    `json:"host_port"`
	Domain   string `json:"domain,omitempty"`
}

// ListBodiesResponse is the response payload for GET /api/v1/bodies.
type ListBodiesResponse struct {
	Bodies []BodyResponse `json:"bodies"`
}

// CreateBodyResponse is the response payload for POST /api/v1/bodies.
type CreateBodyResponse struct {
	ID      string `json:"id"`
	State   string `json:"state"`
	Message string `json:"message"`
}

// ActionResponse is the response payload for body actions (start/stop/restart/snapshot).
type ActionResponse struct {
	ID    string `json:"id"`
	State string `json:"state"`
}

// NodeResponse is the response payload for a single node.
type NodeResponse struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Address     string       `json:"address"`
	State       string       `json:"state"`
	Capacity    CapacityInfo `json:"capacity"`
	BodiesCount int          `json:"bodies_count"`
	Provider    string       `json:"provider,omitempty"`
	Region      string       `json:"region,omitempty"`
	LastSeenAt  string       `json:"last_seen_at"`
}

// CapacityInfo describes node capacity information.
type CapacityInfo struct {
	CPUMHZ   int `json:"cpu_mhz"`
	MemoryMB int `json:"memory_mb"`
	DiskGB   int `json:"disk_gb"`
}

// ListNodesResponse is the response payload for GET /api/v1/nodes.
type ListNodesResponse struct {
	Nodes []NodeResponse `json:"nodes"`
}

// HealthzResponse is the response payload for GET /healthz.
type HealthzResponse struct {
	Status          string `json:"status"`
	Version         string `json:"version"`
	NomadConnected  bool   `json:"nomad_connected"`
	ConsulConnected bool   `json:"consul_connected"`
	BodiesCount     int    `json:"bodies_count"`
	NodesCount      int    `json:"nodes_count"`
}
