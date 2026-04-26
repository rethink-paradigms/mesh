//go:build integration

package integration

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rethink-paradigms/mesh/internal/adapter"
	"github.com/rethink-paradigms/mesh/internal/body"
	"github.com/rethink-paradigms/mesh/internal/config"
	"github.com/rethink-paradigms/mesh/internal/daemon"
	"github.com/rethink-paradigms/mesh/internal/manifest"
	"github.com/rethink-paradigms/mesh/internal/mcp"
	"github.com/rethink-paradigms/mesh/internal/store"
	"gopkg.in/yaml.v3"
)

func tempStore(t *testing.T) *store.Store {
	t.Helper()
	f, err := os.CreateTemp("", "integration-test-*.db")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	path := f.Name()
	f.Close()
	s, err := store.Open(path)
	if err != nil {
		os.Remove(path)
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() {
		s.Close()
		os.Remove(path)
	})
	return s
}

func writeTestConfig(t *testing.T, tmpDir string) *config.Config {
	t.Helper()
	cfg := &config.Config{
		Daemon: config.DaemonConfig{
			SocketPath: filepath.Join(tmpDir, "mesh.sock"),
			PIDFile:    filepath.Join(tmpDir, "mesh.pid"),
			LogLevel:   "debug",
		},
		Store: config.StoreConfig{
			Path: filepath.Join(tmpDir, "state.db"),
		},
		Docker: config.DockerConfig{
			Host:       "unix:///var/run/docker.sock",
			APIVersion: "1.48",
		},
	}
	cfgData, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	cfgPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(cfgPath, cfgData, 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return cfg
}

type harness struct {
	srv     *mcp.Server
	stdinW  *os.File
	stdoutR *os.File
	scanner *bufio.Scanner
	cancel  context.CancelFunc
	done    chan error
}

func newHarness(t *testing.T, s *store.Store) *harness {
	t.Helper()
	inR, inW, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe in: %v", err)
	}
	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe out: %v", err)
	}
	srv := mcp.NewWithIO(s, inR, outW)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- srv.Start(ctx)
		inR.Close()
	}()
	scanner := bufio.NewScanner(outR)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	return &harness{srv: srv, stdinW: inW, stdoutR: outR, scanner: scanner, cancel: cancel, done: done}
}

func (h *harness) send(t *testing.T, v interface{}) {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	h.stdinW.Write(append(data, '\n'))
}

func (h *harness) close() {
	h.stdinW.Close()
}

func (h *harness) readResponse(t *testing.T) map[string]interface{} {
	t.Helper()
	respCh := make(chan map[string]interface{}, 1)
	go func() {
		for h.scanner.Scan() {
			line := strings.TrimSpace(h.scanner.Text())
			if line == "" {
				continue
			}
			var resp map[string]interface{}
			if err := json.Unmarshal([]byte(line), &resp); err != nil {
				continue
			}
			respCh <- resp
			return
		}
	}()
	select {
	case resp := <-respCh:
		return resp
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for MCP response")
		return nil
	}
}

func rawJSON(t *testing.T, v interface{}) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return data
}

func TestDaemonFullPipeline(t *testing.T) {
	tmpDir := t.TempDir()

	// Step 1: Write config and load it back
	cfg := writeTestConfig(t, tmpDir)
	cfgPath := filepath.Join(tmpDir, "config.yaml")

	loadedCfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("config load: %v", err)
	}
	if loadedCfg.Daemon.LogLevel != "debug" {
		t.Errorf("log level = %q, want debug", loadedCfg.Daemon.LogLevel)
	}

	// Step 2: Open store
	s, err := store.Open(cfg.Store.Path)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer s.Close()

	// Step 3: Create mock adapter + body manager
	mockAdapter := &mockSubstrateAdapter{}
	bm := body.NewBodyManager(s, mockAdapter)

	// Step 4: Create a body via BodyManager
	ctx := context.Background()
	spec := adapter.BodySpec{Image: "alpine:latest", Cmd: []string{"sleep", "3600"}}
	b, err := bm.Create(ctx, "test-body-1", spec)
	if err != nil {
		t.Fatalf("create body: %v", err)
	}
	if b.State != adapter.StateRunning {
		t.Fatalf("state = %s, want Running", b.State)
	}

	// Step 5: Verify body persisted in store
	record, err := s.GetBody(ctx, b.ID)
	if err != nil {
		t.Fatalf("get body from store: %v", err)
	}
	if record.Name != "test-body-1" {
		t.Errorf("name = %q, want test-body-1", record.Name)
	}
	if record.State != adapter.StateRunning {
		t.Errorf("store state = %s, want Running", record.State)
	}

	// Step 6: Start MCP server with pipe IO
	h := newHarness(t, s)
	h.srv.SetBodyManager(bm)
	migrator := body.NewMigrationCoordinator(s, mockAdapter, bm)
	h.srv.SetMigrator(migrator)

	// Step 7: List bodies via MCP
	h.send(t, mcp.Request{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params:  rawJSON(t, map[string]interface{}{"name": "list_bodies", "arguments": map[string]interface{}{}}),
	})
	resp := h.readResponse(t)
	if resp["error"] != nil {
		t.Fatalf("list_bodies error: %v", resp["error"])
	}
	result := resp["result"].(map[string]interface{})
	content := result["content"].([]interface{})
	text := content[0].(map[string]interface{})["text"].(string)
	var bodies []interface{}
	if err := json.Unmarshal([]byte(text), &bodies); err != nil {
		t.Fatalf("unmarshal bodies: %v", err)
	}
	if len(bodies) != 1 {
		t.Fatalf("len(bodies) = %d, want 1", len(bodies))
	}

	// Step 8: Create another body via MCP
	h.send(t, mcp.Request{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/call",
		Params: rawJSON(t, map[string]interface{}{
			"name":      "create_body",
			"arguments": map[string]interface{}{"name": "body-via-mcp", "image": "alpine:3.19"},
		}),
	})
	resp = h.readResponse(t)
	if resp["error"] != nil {
		t.Fatalf("create_body error: %v", resp["error"])
	}
	result = resp["result"].(map[string]interface{})
	content = result["content"].([]interface{})
	text = content[0].(map[string]interface{})["text"].(string)
	var createResp map[string]interface{}
	if err := json.Unmarshal([]byte(text), &createResp); err != nil {
		t.Fatalf("unmarshal create response: %v", err)
	}
	if createResp["state"] != "Running" {
		t.Fatalf("created body state = %v, want Running", createResp["state"])
	}

	// Step 9: List again — should have 2 bodies
	h.send(t, mcp.Request{
		JSONRPC: "2.0",
		ID:      3,
		Method:  "tools/call",
		Params:  rawJSON(t, map[string]interface{}{"name": "list_bodies", "arguments": map[string]interface{}{}}),
	})
	resp = h.readResponse(t)
	result = resp["result"].(map[string]interface{})
	content = result["content"].([]interface{})
	text = content[0].(map[string]interface{})["text"].(string)
	if err := json.Unmarshal([]byte(text), &bodies); err != nil {
		t.Fatalf("unmarshal bodies: %v", err)
	}
	if len(bodies) != 2 {
		t.Fatalf("len(bodies) = %d, want 2", len(bodies))
	}

	// Step 10: Graceful MCP shutdown
	h.cancel()
	h.close()
	select {
	case err := <-h.done:
		if err != nil && err != context.Canceled {
			t.Errorf("MCP shutdown error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("MCP server did not shut down")
	}

	// Step 11: Daemon stop is idempotent
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer stopCancel()
	if err := h.srv.Stop(stopCtx); err != nil {
		t.Errorf("second Stop: %v", err)
	}

	t.Log("Full pipeline passed: config → store → body create → MCP list → MCP create → MCP list → shutdown")
}

func TestConfigLoadRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		Daemon: config.DaemonConfig{
			SocketPath: "/tmp/test.sock",
			PIDFile:    filepath.Join(tmpDir, "test.pid"),
			LogLevel:   "debug",
		},
		Store: config.StoreConfig{
			Path: filepath.Join(tmpDir, "state.db"),
		},
		Docker: config.DockerConfig{
			Host:       "unix:///var/run/docker.sock",
			APIVersion: "1.45",
		},
		Bodies: []config.BodyConfig{
			{Name: "agent-1", Image: "alpine:latest", Cmd: []string{"sleep", "inf"}, MemoryMB: 512},
		},
	}

	cfgData, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	cfgPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(cfgPath, cfgData, 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	loaded, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if loaded.Daemon.LogLevel != "debug" {
		t.Errorf("log level = %q, want debug", loaded.Daemon.LogLevel)
	}
	if loaded.Docker.APIVersion != "1.45" {
		t.Errorf("api version = %q, want 1.45", loaded.Docker.APIVersion)
	}
	if len(loaded.Bodies) != 1 {
		t.Fatalf("bodies = %d, want 1", len(loaded.Bodies))
	}
	if loaded.Bodies[0].Name != "agent-1" {
		t.Errorf("body name = %q, want agent-1", loaded.Bodies[0].Name)
	}
	if loaded.Bodies[0].MemoryMB != 512 {
		t.Errorf("body memory = %d, want 512", loaded.Bodies[0].MemoryMB)
	}
}

func TestStoreReopen(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	ctx := context.Background()

	s1, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open s1: %v", err)
	}

	if err := s1.CreateBody(ctx, "body-1", "my-body", adapter.StateRunning, `{"image":"alpine"}`, "local", "inst-1"); err != nil {
		t.Fatalf("create body: %v", err)
	}

	s1.Close()

	s2, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open s2: %v", err)
	}
	defer s2.Close()

	rec, err := s2.GetBody(ctx, "body-1")
	if err != nil {
		t.Fatalf("get body after reopen: %v", err)
	}
	if rec.Name != "my-body" {
		t.Errorf("name = %q, want my-body", rec.Name)
	}
	if rec.State != adapter.StateRunning {
		t.Errorf("state = %s, want Running", rec.State)
	}
}

func TestMigrationStatePersistence(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	ctx := context.Background()

	s1, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open s1: %v", err)
	}

	if err := s1.CreateBody(ctx, "b1", "mig-body", adapter.StateRunning, `{}`, "local", "inst-1"); err != nil {
		t.Fatalf("create body: %v", err)
	}
	if err := s1.CreateMigration(ctx, "mig-1", "b1", "fleet", "snap-1"); err != nil {
		t.Fatalf("create migration: %v", err)
	}
	if err := s1.UpdateMigration(ctx, "mig-1", 4, ""); err != nil {
		t.Fatalf("update migration: %v", err)
	}

	s1.Close()

	s2, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open s2: %v", err)
	}
	defer s2.Close()

	mig, err := s2.GetMigration(ctx, "mig-1")
	if err != nil {
		t.Fatalf("get migration after reopen: %v", err)
	}
	if mig.BodyID != "b1" {
		t.Errorf("body_id = %q, want b1", mig.BodyID)
	}
	if mig.TargetSubstrate != "fleet" {
		t.Errorf("target = %q, want fleet", mig.TargetSubstrate)
	}
	if mig.CurrentStep != 4 {
		t.Errorf("step = %d, want 4", mig.CurrentStep)
	}
	if mig.SnapshotID != "snap-1" {
		t.Errorf("snapshot_id = %q, want snap-1", mig.SnapshotID)
	}

	bodyRec, err := s2.GetBody(ctx, "b1")
	if err != nil {
		t.Fatalf("get body after reopen: %v", err)
	}
	if bodyRec.State != adapter.StateRunning {
		t.Errorf("body state = %s, want Running", bodyRec.State)
	}
}

func TestManifestV2RoundTrip(t *testing.T) {
	tmpDir := t.TempDir()

	ts := time.Date(2026, 4, 27, 12, 30, 0, 0, time.UTC)
	m := manifest.NewV2()
	m.AgentName = "test-agent"
	m.Timestamp = ts
	m.SourceMachine = "workstation"
	m.SourceWorkdir = "/home/user/agents/test"
	m.Checksum = "abc123def456"
	m.Size = 45200
	m.Image = "alpine:latest"
	m.Platform = "linux/amd64"
	m.Adapter = "docker"
	m.Env = map[string]string{"FOO": "bar", "BAZ": "qux"}
	m.Cmd = []string{"./start.sh", "--verbose"}
	m.BodyID = "body-42"

	manifestPath := filepath.Join(tmpDir, "snapshots", "test-agent.json")
	if err := manifest.Write(manifestPath, m); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	loaded, err := manifest.Read(manifestPath)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}

	if manifest.ManifestVersion(loaded) != 2 {
		t.Errorf("version = %d, want 2", manifest.ManifestVersion(loaded))
	}
	if loaded.AgentName != "test-agent" {
		t.Errorf("agent_name = %q, want test-agent", loaded.AgentName)
	}
	if loaded.Image != "alpine:latest" {
		t.Errorf("image = %q, want alpine:latest", loaded.Image)
	}
	if loaded.Platform != "linux/amd64" {
		t.Errorf("platform = %q, want linux/amd64", loaded.Platform)
	}
	if loaded.Adapter != "docker" {
		t.Errorf("adapter = %q, want docker", loaded.Adapter)
	}
	if loaded.Env["FOO"] != "bar" {
		t.Errorf("env FOO = %q, want bar", loaded.Env["FOO"])
	}
	if len(loaded.Cmd) != 2 || loaded.Cmd[0] != "./start.sh" {
		t.Errorf("cmd = %v, want [./start.sh --verbose]", loaded.Cmd)
	}
	if loaded.BodyID != "body-42" {
		t.Errorf("body_id = %q, want body-42", loaded.BodyID)
	}
	if loaded.Checksum != "abc123def456" {
		t.Errorf("checksum = %q, want abc123def456", loaded.Checksum)
	}
	if loaded.Size != 45200 {
		t.Errorf("size = %d, want 45200", loaded.Size)
	}
}

func TestDaemonWithMCPEndToEnd(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		Daemon: config.DaemonConfig{
			PIDFile: filepath.Join(tmpDir, "mesh.pid"),
		},
		Store: config.StoreConfig{
			Path: filepath.Join(tmpDir, "state.db"),
		},
		Docker: config.DockerConfig{
			Host: "unix:///var/run/docker.sock",
		},
	}

	d, err := daemon.New(cfg)
	if err != nil {
		t.Fatalf("new daemon: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	daemonDone := make(chan error, 1)
	go func() {
		daemonDone <- d.Start(ctx)
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

	if !d.Ready() {
		cancel()
		t.Fatal("daemon never became ready")
	}

	cancel()

	select {
	case err := <-daemonDone:
		if err != nil {
			t.Fatalf("daemon start error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("daemon did not stop")
	}

	select {
	case <-d.Done():
	default:
		t.Fatal("Done channel should be closed after stop")
	}
}

func TestBodyLifecycleViaManager(t *testing.T) {
	s := tempStore(t)
	mockAdapter := &mockSubstrateAdapter{}
	bm := body.NewBodyManager(s, mockAdapter)
	ctx := context.Background()

	b, err := bm.Create(ctx, "lifecycle-body", adapter.BodySpec{Image: "alpine:latest"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if b.State != adapter.StateRunning {
		t.Fatalf("state after create = %s, want Running", b.State)
	}

	status, err := bm.GetStatus(ctx, b.ID)
	if err != nil {
		t.Fatalf("get status: %v", err)
	}
	if status.State != adapter.StateRunning {
		t.Errorf("status state = %s, want Running", status.State)
	}

	if err := bm.Stop(ctx, b.ID, adapter.StopOpts{}); err != nil {
		t.Fatalf("stop: %v", err)
	}

	if err := bm.Destroy(ctx, b.ID); err != nil {
		t.Fatalf("destroy: %v", err)
	}

	if _, err := s.GetBody(ctx, b.ID); err == nil {
		t.Fatal("body should be deleted after destroy")
	}

	if len(mockAdapter.created) != 1 {
		t.Errorf("adapter creates = %d, want 1", len(mockAdapter.created))
	}
	if len(mockAdapter.stopped) != 1 {
		t.Errorf("adapter stops = %d, want 1", len(mockAdapter.stopped))
	}
	if len(mockAdapter.destroyed) != 1 {
		t.Errorf("adapter destroys = %d, want 1", len(mockAdapter.destroyed))
	}
}

func TestSnapshotCRUDIntegration(t *testing.T) {
	s := tempStore(t)
	ctx := context.Background()

	if err := s.CreateBody(ctx, "b1", "snap-body", adapter.StateRunning, `{}`, "local", "inst-1"); err != nil {
		t.Fatalf("create body: %v", err)
	}

	if err := s.CreateSnapshot(ctx, "snap-1", "b1", `{"checksum":"abc"}`, "/tmp/snap-1.tar.zst", 2048); err != nil {
		t.Fatalf("create snapshot: %v", err)
	}

	snap, err := s.GetSnapshot(ctx, "snap-1")
	if err != nil {
		t.Fatalf("get snapshot: %v", err)
	}
	if snap.BodyID != "b1" {
		t.Errorf("body_id = %q, want b1", snap.BodyID)
	}
	if snap.SizeBytes != 2048 {
		t.Errorf("size = %d, want 2048", snap.SizeBytes)
	}

	snaps, err := s.ListSnapshots(ctx, "b1")
	if err != nil {
		t.Fatalf("list snapshots: %v", err)
	}
	if len(snaps) != 1 {
		t.Fatalf("snapshots = %d, want 1", len(snaps))
	}

	if err := s.DeleteSnapshot(ctx, "snap-1"); err != nil {
		t.Fatalf("delete snapshot: %v", err)
	}
	if _, err := s.GetSnapshot(ctx, "snap-1"); err == nil {
		t.Fatal("snapshot should be deleted")
	}
}

func TestExportFilesystemIntegration(t *testing.T) {
	s := tempStore(t)
	mockAdapter := &mockSubstrateAdapter{}
	bm := body.NewBodyManager(s, mockAdapter)
	ctx := context.Background()

	b, err := bm.Create(ctx, "export-body", adapter.BodySpec{Image: "alpine:latest"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	rc, err := mockAdapter.ExportFilesystem(ctx, b.InstanceID)
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(data) == 0 {
		t.Error("expected non-empty export data")
	}

	caps := mockAdapter.Capabilities()
	if !caps.ExportFilesystem {
		t.Error("adapter should support ExportFilesystem")
	}
	if !caps.ImportFilesystem {
		t.Error("adapter should support ImportFilesystem")
	}
}
