package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/rethink-paradigms/mesh/internal/config"
	"github.com/rethink-paradigms/mesh/internal/store"
)

func testConfig(t *testing.T) *config.Config {
	t.Helper()
	return &config.Config{
		Daemon: config.DaemonConfig{
			PIDFile: filepath.Join(t.TempDir(), "mesh.pid"),
		},
		Store: config.StoreConfig{
			Path: filepath.Join(t.TempDir(), "test.db"),
		},
		Docker: config.DockerConfig{
			Host: "unix:///var/run/docker.sock",
		},
	}
}

func TestDaemonNew(t *testing.T) {
	cfg := testConfig(t)
	d, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if d.ready {
		t.Error("daemon should not be ready before Start")
	}
}

func TestDaemonStartStop(t *testing.T) {
	cfg := testConfig(t)
	d, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- d.Start(ctx)
	}()

	time.Sleep(200 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Start returned error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Start did not return within timeout")
	}
}

func TestPIDFile(t *testing.T) {
	cfg := testConfig(t)
	d, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- d.Start(ctx)
	}()

	var found bool
	for i := 0; i < 50; i++ {
		if _, err := os.Stat(cfg.Daemon.PIDFile); err == nil {
			found = true
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !found {
		t.Fatal("PID file was not created")
	}

	data, err := os.ReadFile(cfg.Daemon.PIDFile)
	if err != nil {
		t.Fatalf("read PID file: %v", err)
	}
	expected := fmt.Sprintf("%d", os.Getpid())
	if string(data) != expected {
		t.Fatalf("PID file content = %q, want %q", data, expected)
	}

	cancel()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Start did not return within timeout")
	}

	if _, err := os.Stat(cfg.Daemon.PIDFile); !os.IsNotExist(err) {
		t.Fatal("PID file should be removed after stop")
	}
}

func TestHealthEndpoint(t *testing.T) {
	cfg := testConfig(t)
	d, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- d.Start(ctx)
	}()

	var addr string
	for i := 0; i < 50; i++ {
		addr = d.HTTPAddr()
		if addr != "" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if addr == "" {
		cancel()
		t.Fatal("health server never started")
	}

	resp, err := http.Get("http://" + addr + "/healthz")
	if err != nil {
		cancel()
		t.Fatalf("GET /healthz: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		cancel()
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("status = %v, want ok", body["status"])
	}

	cancel()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Start did not return within timeout")
	}
}

func TestHealthEndpointNotReady(t *testing.T) {
	cfg := testConfig(t)
	d, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	d.handleHealth(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["status"] != "not ready" {
		t.Fatalf("status = %q, want %q", body["status"], "not ready")
	}
}

func TestReconcileEmpty(t *testing.T) {
	cfg := testConfig(t)
	d, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	s, err := store.Open(cfg.Store.Path)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	d.store = s
	defer s.Close()

	if err := d.reconcile(context.Background()); err != nil {
		t.Fatalf("reconcile on empty store: %v", err)
	}
}

func TestSignalHandling(t *testing.T) {
	cfg := testConfig(t)
	d, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	done := make(chan error, 1)
	go func() {
		done <- d.Start(context.Background())
	}()

	for i := 0; i < 50; i++ {
		if d.Ready() {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !d.Ready() {
		t.Fatal("daemon never became ready")
	}

	syscall.Kill(os.Getpid(), syscall.SIGTERM)

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Start after SIGTERM: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Start did not return after SIGTERM within timeout")
	}

	select {
	case <-d.Done():
	default:
		t.Fatal("Done channel should be closed after stop")
	}
}

func TestSetMCP(t *testing.T) {
	cfg := testConfig(t)
	d, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	stopped := false
	mockMCP := &mockMCPServer{onStop: func() { stopped = true }}
	d.SetMCP(mockMCP)

	d.Stop(context.Background())

	if !stopped {
		t.Fatal("MCP server Stop should have been called")
	}
}

func TestHealthEndpointVerbose(t *testing.T) {
	cfg := testConfig(t)
	d, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	s, err := store.Open(cfg.Store.Path)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	d.store = s
	d.ready = true
	defer s.Close()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz?verbose=true", nil)
	d.handleHealth(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, ok := body["config"]; !ok {
		t.Fatal("verbose response should include config")
	}
}

func TestHealthBodiesCount(t *testing.T) {
	cfg := testConfig(t)
	d, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	s, err := store.Open(cfg.Store.Path)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	d.store = s
	d.ready = true
	defer s.Close()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	d.handleHealth(rec, req)

	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if v, ok := body["bodies"]; !ok || v != float64(0) {
		t.Fatalf("bodies = %v, want 0", v)
	}
}

func TestPIDFileEmpty(t *testing.T) {
	cfg := testConfig(t)
	cfg.Daemon.PIDFile = ""
	d, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := d.writePIDFile(); err != nil {
		t.Fatalf("writePIDFile with empty path: %v", err)
	}
	d.removePIDFile()
}

func TestStopIdempotent(t *testing.T) {
	cfg := testConfig(t)
	d, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if err := d.Stop(context.Background()); err != nil {
		t.Fatalf("first Stop: %v", err)
	}
	if err := d.Stop(context.Background()); err != nil {
		t.Fatalf("second Stop: %v", err)
	}
}

type mockMCPServer struct {
	onStop func()
}

func (m *mockMCPServer) Stop(_ context.Context) error {
	if m.onStop != nil {
		m.onStop()
	}
	return nil
}
