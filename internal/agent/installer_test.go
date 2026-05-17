package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/rethink-paradigms/mesh/internal/body"
	"github.com/rethink-paradigms/mesh/internal/ingress"
	"github.com/rethink-paradigms/mesh/internal/orchestrator"
	"github.com/rethink-paradigms/mesh/internal/service"
	"github.com/rethink-paradigms/mesh/internal/store"
)

type mockIngress struct {
	allocPortFunc func(ctx context.Context, containerPort int) (int, error)
	addRouteFunc  func(ctx context.Context, domain, upstream string, port int) error
}

func (m *mockIngress) AddRoute(ctx context.Context, domain, upstream string, port int) error {
	if m.addRouteFunc != nil {
		return m.addRouteFunc(ctx, domain, upstream, port)
	}
	return nil
}
func (m *mockIngress) RemoveRoute(ctx context.Context, domain string) error { return nil }
func (m *mockIngress) ListRoutes(ctx context.Context) ([]ingress.Route, error) {
	return nil, nil
}
func (m *mockIngress) AllocPort(ctx context.Context, containerPort int) (int, error) {
	if m.allocPortFunc != nil {
		return m.allocPortFunc(ctx, containerPort)
	}
	return containerPort + 10000, nil
}
func (m *mockIngress) FreePort(hostPort int) error { return nil }

type mockOrchAdapter struct {
	handle orchestrator.Handle
}

func (m *mockOrchAdapter) ScheduleBody(_ context.Context, _ orchestrator.BodySpec) (orchestrator.Handle, error) {
	if m.handle == "" {
		return "mock-handle", nil
	}
	return m.handle, nil
}
func (m *mockOrchAdapter) StartBody(_ context.Context, _ orchestrator.Handle) error   { return nil }
func (m *mockOrchAdapter) StopBody(_ context.Context, _ orchestrator.Handle) error    { return nil }
func (m *mockOrchAdapter) DestroyBody(_ context.Context, _ orchestrator.Handle) error { return nil }
func (m *mockOrchAdapter) GetBodyStatus(_ context.Context, _ orchestrator.Handle) (orchestrator.BodyStatus, error) {
	return orchestrator.BodyStatus{State: orchestrator.StateRunning}, nil
}
func (m *mockOrchAdapter) Name() string                     { return "mock" }
func (m *mockOrchAdapter) IsHealthy(_ context.Context) bool { return true }

func tempStore(t *testing.T) *store.Store {
	t.Helper()
	f, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("open temp store: %v", err)
	}
	t.Cleanup(func() { f.Close() })
	return f
}

func TestInstallAgent(t *testing.T) {
	s := tempStore(t)
	bm := body.NewBodyManager(s, &mockOrchAdapter{}, "")
	ing := &mockIngress{}

	manifests := map[string]*Descriptor{
		"test-agent": {
			Name:    "test-agent",
			Image:   "test-image",
			Command: []string{"/app/test"},
			Ports: []PortMapping{
				{Name: "api", ContainerPort: 8080, Protocol: "http", Expose: true},
				{Name: "ws", ContainerPort: 8081, Protocol: "tcp", Expose: false},
			},
			Env: EnvConfig{Required: []string{"API_KEY"}},
			HealthCheck: &HealthCheck{
				Type: "http", Path: "/health", Port: "8080", IntervalSeconds: 5,
			},
			Resources: ResourceLimits{MemoryMB: 256, CPUShares: 128},
		},
	}

	installer := NewInstaller(bm, ing, nil, manifests)
	installer.healthPoll = func(context.Context, *Descriptor, string) {}
	ctx := context.Background()

	result, err := installer.Install(ctx, "test-agent", "my-test", map[string]string{"API_KEY": "secret"}, "")
	if err != nil {
		t.Fatalf("Install: %v", err)
	}

	if result.BodyID == "" {
		t.Error("BodyID is empty")
	}
	if result.Name != "my-test" {
		t.Errorf("Name = %q, want my-test", result.Name)
	}
	if len(result.AccessURLs) != 1 {
		t.Errorf("AccessURLs len = %d, want 1", len(result.AccessURLs))
	}
	if result.AllocatedPorts["api"] == 0 {
		t.Error("api port not allocated")
	}
	if _, ok := result.AllocatedPorts["ws"]; ok {
		t.Error("ws port should not be allocated (expose=false)")
	}
}

func TestInstallAgentMissingEnvVar(t *testing.T) {
	s := tempStore(t)
	bm := body.NewBodyManager(s, &mockOrchAdapter{}, "")

	manifests := map[string]*Descriptor{
		"test-agent": {
			Name:  "test-agent",
			Image: "test-image",
			Env:   EnvConfig{Required: []string{"API_KEY"}},
		},
	}

	installer := NewInstaller(bm, nil, nil, manifests)
	ctx := context.Background()

	_, err := installer.Install(ctx, "test-agent", "my-test", map[string]string{}, "")
	if err == nil {
		t.Fatal("expected error for missing env var, got nil")
	}

	var valErr *service.ValidationError
	if ok := err.(*service.ValidationError); ok == nil {
		t.Fatalf("expected ValidationError, got %T", err)
	}
	_ = valErr
}

func TestInstallAgentDuplicateName(t *testing.T) {
	s := tempStore(t)
	bm := body.NewBodyManager(s, &mockOrchAdapter{}, "")

	manifests := map[string]*Descriptor{
		"test-agent": {
			Name:  "test-agent",
			Image: "test-image",
			Env:   EnvConfig{Required: []string{"API_KEY"}},
		},
	}

	installer := NewInstaller(bm, nil, nil, manifests)
	ctx := context.Background()

	// First install
	_, err := installer.Install(ctx, "test-agent", "my-test", map[string]string{"API_KEY": "secret"}, "")
	if err != nil {
		t.Fatalf("first install: %v", err)
	}

	// Second install with same name
	_, err = installer.Install(ctx, "test-agent", "my-test", map[string]string{"API_KEY": "secret"}, "")
	if err == nil {
		t.Fatal("expected error for duplicate name, got nil")
	}

	var confErr *service.ConflictError
	if ok := err.(*service.ConflictError); ok == nil {
		t.Fatalf("expected ConflictError, got %T", err)
	}
	_ = confErr
}

func TestInstallAgentUnknownType(t *testing.T) {
	s := tempStore(t)
	bm := body.NewBodyManager(s, &mockOrchAdapter{}, "")

	installer := NewInstaller(bm, nil, nil, map[string]*Descriptor{})
	ctx := context.Background()

	_, err := installer.Install(ctx, "unknown-agent", "my-test", map[string]string{}, "")
	if err == nil {
		t.Fatal("expected error for unknown type, got nil")
	}

	var notFound *service.NotFoundError
	if ok := err.(*service.NotFoundError); ok == nil {
		t.Fatalf("expected NotFoundError, got %T", err)
	}
	_ = notFound
}

func TestInstallAgentInlineManifest(t *testing.T) {
	s := tempStore(t)
	bm := body.NewBodyManager(s, &mockOrchAdapter{}, "")
	ing := &mockIngress{}

	installer := NewInstaller(bm, ing, nil, map[string]*Descriptor{})
	installer.healthPoll = func(context.Context, *Descriptor, string) {}
	ctx := context.Background()

	inlineYAML := `
name: inline-agent
image: inline-image
command: ["/app/inline"]
ports:
  - name: api
    container_port: 8080
    protocol: http
    expose: true
env:
  required: [API_KEY]
resources:
  memory_mb: 256
  cpu_shares: 128
`

	result, err := installer.Install(ctx, "inline-agent", "my-inline", map[string]string{"API_KEY": "secret"}, inlineYAML)
	if err != nil {
		t.Fatalf("Install with inline descriptor: %v", err)
	}
	if result.BodyID == "" {
		t.Error("BodyID is empty")
	}
	if result.Name != "my-inline" {
		t.Errorf("Name = %q, want my-inline", result.Name)
	}
	if len(result.AccessURLs) != 1 {
		t.Errorf("AccessURLs len = %d, want 1", len(result.AccessURLs))
	}
}

func TestInstallAgentInlineManifestInvalidYAML(t *testing.T) {
	s := tempStore(t)
	bm := body.NewBodyManager(s, &mockOrchAdapter{}, "")

	installer := NewInstaller(bm, nil, nil, map[string]*Descriptor{})
	ctx := context.Background()

	invalidYAML := `
name: [broken
image: test
`

	_, err := installer.Install(ctx, "anything", "my-test", map[string]string{}, invalidYAML)
	if err == nil {
		t.Fatal("expected error for invalid inline YAML, got nil")
	}
	if !strings.Contains(err.Error(), "parse") {
		t.Errorf("expected error containing 'parse', got %q", err.Error())
	}
}

func TestInstallAgentInlineManifestMissingEnv(t *testing.T) {
	s := tempStore(t)
	bm := body.NewBodyManager(s, &mockOrchAdapter{}, "")

	installer := NewInstaller(bm, nil, nil, map[string]*Descriptor{})
	ctx := context.Background()

	inlineYAML := `
name: inline-agent
image: inline-image
env:
  required: [API_KEY]
`

	_, err := installer.Install(ctx, "inline-agent", "my-test", map[string]string{}, inlineYAML)
	if err == nil {
		t.Fatal("expected error for missing env var, got nil")
	}

	var valErr *service.ValidationError
	if ok := err.(*service.ValidationError); ok == nil {
		t.Fatalf("expected ValidationError, got %T", err)
	}
	_ = valErr
}

func TestInstallAgentEmptyManifestFallsBackToLocal(t *testing.T) {
	s := tempStore(t)
	bm := body.NewBodyManager(s, &mockOrchAdapter{}, "")

	manifests := map[string]*Descriptor{
		"test-agent": {
			Name:  "test-agent",
			Image: "test-image",
			Env:   EnvConfig{Required: []string{"API_KEY"}},
		},
	}

	installer := NewInstaller(bm, nil, nil, manifests)
	ctx := context.Background()

	_, err := installer.Install(ctx, "test-agent", "my-test", map[string]string{"API_KEY": "secret"}, "")
	if err != nil {
		t.Fatalf("Install with empty descriptor (fallback): %v", err)
	}
}

func TestInstallAgentInlineManifestTakesPrecedence(t *testing.T) {
	s := tempStore(t)
	bm := body.NewBodyManager(s, &mockOrchAdapter{}, "")

	manifests := map[string]*Descriptor{
		"test-agent": {
			Name:  "test-agent",
			Image: "local-image",
			Env:   EnvConfig{Required: []string{"API_KEY"}},
		},
	}

	installer := NewInstaller(bm, nil, nil, manifests)
	ctx := context.Background()

	inlineYAML := `
name: inline-agent
image: inline-image
env:
  required: [API_KEY]
`

	_, err := installer.Install(ctx, "test-agent", "my-test", map[string]string{"API_KEY": "secret"}, inlineYAML)
	if err != nil {
		t.Fatalf("Install with inline descriptor (precedence): %v", err)
	}
}
