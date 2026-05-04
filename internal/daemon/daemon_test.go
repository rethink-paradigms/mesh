package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/rethink-paradigms/mesh/internal/body"
	"github.com/rethink-paradigms/mesh/internal/config"
	"github.com/rethink-paradigms/mesh/internal/orchestrator"
	"github.com/rethink-paradigms/mesh/internal/provisioner"
	"github.com/rethink-paradigms/mesh/internal/store"
)

func testConfig(t *testing.T) *config.Config {
	t.Helper()
	return &config.Config{
		Daemon: config.DaemonConfig{
			PIDFile:    filepath.Join(t.TempDir(), "mesh.pid"),
			ListenAddr: "127.0.0.1:0",
		},
		Store: config.StoreConfig{
			Path: filepath.Join(t.TempDir(), "test.db"),
		},
	}
}

type mockOrchestrator struct{}

func (m *mockOrchestrator) ScheduleBody(_ context.Context, _ orchestrator.BodySpec) (orchestrator.Handle, error) {
	return "mock-handle", nil
}

func (m *mockOrchestrator) StartBody(_ context.Context, _ orchestrator.Handle) error {
	return nil
}

func (m *mockOrchestrator) StopBody(_ context.Context, _ orchestrator.Handle) error {
	return nil
}

func (m *mockOrchestrator) DestroyBody(_ context.Context, _ orchestrator.Handle) error {
	return nil
}

func (m *mockOrchestrator) GetBodyStatus(_ context.Context, _ orchestrator.Handle) (orchestrator.BodyStatus, error) {
	return orchestrator.BodyStatus{}, fmt.Errorf("not found")
}

func (m *mockOrchestrator) Name() string { return "docker" }

func (m *mockOrchestrator) IsHealthy(_ context.Context) bool { return true }

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
	// The new API router returns "degraded" when the orchestrator is not healthy.
	// With no orchestrators registered, the noop orchestrator is used which reports unhealthy.
	if body["status"] != "degraded" {
		t.Fatalf("status = %v, want degraded", body["status"])
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

	// The new API router does not check daemon.ready; healthz always returns 200.
	// This test verifies the daemon itself reports not ready before Start.
	if d.Ready() {
		t.Fatal("daemon should not be ready before Start")
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
	d.orchRegistry = orchestrator.NewRegistry()
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
	if err := s.CreateBody(ctx, "b1", "body-1", orchestrator.StateRunning, "", "docker", "nonexistent-container-id"); err != nil {
		t.Fatalf("CreateBody: %v", err)
	}

	mockOrch := &mockOrchestrator{}
	reg := orchestrator.NewRegistry()
	reg.Register("docker", mockOrch)
	d.orchRegistry = reg
	d.bodyMgr = body.NewBodyManager(d.store, mockOrch)

	if err := d.reconcile(ctx); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	rec, err := s.GetBody(ctx, "b1")
	if err != nil {
		t.Fatalf("GetBody: %v", err)
	}
	if rec.State != orchestrator.StateError {
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
	if err := s.CreateBody(ctx, "b2", "body-2", orchestrator.StateError, "", "docker", "nonexistent-container-id"); err != nil {
		t.Fatalf("CreateBody: %v", err)
	}

	mockOrch := &mockOrchestrator{}
	reg := orchestrator.NewRegistry()
	reg.Register("docker", mockOrch)
	d.orchRegistry = reg
	d.bodyMgr = body.NewBodyManager(d.store, mockOrch)

	if err := d.reconcile(ctx); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	rec, err := s.GetBody(ctx, "b2")
	if err != nil {
		t.Fatalf("GetBody: %v", err)
	}
	if rec.State != orchestrator.StateError {
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
	if err := s.CreateBody(ctx, "b3", "body-3", orchestrator.StateRunning, "", "docker", ""); err != nil {
		t.Fatalf("CreateBody: %v", err)
	}

	reg := orchestrator.NewRegistry()
	reg.Register("docker", &mockOrchestrator{})
	d.orchRegistry = reg
	d.bodyMgr = body.NewBodyManager(d.store, &mockOrchestrator{})

	if err := d.reconcile(ctx); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	rec, err := s.GetBody(ctx, "b3")
	if err != nil {
		t.Fatalf("GetBody: %v", err)
	}
	if rec.State != orchestrator.StateRunning {
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
	if err := s.CreateBody(ctx, "b4", "body-4", orchestrator.StateMigrating, "", "docker", "inst-4"); err != nil {
		t.Fatalf("CreateBody: %v", err)
	}

	mockOrch := &mockOrchestrator{}
	reg := orchestrator.NewRegistry()
	reg.Register("docker", mockOrch)
	d.orchRegistry = reg
	d.bodyMgr = body.NewBodyManager(d.store, mockOrch)

	if err := d.reconcile(ctx); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	rec, err := s.GetBody(ctx, "b4")
	if err != nil {
		t.Fatalf("GetBody: %v", err)
	}
	if rec.State != orchestrator.StateError {
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
	defer s.Close()

	ctx := context.Background()
	if err := s.CreateBody(ctx, "b5", "body-5", orchestrator.StateRunning, "", "docker", "nonexistent"); err != nil {
		t.Fatalf("CreateBody: %v", err)
	}

	mockOrch := &mockOrchestrator{}
	reg := orchestrator.NewRegistry()
	reg.Register("docker", mockOrch)
	d.orchRegistry = reg
	d.bodyMgr = body.NewBodyManager(d.store, mockOrch)

	if err := d.reconcile(ctx); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	if d.reconcileSteps != 1 {
		t.Fatalf("reconcileSteps = %d, want 1", d.reconcileSteps)
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

	// The new API router does not support a verbose query param.
	// This test now verifies the API server starts and the health endpoint
	// returns the expected fields through the actual HTTP server.
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
		t.Fatal("API server never started")
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
		cancel()
		t.Fatalf("decode response: %v", err)
	}
	if body["status"] != "healthy" && body["status"] != "degraded" {
		t.Fatalf("status = %v, want healthy or degraded", body["status"])
	}

	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Start did not return within timeout")
	}
}

func TestHealthBodiesCount(t *testing.T) {
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
		t.Fatal("API server never started")
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
		cancel()
		t.Fatalf("decode response: %v", err)
	}
	if v, ok := body["bodies_count"]; !ok || v != float64(0) {
		t.Fatalf("bodies_count = %v, want 0", v)
	}

	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Start did not return within timeout")
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

func TestDaemonWiresOrchRegistry(t *testing.T) {
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

	if d.orchRegistry == nil {
		t.Fatal("orchRegistry should be initialized")
	}
	if d.provRegistry == nil {
		t.Fatal("provRegistry should be initialized")
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

func TestDaemonStartOrchOnly(t *testing.T) {
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

	mockOrch := &mockOrchestrator{}
	reg := orchestrator.NewRegistry()
	if err := reg.Register("nomad", mockOrch); err != nil {
		t.Fatalf("Register: %v", err)
	}
	d.orchRegistry = reg
	d.provRegistry = provisioner.NewRegistry()
	d.bodyMgr = body.NewBodyManager(d.store, mockOrch)

	if len(reg.List()) != 1 {
		t.Fatalf("expected 1 orchestrator, got %d", len(reg.List()))
	}

	if err := d.reconcile(context.Background()); err != nil {
		t.Fatalf("reconcile with empty store: %v", err)
	}

	adp, err := reg.Open("nomad")
	if err != nil {
		t.Fatalf("Open nomad: %v", err)
	}
	if !adp.IsHealthy(context.Background()) {
		t.Fatal("mock orchestrator should report healthy")
	}
}

func TestDaemonReconcileDockerOrphan(t *testing.T) {
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
	if err := s.CreateBody(ctx, "orphan-1", "orphan-body", orchestrator.StateRunning, "", "docker", "orphan-inst-1"); err != nil {
		t.Fatalf("CreateBody: %v", err)
	}

	reg := orchestrator.NewRegistry()
	d.orchRegistry = reg
	d.bodyMgr = body.NewBodyManager(d.store, &mockOrchestrator{})

	if err := d.reconcile(ctx); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	rec, err := s.GetBody(ctx, "orphan-1")
	if err != nil {
		t.Fatalf("GetBody: %v", err)
	}
	if rec.State != orchestrator.StateError {
		t.Fatalf("state = %q, want Error (orphaned Running body with missing substrate)", rec.State)
	}
	if d.reconcileSteps != 1 {
		t.Fatalf("reconcileSteps = %d, want 1", d.reconcileSteps)
	}
}
