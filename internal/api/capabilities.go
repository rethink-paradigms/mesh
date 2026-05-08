package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os/exec"
)

func handleCapabilities(cfg RouterConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var orchCaps []OrchestratorCapability
		if cfg.OrchRegistry != nil {
			for _, name := range cfg.OrchRegistry.List() {
				adapter, err := cfg.OrchRegistry.Open(name)
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

		features := cfg.Features
		if features == nil {
			features = make(map[string]bool)
		}

		tier := cfg.Tier
		if tier == "" {
			tier = "lite"
		}

		limits := cfg.Limits
		if limits.MaxBodies == 0 {
			limits.MaxBodies = 10
		}
		if limits.MaxSnapshots == 0 {
			limits.MaxSnapshots = 5
		}

		WriteJSON(w, http.StatusOK, CapabilitiesResponse{
			Version:       cfg.Version,
			Tier:          tier,
			Orchestrators: orchCaps,
			Providers:     providers,
			Features:      features,
			Limits:        limits,
		})
	}
}

// getProviders runs mesh-provision providers --output json and returns the output.
// If mesh-provision is not available, returns a JSON status: "unavailable".
func getProviders() json.RawMessage {
	cmd := exec.Command("mesh-provision", "providers", "--output", "json")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		fallback, _ := json.Marshal(map[string]interface{}{
			"status":    "unavailable",
			"providers": []interface{}{},
		})
		return fallback
	}

	return json.RawMessage(stdout.Bytes())
}
