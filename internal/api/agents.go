package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/rethink-paradigms/mesh/internal/agent"
	"github.com/rethink-paradigms/mesh/internal/service"
)

type Installer interface {
	Install(ctx context.Context, agentType, name string, env map[string]string) (*agent.InstallResult, error)
}

// InstallAgentRequest is the request payload for POST /api/v1/agents/install.
type InstallAgentRequest struct {
	AgentType string            `json:"agent_type"`
	Name      string            `json:"name"`
	Env       map[string]string `json:"env,omitempty"`
}

// @Summary Install an agent
// @Description Installs an agent (e.g., Hermes, OpenClaw) on the daemon with the specified type, name, and environment variables.
// @Tags agents
// @Security BearerAuth
// @Param body body InstallAgentRequest true "Agent install request"
// @Success 201 {object} agent.InstallResult
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/agents/install [post]
func (h *Handler) InstallAgent(w http.ResponseWriter, r *http.Request) {
	if h.cfg.Installer == nil {
		WriteError(w, ErrCodeInternal, "installer not configured", http.StatusInternalServerError)
		return
	}

	var req InstallAgentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, ErrCodeBadRequest, fmt.Sprintf("decode request: %v", err), http.StatusBadRequest)
		return
	}

	if req.AgentType == "" {
		WriteError(w, ErrCodeBadRequest, "agent_type is required", http.StatusBadRequest)
		return
	}
	if req.Name == "" {
		WriteError(w, ErrCodeBadRequest, "name is required", http.StatusBadRequest)
		return
	}

	result, err := h.cfg.Installer.Install(r.Context(), req.AgentType, req.Name, req.Env)
	if err != nil {
		code, status := mapAgentInstallError(err)
		WriteError(w, code, fmt.Sprintf("install agent: %v", err), status)
		return
	}

	WriteJSON(w, http.StatusCreated, result)
}

func mapAgentInstallError(err error) (code string, status int) {
	var notFound *service.NotFoundError
	if errors.As(err, &notFound) {
		return ErrCodeBodyNotFound, http.StatusNotFound
	}
	var conflict *service.ConflictError
	if errors.As(err, &conflict) {
		return ErrCodeBodyConflict, http.StatusConflict
	}
	var validation *service.ValidationError
	if errors.As(err, &validation) {
		return ErrCodeBadRequest, http.StatusBadRequest
	}
	return ErrCodeInternal, http.StatusInternalServerError
}
