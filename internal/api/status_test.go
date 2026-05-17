package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/rethink-paradigms/mesh/internal/ingress"
	"github.com/rethink-paradigms/mesh/internal/orchestrator"
	"github.com/rethink-paradigms/mesh/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func tempStore(t *testing.T) *store.Store {
	t.Helper()
	f, err := os.CreateTemp("", "mesh-api-test-*.db")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	path := f.Name()
	f.Close()
	s, err := store.Open(path)
	if err != nil {
		os.Remove(path)
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() {
		s.Close()
		os.Remove(path)
	})
	return s
}

func TestHandleStatus(t *testing.T) {
	s := tempStore(t)
	ctx := context.Background()

	require.NoError(t, s.CreateBody(ctx, "b1", "body-alpha", orchestrator.StateRunning, `{}`, "docker", "inst-1"))
	require.NoError(t, s.CreateBody(ctx, "b2", "body-beta", orchestrator.StateStopped, `{}`, "docker", ""))

	reg := orchestrator.NewRegistry()
	require.NoError(t, reg.Register("docker", &mockOrchAdapter{name: "docker", healthy: true}))

	startTime := time.Now().Add(-2 * time.Hour)

	cfg := RouterConfig{
		Version:      "0.1.0",
		Tier:         "lite",
		Store:        s,
		Uptime:       startTime,
		Ingress:      ingress.NewNoopAdapter(),
		OrchRegistry: reg,
		Orchestrator: &mockOrchAdapter{name: "docker", healthy: true},
	}
	h := NewHandler(cfg)

	req := httptest.NewRequest("GET", "/api/v1/status", nil)
	rr := httptest.NewRecorder()
	h.Status(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)

	var resp StatusResponse
	err := json.NewDecoder(rr.Body).Decode(&resp)
	require.NoError(t, err)

	// Daemon section
	assert.Equal(t, "0.1.0", resp.Daemon.Version)
	assert.Greater(t, resp.Daemon.UptimeSec, int64(0))
	assert.Contains(t, resp.Daemon.StartTime, "T") // RFC3339

	// Tier section
	assert.Equal(t, "lite", resp.Tier)

	// Bodies section
	assert.Equal(t, 2, resp.Bodies.Total)
	assert.Equal(t, 1, resp.Bodies.Running)
	assert.Equal(t, 1, resp.Bodies.Stopped)
	assert.Equal(t, 0, resp.Bodies.Error)
	assert.Len(t, resp.Bodies.List, 2)

	// Ports section - NoopAdapter returns zero values
	assert.Equal(t, 0, resp.Ports.Used)
	assert.Equal(t, 0, resp.Ports.Free)
	assert.Equal(t, 0, resp.Ports.PoolStart)
	assert.Equal(t, 0, resp.Ports.PoolEnd)

	// Ingress section
	assert.Equal(t, 0, resp.Ingress.RouteCount)

	// Capacity section
	assert.GreaterOrEqual(t, resp.Capacity.CPUPercent, float64(0))
}

func TestHandleStatusBodyCounts(t *testing.T) {
	s := tempStore(t)
	ctx := context.Background()

	require.NoError(t, s.CreateBody(ctx, "b1", "running-1", orchestrator.StateRunning, `{}`, "docker", "inst-1"))
	require.NoError(t, s.CreateBody(ctx, "b2", "running-2", orchestrator.StateRunning, `{}`, "docker", "inst-2"))
	require.NoError(t, s.CreateBody(ctx, "b3", "stopped-1", orchestrator.StateStopped, `{}`, "docker", ""))
	require.NoError(t, s.CreateBody(ctx, "b4", "error-1", orchestrator.StateError, `{}`, "docker", ""))

	cfg := RouterConfig{
		Store:        s,
		Version:      "1.0.0",
		Tier:         "pro",
		Ingress:      ingress.NewNoopAdapter(),
		Orchestrator: &mockOrchAdapter{name: "docker", healthy: true},
		Uptime:       time.Now(),
	}
	h := NewHandler(cfg)

	req := httptest.NewRequest("GET", "/api/v1/status", nil)
	rr := httptest.NewRecorder()
	h.Status(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)

	var resp StatusResponse
	err := json.NewDecoder(rr.Body).Decode(&resp)
	require.NoError(t, err)

	assert.Equal(t, 4, resp.Bodies.Total)
	assert.Equal(t, 2, resp.Bodies.Running)
	assert.Equal(t, 1, resp.Bodies.Stopped)
	assert.Equal(t, 1, resp.Bodies.Error)

	// Verify list entries
	assert.Len(t, resp.Bodies.List, 4)
	for _, item := range resp.Bodies.List {
		switch item.ID {
		case "b1":
			assert.Equal(t, "running-1", item.Name)
			assert.Equal(t, "Running", item.State)
		case "b2":
			assert.Equal(t, "running-2", item.Name)
			assert.Equal(t, "Running", item.State)
		case "b3":
			assert.Equal(t, "stopped-1", item.Name)
			assert.Equal(t, "Stopped", item.State)
		case "b4":
			assert.Equal(t, "error-1", item.Name)
			assert.Equal(t, "Error", item.State)
		default:
			t.Errorf("unexpected body id: %s", item.ID)
		}
	}
}

func TestHandleStatusPortTracking(t *testing.T) {
	s := tempStore(t)
	ctx := context.Background()

	require.NoError(t, s.CreateBody(ctx, "b1", "body-1", orchestrator.StateRunning, `{}`, "docker", ""))

	ca := ingress.NewCaddyAdapter(ingress.CaddyConfig{
		PortPoolStart: 9000,
		PortPoolEnd:   9004,
	})
	_, _ = ca.AllocPort(ctx, 8080)
	_, _ = ca.AllocPort(ctx, 8081)

	cfg := RouterConfig{
		Store:        s,
		Version:      "0.5.0",
		Ingress:      ca,
		Orchestrator: &mockOrchAdapter{name: "docker", healthy: true},
		Uptime:       time.Now(),
	}
	h := NewHandler(cfg)

	req := httptest.NewRequest("GET", "/api/v1/status", nil)
	rr := httptest.NewRecorder()
	h.Status(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)

	var resp StatusResponse
	err := json.NewDecoder(rr.Body).Decode(&resp)
	require.NoError(t, err)

	assert.Equal(t, 9000, resp.Ports.PoolStart)
	assert.Equal(t, 9004, resp.Ports.PoolEnd)
	assert.Equal(t, 2, resp.Ports.Used)
	assert.Equal(t, 3, resp.Ports.Free)
}

func TestHandleStatusCapacity(t *testing.T) {
	s := tempStore(t)
	ctx := context.Background()

	require.NoError(t, s.CreateBody(ctx, "b1", "body-1", orchestrator.StateCreated, `{}`, "docker", ""))

	cfg := RouterConfig{
		Store:        s,
		Version:      "0.5.0",
		Ingress:      ingress.NewNoopAdapter(),
		Orchestrator: &mockOrchAdapter{name: "docker", healthy: true},
		Uptime:       time.Now(),
	}
	h := NewHandler(cfg)

	req := httptest.NewRequest("GET", "/api/v1/status", nil)
	rr := httptest.NewRecorder()
	h.Status(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)

	var resp StatusResponse
	err := json.NewDecoder(rr.Body).Decode(&resp)
	require.NoError(t, err)

	// Capacity section: values >= 0
	cap := resp.Capacity
	assert.GreaterOrEqual(t, cap.CPUPercent, float64(0), "cpu_percent should be >= 0")
	assert.GreaterOrEqual(t, cap.MemoryMBUsed, int64(0), "memory_mb_used should be >= 0")
	assert.GreaterOrEqual(t, cap.MemoryMBTotal, int64(0), "memory_mb_total should be >= 0")
	assert.GreaterOrEqual(t, cap.DiskGBUsed, float64(0), "disk_gb_used should be >= 0")
	assert.GreaterOrEqual(t, cap.DiskGBTotal, float64(0), "disk_gb_total should be >= 0")
}
