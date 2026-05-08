package service

import (
	"context"
	"fmt"
	"time"

	"github.com/rethink-paradigms/mesh/internal/body"
	"github.com/rethink-paradigms/mesh/internal/orchestrator"
	"github.com/rethink-paradigms/mesh/internal/store"
)

// BodyService consolidates body lifecycle validation from REST and MCP handlers.
type BodyService struct {
	bodyMgr      *body.BodyManager
	store        *store.Store
	orchRegistry *orchestrator.Registry
}

// NewBodyService creates a new BodyService.
func NewBodyService(bm *body.BodyManager, s *store.Store, or *orchestrator.Registry) *BodyService {
	return &BodyService{bodyMgr: bm, store: s, orchRegistry: or}
}

// Create validates required fields, resolves the substrate, and delegates to BodyManager.Create.
func (s *BodyService) Create(ctx context.Context, name, image string, opts orchestrator.BodySpec) (*body.Body, error) {
	if name == "" {
		return nil, &ValidationError{Field: "name", Message: "name is required"}
	}
	if image == "" {
		return nil, &ValidationError{Field: "image", Message: "image is required"}
	}

	substrate := "local"
	if s.orchRegistry != nil {
		names := s.orchRegistry.List()
		switch len(names) {
		case 0:
			return nil, &ValidationError{Field: "substrate", Message: "no substrate available: no orchestrators registered"}
		case 1:
			substrate = names[0]
		default:
			adp, err := s.orchRegistry.Default()
			if err != nil {
				return nil, &ValidationError{Field: "substrate", Message: fmt.Sprintf("substrate required when multiple orchestrators registered; available: %v", names)}
			}
			substrate = adp.Name()
		}
	}

	b, err := s.bodyMgr.Create(ctx, name, opts)
	if err != nil {
		return nil, err
	}
	b.Substrate = substrate
	return b, nil
}

// Start gets the body, validates state is Stopped, and delegates to BodyManager.Start.
func (s *BodyService) Start(ctx context.Context, id string) error {
	b, err := s.bodyMgr.Get(ctx, id)
	if err != nil {
		return &NotFoundError{ID: id}
	}
	if b.State != orchestrator.StateStopped {
		return &ConflictError{State: string(b.State), Required: "Stopped"}
	}
	return s.bodyMgr.Start(ctx, id)
}

// Stop gets the body, validates state is Running or Starting, and delegates to BodyManager.Stop.
func (s *BodyService) Stop(ctx context.Context, id string) error {
	b, err := s.bodyMgr.Get(ctx, id)
	if err != nil {
		return &NotFoundError{ID: id}
	}
	if b.State != orchestrator.StateRunning && b.State != orchestrator.StateStarting {
		return &ConflictError{State: string(b.State), Required: "Running or Starting"}
	}
	return s.bodyMgr.Stop(ctx, id, orchestrator.StopOpts{Timeout: 30 * time.Second})
}

// Destroy gets the body, validates it is not Running, and delegates to BodyManager.Destroy.
func (s *BodyService) Destroy(ctx context.Context, id string) error {
	b, err := s.bodyMgr.Get(ctx, id)
	if err != nil {
		return &NotFoundError{ID: id}
	}
	if b.State == orchestrator.StateRunning {
		return &ConflictError{State: string(b.State), Required: "Stopped or Error"}
	}
	return s.bodyMgr.Destroy(ctx, id)
}

// Get delegates to BodyManager.Get and wraps nil/error in NotFoundError.
func (s *BodyService) Get(ctx context.Context, id string) (*body.Body, error) {
	b, err := s.bodyMgr.Get(ctx, id)
	if err != nil {
		return nil, &NotFoundError{ID: id}
	}
	return b, nil
}

// List delegates to BodyManager.List.
func (s *BodyService) List(ctx context.Context) ([]*body.Body, error) {
	return s.bodyMgr.List(ctx)
}

// Exec gets the body, validates state is Running, and delegates to BodyManager.Exec.
func (s *BodyService) Exec(ctx context.Context, id string, cmd []string) (orchestrator.ExecResult, error) {
	b, err := s.bodyMgr.Get(ctx, id)
	if err != nil {
		return orchestrator.ExecResult{}, &NotFoundError{ID: id}
	}
	if b.State != orchestrator.StateRunning {
		return orchestrator.ExecResult{}, &ConflictError{State: string(b.State), Required: "Running"}
	}
	return s.bodyMgr.Exec(ctx, id, cmd)
}

// GetStatus delegates to BodyManager.GetStatus.
func (s *BodyService) GetStatus(ctx context.Context, id string) (orchestrator.BodyStatus, error) {
	return s.bodyMgr.GetStatus(ctx, id)
}
