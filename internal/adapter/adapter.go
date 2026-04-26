// Package adapter defines the SubstrateAdapter interface for substrate-agnostic
// body provisioning. Implementations include Docker, Nomad, and sandbox providers.
package adapter

import (
	"context"
	"io"
	"time"
)

// Handle is a substrate-specific body instance identifier.
type Handle string

// BodyState represents the lifecycle state of a body.
type BodyState string

const (
	StateCreated   BodyState = "Created"
	StateStarting  BodyState = "Starting"
	StateRunning   BodyState = "Running"
	StateStopping  BodyState = "Stopping"
	StateStopped   BodyState = "Stopped"
	StateError     BodyState = "Error"
	StateMigrating BodyState = "Migrating"
	StateDestroyed BodyState = "Destroyed"
)

// BodySpec defines the desired state of a body at creation time.
type BodySpec struct {
	Image     string
	Workdir   string
	Env       map[string]string
	Cmd       []string
	MemoryMB  int
	CPUShares int
}

// BodyStatus represents the current status of a running body.
type BodyStatus struct {
	State      BodyState
	Uptime     time.Duration
	MemoryMB   int64
	CPUPercent float64
	StartedAt  time.Time
}

// StopOpts controls how a body is stopped.
type StopOpts struct {
	Signal  string
	Timeout time.Duration
}

// ExecResult contains the output of a command executed inside a body.
type ExecResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// AdapterCapabilities describes which optional verbs an adapter supports.
type AdapterCapabilities struct {
	ExportFilesystem bool
	ImportFilesystem bool
	Inspect          bool
}

// ImportOpts controls filesystem import behavior.
type ImportOpts struct {
	Overwrite bool
}

// ContainerMetadata contains metadata about a container.
type ContainerMetadata struct {
	Image    string
	Env      map[string]string
	Cmd      []string
	Workdir  string
	Platform string
}

// SubstrateAdapter defines the interface for managing body instances on a substrate.
// Required verbs: Create, Start, Stop, Destroy, GetStatus, Exec
// Optional verbs: ExportFilesystem, ImportFilesystem, Inspect
type SubstrateAdapter interface {
	Create(ctx context.Context, spec BodySpec) (Handle, error)
	Start(ctx context.Context, id Handle) error
	Stop(ctx context.Context, id Handle, opts StopOpts) error
	Destroy(ctx context.Context, id Handle) error
	GetStatus(ctx context.Context, id Handle) (BodyStatus, error)
	Exec(ctx context.Context, id Handle, cmd []string) (ExecResult, error)

	// Optional verbs
	ExportFilesystem(ctx context.Context, id Handle) (io.ReadCloser, error)
	ImportFilesystem(ctx context.Context, id Handle, tarball io.Reader, opts ImportOpts) error
	Inspect(ctx context.Context, id Handle) (ContainerMetadata, error)
	Capabilities() AdapterCapabilities
}
