package adapter

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"
)

// mockAdapter implements SubstrateAdapter for compile-time interface verification.
type mockAdapter struct{}

func (m *mockAdapter) Create(ctx context.Context, spec BodySpec) (Handle, error) {
	return Handle("test-body"), nil
}

func (m *mockAdapter) Start(ctx context.Context, id Handle) error {
	return nil
}

func (m *mockAdapter) Stop(ctx context.Context, id Handle, opts StopOpts) error {
	return nil
}

func (m *mockAdapter) Destroy(ctx context.Context, id Handle) error {
	return nil
}

func (m *mockAdapter) GetStatus(ctx context.Context, id Handle) (BodyStatus, error) {
	return BodyStatus{State: StateRunning}, nil
}

func (m *mockAdapter) Exec(ctx context.Context, id Handle, cmd []string) (ExecResult, error) {
	return ExecResult{ExitCode: 0}, nil
}

func (m *mockAdapter) ExportFilesystem(ctx context.Context, id Handle) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("")), nil
}

func (m *mockAdapter) ImportFilesystem(ctx context.Context, id Handle, tarball io.Reader, opts ImportOpts) error {
	return nil
}

func (m *mockAdapter) Inspect(ctx context.Context, id Handle) (ContainerMetadata, error) {
	return ContainerMetadata{}, nil
}

func (m *mockAdapter) Capabilities() AdapterCapabilities {
	return AdapterCapabilities{}
}

// Compile-time check that mockAdapter implements SubstrateAdapter.
var _ SubstrateAdapter = (*mockAdapter)(nil)

func TestInterfaceCompilation(t *testing.T) {
	_ = (*mockAdapter)(nil)
}

func TestBodyStateStrings(t *testing.T) {
	tests := []struct {
		state    BodyState
		expected string
	}{
		{StateCreated, "Created"},
		{StateStarting, "Starting"},
		{StateRunning, "Running"},
		{StateStopping, "Stopping"},
		{StateStopped, "Stopped"},
		{StateError, "Error"},
		{StateMigrating, "Migrating"},
		{StateDestroyed, "Destroyed"},
	}

	for _, tt := range tests {
		t.Run(string(tt.state), func(t *testing.T) {
			if tt.state != BodyState(tt.expected) {
				t.Fatalf("BodyState mismatch: got %q, want %q", tt.state, tt.expected)
			}
		})
	}
}

func TestBodySpecMarshaling(t *testing.T) {
	original := BodySpec{
		Image:     "nginx:latest",
		Workdir:   "/app",
		Env:       map[string]string{"PATH": "/usr/bin", "HOME": "/root"},
		Cmd:       []string{"nginx", "-g", "daemon off;"},
		MemoryMB:  512,
		CPUShares: 1024,
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var unmarshaled BodySpec
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if unmarshaled.Image != original.Image {
		t.Errorf("Image mismatch: got %q, want %q", unmarshaled.Image, original.Image)
	}
	if unmarshaled.Workdir != original.Workdir {
		t.Errorf("Workdir mismatch: got %q, want %q", unmarshaled.Workdir, original.Workdir)
	}
	if len(unmarshaled.Env) != len(original.Env) {
		t.Errorf("Env map size mismatch: got %d, want %d", len(unmarshaled.Env), len(original.Env))
	}
	for k, v := range original.Env {
		if unmarshaled.Env[k] != v {
			t.Errorf("Env[%q] mismatch: got %q, want %q", k, unmarshaled.Env[k], v)
		}
	}
	if len(unmarshaled.Cmd) != len(original.Cmd) {
		t.Fatalf("Cmd length mismatch: got %d, want %d", len(unmarshaled.Cmd), len(original.Cmd))
	}
	for i, cmd := range original.Cmd {
		if unmarshaled.Cmd[i] != cmd {
			t.Errorf("Cmd[%d] mismatch: got %q, want %q", i, unmarshaled.Cmd[i], cmd)
		}
	}
	if unmarshaled.MemoryMB != original.MemoryMB {
		t.Errorf("MemoryMB mismatch: got %d, want %d", unmarshaled.MemoryMB, original.MemoryMB)
	}
	if unmarshaled.CPUShares != original.CPUShares {
		t.Errorf("CPUShares mismatch: got %d, want %d", unmarshaled.CPUShares, original.CPUShares)
	}
}

func TestBodyStatusMarshaling(t *testing.T) {
	startTime := time.Now().Truncate(time.Millisecond)
	original := BodyStatus{
		State:      StateRunning,
		Uptime:     5 * time.Minute + 30 * time.Second,
		MemoryMB:   256,
		CPUPercent: 12.5,
		StartedAt:  startTime,
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var unmarshaled BodyStatus
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if unmarshaled.State != original.State {
		t.Errorf("State mismatch: got %q, want %q", unmarshaled.State, original.State)
	}
	if unmarshaled.Uptime != original.Uptime {
		t.Errorf("Uptime mismatch: got %v, want %v", unmarshaled.Uptime, original.Uptime)
	}
	if unmarshaled.MemoryMB != original.MemoryMB {
		t.Errorf("MemoryMB mismatch: got %d, want %d", unmarshaled.MemoryMB, original.MemoryMB)
	}
	if unmarshaled.CPUPercent != original.CPUPercent {
		t.Errorf("CPUPercent mismatch: got %f, want %f", unmarshaled.CPUPercent, original.CPUPercent)
	}
	if !unmarshaled.StartedAt.Equal(startTime) && unmarshaled.StartedAt.Sub(startTime) > time.Millisecond {
		t.Errorf("StartedAt mismatch: got %v, want %v (diff: %v)", unmarshaled.StartedAt, startTime, unmarshaled.StartedAt.Sub(startTime))
	}
}

func TestStopOptsMarshaling(t *testing.T) {
	original := StopOpts{
		Signal:  "SIGTERM",
		Timeout: 30 * time.Second,
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var unmarshaled StopOpts
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if unmarshaled.Signal != original.Signal {
		t.Errorf("Signal mismatch: got %q, want %q", unmarshaled.Signal, original.Signal)
	}
	if unmarshaled.Timeout != original.Timeout {
		t.Errorf("Timeout mismatch: got %v, want %v", unmarshaled.Timeout, original.Timeout)
	}
}

func TestExecResultMarshaling(t *testing.T) {
	original := ExecResult{
		Stdout:   "hello world",
		Stderr:   "error message",
		ExitCode: 1,
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var unmarshaled ExecResult
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if unmarshaled.Stdout != original.Stdout {
		t.Errorf("Stdout mismatch: got %q, want %q", unmarshaled.Stdout, original.Stdout)
	}
	if unmarshaled.Stderr != original.Stderr {
		t.Errorf("Stderr mismatch: got %q, want %q", unmarshaled.Stderr, original.Stderr)
	}
	if unmarshaled.ExitCode != original.ExitCode {
		t.Errorf("ExitCode mismatch: got %d, want %d", unmarshaled.ExitCode, original.ExitCode)
	}
}

func TestAdapterCapabilitiesDefaults(t *testing.T) {
	caps := AdapterCapabilities{}

	if caps.ExportFilesystem {
		t.Errorf("ExportFilesystem should default to false, got true")
	}
	if caps.ImportFilesystem {
		t.Errorf("ImportFilesystem should default to false, got true")
	}
	if caps.Inspect {
		t.Errorf("Inspect should default to false, got true")
	}
}

func TestAdapterCapabilitiesMarshaling(t *testing.T) {
	original := AdapterCapabilities{
		ExportFilesystem: true,
		ImportFilesystem: false,
		Inspect:          true,
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var unmarshaled AdapterCapabilities
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if unmarshaled.ExportFilesystem != original.ExportFilesystem {
		t.Errorf("ExportFilesystem mismatch: got %v, want %v", unmarshaled.ExportFilesystem, original.ExportFilesystem)
	}
	if unmarshaled.ImportFilesystem != original.ImportFilesystem {
		t.Errorf("ImportFilesystem mismatch: got %v, want %v", unmarshaled.ImportFilesystem, original.ImportFilesystem)
	}
	if unmarshaled.Inspect != original.Inspect {
		t.Errorf("Inspect mismatch: got %v, want %v", unmarshaled.Inspect, original.Inspect)
	}
}

func TestImportOptsMarshaling(t *testing.T) {
	original := ImportOpts{
		Overwrite: true,
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var unmarshaled ImportOpts
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if unmarshaled.Overwrite != original.Overwrite {
		t.Errorf("Overwrite mismatch: got %v, want %v", unmarshaled.Overwrite, original.Overwrite)
	}
}

func TestContainerMetadataMarshaling(t *testing.T) {
	original := ContainerMetadata{
		Image:    "redis:7",
		Env:      map[string]string{"REDIS_MODE": "standalone"},
		Cmd:      []string{"redis-server"},
		Workdir:  "/data",
		Platform: "linux/amd64",
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var unmarshaled ContainerMetadata
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if unmarshaled.Image != original.Image {
		t.Errorf("Image mismatch: got %q, want %q", unmarshaled.Image, original.Image)
	}
	if len(unmarshaled.Env) != len(original.Env) {
		t.Errorf("Env map size mismatch: got %d, want %d", len(unmarshaled.Env), len(original.Env))
	}
	for k, v := range original.Env {
		if unmarshaled.Env[k] != v {
			t.Errorf("Env[%q] mismatch: got %q, want %q", k, unmarshaled.Env[k], v)
		}
	}
	if len(unmarshaled.Cmd) != len(original.Cmd) {
		t.Fatalf("Cmd length mismatch: got %d, want %d", len(unmarshaled.Cmd), len(original.Cmd))
	}
	for i, cmd := range original.Cmd {
		if unmarshaled.Cmd[i] != cmd {
			t.Errorf("Cmd[%d] mismatch: got %q, want %q", i, unmarshaled.Cmd[i], cmd)
		}
	}
	if unmarshaled.Workdir != original.Workdir {
		t.Errorf("Workdir mismatch: got %q, want %q", unmarshaled.Workdir, original.Workdir)
	}
	if unmarshaled.Platform != original.Platform {
		t.Errorf("Platform mismatch: got %q, want %q", unmarshaled.Platform, original.Platform)
	}
}

func TestHandleString(t *testing.T) {
	h1 := Handle("my-body-123")
	h2 := Handle("another-body-456")

	s := string(h1)
	if s != "my-body-123" {
		t.Errorf("Handle string conversion failed: got %q, want %q", s, "my-body-123")
	}

	if h1 == h2 {
		t.Errorf("Different handles should not be equal")
	}

	h3 := Handle(s)
	if h3 != h1 {
		t.Errorf("Handle creation from string failed: got %q, want %q", h3, h1)
	}
}

func TestEmptyHandle(t *testing.T) {
	h := Handle("")
	if string(h) != "" {
		t.Errorf("Empty handle should convert to empty string")
	}

	_ = Handle("")
}

func TestBodySpecEmpty(t *testing.T) {
	original := BodySpec{}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var unmarshaled BodySpec
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if unmarshaled.Image != "" {
		t.Errorf("Image should be empty, got %q", unmarshaled.Image)
	}
	if unmarshaled.Workdir != "" {
		t.Errorf("Workdir should be empty, got %q", unmarshaled.Workdir)
	}
	if unmarshaled.Env != nil {
		t.Errorf("Env should be nil for empty map, got %v", unmarshaled.Env)
	}
	if unmarshaled.Cmd != nil {
		t.Errorf("Cmd should be nil for empty slice, got %v", unmarshaled.Cmd)
	}
}

func TestBodyStatusZeroValues(t *testing.T) {
	original := BodyStatus{
		State:      StateCreated,
		Uptime:     0,
		MemoryMB:   0,
		CPUPercent: 0.0,
		StartedAt:  time.Time{},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var unmarshaled BodyStatus
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if unmarshaled.State != StateCreated {
		t.Errorf("State mismatch: got %q, want %q", unmarshaled.State, StateCreated)
	}
	if unmarshaled.Uptime != 0 {
		t.Errorf("Uptime should be 0, got %v", unmarshaled.Uptime)
	}
	if unmarshaled.MemoryMB != 0 {
		t.Errorf("MemoryMB should be 0, got %d", unmarshaled.MemoryMB)
	}
	if unmarshaled.CPUPercent != 0.0 {
		t.Errorf("CPUPercent should be 0.0, got %f", unmarshaled.CPUPercent)
	}
	if !unmarshaled.StartedAt.IsZero() {
		t.Errorf("StartedAt should be zero, got %v", unmarshaled.StartedAt)
	}
}

func TestExecResultEmpty(t *testing.T) {
	original := ExecResult{
		Stdout:   "",
		Stderr:   "",
		ExitCode: 0,
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var unmarshaled ExecResult
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if unmarshaled.Stdout != "" {
		t.Errorf("Stdout should be empty, got %q", unmarshaled.Stdout)
	}
	if unmarshaled.Stderr != "" {
		t.Errorf("Stderr should be empty, got %q", unmarshaled.Stderr)
	}
	if unmarshaled.ExitCode != 0 {
		t.Errorf("ExitCode should be 0, got %d", unmarshaled.ExitCode)
	}
}

func TestExecResultWithMultilineOutput(t *testing.T) {
	original := ExecResult{
		Stdout: "line1\nline2\nline3",
		Stderr: "error\nwith\nnewlines",
		ExitCode: 127,
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var unmarshaled ExecResult
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if unmarshaled.Stdout != original.Stdout {
		t.Errorf("Stdout mismatch: got %q, want %q", unmarshaled.Stdout, original.Stdout)
	}
	if unmarshaled.Stderr != original.Stderr {
		t.Errorf("Stderr mismatch: got %q, want %q", unmarshaled.Stderr, original.Stderr)
	}
	if unmarshaled.ExitCode != original.ExitCode {
		t.Errorf("ExitCode mismatch: got %d, want %d", unmarshaled.ExitCode, original.ExitCode)
	}
}

func TestStopOptsZeroValues(t *testing.T) {
	original := StopOpts{
		Signal:  "",
		Timeout: 0,
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var unmarshaled StopOpts
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if unmarshaled.Signal != "" {
		t.Errorf("Signal should be empty, got %q", unmarshaled.Signal)
	}
	if unmarshaled.Timeout != 0 {
		t.Errorf("Timeout should be 0, got %v", unmarshaled.Timeout)
	}
}

func TestMockAdapterImplementation(t *testing.T) {
	m := &mockAdapter{}

	ctx := context.Background()

	h, err := m.Create(ctx, BodySpec{Image: "test"})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if h != Handle("test-body") {
		t.Errorf("Create returned unexpected handle: got %q, want %q", h, "test-body")
	}

	if err := m.Start(ctx, h); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	if err := m.Stop(ctx, h, StopOpts{}); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	status, err := m.GetStatus(ctx, h)
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}
	if status.State != StateRunning {
		t.Errorf("GetStatus returned unexpected state: got %q, want %q", status.State, StateRunning)
	}

	result, err := m.Exec(ctx, h, []string{"echo", "test"})
	if err != nil {
		t.Fatalf("Exec failed: %v", err)
	}
	if result.ExitCode != 0 {
		t.Errorf("Exec returned unexpected exit code: got %d, want 0", result.ExitCode)
	}

	rc, err := m.ExportFilesystem(ctx, h)
	if err != nil {
		t.Fatalf("ExportFilesystem failed: %v", err)
	}
	if rc == nil {
		t.Errorf("ExportFilesystem returned nil ReadCloser")
	}
	rc.Close()

	if err := m.ImportFilesystem(ctx, h, bytes.NewReader([]byte{}), ImportOpts{}); err != nil {
		t.Fatalf("ImportFilesystem failed: %v", err)
	}

	_, err = m.Inspect(ctx, h)
	if err != nil {
		t.Fatalf("Inspect failed: %v", err)
	}

	caps := m.Capabilities()
	if caps.ExportFilesystem {
		t.Errorf("Capabilities should have ExportFilesystem=false")
	}
}
