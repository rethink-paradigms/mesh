package daemon

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDaemonInstallerWithManifests(t *testing.T) {
	cfg := testConfig(t)

	agentsDir := t.TempDir()
	manifestContent := `
name: "test-agent"
image: "test-image:latest"
command: ["sleep", "infinity"]
ports:
  - name: "http"
    container_port: 8080
    protocol: "tcp"
    expose: true
env:
  required:
    - "API_KEY"
  optional:
    - "LOG_LEVEL"
resources:
  memory_mb: 256
  cpu_shares: 512
`
	if err := os.WriteFile(filepath.Join(agentsDir, "test-agent.yaml"), []byte(manifestContent), 0644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	cfg.AgentsDir = agentsDir

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

	if d.installer == nil {
		cancel()
		t.Fatal("installer should be initialized when manifests are present")
	}

	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Start did not return within timeout")
	}
}

func TestDaemonInstallerNoManifests(t *testing.T) {
	cfg := testConfig(t)

	agentsDir := t.TempDir() // empty directory — no manifest files
	cfg.AgentsDir = agentsDir

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

	if d.installer != nil {
		cancel()
		t.Fatal("installer should be nil when no manifests are present")
	}

	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Start did not return within timeout")
	}
}

func TestDaemonInstallerNonexistentDir(t *testing.T) {
	cfg := testConfig(t)

	cfg.AgentsDir = "/nonexistent/mesh/agents"

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

	if d.installer != nil {
		cancel()
		t.Fatal("installer should be nil when agents dir does not exist")
	}

	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Start did not return within timeout")
	}
}

func TestDaemonInstallerNotConfigured(t *testing.T) {
	cfg := testConfig(t)
	// cfg.AgentsDir is left as zero-value "" (not configured)

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

	if d.installer != nil {
		cancel()
		t.Fatal("installer should be nil when agents_dir is empty")
	}

	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Start did not return within timeout")
	}
}
