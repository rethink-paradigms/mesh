// Package provisioner defines the ProvisionerAdapter interface for managing
// compute machine lifecycle, and a registry for adapter discovery.
package provisioner

import (
	"context"
	"fmt"
	"sort"
	"sync"
)

// MachineID is an opaque string identifier for a compute machine.
type MachineID string

// MachineSpec defines the desired state of a machine at creation time.
type MachineSpec struct {
	Image     string
	MemoryMB  int
	CPUShares int
	Region    string
}

// MachineStatus represents the current status of a machine.
type MachineStatus struct {
	State string
	ID    MachineID
}

// MachineInfo provides summary information about a machine.
type MachineInfo struct {
	ID    MachineID
	Name  string
	State string
}

// ProvisionerAdapter defines the interface for managing compute machine
// lifecycle on a substrate.
type ProvisionerAdapter interface {
	CreateMachine(ctx context.Context, spec MachineSpec, userData string) (MachineID, error)
	DestroyMachine(ctx context.Context, id MachineID) error
	GetMachineStatus(ctx context.Context, id MachineID) (MachineStatus, error)
	ListMachines(ctx context.Context) ([]MachineInfo, error)
	Name() string
	IsHealthy(ctx context.Context) bool
}

// Registry provides thread-safe registration and lookup of ProvisionerAdapter
// implementations using the database/sql pattern.
type Registry struct {
	mu       sync.RWMutex
	adapters map[string]ProvisionerAdapter
}

// NewRegistry creates a new empty Registry.
func NewRegistry() *Registry {
	return &Registry{
		adapters: make(map[string]ProvisionerAdapter),
	}
}

// errNotFound is returned when an adapter is not found in the registry.
// The error message includes the names of available adapters.
type errNotFound struct {
	name      string
	available []string
}

func (e *errNotFound) Error() string {
	if len(e.available) == 0 {
		return fmt.Sprintf("provisioner adapter %q not found (no adapters registered)", e.name)
	}
	return fmt.Sprintf("provisioner adapter %q not found; available: %v", e.name, e.available)
}

// Register adds a named adapter to the registry.
// Returns an error if the name is already registered.
func (r *Registry) Register(name string, adapter ProvisionerAdapter) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.adapters[name]; exists {
		return fmt.Errorf("provisioner adapter %q already registered", name)
	}
	r.adapters[name] = adapter
	return nil
}

// Open returns the adapter registered under the given name.
// Returns errNotFound if no adapter is registered with that exact name.
func (r *Registry) Open(name string) (ProvisionerAdapter, error) {
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
func Register(name string, adapter ProvisionerAdapter) error {
	return DefaultRegistry.Register(name, adapter)
}

// Open returns an adapter from the default registry.
func Open(name string) (ProvisionerAdapter, error) {
	return DefaultRegistry.Open(name)
}

// List returns all adapter names from the default registry.
func List() []string {
	return DefaultRegistry.List()
}
