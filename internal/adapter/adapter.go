// Package adapter provides deprecated type aliases for backward compatibility.
// All types re-export orchestrator types. New code should import orchestrator directly.
//
// Deprecated: Use github.com/rethink-paradigms/mesh/internal/orchestrator instead.
package adapter

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/rethink-paradigms/mesh/internal/orchestrator"
)

// Type aliases — re-exported from orchestrator for backward compatibility.
// Deprecated: Use orchestrator.Handle instead.
type Handle = orchestrator.Handle

// Deprecated: Use orchestrator.BodyState instead.
type BodyState = orchestrator.BodyState

// Deprecated: Use orchestrator.StateCreated etc. instead.
const (
	StateCreated   = orchestrator.StateCreated
	StateStarting  = orchestrator.StateStarting
	StateRunning   = orchestrator.StateRunning
	StateStopping  = orchestrator.StateStopping
	StateStopped   = orchestrator.StateStopped
	StateError     = orchestrator.StateError
	StateMigrating = orchestrator.StateMigrating
	StateDestroyed = orchestrator.StateDestroyed
)

// Deprecated: Use orchestrator.BodySpec instead.
type BodySpec = orchestrator.BodySpec

// Deprecated: Use orchestrator.BodyStatus instead.
type BodyStatus = orchestrator.BodyStatus

// Deprecated: Use orchestrator.StopOpts instead.
type StopOpts = orchestrator.StopOpts

// Deprecated: Use orchestrator.ExecResult instead.
type ExecResult = orchestrator.ExecResult

// Deprecated: Use orchestrator.ContainerMetadata instead.
type ContainerMetadata = orchestrator.ContainerMetadata

// ImportOpts controls filesystem import behavior.
// Deprecated: This type is not used by the orchestrator interface.
type ImportOpts struct {
	Overwrite bool
}

// AdapterCapabilities describes which optional verbs an adapter supports.
// Deprecated: Use orchestrator capability interfaces (Exporter, Importer, Inspector, Executor) instead.
type AdapterCapabilities struct {
	ExportFilesystem bool
	ImportFilesystem bool
	Inspect          bool
}

// SubstrateAdapter defines the legacy interface for managing body instances on a substrate.
// Deprecated: Use orchestrator.OrchestratorAdapter and capability interfaces instead.
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
// Deprecated: Use orchestrator.Registry instead.
type MultiAdapter struct {
	adapters map[string]SubstrateAdapter
}

// NewMultiAdapter creates a new empty MultiAdapter.
// Deprecated: Use orchestrator.Registry instead.
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

// Capabilities returns empty capabilities.
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
