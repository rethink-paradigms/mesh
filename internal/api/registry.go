package api

import (
	"encoding/json"
	"net/http"
)

// configureS3Registry handles POST /api/v1/registry/s3.
// Validates S3 credentials against the endpoint, creates the plugin,
// persists the config, and hot-swaps it into the running daemon.
func (h *Handler) configureS3Registry(w http.ResponseWriter, r *http.Request) {
	rm := h.cfg.RegistryManager
	if rm == nil {
		WriteError(w, ErrCodeInternal, "registry manager not available", http.StatusInternalServerError)
		return
	}

	var cfg S3RegistryConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		WriteError(w, ErrCodeBadRequest, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}
	if cfg.Bucket == "" {
		WriteError(w, ErrCodeBadRequest, "bucket is required", http.StatusBadRequest)
		return
	}
	if cfg.Region == "" {
		WriteError(w, ErrCodeBadRequest, "region is required", http.StatusBadRequest)
		return
	}

	if err := rm.ConfigureS3(r.Context(), cfg); err != nil {
		WriteError(w, ErrCodeBadRequest, err.Error(), http.StatusBadRequest)
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"status": "configured"})
}

// disconnectS3Registry handles DELETE /api/v1/registry/s3.
// Removes the S3 registry plugin from the running daemon.
// Future migrations fall back to same-machine transfer.
func (h *Handler) disconnectS3Registry(w http.ResponseWriter, r *http.Request) {
	rm := h.cfg.RegistryManager
	if rm == nil {
		WriteError(w, ErrCodeInternal, "registry manager not available", http.StatusInternalServerError)
		return
	}

	if err := rm.DisconnectS3(r.Context()); err != nil {
		WriteError(w, ErrCodeInternal, err.Error(), http.StatusInternalServerError)
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"status": "disconnected"})
}

// registryStatus handles GET /api/v1/registry/status.
func (h *Handler) registryStatus(w http.ResponseWriter, r *http.Request) {
	rm := h.cfg.RegistryManager
	if rm == nil {
		WriteJSON(w, http.StatusOK, RegistryStatusResponse{Configured: false})
		return
	}

	status := rm.RegistryStatus(r.Context())
	WriteJSON(w, http.StatusOK, status)
}
