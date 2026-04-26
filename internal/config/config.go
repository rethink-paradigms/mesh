// Package config provides YAML configuration parsing and validation for the Mesh daemon.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// DaemonConfig holds daemon runtime settings.
type DaemonConfig struct {
	SocketPath string `yaml:"socket_path"`
	PIDFile    string `yaml:"pid_file"`
	LogLevel   string `yaml:"log_level"`
}

// StoreConfig holds SQLite store settings.
type StoreConfig struct {
	Path string `yaml:"path"`
}

// DockerConfig holds Docker adapter settings.
type DockerConfig struct {
	Host       string `yaml:"host"`
	APIVersion string `yaml:"api_version"`
}

// BodyConfig defines a body to be managed by the daemon.
type BodyConfig struct {
	Name      string            `yaml:"name"`
	Image     string            `yaml:"image"`
	Workdir   string            `yaml:"workdir"`
	Env       map[string]string `yaml:"env"`
	Cmd       []string          `yaml:"cmd"`
	MemoryMB  int               `yaml:"memory_mb"`
	CPUShares int               `yaml:"cpu_shares"`
}

// Config is the top-level v1 configuration.
type Config struct {
	Daemon DaemonConfig `yaml:"daemon"`
	Store  StoreConfig  `yaml:"store"`
	Docker DockerConfig `yaml:"docker"`
	Bodies []BodyConfig `yaml:"bodies"`
}

// DefaultPath returns the default config file path (~/.mesh/config.yaml),
// respecting the MESH_CONFIG env var.
func DefaultPath() string {
	if p := os.Getenv("MESH_CONFIG"); p != "" {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join("~", ".mesh", "config.yaml")
	}
	return filepath.Join(home, ".mesh", "config.yaml")
}

// Load reads and parses a YAML config file, applying defaults.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("config: read %s: %w", path, err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("config: parse %s: %w", path, err)
	}
	applyDefaults(&cfg)
	if err := validate(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func applyDefaults(cfg *Config) {
	if cfg.Daemon.SocketPath == "" {
		cfg.Daemon.SocketPath = "/tmp/mesh.sock"
	}
	if cfg.Daemon.PIDFile == "" {
		home, err := os.UserHomeDir()
		if err == nil {
			cfg.Daemon.PIDFile = filepath.Join(home, ".mesh", "mesh.pid")
		}
	}
	if cfg.Daemon.LogLevel == "" {
		cfg.Daemon.LogLevel = "info"
	}
	if cfg.Store.Path == "" {
		home, err := os.UserHomeDir()
		if err == nil {
			cfg.Store.Path = filepath.Join(home, ".mesh", "state.db")
		}
	}
	if cfg.Docker.Host == "" {
		cfg.Docker.Host = "unix:///var/run/docker.sock"
	}
	if cfg.Docker.APIVersion == "" {
		cfg.Docker.APIVersion = "1.48"
	}
}

func validate(cfg *Config) error {
	for _, b := range cfg.Bodies {
		if b.Name == "" {
			return fmt.Errorf("config: body has empty name")
		}
		if b.Image == "" {
			return fmt.Errorf("config: body %q: image must not be empty", b.Name)
		}
	}
	return nil
}
