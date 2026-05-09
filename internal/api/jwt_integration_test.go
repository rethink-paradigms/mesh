package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/rethink-paradigms/mesh/internal/body"
	"github.com/rethink-paradigms/mesh/internal/orchestrator"
)

type mockBodyServiceJWT struct {
	listByClusterFunc func(ctx context.Context, clusterID string) ([]*body.Body, error)
	getByClusterFunc  func(ctx context.Context, id, clusterID string) (*body.Body, error)
}

func (m *mockBodyServiceJWT) List(ctx context.Context) ([]*body.Body, error)                       { return nil, nil }
func (m *mockBodyServiceJWT) ListByCluster(ctx context.Context, clusterID string) ([]*body.Body, error) {
	if m.listByClusterFunc != nil {
		return m.listByClusterFunc(ctx, clusterID)
	}
	return nil, nil
}
func (m *mockBodyServiceJWT) Create(ctx context.Context, name, image string, opts orchestrator.BodySpec) (*body.Body, error) {
	return nil, nil
}
func (m *mockBodyServiceJWT) Get(ctx context.Context, id string) (*body.Body, error) { return nil, nil }
func (m *mockBodyServiceJWT) GetByCluster(ctx context.Context, id, clusterID string) (*body.Body, error) {
	if m.getByClusterFunc != nil {
		return m.getByClusterFunc(ctx, id, clusterID)
	}
	return nil, nil
}
func (m *mockBodyServiceJWT) Start(ctx context.Context, id string) error  { return nil }
func (m *mockBodyServiceJWT) Stop(ctx context.Context, id string) error   { return nil }
func (m *mockBodyServiceJWT) Destroy(ctx context.Context, id string) error { return nil }
func (m *mockBodyServiceJWT) DestroyByCluster(ctx context.Context, id, clusterID string) error { return nil }
func (m *mockBodyServiceJWT) GetStatus(ctx context.Context, id string) (orchestrator.BodyStatus, error) {
	return orchestrator.BodyStatus{}, nil
}

func newTestRouterWithJWT(t *testing.T, cfg RouterConfig) (http.Handler, *httptest.Server) {
	t.Helper()
	router := NewRouter(cfg)
	ts := httptest.NewServer(router)
	t.Cleanup(func() { ts.Close() })
	return router, ts
}

func TestJWTIntegration_ValidTokenGrantsAccess(t *testing.T) {
	key := generateTestKeys(t)
	validator, server := newTestValidator(t, key, "https://mesh-api", "https://test-auth0.example.com/", "")
	defer server.Close()
	defer validator.Close()

	mock := &mockBodyServiceJWT{
		listByClusterFunc: func(ctx context.Context, clusterID string) ([]*body.Body, error) {
			return []*body.Body{{ID: "b1", Name: "test-body", State: orchestrator.StateRunning}}, nil
		},
	}

	cfg := RouterConfig{
		BodyService:    mock,
		AuthMode:       "jwt",
		Auth0Domain:    "test-auth0.example.com",
		Auth0Audience:  "https://mesh-api",
		ClusterOwnerID: "",
		JWTValidator:   validator,
	}

	_, ts := newTestRouterWithJWT(t, cfg)

	claims := jwt.MapClaims{
		"sub": "auth0|user123",
		"iss": "https://test-auth0.example.com/",
		"aud": "https://mesh-api",
		"exp": time.Now().Add(time.Hour).Unix(),
		"iat": time.Now().Unix(),
	}
	token := generateTestJWT(t, key, "test-key-1", claims)

	req, _ := http.NewRequest("GET", ts.URL+"/api/v1/bodies", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

func TestJWTIntegration_ExpiredTokenRejected(t *testing.T) {
	key := generateTestKeys(t)
	validator, server := newTestValidator(t, key, "https://mesh-api", "https://test-auth0.example.com/", "")
	defer server.Close()
	defer validator.Close()

	mock := &mockBodyServiceJWT{}
	cfg := RouterConfig{
		BodyService:    mock,
		AuthMode:       "jwt",
		Auth0Domain:    "test-auth0.example.com",
		Auth0Audience:  "https://mesh-api",
		ClusterOwnerID: "",
		JWTValidator:   validator,
	}

	_, ts := newTestRouterWithJWT(t, cfg)

	claims := jwt.MapClaims{
		"sub": "auth0|user123",
		"iss": "https://test-auth0.example.com/",
		"aud": "https://mesh-api",
		"exp": time.Now().Add(-time.Hour).Unix(),
		"iat": time.Now().Add(-2 * time.Hour).Unix(),
	}
	token := generateTestJWT(t, key, "test-key-1", claims)

	req, _ := http.NewRequest("GET", ts.URL+"/api/v1/bodies", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}

	var apiErr ErrorResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiErr); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if apiErr.Error.Code != ErrCodeUnauthorized {
		t.Errorf("error code = %q, want %q", apiErr.Error.Code, ErrCodeUnauthorized)
	}
}

func TestJWTIntegration_MissingTokenRejected(t *testing.T) {
	key := generateTestKeys(t)
	validator, server := newTestValidator(t, key, "https://mesh-api", "https://test-auth0.example.com/", "")
	defer server.Close()
	defer validator.Close()

	mock := &mockBodyServiceJWT{}
	cfg := RouterConfig{
		BodyService:    mock,
		AuthMode:       "jwt",
		Auth0Domain:    "test-auth0.example.com",
		Auth0Audience:  "https://mesh-api",
		ClusterOwnerID: "",
		JWTValidator:   validator,
	}

	_, ts := newTestRouterWithJWT(t, cfg)

	req, _ := http.NewRequest("GET", ts.URL+"/api/v1/bodies", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}

	var apiErr ErrorResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiErr); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if apiErr.Error.Code != ErrCodeUnauthorized {
		t.Errorf("error code = %q, want %q", apiErr.Error.Code, ErrCodeUnauthorized)
	}
}

func TestJWTIntegration_WrongOwnerRejected(t *testing.T) {
	key := generateTestKeys(t)
	validator, server := newTestValidator(t, key, "https://mesh-api", "https://test-auth0.example.com/", "auth0|owner123")
	defer server.Close()
	defer validator.Close()

	mock := &mockBodyServiceJWT{}
	cfg := RouterConfig{
		BodyService:    mock,
		AuthMode:       "jwt",
		Auth0Domain:    "test-auth0.example.com",
		Auth0Audience:  "https://mesh-api",
		ClusterOwnerID: "auth0|owner123",
		JWTValidator:   validator,
	}

	_, ts := newTestRouterWithJWT(t, cfg)

	claims := jwt.MapClaims{
		"sub": "auth0|user123",
		"iss": "https://test-auth0.example.com/",
		"aud": "https://mesh-api",
		"exp": time.Now().Add(time.Hour).Unix(),
		"iat": time.Now().Unix(),
	}
	token := generateTestJWT(t, key, "test-key-1", claims)

	req, _ := http.NewRequest("GET", ts.URL+"/api/v1/bodies", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}

	var apiErr ErrorResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiErr); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if apiErr.Error.Code != ErrCodeUnauthorized {
		t.Errorf("error code = %q, want %q", apiErr.Error.Code, ErrCodeUnauthorized)
	}
}

func TestJWTIntegration_BothMode_AcceptsToken(t *testing.T) {
	key := generateTestKeys(t)
	validator, server := newTestValidator(t, key, "https://mesh-api", "https://test-auth0.example.com/", "")
	defer server.Close()
	defer validator.Close()

	mock := &mockBodyServiceJWT{
		listByClusterFunc: func(ctx context.Context, clusterID string) ([]*body.Body, error) {
			return []*body.Body{{ID: "b1", Name: "test-body", State: orchestrator.StateRunning}}, nil
		},
	}

	cfg := RouterConfig{
		BodyService:    mock,
		AuthToken:      "daemon-secret",
		AuthMode:       "both",
		Auth0Domain:    "test-auth0.example.com",
		Auth0Audience:  "https://mesh-api",
		ClusterOwnerID: "",
		JWTValidator:   validator,
	}

	_, ts := newTestRouterWithJWT(t, cfg)

	req, _ := http.NewRequest("GET", ts.URL+"/api/v1/bodies", nil)
	req.Header.Set("Authorization", "Bearer daemon-secret")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

func TestJWTIntegration_BothMode_AcceptsJWT(t *testing.T) {
	key := generateTestKeys(t)
	validator, server := newTestValidator(t, key, "https://mesh-api", "https://test-auth0.example.com/", "")
	defer server.Close()
	defer validator.Close()

	var capturedClusterID string
	mock := &mockBodyServiceJWT{
		listByClusterFunc: func(ctx context.Context, clusterID string) ([]*body.Body, error) {
			capturedClusterID = clusterID
			return []*body.Body{{ID: "b1", Name: "test-body", State: orchestrator.StateRunning}}, nil
		},
	}

	cfg := RouterConfig{
		BodyService:    mock,
		AuthToken:      "daemon-secret",
		AuthMode:       "both",
		Auth0Domain:    "test-auth0.example.com",
		Auth0Audience:  "https://mesh-api",
		ClusterOwnerID: "",
		JWTValidator:   validator,
	}

	_, ts := newTestRouterWithJWT(t, cfg)

	claims := jwt.MapClaims{
		"sub": "auth0|user456",
		"iss": "https://test-auth0.example.com/",
		"aud": "https://mesh-api",
		"exp": time.Now().Add(time.Hour).Unix(),
		"iat": time.Now().Unix(),
	}
	token := generateTestJWT(t, key, "test-key-1", claims)

	req, _ := http.NewRequest("GET", ts.URL+"/api/v1/bodies", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if capturedClusterID != "auth0|user456" {
		t.Errorf("cluster_id = %q, want %q", capturedClusterID, "auth0|user456")
	}
}
