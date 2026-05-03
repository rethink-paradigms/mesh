package main

import (
	"context"
	"fmt"
	"os"

	"github.com/hashicorp/go-plugin"
	"github.com/rethink-paradigms/mesh/internal/adapter"
	"github.com/rethink-paradigms/mesh/internal/nomad"
	"github.com/rethink-paradigms/mesh/internal/orchestrator"
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

// Deprecated: use GetOrchestrator.
func (p *NomadPlugin) GetAdapter(ctx context.Context) (adapter.SubstrateAdapter, error) {
	return nil, fmt.Errorf("GetAdapter is deprecated: use GetOrchestrator instead")
}

func (p *NomadPlugin) GetOrchestrator(ctx context.Context) (orchestrator.OrchestratorAdapter, error) {
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
