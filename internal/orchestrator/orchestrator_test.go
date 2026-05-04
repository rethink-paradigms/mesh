package orchestrator

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"
)

// mockAdapter is a minimal OrchestratorAdapter implementation for testing.
type mockAdapter struct {
	mu      sync.Mutex
	name    string
	healthy bool
}

func newMockAdapter(name string) *mockAdapter {
	return &mockAdapter{name: name, healthy: true}
}

func (m *mockAdapter) ScheduleBody(ctx context.Context, spec BodySpec) (Handle, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return Handle(m.name + ":" + spec.Image), nil
}

func (m *mockAdapter) StartBody(ctx context.Context, id Handle) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return nil
}

func (m *mockAdapter) StopBody(ctx context.Context, id Handle) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return nil
}

func (m *mockAdapter) DestroyBody(ctx context.Context, id Handle) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return nil
}

func (m *mockAdapter) GetBodyStatus(ctx context.Context, id Handle) (BodyStatus, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return BodyStatus{State: StateRunning}, nil
}

func (m *mockAdapter) Name() string {
	return m.name
}

func (m *mockAdapter) IsHealthy(ctx context.Context) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.healthy
}

func TestRegistryRegisterAndOpen(t *testing.T) {
	reg := NewRegistry()
	mock := newMockAdapter("nomad")

	if err := reg.Register("nomad", mock); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	got, err := reg.Open("nomad")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	if got != mock {
		t.Fatalf("Open returned wrong adapter: got %p, want %p", got, mock)
	}

	// Verify the adapter works
	ctx := context.Background()
	handle, err := got.ScheduleBody(ctx, BodySpec{Image: "test:latest"})
	if err != nil {
		t.Fatalf("ScheduleBody failed: %v", err)
	}
	if !strings.HasPrefix(string(handle), "nomad:") {
		t.Fatalf("unexpected handle: %s", handle)
	}
}

func TestRegistryOpenNotFound(t *testing.T) {
	reg := NewRegistry()
	mockA := newMockAdapter("alpha")
	mockB := newMockAdapter("beta")

	_ = reg.Register("alpha", mockA)
	_ = reg.Register("beta", mockB)

	_, err := reg.Open("gamma")
	if err == nil {
		t.Fatal("expected error for unknown adapter, got nil")
	}

	msg := err.Error()
	if !strings.Contains(msg, "gamma") {
		t.Fatalf("error message should contain requested name, got: %s", msg)
	}
	if !strings.Contains(msg, "alpha") || !strings.Contains(msg, "beta") {
		t.Fatalf("error message should contain available names, got: %s", msg)
	}
}

func TestRegistryDuplicateRegister(t *testing.T) {
	reg := NewRegistry()
	mock1 := newMockAdapter("nomad")
	mock2 := newMockAdapter("nomad")

	if err := reg.Register("nomad", mock1); err != nil {
		t.Fatalf("first Register failed: %v", err)
	}

	err := reg.Register("nomad", mock2)
	if err == nil {
		t.Fatal("expected error for duplicate registration, got nil")
	}
	if !strings.Contains(err.Error(), "already registered") {
		t.Fatalf("expected 'already registered' in error, got: %s", err.Error())
	}
}

func TestRegistryList(t *testing.T) {
	reg := NewRegistry()
	_ = reg.Register("charlie", newMockAdapter("charlie"))
	_ = reg.Register("alpha", newMockAdapter("alpha"))
	_ = reg.Register("bravo", newMockAdapter("bravo"))

	names := reg.List()
	if len(names) != 3 {
		t.Fatalf("expected 3 names, got %d", len(names))
	}
	want := []string{"alpha", "bravo", "charlie"}
	for i, name := range names {
		if name != want[i] {
			t.Fatalf("List[%d] = %q, want %q", i, name, want[i])
		}
	}
}

func TestRegistryListEmpty(t *testing.T) {
	reg := NewRegistry()
	names := reg.List()
	if names == nil {
		t.Fatal("List on empty registry should return empty slice, not nil")
	}
	if len(names) != 0 {
		t.Fatalf("expected 0 names, got %d", len(names))
	}
}

func TestRegistryOpenEmptyRegistry(t *testing.T) {
	reg := NewRegistry()
	_, err := reg.Open("anything")
	if err == nil {
		t.Fatal("expected error on empty registry, got nil")
	}
	if !strings.Contains(err.Error(), "no adapters registered") {
		t.Fatalf("expected 'no adapters registered' in error, got: %s", err.Error())
	}
}

func TestDefaultRegistry(t *testing.T) {
	mock := newMockAdapter("default-test")
	if err := Register("default-test", mock); err != nil {
		t.Fatalf("Register on default registry failed: %v", err)
	}

	got, err := Open("default-test")
	if err != nil {
		t.Fatalf("Open on default registry failed: %v", err)
	}
	if got != mock {
		t.Fatal("Open on default registry returned wrong adapter")
	}

	names := List()
	found := false
	for _, n := range names {
		if n == "default-test" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("List on default registry missing 'default-test'")
	}
}

func TestRegistryConcurrency(t *testing.T) {
	reg := NewRegistry()
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			name := string(rune('a' + n%26))
			_ = reg.Register(name, newMockAdapter(name))
		}(i)
	}

	wg.Wait()

	// Should have exactly 26 unique adapters
	names := reg.List()
	if len(names) != 26 {
		t.Fatalf("expected 26 unique adapters, got %d", len(names))
	}
}

func TestMockAdapterMethods(t *testing.T) {
	mock := newMockAdapter("test")
	ctx := context.Background()

	handle, err := mock.ScheduleBody(ctx, BodySpec{Image: "img"})
	if err != nil {
		t.Fatalf("ScheduleBody: %v", err)
	}
	if handle == "" {
		t.Fatal("ScheduleBody returned empty handle")
	}

	if err := mock.StartBody(ctx, handle); err != nil {
		t.Fatalf("StartBody: %v", err)
	}
	if err := mock.StopBody(ctx, handle); err != nil {
		t.Fatalf("StopBody: %v", err)
	}
	if err := mock.DestroyBody(ctx, handle); err != nil {
		t.Fatalf("DestroyBody: %v", err)
	}

	status, err := mock.GetBodyStatus(ctx, handle)
	if err != nil {
		t.Fatalf("GetBodyStatus: %v", err)
	}
	if status.State != StateRunning {
		t.Fatalf("unexpected state: %s", status.State)
	}

	if mock.Name() != "test" {
		t.Fatalf("Name = %q, want %q", mock.Name(), "test")
	}

	if !mock.IsHealthy(ctx) {
		t.Fatal("expected healthy mock")
	}
}

func TestBodyTypes(t *testing.T) {
	// Verify BodySpec fields
	spec := BodySpec{
		Image:     "alpine:latest",
		Workdir:   "/app",
		Env:       map[string]string{"KEY": "val"},
		Cmd:       []string{"sh", "-c", "echo hi"},
		MemoryMB:  512,
		CPUShares: 1024,
	}
	if spec.Image != "alpine:latest" {
		t.Fatal("BodySpec.Image mismatch")
	}

	// Verify BodyStatus fields
	status := BodyStatus{
		State:      StateStarting,
		Uptime:     time.Minute,
		MemoryMB:   256,
		CPUPercent: 12.5,
		StartedAt:  time.Now(),
	}
	if status.State != StateStarting {
		t.Fatal("BodyStatus.State mismatch")
	}

	// Verify StopOpts fields
	opts := StopOpts{Signal: "SIGTERM", Timeout: 30 * time.Second}
	if opts.Signal != "SIGTERM" {
		t.Fatal("StopOpts.Signal mismatch")
	}

	// Verify all state constants
	states := []BodyState{
		StateCreated, StateStarting, StateRunning, StateStopping,
		StateStopped, StateError, StateMigrating, StateDestroyed,
	}
	if len(states) != 8 {
		t.Fatalf("expected 8 states, got %d", len(states))
	}
}
