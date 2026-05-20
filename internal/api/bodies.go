package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/rethink-paradigms/mesh/internal/body"
	"github.com/rethink-paradigms/mesh/internal/orchestrator"
	"github.com/rethink-paradigms/mesh/internal/service"
)

func mapServiceError(err error) (code string, status int) {
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

// @Summary List all bodies
// @Description Returns all agent bodies managed by this daemon. Supports optional cluster-scoped filtering via the X-Cluster-ID header.
// @Tags bodies
// @Security BearerAuth
// @Success 200 {object} ListBodiesResponse
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/bodies [get]
func (h *Handler) ListBodies(w http.ResponseWriter, r *http.Request) {
	clusterID := ClusterIDFromContext(r.Context())

	var bodies []*body.Body
	var err error
	if clusterID != "" {
		bodies, err = h.cfg.BodyService.ListByCluster(r.Context(), clusterID)
	} else {
		bodies, err = h.cfg.BodyService.List(r.Context())
	}
	if err != nil {
		code, status := mapServiceError(err)
		WriteError(w, code, fmt.Sprintf("list bodies: %v", err), status)
		return
	}

	responses := make([]BodyResponse, 0, len(bodies))
	for _, b := range bodies {
		status, err := h.cfg.BodyService.GetStatus(r.Context(), b.ID)
		if err != nil {
			slog.Warn("failed to get body status during list", "body_id", b.ID, "error", err)
		}
		responses = append(responses, bodyToResponse(b, status))
	}

	WriteJSON(w, http.StatusOK, ListBodiesResponse{Bodies: responses})
}

// @Summary Create a new body
// @Description Creates a new agent body with the specified image, resources, and configuration.
// @Tags bodies
// @Security BearerAuth
// @Param body body CreateBodyRequest true "Body creation request"
// @Success 201 {object} CreateBodyResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/bodies [post]
func (h *Handler) CreateBody(w http.ResponseWriter, r *http.Request) {
	var req CreateBodyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, ErrCodeBadRequest, fmt.Sprintf("decode request: %v", err), http.StatusBadRequest)
		return
	}

	spec := requestToSpec(req)
	b, err := h.cfg.BodyService.Create(r.Context(), req.Name, req.Image, spec)
	if err != nil {
		code, status := mapServiceError(err)
		WriteError(w, code, fmt.Sprintf("create body: %v", err), status)
		return
	}

	WriteJSON(w, http.StatusCreated, CreateBodyResponse{
		ID:      b.ID,
		State:   string(b.State),
		Message: "Body created",
	})
}

// @Summary Get body by ID
// @Description Returns details for a single agent body by its unique identifier.
// @Tags bodies
// @Security BearerAuth
// @Param id path string true "Body ID"
// @Success 200 {object} BodyResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/bodies/{id} [get]
func (h *Handler) GetBody(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		WriteError(w, ErrCodeBadRequest, "body id is required", http.StatusBadRequest)
		return
	}

	clusterID := ClusterIDFromContext(r.Context())

	var b *body.Body
	var err error
	if clusterID != "" {
		b, err = h.cfg.BodyService.GetByCluster(r.Context(), id, clusterID)
	} else {
		b, err = h.cfg.BodyService.Get(r.Context(), id)
	}
	if err != nil {
		code, status := mapServiceError(err)
		WriteError(w, code, fmt.Sprintf("get body: %v", err), status)
		return
	}

	status, err := h.cfg.BodyService.GetStatus(r.Context(), id)
	if err != nil {
		slog.Warn("failed to get body status", "body_id", id, "error", err)
	}
	resp := bodyToResponse(b, status)
	WriteJSON(w, http.StatusOK, resp)
}

// @Summary Stop a body
// @Description Initiates a graceful stop of a running agent body.
// @Tags bodies
// @Security BearerAuth
// @Param id path string true "Body ID"
// @Success 200 {object} ActionResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/bodies/{id}/stop [post]
func (h *Handler) StopBody(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		WriteError(w, ErrCodeBadRequest, "body id is required", http.StatusBadRequest)
		return
	}

	if err := h.cfg.BodyService.Stop(r.Context(), id); err != nil {
		code, status := mapServiceError(err)
		WriteError(w, code, fmt.Sprintf("stop body: %v", err), status)
		return
	}

	WriteJSON(w, http.StatusOK, ActionResponse{
		ID:    id,
		State: "stopping",
	})
}

// @Summary Start a stopped body
// @Description Starts a previously stopped agent body.
// @Tags bodies
// @Security BearerAuth
// @Param id path string true "Body ID"
// @Success 200 {object} ActionResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/bodies/{id}/start [post]
func (h *Handler) StartBody(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		WriteError(w, ErrCodeBadRequest, "body id is required", http.StatusBadRequest)
		return
	}

	if err := h.cfg.BodyService.Start(r.Context(), id); err != nil {
		code, status := mapServiceError(err)
		WriteError(w, code, fmt.Sprintf("start body: %v", err), status)
		return
	}

	WriteJSON(w, http.StatusOK, ActionResponse{
		ID:    id,
		State: "starting",
	})
}

// @Summary Destroy a body
// @Description Permanently destroys an agent body and its associated resources.
// @Tags bodies
// @Security BearerAuth
// @Param id path string true "Body ID"
// @Success 200 {object} ActionResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/bodies/{id} [delete]
func (h *Handler) DestroyBody(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		WriteError(w, ErrCodeBadRequest, "body id is required", http.StatusBadRequest)
		return
	}

	clusterID := ClusterIDFromContext(r.Context())

	var err error
	if clusterID != "" {
		err = h.cfg.BodyService.DestroyByCluster(r.Context(), id, clusterID)
	} else {
		err = h.cfg.BodyService.Destroy(r.Context(), id)
	}
	if err != nil {
		code, status := mapServiceError(err)
		WriteError(w, code, fmt.Sprintf("destroy body: %v", err), status)
		return
	}

	WriteJSON(w, http.StatusOK, ActionResponse{
		ID:    id,
		State: "destroyed",
	})
}

// @Summary Bulk destroy bodies
// @Description Permanently destroys multiple agent bodies and their associated resources.
// @Tags bodies
// @Security BearerAuth
// @Param body body BulkDestroyBodiesRequest true "Bulk destroy request"
// @Success 200 {object} BulkDestroyBodiesResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/bodies [delete]
func (h *Handler) BulkDestroyBodies(w http.ResponseWriter, r *http.Request) {
	var req BulkDestroyBodiesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, ErrCodeBadRequest, fmt.Sprintf("decode request: %v", err), http.StatusBadRequest)
		return
	}

	if len(req.IDs) == 0 {
		WriteError(w, ErrCodeBadRequest, "no body IDs provided", http.StatusBadRequest)
		return
	}

	clusterID := ClusterIDFromContext(r.Context())

	var resp BulkDestroyBodiesResponse
	for _, id := range req.IDs {
		var err error
		if clusterID != "" {
			err = h.cfg.BodyService.DestroyByCluster(r.Context(), id, clusterID)
		} else {
			err = h.cfg.BodyService.Destroy(r.Context(), id)
		}
		if err != nil {
			resp.Failed++
			resp.Failures = append(resp.Failures, BulkDestroyFailure{
				ID:    id,
				Error: err.Error(),
			})
		} else {
			resp.Destroyed++
		}
	}

	WriteJSON(w, http.StatusOK, resp)
}

func bodyToResponse(b *body.Body, status orchestrator.BodyStatus) BodyResponse {
	resp := BodyResponse{
		ID:        b.ID,
		Name:      b.Name,
		Image:     b.Spec.Image,
		State:     string(b.State),
		Resources: ResourceSpec{CPUMHZ: b.Spec.CPUShares, MemoryMB: b.Spec.MemoryMB},
		Ports:     make(map[string]PortInfo),
	}

	if b.InstanceID != "" {
		resp.NodeID = string(b.InstanceID)
	}

	for _, alloc := range b.PortAllocations {
		resp.Ports[alloc.Name] = PortInfo{
			HostPort: alloc.HostPort,
			Domain:   alloc.AccessURL,
		}
	}

	if !status.StartedAt.IsZero() {
		resp.StartedAt = status.StartedAt.Format(time.RFC3339)
		resp.UptimeSeconds = int64(status.Uptime.Seconds())
	}

	return resp
}

func requestToSpec(req CreateBodyRequest) orchestrator.BodySpec {
	ports := make([]orchestrator.BodyPort, len(req.Ports))
	for i, p := range req.Ports {
		ports[i] = orchestrator.BodyPort{
			Name:          p.Name,
			ContainerPort: p.ContainerPort,
			Protocol:      p.Protocol,
			Expose:        p.Expose,
		}
	}
	return orchestrator.BodySpec{
		Image:     req.Image,
		Workdir:   "/workspace",
		Env:       req.Env,
		Cmd:       req.Command,
		MemoryMB:  req.Resources.MemoryMB,
		CPUShares: req.Resources.CPUMHZ,
		Ports:     ports,
	}
}
