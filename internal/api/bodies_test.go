package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rethink-paradigms/mesh/internal/body"
	"github.com/rethink-paradigms/mesh/internal/orchestrator"
	"github.com/rethink-paradigms/mesh/internal/service"
)

type mockBodyService struct {
	listFunc        func(ctx context.Context) ([]*body.Body, error)
	listByClusterFunc func(ctx context.Context, clusterID string) ([]*body.Body, error)
	createFunc      func(ctx context.Context, name, image string, opts orchestrator.BodySpec) (*body.Body, error)
	getFunc         func(ctx context.Context, id string) (*body.Body, error)
	getByClusterFunc func(ctx context.Context, id, clusterID string) (*body.Body, error)
	startFunc       func(ctx context.Context, id string) error
	stopFunc        func(ctx context.Context, id string) error
	destroyFunc     func(ctx context.Context, id string) error
	destroyByClusterFunc func(ctx context.Context, id, clusterID string) error
	getStatusFunc   func(ctx context.Context, id string) (orchestrator.BodyStatus, error)
}

func (m *mockBodyService) List(ctx context.Context) ([]*body.Body, error) {
	if m.listFunc != nil {
		return m.listFunc(ctx)
	}
	return nil, nil
}

func (m *mockBodyService) Create(ctx context.Context, name, image string, opts orchestrator.BodySpec) (*body.Body, error) {
	if m.createFunc != nil {
		return m.createFunc(ctx, name, image, opts)
	}
	return nil, nil
}

func (m *mockBodyService) Get(ctx context.Context, id string) (*body.Body, error) {
	if m.getFunc != nil {
		return m.getFunc(ctx, id)
	}
	return nil, nil
}

func (m *mockBodyService) Start(ctx context.Context, id string) error {
	if m.startFunc != nil {
		return m.startFunc(ctx, id)
	}
	return nil
}

func (m *mockBodyService) Stop(ctx context.Context, id string) error {
	if m.stopFunc != nil {
		return m.stopFunc(ctx, id)
	}
	return nil
}

func (m *mockBodyService) Destroy(ctx context.Context, id string) error {
	if m.destroyFunc != nil {
		return m.destroyFunc(ctx, id)
	}
	return nil
}

func (m *mockBodyService) GetStatus(ctx context.Context, id string) (orchestrator.BodyStatus, error) {
	if m.getStatusFunc != nil {
		return m.getStatusFunc(ctx, id)
	}
	return orchestrator.BodyStatus{}, nil
}

func (m *mockBodyService) ListByCluster(ctx context.Context, clusterID string) ([]*body.Body, error) {
	if m.listByClusterFunc != nil {
		return m.listByClusterFunc(ctx, clusterID)
	}
	return nil, nil
}

func (m *mockBodyService) GetByCluster(ctx context.Context, id, clusterID string) (*body.Body, error) {
	if m.getByClusterFunc != nil {
		return m.getByClusterFunc(ctx, id, clusterID)
	}
	return nil, nil
}

func (m *mockBodyService) DestroyByCluster(ctx context.Context, id, clusterID string) error {
	if m.destroyByClusterFunc != nil {
		return m.destroyByClusterFunc(ctx, id, clusterID)
	}
	return nil
}

func TestMapServiceError_NotFound(t *testing.T) {
	err := &service.NotFoundError{ID: "test-body"}
	code, status := mapServiceError(err)
	if status != http.StatusNotFound {
		t.Errorf("status = %d, want %d", status, http.StatusNotFound)
	}
	if code != ErrCodeBodyNotFound {
		t.Errorf("code = %q, want %q", code, ErrCodeBodyNotFound)
	}
}

func TestMapServiceError_Conflict(t *testing.T) {
	err := &service.ConflictError{State: "Running", Required: "Stopped"}
	code, status := mapServiceError(err)
	if status != http.StatusConflict {
		t.Errorf("status = %d, want %d", status, http.StatusConflict)
	}
	if code != ErrCodeBodyConflict {
		t.Errorf("code = %q, want %q", code, ErrCodeBodyConflict)
	}
}

func TestMapServiceError_Validation(t *testing.T) {
	err := &service.ValidationError{Field: "name", Message: "name is required"}
	code, status := mapServiceError(err)
	if status != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", status, http.StatusBadRequest)
	}
	if code != ErrCodeBadRequest {
		t.Errorf("code = %q, want %q", code, ErrCodeBadRequest)
	}
}

func TestMapServiceError_Unknown(t *testing.T) {
	err := &service.NotFoundError{ID: "test-body"}
	code, status := mapServiceError(err)
	if status != http.StatusNotFound {
		t.Errorf("status = %d, want %d", status, http.StatusNotFound)
	}
	if code != ErrCodeBodyNotFound {
		t.Errorf("code = %q, want %q", code, ErrCodeBodyNotFound)
	}
}

func TestHandleGetBody_NotFoundError(t *testing.T) {
	mock := &mockBodyService{
		getFunc: func(ctx context.Context, id string) (*body.Body, error) {
			return nil, &service.NotFoundError{ID: id}
		},
	}
	cfg := RouterConfig{BodyService: mock}
	h := NewHandler(cfg)

	req := httptest.NewRequest("GET", "/api/v1/bodies/missing-id", nil)
	req.SetPathValue("id", "missing-id")
	rr := httptest.NewRecorder()
	h.GetBody(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusNotFound)
	}

	var resp ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Error.Code != ErrCodeBodyNotFound {
		t.Errorf("error code = %q, want %q", resp.Error.Code, ErrCodeBodyNotFound)
	}
}

func TestHandleStartBody_ConflictError(t *testing.T) {
	mock := &mockBodyService{
		startFunc: func(ctx context.Context, id string) error {
			return &service.ConflictError{State: "Running", Required: "Stopped"}
		},
	}
	cfg := RouterConfig{BodyService: mock}
	h := NewHandler(cfg)

	req := httptest.NewRequest("POST", "/api/v1/bodies/test-id/start", nil)
	req.SetPathValue("id", "test-id")
	rr := httptest.NewRecorder()
	h.StartBody(rr, req)

	if rr.Code != http.StatusConflict {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusConflict)
	}

	var resp ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Error.Code != ErrCodeBodyConflict {
		t.Errorf("error code = %q, want %q", resp.Error.Code, ErrCodeBodyConflict)
	}
}

func TestHandleCreateBody_ValidationError(t *testing.T) {
	mock := &mockBodyService{
		createFunc: func(ctx context.Context, name, image string, opts orchestrator.BodySpec) (*body.Body, error) {
			return nil, &service.ValidationError{Field: "image", Message: "image is required"}
		},
	}
	cfg := RouterConfig{BodyService: mock}
	h := NewHandler(cfg)

	req := httptest.NewRequest("POST", "/api/v1/bodies", nil)
	rr := httptest.NewRecorder()
	h.CreateBody(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}

	var resp ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Error.Code != ErrCodeBadRequest {
		t.Errorf("error code = %q, want %q", resp.Error.Code, ErrCodeBadRequest)
	}
}

func TestHandleStopBody_NotFoundError(t *testing.T) {
	mock := &mockBodyService{
		stopFunc: func(ctx context.Context, id string) error {
			return &service.NotFoundError{ID: id}
		},
	}
	cfg := RouterConfig{BodyService: mock}
	h := NewHandler(cfg)

	req := httptest.NewRequest("POST", "/api/v1/bodies/missing-id/stop", nil)
	req.SetPathValue("id", "missing-id")
	rr := httptest.NewRecorder()
	h.StopBody(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusNotFound)
	}

	var resp ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Error.Code != ErrCodeBodyNotFound {
		t.Errorf("error code = %q, want %q", resp.Error.Code, ErrCodeBodyNotFound)
	}
}

func TestHandleDestroyBody_ConflictError(t *testing.T) {
	mock := &mockBodyService{
		destroyFunc: func(ctx context.Context, id string) error {
			return &service.ConflictError{State: "Running", Required: "Stopped or Error"}
		},
	}
	cfg := RouterConfig{BodyService: mock}
	h := NewHandler(cfg)

	req := httptest.NewRequest("DELETE", "/api/v1/bodies/test-id", nil)
	req.SetPathValue("id", "test-id")
	rr := httptest.NewRecorder()
	h.DestroyBody(rr, req)

	if rr.Code != http.StatusConflict {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusConflict)
	}

	var resp ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Error.Code != ErrCodeBodyConflict {
		t.Errorf("error code = %q, want %q", resp.Error.Code, ErrCodeBodyConflict)
	}
}
