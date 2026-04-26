package manifest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRoundTrip(t *testing.T) {
	dir := t.TempDir()
	ts := time.Date(2026, 4, 23, 12, 30, 0, 0, time.UTC)

	original := &Manifest{
		AgentName:     "test-agent",
		Timestamp:     ts,
		SourceMachine: "host-01",
		SourceWorkdir: "/home/agent/work",
		StartCmd:      "/usr/bin/agent start",
		StopTimeout:   "30s",
		Checksum:      "sha256:abcdef1234567890",
		Size:          4096,
	}

	path := filepath.Join(dir, "agent-20260423-123000.json")
	if err := Write(path, original); err != nil {
		t.Fatalf("Write: %v", err)
	}

	got, err := Read(path)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	if got.AgentName != original.AgentName {
		t.Errorf("AgentName: got %q, want %q", got.AgentName, original.AgentName)
	}
	if !got.Timestamp.Equal(original.Timestamp) {
		t.Errorf("Timestamp: got %v, want %v", got.Timestamp, original.Timestamp)
	}
	if got.SourceMachine != original.SourceMachine {
		t.Errorf("SourceMachine: got %q, want %q", got.SourceMachine, original.SourceMachine)
	}
	if got.SourceWorkdir != original.SourceWorkdir {
		t.Errorf("SourceWorkdir: got %q, want %q", got.SourceWorkdir, original.SourceWorkdir)
	}
	if got.StartCmd != original.StartCmd {
		t.Errorf("StartCmd: got %q, want %q", got.StartCmd, original.StartCmd)
	}
	if got.StopTimeout != original.StopTimeout {
		t.Errorf("StopTimeout: got %q, want %q", got.StopTimeout, original.StopTimeout)
	}
	if got.Checksum != original.Checksum {
		t.Errorf("Checksum: got %q, want %q", got.Checksum, original.Checksum)
	}
	if got.Size != original.Size {
		t.Errorf("Size: got %d, want %d", got.Size, original.Size)
	}
}

func TestAllFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.json")

	m := &Manifest{
		AgentName:     "agent",
		Timestamp:     time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		SourceMachine: "machine",
		SourceWorkdir: "/work",
		StartCmd:      "run",
		StopTimeout:   "10s",
		Checksum:      "abc",
		Size:          100,
	}

	if err := Write(path, m); err != nil {
		t.Fatalf("Write: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}

	expectedKeys := []string{
		"agent_name", "timestamp", "source_machine",
		"source_workdir", "start_cmd", "stop_timeout",
		"checksum", "size",
	}

	for _, key := range expectedKeys {
		if !strings.Contains(string(data), `"`+key+`"`) {
			t.Errorf("JSON missing key %q", key)
		}
	}
}

func TestMalformedJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")

	if err := os.WriteFile(path, []byte("{not valid json}"), 0644); err != nil {
		t.Fatalf("write bad file: %v", err)
	}

	_, err := Read(path)
	if err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}
}

func TestManifestPath(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{
			input: "agent-20260423-120000.tar.zst",
			want:  "agent-20260423-120000.json",
		},
		{
			input: "/snapshots/my-agent-20260101-000000.tar.zst",
			want:  "/snapshots/my-agent-20260101-000000.json",
		},
		{
			input: "snapshot",
			want:  "snapshot.json",
		},
	}

	for _, tt := range tests {
		got := ManifestPath(tt.input)
		if got != tt.want {
			t.Errorf("ManifestPath(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestManifestPathNoExt(t *testing.T) {
	got := ManifestPath("some-file")
	want := "some-file.json"
	if got != want {
		t.Errorf("ManifestPath(%q) = %q, want %q", "some-file", got, want)
	}
}

func TestTimestampFormat(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ts-test.json")

	ts := time.Date(2026, 4, 23, 15, 30, 45, 0, time.UTC)
	m := &Manifest{Timestamp: ts}

	if err := Write(path, m); err != nil {
		t.Fatalf("Write: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal raw: %v", err)
	}

	tsStr, ok := raw["timestamp"].(string)
	if !ok {
		t.Fatal("timestamp is not a string")
	}

	parsed, err := time.Parse(time.RFC3339, tsStr)
	if err != nil {
		t.Fatalf("timestamp %q is not RFC3339: %v", tsStr, err)
	}

	if !parsed.Equal(ts) {
		t.Errorf("parsed timestamp %v != original %v", parsed, ts)
	}
}

func TestEmptyManifest(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.json")

	original := &Manifest{}

	if err := Write(path, original); err != nil {
		t.Fatalf("Write: %v", err)
	}

	got, err := Read(path)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	if got.AgentName != "" {
		t.Errorf("AgentName: got %q, want empty", got.AgentName)
	}
	if got.Size != 0 {
		t.Errorf("Size: got %d, want 0", got.Size)
	}
	if !got.Timestamp.IsZero() {
		t.Errorf("Timestamp: got %v, want zero", got.Timestamp)
	}
}

func TestManifestV2BackwardCompat(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "v1compat.json")

	v1JSON := `{
		"agent_name": "old-agent",
		"timestamp": "2026-04-23T12:00:00Z",
		"source_machine": "host-01",
		"source_workdir": "/work",
		"start_cmd": "run",
		"stop_timeout": "30s",
		"checksum": "abc123",
		"size": 4096
	}`

	if err := os.WriteFile(path, []byte(v1JSON), 0644); err != nil {
		t.Fatalf("write v1 json: %v", err)
	}

	got, err := Read(path)
	if err != nil {
		t.Fatalf("Read v1: %v", err)
	}

	if got.AgentName != "old-agent" {
		t.Errorf("AgentName: got %q, want %q", got.AgentName, "old-agent")
	}
	if got.Size != 4096 {
		t.Errorf("Size: got %d, want 4096", got.Size)
	}
	if v := ManifestVersion(got); v != 1 {
		t.Errorf("ManifestVersion: got %d, want 1", v)
	}
	if got.Image != "" {
		t.Errorf("Image should be empty for v1, got %q", got.Image)
	}
	if got.Env != nil {
		t.Errorf("Env should be nil for v1, got %v", got.Env)
	}
}

func TestManifestV2RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "v2.json")

	original := &Manifest{
		Version:       2,
		AgentName:     "body-agent",
		Timestamp:     time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC),
		SourceMachine: "substrate-01",
		SourceWorkdir: "/agents/body-agent",
		StartCmd:      "mesh start",
		StopTimeout:   "60s",
		Checksum:      "sha256:deadbeef",
		Size:          8192,
		Image:         "mesh-agent:latest",
		Platform:      "linux/amd64",
		Adapter:       "docker",
		Env:           map[string]string{"FOO": "bar", "BAZ": "qux"},
		Cmd:           []string{"/bin/sh", "-c", "run-agent"},
		BodyID:        "body-abc123",
	}

	if err := Write(path, original); err != nil {
		t.Fatalf("Write: %v", err)
	}

	got, err := Read(path)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	if got.Version != 2 {
		t.Errorf("Version: got %d, want 2", got.Version)
	}
	if got.Image != original.Image {
		t.Errorf("Image: got %q, want %q", got.Image, original.Image)
	}
	if got.Platform != original.Platform {
		t.Errorf("Platform: got %q, want %q", got.Platform, original.Platform)
	}
	if got.Adapter != original.Adapter {
		t.Errorf("Adapter: got %q, want %q", got.Adapter, original.Adapter)
	}
	if got.BodyID != original.BodyID {
		t.Errorf("BodyID: got %q, want %q", got.BodyID, original.BodyID)
	}
	if len(got.Env) != len(original.Env) {
		t.Errorf("Env length: got %d, want %d", len(got.Env), len(original.Env))
	}
	for k, v := range original.Env {
		if got.Env[k] != v {
			t.Errorf("Env[%q]: got %q, want %q", k, got.Env[k], v)
		}
	}
	if len(got.Cmd) != len(original.Cmd) {
		t.Errorf("Cmd length: got %d, want %d", len(got.Cmd), len(original.Cmd))
	}
	for i, v := range original.Cmd {
		if got.Cmd[i] != v {
			t.Errorf("Cmd[%d]: got %q, want %q", i, got.Cmd[i], v)
		}
	}
}

func TestManifestV2DefaultVersion(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "noversion.json")

	m := &Manifest{AgentName: "no-version"}
	if err := Write(path, m); err != nil {
		t.Fatalf("Write: %v", err)
	}

	got, err := Read(path)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	if v := ManifestVersion(got); v != 1 {
		t.Errorf("ManifestVersion with Version=0: got %d, want 1", v)
	}
}

func TestManifestV2ExplicitVersion(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "v2explicit.json")

	m := NewV2()
	m.AgentName = "explicit-v2"
	if err := Write(path, m); err != nil {
		t.Fatalf("Write: %v", err)
	}

	got, err := Read(path)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	if v := ManifestVersion(got); v != 2 {
		t.Errorf("ManifestVersion: got %d, want 2", v)
	}
}

func TestManifestV2FileSize(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "size.json")

	m := NewV2()
	m.Size = 12345678
	if err := Write(path, m); err != nil {
		t.Fatalf("Write: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}

	if !strings.Contains(string(data), `"size": 12345678`) {
		t.Errorf("JSON missing size field, got: %s", string(data))
	}

	got, err := Read(path)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if got.Size != 12345678 {
		t.Errorf("Size: got %d, want 12345678", got.Size)
	}
}

func TestManifestV2EmptyOptional(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty-opt.json")

	m := NewV2()
	m.AgentName = "lean"
	if err := Write(path, m); err != nil {
		t.Fatalf("Write: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}

	omitKeys := []string{"image", "platform", "adapter", "env", "cmd", "body_id"}
	for _, key := range omitKeys {
		if strings.Contains(string(data), `"`+key+`"`) {
			t.Errorf("JSON should omit empty field %q, found in output", key)
		}
	}

	got, err := Read(path)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if got.Image != "" {
		t.Errorf("Image: got %q, want empty", got.Image)
	}
	if got.Env != nil {
		t.Errorf("Env: got %v, want nil", got.Env)
	}
	if got.Cmd != nil {
		t.Errorf("Cmd: got %v, want nil", got.Cmd)
	}
}

func TestManifestV2AdapterField(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "adapter.json")

	original := &Manifest{
		Version:   2,
		Adapter:   "docker",
		Platform:  "linux/amd64",
		AgentName: "docker-agent",
	}

	if err := Write(path, original); err != nil {
		t.Fatalf("Write: %v", err)
	}

	got, err := Read(path)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	if got.Adapter != "docker" {
		t.Errorf("Adapter: got %q, want %q", got.Adapter, "docker")
	}
	if got.Platform != "linux/amd64" {
		t.Errorf("Platform: got %q, want %q", got.Platform, "linux/amd64")
	}
	if v := ManifestVersion(got); v != 2 {
		t.Errorf("ManifestVersion: got %d, want 2", v)
	}
}

func TestWriteCreatesParentDirs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "deep", "manifest.json")

	if err := Write(path, &Manifest{AgentName: "test"}); err != nil {
		t.Fatalf("Write with nested dirs: %v", err)
	}

	got, err := Read(path)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	if got.AgentName != "test" {
		t.Errorf("AgentName: got %q, want %q", got.AgentName, "test")
	}
}
