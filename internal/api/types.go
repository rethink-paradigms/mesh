// Package api defines request/response DTOs and error contracts
// for the Mesh daemon REST API.
package api

import (
	"context"
	"encoding/json"
)

// CreateBodyRequest is the request payload for POST /api/v1/bodies.
// @Description Request payload for creating a new body
type CreateBodyRequest struct {
	Name        string            `json:"name" example:"my-agent-body"`
	Image       string            `json:"image" example:"ghcr.io/rethink-paradigms/hermes:latest"`
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
// @Description Response payload for a single body
type BodyResponse struct {
	ID            string              `json:"id" example:"body_abc123"`
	Name          string              `json:"name" example:"my-agent-body"`
	Image         string              `json:"image"`
	State         string              `json:"state" example:"running"`
	NodeID        string              `json:"node_id,omitempty" example:"node_xyz789"`
	Ports         map[string]PortInfo `json:"ports,omitempty"`
	Resources     ResourceSpec        `json:"resources"`
	Health        *HealthCheckSpec    `json:"health,omitempty"`
	UptimeSeconds int64               `json:"uptime_seconds,omitempty"`
	CreatedAt     string              `json:"created_at"`
	StartedAt     string              `json:"started_at,omitempty"`
}

// PortInfo describes port information for a running body.
type PortInfo struct {
	HostPort int    `json:"host_port"`
	Domain   string `json:"domain,omitempty"`
}

// ListBodiesResponse is the response payload for GET /api/v1/bodies.
// @Description Response payload for listing all bodies
type ListBodiesResponse struct {
	Bodies []BodyResponse `json:"bodies"`
}

// CreateBodyResponse is the response payload for POST /api/v1/bodies.
// @Description Response payload after creating a body
type CreateBodyResponse struct {
	ID      string `json:"id"`
	State   string `json:"state"`
	Message string `json:"message"`
}

// ActionResponse is the response payload for body actions (start/stop/restart/snapshot).
// @Description Response payload for body actions (start/stop/destroy)
type ActionResponse struct {
	ID    string `json:"id"`
	State string `json:"state"`
}

// BulkDestroyBodiesRequest is the request payload for DELETE /api/v1/bodies.
type BulkDestroyBodiesRequest struct {
	IDs []string `json:"ids"`
}

// BulkDestroyBodiesResponse is the response payload for bulk body destroy.
type BulkDestroyBodiesResponse struct {
	Destroyed int               `json:"destroyed"`
	Failed    int               `json:"failed"`
	Failures  []BulkDestroyFailure `json:"failures,omitempty"`
}

// BulkDestroyFailure describes a single body that could not be destroyed.
type BulkDestroyFailure struct {
	ID    string `json:"id"`
	Error string `json:"error"`
}

// NodeResponse is the response payload for a single node.
// @Description Response payload for a single node
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
// @Description Response payload for listing all nodes
type ListNodesResponse struct {
	Nodes []NodeResponse `json:"nodes"`
}

// HealthzResponse is the response payload for GET /healthz.
// @Description Response payload for health check
type HealthzResponse struct {
	Status                       string `json:"status"`
	Version                      string `json:"version"`
	Commit                       string `json:"commit"`
	BuildTime                    string `json:"build_time"`
	NomadConnected               bool   `json:"nomad_connected"`
	BodiesCount                  int    `json:"bodies_count"`
	NodesCount                   int    `json:"nodes_count"`
	OrchestratorConnected        bool   `json:"orchestrator_connected"`
	GatewayURL                   string `json:"gateway_url"`
	GatewayReachable             bool   `json:"gateway_reachable"`
	LastHeartbeatSuccess         string `json:"last_heartbeat_success,omitempty"`
	HeartbeatConsecutiveFailures int    `json:"heartbeat_consecutive_failures"`
	HeartbeatEnabled             bool   `json:"heartbeat_enabled"`
	SQLiteHealthy                bool   `json:"sqlite_healthy"`
	StuckStartingCount           int    `json:"stuck_starting_count"`
}

// CapabilitiesResponse is the response payload for GET /api/v1/capabilities.
// @Description Response payload for daemon capabilities
type CapabilitiesResponse struct {
	Version       string                   `json:"version"`
	Tier          string                   `json:"tier"`
	Orchestrators []OrchestratorCapability `json:"orchestrators"`
	Providers     json.RawMessage          `json:"providers"`
	Features      map[string]bool          `json:"features"`
	Limits        CapabilityLimits         `json:"limits"`
}

// OrchestratorCapability describes a registered orchestrator adapter and its health.
type OrchestratorCapability struct {
	Name    string `json:"name"`
	Healthy bool   `json:"healthy"`
}

// CapabilityLimits defines hard limits for the daemon.
type CapabilityLimits struct {
	MaxBodies    int `json:"max_bodies"`
	MaxSnapshots int `json:"max_snapshots"`
}

// StatusResponse is the response payload for GET /api/v1/status.
// @Description Response payload for daemon status
type StatusResponse struct {
	Daemon   DaemonStatusInfo   `json:"daemon"`
	Tier     string             `json:"tier"`
	Bodies   BodiesStatusInfo   `json:"bodies"`
	Ports    PortsStatusInfo    `json:"ports"`
	Ingress  IngressStatusInfo  `json:"ingress"`
	Capacity CapacityStatusInfo `json:"capacity"`
}

// DaemonStatusInfo describes the daemon itself.
type DaemonStatusInfo struct {
	Version   string `json:"version"`
	UptimeSec int64  `json:"uptime_seconds"`
	StartTime string `json:"start_time"`
}

// BodiesStatusInfo describes the aggregate state of all bodies.
type BodiesStatusInfo struct {
	Total   int              `json:"total"`
	Running int              `json:"running"`
	Stopped int              `json:"stopped"`
	Error   int              `json:"error"`
	List    []BodyStatusItem `json:"list"`
}

// BodyStatusItem is a summary entry for a single body in the status list.
type BodyStatusItem struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	State string `json:"state"`
}

// PortsStatusInfo describes port pool usage.
type PortsStatusInfo struct {
	Used      int `json:"used"`
	Free      int `json:"free"`
	PoolStart int `json:"pool_start"`
	PoolEnd   int `json:"pool_end"`
}

// IngressStatusInfo describes the ingress router state.
type IngressStatusInfo struct {
	RouteCount int `json:"route_count"`
}

// CapacityStatusInfo describes daemon host capacity.
type CapacityStatusInfo struct {
	// CPUPercent is always 0.0 — real-time CPU sampling not implemented
	CPUPercent    float64 `json:"cpu_percent"`
	MemoryMBUsed  int64   `json:"memory_mb_used"`
	MemoryMBTotal int64   `json:"memory_mb_total"`
	DiskGBUsed    float64 `json:"disk_gb_used"`
	DiskGBTotal   float64 `json:"disk_gb_total"`
}

// ─── Registry ──────────────────────────────────────────────────────────────────

// S3RegistryConfig is the JSON payload for configuring S3 on a running daemon.
type S3RegistryConfig struct {
	Bucket          string `json:"bucket"`
	Region          string `json:"region"`
	Endpoint        string `json:"endpoint,omitempty"`
	AccessKeyID     string `json:"access_key_id,omitempty"`
	SecretAccessKey string `json:"secret_access_key,omitempty"`
}

// RegistryManager is the interface for hot-swapping the S3 registry at runtime.
// The Daemon implements this and passes it to the API handlers.
type RegistryManager interface {
	// ConfigureS3 validates S3 credentials and sets the registry plugin.
	// Thread-safe — existing migrations are unaffected.
	ConfigureS3(ctx context.Context, cfg S3RegistryConfig) error
	// DisconnectS3 clears the registry plugin. Falls back to same-machine migration.
	DisconnectS3(ctx context.Context) error
	// RegistryStatus returns the current registry state.
	RegistryStatus(ctx context.Context) map[string]any
}

// RegistryStatusResponse is the response payload for GET /api/v1/registry/status.
type RegistryStatusResponse struct {
	Configured bool   `json:"configured"`
	Type       string `json:"type"`
	Bucket     string `json:"bucket,omitempty"`
	Region     string `json:"region,omitempty"`
	Healthy    bool   `json:"healthy"`
}
