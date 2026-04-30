package plugin

import (
	"context"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/hashicorp/go-plugin"
	"github.com/rethink-paradigms/mesh/internal/adapter"
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
}
