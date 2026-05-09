package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
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

func handleListBodies(cfg RouterConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		clusterID := ClusterIDFromContext(r.Context())

		var bodies []*body.Body
		var err error
		if clusterID != "" {
			bodies, err = cfg.BodyService.ListByCluster(r.Context(), clusterID)
		} else {
			bodies, err = cfg.BodyService.List(r.Context())
		}
		if err != nil {
			code, status := mapServiceError(err)
			WriteError(w, code, fmt.Sprintf("list bodies: %v", err), status)
			return
		}

		responses := make([]BodyResponse, 0, len(bodies))
		for _, b := range bodies {
			status, err := cfg.BodyService.GetStatus(r.Context(), b.ID)
			if err != nil {
				fmt.Fprintf(os.Stderr, "api: get status for body %s: %v\n", b.ID, err)
			}
			responses = append(responses, bodyToResponse(b, status))
		}

		WriteJSON(w, http.StatusOK, ListBodiesResponse{Bodies: responses})
	}
}

func handleCreateBody(cfg RouterConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req CreateBodyRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			WriteError(w, ErrCodeBadRequest, fmt.Sprintf("decode request: %v", err), http.StatusBadRequest)
			return
		}

		spec := requestToSpec(req)
		b, err := cfg.BodyService.Create(r.Context(), req.Name, req.Image, spec)
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
}

func handleGetBody(cfg RouterConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id == "" {
			WriteError(w, ErrCodeBadRequest, "body id is required", http.StatusBadRequest)
			return
		}

		clusterID := ClusterIDFromContext(r.Context())

		var b *body.Body
		var err error
		if clusterID != "" {
			b, err = cfg.BodyService.GetByCluster(r.Context(), id, clusterID)
		} else {
			b, err = cfg.BodyService.Get(r.Context(), id)
		}
		if err != nil {
			code, status := mapServiceError(err)
			WriteError(w, code, fmt.Sprintf("get body: %v", err), status)
			return
		}

		status, err := cfg.BodyService.GetStatus(r.Context(), id)
		if err != nil {
			fmt.Fprintf(os.Stderr, "api: get status for body %s: %v\n", id, err)
		}
		resp := bodyToResponse(b, status)
		WriteJSON(w, http.StatusOK, resp)
	}
}

func handleStopBody(cfg RouterConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id == "" {
			WriteError(w, ErrCodeBadRequest, "body id is required", http.StatusBadRequest)
			return
		}

		if err := cfg.BodyService.Stop(r.Context(), id); err != nil {
			code, status := mapServiceError(err)
			WriteError(w, code, fmt.Sprintf("stop body: %v", err), status)
			return
		}

		WriteJSON(w, http.StatusOK, ActionResponse{
			ID:    id,
			State: "stopping",
		})
	}
}

func handleStartBody(cfg RouterConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id == "" {
			WriteError(w, ErrCodeBadRequest, "body id is required", http.StatusBadRequest)
			return
		}

		if err := cfg.BodyService.Start(r.Context(), id); err != nil {
			code, status := mapServiceError(err)
			WriteError(w, code, fmt.Sprintf("start body: %v", err), status)
			return
		}

		WriteJSON(w, http.StatusOK, ActionResponse{
			ID:    id,
			State: "starting",
		})
	}
}

func handleDestroyBody(cfg RouterConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id == "" {
			WriteError(w, ErrCodeBadRequest, "body id is required", http.StatusBadRequest)
			return
		}

		clusterID := ClusterIDFromContext(r.Context())

		var err error
		if clusterID != "" {
			err = cfg.BodyService.DestroyByCluster(r.Context(), id, clusterID)
		} else {
			err = cfg.BodyService.Destroy(r.Context(), id)
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

	if !status.StartedAt.IsZero() {
		resp.StartedAt = status.StartedAt.Format(time.RFC3339)
		resp.UptimeSeconds = int64(status.Uptime.Seconds())
	}

	return resp
}

func requestToSpec(req CreateBodyRequest) orchestrator.BodySpec {
	return orchestrator.BodySpec{
		Image:     req.Image,
		Workdir:   "/workspace",
		Env:       req.Env,
		Cmd:       req.Command,
		MemoryMB:  req.Resources.MemoryMB,
		CPUShares: req.Resources.CPUMHZ,
	}
}
