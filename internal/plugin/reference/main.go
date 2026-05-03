package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/hashicorp/go-plugin"
	"github.com/rethink-paradigms/mesh/internal/adapter"
	meshplugin "github.com/rethink-paradigms/mesh/internal/plugin"
	"github.com/rethink-paradigms/mesh/internal/orchestrator"
)

type ReferencePlugin struct{}

func (p *ReferencePlugin) PluginInfo(ctx context.Context) (meshplugin.PluginMeta, error) {
	return meshplugin.PluginMeta{
		Name:        "reference",
		Version:     "1.0.0",
		Description: "Reference filesystem-local adapter plugin",
		Author:      "mesh",
	}, nil
}

func (p *ReferencePlugin) GetAdapter(ctx context.Context) (adapter.SubstrateAdapter, error) {
	return &LocalAdapter{}, nil
}

func (p *ReferencePlugin) GetOrchestrator(ctx context.Context) (orchestrator.OrchestratorAdapter, error) {
	return nil, fmt.Errorf("orchestrator adapter not yet implemented in reference plugin")
}

type LocalAdapter struct{}

func (a *LocalAdapter) Create(ctx context.Context, spec adapter.BodySpec) (adapter.Handle, error) {
	return adapter.Handle("local-" + spec.Image), nil
}
func (a *LocalAdapter) Start(ctx context.Context, id adapter.Handle) error   { return nil }
func (a *LocalAdapter) Stop(ctx context.Context, id adapter.Handle, opts adapter.StopOpts) error {
	return nil
}
func (a *LocalAdapter) Destroy(ctx context.Context, id adapter.Handle) error { return nil }
func (a *LocalAdapter) GetStatus(ctx context.Context, id adapter.Handle) (adapter.BodyStatus, error) {
	return adapter.BodyStatus{State: adapter.StateRunning}, nil
}
func (a *LocalAdapter) Exec(ctx context.Context, id adapter.Handle, cmd []string) (adapter.ExecResult, error) {
	return adapter.ExecResult{ExitCode: 0}, nil
}
func (a *LocalAdapter) ExportFilesystem(ctx context.Context, id adapter.Handle) (io.ReadCloser, error) {
	return nil, fmt.Errorf("not supported")
}
func (a *LocalAdapter) ImportFilesystem(ctx context.Context, id adapter.Handle, tarball io.Reader, opts adapter.ImportOpts) error {
	return fmt.Errorf("not supported")
}
func (a *LocalAdapter) Inspect(ctx context.Context, id adapter.Handle) (adapter.ContainerMetadata, error) {
	return adapter.ContainerMetadata{}, nil
}
func (a *LocalAdapter) Capabilities() adapter.AdapterCapabilities {
	return adapter.AdapterCapabilities{}
}
func (a *LocalAdapter) SubstrateName() string {
	return "local"
}
func (a *LocalAdapter) IsHealthy(ctx context.Context) bool {
	return true
}

func main() {
	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: meshplugin.Handshake,
		Plugins: map[string]plugin.Plugin{
			meshplugin.PluginName: &meshplugin.MeshPluginGRPC{Impl: &ReferencePlugin{}},
		},
		GRPCServer: plugin.DefaultGRPCServer,
	})
	os.Exit(0)
}
