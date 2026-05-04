package plugin

import (
	"context"

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
	GetOrchestrator(ctx context.Context) (orchestrator.OrchestratorAdapter, error)
}
