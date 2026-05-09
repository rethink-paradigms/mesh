package api

import (
	"context"
	"net/http"
	"time"

	"github.com/rethink-paradigms/mesh/internal/body"
	"github.com/rethink-paradigms/mesh/internal/ingress"
	"github.com/rethink-paradigms/mesh/internal/orchestrator"
	"github.com/rethink-paradigms/mesh/internal/service"
	"github.com/rethink-paradigms/mesh/internal/store"
)

type RouterConfig struct {
	BodyManager  *body.BodyManager
	BodyService  bodyServiceAdapter
	Store        *store.Store
	Orchestrator orchestrator.OrchestratorAdapter
	Ingress      ingress.IngressAdapter
	AuthToken    string
	Version      string
	Tier         string
	OrchRegistry *orchestrator.Registry
	Features     map[string]bool
	Limits       CapabilityLimits
	Uptime       time.Time // daemon start time, used for uptime calculation
	Installer    Installer
}

type bodyServiceAdapter interface {
	List(ctx context.Context) ([]*body.Body, error)
	Create(ctx context.Context, name, image string, opts orchestrator.BodySpec) (*body.Body, error)
	Get(ctx context.Context, id string) (*body.Body, error)
	Start(ctx context.Context, id string) error
	Stop(ctx context.Context, id string) error
	Destroy(ctx context.Context, id string) error
	GetStatus(ctx context.Context, id string) (orchestrator.BodyStatus, error)
}

var _ bodyServiceAdapter = (*service.BodyService)(nil)

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
	apiMux.HandleFunc("GET /api/v1/capabilities", handleCapabilities(cfg))
	apiMux.HandleFunc("GET /api/v1/status", handleStatus(cfg))
	apiMux.HandleFunc("POST /api/v1/agents/install", handleInstallAgent(cfg))

	mux.Handle("/api/v1/", BearerAuth(cfg.AuthToken, apiMux))

	return jsonContentType(mux)
}

func jsonContentType(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		next.ServeHTTP(w, r)
	})
}
