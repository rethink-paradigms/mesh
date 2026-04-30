// Package config tests YAML configuration parsing and validation.
package config

import (
	"fmt"
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
registry:
  type: s3
  bucket: my-bucket
plugin:
  dir: /tmp
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
registry:
  type: s3
  bucket: my-bucket
plugin:
  dir: /tmp
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
registry:
  type: s3
  bucket: my-bucket
plugin:
  dir: /tmp
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
registry:
  type: s3
  bucket: my-bucket
plugin:
  dir: /tmp
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

// TestLoadRegistryConfig loads a config with registry section.
func TestLoadRegistryConfig(t *testing.T) {
	content := `
registry:
  type: s3
  bucket: my-snapshots
  region: us-east-1
  endpoint: http://localhost:9000
  access_key_id: AKIAIOSFODNN7EXAMPLE
  secret_access_key: wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
plugin:
  dir: /tmp
bodies:
  - name: agent1
    image: alpine:latest
`
	path := writeConfig(t, content)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Registry.Type != "s3" {
		t.Errorf("Registry.Type = %q, want %q", cfg.Registry.Type, "s3")
	}
	if cfg.Registry.Bucket != "my-snapshots" {
		t.Errorf("Registry.Bucket = %q, want %q", cfg.Registry.Bucket, "my-snapshots")
	}
	if cfg.Registry.Region != "us-east-1" {
		t.Errorf("Registry.Region = %q, want %q", cfg.Registry.Region, "us-east-1")
	}
	if cfg.Registry.Endpoint != "http://localhost:9000" {
		t.Errorf("Registry.Endpoint = %q, want %q", cfg.Registry.Endpoint, "http://localhost:9000")
	}
	if cfg.Registry.AccessKeyID != "AKIAIOSFODNN7EXAMPLE" {
		t.Errorf("Registry.AccessKeyID = %q, want %q", cfg.Registry.AccessKeyID, "AKIAIOSFODNN7EXAMPLE")
	}
	if cfg.Registry.SecretAccessKey != "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY" {
		t.Errorf("Registry.SecretAccessKey = %q, want %q", cfg.Registry.SecretAccessKey, "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY")
	}
}

// TestLoadPluginConfig loads a config with plugin section.
func TestLoadPluginConfig(t *testing.T) {
	// Create a temp plugin dir so validation passes
	tmpDir := t.TempDir()
	pluginDir := filepath.Join(tmpDir, "plugins")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatalf("mkdir plugins: %v", err)
	}

	content := fmt.Sprintf(`
registry:
  type: s3
  bucket: my-bucket
plugin:
  dir: %s
  enabled:
    - docker
    - nomad
bodies:
  - name: agent1
    image: alpine:latest
`, pluginDir)
	path := writeConfig(t, content)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Plugin.Dir != pluginDir {
		t.Errorf("Plugin.Dir = %q, want %q", cfg.Plugin.Dir, pluginDir)
	}
	if len(cfg.Plugin.Enabled) != 2 {
		t.Fatalf("len(Plugin.Enabled) = %d, want 2", len(cfg.Plugin.Enabled))
	}
	if cfg.Plugin.Enabled[0] != "docker" {
		t.Errorf("Plugin.Enabled[0] = %q, want %q", cfg.Plugin.Enabled[0], "docker")
	}
	if cfg.Plugin.Enabled[1] != "nomad" {
		t.Errorf("Plugin.Enabled[1] = %q, want %q", cfg.Plugin.Enabled[1], "nomad")
	}
}

// TestLoadNomadConfig loads a config with nomad section.
func TestLoadNomadConfig(t *testing.T) {
	content := `
registry:
  type: s3
  bucket: my-bucket
nomad:
  address: http://nomad.example.com:4646
  token: abc123
  region: us-west-2
  namespace: production
plugin:
  dir: /tmp
bodies:
  - name: agent1
    image: alpine:latest
`
	path := writeConfig(t, content)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Nomad.Address != "http://nomad.example.com:4646" {
		t.Errorf("Nomad.Address = %q, want %q", cfg.Nomad.Address, "http://nomad.example.com:4646")
	}
	if cfg.Nomad.Token != "abc123" {
		t.Errorf("Nomad.Token = %q, want %q", cfg.Nomad.Token, "abc123")
	}
	if cfg.Nomad.Region != "us-west-2" {
		t.Errorf("Nomad.Region = %q, want %q", cfg.Nomad.Region, "us-west-2")
	}
	if cfg.Nomad.Namespace != "production" {
		t.Errorf("Nomad.Namespace = %q, want %q", cfg.Nomad.Namespace, "production")
	}
}

// TestLoadBodySubstrate loads a config with body substrate field.
func TestLoadBodySubstrate(t *testing.T) {
	content := `
registry:
  type: s3
  bucket: my-bucket
plugin:
  dir: /tmp
bodies:
  - name: agent1
    image: alpine:latest
    substrate: nomad
`
	path := writeConfig(t, content)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if len(cfg.Bodies) != 1 {
		t.Fatalf("len(Bodies) = %d, want 1", len(cfg.Bodies))
	}
	if cfg.Bodies[0].Substrate != "nomad" {
		t.Errorf("Bodies[0].Substrate = %q, want %q", cfg.Bodies[0].Substrate, "nomad")
	}
}

// TestLoadDefaultsNewSections verifies defaults for new sections.
func TestLoadDefaultsNewSections(t *testing.T) {
	content := `
registry:
  type: s3
  bucket: my-bucket
plugin:
  dir: /tmp
bodies:
  - name: agent1
    image: alpine:latest
`
	path := writeConfig(t, content)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Registry.Type != "s3" {
		t.Errorf("Registry.Type = %q, want %q", cfg.Registry.Type, "s3")
	}

	if cfg.Nomad.Address != "http://127.0.0.1:4646" {
		t.Errorf("Nomad.Address = %q, want %q", cfg.Nomad.Address, "http://127.0.0.1:4646")
	}

	if cfg.Bodies[0].Substrate != "docker" {
		t.Errorf("Bodies[0].Substrate = %q, want %q", cfg.Bodies[0].Substrate, "docker")
	}
}

// TestValidatePluginDirMissing verifies error for missing plugin dir.
func TestValidatePluginDirMissing(t *testing.T) {
	content := `
plugin:
  dir: /nonexistent/plugins/dir
bodies:
  - name: agent1
    image: alpine:latest
`
	path := writeConfig(t, content)

	_, err := Load(path)
	if err == nil {
		t.Fatal("Load() expected error for missing plugin dir, got nil")
	}
	if !strings.Contains(err.Error(), "plugin dir") {
		t.Errorf("Load() error = %v, want error containing 'plugin dir'", err)
	}
}

// TestValidateNomadAddressInvalid verifies error for invalid nomad address.
func TestValidateNomadAddressInvalid(t *testing.T) {
	tests := []struct {
		name    string
		address string
	}{
		{"empty scheme", "nomad.example.com:4646"},
		{"invalid scheme", "ftp://nomad.example.com:4646"},
		{"missing host", "http://"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content := fmt.Sprintf(`
registry:
  type: s3
  bucket: my-bucket
plugin:
  dir: /tmp
nomad:
  address: %s
bodies:
  - name: agent1
    image: alpine:latest
`, tt.address)
			path := writeConfig(t, content)
			_, err := Load(path)
			if err == nil {
				t.Fatal("Load() expected error for invalid nomad address, got nil")
			}
			if !strings.Contains(err.Error(), "nomad address") {
				t.Errorf("Load() error = %v, want error containing 'nomad address'", err)
			}
		})
	}
}

// TestValidateRegistryS3MissingBucket verifies error for s3 registry without bucket.
func TestValidateRegistryS3MissingBucket(t *testing.T) {
	content := `
registry:
  type: s3
plugin:
  dir: /tmp
bodies:
  - name: agent1
    image: alpine:latest
`
	path := writeConfig(t, content)

	_, err := Load(path)
	if err == nil {
		t.Fatal("Load() expected error for s3 registry without bucket, got nil")
	}
	if !strings.Contains(err.Error(), "registry bucket") {
		t.Errorf("Load() error = %v, want error containing 'registry bucket'", err)
	}
}

// TestConfigStructTagsNew verifies new structs have yaml tags.
func TestConfigStructTagsNew(t *testing.T) {
	types := []interface{}{
		RegistryConfig{},
		PluginConfig{},
		NomadConfig{},
	}

	for _, typ := range types {
		val := reflect.ValueOf(typ)
		if val.Kind() == reflect.Ptr {
			val = val.Elem()
		}
		typVal := val.Type()

		for i := 0; i < typVal.NumField(); i++ {
			field := typVal.Field(i)
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
