package api

import (
	"net/http"

	"github.com/rethink-paradigms/mesh/internal/orchestrator"
)

func (h *Handler) Healthz(w http.ResponseWriter, r *http.Request) {
	nomadConnected := h.cfg.Orchestrator.IsHealthy(r.Context())

	bodiesCount := 0
	if h.cfg.Store != nil {
		bodies, err := h.cfg.Store.ListBodies(r.Context())
		if err == nil {
			bodiesCount = len(bodies)
		}
	}

	nodesCount := 0
	if orchestrator.HasCapability[orchestrator.NodeLister](h.cfg.Orchestrator) {
		if lister, ok := h.cfg.Orchestrator.(orchestrator.NodeLister); ok {
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
		Status:                status,
		Version:               h.cfg.Version,
		NomadConnected:        nomadConnected,
		ConsulConnected:       false,
		BodiesCount:           bodiesCount,
		NodesCount:            nodesCount,
		OrchestratorConnected: nomadConnected,
	})
}
