//go:build integration

package integration

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rethink-paradigms/mesh/internal/adapter"
	"github.com/rethink-paradigms/mesh/internal/body"
	"github.com/rethink-paradigms/mesh/internal/config"
	"github.com/rethink-paradigms/mesh/internal/daemon"
	"github.com/rethink-paradigms/mesh/internal/manifest"
	"github.com/rethink-paradigms/mesh/internal/mcp"
	"github.com/rethink-paradigms/mesh/internal/plugin"
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
	pluginDir := filepath.Join(tmpDir, "plugins")
	os.MkdirAll(pluginDir, 0755)
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
		Plugin: config.PluginConfig{
			Dir:     pluginDir,
			Enabled: []string{},
		},
		Registry: config.RegistryConfig{
			Type:   "s3",
			Bucket: "test-bucket",
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
	migrator := body.NewMigrationCoordinator(s, mockAdapter, bm, nil)
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
	pluginDir := filepath.Join(tmpDir, "plugins")
	os.MkdirAll(pluginDir, 0755)

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
		Plugin: config.PluginConfig{
			Dir:     pluginDir,
			Enabled: []string{},
		},
		Registry: config.RegistryConfig{
			Type:   "s3",
			Bucket: "test-bucket",
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

func TestCrossMachineMigrationViaRegistry(t *testing.T) {
	s := tempStore(t)
	mockAdapter := &mockSubstrateAdapter{substrate: "docker"}
	bm := body.NewBodyManager(s, mockAdapter)
	ctx := context.Background()

	b, err := bm.Create(ctx, "cross-mig-body", adapter.BodySpec{Image: "alpine:latest"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	reg := &mockRegistry{}
	mc := body.NewMigrationCoordinator(s, mockAdapter, bm, reg)
	migID, err := mc.BeginMigration(ctx, b.ID, "fleet")
	if err != nil {
		t.Fatalf("BeginMigration: %v", err)
	}

	if len(reg.pushed) != 1 {
		t.Errorf("pushed = %d, want 1", len(reg.pushed))
	}
	if len(reg.pulled) != 1 {
		t.Errorf("pulled = %d, want 1", len(reg.pulled))
	}

	_, err = s.GetMigration(ctx, migID)
	if err == nil {
		t.Fatal("migration record should be deleted after completion")
	}

	bodyRec, err := s.GetBody(ctx, b.ID)
	if err != nil {
		t.Fatalf("get body after migration: %v", err)
	}
	if bodyRec.State != adapter.StateRunning {
		t.Errorf("body state = %s, want Running", bodyRec.State)
	}

	if len(mockAdapter.importedTo) != 1 {
		t.Errorf("importedTo = %d, want 1", len(mockAdapter.importedTo))
	}
}

func TestSameMachineMigrationSkipsRegistry(t *testing.T) {
	s := tempStore(t)
	mockAdapter := &mockSubstrateAdapter{substrate: "docker"}
	bm := body.NewBodyManager(s, mockAdapter)
	ctx := context.Background()

	b, err := bm.Create(ctx, "same-mig-body", adapter.BodySpec{Image: "alpine:latest"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	reg := &mockRegistry{}
	mc := body.NewMigrationCoordinator(s, mockAdapter, bm, reg)
	_, err = mc.BeginMigration(ctx, b.ID, "docker")
	if err != nil {
		t.Fatalf("BeginMigration: %v", err)
	}

	if len(reg.pushed) != 0 {
		t.Errorf("pushed = %d, want 0 (same-machine should skip registry)", len(reg.pushed))
	}
	if len(reg.pulled) != 0 {
		t.Errorf("pulled = %d, want 0 (same-machine should skip registry)", len(reg.pulled))
	}
}

func TestPluginManagement(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("plugin build skipped on windows")
	}

	tmpDir := t.TempDir()
	pluginDir := filepath.Join(tmpDir, "plugins")
	os.MkdirAll(pluginDir, 0755)

	// Build reference plugin binary
	pluginBin := filepath.Join(pluginDir, "reference-plugin")
	buildCmd := exec.Command("go", "build", "-o", pluginBin, "./internal/plugin/reference/")
	buildCmd.Dir = "/Users/samanvayayagsen/project/rethink-paradigms/mesh-impl"
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Skipf("plugin build failed (skipping): %v\n%s", err, out)
	}

	// Verify binary is executable
	info, err := os.Stat(pluginBin)
	if err != nil {
		t.Fatalf("stat plugin binary: %v", err)
	}
	if info.Mode()&0o111 == 0 {
		t.Fatal("plugin binary is not executable")
	}

	// Create plugin manager with reference plugin enabled
	pm := plugin.NewPluginManager(pluginDir, []string{"reference-plugin"})

	// Scan should find the plugin
	found, err := pm.Scan()
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if len(found) != 1 {
		t.Fatalf("found %d plugins, want 1", len(found))
	}
	if _, ok := found["reference-plugin"]; !ok {
		t.Fatal("reference-plugin not found in scan")
	}

	// Load the plugin
	if err := pm.Load("reference-plugin", pluginBin); err != nil {
		t.Fatalf("load plugin: %v", err)
	}

	// Verify plugin state
	rec := pm.Get("reference-plugin")
	if rec == nil {
		t.Fatal("plugin record not found after load")
	}
	if rec.Meta.Name != "reference" {
		t.Errorf("plugin name = %q, want reference", rec.Meta.Name)
	}
	if rec.Meta.Version != "1.0.0" {
		t.Errorf("plugin version = %q, want 1.0.0", rec.Meta.Version)
	}
	if rec.GetState() != plugin.StateHealthy {
		t.Errorf("plugin state = %s, want Healthy", rec.GetState())
	}

	// Test list_plugins via MCP
	s := tempStore(t)
	h := newHarness(t, s)
	h.srv.SetPluginManager(pm)

	h.send(t, mcp.Request{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params:  rawJSON(t, map[string]interface{}{"name": "list_plugins", "arguments": map[string]interface{}{}}),
	})
	resp := h.readResponse(t)
	if resp["error"] != nil {
		t.Fatalf("list_plugins error: %v", resp["error"])
	}
	result := resp["result"].(map[string]interface{})
	content := result["content"].([]interface{})
	text := content[0].(map[string]interface{})["text"].(string)
	var plugins []map[string]interface{}
	if err := json.Unmarshal([]byte(text), &plugins); err != nil {
		t.Fatalf("unmarshal plugins: %v", err)
	}
	if len(plugins) != 1 {
		t.Fatalf("len(plugins) = %d, want 1", len(plugins))
	}
	if plugins[0]["name"] != "reference" {
		t.Errorf("plugin name = %v, want reference", plugins[0]["name"])
	}
	if plugins[0]["healthy"] != true {
		t.Errorf("plugin healthy = %v, want true", plugins[0]["healthy"])
	}

	// Test plugin_health via MCP
	h.send(t, mcp.Request{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/call",
		Params: rawJSON(t, map[string]interface{}{
			"name":      "plugin_health",
			"arguments": map[string]interface{}{"plugin_name": "reference-plugin"},
		}),
	})
	resp = h.readResponse(t)
	if resp["error"] != nil {
		t.Fatalf("plugin_health error: %v", resp["error"])
	}
	result = resp["result"].(map[string]interface{})
	content = result["content"].([]interface{})
	text = content[0].(map[string]interface{})["text"].(string)
	var healthResp map[string]interface{}
	if err := json.Unmarshal([]byte(text), &healthResp); err != nil {
		t.Fatalf("unmarshal health: %v", err)
	}
	if healthResp["name"] != "reference" {
		t.Errorf("health name = %v, want reference", healthResp["name"])
	}
	if healthResp["healthy"] != true {
		t.Errorf("health healthy = %v, want true", healthResp["healthy"])
	}

	// Cleanup
	h.cancel()
	h.close()
	select {
	case <-h.done:
	case <-time.After(5 * time.Second):
		t.Fatal("MCP server did not shut down")
	}

	// Stop plugin manager
	if err := pm.Stop(); err != nil {
		t.Errorf("stop plugin manager: %v", err)
	}
}

func TestDaemonCrashRecovery(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()

	// Pre-seed store with a body that has a fake instance ID
	dbPath := filepath.Join(tmpDir, "state.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	// Create a body in Running state with a fake instance ID
	if err := s.CreateBody(ctx, "crash-body-1", "crash-test-body", adapter.StateRunning, `{"image":"alpine:latest"}`, "docker", "fake-instance-123"); err != nil {
		t.Fatalf("create body: %v", err)
	}
	s.Close()

	// Create config that points to the pre-seeded store
	cfg := &config.Config{
		Daemon: config.DaemonConfig{
			PIDFile: filepath.Join(tmpDir, "mesh.pid"),
		},
		Store: config.StoreConfig{
			Path: dbPath,
		},
		Plugin: config.PluginConfig{
			Dir:     filepath.Join(tmpDir, "plugins"),
			Enabled: []string{},
		},
	}

	// Start daemon — reconcile should detect orphaned container and transition to Error
	d, err := daemon.New(cfg)
	if err != nil {
		t.Fatalf("new daemon: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	daemonDone := make(chan error, 1)
	go func() {
		daemonDone <- d.Start(ctx)
	}()

	// Wait for daemon to become ready
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

	// Give reconcile time to run
	time.Sleep(100 * time.Millisecond)

	// Verify via health endpoint that reconcile ran
	addr := d.HTTPAddr()
	if addr == "" {
		cancel()
		t.Fatal("health server address not available")
	}

	resp, err := http.Get("http://" + addr + "/healthz")
	if err != nil {
		cancel()
		t.Fatalf("health check: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		cancel()
		t.Fatalf("health status = %d, want 200", resp.StatusCode)
	}

	var healthResp map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&healthResp); err != nil {
		cancel()
		t.Fatalf("decode health: %v", err)
	}

	// reconcile_steps should be > 0 since we had an orphaned body
	reconcileSteps, _ := healthResp["reconcile_steps"].(float64)
	if reconcileSteps < 1 {
		t.Errorf("reconcile_steps = %v, want >= 1", reconcileSteps)
	}

	// Stop daemon
	cancel()
	select {
	case err := <-daemonDone:
		if err != nil {
			t.Fatalf("daemon start error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("daemon did not stop")
	}

	// Reopen store and verify body transitioned to Error
	// Use fresh context since the daemon context was canceled
	verifyCtx := context.Background()
	s2, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("reopen store: %v", err)
	}
	defer s2.Close()

	bodyRec, err := s2.GetBody(verifyCtx, "crash-body-1")
	if err != nil {
		t.Fatalf("get body after reconcile: %v", err)
	}
	if bodyRec.State != adapter.StateError {
		t.Errorf("body state = %s, want Error after crash recovery", bodyRec.State)
	}
}

func TestConcurrentBodyOperations(t *testing.T) {
	s := tempStore(t)
	mockAdapter := &mockSubstrateAdapter{}
	bm := body.NewBodyManager(s, mockAdapter)
	ctx := context.Background()

	// Concurrent creates
	var wg sync.WaitGroup
	numBodies := 10
	bodyIDs := make([]string, numBodies)
	var mu sync.Mutex

	for i := 0; i < numBodies; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			b, err := bm.Create(ctx, fmt.Sprintf("concurrent-body-%d", idx), adapter.BodySpec{Image: "alpine:latest"})
			if err != nil {
				t.Errorf("create body %d: %v", idx, err)
				return
			}
			mu.Lock()
			bodyIDs[idx] = b.ID
			mu.Unlock()
		}(i)
	}
	wg.Wait()

	// Verify all bodies created
	bodies, err := s.ListBodies(ctx)
	if err != nil {
		t.Fatalf("list bodies: %v", err)
	}
	if len(bodies) != numBodies {
		t.Fatalf("bodies = %d, want %d", len(bodies), numBodies)
	}

	// Concurrent stops
	for i := 0; i < numBodies; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			mu.Lock()
			id := bodyIDs[idx]
			mu.Unlock()
			if id == "" {
				t.Errorf("body %d has no ID", idx)
				return
			}
			if err := bm.Stop(ctx, id, adapter.StopOpts{}); err != nil {
				t.Errorf("stop body %d: %v", idx, err)
			}
		}(i)
	}
	wg.Wait()

	// Verify all bodies stopped
	for _, b := range bodies {
		rec, err := s.GetBody(ctx, b.ID)
		if err != nil {
			t.Fatalf("get body %s: %v", b.ID, err)
		}
		if rec.State != adapter.StateStopped {
			t.Errorf("body %s state = %s, want Stopped", b.ID, rec.State)
		}
	}

	// Verify adapter calls are race-free
	if len(mockAdapter.created) != numBodies {
		t.Errorf("adapter creates = %d, want %d", len(mockAdapter.created), numBodies)
	}
	if len(mockAdapter.stopped) != numBodies {
		t.Errorf("adapter stops = %d, want %d", len(mockAdapter.stopped), numBodies)
	}
}

func TestBodyLifecycleFull(t *testing.T) {
	s := tempStore(t)
	mockAdapter := &mockSubstrateAdapter{}
	bm := body.NewBodyManager(s, mockAdapter)
	ctx := context.Background()

	// Step 1: Create body
	b, err := bm.Create(ctx, "lifecycle-full", adapter.BodySpec{Image: "alpine:latest"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if b.State != adapter.StateRunning {
		t.Fatalf("state after create = %s, want Running", b.State)
	}

	// Step 2: Exec command inside running body
	result, err := bm.Exec(ctx, b.ID, []string{"echo", "hello"})
	if err != nil {
		t.Fatalf("exec: %v", err)
	}
	if result.ExitCode != 0 {
		t.Errorf("exit code = %d, want 0", result.ExitCode)
	}

	// Step 3: Create snapshot via MCP
	h := newHarness(t, s)
	h.srv.SetBodyManager(bm)

	h.send(t, mcp.Request{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params: rawJSON(t, map[string]interface{}{
			"name":      "create_snapshot",
			"arguments": map[string]interface{}{"body_id": b.ID, "label": "test-snap"},
		}),
	})
	resp := h.readResponse(t)
	if resp["error"] != nil {
		t.Fatalf("create_snapshot error: %v", resp["error"])
	}
	resultMap := resp["result"].(map[string]interface{})
	content := resultMap["content"].([]interface{})
	text := content[0].(map[string]interface{})["text"].(string)
	var snapResp map[string]interface{}
	if err := json.Unmarshal([]byte(text), &snapResp); err != nil {
		t.Fatalf("unmarshal snapshot response: %v", err)
	}
	snapID := snapResp["id"].(string)
	if snapID == "" {
		t.Fatal("snapshot ID is empty")
	}

	// Step 4: Verify snapshot exists in store
	snap, err := s.GetSnapshot(ctx, snapID)
	if err != nil {
		t.Fatalf("get snapshot: %v", err)
	}
	if snap.BodyID != b.ID {
		t.Errorf("snapshot body_id = %q, want %q", snap.BodyID, b.ID)
	}

	// Step 5: Restore body from snapshot via MCP
	h.send(t, mcp.Request{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/call",
		Params: rawJSON(t, map[string]interface{}{
			"name":      "restore_body",
			"arguments": map[string]interface{}{"snapshot_id": snapID},
		}),
	})
	resp = h.readResponse(t)
	if resp["error"] != nil {
		t.Fatalf("restore_body error: %v", resp["error"])
	}

	// Step 6: Verify body still exists after restore
	bodyRec, err := s.GetBody(ctx, b.ID)
	if err != nil {
		t.Fatalf("get body after restore: %v", err)
	}
	if bodyRec.State != adapter.StateRunning {
		t.Errorf("body state after restore = %s, want Running", bodyRec.State)
	}

	// Step 7: Stop body
	if err := bm.Stop(ctx, b.ID, adapter.StopOpts{}); err != nil {
		t.Fatalf("stop: %v", err)
	}

	// Step 8: Destroy body
	if err := bm.Destroy(ctx, b.ID); err != nil {
		t.Fatalf("destroy: %v", err)
	}

	// Verify body deleted
	if _, err := s.GetBody(ctx, b.ID); err == nil {
		t.Fatal("body should be deleted after destroy")
	}

	// Cleanup MCP
	h.cancel()
	h.close()
	select {
	case <-h.done:
	case <-time.After(5 * time.Second):
		t.Fatal("MCP server did not shut down")
	}
}

func TestMCPToolsEndToEnd(t *testing.T) {
	s := tempStore(t)
	mockAdapter := &mockSubstrateAdapter{}
	bm := body.NewBodyManager(s, mockAdapter)
	ctx := context.Background()

	// Create a body to work with
	b, err := bm.Create(ctx, "mcp-tools-body", adapter.BodySpec{Image: "alpine:latest"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// Create a stopped body for start/stop tests
	stoppedBody, err := bm.Create(ctx, "mcp-stopped-body", adapter.BodySpec{Image: "alpine:latest"})
	if err != nil {
		t.Fatalf("create stopped body: %v", err)
	}
	if err := bm.Stop(ctx, stoppedBody.ID, adapter.StopOpts{}); err != nil {
		t.Fatalf("stop body: %v", err)
	}

	h := newHarness(t, s)
	h.srv.SetBodyManager(bm)

	// Test 1: execute_command
	h.send(t, mcp.Request{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params: rawJSON(t, map[string]interface{}{
			"name":      "execute_command",
			"arguments": map[string]interface{}{"body_id": b.ID, "command": []string{"echo", "hello"}},
		}),
	})
	resp := h.readResponse(t)
	if resp["error"] != nil {
		t.Fatalf("execute_command error: %v", resp["error"])
	}
	result := resp["result"].(map[string]interface{})
	content := result["content"].([]interface{})
	text := content[0].(map[string]interface{})["text"].(string)
	var execResp map[string]interface{}
	if err := json.Unmarshal([]byte(text), &execResp); err != nil {
		t.Fatalf("unmarshal exec response: %v", err)
	}
	if execResp["exit_code"] != float64(0) {
		t.Errorf("exit_code = %v, want 0", execResp["exit_code"])
	}

	// Test 2: get_body_status
	h.send(t, mcp.Request{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/call",
		Params: rawJSON(t, map[string]interface{}{
			"name":      "get_body_status",
			"arguments": map[string]interface{}{"body_id": b.ID},
		}),
	})
	resp = h.readResponse(t)
	if resp["error"] != nil {
		t.Fatalf("get_body_status error: %v", resp["error"])
	}
	result = resp["result"].(map[string]interface{})
	content = result["content"].([]interface{})
	text = content[0].(map[string]interface{})["text"].(string)
	var statusResp map[string]interface{}
	if err := json.Unmarshal([]byte(text), &statusResp); err != nil {
		t.Fatalf("unmarshal status response: %v", err)
	}
	if statusResp["state"] != "Running" {
		t.Errorf("status state = %v, want Running", statusResp["state"])
	}

	// Test 3: get_body_logs
	h.send(t, mcp.Request{
		JSONRPC: "2.0",
		ID:      3,
		Method:  "tools/call",
		Params: rawJSON(t, map[string]interface{}{
			"name":      "get_body_logs",
			"arguments": map[string]interface{}{"body_id": b.ID, "tail": 10},
		}),
	})
	resp = h.readResponse(t)
	if resp["error"] != nil {
		t.Fatalf("get_body_logs error: %v", resp["error"])
	}
	result = resp["result"].(map[string]interface{})
	content = result["content"].([]interface{})
	text = content[0].(map[string]interface{})["text"].(string)
	var logsResp map[string]interface{}
	if err := json.Unmarshal([]byte(text), &logsResp); err != nil {
		t.Fatalf("unmarshal logs response: %v", err)
	}
	if logsResp["body_id"] != b.ID {
		t.Errorf("logs body_id = %v, want %s", logsResp["body_id"], b.ID)
	}

	// Test 4: stop_body
	h.send(t, mcp.Request{
		JSONRPC: "2.0",
		ID:      4,
		Method:  "tools/call",
		Params: rawJSON(t, map[string]interface{}{
			"name":      "stop_body",
			"arguments": map[string]interface{}{"body_id": b.ID},
		}),
	})
	resp = h.readResponse(t)
	if resp["error"] != nil {
		t.Fatalf("stop_body error: %v", resp["error"])
	}
	result = resp["result"].(map[string]interface{})
	content = result["content"].([]interface{})
	text = content[0].(map[string]interface{})["text"].(string)
	var stopResp map[string]interface{}
	if err := json.Unmarshal([]byte(text), &stopResp); err != nil {
		t.Fatalf("unmarshal stop response: %v", err)
	}
	if stopResp["state"] != "Stopped" {
		t.Errorf("stop state = %v, want Stopped", stopResp["state"])
	}

	// Test 5: start_body (on the stopped body)
	h.send(t, mcp.Request{
		JSONRPC: "2.0",
		ID:      5,
		Method:  "tools/call",
		Params: rawJSON(t, map[string]interface{}{
			"name":      "start_body",
			"arguments": map[string]interface{}{"body_id": stoppedBody.ID},
		}),
	})
	resp = h.readResponse(t)
	if resp["error"] != nil {
		t.Fatalf("start_body error: %v", resp["error"])
	}
	result = resp["result"].(map[string]interface{})
	content = result["content"].([]interface{})
	text = content[0].(map[string]interface{})["text"].(string)
	var startResp map[string]interface{}
	if err := json.Unmarshal([]byte(text), &startResp); err != nil {
		t.Fatalf("unmarshal start response: %v", err)
	}
	if startResp["state"] != "Running" {
		t.Errorf("start state = %v, want Running", startResp["state"])
	}

	// Test 6: delete_body (on the originally stopped body, now running again)
	// First stop it
	if err := bm.Stop(ctx, stoppedBody.ID, adapter.StopOpts{}); err != nil {
		t.Fatalf("pre-stop for delete: %v", err)
	}

	h.send(t, mcp.Request{
		JSONRPC: "2.0",
		ID:      6,
		Method:  "tools/call",
		Params: rawJSON(t, map[string]interface{}{
			"name":      "delete_body",
			"arguments": map[string]interface{}{"id": stoppedBody.ID},
		}),
	})
	resp = h.readResponse(t)
	if resp["error"] != nil {
		t.Fatalf("delete_body error: %v", resp["error"])
	}
	result = resp["result"].(map[string]interface{})
	content = result["content"].([]interface{})
	text = content[0].(map[string]interface{})["text"].(string)
	var deleteResp map[string]bool
	if err := json.Unmarshal([]byte(text), &deleteResp); err != nil {
		t.Fatalf("unmarshal delete response: %v", err)
	}
	if !deleteResp["deleted"] {
		t.Errorf("deleted = %v, want true", deleteResp["deleted"])
	}

	// Verify deleted
	if _, err := s.GetBody(ctx, stoppedBody.ID); err == nil {
		t.Fatal("body should be deleted")
	}

	// Cleanup
	h.cancel()
	h.close()
	select {
	case <-h.done:
	case <-time.After(5 * time.Second):
		t.Fatal("MCP server did not shut down")
	}
}
