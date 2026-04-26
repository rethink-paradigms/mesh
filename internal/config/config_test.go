// Package config tests YAML configuration parsing and validation.
package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// writeConfig is a test helper that writes a YAML config to a temp file.
func writeConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return p
}

// TestLoadValidConfig loads a valid YAML config with all sections.
func TestLoadValidConfig(t *testing.T) {
	content := `
daemon:
  socket_path: /tmp/mesh.sock
  pid_file: /tmp/mesh.pid
  log_level: debug
store:
  path: /tmp/mesh.db
docker:
  host: unix:///var/run/docker.sock
  api_version: "1.48"
bodies:
  - name: agent1
    image: alpine:latest
    workdir: /home/agent
    env:
      FOO: bar
    cmd: ["sleep", "infinity"]
    memory_mb: 256
    cpu_shares: 512
`
	path := writeConfig(t, content)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Verify daemon config
	if cfg.Daemon.SocketPath != "/tmp/mesh.sock" {
		t.Errorf("SocketPath = %q, want %q", cfg.Daemon.SocketPath, "/tmp/mesh.sock")
	}
	if cfg.Daemon.PIDFile != "/tmp/mesh.pid" {
		t.Errorf("PIDFile = %q, want %q", cfg.Daemon.PIDFile, "/tmp/mesh.pid")
	}
	if cfg.Daemon.LogLevel != "debug" {
		t.Errorf("LogLevel = %q, want %q", cfg.Daemon.LogLevel, "debug")
	}

	// Verify store config
	if cfg.Store.Path != "/tmp/mesh.db" {
		t.Errorf("Store.Path = %q, want %q", cfg.Store.Path, "/tmp/mesh.db")
	}

	// Verify docker config
	if cfg.Docker.Host != "unix:///var/run/docker.sock" {
		t.Errorf("Docker.Host = %q, want %q", cfg.Docker.Host, "unix:///var/run/docker.sock")
	}
	if cfg.Docker.APIVersion != "1.48" {
		t.Errorf("Docker.APIVersion = %q, want %q", cfg.Docker.APIVersion, "1.48")
	}

	// Verify bodies
	if len(cfg.Bodies) != 1 {
		t.Fatalf("len(Bodies) = %d, want 1", len(cfg.Bodies))
	}
	body := cfg.Bodies[0]
	if body.Name != "agent1" {
		t.Errorf("Body.Name = %q, want %q", body.Name, "agent1")
	}
	if body.Image != "alpine:latest" {
		t.Errorf("Body.Image = %q, want %q", body.Image, "alpine:latest")
	}
	if body.Workdir != "/home/agent" {
		t.Errorf("Body.Workdir = %q, want %q", body.Workdir, "/home/agent")
	}
	if body.Env == nil {
		t.Fatal("Body.Env is nil")
	}
	if body.Env["FOO"] != "bar" {
		t.Errorf("Body.Env[\"FOO\"] = %q, want %q", body.Env["FOO"], "bar")
	}
	if len(body.Cmd) != 2 || body.Cmd[0] != "sleep" || body.Cmd[1] != "infinity" {
		t.Errorf("Body.Cmd = %v, want [\"sleep\", \"infinity\"]", body.Cmd)
	}
	if body.MemoryMB != 256 {
		t.Errorf("Body.MemoryMB = %d, want 256", body.MemoryMB)
	}
	if body.CPUShares != 512 {
		t.Errorf("Body.CPUShares = %d, want 512", body.CPUShares)
	}
}

// TestLoadDefaults loads a minimal YAML (only bodies with name+image), verifies defaults applied.
func TestLoadDefaults(t *testing.T) {
	content := `
bodies:
  - name: agent1
    image: alpine:latest
`
	path := writeConfig(t, content)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Verify daemon defaults
	if cfg.Daemon.SocketPath != "/tmp/mesh.sock" {
		t.Errorf("SocketPath = %q, want %q", cfg.Daemon.SocketPath, "/tmp/mesh.sock")
	}
	if cfg.Daemon.LogLevel != "info" {
		t.Errorf("LogLevel = %q, want %q", cfg.Daemon.LogLevel, "info")
	}

	// Verify PIDFile has default (home-dependent, so just check it's set)
	if cfg.Daemon.PIDFile == "" {
		t.Error("PIDFile is empty, want default")
	}

	// Verify store path default
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir() error = %v", err)
	}
	wantStorePath := filepath.Join(home, ".mesh", "state.db")
	if cfg.Store.Path != wantStorePath {
		t.Errorf("Store.Path = %q, want %q", cfg.Store.Path, wantStorePath)
	}

	// Verify docker defaults
	if cfg.Docker.Host != "unix:///var/run/docker.sock" {
		t.Errorf("Docker.Host = %q, want %q", cfg.Docker.Host, "unix:///var/run/docker.sock")
	}
	if cfg.Docker.APIVersion != "1.48" {
		t.Errorf("Docker.APIVersion = %q, want %q", cfg.Docker.APIVersion, "1.48")
	}
}

// TestLoadMissingRequiredFields verifies errors for missing required fields.
func TestLoadMissingRequiredFields(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantErr string
	}{
		{
			name: "empty body name",
			content: `
bodies:
  - name: ""
    image: alpine:latest
`,
			wantErr: "empty name",
		},
		{
			name: "empty body image",
			content: `
bodies:
  - name: agent1
    image: ""
`,
			wantErr: "image must not be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeConfig(t, tt.content)
			_, err := Load(path)
			if err == nil {
				t.Fatalf("Load() expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("Load() error = %v, want error containing %q", err, tt.wantErr)
			}
		})
	}
}

// TestLoadInvalidYAML loads invalid YAML (malformed), verifies error.
func TestLoadInvalidYAML(t *testing.T) {
	content := `
daemon:
  socket_path: /tmp/mesh.sock
  invalid yaml here [
`
	path := writeConfig(t, content)

	_, err := Load(path)
	if err == nil {
		t.Fatal("Load() expected error for invalid YAML, got nil")
	}
	if !strings.Contains(err.Error(), "parse") {
		t.Errorf("Load() error = %v, want parse error", err)
	}
}

// TestLoadNonexistentFile verifies error for non-existent path.
func TestLoadNonexistentFile(t *testing.T) {
	_, err := Load("/nonexistent/path/config.yaml")
	if err == nil {
		t.Fatal("Load() expected error for non-existent file, got nil")
	}
	if !strings.Contains(err.Error(), "read") {
		t.Errorf("Load() error = %v, want read error", err)
	}
}

// TestDefaultPath verifies DefaultPath returns ~/.mesh/config.yaml.
func TestDefaultPath(t *testing.T) {
	// Clear MESH_CONFIG if set
	oldVal := os.Getenv("MESH_CONFIG")
	os.Unsetenv("MESH_CONFIG")
	defer func() {
		if oldVal != "" {
			os.Setenv("MESH_CONFIG", oldVal)
		}
	}()

	path := DefaultPath()
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir() error = %v", err)
	}
	wantPath := filepath.Join(home, ".mesh", "config.yaml")
	if path != wantPath {
		t.Errorf("DefaultPath() = %q, want %q", path, wantPath)
	}
}

// TestDefaultPathEnvOverride verifies MESH_CONFIG env var overrides default path.
func TestDefaultPathEnvOverride(t *testing.T) {
	// Set MESH_CONFIG
	oldVal := os.Getenv("MESH_CONFIG")
	os.Setenv("MESH_CONFIG", "/custom/config.yaml")
	defer func() {
		if oldVal != "" {
			os.Setenv("MESH_CONFIG", oldVal)
		} else {
			os.Unsetenv("MESH_CONFIG")
		}
	}()

	path := DefaultPath()
	if path != "/custom/config.yaml" {
		t.Errorf("DefaultPath() = %q, want %q", path, "/custom/config.yaml")
	}
}

// TestConfigStructTags verifies all exported fields have yaml tags.
func TestConfigStructTags(t *testing.T) {
	types := []interface{}{
		DaemonConfig{},
		StoreConfig{},
		DockerConfig{},
		BodyConfig{},
		Config{},
	}

	for _, typ := range types {
		val := reflect.ValueOf(typ)
		if val.Kind() == reflect.Ptr {
			val = val.Elem()
		}
		typVal := val.Type()

		for i := 0; i < typVal.NumField(); i++ {
			field := typVal.Field(i)
			// Skip unexported fields
			if !field.IsExported() {
				continue
			}

			tag := field.Tag.Get("yaml")
			if tag == "" {
				t.Errorf("Field %s.%s has no yaml tag", typVal.Name(), field.Name)
			}
		}
	}
}

// TestBodiesEmptyList verifies empty bodies list loads without error.
func TestBodiesEmptyList(t *testing.T) {
	content := `
bodies: []
`
	path := writeConfig(t, content)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Bodies == nil {
		t.Fatal("Bodies is nil, want empty slice")
	}
	if len(cfg.Bodies) != 0 {
		t.Errorf("len(Bodies) = %d, want 0", len(cfg.Bodies))
	}
}

// TestLoadMultipleBodies verifies loading multiple body configs.
func TestLoadMultipleBodies(t *testing.T) {
	content := `
bodies:
  - name: agent1
    image: alpine:latest
    workdir: /home/agent1
  - name: agent2
    image: ubuntu:latest
    workdir: /home/agent2
    memory_mb: 512
    cpu_shares: 1024
`
	path := writeConfig(t, content)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(cfg.Bodies) != 2 {
		t.Fatalf("len(Bodies) = %d, want 2", len(cfg.Bodies))
	}

	// Verify first body
	if cfg.Bodies[0].Name != "agent1" {
		t.Errorf("Bodies[0].Name = %q, want %q", cfg.Bodies[0].Name, "agent1")
	}
	if cfg.Bodies[0].Image != "alpine:latest" {
		t.Errorf("Bodies[0].Image = %q, want %q", cfg.Bodies[0].Image, "alpine:latest")
	}

	// Verify second body
	if cfg.Bodies[1].Name != "agent2" {
		t.Errorf("Bodies[1].Name = %q, want %q", cfg.Bodies[1].Name, "agent2")
	}
	if cfg.Bodies[1].Image != "ubuntu:latest" {
		t.Errorf("Bodies[1].Image = %q, want %q", cfg.Bodies[1].Image, "ubuntu:latest")
	}
	if cfg.Bodies[1].MemoryMB != 512 {
		t.Errorf("Bodies[1].MemoryMB = %d, want 512", cfg.Bodies[1].MemoryMB)
	}
	if cfg.Bodies[1].CPUShares != 1024 {
		t.Errorf("Bodies[1].CPUShares = %d, want 1024", cfg.Bodies[1].CPUShares)
	}
}

// TestValidateBodyNameEmpty verifies validation error for empty body name.
func TestValidateBodyNameEmpty(t *testing.T) {
	cfg := &Config{
		Bodies: []BodyConfig{
			{Name: "", Image: "alpine:latest"},
		},
	}
	err := validate(cfg)
	if err == nil {
		t.Fatal("validate() expected error for empty name, got nil")
	}
	if !strings.Contains(err.Error(), "empty name") {
		t.Errorf("validate() error = %v, want error containing 'empty name'", err)
	}
}

// TestValidateBodyImageEmpty verifies validation error for empty body image.
func TestValidateBodyImageEmpty(t *testing.T) {
	cfg := &Config{
		Bodies: []BodyConfig{
			{Name: "agent1", Image: ""},
		},
	}
	err := validate(cfg)
	if err == nil {
		t.Fatal("validate() expected error for empty image, got nil")
	}
	if !strings.Contains(err.Error(), "image must not be empty") {
		t.Errorf("validate() error = %v, want error containing 'image must not be empty'", err)
	}
}
