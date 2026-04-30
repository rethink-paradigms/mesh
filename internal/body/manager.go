package body

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"

	"github.com/google/uuid"
	"github.com/rethink-paradigms/mesh/internal/adapter"
	"github.com/rethink-paradigms/mesh/internal/store"
)

// BodyManager orchestrates body lifecycle operations against a store and substrate adapter.
type BodyManager struct {
	store   *store.Store
	adapter adapter.SubstrateAdapter
	mu      sync.Mutex
	bodies  map[string]*Body
}

// NewBodyManager creates a new BodyManager.
func NewBodyManager(s *store.Store, a adapter.SubstrateAdapter) *BodyManager {
	return &BodyManager{
		store:   s,
		adapter: a,
		bodies:  make(map[string]*Body),
	}
}

func (bm *BodyManager) getOrCreateBody(id string) *Body {
	bm.mu.Lock()
	defer bm.mu.Unlock()
	if b, ok := bm.bodies[id]; ok {
		return b
	}
	b := &Body{ID: id}
	bm.bodies[id] = b
	return b
}

func specToJSON(spec adapter.BodySpec) string {
	data, _ := json.Marshal(spec)
	return string(data)
}

// Create creates a new body: inserts a store record, provisions on the substrate,
// and transitions from Created → Starting → Running.
func (bm *BodyManager) Create(ctx context.Context, name string, spec adapter.BodySpec) (*Body, error) {
	id := uuid.New().String()

	b := bm.getOrCreateBody(id)
	b.mu.Lock()
	defer b.mu.Unlock()

	handle, err := bm.adapter.Create(ctx, spec)
	if err != nil {
		return nil, fmt.Errorf("adapter create: %w", err)
	}

	specJSON := specToJSON(spec)
	if err := bm.store.CreateBody(ctx, id, name, adapter.StateCreated, specJSON, "local", string(handle)); err != nil {
		return nil, fmt.Errorf("store create body: %w", err)
	}

	b.ID = id
	b.Name = name
	b.State = adapter.StateCreated
	b.InstanceID = handle
	b.Spec = spec
	b.Substrate = "local"

	if err := bm.transitionPersisted(ctx, b, adapter.StateStarting); err != nil {
		return nil, err
	}

	if err := bm.adapter.Start(ctx, handle); err != nil {
		_ = bm.transitionPersisted(ctx, b, adapter.StateError)
		return nil, fmt.Errorf("adapter start: %w", err)
	}

	if err := bm.transitionPersisted(ctx, b, adapter.StateRunning); err != nil {
		return nil, err
	}

	return b, nil
}

// Start resumes a stopped body: transitions Stopped → Starting → Running.
func (bm *BodyManager) Start(ctx context.Context, bodyID string) error {
	b := bm.getOrCreateBody(bodyID)
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.State != adapter.StateStopped && b.State != adapter.StateCreated {
		return fmt.Errorf("cannot start body in state %s (must be Stopped or Created)", b.State)
	}

	if err := bm.transitionPersisted(ctx, b, adapter.StateStarting); err != nil {
		return err
	}

	if err := bm.adapter.Start(ctx, b.InstanceID); err != nil {
		_ = bm.transitionPersisted(ctx, b, adapter.StateError)
		return fmt.Errorf("adapter start: %w", err)
	}

	return bm.transitionPersisted(ctx, b, adapter.StateRunning)
}

// Stop stops a running body: transitions Running → Stopping → Stopped.
func (bm *BodyManager) Stop(ctx context.Context, bodyID string, opts adapter.StopOpts) error {
	b := bm.getOrCreateBody(bodyID)
	b.mu.Lock()
	defer b.mu.Unlock()

	if err := bm.transitionPersisted(ctx, b, adapter.StateStopping); err != nil {
		return err
	}

	if err := bm.adapter.Stop(ctx, b.InstanceID, opts); err != nil {
		_ = bm.transitionPersisted(ctx, b, adapter.StateError)
		return fmt.Errorf("adapter stop: %w", err)
	}

	return bm.transitionPersisted(ctx, b, adapter.StateStopped)
}

// Destroy destroys a stopped or errored body.
func (bm *BodyManager) Destroy(ctx context.Context, bodyID string) error {
	b := bm.getOrCreateBody(bodyID)
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.State != adapter.StateStopped && b.State != adapter.StateError {
		return fmt.Errorf("cannot destroy body in state %s (must be Stopped or Error)", b.State)
	}

	if err := bm.adapter.Destroy(ctx, b.InstanceID); err != nil {
		return fmt.Errorf("adapter destroy: %w", err)
	}

	if err := bm.transitionPersisted(ctx, b, adapter.StateDestroyed); err != nil {
		return err
	}

	if err := bm.store.DeleteBody(ctx, bodyID); err != nil {
		return fmt.Errorf("store delete body: %w", err)
	}

	bm.mu.Lock()
	delete(bm.bodies, bodyID)
	bm.mu.Unlock()

	return nil
}

// GetStatus returns the current status of a body by combining store and adapter state.
func (bm *BodyManager) GetStatus(ctx context.Context, bodyID string) (adapter.BodyStatus, error) {
	b := bm.getOrCreateBody(bodyID)
	b.mu.Lock()
	defer b.mu.Unlock()

	status, err := bm.adapter.GetStatus(ctx, b.InstanceID)
	if err != nil {
		return adapter.BodyStatus{}, fmt.Errorf("adapter get status: %w", err)
	}

	return status, nil
}

// List returns all bodies from the store, refreshing the in-memory cache.
func (bm *BodyManager) List(ctx context.Context) ([]*Body, error) {
	records, err := bm.store.ListBodies(ctx)
	if err != nil {
		return nil, err
	}

	var bodies []*Body
	for _, rec := range records {
		b := bm.getOrCreateBody(rec.ID)
		b.mu.Lock()
		b.ID = rec.ID
		b.Name = rec.Name
		b.State = rec.State
		b.InstanceID = adapter.Handle(rec.InstanceID)
		b.Substrate = rec.Substrate
		b.mu.Unlock()
		bodies = append(bodies, b)
	}

	return bodies, nil
}

func (bm *BodyManager) ExportFilesystem(ctx context.Context, bodyID string) (io.ReadCloser, error) {
	b := bm.getOrCreateBody(bodyID)
	b.mu.Lock()
	defer b.mu.Unlock()

	rc, err := bm.adapter.ExportFilesystem(ctx, b.InstanceID)
	if err != nil {
		return nil, fmt.Errorf("adapter export filesystem: %w", err)
	}
	return rc, nil
}

// Get retrieves a single body by ID from the store.
func (bm *BodyManager) Get(ctx context.Context, bodyID string) (*Body, error) {
	rec, err := bm.store.GetBody(ctx, bodyID)
	if err != nil {
		return nil, err
	}

	b := bm.getOrCreateBody(rec.ID)
	b.mu.Lock()
	defer b.mu.Unlock()

	b.ID = rec.ID
	b.Name = rec.Name
	b.State = rec.State
	b.InstanceID = adapter.Handle(rec.InstanceID)
	b.Substrate = rec.Substrate

	return b, nil
}

func (bm *BodyManager) transitionPersisted(ctx context.Context, b *Body, target adapter.BodyState) error {
	if err := b.Transition(target); err != nil {
		return err
	}
	return bm.store.UpdateBodyState(ctx, b.ID, target)
}

func (bm *BodyManager) TransitionBody(ctx context.Context, bodyID string, target adapter.BodyState) error {
	rec, err := bm.store.GetBody(ctx, bodyID)
	if err != nil {
		return fmt.Errorf("get body %s: %w", bodyID, err)
	}

	b := bm.getOrCreateBody(rec.ID)
	b.mu.Lock()
	defer b.mu.Unlock()

	b.ID = rec.ID
	b.Name = rec.Name
	b.State = rec.State
	b.InstanceID = adapter.Handle(rec.InstanceID)
	b.Substrate = rec.Substrate

	return bm.transitionPersisted(ctx, b, target)
}

func (bm *BodyManager) Exec(ctx context.Context, bodyID string, cmd []string) (adapter.ExecResult, error) {
	b := bm.getOrCreateBody(bodyID)
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.State != adapter.StateRunning {
		return adapter.ExecResult{}, fmt.Errorf("cannot exec in body %s: state is %s (must be Running)", bodyID, b.State)
	}

	return bm.adapter.Exec(ctx, b.InstanceID, cmd)
}
