package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/rethink-paradigms/mesh/internal/body"
	"github.com/rethink-paradigms/mesh/internal/orchestrator"
)

func handleListBodies(cfg RouterConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		bodies, err := cfg.BodyManager.List(r.Context())
		if err != nil {
			WriteError(w, ErrCodeInternal, fmt.Sprintf("list bodies: %v", err), http.StatusInternalServerError)
			return
		}

		responses := make([]BodyResponse, 0, len(bodies))
		for _, b := range bodies {
			status, err := cfg.BodyManager.GetStatus(r.Context(), b.ID)
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

		if req.Name == "" {
			WriteError(w, ErrCodeBadRequest, "name is required", http.StatusBadRequest)
			return
		}
		if req.Image == "" {
			WriteError(w, ErrCodeBadRequest, "image is required", http.StatusBadRequest)
			return
		}

		spec := requestToSpec(req)
		b, err := cfg.BodyManager.Create(r.Context(), req.Name, spec)
		if err != nil {
			WriteError(w, ErrCodeInternal, fmt.Sprintf("create body: %v", err), http.StatusInternalServerError)
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

		b, err := cfg.BodyManager.Get(r.Context(), id)
		if err != nil {
			WriteError(w, ErrCodeBodyNotFound, fmt.Sprintf("body not found: %v", err), http.StatusNotFound)
			return
		}

		status, err := cfg.BodyManager.GetStatus(r.Context(), id)
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

		b, err := cfg.BodyManager.Get(r.Context(), id)
		if err != nil {
			WriteError(w, ErrCodeBodyNotFound, fmt.Sprintf("body not found: %v", err), http.StatusNotFound)
			return
		}

		state := string(b.State)
		if state != "Running" && state != "Starting" {
			WriteError(w, ErrCodeBodyConflict, "Body must be Running or Starting to stop", http.StatusConflict)
			return
		}

		if err := cfg.BodyManager.Stop(r.Context(), id, orchestrator.StopOpts{Timeout: 30 * time.Second}); err != nil {
			WriteError(w, ErrCodeInternal, fmt.Sprintf("stop body: %v", err), http.StatusInternalServerError)
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

		b, err := cfg.BodyManager.Get(r.Context(), id)
		if err != nil {
			WriteError(w, ErrCodeBodyNotFound, fmt.Sprintf("body not found: %v", err), http.StatusNotFound)
			return
		}

		if string(b.State) != "Stopped" {
			WriteError(w, ErrCodeBodyConflict, "Body must be Stopped to start", http.StatusConflict)
			return
		}

		if err := cfg.BodyManager.Start(r.Context(), id); err != nil {
			WriteError(w, ErrCodeInternal, fmt.Sprintf("start body: %v", err), http.StatusInternalServerError)
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

		if _, err := cfg.BodyManager.Get(r.Context(), id); err != nil {
			WriteError(w, ErrCodeBodyNotFound, fmt.Sprintf("body not found: %v", err), http.StatusNotFound)
			return
		}

		if err := cfg.BodyManager.Destroy(r.Context(), id); err != nil {
			WriteError(w, ErrCodeInternal, fmt.Sprintf("destroy body: %v", err), http.StatusInternalServerError)
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
