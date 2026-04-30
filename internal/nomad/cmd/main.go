package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/hashicorp/go-plugin"
	"github.com/rethink-paradigms/mesh/internal/adapter"
	"github.com/rethink-paradigms/mesh/internal/nomad"
	meshplugin "github.com/rethink-paradigms/mesh/internal/plugin"
)

type NomadPlugin struct {
	adapter *nomad.Adapter
}

func (p *NomadPlugin) PluginInfo(ctx context.Context) (meshplugin.PluginMeta, error) {
	return meshplugin.PluginMeta{
		Name:        "nomad",
		Version:     "1.0.0",
		Description: "Nomad substrate adapter for Mesh",
		Author:      "mesh",
	}, nil
}

func (p *NomadPlugin) GetAdapter(ctx context.Context) (adapter.SubstrateAdapter, error) {
	if p.adapter == nil {
		cfg := nomad.Config{
			Address:   os.Getenv("NOMAD_ADDR"),
			Token:     os.Getenv("NOMAD_TOKEN"),
			Region:    os.Getenv("NOMAD_REGION"),
			Namespace: os.Getenv("NOMAD_NAMESPACE"),
		}
		if cfg.Address == "" {
			cfg.Address = "http://127.0.0.1:4646"
		}
		p.adapter = nomad.New(cfg)
	}
	return p.adapter, nil
}

type localAdapter struct{}

func (a *localAdapter) Create(ctx context.Context, spec adapter.BodySpec) (adapter.Handle, error) {
	return adapter.Handle("local-" + spec.Image), nil
}
func (a *localAdapter) Start(ctx context.Context, id adapter.Handle) error   { return nil }
func (a *localAdapter) Stop(ctx context.Context, id adapter.Handle, opts adapter.StopOpts) error {
	return nil
}
func (a *localAdapter) Destroy(ctx context.Context, id adapter.Handle) error { return nil }
func (a *localAdapter) GetStatus(ctx context.Context, id adapter.Handle) (adapter.BodyStatus, error) {
	return adapter.BodyStatus{State: adapter.StateRunning}, nil
}
func (a *localAdapter) Exec(ctx context.Context, id adapter.Handle, cmd []string) (adapter.ExecResult, error) {
	return adapter.ExecResult{ExitCode: 0}, nil
}
func (a *localAdapter) ExportFilesystem(ctx context.Context, id adapter.Handle) (io.ReadCloser, error) {
	return nil, fmt.Errorf("not supported")
}
func (a *localAdapter) ImportFilesystem(ctx context.Context, id adapter.Handle, tarball io.Reader, opts adapter.ImportOpts) error {
	return fmt.Errorf("not supported")
}
func (a *localAdapter) Inspect(ctx context.Context, id adapter.Handle) (adapter.ContainerMetadata, error) {
	return adapter.ContainerMetadata{}, nil
}
func (a *localAdapter) Capabilities() adapter.AdapterCapabilities {
	return adapter.AdapterCapabilities{}
}
func (a *localAdapter) SubstrateName() string {
	return "local"
}
func (a *localAdapter) IsHealthy(ctx context.Context) bool {
	return true
}

func main() {
	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: meshplugin.Handshake,
		Plugins: map[string]plugin.Plugin{
			meshplugin.PluginName: &meshplugin.MeshPluginGRPC{Impl: &NomadPlugin{adapter: nomad.NewFromEnv()}},
		},
		GRPCServer: plugin.DefaultGRPCServer,
	})
	os.Exit(0)
}
