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

func TestHealthzNoConsulConnectedField(t *testing.T) {
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

	// Verify the JSON response does NOT contain consul_connected key
	var rawResp map[string]interface{}
	err := json.NewDecoder(rr.Body).Decode(&rawResp)
	require.NoError(t, err)

	_, exists := rawResp["consul_connected"]
	assert.False(t, exists, "consul_connected field should not exist in healthz response")
}

func TestHealthzStandardNomadHealthy(t *testing.T) {
	reg := orchestrator.NewRegistry()
	reg.Register("docker", &mockOrchAdapter{name: "docker", healthy: false})
	reg.Register("nomad", &mockOrchAdapter{name: "nomad", healthy: true})
	_ = reg.SetDefault("nomad")

	cfg := RouterConfig{
		Version:      "0.1.0",
		Tier:         "STANDARD",
		OrchRegistry: reg,
		Orchestrator: &mockOrchAdapter{name: "nomad", healthy: true},
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
	assert.True(t, resp.NomadConnected)
}

func TestHealthzLiteDockerHealthy(t *testing.T) {
	reg := orchestrator.NewRegistry()
	reg.Register("docker", &mockOrchAdapter{name: "docker", healthy: true})
	_ = reg.SetDefault("docker")

	cfg := RouterConfig{
		Version:      "0.1.0",
		Tier:         "LITE",
		OrchRegistry: reg,
		Orchestrator: &mockOrchAdapter{name: "docker", healthy: true},
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
}

func TestHealthzLiteNeverDegraded(t *testing.T) {
	reg := orchestrator.NewRegistry()
	reg.Register("docker", &mockOrchAdapter{name: "docker", healthy: false})
	_ = reg.SetDefault("docker")

	cfg := RouterConfig{
		Version:      "0.1.0",
		Tier:         "LITE",
		OrchRegistry: reg,
		Orchestrator: &mockOrchAdapter{name: "docker", healthy: false},
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
}
