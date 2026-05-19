package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os/exec"
)

// @Summary Get daemon capabilities
// @Description Returns daemon capabilities including registered orchestrator adapters, infrastructure providers (from external mesh-provision CLI call — may be unavailable), enabled feature flags, and hard limits. The providers field is populated by calling 'mesh-provision providers' externally and may return {"status":"unavailable"} if the CLI is not installed.
// @Tags system
// @Security BearerAuth
// @Success 200 {object} CapabilitiesResponse
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/capabilities [get]
func (h *Handler) Capabilities(w http.ResponseWriter, r *http.Request) {
	var orchCaps []OrchestratorCapability
	if h.cfg.OrchRegistry != nil {
		for _, name := range h.cfg.OrchRegistry.List() {
			adapter, err := h.cfg.OrchRegistry.Open(name)
			healthy := err == nil && adapter.IsHealthy(r.Context())
			orchCaps = append(orchCaps, OrchestratorCapability{
				Name:    name,
				Healthy: healthy,
			})
		}
	}
	if orchCaps == nil {
		orchCaps = []OrchestratorCapability{}
	}

	providers := getProviders()

	features := h.cfg.Features
	if features == nil {
		features = make(map[string]bool)
	}

	tier := h.cfg.Tier
	if tier == "" {
		tier = "solo"
	}

	limits := h.cfg.Limits
	if limits.MaxBodies == 0 {
		limits.MaxBodies = 10
	}
	if limits.MaxSnapshots == 0 {
		limits.MaxSnapshots = 5
	}

	WriteJSON(w, http.StatusOK, CapabilitiesResponse{
		Version:       h.cfg.Version,
		Tier:          tier,
		Orchestrators: orchCaps,
		Providers:     providers,
		Features:      features,
		Limits:        limits,
	})
}

// getProviders runs mesh-provision providers --output json and returns the output.
// If mesh-provision is not available, returns a JSON status: "unavailable".
func getProviders() json.RawMessage {
	cmd := exec.Command("mesh-provision", "providers", "--output", "json")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		fallback, _ := json.Marshal(map[string]any{
			"status":    "unavailable",
			"providers": []any{},
		})
		return fallback
	}

	return json.RawMessage(stdout.Bytes())
}
