package api

import (
	"net/http"

	"github.com/rethink-paradigms/mesh/internal/orchestrator"
)

func handleHealthz(cfg RouterConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		nomadConnected := cfg.Orchestrator.IsHealthy(r.Context())

		bodiesCount := 0
		if cfg.Store != nil {
			bodies, err := cfg.Store.ListBodies(r.Context())
			if err == nil {
				bodiesCount = len(bodies)
			}
		}

		nodesCount := 0
		if orchestrator.HasCapability[orchestrator.NodeLister](cfg.Orchestrator) {
			if lister, ok := cfg.Orchestrator.(orchestrator.NodeLister); ok {
				nodes, err := lister.ListNodes(r.Context())
				if err == nil {
					nodesCount = len(nodes)
				}
			}
		}

		status := "healthy"
		if !nomadConnected {
			status = "degraded"
		}

		WriteJSON(w, http.StatusOK, HealthzResponse{
			Status:          status,
			Version:         cfg.Version,
			NomadConnected:  nomadConnected,
			ConsulConnected: false,
			BodiesCount:     bodiesCount,
			NodesCount:      nodesCount,
		})
	}
}
