package plugin

import (
	"context"

	"github.com/rethink-paradigms/mesh/internal/adapter"
)

type PluginMeta struct {
	Name        string
	Version     string
	Description string
	Author      string
}

type MeshPlugin interface {
	PluginInfo(ctx context.Context) (PluginMeta, error)
	GetAdapter(ctx context.Context) (adapter.SubstrateAdapter, error)
}
