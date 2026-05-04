package api

import (
	"net/http"
	"time"

	"github.com/rethink-paradigms/mesh/internal/orchestrator"
)

func handleListNodes(cfg RouterConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !orchestrator.HasCapability[orchestrator.NodeLister](cfg.Orchestrator) {
			WriteError(w, ErrCodeInternal, "node listing not supported by this orchestrator", http.StatusNotImplemented)
			return
		}

		lister, _ := cfg.Orchestrator.(orchestrator.NodeLister)
		nodes, err := lister.ListNodes(r.Context())
		if err != nil {
			WriteError(w, ErrCodeNomadUnreachable, err.Error(), http.StatusBadGateway)
			return
		}

		responses := make([]NodeResponse, 0, len(nodes))
		for _, n := range nodes {
			responses = append(responses, NodeResponse{
				ID:      n.ID,
				Name:    n.Name,
				Address: n.Address,
				State:   n.State,
				Capacity: CapacityInfo{
					CPUMHZ:   n.Capacity.CPUMHZ,
					MemoryMB: n.Capacity.MemoryMB,
					DiskGB:   n.Capacity.DiskGB,
				},
				BodiesCount: 0,
				Provider:    n.Provider,
				Region:      n.Region,
				LastSeenAt:  n.LastSeenAt.Format(time.RFC3339),
			})
		}

		WriteJSON(w, http.StatusOK, ListNodesResponse{Nodes: responses})
	}
}
