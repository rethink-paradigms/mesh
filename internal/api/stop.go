package api

import (
	"context"
	"net/http"
	"time"
)

// @Summary Stop the daemon
// @Description Gracefully shuts down the daemon. Returns immediately with 202 Accepted; the daemon stops asynchronously.
// @Tags system
// @Security BearerAuth
// @Success 202 {object} map[string]string
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/stop [post]
func (h *Handler) StopDaemon(w http.ResponseWriter, r *http.Request) {
	if h.cfg.StopDaemon == nil {
		WriteError(w, "internal_error", "stop not available", http.StatusInternalServerError)
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_ = h.cfg.StopDaemon(ctx)
	}()
	WriteJSON(w, http.StatusAccepted, map[string]string{"status": "stopping"})
}
