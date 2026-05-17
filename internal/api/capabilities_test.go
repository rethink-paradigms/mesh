package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rethink-paradigms/mesh/internal/orchestrator"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockOrchAdapter struct {
	name    string
	healthy bool
}

func (m *mockOrchAdapter) ScheduleBody(ctx context.Context, spec orchestrator.BodySpec) (orchestrator.Handle, error) {
	return "", nil
}

func (m *mockOrchAdapter) StartBody(ctx context.Context, id orchestrator.Handle) error { return nil }
func (m *mockOrchAdapter) StopBody(ctx context.Context, id orchestrator.Handle) error  { return nil }
func (m *mockOrchAdapter) DestroyBody(ctx context.Context, id orchestrator.Handle) error {
	return nil
}
func (m *mockOrchAdapter) GetBodyStatus(ctx context.Context, id orchestrator.Handle) (orchestrator.BodyStatus, error) {
	return orchestrator.BodyStatus{}, nil
}
func (m *mockOrchAdapter) Name() string                       { return m.name }
func (m *mockOrchAdapter) IsHealthy(ctx context.Context) bool { return m.healthy }

func TestHandleCapabilities(t *testing.T) {
	reg := orchestrator.NewRegistry()
	err := reg.Register("docker", &mockOrchAdapter{name: "docker", healthy: true})
	require.NoError(t, err)

	cfg := RouterConfig{
		Version:      "0.1.0",
		Tier:         "lite",
		OrchRegistry: reg,
		Features:     map[string]bool{"snapshots": true, "migration": false},
	}
	h := NewHandler(cfg)

	req := httptest.NewRequest("GET", "/api/v1/capabilities", nil)
	rr := httptest.NewRecorder()
	h.Capabilities(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)

	var resp CapabilitiesResponse
	err = json.NewDecoder(rr.Body).Decode(&resp)
	require.NoError(t, err)

	assert.Equal(t, "0.1.0", resp.Version)
	assert.Equal(t, "lite", resp.Tier)
	assert.Len(t, resp.Orchestrators, 1)
	assert.Equal(t, "docker", resp.Orchestrators[0].Name)
	assert.True(t, resp.Orchestrators[0].Healthy)

	assert.Equal(t, true, resp.Features["snapshots"])
	assert.Equal(t, false, resp.Features["migration"])

	assert.Equal(t, 10, resp.Limits.MaxBodies)
	assert.Equal(t, 5, resp.Limits.MaxSnapshots)

	// Providers should show unavailable when mesh-provision is not in PATH
	require.NotNil(t, resp.Providers)
	var providers struct {
		Status    string          `json:"status"`
		Providers json.RawMessage `json:"providers"`
	}
	err = json.Unmarshal(resp.Providers, &providers)
	require.NoError(t, err)
	assert.Equal(t, "unavailable", providers.Status)
}

func TestHandleCapabilitiesMultipleOrchestrators(t *testing.T) {
	reg := orchestrator.NewRegistry()
	reg.Register("docker", &mockOrchAdapter{name: "docker", healthy: true})
	reg.Register("nomad", &mockOrchAdapter{name: "nomad", healthy: false})

	cfg := RouterConfig{
		Version:      "0.2.0",
		Tier:         "pro",
		OrchRegistry: reg,
		Features:     map[string]bool{},
	}
	h := NewHandler(cfg)

	req := httptest.NewRequest("GET", "/api/v1/capabilities", nil)
	rr := httptest.NewRecorder()
	h.Capabilities(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)

	var resp CapabilitiesResponse
	err := json.NewDecoder(rr.Body).Decode(&resp)
	require.NoError(t, err)

	require.Len(t, resp.Orchestrators, 2)
	assert.Equal(t, "docker", resp.Orchestrators[0].Name)
	assert.True(t, resp.Orchestrators[0].Healthy)
	assert.Equal(t, "nomad", resp.Orchestrators[1].Name)
	assert.False(t, resp.Orchestrators[1].Healthy)
}

func TestHandleCapabilitiesDefaultFeatures(t *testing.T) {
	reg := orchestrator.NewRegistry()
	reg.Register("docker", &mockOrchAdapter{name: "docker", healthy: true})

	cfg := RouterConfig{
		Version:      "0.1.0",
		Tier:         "",
		OrchRegistry: reg,
		Features:     nil,
	}
	h := NewHandler(cfg)

	req := httptest.NewRequest("GET", "/api/v1/capabilities", nil)
	rr := httptest.NewRecorder()
	h.Capabilities(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)

	var resp CapabilitiesResponse
	err := json.NewDecoder(rr.Body).Decode(&resp)
	require.NoError(t, err)

	assert.Equal(t, "solo", resp.Tier)
	assert.Empty(t, resp.Features)
}
