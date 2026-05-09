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

func createJWTValidator(cfg RouterConfig) *JWTValidator {
	if cfg.JWTValidator != nil {
		return cfg.JWTValidator
	}
	if cfg.Auth0Domain == "" || cfg.Auth0Audience == "" {
		return nil
	}
	validator, err := NewJWTValidator(cfg.Auth0Domain, cfg.Auth0Audience, cfg.ClusterOwnerID)
	if err != nil {
		return nil
	}
	return validator
}

type RouterConfig struct {
	BodyManager  *body.BodyManager
	BodyService  bodyServiceAdapter
	Store        *store.Store
	Orchestrator orchestrator.OrchestratorAdapter
	Ingress      ingress.IngressAdapter
	AuthToken     string
	AuthMode      string
	Auth0Domain   string
	Auth0Audience string
	ClusterOwnerID string
	ClusterID     string
	Version       string
	Tier         string
	OrchRegistry *orchestrator.Registry
	Features     map[string]bool
	Limits       CapabilityLimits
	Uptime       time.Time // daemon start time, used for uptime calculation
	Installer    Installer
	JWTValidator *JWTValidator
}

type bodyServiceAdapter interface {
	List(ctx context.Context) ([]*body.Body, error)
	ListByCluster(ctx context.Context, clusterID string) ([]*body.Body, error)
	Create(ctx context.Context, name, image string, opts orchestrator.BodySpec) (*body.Body, error)
	Get(ctx context.Context, id string) (*body.Body, error)
	GetByCluster(ctx context.Context, id, clusterID string) (*body.Body, error)
	Start(ctx context.Context, id string) error
	Stop(ctx context.Context, id string) error
	Destroy(ctx context.Context, id string) error
	DestroyByCluster(ctx context.Context, id, clusterID string) error
	GetStatus(ctx context.Context, id string) (orchestrator.BodyStatus, error)
}

var _ bodyServiceAdapter = (*service.BodyService)(nil)

func NewRouter(cfg RouterConfig) http.Handler {
	mux := http.NewServeMux()

	h := NewHandler(cfg)
	mux.HandleFunc("GET /healthz", h.Healthz)

	apiMux := http.NewServeMux()
	apiMux.HandleFunc("GET /api/v1/bodies", h.ListBodies)
	apiMux.HandleFunc("POST /api/v1/bodies", h.CreateBody)
	apiMux.HandleFunc("GET /api/v1/bodies/{id}", h.GetBody)
	apiMux.HandleFunc("POST /api/v1/bodies/{id}/stop", h.StopBody)
	apiMux.HandleFunc("POST /api/v1/bodies/{id}/start", h.StartBody)
	apiMux.HandleFunc("DELETE /api/v1/bodies/{id}", h.DestroyBody)
	apiMux.HandleFunc("GET /api/v1/nodes", h.ListNodes)
	apiMux.HandleFunc("GET /api/v1/capabilities", h.Capabilities)
	apiMux.HandleFunc("GET /api/v1/status", h.Status)
	apiMux.HandleFunc("POST /api/v1/agents/install", h.InstallAgent)

	var validator *JWTValidator
	if cfg.AuthMode == "jwt" || cfg.AuthMode == "both" {
		validator = createJWTValidator(cfg)
	}

	mux.Handle("/api/v1/", JWTOrTokenAuth(cfg, validator, apiMux))

	return jsonContentType(mux)
}

func jsonContentType(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		next.ServeHTTP(w, r)
	})
}
