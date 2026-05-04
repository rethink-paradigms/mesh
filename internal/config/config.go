// Package config provides YAML configuration parsing and validation for the Mesh daemon.
package config

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// DaemonConfig holds daemon runtime settings.
type DaemonConfig struct {
	SocketPath string `yaml:"socket_path"`
	PIDFile    string `yaml:"pid_file"`
	LogLevel   string `yaml:"log_level"`
	ListenAddr string `yaml:"listen_addr"`
	AuthToken  string `yaml:"auth_token"`
}

// StoreConfig holds SQLite store settings.
type StoreConfig struct {
	Path string `yaml:"path"`
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
	Substrate string            `yaml:"substrate"`
}

type RegistryConfig struct {
	Type            string `yaml:"type"`
	Bucket          string `yaml:"bucket"`
	Region          string `yaml:"region"`
	Endpoint        string `yaml:"endpoint"`
	AccessKeyID     string `yaml:"access_key_id"`
	SecretAccessKey string `yaml:"secret_access_key"`
}

type PluginConfig struct {
	Dir     string   `yaml:"dir"`
	Enabled []string `yaml:"enabled"`
}

// Config is the top-level v1 configuration.
type Config struct {
	Daemon        DaemonConfig                 `yaml:"daemon"`
	Store         StoreConfig                  `yaml:"store"`
	Orchestrators map[string]map[string]string `yaml:"orchestrators"`
	Provisioners  map[string]map[string]string `yaml:"provisioners"`
	Bodies        []BodyConfig                 `yaml:"bodies"`
	Registry      RegistryConfig               `yaml:"registry"`
	Plugin        PluginConfig                 `yaml:"plugin"`

	// Legacy fields for backward compatibility — parsed then migrated to Orchestrators
	Nomad nomadCompat `yaml:"nomad"`
}

// nomadCompat captures the legacy [nomad] section for backward compatibility.
type nomadCompat struct {
	Address   string `yaml:"address"`
	Token     string `yaml:"token"`
	Region    string `yaml:"region"`
	Namespace string `yaml:"namespace"`
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
	if cfg.Daemon.ListenAddr == "" {
		cfg.Daemon.ListenAddr = "127.0.0.1:8080"
	}
	if cfg.Store.Path == "" {
		home, err := os.UserHomeDir()
		if err == nil {
			cfg.Store.Path = filepath.Join(home, ".mesh", "state.db")
		}
	}
	if cfg.Registry.Type == "" {
		cfg.Registry.Type = "s3"
	}
	if cfg.Plugin.Dir == "" {
		home, err := os.UserHomeDir()
		if err == nil {
			cfg.Plugin.Dir = filepath.Join(home, ".mesh", "plugins")
		}
	}

	// Initialize maps
	if cfg.Orchestrators == nil {
		cfg.Orchestrators = make(map[string]map[string]string)
	}
	if cfg.Provisioners == nil {
		cfg.Provisioners = make(map[string]map[string]string)
	}

	// Backward compatibility: migrate legacy [nomad] section to orchestrators.nomad
	if cfg.Nomad.Address != "" || cfg.Nomad.Token != "" || cfg.Nomad.Region != "" || cfg.Nomad.Namespace != "" {
		if cfg.Orchestrators["nomad"] == nil {
			cfg.Orchestrators["nomad"] = make(map[string]string)
		}
		if cfg.Nomad.Address != "" {
			cfg.Orchestrators["nomad"]["address"] = cfg.Nomad.Address
		}
		if cfg.Nomad.Token != "" {
			cfg.Orchestrators["nomad"]["token"] = cfg.Nomad.Token
		}
		if cfg.Nomad.Region != "" {
			cfg.Orchestrators["nomad"]["region"] = cfg.Nomad.Region
		}
		if cfg.Nomad.Namespace != "" {
			cfg.Orchestrators["nomad"]["namespace"] = cfg.Nomad.Namespace
		}
	}

	// Default nomad address if not set
	if cfg.Orchestrators["nomad"] == nil {
		cfg.Orchestrators["nomad"] = make(map[string]string)
	}
	if cfg.Orchestrators["nomad"]["address"] == "" {
		cfg.Orchestrators["nomad"]["address"] = "http://127.0.0.1:4646"
	}

	for i := range cfg.Bodies {
		if cfg.Bodies[i].Substrate == "" {
			cfg.Bodies[i].Substrate = "docker"
		}
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
	if cfg.Plugin.Dir != "" {
		if _, err := os.Stat(cfg.Plugin.Dir); err != nil {
			if os.Getenv("MESH_TESTING") == "" {
				return fmt.Errorf("config: plugin dir %q does not exist", cfg.Plugin.Dir)
			}
			_ = os.MkdirAll(cfg.Plugin.Dir, 0755)
		}
	}
	if addr := cfg.Orchestrators["nomad"]["address"]; addr != "" {
		u, err := url.Parse(addr)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return fmt.Errorf("config: nomad address %q is not a valid URL", addr)
		}
	}
	if cfg.Registry.Type == "s3" && cfg.Registry.Bucket == "" {
		return fmt.Errorf("config: registry bucket is required when type is s3")
	}
	return nil
}
