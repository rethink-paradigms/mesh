package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/rethink-paradigms/mesh/internal/adapter"
	"github.com/rethink-paradigms/mesh/internal/body"
	"github.com/rethink-paradigms/mesh/internal/store"
)

type mockSubstrateAdapter struct {
	handle adapter.Handle
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
	return adapter.BodyStatus{State: adapter.StateRunning}, nil
}
func (m *mockSubstrateAdapter) Exec(_ context.Context, _ adapter.Handle, _ []string) (adapter.ExecResult, error) {
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
	return body.NewMigrationCoordinator(s, &mockSubstrateAdapter{}, bm)
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

func TestExecCommandNotImplemented(t *testing.T) {
	s := tempStore(t)
	h := newHarness(t, s)
	defer h.close()

	h.send(t, Request{
		JSONRPC: "2.0",
		ID:      7,
		Method:  "tools/call",
		Params:  rawMessage(t, map[string]interface{}{
			"name":      "execute_command",
			"arguments": map[string]interface{}{"body_id": "b1", "command": []string{"ls"}},
		}),
	})

	resp := h.readResponse(t)
	rpcErr := resp["error"].(map[string]interface{})
	msg := rpcErr["message"].(string)
	if !strings.Contains(msg, "not yet implemented") {
		t.Fatalf("error message = %q, want 'not yet implemented'", msg)
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

func rawMessage(t *testing.T, v interface{}) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return data
}
