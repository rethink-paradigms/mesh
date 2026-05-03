package plugin

import (
	"context"

	"github.com/rethink-paradigms/mesh/internal/adapter"
	"github.com/rethink-paradigms/mesh/internal/orchestrator"
)

type PluginMeta struct {
	Name        string
	Version     string
	Description string
	Author      string
}

type MeshPlugin interface {
	PluginInfo(ctx context.Context) (PluginMeta, error)
	// Deprecated: use GetOrchestrator instead
	GetAdapter(ctx context.Context) (adapter.SubstrateAdapter, error)
	GetOrchestrator(ctx context.Context) (orchestrator.OrchestratorAdapter, error)
}
