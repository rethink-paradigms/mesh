package nomad

import (
	"context"
	"strings"
	"testing"

	"github.com/rethink-paradigms/mesh/internal/adapter"
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

func TestCapabilities(t *testing.T) {
	a := New(Config{})
	caps := a.Capabilities()

	if !caps.ExportFilesystem {
		t.Error("ExportFilesystem should be true")
	}
	if !caps.ImportFilesystem {
		t.Error("ImportFilesystem should be true")
	}
	if !caps.Inspect {
		t.Error("Inspect should be true")
	}
}

func TestSubstrateName(t *testing.T) {
	a := New(Config{})
	if a.SubstrateName() != "nomad" {
		t.Errorf("SubstrateName() = %q, want %q", a.SubstrateName(), "nomad")
	}
}

func TestMapNomadClientStatus(t *testing.T) {
	tests := []struct {
		nomadStatus string
		want        adapter.BodyState
	}{
		{"pending", adapter.StateStarting},
		{"running", adapter.StateRunning},
		{"failed", adapter.StateError},
		{"lost", adapter.StateError},
		{"complete", adapter.StateStopped},
		{"terminal", adapter.StateStopped},
		{"unknown", adapter.StateCreated},
		{"", adapter.StateCreated},
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

func TestCreateErrorWithoutNomad(t *testing.T) {
	a := New(Config{Address: "http://127.0.0.1:1"})
	ctx := context.Background()

	_, err := a.Create(ctx, adapter.BodySpec{Image: "alpine"})
	if err == nil {
		t.Error("Create should fail when Nomad is not available")
	}
}

func TestStartErrorWithoutNomad(t *testing.T) {
	a := New(Config{Address: "http://127.0.0.1:1"})
	ctx := context.Background()

	err := a.Start(ctx, adapter.Handle("nonexistent"))
	if err == nil {
		t.Error("Start should fail when Nomad is not available")
	}
}

func TestStopErrorWithoutNomad(t *testing.T) {
	a := New(Config{Address: "http://127.0.0.1:1"})
	ctx := context.Background()

	err := a.Stop(ctx, adapter.Handle("nonexistent"), adapter.StopOpts{})
	if err == nil {
		t.Error("Stop should fail when Nomad is not available")
	}
}

func TestDestroyErrorWithoutNomad(t *testing.T) {
	a := New(Config{Address: "http://127.0.0.1:1"})
	ctx := context.Background()

	err := a.Destroy(ctx, adapter.Handle("nonexistent"))
	if err == nil {
		t.Error("Destroy should fail when Nomad is not available")
	}
}

func TestGetStatusErrorWithoutNomad(t *testing.T) {
	a := New(Config{Address: "http://127.0.0.1:1"})
	ctx := context.Background()

	_, err := a.GetStatus(ctx, adapter.Handle("nonexistent"))
	if err == nil {
		t.Error("GetStatus should fail when Nomad is not available")
	}
}

func TestExecErrorWithoutNomad(t *testing.T) {
	a := New(Config{Address: "http://127.0.0.1:1"})
	ctx := context.Background()

	_, err := a.Exec(ctx, adapter.Handle("nonexistent"), []string{"echo", "hi"})
	if err == nil {
		t.Error("Exec should fail when Nomad is not available")
	}
}

func TestExportFilesystemErrorWithoutNomad(t *testing.T) {
	a := New(Config{Address: "http://127.0.0.1:1"})
	ctx := context.Background()

	_, err := a.ExportFilesystem(ctx, adapter.Handle("nonexistent"))
	if err == nil {
		t.Error("ExportFilesystem should fail when Nomad is not available")
	}
}

func TestImportFilesystemErrorWithoutNomad(t *testing.T) {
	a := New(Config{Address: "http://127.0.0.1:1"})
	ctx := context.Background()

	err := a.ImportFilesystem(ctx, adapter.Handle("nonexistent"), nil, adapter.ImportOpts{})
	if err == nil {
		t.Error("ImportFilesystem should fail when Nomad is not available")
	}
}

func TestInspectErrorWithoutNomad(t *testing.T) {
	a := New(Config{Address: "http://127.0.0.1:1"})
	ctx := context.Background()

	_, err := a.Inspect(ctx, adapter.Handle("nonexistent"))
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
	var _ adapter.SubstrateAdapter = (*Adapter)(nil)
}
