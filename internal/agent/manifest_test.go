package agent

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadManifest(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test-agent.yaml")
	content := `
name: test-agent
image: test-image:latest
command: ["/app/test"]
ports:
  - name: api
    container_port: 8080
    protocol: http
    expose: true
env:
  required:
    - API_KEY
  optional:
    - LOG_LEVEL
health_check:
  type: http
  path: /health
  port: "8080"
  interval_seconds: 5
resources:
  memory_mb: 256
  cpu_shares: 128
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write test manifest: %v", err)
	}

	m, err := LoadManifest(path)
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}

	if m.Name != "test-agent" {
		t.Errorf("Name = %q, want test-agent", m.Name)
	}
	if m.Image != "test-image:latest" {
		t.Errorf("Image = %q, want test-image:latest", m.Image)
	}
	if len(m.Command) != 1 || m.Command[0] != "/app/test" {
		t.Errorf("Command = %v, want [/app/test]", m.Command)
	}
	if len(m.Ports) != 1 {
		t.Fatalf("Ports len = %d, want 1", len(m.Ports))
	}
	if m.Ports[0].Name != "api" {
		t.Errorf("Port.Name = %q, want api", m.Ports[0].Name)
	}
	if m.Ports[0].ContainerPort != 8080 {
		t.Errorf("Port.ContainerPort = %d, want 8080", m.Ports[0].ContainerPort)
	}
	if m.Ports[0].Protocol != "http" {
		t.Errorf("Port.Protocol = %q, want http", m.Ports[0].Protocol)
	}
	if !m.Ports[0].Expose {
		t.Error("Port.Expose = false, want true")
	}
	if len(m.Env.Required) != 1 || m.Env.Required[0] != "API_KEY" {
		t.Errorf("Env.Required = %v, want [API_KEY]", m.Env.Required)
	}
	if len(m.Env.Optional) != 1 || m.Env.Optional[0] != "LOG_LEVEL" {
		t.Errorf("Env.Optional = %v, want [LOG_LEVEL]", m.Env.Optional)
	}
	if m.HealthCheck == nil {
		t.Fatal("HealthCheck is nil")
	}
	if m.HealthCheck.Type != "http" {
		t.Errorf("HealthCheck.Type = %q, want http", m.HealthCheck.Type)
	}
	if m.HealthCheck.Path != "/health" {
		t.Errorf("HealthCheck.Path = %q, want /health", m.HealthCheck.Path)
	}
	if m.HealthCheck.Port != "8080" {
		t.Errorf("HealthCheck.Port = %q, want 8080", m.HealthCheck.Port)
	}
	if m.HealthCheck.IntervalSeconds != 5 {
		t.Errorf("HealthCheck.IntervalSeconds = %d, want 5", m.HealthCheck.IntervalSeconds)
	}
	if m.Resources.MemoryMB != 256 {
		t.Errorf("Resources.MemoryMB = %d, want 256", m.Resources.MemoryMB)
	}
	if m.Resources.CPUShares != 128 {
		t.Errorf("Resources.CPUShares = %d, want 128", m.Resources.CPUShares)
	}
}

func TestLoadManifestMissingRequired(t *testing.T) {
	tmpDir := t.TempDir()

	// Missing name
	path1 := filepath.Join(tmpDir, "missing-name.yaml")
	if err := os.WriteFile(path1, []byte("image: test\n"), 0644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	if _, err := LoadManifest(path1); err == nil {
		t.Error("expected error for missing name, got nil")
	}

	// Missing image
	path2 := filepath.Join(tmpDir, "missing-image.yaml")
	if err := os.WriteFile(path2, []byte("name: test\n"), 0644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	if _, err := LoadManifest(path2); err == nil {
		t.Error("expected error for missing image, got nil")
	}
}

func TestLoadManifestDir(t *testing.T) {
	tmpDir := t.TempDir()

	content1 := "name: agent-a\nimage: img-a\n"
	content2 := "name: agent-b\nimage: img-b\n"
	content3 := "not a yaml manifest"

	if err := os.WriteFile(filepath.Join(tmpDir, "agent-a.yaml"), []byte(content1), 0644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "agent-b.yaml"), []byte(content2), 0644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "readme.txt"), []byte(content3), 0644); err != nil {
		t.Fatalf("write readme: %v", err)
	}

	manifests, err := LoadManifestDir(tmpDir)
	if err != nil {
		t.Fatalf("LoadManifestDir: %v", err)
	}

	if len(manifests) != 2 {
		t.Fatalf("len(manifests) = %d, want 2", len(manifests))
	}
	if manifests["agent-a"] == nil {
		t.Error("missing agent-a")
	}
	if manifests["agent-b"] == nil {
		t.Error("missing agent-b")
	}
	if manifests["agent-a"].Image != "img-a" {
		t.Errorf("agent-a.Image = %q, want img-a", manifests["agent-a"].Image)
	}
}

func TestValidateEnv(t *testing.T) {
	m := &AgentManifest{
		Name:  "test",
		Image: "test",
		Env: EnvConfig{
			Required: []string{"API_KEY", "SECRET"},
			Optional: []string{"DEBUG"},
		},
	}

	// All required provided
	err := ValidateEnv(m, map[string]string{"API_KEY": "val1", "SECRET": "val2"})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Missing required
	err = ValidateEnv(m, map[string]string{"API_KEY": "val1"})
	if err == nil {
		t.Error("expected error for missing SECRET, got nil")
	}

	// Empty provided
	err = ValidateEnv(m, map[string]string{})
	if err == nil {
		t.Error("expected error for missing env vars, got nil")
	}
}
