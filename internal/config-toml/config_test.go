package configtoml

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return p
}

const validConfig = `
[[machines]]
name = "local"
host = "localhost"
port = 22
user = "mesh"
agent_dir = "/opt/mesh/agents"

[[agents]]
name = "my-agent"
machine = "local"
workdir = "/var/lib/mesh/agents/my-agent"
start_cmd = "./run.sh"
`

func TestLoadValidConfig(t *testing.T) {
	p := writeConfig(t, validConfig)
	cfg, err := Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.Machines) != 1 || cfg.Machines[0].Name != "local" {
		t.Fatalf("machines: got %+v", cfg.Machines)
	}
	if len(cfg.Agents) != 1 || cfg.Agents[0].Name != "my-agent" {
		t.Fatalf("agents: got %+v", cfg.Agents)
	}
}

func TestLoadEmptyAgentName(t *testing.T) {
	content := `
[[machines]]
name = "local"
host = "localhost"

[[agents]]
name = ""
machine = "local"
workdir = "/tmp"
start_cmd = "./run"
`
	p := writeConfig(t, content)
	_, err := Load(p)
	if err == nil {
		t.Fatal("expected error for empty agent name")
	}
	if !contains(err.Error(), "empty name") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadEmptyMachineName(t *testing.T) {
	content := `
[[machines]]
name = ""
host = "localhost"

[[agents]]
name = "a1"
machine = ""
workdir = "/tmp"
start_cmd = "./run"
`
	p := writeConfig(t, content)
	_, err := Load(p)
	if err == nil {
		t.Fatal("expected error for empty machine name")
	}
	if !contains(err.Error(), "empty name") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadDuplicateAgentName(t *testing.T) {
	content := `
[[machines]]
name = "local"
host = "localhost"

[[agents]]
name = "dup"
machine = "local"
workdir = "/tmp/a"
start_cmd = "./run"

[[agents]]
name = "dup"
machine = "local"
workdir = "/tmp/b"
start_cmd = "./run"
`
	p := writeConfig(t, content)
	_, err := Load(p)
	if err == nil {
		t.Fatal("expected error for duplicate agent name")
	}
	if !contains(err.Error(), "duplicate agent name") || !contains(err.Error(), "dup") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadNonexistentMachineRef(t *testing.T) {
	content := `
[[machines]]
name = "local"
host = "localhost"

[[agents]]
name = "a1"
machine = "nonexistent"
workdir = "/tmp"
start_cmd = "./run"
`
	p := writeConfig(t, content)
	_, err := Load(p)
	if err == nil {
		t.Fatal("expected error for non-existent machine ref")
	}
	if !contains(err.Error(), "non-existent machine") || !contains(err.Error(), "nonexistent") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadNonexistentSSHKey(t *testing.T) {
	content := `
[[machines]]
name = "remote"
host = "1.2.3.4"
ssh_key = "/nonexistent/path/id_rsa"

[[agents]]
name = "a1"
machine = "remote"
workdir = "/tmp"
start_cmd = "./run"
`
	p := writeConfig(t, content)
	_, err := Load(p)
	if err == nil {
		t.Fatal("expected error for non-existent ssh_key")
	}
	if !contains(err.Error(), "does not exist") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadSSHKeyWrongPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission checks skipped on Windows")
	}

	dir := t.TempDir()
	keyPath := filepath.Join(dir, "id_rsa")
	if err := os.WriteFile(keyPath, []byte("fake-key"), 0644); err != nil {
		t.Fatalf("write key: %v", err)
	}

	content := `
[[machines]]
name = "remote"
host = "1.2.3.4"
ssh_key = "` + keyPath + `"

[[agents]]
name = "a1"
machine = "remote"
workdir = "/tmp"
start_cmd = "./run"
`
	p := writeConfig(t, content)
	_, err := Load(p)
	if err == nil {
		t.Fatal("expected error for wrong ssh_key permissions")
	}
	if !contains(err.Error(), "must be <= 0600") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDefaultPathExpandsHome(t *testing.T) {
	p := DefaultPath()
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir: %v", err)
	}
	expected := filepath.Join(home, ".mesh", "config.toml")
	if p != expected {
		t.Fatalf("DefaultPath: got %q, want %q", p, expected)
	}
}

func TestDefaultPathMeshConfigEnvOverride(t *testing.T) {
	custom := "/custom/path/config.toml"
	t.Setenv("MESH_CONFIG", custom)
	p := DefaultPath()
	if p != custom {
		t.Fatalf("DefaultPath with MESH_CONFIG: got %q, want %q", p, custom)
	}
}

func TestDefaultsApplied(t *testing.T) {
	content := `
[[machines]]
name = "local"
host = "localhost"

[[agents]]
name = "a1"
machine = "local"
workdir = "/tmp"
start_cmd = "./run"
`
	p := writeConfig(t, content)
	cfg, err := Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Machines[0].Port != 22 {
		t.Fatalf("default port: got %d, want 22", cfg.Machines[0].Port)
	}
	if cfg.Agents[0].StopSignal != "SIGTERM" {
		t.Fatalf("default stop_signal: got %q, want SIGTERM", cfg.Agents[0].StopSignal)
	}
	if cfg.Agents[0].StopTimeout != "30s" {
		t.Fatalf("default stop_timeout: got %q, want 30s", cfg.Agents[0].StopTimeout)
	}
	if cfg.Agents[0].MaxSnapshots != 10 {
		t.Fatalf("default max_snapshots: got %d, want 10", cfg.Agents[0].MaxSnapshots)
	}
}

func TestDefaultsNotOverridden(t *testing.T) {
	content := `
[[machines]]
name = "local"
host = "localhost"
port = 2222

[[agents]]
name = "a1"
machine = "local"
workdir = "/tmp"
start_cmd = "./run"
stop_signal = "SIGINT"
stop_timeout = "5s"
max_snapshots = 3
`
	p := writeConfig(t, content)
	cfg, err := Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Machines[0].Port != 2222 {
		t.Fatalf("port: got %d, want 2222", cfg.Machines[0].Port)
	}
	if cfg.Agents[0].StopSignal != "SIGINT" {
		t.Fatalf("stop_signal: got %q, want SIGINT", cfg.Agents[0].StopSignal)
	}
	if cfg.Agents[0].StopTimeout != "5s" {
		t.Fatalf("stop_timeout: got %q, want 5s", cfg.Agents[0].StopTimeout)
	}
	if cfg.Agents[0].MaxSnapshots != 3 {
		t.Fatalf("max_snapshots: got %d, want 3", cfg.Agents[0].MaxSnapshots)
	}
}

func TestAgentEmptyWorkdir(t *testing.T) {
	content := `
[[machines]]
name = "local"
host = "localhost"

[[agents]]
name = "a1"
machine = "local"
workdir = ""
start_cmd = "./run"
`
	p := writeConfig(t, content)
	_, err := Load(p)
	if err == nil {
		t.Fatal("expected error for empty workdir")
	}
	if !contains(err.Error(), "workdir must not be empty") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidSSHKey(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "id_rsa")
	if err := os.WriteFile(keyPath, []byte("fake-key"), 0600); err != nil {
		t.Fatalf("write key: %v", err)
	}

	content := `
[[machines]]
name = "remote"
host = "1.2.3.4"
ssh_key = "` + keyPath + `"

[[agents]]
name = "a1"
machine = "remote"
workdir = "/tmp"
start_cmd = "./run"
`
	p := writeConfig(t, content)
	cfg, err := Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Machines[0].SSHKey != keyPath {
		t.Fatalf("ssh_key: got %q, want %q", cfg.Machines[0].SSHKey, keyPath)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstr(s, substr))
}

func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
