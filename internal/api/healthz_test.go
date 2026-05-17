package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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

func TestHealthzGatewayReachable(t *testing.T) {
	reg := orchestrator.NewRegistry()
	reg.Register("docker", &mockOrchAdapter{name: "docker", healthy: true})

	cfg := RouterConfig{
		Version:                  "0.1.0",
		OrchRegistry:             reg,
		Orchestrator:             &mockOrchAdapter{name: "docker", healthy: true},
		GatewayURL:               "https://gateway.example.com",
		HeartbeatIntervalSeconds: 30,
		HeartbeatStatusFn: func() GatewayHeartbeatStatus {
			return GatewayHeartbeatStatus{
				Reachable:           true,
				LastSuccess:         time.Now().Add(-5 * time.Second),
				ConsecutiveFailures: 0,
			}
		},
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
	assert.True(t, resp.GatewayReachable)
	assert.True(t, resp.HeartbeatEnabled)
	assert.NotEmpty(t, resp.LastHeartbeatSuccess)
	assert.Equal(t, 0, resp.HeartbeatConsecutiveFailures)
}

func TestHealthzGatewayDegradedAfterFailure(t *testing.T) {
	reg := orchestrator.NewRegistry()
	reg.Register("docker", &mockOrchAdapter{name: "docker", healthy: true})

	// Simulate heartbeats failing for > 120s
	longAgo := time.Now().Add(-130 * time.Second)
	cfg := RouterConfig{
		Version:                  "0.1.0",
		OrchRegistry:             reg,
		Orchestrator:             &mockOrchAdapter{name: "docker", healthy: true},
		GatewayURL:               "https://gateway.example.com",
		HeartbeatIntervalSeconds: 30,
		HeartbeatStatusFn: func() GatewayHeartbeatStatus {
			return GatewayHeartbeatStatus{
				Reachable:           false,
				LastSuccess:         longAgo,
				ConsecutiveFailures: 5,
				LastError:           "connection refused",
			}
		},
	}
	h := NewHandler(cfg)

	req := httptest.NewRequest("GET", "/healthz", nil)
	rr := httptest.NewRecorder()
	h.Healthz(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)

	var resp HealthzResponse
	err := json.NewDecoder(rr.Body).Decode(&resp)
	require.NoError(t, err)

	assert.Equal(t, "degraded", resp.Status)
	assert.False(t, resp.GatewayReachable)
	assert.Equal(t, 5, resp.HeartbeatConsecutiveFailures)
	assert.NotEmpty(t, resp.LastHeartbeatSuccess)
}

func TestHealthzGatewayDegradedNoInitialHeartbeat(t *testing.T) {
	reg := orchestrator.NewRegistry()
	reg.Register("docker", &mockOrchAdapter{name: "docker", healthy: true})

	cfg := RouterConfig{
		Version:                  "0.1.0",
		OrchRegistry:             reg,
		Orchestrator:             &mockOrchAdapter{name: "docker", healthy: true},
		GatewayURL:               "https://gateway.example.com",
		HeartbeatIntervalSeconds: 30,
		HeartbeatStatusFn: func() GatewayHeartbeatStatus {
			return GatewayHeartbeatStatus{
				Reachable:           false,
				ConsecutiveFailures: 3, // 3 failures without any success → degraded
				LastError:           "connection refused",
			}
		},
	}
	h := NewHandler(cfg)

	req := httptest.NewRequest("GET", "/healthz", nil)
	rr := httptest.NewRecorder()
	h.Healthz(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)

	var resp HealthzResponse
	err := json.NewDecoder(rr.Body).Decode(&resp)
	require.NoError(t, err)

	assert.Equal(t, "degraded", resp.Status, "should degrade after 3+ consecutive failures with no prior success")
	assert.False(t, resp.GatewayReachable)
}

func TestHealthzGatewayNoInitialHeartbeatStillHealthy(t *testing.T) {
	reg := orchestrator.NewRegistry()
	reg.Register("docker", &mockOrchAdapter{name: "docker", healthy: true})

	cfg := RouterConfig{
		Version:                  "0.1.0",
		OrchRegistry:             reg,
		Orchestrator:             &mockOrchAdapter{name: "docker", healthy: true},
		GatewayURL:               "https://gateway.example.com",
		HeartbeatIntervalSeconds: 30,
		HeartbeatStatusFn: func() GatewayHeartbeatStatus {
			return GatewayHeartbeatStatus{
				Reachable:           false,
				ConsecutiveFailures: 1, // 1 failure is within grace window
				LastError:           "connection refused",
			}
		},
	}
	h := NewHandler(cfg)

	req := httptest.NewRequest("GET", "/healthz", nil)
	rr := httptest.NewRecorder()
	h.Healthz(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)

	var resp HealthzResponse
	err := json.NewDecoder(rr.Body).Decode(&resp)
	require.NoError(t, err)

	// Should still be healthy — only 1 failure is within grace window
	// Other factors (nomad_connected, sqlite) are fine
	assert.Equal(t, "healthy", resp.Status, "should be healthy with only 1 failure (within grace window)")
	assert.False(t, resp.GatewayReachable, "should report not reachable")
	assert.Equal(t, 1, resp.HeartbeatConsecutiveFailures)
}

func TestHealthzGatewayFieldsWithoutHeartbeatStatusFn(t *testing.T) {
	reg := orchestrator.NewRegistry()
	reg.Register("docker", &mockOrchAdapter{name: "docker", healthy: true})

	cfg := RouterConfig{
		Version:                  "0.1.0",
		OrchRegistry:             reg,
		Orchestrator:             &mockOrchAdapter{name: "docker", healthy: true},
		GatewayURL:               "https://gateway.example.com",
		HeartbeatIntervalSeconds: 30,
		HeartbeatStatusFn:        nil, // no status function provided
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
	assert.False(t, resp.GatewayReachable, "gateway_reachable should be false when no status function")
	assert.Empty(t, resp.LastHeartbeatSuccess)
	assert.Equal(t, 0, resp.HeartbeatConsecutiveFailures)
}

func TestHealthzHeartbeatNotEnabledNoStatusFn(t *testing.T) {
	reg := orchestrator.NewRegistry()
	reg.Register("docker", &mockOrchAdapter{name: "docker", healthy: true})

	cfg := RouterConfig{
		Version:                  "0.1.0",
		OrchRegistry:             reg,
		Orchestrator:             &mockOrchAdapter{name: "docker", healthy: true},
		GatewayURL:               "",
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

	assert.Equal(t, "healthy", resp.Status)
	assert.False(t, resp.HeartbeatEnabled)
	assert.False(t, resp.GatewayReachable)
	assert.Empty(t, resp.GatewayURL)
}

func TestHealthzOrchestratorUnhealthyAlwaysDegraded(t *testing.T) {
	// Orchestrator health always matters — if unhealthy, status is "degraded"
	// regardless of tier (solo or cluster both run Nomad).
	reg := orchestrator.NewRegistry()
	reg.Register("docker", &mockOrchAdapter{name: "docker", healthy: false})
	_ = reg.SetDefault("docker")

	cfg := RouterConfig{
		Version:      "0.1.0",
		Tier:         "solo",
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

	assert.Equal(t, "degraded", resp.Status)
}
