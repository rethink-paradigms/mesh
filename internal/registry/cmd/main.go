package main

import (
	"context"
	"fmt"
	"os"

	"github.com/hashicorp/go-plugin"
	"github.com/rethink-paradigms/mesh/internal/adapter"
	"github.com/rethink-paradigms/mesh/internal/orchestrator"
	meshplugin "github.com/rethink-paradigms/mesh/internal/plugin"
	"github.com/rethink-paradigms/mesh/internal/registry"
)

type S3RegistryPlugin struct{}

func (p *S3RegistryPlugin) PluginInfo(ctx context.Context) (meshplugin.PluginMeta, error) {
	return meshplugin.PluginMeta{
		Name:        "s3-registry",
		Version:     "1.0.0",
		Description: "S3-backed snapshot registry plugin",
		Author:      "mesh",
	}, nil
}

func (p *S3RegistryPlugin) GetAdapter(ctx context.Context) (adapter.SubstrateAdapter, error) {
	cfg := registry.RegistryConfig{
		Type:            os.Getenv("MESH_REGISTRY_TYPE"),
		Bucket:          os.Getenv("MESH_REGISTRY_BUCKET"),
		Region:          os.Getenv("MESH_REGISTRY_REGION"),
		Endpoint:        os.Getenv("MESH_REGISTRY_ENDPOINT"),
		AccessKeyID:     os.Getenv("MESH_REGISTRY_ACCESS_KEY_ID"),
		SecretAccessKey: os.Getenv("MESH_REGISTRY_SECRET_ACCESS_KEY"),
	}

	if cfg.Bucket == "" {
		return nil, fmt.Errorf("MESH_REGISTRY_BUCKET is required")
	}

	plugin, err := registry.NewS3RegistryPlugin(cfg)
	if err != nil {
		return nil, err
	}
	return plugin.GetAdapter(ctx)
}

func (p *S3RegistryPlugin) GetOrchestrator(ctx context.Context) (orchestrator.OrchestratorAdapter, error) {
	return nil, fmt.Errorf("S3RegistryPlugin does not provide an orchestrator")
}

func main() {
	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: meshplugin.Handshake,
		Plugins: map[string]plugin.Plugin{
			meshplugin.PluginName: &meshplugin.MeshPluginGRPC{Impl: &S3RegistryPlugin{}},
		},
		GRPCServer: plugin.DefaultGRPCServer,
	})
	os.Exit(0)
}
