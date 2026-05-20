package api

import (
	"net/http"
	"time"

	"github.com/rethink-paradigms/mesh/internal/orchestrator"
	"github.com/rethink-paradigms/mesh/internal/version"
)

// @Summary Health check
// @Description Returns daemon health status, including orchestrator connectivity and body/node counts.
// @Tags system
// @Success 200 {object} HealthzResponse
// @Router /healthz [get]
func (h *Handler) Healthz(w http.ResponseWriter, r *http.Request) {
	nomadConnected := h.cfg.Orchestrator.IsHealthy(r.Context())

	bodiesCount := 0
	stuckStartingCount := 0
	sqliteHealthy := false
	if h.cfg.Store != nil {
		// SQLite health check
		var one int
		if err := h.cfg.Store.QueryRow(r.Context(), "SELECT 1").Scan(&one); err == nil {
			sqliteHealthy = true
		}

		bodies, err := h.cfg.Store.ListBodies(r.Context())
		if err == nil {
			bodiesCount = len(bodies)
			cutoff := time.Now().UTC().Add(-5 * time.Minute)
			for _, b := range bodies {
				if b.State == orchestrator.StateStarting {
					if updatedAt, err := time.Parse(time.RFC3339, b.UpdatedAt); err == nil && updatedAt.Before(cutoff) {
						stuckStartingCount++
					}
				}
			}
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
	if !nomadConnected || stuckStartingCount > 0 {
		status = "degraded"
	}
	// Only count SQLite as unhealthy if a Store is configured
	if h.cfg.Store != nil && !sqliteHealthy {
		status = "degraded"
	}

	heartbeatEnabled := h.cfg.GatewayURL != "" && h.cfg.HeartbeatIntervalSeconds > 0

	// Check gateway reachability from heartbeat tracking
	gatewayReachable := false
	lastHeartbeatSuccess := ""
	heartbeatConsecutiveFailures := 0
	if heartbeatEnabled && h.cfg.HeartbeatStatusFn != nil {
		hbStatus := h.cfg.HeartbeatStatusFn()
		gatewayReachable = hbStatus.Reachable
		heartbeatConsecutiveFailures = hbStatus.ConsecutiveFailures
		if !hbStatus.LastSuccess.IsZero() {
			lastHeartbeatSuccess = hbStatus.LastSuccess.UTC().Format(time.RFC3339)
		}

		// If heartbeat has been failing for > 120 seconds, degrade status
		if hbStatus.ConsecutiveFailures > 0 {
			sinceLastSuccess := time.Since(hbStatus.LastSuccess)
			// If we've never succeeded, use time since first attempt
			lastKnown := hbStatus.LastSuccess
			if lastKnown.IsZero() {
				// Degrade after a brief grace window even without any successful heartbeat
				// (the client starts sending immediately, so a few intervals is enough)
				if hbStatus.ConsecutiveFailures >= 3 {
					status = "degraded"
				}
			} else if sinceLastSuccess > 120*time.Second {
				status = "degraded"
			}
		}
	}

	WriteJSON(w, http.StatusOK, HealthzResponse{
		Status:                       status,
		Version:                      h.cfg.Version,
		Commit:                       version.Commit,
		BuildTime:                    version.BuildTime,
		NomadConnected:               nomadConnected,
		BodiesCount:                  bodiesCount,
		NodesCount:                   nodesCount,
		OrchestratorConnected:        nomadConnected,
		GatewayURL:                   h.cfg.GatewayURL,
		GatewayReachable:             gatewayReachable,
		LastHeartbeatSuccess:         lastHeartbeatSuccess,
		HeartbeatConsecutiveFailures: heartbeatConsecutiveFailures,
		HeartbeatEnabled:             heartbeatEnabled,
		SQLiteHealthy:                sqliteHealthy,
		StuckStartingCount:           stuckStartingCount,
	})
}
