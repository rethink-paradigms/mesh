package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rethink-paradigms/mesh/internal/agent"
	"github.com/rethink-paradigms/mesh/internal/body"
	"github.com/rethink-paradigms/mesh/internal/service"
)

type mockInstaller struct {
	installFunc func(ctx context.Context, agentType, name string, env map[string]string, manifest string) (*agent.InstallResult, error)
}

func (m *mockInstaller) Install(ctx context.Context, agentType, name string, env map[string]string, manifest string) (*agent.InstallResult, error) {
	if m.installFunc != nil {
		return m.installFunc(ctx, agentType, name, env, manifest)
	}
	return &agent.InstallResult{BodyID: "test-id", Name: name}, nil
}

func TestHandleInstallAgent(t *testing.T) {
	installer := &mockInstaller{
		installFunc: func(ctx context.Context, agentType, name string, env map[string]string, manifest string) (*agent.InstallResult, error) {
			return &agent.InstallResult{
				BodyID:         "body-123",
				Name:           name,
				AccessURLs:     []string{"http://my-agent-8080.mesh.local"},
				AllocatedPorts: map[string]int{"api": 20001},
			}, nil
		},
	}

	cfg := RouterConfig{AuthToken: "test-token", Installer: installer}
	h := NewHandler(cfg)

	reqBody, _ := json.Marshal(InstallAgentRequest{
		AgentType: "hermes",
		Name:      "my-hermes",
		Env:       map[string]string{"OPENAI_API_KEY": "sk-test"},
	})
	req := httptest.NewRequest("POST", "/api/v1/agents/install", bytes.NewReader(reqBody))
	req.Header.Set("Authorization", "Bearer test-token")
	rr := httptest.NewRecorder()

	h.InstallAgent(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusCreated)
	}

	var result agent.InstallResult
	if err := json.NewDecoder(rr.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if result.BodyID != "body-123" {
		t.Errorf("BodyID = %q, want body-123", result.BodyID)
	}
	if result.Name != "my-hermes" {
		t.Errorf("Name = %q, want my-hermes", result.Name)
	}
}

func TestHandleInstallAgentNoAuth(t *testing.T) {
	cfg := RouterConfig{AuthToken: "test-token", Installer: &mockInstaller{}}
	router := NewRouter(cfg)

	reqBody, _ := json.Marshal(InstallAgentRequest{AgentType: "hermes", Name: "my-hermes"})
	req := httptest.NewRequest("POST", "/api/v1/agents/install", bytes.NewReader(reqBody))
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestHandleInstallAgentMissingType(t *testing.T) {
	cfg := RouterConfig{AuthToken: "test-token", Installer: &mockInstaller{}}
	h := NewHandler(cfg)

	reqBody, _ := json.Marshal(InstallAgentRequest{Name: "my-hermes"})
	req := httptest.NewRequest("POST", "/api/v1/agents/install", bytes.NewReader(reqBody))
	req.Header.Set("Authorization", "Bearer test-token")
	rr := httptest.NewRecorder()

	h.InstallAgent(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestHandleInstallAgentMissingName(t *testing.T) {
	cfg := RouterConfig{AuthToken: "test-token", Installer: &mockInstaller{}}
	h := NewHandler(cfg)

	reqBody, _ := json.Marshal(InstallAgentRequest{AgentType: "hermes"})
	req := httptest.NewRequest("POST", "/api/v1/agents/install", bytes.NewReader(reqBody))
	req.Header.Set("Authorization", "Bearer test-token")
	rr := httptest.NewRecorder()

	h.InstallAgent(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestHandleInstallAgentNotFound(t *testing.T) {
	installer := &mockInstaller{
		installFunc: func(ctx context.Context, agentType, name string, env map[string]string, manifest string) (*agent.InstallResult, error) {
			return nil, &service.NotFoundError{ID: agentType}
		},
	}

	cfg := RouterConfig{AuthToken: "test-token", Installer: installer}
	h := NewHandler(cfg)

	reqBody, _ := json.Marshal(InstallAgentRequest{AgentType: "unknown", Name: "my-agent"})
	req := httptest.NewRequest("POST", "/api/v1/agents/install", bytes.NewReader(reqBody))
	req.Header.Set("Authorization", "Bearer test-token")
	rr := httptest.NewRecorder()

	h.InstallAgent(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestHandleInstallAgentConflict(t *testing.T) {
	installer := &mockInstaller{
		installFunc: func(ctx context.Context, agentType, name string, env map[string]string, manifest string) (*agent.InstallResult, error) {
			return nil, &service.ConflictError{State: "exists", Required: "unique name"}
		},
	}

	cfg := RouterConfig{AuthToken: "test-token", Installer: installer}
	h := NewHandler(cfg)

	reqBody, _ := json.Marshal(InstallAgentRequest{AgentType: "hermes", Name: "dup"})
	req := httptest.NewRequest("POST", "/api/v1/agents/install", bytes.NewReader(reqBody))
	req.Header.Set("Authorization", "Bearer test-token")
	rr := httptest.NewRecorder()

	h.InstallAgent(rr, req)

	if rr.Code != http.StatusConflict {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusConflict)
	}
}

func TestHandleInstallAgentValidationError(t *testing.T) {
	installer := &mockInstaller{
		installFunc: func(ctx context.Context, agentType, name string, env map[string]string, manifest string) (*agent.InstallResult, error) {
			return nil, &service.ValidationError{Field: "env", Message: "required env var missing"}
		},
	}

	cfg := RouterConfig{AuthToken: "test-token", Installer: installer}
	h := NewHandler(cfg)

	reqBody, _ := json.Marshal(InstallAgentRequest{AgentType: "hermes", Name: "my-agent"})
	req := httptest.NewRequest("POST", "/api/v1/agents/install", bytes.NewReader(reqBody))
	req.Header.Set("Authorization", "Bearer test-token")
	rr := httptest.NewRecorder()

	h.InstallAgent(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestMapAgentInstallError(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantCode   string
		wantStatus int
	}{
		{"not found", &service.NotFoundError{ID: "x"}, ErrCodeBodyNotFound, http.StatusNotFound},
		{"conflict", &service.ConflictError{State: "x", Required: "y"}, ErrCodeBodyConflict, http.StatusConflict},
		{"validation", &service.ValidationError{Field: "x", Message: "y"}, ErrCodeBadRequest, http.StatusBadRequest},
		{"generic", &service.NotFoundError{ID: "x"}, ErrCodeBodyNotFound, http.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, status := mapAgentInstallError(tt.err)
			if code != tt.wantCode {
				t.Errorf("code = %q, want %q", code, tt.wantCode)
			}
			if status != tt.wantStatus {
				t.Errorf("status = %d, want %d", status, tt.wantStatus)
			}
		})
	}
}

func TestInstallAgentRequestManifestDeserialization(t *testing.T) {
	// RED phase test: manifest field should deserialize from JSON
	input := `{"agent_type":"test","name":"body1","manifest":"name: agent\nimage: test:latest"}`
	var req InstallAgentRequest
	if err := json.Unmarshal([]byte(input), &req); err != nil {
		t.Fatalf("unmarshal with manifest: %v", err)
	}
	if req.Manifest != "name: agent\nimage: test:latest" {
		t.Errorf("Manifest = %q, want %q", req.Manifest, "name: agent\nimage: test:latest")
	}
	if req.AgentType != "test" {
		t.Errorf("AgentType = %q, want test", req.AgentType)
	}
	if req.Name != "body1" {
		t.Errorf("Name = %q, want body1", req.Name)
	}
}

func TestInstallAgentRequestNoManifest(t *testing.T) {
	// Manifest should be empty string when not in JSON
	input := `{"agent_type":"test","name":"body1"}`
	var req InstallAgentRequest
	if err := json.Unmarshal([]byte(input), &req); err != nil {
		t.Fatalf("unmarshal without manifest: %v", err)
	}
	if req.Manifest != "" {
		t.Errorf("Manifest = %q, want empty string", req.Manifest)
	}
	// omitempty should suppress manifest in output when empty
	out, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if bytes.Contains(out, []byte("manifest")) {
		t.Errorf("output %q should not contain manifest when empty (omitempty)", string(out))
	}
}

func TestHandleInstallAgentNoInstaller(t *testing.T) {
	cfg := RouterConfig{AuthToken: "test-token", Installer: nil}
	h := NewHandler(cfg)

	reqBody, _ := json.Marshal(InstallAgentRequest{AgentType: "hermes", Name: "my-agent"})
	req := httptest.NewRequest("POST", "/api/v1/agents/install", bytes.NewReader(reqBody))
	req.Header.Set("Authorization", "Bearer test-token")
	rr := httptest.NewRecorder()

	h.InstallAgent(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
	}
}

func TestHandleInstallAgentRealInstaller(t *testing.T) {
	s := tempStore(t)
	bm := body.NewBodyManager(s, &mockOrchAdapter{name: "mock", healthy: true})

	manifests := map[string]*agent.AgentManifest{
		"test-agent": {
			Name:  "test-agent",
			Image: "test-image",
			Env:   agent.EnvConfig{Required: []string{"API_KEY"}},
		},
	}

	installer := agent.NewInstaller(bm, nil, nil, manifests)

	cfg := RouterConfig{AuthToken: "test-token", Installer: installer}
	h := NewHandler(cfg)

	reqBody, _ := json.Marshal(InstallAgentRequest{
		AgentType: "test-agent",
		Name:      "real-test",
		Env:       map[string]string{"API_KEY": "secret"},
	})
	req := httptest.NewRequest("POST", "/api/v1/agents/install", bytes.NewReader(reqBody))
	req.Header.Set("Authorization", "Bearer test-token")
	rr := httptest.NewRecorder()

	h.InstallAgent(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusCreated)
	}

	var result agent.InstallResult
	if err := json.NewDecoder(rr.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if result.Name != "real-test" {
		t.Errorf("Name = %q, want real-test", result.Name)
	}
	if result.BodyID == "" {
		t.Error("BodyID is empty")
	}
}


