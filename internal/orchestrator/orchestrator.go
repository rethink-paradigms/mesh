// Package orchestrator defines the OrchestratorAdapter interface for managing
// body lifecycle on compute substrates, and a registry for adapter discovery.
package orchestrator

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"
)

// Handle is an opaque string identifier for a body instance.
// No routing information is encoded in the handle.
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

// OrchestratorAdapter defines the interface for managing body lifecycle
// on a compute substrate.
type OrchestratorAdapter interface {
	ScheduleBody(ctx context.Context, spec BodySpec) (Handle, error)
	StartBody(ctx context.Context, id Handle) error
	StopBody(ctx context.Context, id Handle) error
	DestroyBody(ctx context.Context, id Handle) error
	GetBodyStatus(ctx context.Context, id Handle) (BodyStatus, error)
	Name() string
	IsHealthy(ctx context.Context) bool
}

// Registry provides thread-safe registration and lookup of OrchestratorAdapter
// implementations using the database/sql pattern.
type Registry struct {
	mu       sync.RWMutex
	adapters map[string]OrchestratorAdapter
}

// NewRegistry creates a new empty Registry.
func NewRegistry() *Registry {
	return &Registry{
		adapters: make(map[string]OrchestratorAdapter),
	}
}

// ErrNotFound is returned when an adapter is not found in the registry.
// The error message includes the names of available adapters.
type errNotFound struct {
	name     string
	available []string
}

func (e *errNotFound) Error() string {
	if len(e.available) == 0 {
		return fmt.Sprintf("orchestrator adapter %q not found (no adapters registered)", e.name)
	}
	return fmt.Sprintf("orchestrator adapter %q not found; available: %v", e.name, e.available)
}

// Register adds a named adapter to the registry.
// Returns an error if the name is already registered.
func (r *Registry) Register(name string, adapter OrchestratorAdapter) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.adapters[name]; exists {
		return fmt.Errorf("orchestrator adapter %q already registered", name)
	}
	r.adapters[name] = adapter
	return nil
}

// Open returns the adapter registered under the given name.
// Returns ErrNotFound if no adapter is registered with that exact name.
func (r *Registry) Open(name string) (OrchestratorAdapter, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	adapter, ok := r.adapters[name]
	if !ok {
		return nil, &errNotFound{name: name, available: r.listNames()}
	}
	return adapter, nil
}

// List returns the names of all registered adapters, sorted alphabetically.
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.listNames()
}

// listNames returns sorted names without holding the lock.
// Caller must hold at least a read lock.
func (r *Registry) listNames() []string {
	names := make([]string, 0, len(r.adapters))
	for name := range r.adapters {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// DefaultRegistry is the package-level default registry.
var DefaultRegistry = NewRegistry()

// Register registers an adapter with the default registry.
func Register(name string, adapter OrchestratorAdapter) error {
	return DefaultRegistry.Register(name, adapter)
}

// Open returns an adapter from the default registry.
func Open(name string) (OrchestratorAdapter, error) {
	return DefaultRegistry.Open(name)
}

// List returns all adapter names from the default registry.
func List() []string {
	return DefaultRegistry.List()
}
