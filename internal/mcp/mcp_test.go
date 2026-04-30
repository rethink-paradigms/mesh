package mcp

import (
	"archive/tar"
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/klauspost/compress/zstd"
	"github.com/rethink-paradigms/mesh/internal/adapter"
	"github.com/rethink-paradigms/mesh/internal/body"
	"github.com/rethink-paradigms/mesh/internal/plugin"
	"github.com/rethink-paradigms/mesh/internal/store"
)

type mockSubstrateAdapter struct {
	handle      adapter.Handle
	status      adapter.BodyStatus
	execOutputs map[string]adapter.ExecResult
}

func (m *mockSubstrateAdapter) Create(_ context.Context, _ adapter.BodySpec) (adapter.Handle, error) {
	if m.handle == "" {
		return "mock-handle-1", nil
	}
	return m.handle, nil
}
func (m *mockSubstrateAdapter) Start(_ context.Context, _ adapter.Handle) error  { return nil }
func (m *mockSubstrateAdapter) Stop(_ context.Context, _ adapter.Handle, _ adapter.StopOpts) error {
	return nil
}
func (m *mockSubstrateAdapter) Destroy(_ context.Context, _ adapter.Handle) error { return nil }
func (m *mockSubstrateAdapter) GetStatus(_ context.Context, _ adapter.Handle) (adapter.BodyStatus, error) {
	if m.status.State != "" {
		return m.status, nil
	}
	return adapter.BodyStatus{State: adapter.StateRunning}, nil
}
func (m *mockSubstrateAdapter) Exec(ctx context.Context, _ adapter.Handle, cmd []string) (adapter.ExecResult, error) {
	if len(cmd) >= 2 && cmd[0] == "sleep" {
		select {
		case <-ctx.Done():
			return adapter.ExecResult{}, ctx.Err()
		case <-time.After(10 * time.Second):
			return adapter.ExecResult{}, nil
		}
	}
	key := strings.Join(cmd, " ")
	if m.execOutputs != nil {
		if result, ok := m.execOutputs[key]; ok {
			return result, nil
		}
	}
	return adapter.ExecResult{Stdout: "ok", ExitCode: 0}, nil
}
func (m *mockSubstrateAdapter) ExportFilesystem(_ context.Context, _ adapter.Handle) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("")), nil
}
func (m *mockSubstrateAdapter) ImportFilesystem(_ context.Context, _ adapter.Handle, _ io.Reader, _ adapter.ImportOpts) error {
	return nil
}
func (m *mockSubstrateAdapter) Inspect(_ context.Context, _ adapter.Handle) (adapter.ContainerMetadata, error) {
	return adapter.ContainerMetadata{}, nil
}
func (m *mockSubstrateAdapter) Capabilities() adapter.AdapterCapabilities {
	return adapter.AdapterCapabilities{}
}

func (m *mockSubstrateAdapter) SubstrateName() string {
	return "mock"
}

func (m *mockSubstrateAdapter) IsHealthy(_ context.Context) bool {
	return true
}

func tempStore(t *testing.T) *store.Store {
	t.Helper()
	f, err := os.CreateTemp("", "mcp-test-*.db")
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

func testBodyManager(t *testing.T, s *store.Store) *body.BodyManager {
	t.Helper()
	return body.NewBodyManager(s, &mockSubstrateAdapter{})
}

func testMigrator(t *testing.T, s *store.Store, bm *body.BodyManager) *body.MigrationCoordinator {
	t.Helper()
	return body.NewMigrationCoordinator(s, &mockSubstrateAdapter{}, bm, nil)
}

type testHarness struct {
	srv      *Server
	stdinW   *os.File
	stdoutR  *os.File
	scanner  *bufio.Scanner
	cancel   context.CancelFunc
	done     chan error
}

func newHarness(t *testing.T, s *store.Store) *testHarness {
	t.Helper()
	inR, inW, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe in: %v", err)
	}
	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe out: %v", err)
	}

	srv := NewWithIO(s, inR, outW)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- srv.Start(ctx)
		inR.Close()
	}()

	scanner := bufio.NewScanner(outR)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	return &testHarness{
		srv:     srv,
		stdinW:  inW,
		stdoutR: outR,
		scanner: scanner,
		cancel:  cancel,
		done:    done,
	}
}

func (h *testHarness) send(t *testing.T, v interface{}) {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	h.stdinW.Write(append(data, '\n'))
}

func (h *testHarness) close() {
	h.stdinW.Close()
}

func (h *testHarness) readResponse(t *testing.T) map[string]interface{} {
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
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for response")
		return nil
	}
}

func TestInitialize(t *testing.T) {
	s := tempStore(t)
	h := newHarness(t, s)
	defer h.close()

	h.send(t, Request{JSONRPC: "2.0", ID: 1, Method: "initialize"})

	resp := h.readResponse(t)
	if resp["id"].(float64) != 1 {
		t.Fatalf("id = %v, want 1", resp["id"])
	}
	result := resp["result"].(map[string]interface{})
	if result["protocolVersion"] == "" {
		t.Fatal("expected protocolVersion in initialize response")
	}
	caps, ok := result["capabilities"].(map[string]interface{})
	if !ok {
		t.Fatal("expected capabilities map")
	}
	if _, ok := caps["tools"]; !ok {
		t.Fatal("expected tools capability")
	}
}

func TestPing(t *testing.T) {
	s := tempStore(t)
	h := newHarness(t, s)
	defer h.close()

	h.send(t, Request{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/call",
		Params:  rawMessage(t, map[string]interface{}{"name": "ping", "arguments": map[string]interface{}{}}),
	})

	resp := h.readResponse(t)
	result := resp["result"].(map[string]interface{})
	content := result["content"].([]interface{})
	text := content[0].(map[string]interface{})["text"].(string)

	var pong map[string]interface{}
	if err := json.Unmarshal([]byte(text), &pong); err != nil {
		t.Fatalf("unmarshal pong: %v", err)
	}
	if pong["pong"] != true {
		t.Fatalf("pong = %v, want true", pong["pong"])
	}
}

func TestListBodies(t *testing.T) {
	s := tempStore(t)
	ctx := context.Background()
	s.CreateBody(ctx, "b1", "test-body", adapter.StateRunning, `{"image":"alpine"}`, "docker", "inst-1")

	h := newHarness(t, s)
	defer h.close()

	h.send(t, Request{
		JSONRPC: "2.0",
		ID:      3,
		Method:  "tools/call",
		Params:  rawMessage(t, map[string]interface{}{"name": "list_bodies", "arguments": map[string]interface{}{}}),
	})

	resp := h.readResponse(t)
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
}

func TestGetBody(t *testing.T) {
	s := tempStore(t)
	ctx := context.Background()
	s.CreateBody(ctx, "b1", "my-body", adapter.StateRunning, `{"image":"alpine"}`, "docker", "inst-1")

	h := newHarness(t, s)
	defer h.close()

	h.send(t, Request{
		JSONRPC: "2.0",
		ID:      4,
		Method:  "tools/call",
		Params:  rawMessage(t, map[string]interface{}{"name": "get_body", "arguments": map[string]interface{}{"id": "b1"}}),
	})

	resp := h.readResponse(t)
	result := resp["result"].(map[string]interface{})
	content := result["content"].([]interface{})
	text := content[0].(map[string]interface{})["text"].(string)

	var body map[string]interface{}
	if err := json.Unmarshal([]byte(text), &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if body["ID"] != "b1" {
		t.Fatalf("ID = %v, want b1", body["ID"])
	}
	if body["Name"] != "my-body" {
		t.Fatalf("Name = %v, want my-body", body["Name"])
	}
}

func TestGetBodyNotFound(t *testing.T) {
	s := tempStore(t)
	h := newHarness(t, s)
	defer h.close()

	h.send(t, Request{
		JSONRPC: "2.0",
		ID:      5,
		Method:  "tools/call",
		Params:  rawMessage(t, map[string]interface{}{"name": "get_body", "arguments": map[string]interface{}{"id": "nonexistent"}}),
	})

	resp := h.readResponse(t)
	rpcErr := resp["error"].(map[string]interface{})
	code := rpcErr["code"].(float64)
	if code != -32603 {
		t.Fatalf("error code = %v, want -32603", code)
	}
	msg := rpcErr["message"].(string)
	if !strings.Contains(msg, "not found") {
		t.Fatalf("error message = %q, want 'not found'", msg)
	}
}

func TestGetSnapshot(t *testing.T) {
	s := tempStore(t)
	ctx := context.Background()
	s.CreateBody(ctx, "b1", "body1", adapter.StateRunning, `{}`, "docker", "inst-1")
	s.CreateSnapshot(ctx, "snap1", "b1", `{"checksum":"abc"}`, "/tmp/snap1.tar.zst", 1024)

	h := newHarness(t, s)
	defer h.close()

	h.send(t, Request{
		JSONRPC: "2.0",
		ID:      6,
		Method:  "tools/call",
		Params:  rawMessage(t, map[string]interface{}{"name": "get_snapshot", "arguments": map[string]interface{}{"id": "snap1"}}),
	})

	resp := h.readResponse(t)
	result := resp["result"].(map[string]interface{})
	content := result["content"].([]interface{})
	text := content[0].(map[string]interface{})["text"].(string)

	var snap map[string]interface{}
	if err := json.Unmarshal([]byte(text), &snap); err != nil {
		t.Fatalf("unmarshal snapshot: %v", err)
	}
	if snap["ID"] != "snap1" {
		t.Fatalf("ID = %v, want snap1", snap["ID"])
	}
}

func TestExecCommandSuccess(t *testing.T) {
	s := tempStore(t)
	bm := testBodyManager(t, s)
	ctx := context.Background()

	created, err := bm.Create(ctx, "exec-test", adapter.BodySpec{Image: "alpine"})
	if err != nil {
		t.Fatalf("create body: %v", err)
	}

	h := newHarness(t, s)
	h.srv.SetBodyManager(bm)
	defer h.close()

	h.send(t, Request{
		JSONRPC: "2.0",
		ID:      7,
		Method:  "tools/call",
		Params: rawMessage(t, map[string]interface{}{
			"name":      "execute_command",
			"arguments": map[string]interface{}{"body_id": created.ID, "command": []string{"echo", "hello"}},
		}),
	})

	resp := h.readResponse(t)
	result := resp["result"].(map[string]interface{})
	content := result["content"].([]interface{})
	text := content[0].(map[string]interface{})["text"].(string)

	var execResp map[string]interface{}
	if err := json.Unmarshal([]byte(text), &execResp); err != nil {
		t.Fatalf("unmarshal exec response: %v", err)
	}
	if execResp["stdout"] != "ok" {
		t.Fatalf("stdout = %v, want 'ok'", execResp["stdout"])
	}
	if execResp["exit_code"] != float64(0) {
		t.Fatalf("exit_code = %v, want 0", execResp["exit_code"])
	}
}

func TestExecCommandNotRunning(t *testing.T) {
	s := tempStore(t)
	bm := testBodyManager(t, s)
	ctx := context.Background()

	created, err := bm.Create(ctx, "exec-stopped", adapter.BodySpec{Image: "alpine"})
	if err != nil {
		t.Fatalf("create body: %v", err)
	}
	if err := bm.Stop(ctx, created.ID, adapter.StopOpts{}); err != nil {
		t.Fatalf("stop body: %v", err)
	}

	h := newHarness(t, s)
	h.srv.SetBodyManager(bm)
	defer h.close()

	h.send(t, Request{
		JSONRPC: "2.0",
		ID:      7,
		Method:  "tools/call",
		Params: rawMessage(t, map[string]interface{}{
			"name":      "execute_command",
			"arguments": map[string]interface{}{"body_id": created.ID, "command": []string{"echo", "hello"}},
		}),
	})

	resp := h.readResponse(t)
	rpcErr := resp["error"].(map[string]interface{})
	msg := rpcErr["message"].(string)
	if !strings.Contains(msg, "not running") {
		t.Fatalf("error message = %q, want 'not running'", msg)
	}
}

func TestExecCommandTimeout(t *testing.T) {
	s := tempStore(t)
	bm := testBodyManager(t, s)
	ctx := context.Background()

	created, err := bm.Create(ctx, "exec-timeout", adapter.BodySpec{Image: "alpine"})
	if err != nil {
		t.Fatalf("create body: %v", err)
	}

	h := newHarness(t, s)
	h.srv.SetBodyManager(bm)
	defer h.close()

	h.send(t, Request{
		JSONRPC: "2.0",
		ID:      7,
		Method:  "tools/call",
		Params: rawMessage(t, map[string]interface{}{
			"name":      "execute_command",
			"arguments": map[string]interface{}{"body_id": created.ID, "command": []string{"sleep", "10"}, "timeout_seconds": 1},
		}),
	})

	resp := h.readResponse(t)
	rpcErr := resp["error"].(map[string]interface{})
	msg := rpcErr["message"].(string)
	if !strings.Contains(msg, "timeout") {
		t.Fatalf("error message = %q, want 'timeout'", msg)
	}
}

func TestExecCommandEmptyCommand(t *testing.T) {
	s := tempStore(t)
	bm := testBodyManager(t, s)
	ctx := context.Background()

	created, err := bm.Create(ctx, "exec-empty", adapter.BodySpec{Image: "alpine"})
	if err != nil {
		t.Fatalf("create body: %v", err)
	}

	h := newHarness(t, s)
	h.srv.SetBodyManager(bm)
	defer h.close()

	h.send(t, Request{
		JSONRPC: "2.0",
		ID:      7,
		Method:  "tools/call",
		Params: rawMessage(t, map[string]interface{}{
			"name":      "execute_command",
			"arguments": map[string]interface{}{"body_id": created.ID, "command": []string{}},
		}),
	})

	resp := h.readResponse(t)
	rpcErr := resp["error"].(map[string]interface{})
	msg := rpcErr["message"].(string)
	if !strings.Contains(msg, "command are required") {
		t.Fatalf("error message = %q, want 'command are required'", msg)
	}
}

func TestExecCommandBodyNotFound(t *testing.T) {
	s := tempStore(t)
	bm := testBodyManager(t, s)

	h := newHarness(t, s)
	h.srv.SetBodyManager(bm)
	defer h.close()

	h.send(t, Request{
		JSONRPC: "2.0",
		ID:      7,
		Method:  "tools/call",
		Params: rawMessage(t, map[string]interface{}{
			"name":      "execute_command",
			"arguments": map[string]interface{}{"body_id": "nonexistent", "command": []string{"echo", "hello"}},
		}),
	})

	resp := h.readResponse(t)
	rpcErr := resp["error"].(map[string]interface{})
	msg := rpcErr["message"].(string)
	if !strings.Contains(msg, "not found") {
		t.Fatalf("error message = %q, want 'not found'", msg)
	}
}

func TestExecCommandNoBodyManager(t *testing.T) {
	s := tempStore(t)
	h := newHarness(t, s)
	defer h.close()

	h.send(t, Request{
		JSONRPC: "2.0",
		ID:      7,
		Method:  "tools/call",
		Params: rawMessage(t, map[string]interface{}{
			"name":      "execute_command",
			"arguments": map[string]interface{}{"body_id": "b1", "command": []string{"ls"}},
		}),
	})

	resp := h.readResponse(t)
	rpcErr := resp["error"].(map[string]interface{})
	msg := rpcErr["message"].(string)
	if !strings.Contains(msg, "body manager not available") {
		t.Fatalf("error message = %q, want 'body manager not available'", msg)
	}
}

func TestToolsList(t *testing.T) {
	s := tempStore(t)
	h := newHarness(t, s)
	defer h.close()

	h.send(t, Request{JSONRPC: "2.0", ID: 8, Method: "tools/list"})

	resp := h.readResponse(t)
	result := resp["result"].(map[string]interface{})
	tools := result["tools"].([]interface{})

	names := map[string]bool{}
	for _, tool := range tools {
		name := tool.(map[string]interface{})["name"].(string)
		names[name] = true
	}
	for _, want := range []string{"ping", "list_bodies", "get_body", "get_snapshot", "execute_command", "create_body", "delete_body", "migrate_body"} {
		if !names[want] {
			t.Errorf("missing tool %q in tools/list", want)
		}
	}
}

func TestInvalidJSON(t *testing.T) {
	s := tempStore(t)
	h := newHarness(t, s)
	defer h.close()

	h.stdinW.Write([]byte("not-json\n"))

	resp := h.readResponse(t)
	rpcErr := resp["error"].(map[string]interface{})
	code := rpcErr["code"].(float64)
	if code != -32700 {
		t.Fatalf("error code = %v, want -32700 (parse error)", code)
	}
}

func TestUnknownMethod(t *testing.T) {
	s := tempStore(t)
	h := newHarness(t, s)
	defer h.close()

	h.send(t, Request{JSONRPC: "2.0", ID: 9, Method: "nonexistent/method"})

	resp := h.readResponse(t)
	rpcErr := resp["error"].(map[string]interface{})
	code := rpcErr["code"].(float64)
	if code != -32601 {
		t.Fatalf("error code = %v, want -32601 (method not found)", code)
	}
}

func TestToolNotFound(t *testing.T) {
	s := tempStore(t)
	h := newHarness(t, s)
	defer h.close()

	h.send(t, Request{
		JSONRPC: "2.0",
		ID:      10,
		Method:  "tools/call",
		Params:  rawMessage(t, map[string]interface{}{"name": "nonexistent_tool", "arguments": map[string]interface{}{}}),
	})

	resp := h.readResponse(t)
	rpcErr := resp["error"].(map[string]interface{})
	code := rpcErr["code"].(float64)
	if code != -32601 {
		t.Fatalf("error code = %v, want -32601", code)
	}
}

func TestGracefulShutdown(t *testing.T) {
	s := tempStore(t)
	h := newHarness(t, s)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := h.srv.Stop(ctx)
	if err != nil {
		t.Fatalf("Stop: %v", err)
	}
	h.close()

	select {
	case <-h.done:
	case <-time.After(3 * time.Second):
		t.Fatal("server did not stop")
	}
}

func TestDaemonSetMCPIntegration(t *testing.T) {
	s := tempStore(t)
	srv := New(s)

	daemonIface := srv
	_ = daemonIface

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if err := srv.Stop(ctx); err != nil {
		t.Fatalf("Stop: %v", err)
	}
}

func TestCreateBody(t *testing.T) {
	s := tempStore(t)
	bm := testBodyManager(t, s)
	h := newHarness(t, s)
	h.srv.SetBodyManager(bm)
	defer h.close()

	h.send(t, Request{
		JSONRPC: "2.0",
		ID:      20,
		Method:  "tools/call",
		Params:  rawMessage(t, map[string]interface{}{"name": "create_body", "arguments": map[string]interface{}{"name": "test-body", "image": "alpine:latest"}}),
	})

	resp := h.readResponse(t)
	result := resp["result"].(map[string]interface{})
	content := result["content"].([]interface{})
	text := content[0].(map[string]interface{})["text"].(string)

	var body map[string]interface{}
	if err := json.Unmarshal([]byte(text), &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if body["name"] != "test-body" {
		t.Fatalf("name = %v, want test-body", body["name"])
	}
	if body["state"] != "Running" {
		t.Fatalf("state = %v, want Running", body["state"])
	}
	if body["id"] == "" {
		t.Fatal("expected non-empty id")
	}
	if body["handle"] == "" {
		t.Fatal("expected non-empty handle")
	}
}

func TestCreateBodyMissingRequired(t *testing.T) {
	s := tempStore(t)
	bm := testBodyManager(t, s)
	h := newHarness(t, s)
	h.srv.SetBodyManager(bm)
	defer h.close()

	h.send(t, Request{
		JSONRPC: "2.0",
		ID:      21,
		Method:  "tools/call",
		Params:  rawMessage(t, map[string]interface{}{"name": "create_body", "arguments": map[string]interface{}{"name": "test-body"}}),
	})

	resp := h.readResponse(t)
	rpcErr := resp["error"].(map[string]interface{})
	msg := rpcErr["message"].(string)
	if !strings.Contains(msg, "image") {
		t.Fatalf("error message = %q, want mention of image requirement", msg)
	}
}

func TestCreateBodyNoBodyManager(t *testing.T) {
	s := tempStore(t)
	h := newHarness(t, s)
	defer h.close()

	h.send(t, Request{
		JSONRPC: "2.0",
		ID:      22,
		Method:  "tools/call",
		Params:  rawMessage(t, map[string]interface{}{"name": "create_body", "arguments": map[string]interface{}{"name": "test-body", "image": "alpine"}}),
	})

	resp := h.readResponse(t)
	rpcErr := resp["error"].(map[string]interface{})
	msg := rpcErr["message"].(string)
	if !strings.Contains(msg, "body manager not available") {
		t.Fatalf("error message = %q, want 'body manager not available'", msg)
	}
}

func TestDeleteBody(t *testing.T) {
	s := tempStore(t)
	bm := testBodyManager(t, s)
	ctx := context.Background()

	created, err := bm.Create(ctx, "to-delete", adapter.BodySpec{Image: "alpine"})
	if err != nil {
		t.Fatalf("create body: %v", err)
	}

	_ = bm.Stop(ctx, created.ID, adapter.StopOpts{})

	h := newHarness(t, s)
	h.srv.SetBodyManager(bm)
	defer h.close()

	h.send(t, Request{
		JSONRPC: "2.0",
		ID:      23,
		Method:  "tools/call",
		Params:  rawMessage(t, map[string]interface{}{"name": "delete_body", "arguments": map[string]interface{}{"id": created.ID}}),
	})

	resp := h.readResponse(t)
	result := resp["result"].(map[string]interface{})
	content := result["content"].([]interface{})
	text := content[0].(map[string]interface{})["text"].(string)

	var delResp map[string]interface{}
	if err := json.Unmarshal([]byte(text), &delResp); err != nil {
		t.Fatalf("unmarshal delete response: %v", err)
	}
	if delResp["deleted"] != true {
		t.Fatalf("deleted = %v, want true", delResp["deleted"])
	}
}

func TestDeleteBodyNoBodyManager(t *testing.T) {
	s := tempStore(t)
	h := newHarness(t, s)
	defer h.close()

	h.send(t, Request{
		JSONRPC: "2.0",
		ID:      24,
		Method:  "tools/call",
		Params:  rawMessage(t, map[string]interface{}{"name": "delete_body", "arguments": map[string]interface{}{"id": "b1"}}),
	})

	resp := h.readResponse(t)
	rpcErr := resp["error"].(map[string]interface{})
	msg := rpcErr["message"].(string)
	if !strings.Contains(msg, "body manager not available") {
		t.Fatalf("error message = %q, want 'body manager not available'", msg)
	}
}

func TestMigrateBody(t *testing.T) {
	s := tempStore(t)
	bm := testBodyManager(t, s)
	mig := testMigrator(t, s, bm)

	ctx := context.Background()
	created, err := bm.Create(ctx, "to-migrate", adapter.BodySpec{Image: "alpine"})
	if err != nil {
		t.Fatalf("create body: %v", err)
	}

	h := newHarness(t, s)
	h.srv.SetBodyManager(bm)
	h.srv.SetMigrator(mig)
	defer h.close()

	h.send(t, Request{
		JSONRPC: "2.0",
		ID:      25,
		Method:  "tools/call",
		Params:  rawMessage(t, map[string]interface{}{"name": "migrate_body", "arguments": map[string]interface{}{"body_id": created.ID, "target_substrate": "fleet"}}),
	})

	resp := h.readResponse(t)
	result := resp["result"].(map[string]interface{})
	content := result["content"].([]interface{})
	text := content[0].(map[string]interface{})["text"].(string)

	var migResp map[string]interface{}
	if err := json.Unmarshal([]byte(text), &migResp); err != nil {
		t.Fatalf("unmarshal migrate response: %v", err)
	}
	if migResp["migration_id"] == "" {
		t.Fatal("expected non-empty migration_id")
	}
}

func TestMigrateBodyNoMigrator(t *testing.T) {
	s := tempStore(t)
	h := newHarness(t, s)
	defer h.close()

	h.send(t, Request{
		JSONRPC: "2.0",
		ID:      26,
		Method:  "tools/call",
		Params:  rawMessage(t, map[string]interface{}{"name": "migrate_body", "arguments": map[string]interface{}{"body_id": "b1", "target_substrate": "fleet"}}),
	})

	resp := h.readResponse(t)
	rpcErr := resp["error"].(map[string]interface{})
	msg := rpcErr["message"].(string)
	if !strings.Contains(msg, "migration coordinator not available") {
		t.Fatalf("error message = %q, want 'migration coordinator not available'", msg)
	}
}

func TestStartBody(t *testing.T) {
	s := tempStore(t)
	bm := testBodyManager(t, s)
	ctx := context.Background()

	created, err := bm.Create(ctx, "to-start", adapter.BodySpec{Image: "alpine"})
	if err != nil {
		t.Fatalf("create body: %v", err)
	}
	if err := bm.Stop(ctx, created.ID, adapter.StopOpts{}); err != nil {
		t.Fatalf("stop body: %v", err)
	}

	h := newHarness(t, s)
	h.srv.SetBodyManager(bm)
	defer h.close()

	h.send(t, Request{
		JSONRPC: "2.0",
		ID:      30,
		Method:  "tools/call",
		Params:  rawMessage(t, map[string]interface{}{"name": "start_body", "arguments": map[string]interface{}{"body_id": created.ID}}),
	})

	resp := h.readResponse(t)
	if resp["error"] != nil {
		rpcErr := resp["error"].(map[string]interface{})
		t.Fatalf("unexpected error: code=%v msg=%v", rpcErr["code"], rpcErr["message"])
	}
	result := resp["result"].(map[string]interface{})
	content := result["content"].([]interface{})
	text := content[0].(map[string]interface{})["text"].(string)

	var startResp map[string]interface{}
	if err := json.Unmarshal([]byte(text), &startResp); err != nil {
		t.Fatalf("unmarshal start response: %v", err)
	}
	if startResp["state"] != "Running" {
		t.Fatalf("state = %v, want Running", startResp["state"])
	}
	if startResp["id"] != created.ID {
		t.Fatalf("id = %v, want %s", startResp["id"], created.ID)
	}
}

func TestStopBody(t *testing.T) {
	s := tempStore(t)
	bm := testBodyManager(t, s)
	ctx := context.Background()

	created, err := bm.Create(ctx, "to-stop", adapter.BodySpec{Image: "alpine"})
	if err != nil {
		t.Fatalf("create body: %v", err)
	}

	h := newHarness(t, s)
	h.srv.SetBodyManager(bm)
	defer h.close()

	h.send(t, Request{
		JSONRPC: "2.0",
		ID:      31,
		Method:  "tools/call",
		Params:  rawMessage(t, map[string]interface{}{"name": "stop_body", "arguments": map[string]interface{}{"body_id": created.ID}}),
	})

	resp := h.readResponse(t)
	result := resp["result"].(map[string]interface{})
	content := result["content"].([]interface{})
	text := content[0].(map[string]interface{})["text"].(string)

	var stopResp map[string]interface{}
	if err := json.Unmarshal([]byte(text), &stopResp); err != nil {
		t.Fatalf("unmarshal stop response: %v", err)
	}
	if stopResp["state"] != "Stopped" {
		t.Fatalf("state = %v, want Stopped", stopResp["state"])
	}
	if stopResp["id"] != created.ID {
		t.Fatalf("id = %v, want %s", stopResp["id"], created.ID)
	}
}

func TestStartBodyAlreadyRunning(t *testing.T) {
	s := tempStore(t)
	bm := testBodyManager(t, s)
	ctx := context.Background()

	created, err := bm.Create(ctx, "already-running", adapter.BodySpec{Image: "alpine"})
	if err != nil {
		t.Fatalf("create body: %v", err)
	}

	h := newHarness(t, s)
	h.srv.SetBodyManager(bm)
	defer h.close()

	h.send(t, Request{
		JSONRPC: "2.0",
		ID:      32,
		Method:  "tools/call",
		Params:  rawMessage(t, map[string]interface{}{"name": "start_body", "arguments": map[string]interface{}{"body_id": created.ID}}),
	})

	resp := h.readResponse(t)
	rpcErr := resp["error"].(map[string]interface{})
	msg := rpcErr["message"].(string)
	if !strings.Contains(msg, "cannot start body in state") {
		t.Fatalf("error message = %q, want 'cannot start body in state'", msg)
	}
}

func TestStopBodyAlreadyStopped(t *testing.T) {
	s := tempStore(t)
	bm := testBodyManager(t, s)
	ctx := context.Background()

	created, err := bm.Create(ctx, "already-stopped", adapter.BodySpec{Image: "alpine"})
	if err != nil {
		t.Fatalf("create body: %v", err)
	}
	if err := bm.Stop(ctx, created.ID, adapter.StopOpts{}); err != nil {
		t.Fatalf("stop body: %v", err)
	}

	h := newHarness(t, s)
	h.srv.SetBodyManager(bm)
	defer h.close()

	h.send(t, Request{
		JSONRPC: "2.0",
		ID:      33,
		Method:  "tools/call",
		Params:  rawMessage(t, map[string]interface{}{"name": "stop_body", "arguments": map[string]interface{}{"body_id": created.ID}}),
	})

	resp := h.readResponse(t)
	rpcErr := resp["error"].(map[string]interface{})
	msg := rpcErr["message"].(string)
	if !strings.Contains(msg, "invalid transition") {
		t.Fatalf("error message = %q, want 'invalid transition'", msg)
	}
}

func TestStartBodyNoBodyManager(t *testing.T) {
	s := tempStore(t)
	h := newHarness(t, s)
	defer h.close()

	h.send(t, Request{
		JSONRPC: "2.0",
		ID:      34,
		Method:  "tools/call",
		Params:  rawMessage(t, map[string]interface{}{"name": "start_body", "arguments": map[string]interface{}{"body_id": "b1"}}),
	})

	resp := h.readResponse(t)
	rpcErr := resp["error"].(map[string]interface{})
	msg := rpcErr["message"].(string)
	if !strings.Contains(msg, "body manager not available") {
		t.Fatalf("error message = %q, want 'body manager not available'", msg)
	}
}

func TestStopBodyNoBodyManager(t *testing.T) {
	s := tempStore(t)
	h := newHarness(t, s)
	defer h.close()

	h.send(t, Request{
		JSONRPC: "2.0",
		ID:      35,
		Method:  "tools/call",
		Params:  rawMessage(t, map[string]interface{}{"name": "stop_body", "arguments": map[string]interface{}{"body_id": "b1"}}),
	})

	resp := h.readResponse(t)
	rpcErr := resp["error"].(map[string]interface{})
	msg := rpcErr["message"].(string)
	if !strings.Contains(msg, "body manager not available") {
		t.Fatalf("error message = %q, want 'body manager not available'", msg)
	}
}

func TestStartBodyMissingID(t *testing.T) {
	s := tempStore(t)
	bm := testBodyManager(t, s)
	h := newHarness(t, s)
	h.srv.SetBodyManager(bm)
	defer h.close()

	h.send(t, Request{
		JSONRPC: "2.0",
		ID:      36,
		Method:  "tools/call",
		Params:  rawMessage(t, map[string]interface{}{"name": "start_body", "arguments": map[string]interface{}{}}),
	})

	resp := h.readResponse(t)
	rpcErr := resp["error"].(map[string]interface{})
	msg := rpcErr["message"].(string)
	if !strings.Contains(msg, "body_id") {
		t.Fatalf("error message = %q, want mention of body_id", msg)
	}
}

func TestStopBodyMissingID(t *testing.T) {
	s := tempStore(t)
	bm := testBodyManager(t, s)
	h := newHarness(t, s)
	h.srv.SetBodyManager(bm)
	defer h.close()

	h.send(t, Request{
		JSONRPC: "2.0",
		ID:      37,
		Method:  "tools/call",
		Params:  rawMessage(t, map[string]interface{}{"name": "stop_body", "arguments": map[string]interface{}{}}),
	})

	resp := h.readResponse(t)
	rpcErr := resp["error"].(map[string]interface{})
	msg := rpcErr["message"].(string)
	if !strings.Contains(msg, "body_id") {
		t.Fatalf("error message = %q, want mention of body_id", msg)
	}
}

func TestMigrateBodyMissingParams(t *testing.T) {
	s := tempStore(t)
	bm := testBodyManager(t, s)
	mig := testMigrator(t, s, bm)
	h := newHarness(t, s)
	h.srv.SetBodyManager(bm)
	h.srv.SetMigrator(mig)
	defer h.close()

	h.send(t, Request{
		JSONRPC: "2.0",
		ID:      27,
		Method:  "tools/call",
		Params:  rawMessage(t, map[string]interface{}{"name": "migrate_body", "arguments": map[string]interface{}{"body_id": "b1"}}),
	})

	resp := h.readResponse(t)
	rpcErr := resp["error"].(map[string]interface{})
	msg := rpcErr["message"].(string)
	if !strings.Contains(msg, "body_id and target_substrate are required") {
		t.Fatalf("error message = %q, want required fields message", msg)
	}
}

func TestDeleteBodyMissingID(t *testing.T) {
	s := tempStore(t)
	bm := testBodyManager(t, s)
	h := newHarness(t, s)
	h.srv.SetBodyManager(bm)
	defer h.close()

	h.send(t, Request{
		JSONRPC: "2.0",
		ID:      28,
		Method:  "tools/call",
		Params:  rawMessage(t, map[string]interface{}{"name": "delete_body", "arguments": map[string]interface{}{}}),
	})

	resp := h.readResponse(t)
	rpcErr := resp["error"].(map[string]interface{})
	msg := rpcErr["message"].(string)
	if !strings.Contains(msg, "id") {
		t.Fatalf("error message = %q, want mention of id", msg)
	}
}

func TestCreateSnapshot(t *testing.T) {
	s := tempStore(t)
	bm := testBodyManager(t, s)
	ctx := context.Background()

	created, err := bm.Create(ctx, "snap-test", adapter.BodySpec{Image: "alpine"})
	if err != nil {
		t.Fatalf("create body: %v", err)
	}

	h := newHarness(t, s)
	h.srv.SetBodyManager(bm)
	defer h.close()

	h.send(t, Request{
		JSONRPC: "2.0",
		ID:      30,
		Method:  "tools/call",
		Params:  rawMessage(t, map[string]interface{}{"name": "create_snapshot", "arguments": map[string]interface{}{"body_id": created.ID, "label": "test-snap"}}),
	})

	resp := h.readResponse(t)
	result := resp["result"].(map[string]interface{})
	content := result["content"].([]interface{})
	text := content[0].(map[string]interface{})["text"].(string)

	var snapResp map[string]interface{}
	if err := json.Unmarshal([]byte(text), &snapResp); err != nil {
		t.Fatalf("unmarshal snapshot response: %v", err)
	}
	if snapResp["body_id"] != created.ID {
		t.Fatalf("body_id = %v, want %s", snapResp["body_id"], created.ID)
	}
	if snapResp["sha256"] == "" {
		t.Fatal("expected non-empty sha256")
	}
	if snapResp["size_bytes"].(float64) == 0 {
		t.Fatal("expected non-zero size_bytes")
	}
}

func TestCreateSnapshotBodyNotFound(t *testing.T) {
	s := tempStore(t)
	bm := testBodyManager(t, s)

	h := newHarness(t, s)
	h.srv.SetBodyManager(bm)
	defer h.close()

	h.send(t, Request{
		JSONRPC: "2.0",
		ID:      31,
		Method:  "tools/call",
		Params:  rawMessage(t, map[string]interface{}{"name": "create_snapshot", "arguments": map[string]interface{}{"body_id": "nonexistent"}}),
	})

	resp := h.readResponse(t)
	rpcErr := resp["error"].(map[string]interface{})
	msg := rpcErr["message"].(string)
	if !strings.Contains(msg, "not found") {
		t.Fatalf("error message = %q, want 'not found'", msg)
	}
}

func TestCreateSnapshotBodyNotRunning(t *testing.T) {
	s := tempStore(t)
	bm := testBodyManager(t, s)
	ctx := context.Background()

	created, err := bm.Create(ctx, "stopped-body", adapter.BodySpec{Image: "alpine"})
	if err != nil {
		t.Fatalf("create body: %v", err)
	}
	_ = bm.Stop(ctx, created.ID, adapter.StopOpts{})

	h := newHarness(t, s)
	h.srv.SetBodyManager(bm)
	defer h.close()

	h.send(t, Request{
		JSONRPC: "2.0",
		ID:      32,
		Method:  "tools/call",
		Params:  rawMessage(t, map[string]interface{}{"name": "create_snapshot", "arguments": map[string]interface{}{"body_id": created.ID}}),
	})

	resp := h.readResponse(t)
	rpcErr := resp["error"].(map[string]interface{})
	msg := rpcErr["message"].(string)
	if !strings.Contains(msg, "not running") {
		t.Fatalf("error message = %q, want 'not running'", msg)
	}
}

func TestCreateSnapshotNoBodyManager(t *testing.T) {
	s := tempStore(t)
	h := newHarness(t, s)
	defer h.close()

	h.send(t, Request{
		JSONRPC: "2.0",
		ID:      33,
		Method:  "tools/call",
		Params:  rawMessage(t, map[string]interface{}{"name": "create_snapshot", "arguments": map[string]interface{}{"body_id": "b1"}}),
	})

	resp := h.readResponse(t)
	rpcErr := resp["error"].(map[string]interface{})
	msg := rpcErr["message"].(string)
	if !strings.Contains(msg, "body manager not available") {
		t.Fatalf("error message = %q, want 'body manager not available'", msg)
	}
}

func TestListSnapshots(t *testing.T) {
	s := tempStore(t)
	ctx := context.Background()
	s.CreateBody(ctx, "b1", "body1", adapter.StateRunning, `{}`, "docker", "inst-1")
	s.CreateSnapshot(ctx, "snap1", "b1", `{"checksum":"abc"}`, "/tmp/snap1.tar.zst", 1024)
	s.CreateSnapshot(ctx, "snap2", "b1", `{"checksum":"def"}`, "/tmp/snap2.tar.zst", 2048)

	h := newHarness(t, s)
	defer h.close()

	h.send(t, Request{
		JSONRPC: "2.0",
		ID:      34,
		Method:  "tools/call",
		Params:  rawMessage(t, map[string]interface{}{"name": "list_snapshots", "arguments": map[string]interface{}{"body_id": "b1"}}),
	})

	resp := h.readResponse(t)
	result := resp["result"].(map[string]interface{})
	content := result["content"].([]interface{})
	text := content[0].(map[string]interface{})["text"].(string)

	var snaps []interface{}
	if err := json.Unmarshal([]byte(text), &snaps); err != nil {
		t.Fatalf("unmarshal snapshots: %v", err)
	}
	if len(snaps) != 2 {
		t.Fatalf("len(snaps) = %d, want 2", len(snaps))
	}
}

func TestListSnapshotsAllBodies(t *testing.T) {
	s := tempStore(t)
	ctx := context.Background()
	s.CreateBody(ctx, "b1", "body1", adapter.StateRunning, `{}`, "docker", "inst-1")
	s.CreateBody(ctx, "b2", "body2", adapter.StateRunning, `{}`, "docker", "inst-2")
	s.CreateSnapshot(ctx, "snap1", "b1", `{"checksum":"abc"}`, "/tmp/snap1.tar.zst", 1024)
	s.CreateSnapshot(ctx, "snap2", "b2", `{"checksum":"def"}`, "/tmp/snap2.tar.zst", 2048)

	h := newHarness(t, s)
	defer h.close()

	h.send(t, Request{
		JSONRPC: "2.0",
		ID:      35,
		Method:  "tools/call",
		Params:  rawMessage(t, map[string]interface{}{"name": "list_snapshots", "arguments": map[string]interface{}{}}),
	})

	resp := h.readResponse(t)
	result := resp["result"].(map[string]interface{})
	content := result["content"].([]interface{})
	text := content[0].(map[string]interface{})["text"].(string)

	var snaps []interface{}
	if err := json.Unmarshal([]byte(text), &snaps); err != nil {
		t.Fatalf("unmarshal snapshots: %v", err)
	}
	if len(snaps) != 2 {
		t.Fatalf("len(snaps) = %d, want 2", len(snaps))
	}
}

func TestRestoreBody(t *testing.T) {
	s := tempStore(t)
	bm := testBodyManager(t, s)
	ctx := context.Background()

	created, err := bm.Create(ctx, "restore-test", adapter.BodySpec{Image: "alpine"})
	if err != nil {
		t.Fatalf("create body: %v", err)
	}

	snapID := "restore-snap-1"
	storagePath := "/tmp/mesh-test-snap.tar.zst"

	tmpDir, err := os.MkdirTemp("", "mesh-restore-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)
	os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("hello world"), 0o644)

	outFile, err := os.Create(storagePath)
	if err != nil {
		t.Fatalf("create test snapshot file: %v", err)
	}

	hasher := sha256.New()
	mw := io.MultiWriter(outFile, hasher)
	zw, err := zstd.NewWriter(mw)
	if err != nil {
		t.Fatalf("create zstd writer: %v", err)
	}

	tw := tar.NewWriter(zw)
	info, _ := os.Stat(filepath.Join(tmpDir, "test.txt"))
	header, _ := tar.FileInfoHeader(info, "")
	header.Name = "test.txt"
	tw.WriteHeader(header)
	f, _ := os.Open(filepath.Join(tmpDir, "test.txt"))
	io.Copy(tw, f)
	f.Close()
	tw.Close()
	zw.Close()
	outFile.Close()

	digest := hex.EncodeToString(hasher.Sum(nil))
	os.WriteFile(storagePath+".sha256", []byte(digest+"\n"), 0o644)

	stat, _ := os.Stat(storagePath)
	manifestJSON := fmt.Sprintf(`{"checksum":"%s","size":%d}`, digest, stat.Size())
	s.CreateSnapshot(ctx, snapID, created.ID, manifestJSON, storagePath, stat.Size())

	h := newHarness(t, s)
	h.srv.SetBodyManager(bm)
	defer h.close()

	h.send(t, Request{
		JSONRPC: "2.0",
		ID:      36,
		Method:  "tools/call",
		Params:  rawMessage(t, map[string]interface{}{"name": "restore_body", "arguments": map[string]interface{}{"snapshot_id": snapID}}),
	})

	resp := h.readResponse(t)
	result := resp["result"].(map[string]interface{})
	contentResult := result["content"].([]interface{})
	text := contentResult[0].(map[string]interface{})["text"].(string)

	var restoreResp map[string]interface{}
	if err := json.Unmarshal([]byte(text), &restoreResp); err != nil {
		t.Fatalf("unmarshal restore response: %v", err)
	}
	if restoreResp["restored"] != true {
		t.Fatalf("restored = %v, want true", restoreResp["restored"])
	}
	if restoreResp["snapshot_id"] != snapID {
		t.Fatalf("snapshot_id = %v, want %s", restoreResp["snapshot_id"], snapID)
	}
	if restoreResp["body_id"] != created.ID {
		t.Fatalf("body_id = %v, want %s", restoreResp["body_id"], created.ID)
	}

	os.Remove(storagePath)
	os.Remove(storagePath + ".sha256")
}

func TestRestoreBodyNoBodyManager(t *testing.T) {
	s := tempStore(t)
	ctx := context.Background()
	s.CreateBody(ctx, "b1", "body1", adapter.StateRunning, `{}`, "docker", "inst-1")
	s.CreateSnapshot(ctx, "snap1", "b1", `{"checksum":"abc"}`, "/tmp/snap1.tar.zst", 1024)

	h := newHarness(t, s)
	defer h.close()

	h.send(t, Request{
		JSONRPC: "2.0",
		ID:      37,
		Method:  "tools/call",
		Params:  rawMessage(t, map[string]interface{}{"name": "restore_body", "arguments": map[string]interface{}{"snapshot_id": "snap1"}}),
	})

	resp := h.readResponse(t)
	rpcErr := resp["error"].(map[string]interface{})
	msg := rpcErr["message"].(string)
	if !strings.Contains(msg, "body manager not available") {
		t.Fatalf("error message = %q, want 'body manager not available'", msg)
	}
}

func TestRestoreBodySnapshotNotFound(t *testing.T) {
	s := tempStore(t)
	bm := testBodyManager(t, s)

	h := newHarness(t, s)
	h.srv.SetBodyManager(bm)
	defer h.close()

	h.send(t, Request{
		JSONRPC: "2.0",
		ID:      38,
		Method:  "tools/call",
		Params:  rawMessage(t, map[string]interface{}{"name": "restore_body", "arguments": map[string]interface{}{"snapshot_id": "nonexistent"}}),
	})

	resp := h.readResponse(t)
	rpcErr := resp["error"].(map[string]interface{})
	msg := rpcErr["message"].(string)
	if !strings.Contains(msg, "not found") {
		t.Fatalf("error message = %q, want 'not found'", msg)
	}
}

func TestGetBodyLogsRunningBody(t *testing.T) {
	s := tempStore(t)
	bm := testBodyManager(t, s)
	ctx := context.Background()

	created, err := bm.Create(ctx, "logs-test", adapter.BodySpec{Image: "alpine"})
	if err != nil {
		t.Fatalf("create body: %v", err)
	}

	h := newHarness(t, s)
	h.srv.SetBodyManager(bm)
	defer h.close()

	h.send(t, Request{
		JSONRPC: "2.0",
		ID:      50,
		Method:  "tools/call",
		Params: rawMessage(t, map[string]interface{}{
			"name":      "get_body_logs",
			"arguments": map[string]interface{}{"body_id": created.ID, "tail": 50},
		}),
	})

	resp := h.readResponse(t)
	result := resp["result"].(map[string]interface{})
	content := result["content"].([]interface{})
	text := content[0].(map[string]interface{})["text"].(string)

	var logsResp map[string]interface{}
	if err := json.Unmarshal([]byte(text), &logsResp); err != nil {
		t.Fatalf("unmarshal logs response: %v", err)
	}
	if logsResp["body_id"] != created.ID {
		t.Fatalf("body_id = %v, want %s", logsResp["body_id"], created.ID)
	}
	if logsResp["tail"].(float64) != 50 {
		t.Fatalf("tail = %v, want 50", logsResp["tail"])
	}
}

func TestGetBodyLogsStoppedBody(t *testing.T) {
	s := tempStore(t)
	bm := testBodyManager(t, s)
	ctx := context.Background()

	created, err := bm.Create(ctx, "logs-stopped", adapter.BodySpec{Image: "alpine"})
	if err != nil {
		t.Fatalf("create body: %v", err)
	}
	if err := bm.Stop(ctx, created.ID, adapter.StopOpts{}); err != nil {
		t.Fatalf("stop body: %v", err)
	}

	h := newHarness(t, s)
	h.srv.SetBodyManager(bm)
	defer h.close()

	h.send(t, Request{
		JSONRPC: "2.0",
		ID:      51,
		Method:  "tools/call",
		Params: rawMessage(t, map[string]interface{}{
			"name":      "get_body_logs",
			"arguments": map[string]interface{}{"body_id": created.ID},
		}),
	})

	resp := h.readResponse(t)
	rpcErr := resp["error"].(map[string]interface{})
	msg := rpcErr["message"].(string)
	if !strings.Contains(msg, "not running") {
		t.Fatalf("error message = %q, want 'not running'", msg)
	}
}

func TestGetBodyLogsBodyNotFound(t *testing.T) {
	s := tempStore(t)
	bm := testBodyManager(t, s)

	h := newHarness(t, s)
	h.srv.SetBodyManager(bm)
	defer h.close()

	h.send(t, Request{
		JSONRPC: "2.0",
		ID:      52,
		Method:  "tools/call",
		Params: rawMessage(t, map[string]interface{}{
			"name":      "get_body_logs",
			"arguments": map[string]interface{}{"body_id": "nonexistent"},
		}),
	})

	resp := h.readResponse(t)
	rpcErr := resp["error"].(map[string]interface{})
	msg := rpcErr["message"].(string)
	if !strings.Contains(msg, "not found") {
		t.Fatalf("error message = %q, want 'not found'", msg)
	}
}

func TestGetBodyLogsNoBodyManager(t *testing.T) {
	s := tempStore(t)
	h := newHarness(t, s)
	defer h.close()

	h.send(t, Request{
		JSONRPC: "2.0",
		ID:      53,
		Method:  "tools/call",
		Params: rawMessage(t, map[string]interface{}{
			"name":      "get_body_logs",
			"arguments": map[string]interface{}{"body_id": "b1"},
		}),
	})

	resp := h.readResponse(t)
	rpcErr := resp["error"].(map[string]interface{})
	msg := rpcErr["message"].(string)
	if !strings.Contains(msg, "body manager not available") {
		t.Fatalf("error message = %q, want 'body manager not available'", msg)
	}
}

func TestGetBodyStatusRunningBody(t *testing.T) {
	s := tempStore(t)
	bm := testBodyManager(t, s)
	ctx := context.Background()

	created, err := bm.Create(ctx, "status-test", adapter.BodySpec{Image: "alpine"})
	if err != nil {
		t.Fatalf("create body: %v", err)
	}

	h := newHarness(t, s)
	h.srv.SetBodyManager(bm)
	defer h.close()

	h.send(t, Request{
		JSONRPC: "2.0",
		ID:      54,
		Method:  "tools/call",
		Params: rawMessage(t, map[string]interface{}{
			"name":      "get_body_status",
			"arguments": map[string]interface{}{"body_id": created.ID},
		}),
	})

	resp := h.readResponse(t)
	result := resp["result"].(map[string]interface{})
	content := result["content"].([]interface{})
	text := content[0].(map[string]interface{})["text"].(string)

	var statusResp map[string]interface{}
	if err := json.Unmarshal([]byte(text), &statusResp); err != nil {
		t.Fatalf("unmarshal status response: %v", err)
	}
	if statusResp["id"] != created.ID {
		t.Fatalf("id = %v, want %s", statusResp["id"], created.ID)
	}
	if statusResp["name"] != "status-test" {
		t.Fatalf("name = %v, want status-test", statusResp["name"])
	}
	if statusResp["state"] != "Running" {
		t.Fatalf("state = %v, want Running", statusResp["state"])
	}
}

func TestGetBodyStatusBodyNotFound(t *testing.T) {
	s := tempStore(t)
	bm := testBodyManager(t, s)

	h := newHarness(t, s)
	h.srv.SetBodyManager(bm)
	defer h.close()

	h.send(t, Request{
		JSONRPC: "2.0",
		ID:      55,
		Method:  "tools/call",
		Params: rawMessage(t, map[string]interface{}{
			"name":      "get_body_status",
			"arguments": map[string]interface{}{"body_id": "nonexistent"},
		}),
	})

	resp := h.readResponse(t)
	rpcErr := resp["error"].(map[string]interface{})
	msg := rpcErr["message"].(string)
	if !strings.Contains(msg, "not found") {
		t.Fatalf("error message = %q, want 'not found'", msg)
	}
}

func TestGetBodyStatusNoBodyManager(t *testing.T) {
	s := tempStore(t)
	h := newHarness(t, s)
	defer h.close()

	h.send(t, Request{
		JSONRPC: "2.0",
		ID:      56,
		Method:  "tools/call",
		Params: rawMessage(t, map[string]interface{}{
			"name":      "get_body_status",
			"arguments": map[string]interface{}{"body_id": "b1"},
		}),
	})

	resp := h.readResponse(t)
	rpcErr := resp["error"].(map[string]interface{})
	msg := rpcErr["message"].(string)
	if !strings.Contains(msg, "body manager not available") {
		t.Fatalf("error message = %q, want 'body manager not available'", msg)
	}
}

func TestListPlugins(t *testing.T) {
	s := tempStore(t)
	pm := plugin.NewPluginManager(t.TempDir(), []string{})

	h := newHarness(t, s)
	h.srv.SetPluginManager(pm)
	defer h.close()

	h.send(t, Request{
		JSONRPC: "2.0",
		ID:      60,
		Method:  "tools/call",
		Params:  rawMessage(t, map[string]interface{}{"name": "list_plugins", "arguments": map[string]interface{}{}}),
	})

	resp := h.readResponse(t)
	result := resp["result"].(map[string]interface{})
	content := result["content"].([]interface{})
	text := content[0].(map[string]interface{})["text"].(string)

	var plugins []interface{}
	if err := json.Unmarshal([]byte(text), &plugins); err != nil {
		t.Fatalf("unmarshal plugins: %v", err)
	}
	if len(plugins) != 0 {
		t.Fatalf("len(plugins) = %d, want 0", len(plugins))
	}
}

func TestListPluginsNoManager(t *testing.T) {
	s := tempStore(t)
	h := newHarness(t, s)
	defer h.close()

	h.send(t, Request{
		JSONRPC: "2.0",
		ID:      61,
		Method:  "tools/call",
		Params:  rawMessage(t, map[string]interface{}{"name": "list_plugins", "arguments": map[string]interface{}{}}),
	})

	resp := h.readResponse(t)
	rpcErr := resp["error"].(map[string]interface{})
	msg := rpcErr["message"].(string)
	if !strings.Contains(msg, "plugin manager not available") {
		t.Fatalf("error message = %q, want 'plugin manager not available'", msg)
	}
}

func TestPluginHealth(t *testing.T) {
	s := tempStore(t)
	pluginDir := t.TempDir()
	pm := plugin.NewPluginManager(pluginDir, []string{"test-plugin"})

	h := newHarness(t, s)
	h.srv.SetPluginManager(pm)
	defer h.close()

	h.send(t, Request{
		JSONRPC: "2.0",
		ID:      62,
		Method:  "tools/call",
		Params:  rawMessage(t, map[string]interface{}{"name": "plugin_health", "arguments": map[string]interface{}{"plugin_name": "nonexistent"}}),
	})

	resp := h.readResponse(t)
	rpcErr := resp["error"].(map[string]interface{})
	msg := rpcErr["message"].(string)
	if !strings.Contains(msg, "not found") {
		t.Fatalf("error message = %q, want 'not found'", msg)
	}
}

func TestPluginHealthNoManager(t *testing.T) {
	s := tempStore(t)
	h := newHarness(t, s)
	defer h.close()

	h.send(t, Request{
		JSONRPC: "2.0",
		ID:      63,
		Method:  "tools/call",
		Params:  rawMessage(t, map[string]interface{}{"name": "plugin_health", "arguments": map[string]interface{}{"plugin_name": "test"}}),
	})

	resp := h.readResponse(t)
	rpcErr := resp["error"].(map[string]interface{})
	msg := rpcErr["message"].(string)
	if !strings.Contains(msg, "plugin manager not available") {
		t.Fatalf("error message = %q, want 'plugin manager not available'", msg)
	}
}

func TestPluginHealthMissingName(t *testing.T) {
	s := tempStore(t)
	pm := plugin.NewPluginManager(t.TempDir(), []string{})

	h := newHarness(t, s)
	h.srv.SetPluginManager(pm)
	defer h.close()

	h.send(t, Request{
		JSONRPC: "2.0",
		ID:      64,
		Method:  "tools/call",
		Params:  rawMessage(t, map[string]interface{}{"name": "plugin_health", "arguments": map[string]interface{}{}}),
	})

	resp := h.readResponse(t)
	rpcErr := resp["error"].(map[string]interface{})
	msg := rpcErr["message"].(string)
	if !strings.Contains(msg, "plugin_name") {
		t.Fatalf("error message = %q, want mention of plugin_name", msg)
	}
}

func TestToolsListIncludesPluginTools(t *testing.T) {
	s := tempStore(t)
	h := newHarness(t, s)
	defer h.close()

	h.send(t, Request{JSONRPC: "2.0", ID: 65, Method: "tools/list"})

	resp := h.readResponse(t)
	result := resp["result"].(map[string]interface{})
	tools := result["tools"].([]interface{})

	names := map[string]bool{}
	for _, tool := range tools {
		name := tool.(map[string]interface{})["name"].(string)
		names[name] = true
	}
	for _, want := range []string{"list_plugins", "plugin_health"} {
		if !names[want] {
			t.Errorf("missing tool %q in tools/list", want)
		}
	}
}

func rawMessage(t *testing.T, v interface{}) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return data
}
