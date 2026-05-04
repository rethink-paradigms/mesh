package api

import (
	"net/http"

	"github.com/rethink-paradigms/mesh/internal/body"
	"github.com/rethink-paradigms/mesh/internal/ingress"
	"github.com/rethink-paradigms/mesh/internal/orchestrator"
	"github.com/rethink-paradigms/mesh/internal/store"
)

type RouterConfig struct {
	BodyManager  *body.BodyManager
	Store        *store.Store
	Orchestrator orchestrator.OrchestratorAdapter
	Ingress      ingress.IngressAdapter
	AuthToken    string
	Version      string
}

func NewRouter(cfg RouterConfig) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", handleHealthz(cfg))

	apiMux := http.NewServeMux()
	apiMux.HandleFunc("GET /api/v1/bodies", handleListBodies(cfg))
	apiMux.HandleFunc("POST /api/v1/bodies", handleCreateBody(cfg))
	apiMux.HandleFunc("GET /api/v1/bodies/{id}", handleGetBody(cfg))
	apiMux.HandleFunc("POST /api/v1/bodies/{id}/stop", handleStopBody(cfg))
	apiMux.HandleFunc("POST /api/v1/bodies/{id}/start", handleStartBody(cfg))
	apiMux.HandleFunc("DELETE /api/v1/bodies/{id}", handleDestroyBody(cfg))
	apiMux.HandleFunc("GET /api/v1/nodes", handleListNodes(cfg))

	mux.Handle("/api/v1/", BearerAuth(cfg.AuthToken, apiMux))

	return jsonContentType(mux)
}

func jsonContentType(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		next.ServeHTTP(w, r)
	})
}
