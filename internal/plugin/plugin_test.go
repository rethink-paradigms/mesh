package plugin

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/hashicorp/go-plugin"
	"github.com/rethink-paradigms/mesh/internal/adapter"
	"github.com/rethink-paradigms/mesh/internal/orchestrator"
)

func buildReferencePlugin(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	binPath := filepath.Join(tmpDir, "mesh-plugin-reference")
	if runtime.GOOS == "windows" {
		binPath += ".exe"
	}

	_, thisFile, _, _ := runtime.Caller(0)
	refDir := filepath.Join(filepath.Dir(thisFile), "reference")

	cmd := exec.Command("go", "build", "-o", binPath, ".")
	cmd.Dir = refDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to build reference plugin: %v\n%s", err, out)
	}
	return binPath
}

type mockPlugin struct{}

func (m *mockPlugin) PluginInfo(ctx context.Context) (PluginMeta, error) {
	return PluginMeta{Name: "mock", Version: "0.0.1"}, nil
}

func (m *mockPlugin) GetAdapter(ctx context.Context) (adapter.SubstrateAdapter, error) {
	return &mockAdapter{}, nil
}

func (m *mockPlugin) GetOrchestrator(ctx context.Context) (orchestrator.OrchestratorAdapter, error) {
	return &mockOrchestrator{}, nil
}

type mockAdapter struct{}

func (a *mockAdapter) Create(ctx context.Context, spec adapter.BodySpec) (adapter.Handle, error) {
	return adapter.Handle("mock-" + spec.Image), nil
}
func (a *mockAdapter) Start(ctx context.Context, id adapter.Handle) error { return nil }
func (a *mockAdapter) Stop(ctx context.Context, id adapter.Handle, opts adapter.StopOpts) error {
	return nil
}
func (a *mockAdapter) Destroy(ctx context.Context, id adapter.Handle) error { return nil }
func (a *mockAdapter) GetStatus(ctx context.Context, id adapter.Handle) (adapter.BodyStatus, error) {
	return adapter.BodyStatus{State: adapter.StateRunning}, nil
}
func (a *mockAdapter) Exec(ctx context.Context, id adapter.Handle, cmd []string) (adapter.ExecResult, error) {
	return adapter.ExecResult{ExitCode: 0}, nil
}
func (a *mockAdapter) ExportFilesystem(ctx context.Context, id adapter.Handle) (io.ReadCloser, error) {
	return nil, fmt.Errorf("not supported")
}
func (a *mockAdapter) ImportFilesystem(ctx context.Context, id adapter.Handle, tarball io.Reader, opts adapter.ImportOpts) error {
	return fmt.Errorf("not supported")
}
func (a *mockAdapter) Inspect(ctx context.Context, id adapter.Handle) (adapter.ContainerMetadata, error) {
	return adapter.ContainerMetadata{}, nil
}
func (a *mockAdapter) Capabilities() adapter.AdapterCapabilities {
	return adapter.AdapterCapabilities{}
}
func (a *mockAdapter) SubstrateName() string              { return "mock" }
func (a *mockAdapter) IsHealthy(ctx context.Context) bool { return true }

type mockOrchestrator struct{}

func (o *mockOrchestrator) ScheduleBody(ctx context.Context, spec orchestrator.BodySpec) (orchestrator.Handle, error) {
	return orchestrator.Handle("mock-" + spec.Image), nil
}
func (o *mockOrchestrator) StartBody(ctx context.Context, id orchestrator.Handle) error   { return nil }
func (o *mockOrchestrator) StopBody(ctx context.Context, id orchestrator.Handle) error    { return nil }
func (o *mockOrchestrator) DestroyBody(ctx context.Context, id orchestrator.Handle) error { return nil }
func (o *mockOrchestrator) GetBodyStatus(ctx context.Context, id orchestrator.Handle) (orchestrator.BodyStatus, error) {
	return orchestrator.BodyStatus{State: orchestrator.StateRunning}, nil
}
func (o *mockOrchestrator) Name() string                       { return "mock" }
func (o *mockOrchestrator) IsHealthy(ctx context.Context) bool { return true }

func TestPluginGetOrchestrator(t *testing.T) {
	mp := &mockPlugin{}
	ctx := context.Background()

	orch, err := mp.GetOrchestrator(ctx)
	if err != nil {
		t.Fatalf("GetOrchestrator failed: %v", err)
	}
	if orch == nil {
		t.Fatal("expected orchestrator adapter, got nil")
	}
	if orch.Name() != "mock" {
		t.Errorf("expected name 'mock', got %q", orch.Name())
	}
	if !orch.IsHealthy(ctx) {
		t.Error("expected orchestrator to be healthy")
	}
}

func TestPluginBackwardCompat(t *testing.T) {
	mp := &mockPlugin{}
	ctx := context.Background()

	adapt, err := mp.GetAdapter(ctx)
	if err != nil {
		t.Fatalf("GetAdapter failed: %v", err)
	}
	if adapt == nil {
		t.Fatal("expected adapter, got nil")
	}
	if adapt.SubstrateName() != "mock" {
		t.Errorf("expected substrate name 'mock', got %q", adapt.SubstrateName())
	}
	if !adapt.IsHealthy(ctx) {
		t.Error("expected adapter to be healthy")
	}
}

func TestPluginInterfaceCompliance(t *testing.T) {
	var _ MeshPlugin = (*mockPlugin)(nil)
}

func TestReferencePlugin(t *testing.T) {
	binPath := buildReferencePlugin(t)

	client := plugin.NewClient(&plugin.ClientConfig{
		HandshakeConfig: Handshake,
		Plugins:         PluginMap,
		Cmd:             exec.Command(binPath),
		AllowedProtocols: []plugin.Protocol{
			plugin.ProtocolGRPC,
		},
	})
	defer client.Kill()

	rpcClient, err := client.Client()
	if err != nil {
		t.Fatalf("failed to create plugin client: %v", err)
	}

	raw, err := rpcClient.Dispense(PluginName)
	if err != nil {
		t.Fatalf("failed to dispense plugin: %v", err)
	}

	meshPlugin := raw.(MeshPlugin)
	ctx := context.Background()

	t.Run("PluginInfo", func(t *testing.T) {
		meta, err := meshPlugin.PluginInfo(ctx)
		if err != nil {
			t.Fatalf("PluginInfo failed: %v", err)
		}
		if meta.Name != "reference" {
			t.Errorf("expected name 'reference', got %q", meta.Name)
		}
		if meta.Version != "1.0.0" {
			t.Errorf("expected version '1.0.0', got %q", meta.Version)
		}
		if meta.Description == "" {
			t.Error("expected non-empty description")
		}
	})

	t.Run("GetAdapter", func(t *testing.T) {
		adapt, err := meshPlugin.GetAdapter(ctx)
		if err != nil {
			t.Fatalf("GetAdapter failed: %v", err)
		}
		if adapt == nil {
			t.Fatal("expected adapter, got nil")
		}
		if adapt.SubstrateName() != "local" {
			t.Errorf("expected substrate name 'local', got %q", adapt.SubstrateName())
		}
		if !adapt.IsHealthy(ctx) {
			t.Error("expected adapter to be healthy")
		}
		caps := adapt.Capabilities()
		if caps.ExportFilesystem || caps.ImportFilesystem || caps.Inspect {
			t.Error("expected no capabilities for reference adapter")
		}
	})

	t.Run("AdapterCreate", func(t *testing.T) {
		adapt, _ := meshPlugin.GetAdapter(ctx)
		handle, err := adapt.Create(ctx, adapter.BodySpec{Image: "test-img"})
		if err == nil {
			t.Fatal("expected Create to fail for proxy adapter")
		}
		if handle != "" {
			t.Error("expected empty handle on error")
		}
	})

	t.Run("GetOrchestratorNotImplemented", func(t *testing.T) {
		_, err := meshPlugin.GetOrchestrator(ctx)
		if err == nil {
			t.Fatal("expected GetOrchestrator to fail over gRPC")
		}
	})
}
