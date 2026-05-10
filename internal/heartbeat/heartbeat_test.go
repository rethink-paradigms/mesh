package heartbeat

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_Start_postsHeartbeat(t *testing.T) {
	var received []heartbeatPayload
	mu := make(chan struct{}, 1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/internal/heartbeat", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)

		var p heartbeatPayload
		require.NoError(t, json.Unmarshal(body, &p))

		received = append(received, p)
		select {
		case mu <- struct{}{}:
		default:
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := &Client{
		httpClient:   &http.Client{Timeout: 10 * time.Second},
		gatewayURL:   server.URL,
		authToken:    "test-token",
		authMode:     "token",
		clusterID:    "cluster-123",
		version:      "1.0.0",
		tier:         "standard",
		orchestrator: "docker",
		bodiesCount:  func() int { return 3 },
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client.Start(ctx, 50*time.Millisecond)

	// Wait for at least 2 heartbeats
	for i := 0; i < 2; i++ {
		select {
		case <-mu:
		case <-time.After(2 * time.Second):
			t.Fatalf("heartbeat %d not received in time", i+1)
		}
	}

	cancel()

	require.GreaterOrEqual(t, len(received), 2, "expected at least 2 heartbeats")
	for _, p := range received {
		assert.Equal(t, "cluster-123", p.ClusterID)
		assert.Equal(t, "healthy", p.Status)
		assert.Equal(t, 3, p.BodiesCount)
		assert.Equal(t, "1.0.0", p.Version)
		assert.Equal(t, "standard", p.Tier)
		assert.Equal(t, "docker", p.Orchestrator)
	}
}

func TestClient_Start_emptyGatewayURL(t *testing.T) {
	client := &Client{
		gatewayURL: "",
		authToken:  "test-token",
		authMode:   "token",
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client.Start(ctx, 50*time.Millisecond)

	time.Sleep(100 * time.Millisecond)
}

func TestClient_Start_pureJWTNoToken(t *testing.T) {
	client := &Client{
		gatewayURL:   "http://example.com",
		authToken:    "",
		authMode:     "jwt",
		clusterID:    "cluster-123",
		version:      "1.0.0",
		tier:         "standard",
		orchestrator: "docker",
		bodiesCount:  func() int { return 0 },
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client.Start(ctx, 50*time.Millisecond)

	time.Sleep(100 * time.Millisecond)
}

func TestClient_sendHeartbeat_payloadShape(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)

		var raw map[string]interface{}
		require.NoError(t, json.Unmarshal(body, &raw))

		assert.Equal(t, "cluster-abc", raw["cluster_id"])
		assert.Equal(t, "healthy", raw["status"])
		assert.Equal(t, float64(5), raw["bodies_count"])
		assert.Equal(t, "2.1.0", raw["version"])
		assert.Equal(t, "premium", raw["tier"])
		assert.Equal(t, "nomad", raw["orchestrator"])

		assert.Len(t, raw, 6, "payload should contain exactly 6 fields")

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := &Client{
		httpClient:   &http.Client{Timeout: 10 * time.Second},
		gatewayURL:   server.URL,
		authToken:    "my-token",
		authMode:     "both",
		clusterID:    "cluster-abc",
		version:      "2.1.0",
		tier:         "premium",
		orchestrator: "nomad",
		bodiesCount:  func() int { return 5 },
	}

	ctx := context.Background()
	err := client.sendHeartbeat(ctx)
	require.NoError(t, err)
}

func TestClient_sendHeartbeat_httpError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"db down"}`))
	}))
	defer server.Close()

	client := &Client{
		httpClient:   &http.Client{Timeout: 10 * time.Second},
		gatewayURL:   server.URL,
		authToken:    "token",
		authMode:     "token",
		clusterID:    "c1",
		version:      "1.0.0",
		tier:         "standard",
		orchestrator: "docker",
		bodiesCount:  func() int { return 1 },
	}

	ctx := context.Background()
	err := client.sendHeartbeat(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func TestClient_sendHeartbeat_timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := &Client{
		httpClient:   &http.Client{Timeout: 50 * time.Millisecond},
		gatewayURL:   server.URL,
		authToken:    "token",
		authMode:     "token",
		clusterID:    "c1",
		version:      "1.0.0",
		tier:         "standard",
		orchestrator: "docker",
		bodiesCount:  func() int { return 1 },
	}

	ctx := context.Background()
	err := client.sendHeartbeat(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Client.Timeout exceeded")
}

func TestClient_Start_exitsOnContextCancel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := &Client{
		httpClient:   &http.Client{Timeout: 10 * time.Second},
		gatewayURL:   server.URL,
		authToken:    "token",
		authMode:     "token",
		clusterID:    "c1",
		version:      "1.0.0",
		tier:         "standard",
		orchestrator: "docker",
		bodiesCount:  func() int { return 1 },
	}

	ctx, cancel := context.WithCancel(context.Background())
	client.Start(ctx, 100*time.Millisecond)

	time.Sleep(150 * time.Millisecond)

	cancel()
}

func TestClient_sendHeartbeat_bodiesCountCallback(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var p heartbeatPayload
		_ = json.Unmarshal(body, &p)
		assert.Equal(t, callCount, p.BodiesCount)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := &Client{
		httpClient:   &http.Client{Timeout: 10 * time.Second},
		gatewayURL:   server.URL,
		authToken:    "token",
		authMode:     "token",
		clusterID:    "c1",
		version:      "1.0.0",
		tier:         "standard",
		orchestrator: "docker",
		bodiesCount: func() int {
			callCount++
			return callCount
		},
	}

	ctx := context.Background()
	err := client.sendHeartbeat(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, callCount)

	err = client.sendHeartbeat(ctx)
	require.NoError(t, err)
	assert.Equal(t, 2, callCount)
}



func TestClient_sendHeartbeat_requestHeaders(t *testing.T) {
	var capturedHeaders http.Header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedHeaders = r.Header
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := &Client{
		httpClient:   &http.Client{Timeout: 10 * time.Second},
		gatewayURL:   server.URL,
		authToken:    "secret-token",
		authMode:     "token",
		clusterID:    "c1",
		version:      "1.0.0",
		tier:         "standard",
		orchestrator: "docker",
		bodiesCount:  func() int { return 1 },
	}

	ctx := context.Background()
	err := client.sendHeartbeat(ctx)
	require.NoError(t, err)

	require.NotNil(t, capturedHeaders)
	assert.Equal(t, "Bearer secret-token", capturedHeaders.Get("Authorization"))
	assert.Equal(t, "application/json", capturedHeaders.Get("Content-Type"))
}

func TestClient_sendHeartbeat_serverURL(t *testing.T) {
	var capturedURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedURL = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := &Client{
		httpClient:   &http.Client{Timeout: 10 * time.Second},
		gatewayURL:   server.URL,
		authToken:    "token",
		authMode:     "token",
		clusterID:    "c1",
		version:      "1.0.0",
		tier:         "standard",
		orchestrator: "docker",
		bodiesCount:  func() int { return 1 },
	}

	ctx := context.Background()
	err := client.sendHeartbeat(ctx)
	require.NoError(t, err)
	assert.Equal(t, "/api/internal/heartbeat", capturedURL)
}

func TestClient_Start_heartbeatInterval(t *testing.T) {
	timestamps := make(chan time.Time, 10)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		timestamps <- time.Now()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := &Client{
		httpClient:   &http.Client{Timeout: 10 * time.Second},
		gatewayURL:   server.URL,
		authToken:    "token",
		authMode:     "token",
		clusterID:    "c1",
		version:      "1.0.0",
		tier:         "standard",
		orchestrator: "docker",
		bodiesCount:  func() int { return 1 },
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	interval := 100 * time.Millisecond
	client.Start(ctx, interval)

	var times []time.Time
	for i := 0; i < 3; i++ {
		select {
		case ts := <-timestamps:
			times = append(times, ts)
		case <-time.After(2 * time.Second):
			t.Fatalf("did not receive heartbeat %d in time", i+1)
		}
	}

	for i := 1; i < len(times); i++ {
		delta := times[i].Sub(times[i-1])
		assert.InDelta(t, interval.Milliseconds(), delta.Milliseconds(), 50,
			"interval between heartbeat %d and %d", i, i+1)
	}
}

func TestClient_Start_logsWarningOnFailure(t *testing.T) {
	var requestCount int
	var mu sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestCount++
		mu.Unlock()
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	client := &Client{
		httpClient:   &http.Client{Timeout: 10 * time.Second},
		gatewayURL:   server.URL,
		authToken:    "token",
		authMode:     "token",
		clusterID:    "c1",
		version:      "1.0.0",
		tier:         "standard",
		orchestrator: "docker",
		bodiesCount:  func() int { return 1 },
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client.Start(ctx, 50*time.Millisecond)

	time.Sleep(150 * time.Millisecond)

	mu.Lock()
	count := requestCount
	mu.Unlock()
	assert.GreaterOrEqual(t, count, 2, "expected multiple heartbeat attempts despite failures")
}

func TestClient_Start_logsWarningOnHTTPError(t *testing.T) {
	var requestCount int
	var mu sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestCount++
		count := requestCount
		mu.Unlock()
		if count <= 2 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := &Client{
		httpClient:   &http.Client{Timeout: 10 * time.Second},
		gatewayURL:   server.URL,
		authToken:    "token",
		authMode:     "token",
		clusterID:    "c1",
		version:      "1.0.0",
		tier:         "standard",
		orchestrator: "docker",
		bodiesCount:  func() int { return 1 },
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client.Start(ctx, 50*time.Millisecond)

	time.Sleep(250 * time.Millisecond)

	mu.Lock()
	count := requestCount
	mu.Unlock()
	assert.GreaterOrEqual(t, count, 3, "expected heartbeat to continue through failures")
}

func TestClient_sendHeartbeat_non200Status(t *testing.T) {
	statusCodes := []int{http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound}
	for _, code := range statusCodes {
		t.Run(fmt.Sprintf("status_%d", code), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(code)
			}))
			defer server.Close()

			client := &Client{
				httpClient:   &http.Client{Timeout: 10 * time.Second},
				gatewayURL:   server.URL,
				authToken:    "token",
				authMode:     "token",
				clusterID:    "c1",
				version:      "1.0.0",
				tier:         "standard",
				orchestrator: "docker",
				bodiesCount:  func() int { return 1 },
			}

			ctx := context.Background()
			err := client.sendHeartbeat(ctx)
			require.Error(t, err)
		})
	}
}
