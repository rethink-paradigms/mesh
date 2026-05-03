package nomad

import (
	"context"
	"strings"
	"testing"

	"github.com/rethink-paradigms/mesh/internal/orchestrator"
)

func TestNew(t *testing.T) {
	a := New(Config{Address: "http://127.0.0.1:4646"})
	if a == nil {
		t.Fatal("New() returned nil")
	}
	if a.config.Address != "http://127.0.0.1:4646" {
		t.Errorf("Address = %q, want %q", a.config.Address, "http://127.0.0.1:4646")
	}
}

func TestNewFromEnv(t *testing.T) {
	t.Setenv("NOMAD_ADDR", "http://nomad.example.com:4646")
	t.Setenv("NOMAD_TOKEN", "test-token")
	t.Setenv("NOMAD_REGION", "global")
	t.Setenv("NOMAD_NAMESPACE", "default")

	a := NewFromEnv()
	if a == nil {
		t.Fatal("NewFromEnv() returned nil")
	}
	if a.config.Address != "http://nomad.example.com:4646" {
		t.Errorf("Address = %q, want %q", a.config.Address, "http://nomad.example.com:4646")
	}
	if a.config.Token != "test-token" {
		t.Errorf("Token = %q, want %q", a.config.Token, "test-token")
	}
	if a.config.Region != "global" {
		t.Errorf("Region = %q, want %q", a.config.Region, "global")
	}
	if a.config.Namespace != "default" {
		t.Errorf("Namespace = %q, want %q", a.config.Namespace, "default")
	}
}

func TestNewFromEnvDefaults(t *testing.T) {
	t.Setenv("NOMAD_ADDR", "")
	t.Setenv("NOMAD_TOKEN", "")

	a := NewFromEnv()
	if a.config.Address != "http://127.0.0.1:4646" {
		t.Errorf("Address = %q, want default", a.config.Address)
	}
}

func TestName(t *testing.T) {
	a := New(Config{})
	if a.Name() != "nomad" {
		t.Errorf("Name() = %q, want %q", a.Name(), "nomad")
	}
}

func TestMapNomadClientStatus(t *testing.T) {
	tests := []struct {
		nomadStatus string
		want        orchestrator.BodyState
	}{
		{"pending", orchestrator.StateStarting},
		{"running", orchestrator.StateRunning},
		{"failed", orchestrator.StateError},
		{"lost", orchestrator.StateError},
		{"complete", orchestrator.StateStopped},
		{"terminal", orchestrator.StateStopped},
		{"unknown", orchestrator.StateCreated},
		{"", orchestrator.StateCreated},
	}

	for _, tt := range tests {
		t.Run(tt.nomadStatus, func(t *testing.T) {
			got := mapNomadClientStatus(tt.nomadStatus)
			if got != tt.want {
				t.Errorf("mapNomadClientStatus(%q) = %q, want %q", tt.nomadStatus, got, tt.want)
			}
		})
	}
}

func TestGenerateJobID(t *testing.T) {
	id := generateJobID("alpine:latest")
	if !strings.HasPrefix(id, "mesh-alpine-latest-") {
		t.Errorf("generateJobID() = %q, expected prefix mesh-alpine-latest-", id)
	}

	id2 := generateJobID("registry.io/my/app:v1.0")
	if !strings.HasPrefix(id2, "mesh-registry-io-my-app-v1-0-") {
		t.Errorf("generateJobID() = %q, expected prefix mesh-registry-io-my-app-v1-0-", id2)
	}
}

func TestScheduleBodyErrorWithoutNomad(t *testing.T) {
	a := New(Config{Address: "http://127.0.0.1:1"})
	ctx := context.Background()

	_, err := a.ScheduleBody(ctx, orchestrator.BodySpec{Image: "alpine"})
	if err == nil {
		t.Error("ScheduleBody should fail when Nomad is not available")
	}
}

func TestStartBodyErrorWithoutNomad(t *testing.T) {
	a := New(Config{Address: "http://127.0.0.1:1"})
	ctx := context.Background()

	err := a.StartBody(ctx, orchestrator.Handle("nonexistent"))
	if err == nil {
		t.Error("StartBody should fail when Nomad is not available")
	}
}

func TestStopBodyErrorWithoutNomad(t *testing.T) {
	a := New(Config{Address: "http://127.0.0.1:1"})
	ctx := context.Background()

	err := a.StopBody(ctx, orchestrator.Handle("nonexistent"))
	if err == nil {
		t.Error("StopBody should fail when Nomad is not available")
	}
}

func TestDestroyBodyErrorWithoutNomad(t *testing.T) {
	a := New(Config{Address: "http://127.0.0.1:1"})
	ctx := context.Background()

	err := a.DestroyBody(ctx, orchestrator.Handle("nonexistent"))
	if err == nil {
		t.Error("DestroyBody should fail when Nomad is not available")
	}
}

func TestGetBodyStatusErrorWithoutNomad(t *testing.T) {
	a := New(Config{Address: "http://127.0.0.1:1"})
	ctx := context.Background()

	_, err := a.GetBodyStatus(ctx, orchestrator.Handle("nonexistent"))
	if err == nil {
		t.Error("GetBodyStatus should fail when Nomad is not available")
	}
}

func TestExecErrorWithoutNomad(t *testing.T) {
	a := New(Config{Address: "http://127.0.0.1:1"})
	ctx := context.Background()

	_, err := a.Exec(ctx, orchestrator.Handle("nonexistent"), []string{"echo", "hi"})
	if err == nil {
		t.Error("Exec should fail when Nomad is not available")
	}
}

func TestExportFilesystemErrorWithoutNomad(t *testing.T) {
	a := New(Config{Address: "http://127.0.0.1:1"})
	ctx := context.Background()

	_, err := a.ExportFilesystem(ctx, orchestrator.Handle("nonexistent"))
	if err == nil {
		t.Error("ExportFilesystem should fail when Nomad is not available")
	}
}

func TestImportFilesystemErrorWithoutNomad(t *testing.T) {
	a := New(Config{Address: "http://127.0.0.1:1"})
	ctx := context.Background()

	err := a.ImportFilesystem(ctx, orchestrator.Handle("nonexistent"), nil)
	if err == nil {
		t.Error("ImportFilesystem should fail when Nomad is not available")
	}
}

func TestInspectErrorWithoutNomad(t *testing.T) {
	a := New(Config{Address: "http://127.0.0.1:1"})
	ctx := context.Background()

	_, err := a.Inspect(ctx, orchestrator.Handle("nonexistent"))
	if err == nil {
		t.Error("Inspect should fail when Nomad is not available")
	}
}

func TestIsHealthyWithoutNomad(t *testing.T) {
	a := New(Config{Address: "http://127.0.0.1:1"})
	ctx := context.Background()

	if a.IsHealthy(ctx) {
		t.Error("IsHealthy should return false when Nomad is not available")
	}
}

func TestCompileTimeInterfaceCheck(t *testing.T) {
	var _ orchestrator.OrchestratorAdapter = (*Adapter)(nil)
}

func TestNomadExtensions(t *testing.T) {
	a := New(Config{})

	if !orchestrator.HasCapability[orchestrator.Exporter](a) {
		t.Error("Adapter should implement Exporter")
	}
	if !orchestrator.HasCapability[orchestrator.Importer](a) {
		t.Error("Adapter should implement Importer")
	}
	if !orchestrator.HasCapability[orchestrator.Inspector](a) {
		t.Error("Adapter should implement Inspector")
	}
	if !orchestrator.HasCapability[orchestrator.Executor](a) {
		t.Error("Adapter should implement Executor")
	}
}

func TestExtensionCompileTimeInterfaceChecks(t *testing.T) {
	var _ orchestrator.Exporter = (*Adapter)(nil)
	var _ orchestrator.Importer = (*Adapter)(nil)
	var _ orchestrator.Inspector = (*Adapter)(nil)
	var _ orchestrator.Executor = (*Adapter)(nil)
}
