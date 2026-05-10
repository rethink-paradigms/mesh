package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rethink-paradigms/mesh/internal/orchestrator"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHealthzWithHeartbeatEnabled(t *testing.T) {
	reg := orchestrator.NewRegistry()
	reg.Register("docker", &mockOrchAdapter{name: "docker", healthy: true})

	cfg := RouterConfig{
		Version:                  "0.1.0",
		OrchRegistry:             reg,
		Orchestrator:             &mockOrchAdapter{name: "docker", healthy: true},
		GatewayURL:               "https://gateway.example.com",
		HeartbeatIntervalSeconds: 30,
	}
	h := NewHandler(cfg)

	req := httptest.NewRequest("GET", "/healthz", nil)
	rr := httptest.NewRecorder()
	h.Healthz(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)

	var resp HealthzResponse
	err := json.NewDecoder(rr.Body).Decode(&resp)
	require.NoError(t, err)

	assert.Equal(t, "healthy", resp.Status)
	assert.Equal(t, "https://gateway.example.com", resp.GatewayURL)
	assert.True(t, resp.HeartbeatEnabled)
}

func TestHealthzWithHeartbeatDisabledEmptyURL(t *testing.T) {
	reg := orchestrator.NewRegistry()
	reg.Register("docker", &mockOrchAdapter{name: "docker", healthy: true})

	cfg := RouterConfig{
		Version:                  "0.1.0",
		OrchRegistry:             reg,
		Orchestrator:             &mockOrchAdapter{name: "docker", healthy: true},
		GatewayURL:               "",
		HeartbeatIntervalSeconds: 30,
	}
	h := NewHandler(cfg)

	req := httptest.NewRequest("GET", "/healthz", nil)
	rr := httptest.NewRecorder()
	h.Healthz(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)

	var resp HealthzResponse
	err := json.NewDecoder(rr.Body).Decode(&resp)
	require.NoError(t, err)

	assert.Equal(t, "", resp.GatewayURL)
	assert.False(t, resp.HeartbeatEnabled)
}

func TestHealthzWithHeartbeatDisabledZeroInterval(t *testing.T) {
	reg := orchestrator.NewRegistry()
	reg.Register("docker", &mockOrchAdapter{name: "docker", healthy: true})

	cfg := RouterConfig{
		Version:                  "0.1.0",
		OrchRegistry:             reg,
		Orchestrator:             &mockOrchAdapter{name: "docker", healthy: true},
		GatewayURL:               "https://gateway.example.com",
		HeartbeatIntervalSeconds: 0,
	}
	h := NewHandler(cfg)

	req := httptest.NewRequest("GET", "/healthz", nil)
	rr := httptest.NewRecorder()
	h.Healthz(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)

	var resp HealthzResponse
	err := json.NewDecoder(rr.Body).Decode(&resp)
	require.NoError(t, err)

	assert.Equal(t, "https://gateway.example.com", resp.GatewayURL)
	assert.False(t, resp.HeartbeatEnabled)
}
