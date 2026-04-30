// Package adapter defines the SubstrateAdapter interface for substrate-agnostic
// body provisioning. Implementations include Docker, Nomad, and sandbox providers.
package adapter

import (
	"context"
	"fmt"
	"io"
	"strings"
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

	SubstrateName() string
	IsHealthy(ctx context.Context) bool
}

// MultiAdapter routes SubstrateAdapter calls to named substrate adapters.
// It implements SubstrateAdapter by delegating to a registered adapter.
type MultiAdapter struct {
	adapters map[string]SubstrateAdapter
}

// NewMultiAdapter creates a new empty MultiAdapter.
func NewMultiAdapter() *MultiAdapter {
	return &MultiAdapter{
		adapters: make(map[string]SubstrateAdapter),
	}
}

// Register adds a named adapter to the router.
func (m *MultiAdapter) Register(name string, adapter SubstrateAdapter) {
	m.adapters[name] = adapter
}

// GetAdapter returns the adapter registered under the given name.
func (m *MultiAdapter) GetAdapter(name string) (SubstrateAdapter, error) {
	adapter, ok := m.adapters[name]
	if !ok {
		return nil, fmt.Errorf("adapter %q not found", name)
	}
	return adapter, nil
}

// ListAdapters returns the names of all registered adapters.
func (m *MultiAdapter) ListAdapters() []string {
	names := make([]string, 0, len(m.adapters))
	for name := range m.adapters {
		names = append(names, name)
	}
	return names
}

// Create delegates to the adapter named by spec.Image prefix before colon.
func (m *MultiAdapter) Create(ctx context.Context, spec BodySpec) (Handle, error) {
	adapter, err := m.resolveAdapter(spec.Image)
	if err != nil {
		return "", err
	}
	return adapter.Create(ctx, spec)
}

// Start delegates to the adapter that owns the handle.
func (m *MultiAdapter) Start(ctx context.Context, id Handle) error {
	adapter, err := m.resolveAdapter(string(id))
	if err != nil {
		return err
	}
	return adapter.Start(ctx, id)
}

// Stop delegates to the adapter that owns the handle.
func (m *MultiAdapter) Stop(ctx context.Context, id Handle, opts StopOpts) error {
	adapter, err := m.resolveAdapter(string(id))
	if err != nil {
		return err
	}
	return adapter.Stop(ctx, id, opts)
}

// Destroy delegates to the adapter that owns the handle.
func (m *MultiAdapter) Destroy(ctx context.Context, id Handle) error {
	adapter, err := m.resolveAdapter(string(id))
	if err != nil {
		return err
	}
	return adapter.Destroy(ctx, id)
}

// GetStatus delegates to the adapter that owns the handle.
func (m *MultiAdapter) GetStatus(ctx context.Context, id Handle) (BodyStatus, error) {
	adapter, err := m.resolveAdapter(string(id))
	if err != nil {
		return BodyStatus{}, err
	}
	return adapter.GetStatus(ctx, id)
}

// Exec delegates to the adapter that owns the handle.
func (m *MultiAdapter) Exec(ctx context.Context, id Handle, cmd []string) (ExecResult, error) {
	adapter, err := m.resolveAdapter(string(id))
	if err != nil {
		return ExecResult{}, err
	}
	return adapter.Exec(ctx, id, cmd)
}

// ExportFilesystem delegates to the adapter that owns the handle.
func (m *MultiAdapter) ExportFilesystem(ctx context.Context, id Handle) (io.ReadCloser, error) {
	adapter, err := m.resolveAdapter(string(id))
	if err != nil {
		return nil, err
	}
	return adapter.ExportFilesystem(ctx, id)
}

// ImportFilesystem delegates to the adapter that owns the handle.
func (m *MultiAdapter) ImportFilesystem(ctx context.Context, id Handle, tarball io.Reader, opts ImportOpts) error {
	adapter, err := m.resolveAdapter(string(id))
	if err != nil {
		return err
	}
	return adapter.ImportFilesystem(ctx, id, tarball, opts)
}

// Inspect delegates to the adapter that owns the handle.
func (m *MultiAdapter) Inspect(ctx context.Context, id Handle) (ContainerMetadata, error) {
	adapter, err := m.resolveAdapter(string(id))
	if err != nil {
		return ContainerMetadata{}, err
	}
	return adapter.Inspect(ctx, id)
}

// Capabilities returns the capabilities of the adapter that owns the handle.
func (m *MultiAdapter) Capabilities() AdapterCapabilities {
	return AdapterCapabilities{}
}

// SubstrateName returns "multi" to indicate this is a routing adapter.
func (m *MultiAdapter) SubstrateName() string {
	return "multi"
}

// IsHealthy returns true if at least one registered adapter is healthy.
func (m *MultiAdapter) IsHealthy(ctx context.Context) bool {
	for _, adapter := range m.adapters {
		if adapter.IsHealthy(ctx) {
			return true
		}
	}
	return false
}

// resolveAdapter picks an adapter based on a handle/image string.
// For now, uses the first registered adapter as default.
func (m *MultiAdapter) resolveAdapter(ref string) (SubstrateAdapter, error) {
	if len(m.adapters) == 0 {
		return nil, fmt.Errorf("no adapters registered")
	}
	for name, adapter := range m.adapters {
		if strings.HasPrefix(ref, name) {
			return adapter, nil
		}
	}
	for _, adapter := range m.adapters {
		return adapter, nil
	}
	return nil, fmt.Errorf("no adapter found for %q", ref)
}
