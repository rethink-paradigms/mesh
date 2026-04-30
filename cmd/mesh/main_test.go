package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/rethink-paradigms/mesh/internal/manifest"
)

func TestParseTimestampFromFilename(t *testing.T) {
	tests := []struct {
		filename string
		agent    string
		want     string
		wantErr  bool
	}{
		{"myagent-20260424-153000.tar.zst", "myagent", "2026-04-24 15:30:00", false},
		{"myagent-20260101-000000.tar.zst", "myagent", "2026-01-01 00:00:00", false},
		{"other-20260424-120000.tar.zst", "myagent", "", true},
		{"myagent-badformat.tar.zst", "myagent", "", true},
		{"myagent-12345678-123456.tar.zst", "myagent", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			got, err := parseTimestampFromFilename(tt.filename, tt.agent)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestHumanSize(t *testing.T) {
	tests := []struct {
		bytes int64
		want  string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{1572864, "1.5 MB"},
		{1073741824, "1.0 GB"},
		{1610612736, "1.5 GB"},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%d", tt.bytes), func(t *testing.T) {
			got := humanSize(tt.bytes)
			if got != tt.want {
				t.Errorf("humanSize(%d) = %q, want %q", tt.bytes, got, tt.want)
			}
		})
	}
}

func TestFindLatestInDir(t *testing.T) {
	tmpDir := t.TempDir()
	agentName := "testagent"

	cacheDir := filepath.Join(tmpDir, agentName)
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatal(err)
	}

	t.Run("empty dir returns error", func(t *testing.T) {
		_, err := findLatestInDir(cacheDir, agentName)
		if err == nil {
			t.Fatal("expected error for empty dir")
		}
	})

	t.Run("returns latest snapshot", func(t *testing.T) {
		mustCreateFile(t, filepath.Join(cacheDir, agentName+"-20260424-100000.tar.zst"), 100)
		mustCreateFile(t, filepath.Join(cacheDir, agentName+"-20260424-150000.tar.zst"), 200)
		mustCreateFile(t, filepath.Join(cacheDir, agentName+"-20260424-120000.tar.zst"), 150)

		got, err := findLatestInDir(cacheDir, agentName)
		if err != nil {
			t.Fatal(err)
		}
		want := filepath.Join(cacheDir, agentName+"-20260424-150000.tar.zst")
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})
}

func TestListCommand(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	agentName := "listagent"
	snapDir := filepath.Join(tmpHome, ".mesh", "snapshots", agentName)
	mustMkdirAll(t, snapDir)

	snap1 := filepath.Join(snapDir, agentName+"-20260424-100000.tar.zst")
	snap2 := filepath.Join(snapDir, agentName+"-20260424-150000.tar.zst")
	mustCreateFile(t, snap1, 1024)
	mustCreateFile(t, snap2, 2048)

	ts := time.Date(2026, 4, 24, 10, 0, 0, 0, time.UTC)
	m1 := &manifest.Manifest{
		AgentName:     agentName,
		Timestamp:     ts,
		SourceMachine: "myserver",
		SourceWorkdir: "/opt/agent",
		StartCmd:      "./start.sh",
		StopTimeout:   "30s",
		Checksum:      "abc123",
		Size:          1024,
	}
	mustWriteManifest(t, manifest.ManifestPath(snap1), m1)

	ts2 := time.Date(2026, 4, 24, 15, 0, 0, 0, time.UTC)
	m2 := &manifest.Manifest{
		AgentName:     agentName,
		Timestamp:     ts2,
		SourceMachine: "",
		SourceWorkdir: "/opt/agent",
		StartCmd:      "./start.sh",
		StopTimeout:   "30s",
		Checksum:      "def456",
		Size:          2048,
	}
	mustWriteManifest(t, manifest.ManifestPath(snap2), m2)

	t.Run("list specific agent", func(t *testing.T) {
		cmd := newListCmd()
		var stdout bytes.Buffer
		cmd.SetOut(&stdout)
		cmd.SetArgs([]string{agentName})

		if err := cmd.Execute(); err != nil {
			t.Fatalf("list command failed: %v", err)
		}

		output := stdout.String()
		mustContain(t, output, "listagent-20260424-100000.tar.zst")
		mustContain(t, output, "listagent-20260424-150000.tar.zst")
		mustContain(t, output, "1.0 KB")
		mustContain(t, output, "2.0 KB")
		mustContain(t, output, "from myserver")
		mustContain(t, output, "2026-04-24 10:00:00")
		mustContain(t, output, "2026-04-24 15:00:00")
	})

	t.Run("list all agents", func(t *testing.T) {
		cmd := newListCmd()
		var stdout bytes.Buffer
		cmd.SetOut(&stdout)
		cmd.SetArgs([]string{})

		if err := cmd.Execute(); err != nil {
			t.Fatalf("list command failed: %v", err)
		}

		output := stdout.String()
		mustContain(t, output, "listagent-20260424-100000.tar.zst")
		mustContain(t, output, "listagent-20260424-150000.tar.zst")
	})
}

func TestListCommandEmpty(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	cmd := newListCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("list command failed: %v", err)
	}

	if stdout.String() != "" {
		t.Errorf("expected empty output for no snapshots, got %q", stdout.String())
	}
}

func TestInspectCommand(t *testing.T) {
	tmpDir := t.TempDir()

	snapPath := filepath.Join(tmpDir, "testagent-20260424-153000.tar.zst")
	mustCreateFile(t, snapPath, 2048)

	ts := time.Date(2026, 4, 24, 15, 30, 0, 0, time.UTC)
	m := &manifest.Manifest{
		AgentName:     "testagent",
		Timestamp:     ts,
		SourceMachine: "prod-server",
		SourceWorkdir: "/opt/agent",
		StartCmd:      "./run.sh",
		StopTimeout:   "30s",
		Checksum:      "sha256abc123",
		Size:          2048,
	}
	mustWriteManifest(t, manifest.ManifestPath(snapPath), m)

	cmd := newInspectCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{snapPath})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("inspect command failed: %v", err)
	}

	output := stdout.String()
	mustContain(t, output, "Agent: testagent")
	mustContain(t, output, "Timestamp: 2026-04-24T15:30:00Z")
	mustContain(t, output, "Source machine: prod-server")
	mustContain(t, output, "Source workdir: /opt/agent")
	mustContain(t, output, "Start cmd: ./run.sh")
	mustContain(t, output, "Stop timeout: 30s")
	mustContain(t, output, "Checksum: sha256abc123")
	mustContain(t, output, "Size: 2.0 KB")

	if strings.Contains(output, "{") || strings.Contains(output, "}") {
		t.Error("output should be plain text, not JSON")
	}
}

func TestInspectCommandWithAgentName(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	agentName := "inspectagent"
	snapDir := filepath.Join(tmpHome, ".mesh", "snapshots", agentName)
	mustMkdirAll(t, snapDir)

	snapPath := filepath.Join(snapDir, agentName+"-20260424-153000.tar.zst")
	mustCreateFile(t, snapPath, 1024)

	ts := time.Date(2026, 4, 24, 15, 30, 0, 0, time.UTC)
	m := &manifest.Manifest{
		AgentName:     agentName,
		Timestamp:     ts,
		SourceMachine: "local",
		SourceWorkdir: "/tmp/agent",
		StartCmd:      "node app.js",
		StopTimeout:   "10s",
		Checksum:      "deadbeef",
		Size:          1024,
	}
	mustWriteManifest(t, manifest.ManifestPath(snapPath), m)

	cmd := newInspectCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{agentName})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("inspect command with agent name failed: %v", err)
	}

	output := stdout.String()
	mustContain(t, output, "Agent: inspectagent")
	mustContain(t, output, "Source machine: local")
}

func TestPruneCommand(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	agentName := "pruneagent"
	snapDir := filepath.Join(tmpHome, ".mesh", "snapshots", agentName)
	mustMkdirAll(t, snapDir)

	for i, ts := range []string{"090000", "100000", "110000", "120000", "130000"} {
		snapPath := filepath.Join(snapDir, fmt.Sprintf("%s-20260424-%s.tar.zst", agentName, ts))
		mustCreateFile(t, snapPath, int64(100*(i+1)))
		os.WriteFile(snapPath+".sha256", []byte(fmt.Sprintf("hash%d\n", i)), 0o644)
		mustWriteManifest(t, manifest.ManifestPath(snapPath), &manifest.Manifest{
			AgentName: agentName,
			Checksum:  fmt.Sprintf("hash%d", i),
		})
	}

	t.Run("prune keeps N newest", func(t *testing.T) {
		cmd := newPruneCmd()
		var stdout bytes.Buffer
		cmd.SetOut(&stdout)
		cmd.SetArgs([]string{agentName, "--keep", "2"})

		if err := cmd.Execute(); err != nil {
			t.Fatalf("prune command failed: %v", err)
		}

		output := stdout.String()
		mustContain(t, output, "Pruned 3 snapshot(s)")
		mustContain(t, output, "kept 2")

		entries, _ := os.ReadDir(snapDir)
		var remaining []string
		for _, e := range entries {
			if strings.HasSuffix(e.Name(), ".tar.zst") {
				remaining = append(remaining, e.Name())
			}
		}
		if len(remaining) != 2 {
			t.Errorf("expected 2 remaining snapshots, got %d: %v", len(remaining), remaining)
		}

		mustContain(t, remaining[0], "120000")
		mustContain(t, remaining[1], "130000")
	})

	t.Run("sidecar files also cleaned up", func(t *testing.T) {
		for _, ext := range []string{".sha256", ".json"} {
			if _, err := os.Stat(filepath.Join(snapDir, agentName+"-20260424-090000.tar.zst"+ext)); err == nil {
				t.Errorf("sidecar %s should have been deleted", ext)
			}
		}
	})
}

func TestPruneCommandNothingToPrune(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	agentName := "nopruneneeded"
	snapDir := filepath.Join(tmpHome, ".mesh", "snapshots", agentName)
	mustMkdirAll(t, snapDir)

	mustCreateFile(t, filepath.Join(snapDir, agentName+"-20260424-100000.tar.zst"), 100)

	cmd := newPruneCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{agentName, "--keep", "5"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("prune command failed: %v", err)
	}

	output := stdout.String()
	mustContain(t, output, "nothing to prune")
}

func TestPruneCommandNoSnapshots(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	agentName := "nosnaps"

	cmd := newPruneCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{agentName})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("prune command failed: %v", err)
	}

	output := stdout.String()
	mustContain(t, output, "No snapshots")
}

func mustCreateFile(t *testing.T, path string, size int64) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if size > 0 {
		if _, err := f.Write(make([]byte, size)); err != nil {
			f.Close()
			t.Fatal(err)
		}
	}
	f.Close()
}

func mustMkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func mustWriteManifest(t *testing.T, path string, m *manifest.Manifest) {
	t.Helper()
	if err := manifest.Write(path, m); err != nil {
		t.Fatal(err)
	}
}

func mustWriteConfig(t *testing.T, tmpHome, agentName, workdir string) string {
	t.Helper()
	cfgDir := filepath.Join(tmpHome, ".mesh")
	mustMkdirAll(t, cfgDir)
	cfgContent := fmt.Sprintf(
		`[[agents]]
name = "%s"
workdir = "%s"
start_cmd = "echo hello"
`, agentName, workdir)
	cfgPath := filepath.Join(cfgDir, "config.toml")
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0o644); err != nil {
		t.Fatal(err)
	}
	return cfgPath
}

func mustWriteYAMLConfig(t *testing.T, tmpHome string, bodies []string) string {
	t.Helper()
	cfgDir := filepath.Join(tmpHome, ".mesh")
	mustMkdirAll(t, cfgDir)
	mustMkdirAll(t, filepath.Join(cfgDir, "plugins"))

	var bodyLines string
	for _, name := range bodies {
		bodyLines += fmt.Sprintf("- name: %s\n  image: alpine:latest\n  workdir: /tmp/%s\n", name, name)
	}

	cfgContent := fmt.Sprintf("daemon:\n  socket_path: /tmp/mesh-test.sock\n  pid_file: %s\n  log_level: info\nstore:\n  path: %s\ndocker:\n  host: unix:///var/run/docker.sock\n  api_version: \"1.48\"\nregistry:\n  type: s3\n  bucket: test-bucket\nbodies:\n%s",
		filepath.Join(cfgDir, "mesh.pid"),
		filepath.Join(cfgDir, "state.db"),
		bodyLines,
	)

	cfgPath := filepath.Join(cfgDir, "config.yaml")
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0o644); err != nil {
		t.Fatal(err)
	}
	return cfgPath
}

func mustContain(t *testing.T, output, substr string) {
	t.Helper()
	if !strings.Contains(output, substr) {
		t.Errorf("output does not contain %q\nfull output:\n%s", substr, output)
	}
}

func TestServeCommandStartsDaemon(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	cfgPath := mustWriteYAMLConfig(t, tmpHome, []string{"test-agent"})

	root := newRootCmd()
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"serve", "--config", cfgPath})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	root.SetContext(ctx)

	done := make(chan error, 1)
	go func() {
		done <- root.Execute()
	}()

	pidFile := filepath.Join(tmpHome, ".mesh", "mesh.pid")
	var pid int
	for i := 0; i < 50; i++ {
		data, err := os.ReadFile(pidFile)
		if err == nil {
			pid, _ = strconv.Atoi(strings.TrimSpace(string(data)))
			if pid > 0 {
				break
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	if pid == 0 {
		t.Fatal("daemon did not write PID file")
	}

	cancel()

	select {
	case err := <-done:
		if pid == 0 {
			t.Fatalf("daemon did not write PID file (serve returned: %v)", err)
		}
		if err != nil {
			t.Fatalf("serve command failed: %v", err)
		}
	case <-time.After(5 * time.Second):
		if pid == 0 {
			t.Fatal("daemon did not write PID file within timeout")
		}
		t.Fatal("serve command did not return within timeout")
	}

	if _, err := os.Stat(pidFile); !os.IsNotExist(err) {
		t.Fatal("PID file should be removed after stop")
	}
}

func TestStopCommandKillsDaemon(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	cfgPath := mustWriteYAMLConfig(t, tmpHome, []string{})

	fakeDaemon := exec.Command("sleep", "30")
	if err := fakeDaemon.Start(); err != nil {
		t.Fatalf("start fake daemon: %v", err)
	}
	defer fakeDaemon.Process.Kill()

	pidFile := filepath.Join(tmpHome, ".mesh", "mesh.pid")
	if err := os.WriteFile(pidFile, []byte(fmt.Sprintf("%d", fakeDaemon.Process.Pid)), 0o644); err != nil {
		t.Fatalf("write PID file: %v", err)
	}

	root := newRootCmd()
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"stop", "--config", cfgPath, "--timeout", "500ms"})

	if err := root.Execute(); err != nil {
		t.Fatalf("stop command failed: %v", err)
	}

	output := stdout.String()
	mustContain(t, output, "Stopping mesh daemon")

	done := make(chan error, 1)
	go func() {
		done <- fakeDaemon.Wait()
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		fakeDaemon.Process.Kill()
		t.Fatal("daemon process should have exited after stop")
	}
}

func TestStopCommandNotRunning(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	cfgPath := mustWriteYAMLConfig(t, tmpHome, []string{})

	root := newRootCmd()
	var stderr bytes.Buffer
	root.SetErr(&stderr)
	root.SetArgs([]string{"stop", "--config", cfgPath})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error when daemon is not running")
	}
	if !strings.Contains(err.Error(), "not running") {
		t.Fatalf("error = %q, want 'not running'", err.Error())
	}
}

func TestStatusCommandRunning(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	cfgPath := mustWriteYAMLConfig(t, tmpHome, []string{})

	fakeDaemon := exec.Command("sleep", "30")
	if err := fakeDaemon.Start(); err != nil {
		t.Fatalf("start fake daemon: %v", err)
	}
	defer fakeDaemon.Process.Kill()

	pidFile := filepath.Join(tmpHome, ".mesh", "mesh.pid")
	if err := os.WriteFile(pidFile, []byte(fmt.Sprintf("%d", fakeDaemon.Process.Pid)), 0o644); err != nil {
		t.Fatalf("write PID file: %v", err)
	}

	root := newRootCmd()
	var stdout bytes.Buffer
	root.SetOut(&stdout)
	root.SetArgs([]string{"status", "--config", cfgPath})

	if err := root.Execute(); err != nil {
		t.Fatalf("status command failed: %v", err)
	}

	output := stdout.String()
	mustContain(t, output, "Mesh daemon: running")
	mustContain(t, output, fmt.Sprintf("(pid %d)", fakeDaemon.Process.Pid))
}

func TestStatusCommandStopped(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	cfgPath := mustWriteYAMLConfig(t, tmpHome, []string{})

	root := newRootCmd()
	var stdout bytes.Buffer
	root.SetOut(&stdout)
	root.SetArgs([]string{"status", "--config", cfgPath})

	if err := root.Execute(); err != nil {
		t.Fatalf("status command failed: %v", err)
	}

	output := stdout.String()
	mustContain(t, output, "Mesh daemon: stopped")
}
