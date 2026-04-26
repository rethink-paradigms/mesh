// Package configtoml parses and validates TOML configuration for agents, machines, and hooks.
package configtoml

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/BurntSushi/toml"
)

// Config represents the top-level mesh configuration.
type Config struct {
	Machines []Machine `toml:"machines"`
	Agents   []Agent   `toml:"agents"`
}

// Machine represents a remote or local machine where agents can run.
type Machine struct {
	Name     string `toml:"name"`
	Host     string `toml:"host"`
	Port     int    `toml:"port"`
	User     string `toml:"user"`
	SSHKey   string `toml:"ssh_key"`
	AgentDir string `toml:"agent_dir"`
}

// Agent represents a single agent process managed by mesh.
type Agent struct {
	Name           string `toml:"name"`
	Machine        string `toml:"machine"`
	Workdir        string `toml:"workdir"`
	StartCmd       string `toml:"start_cmd"`
	StopSignal     string `toml:"stop_signal"`
	StopTimeout    string `toml:"stop_timeout"`
	MaxSnapshots   int    `toml:"max_snapshots"`
	PIDFile        string `toml:"pid_file"`
	PreSnapshotCmd string `toml:"pre_snapshot_cmd"`
	PostRestoreCmd string `toml:"post_restore_cmd"`
}

// DefaultPath returns the default config file path, respecting MESH_CONFIG env var.
// Falls back to ~/.mesh/config.toml with ~ expanded via os.UserHomeDir().
func DefaultPath() string {
	if p := os.Getenv("MESH_CONFIG"); p != "" {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		// Shouldn't happen in practice; return unexpanded path
		return filepath.Join("~", ".mesh", "config.toml")
	}
	return filepath.Join(home, ".mesh", "config.toml")
}

// Load reads and parses a TOML config file, applies defaults, and validates it.
func Load(path string) (*Config, error) {
	var cfg Config
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return nil, fmt.Errorf("config: parse %s: %w", path, err)
	}
	applyDefaults(&cfg)
	if err := Validate(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// applyDefaults fills in zero-valued fields with sensible defaults.
func applyDefaults(cfg *Config) {
	for i := range cfg.Machines {
		m := &cfg.Machines[i]
		if m.Port == 0 {
			m.Port = 22
		}
	}
	for i := range cfg.Agents {
		a := &cfg.Agents[i]
		if a.StopSignal == "" {
			a.StopSignal = "SIGTERM"
		}
		if a.StopTimeout == "" {
			a.StopTimeout = "30s"
		}
		if a.MaxSnapshots == 0 {
			a.MaxSnapshots = 10
		}
	}
}

// Validate checks the config for structural correctness.
func Validate(cfg *Config) error {
	// Build machine name index for reference checks.
	machineNames := make(map[string]bool, len(cfg.Machines))
	for _, m := range cfg.Machines {
		if m.Name == "" {
			return fmt.Errorf("config: machine has empty name")
		}
		machineNames[m.Name] = true

		// Validate SSH key if specified.
		if err := validateSSHKey(m.SSHKey); err != nil {
			return fmt.Errorf("config: machine %q: %w", m.Name, err)
		}
	}

	// Check for duplicate agent names.
	agentNames := make(map[string]bool, len(cfg.Agents))
	for _, a := range cfg.Agents {
		if a.Name == "" {
			return fmt.Errorf("config: agent has empty name")
		}
		if agentNames[a.Name] {
			return fmt.Errorf("config: duplicate agent name %q", a.Name)
		}
		agentNames[a.Name] = true

		if a.Workdir == "" {
			return fmt.Errorf("config: agent %q: workdir must not be empty", a.Name)
		}

		// Machine reference must exist if specified.
		if a.Machine != "" && !machineNames[a.Machine] {
			return fmt.Errorf("config: agent %q references non-existent machine %q", a.Name, a.Machine)
		}
	}

	return nil
}

// validateSSHKey checks that the SSH key file exists and has safe permissions.
func validateSSHKey(keyPath string) error {
	if keyPath == "" {
		return nil
	}

	info, err := os.Stat(keyPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("ssh_key %q does not exist", keyPath)
		}
		return fmt.Errorf("ssh_key %q: %w", keyPath, err)
	}

	// Only check permissions on non-Windows.
	if runtime.GOOS != "windows" {
		perm := info.Mode().Perm()
		if perm > 0600 {
			return fmt.Errorf("ssh_key %q has permissions %04o, must be <= 0600", keyPath, perm)
		}
	}

	// Must be a regular file.
	if !info.Mode().IsRegular() {
		return fmt.Errorf("ssh_key %q is not a regular file", keyPath)
	}

	return nil
}

// ExpandPath expands a leading ~/ in the path to the user's home directory.
func ExpandPath(p string) (string, error) {
	if !strings.HasPrefix(p, "~/") {
		return p, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("expand ~: %w", err)
	}
	return filepath.Join(home, p[2:]), nil
}
