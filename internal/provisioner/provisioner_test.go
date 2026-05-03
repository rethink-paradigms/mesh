package provisioner

import (
	"context"
	"strings"
	"sync"
	"testing"
)

// mockAdapter is a minimal ProvisionerAdapter implementation for testing.
type mockAdapter struct {
	name string
}

func (m *mockAdapter) CreateMachine(ctx context.Context, spec MachineSpec, userData string) (MachineID, error) {
	return MachineID("mock-" + m.name), nil
}

func (m *mockAdapter) DestroyMachine(ctx context.Context, id MachineID) error {
	return nil
}

func (m *mockAdapter) GetMachineStatus(ctx context.Context, id MachineID) (MachineStatus, error) {
	return MachineStatus{State: "running", ID: id}, nil
}

func (m *mockAdapter) ListMachines(ctx context.Context) ([]MachineInfo, error) {
	return nil, nil
}

func (m *mockAdapter) Name() string {
	return m.name
}

func (m *mockAdapter) IsHealthy(ctx context.Context) bool {
	return true
}

func TestEmptyRegistry(t *testing.T) {
	r := NewRegistry()

	// List() on empty registry should return empty slice, not nil
	names := r.List()
	if names == nil {
		t.Error("List() on empty registry returned nil, want empty slice")
	}
	if len(names) != 0 {
		t.Errorf("List() returned %d names, want 0", len(names))
	}

	// Open("anything") should return errNotFound
	_, err := r.Open("anything")
	if err == nil {
		t.Error("Open(\"anything\") on empty registry returned nil error, want errNotFound")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("Open() error message missing 'not found': %v", err)
	}
	if !strings.Contains(err.Error(), "no adapters registered") {
		t.Errorf("Open() error message missing 'no adapters registered': %v", err)
	}
}

func TestRegistryWithMock(t *testing.T) {
	r := NewRegistry()
	mock := &mockAdapter{name: "hetzner"}

	// Register the mock adapter
	if err := r.Register("hetzner", mock); err != nil {
		t.Fatalf("Register() failed: %v", err)
	}

	// Open should return the registered adapter
	adp, err := r.Open("hetzner")
	if err != nil {
		t.Fatalf("Open(\"hetzner\") failed: %v", err)
	}
	if adp == nil {
		t.Fatal("Open(\"hetzner\") returned nil adapter")
	}
	if adp.Name() != "hetzner" {
		t.Errorf("adapter.Name() = %q, want %q", adp.Name(), "hetzner")
	}

	// List should return ["hetzner"]
	names := r.List()
	if len(names) != 1 {
		t.Fatalf("List() returned %d names, want 1", len(names))
	}
	if names[0] != "hetzner" {
		t.Errorf("List()[0] = %q, want %q", names[0], "hetzner")
	}
}

func TestRegistryDuplicateRegister(t *testing.T) {
	r := NewRegistry()
	mock1 := &mockAdapter{name: "hetzner"}
	mock2 := &mockAdapter{name: "hetzner-v2"}

	// First registration should succeed
	if err := r.Register("hetzner", mock1); err != nil {
		t.Fatalf("first Register() failed: %v", err)
	}

	// Second registration with same name should fail
	err := r.Register("hetzner", mock2)
	if err == nil {
		t.Error("duplicate Register() returned nil error, want error")
	}
	if !strings.Contains(err.Error(), "already registered") {
		t.Errorf("duplicate Register() error missing 'already registered': %v", err)
	}
}

func TestRegistryList(t *testing.T) {
	r := NewRegistry()

	// Register multiple adapters out of order
	adapters := []string{"fly", "aws", "hetzner", "gcp"}
	for _, name := range adapters {
		if err := r.Register(name, &mockAdapter{name: name}); err != nil {
			t.Fatalf("Register(%q) failed: %v", name, err)
		}
	}

	// List should return sorted names
	names := r.List()
	want := []string{"aws", "fly", "gcp", "hetzner"}
	if len(names) != len(want) {
		t.Fatalf("List() returned %d names, want %d", len(names), len(want))
	}
	for i, name := range names {
		if name != want[i] {
			t.Errorf("List()[%d] = %q, want %q", i, name, want[i])
		}
	}
}

func TestRegistryConcurrency(t *testing.T) {
	r := NewRegistry()
	var wg sync.WaitGroup

	// Concurrent registrations
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			name := string(rune('a' + i))
			r.Register(name, &mockAdapter{name: name})
		}(i)
	}

	// Concurrent opens
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			name := string(rune('a' + i))
			r.Open(name)
		}(i)
	}

	// Concurrent lists
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r.List()
		}()
	}

	wg.Wait()

	// Verify all registrations succeeded
	names := r.List()
	if len(names) != 10 {
		t.Errorf("List() returned %d names after concurrent ops, want 10", len(names))
	}
}

func TestDefaultRegistry(t *testing.T) {
	// Ensure DefaultRegistry is a fresh empty registry
	if DefaultRegistry == nil {
		t.Fatal("DefaultRegistry is nil")
	}

	// List on default should be empty (or at least not panic)
	names := List()
	if names == nil {
		t.Error("List() on DefaultRegistry returned nil, want empty slice")
	}

	// Package-level Open should return errNotFound
	_, err := Open("nonexistent")
	if err == nil {
		t.Error("Open(\"nonexistent\") on DefaultRegistry returned nil error")
	}
}

func TestErrNotFoundIncludesAvailable(t *testing.T) {
	r := NewRegistry()
	r.Register("aws", &mockAdapter{name: "aws"})
	r.Register("gcp", &mockAdapter{name: "gcp"})

	_, err := r.Open("azure")
	if err == nil {
		t.Fatal("Open(\"azure\") returned nil error")
	}

	msg := err.Error()
	if !strings.Contains(msg, "azure") {
		t.Errorf("error message missing requested name: %s", msg)
	}
	if !strings.Contains(msg, "aws") || !strings.Contains(msg, "gcp") {
		t.Errorf("error message missing available names: %s", msg)
	}
}
