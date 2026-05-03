package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/rethink-paradigms/mesh/internal/adapter"
	"github.com/rethink-paradigms/mesh/internal/body"
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
	}
}

// mockDaemonAdapter is a minimal SubstrateAdapter for daemon tests.
// It returns zero values / not-found errors for all operations.
type mockDaemonAdapter struct{}

func (m *mockDaemonAdapter) Create(_ context.Context, _ adapter.BodySpec) (adapter.Handle, error) {
	return "mock-handle", nil
}

func (m *mockDaemonAdapter) Start(_ context.Context, _ adapter.Handle) error {
	return nil
}

func (m *mockDaemonAdapter) Stop(_ context.Context, _ adapter.Handle, _ adapter.StopOpts) error {
	return nil
}

func (m *mockDaemonAdapter) Destroy(_ context.Context, _ adapter.Handle) error {
	return nil
}

func (m *mockDaemonAdapter) GetStatus(_ context.Context, _ adapter.Handle) (adapter.BodyStatus, error) {
	return adapter.BodyStatus{}, fmt.Errorf("not found")
}

func (m *mockDaemonAdapter) Exec(_ context.Context, _ adapter.Handle, _ []string) (adapter.ExecResult, error) {
	return adapter.ExecResult{}, nil
}

func (m *mockDaemonAdapter) ExportFilesystem(_ context.Context, _ adapter.Handle) (io.ReadCloser, error) {
	return nil, fmt.Errorf("not supported")
}

func (m *mockDaemonAdapter) ImportFilesystem(_ context.Context, _ adapter.Handle, _ io.Reader, _ adapter.ImportOpts) error {
	return fmt.Errorf("not supported")
}

func (m *mockDaemonAdapter) Inspect(_ context.Context, _ adapter.Handle) (adapter.ContainerMetadata, error) {
	return adapter.ContainerMetadata{}, fmt.Errorf("not supported")
}

func (m *mockDaemonAdapter) Capabilities() adapter.AdapterCapabilities {
	return adapter.AdapterCapabilities{}
}

func (m *mockDaemonAdapter) SubstrateName() string {
	return "docker"
}

func (m *mockDaemonAdapter) IsHealthy(_ context.Context) bool {
	return true
}

func newMockDaemonAdapter() *mockDaemonAdapter {
	return &mockDaemonAdapter{}
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

func TestReconcileMissingContainer(t *testing.T) {
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

	ctx := context.Background()
	if err := s.CreateBody(ctx, "b1", "body-1", adapter.StateRunning, "", "docker", "nonexistent-container-id"); err != nil {
		t.Fatalf("CreateBody: %v", err)
	}

	mockAdp := newMockDaemonAdapter()
	multi := adapter.NewMultiAdapter()
	multi.Register("docker", mockAdp)
	d.adapters = multi
	d.bodyMgr = body.NewBodyManager(d.store, &multiAdapterOrchestrator{multi: d.adapters})

	if err := d.reconcile(ctx); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	rec, err := s.GetBody(ctx, "b1")
	if err != nil {
		t.Fatalf("GetBody: %v", err)
	}
	if rec.State != adapter.StateError {
		t.Fatalf("state = %q, want Error", rec.State)
	}
	if d.reconcileSteps != 1 {
		t.Fatalf("reconcileSteps = %d, want 1", d.reconcileSteps)
	}
}

func TestReconcileOrphanedStoreRecord(t *testing.T) {
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

	ctx := context.Background()
	if err := s.CreateBody(ctx, "b2", "body-2", adapter.StateError, "", "docker", "nonexistent-container-id"); err != nil {
		t.Fatalf("CreateBody: %v", err)
	}

	mockAdp := newMockDaemonAdapter()
	multi := adapter.NewMultiAdapter()
	multi.Register("docker", mockAdp)
	d.adapters = multi
	d.bodyMgr = body.NewBodyManager(d.store, &multiAdapterOrchestrator{multi: d.adapters})

	if err := d.reconcile(ctx); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	rec, err := s.GetBody(ctx, "b2")
	if err != nil {
		t.Fatalf("GetBody: %v", err)
	}
	if rec.State != adapter.StateError {
		t.Fatalf("state = %q, want Error (unchanged)", rec.State)
	}
	if d.reconcileSteps != 0 {
		t.Fatalf("reconcileSteps = %d, want 0", d.reconcileSteps)
	}
}

func TestReconcileStateMismatch(t *testing.T) {
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

	ctx := context.Background()
	if err := s.CreateBody(ctx, "b3", "body-3", adapter.StateRunning, "", "docker", ""); err != nil {
		t.Fatalf("CreateBody: %v", err)
	}

	mockAdp := newMockDaemonAdapter()
	multi := adapter.NewMultiAdapter()
	multi.Register("docker", mockAdp)
	d.adapters = multi
	d.bodyMgr = body.NewBodyManager(d.store, &multiAdapterOrchestrator{multi: d.adapters})

	if err := d.reconcile(ctx); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	rec, err := s.GetBody(ctx, "b3")
	if err != nil {
		t.Fatalf("GetBody: %v", err)
	}
	if rec.State != adapter.StateRunning {
		t.Fatalf("state = %q, want Running (unchanged, no instance_id)", rec.State)
	}
	if d.reconcileSteps != 0 {
		t.Fatalf("reconcileSteps = %d, want 0", d.reconcileSteps)
	}
}

func TestReconcileMigrationRecovery(t *testing.T) {
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

	ctx := context.Background()
	if err := s.CreateBody(ctx, "b4", "body-4", adapter.StateMigrating, "", "docker", "inst-4"); err != nil {
		t.Fatalf("CreateBody: %v", err)
	}

	mockAdp := newMockDaemonAdapter()
	multi := adapter.NewMultiAdapter()
	multi.Register("docker", mockAdp)
	d.adapters = multi
	d.bodyMgr = body.NewBodyManager(d.store, &multiAdapterOrchestrator{multi: d.adapters})

	if err := d.reconcile(ctx); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	rec, err := s.GetBody(ctx, "b4")
	if err != nil {
		t.Fatalf("GetBody: %v", err)
	}
	if rec.State != adapter.StateError {
		t.Fatalf("state = %q, want Error (migration record missing)", rec.State)
	}
	if d.reconcileSteps != 1 {
		t.Fatalf("reconcileSteps = %d, want 1", d.reconcileSteps)
	}
}

func TestReconcileHealthSteps(t *testing.T) {
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

	ctx := context.Background()
	if err := s.CreateBody(ctx, "b5", "body-5", adapter.StateRunning, "", "docker", "nonexistent"); err != nil {
		t.Fatalf("CreateBody: %v", err)
	}

	mockAdp := newMockDaemonAdapter()
	multi := adapter.NewMultiAdapter()
	multi.Register("docker", mockAdp)
	d.adapters = multi
	d.bodyMgr = body.NewBodyManager(d.store, &multiAdapterOrchestrator{multi: d.adapters})

	if err := d.reconcile(ctx); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	d.handleHealth(rec, req)

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if v, ok := resp["reconcile_steps"]; !ok || v != float64(1) {
		t.Fatalf("reconcile_steps = %v, want 1", v)
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

func TestDaemonWiresMultiAdapter(t *testing.T) {
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

	for i := 0; i < 50; i++ {
		if d.Ready() {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !d.Ready() {
		cancel()
		t.Fatal("daemon never became ready")
	}

	if d.adapters == nil {
		t.Fatal("adapters should be initialized")
	}

	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Start did not return within timeout")
	}
}

func TestDaemonInitializesBodyManager(t *testing.T) {
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

	for i := 0; i < 50; i++ {
		if d.Ready() {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !d.Ready() {
		cancel()
		t.Fatal("daemon never became ready")
	}

	if d.bodyMgr == nil {
		t.Fatal("bodyMgr should be initialized")
	}

	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Start did not return within timeout")
	}
}

func TestDaemonRefusesDuplicateStart(t *testing.T) {
	cfg := testConfig(t)

	// Spawn a real child process and write its PID to the PID file.
	cmd := exec.Command("sleep", "30")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start child process: %v", err)
	}
	defer cmd.Process.Kill()

	if err := os.WriteFile(cfg.Daemon.PIDFile, []byte(fmt.Sprintf("%d", cmd.Process.Pid)), 0644); err != nil {
		t.Fatalf("write PID file: %v", err)
	}

	d, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	err = d.Start(context.Background())
	if err == nil {
		t.Fatal("daemon should have been refused")
	}
	if !strings.Contains(err.Error(), "already running") {
		t.Fatalf("error = %q, want 'already running'", err.Error())
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

func TestDaemonInitializesPluginManager(t *testing.T) {
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

	for i := 0; i < 50; i++ {
		if d.Ready() {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !d.Ready() {
		cancel()
		t.Fatal("daemon never became ready")
	}

	if d.pluginMgr == nil {
		t.Fatal("pluginMgr should be initialized")
	}

	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Start did not return within timeout")
	}
}

func TestDaemonStopsPluginManagerOnShutdown(t *testing.T) {
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

	for i := 0; i < 50; i++ {
		if d.Ready() {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !d.Ready() {
		cancel()
		t.Fatal("daemon never became ready")
	}

	if d.pluginMgr == nil {
		t.Fatal("pluginMgr should be initialized")
	}

	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Start did not return within timeout")
	}

	select {
	case <-d.Done():
	default:
		t.Fatal("Done channel should be closed after stop")
	}
}
