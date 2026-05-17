// Package config provides YAML configuration parsing and validation for the Mesh daemon.
//
// CANONICAL SCHEMA: contracts/mesh-daemon-config.schema.json
// If this file disagrees with the schema, the schema wins until reconciled.
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
	// ListenAddr is an explicit override for the daemon HTTP listen address.
	// Preferred method is the MESH_PORT environment variable (no default).
	ListenAddr string `yaml:"listen_addr"`
	AuthToken  string `yaml:"auth_token"`

	AuthMode       string `yaml:"auth_mode"`        // "token" | "jwt" | "both"
	Auth0Domain    string `yaml:"auth0_domain"`     // Auth0 tenant domain
	Auth0Audience  string `yaml:"auth0_audience"`   // Auth0 API audience identifier
	ClusterOwnerID string `yaml:"cluster_owner_id"` // Auth0 user ID that owns this cluster
	ClusterID      string `yaml:"cluster_id"`       // Cluster UUID assigned by gateway

	GatewayURL               string `yaml:"gateway_url"`
	HeartbeatIntervalSeconds int    `yaml:"heartbeat_interval_seconds"`
	HeartbeatEnabled         bool   `yaml:"-"`
}

// AuthConfig holds authentication configuration for API consumption.
// It is a subset of DaemonConfig for passing auth settings to services.
type AuthConfig struct {
	Mode           string // "token", "jwt", or "both"
	Token          string // daemon auth_token
	Auth0Domain    string
	Auth0Audience  string
	ClusterOwnerID string
	ClusterID      string
}

// AuthConfig returns the auth configuration derived from daemon settings.
func (c *Config) AuthConfig() AuthConfig {
	mode := c.Daemon.AuthMode
	if mode == "" {
		mode = "token"
	}
	return AuthConfig{
		Mode:           mode,
		Token:          c.Daemon.AuthToken,
		Auth0Domain:    c.Daemon.Auth0Domain,
		Auth0Audience:  c.Daemon.Auth0Audience,
		ClusterOwnerID: c.Daemon.ClusterOwnerID,
		ClusterID:      c.Daemon.ClusterID,
	}
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

type IngressConfig struct {
	Adapter       string `yaml:"adapter"`
	AdminURL      string `yaml:"admin_url"`
	PortPoolStart int    `yaml:"port_pool_start"`
	PortPoolEnd   int    `yaml:"port_pool_end"`
	DomainSuffix  string `yaml:"domain_suffix"`
	PublicDomain  string `yaml:"public_domain"`
}

// Config is the top-level v1 configuration.
type Config struct {
	Daemon        DaemonConfig                 `yaml:"daemon"`
	Store         StoreConfig                  `yaml:"store"`
	Orchestrators map[string]map[string]string `yaml:"orchestrators"`
	Bodies        []BodyConfig                 `yaml:"bodies"`
	Registry      RegistryConfig               `yaml:"registry"`
	Plugin        PluginConfig                 `yaml:"plugin"`
	Ingress       IngressConfig                `yaml:"ingress"`
	Tier          string                       `yaml:"tier"`
	Features      map[string]bool              `yaml:"features"`
	Limits        LimitsConfig                 `yaml:"limits"`
	AgentsDir     string                       `yaml:"agents_dir"`

	// Legacy fields for backward compatibility — parsed then migrated to Orchestrators
	Nomad nomadCompat `yaml:"nomad"`
}

// LimitsConfig defines hard limits for the daemon.
type LimitsConfig struct {
	MaxBodies    int `yaml:"max_bodies"`
	MaxSnapshots int `yaml:"max_snapshots"`
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

	if cfg.Daemon.HeartbeatIntervalSeconds == 0 {
		cfg.Daemon.HeartbeatIntervalSeconds = 30
	}
	cfg.Daemon.HeartbeatEnabled = cfg.Daemon.HeartbeatIntervalSeconds > 0 && cfg.Daemon.GatewayURL != ""
	if cfg.Store.Path == "" {
		home, err := os.UserHomeDir()
		if err == nil {
			cfg.Store.Path = filepath.Join(home, ".mesh", "state.db")
		}
	}
	// Registry type is intentionally NOT defaulted to "s3".
	// An empty or "none" type means no registry — migrations use same-machine transfer.
	// Users can configure S3 at runtime via the API.
	// See: api/registry.go, daemon.go SetRegistry()
	if cfg.Plugin.Dir == "" {
		home, err := os.UserHomeDir()
		if err == nil {
			cfg.Plugin.Dir = filepath.Join(home, ".mesh", "plugins")
		}
	}
	if cfg.Limits.MaxBodies == 0 {
		cfg.Limits.MaxBodies = 10
	}
	if cfg.Limits.MaxSnapshots == 0 {
		cfg.Limits.MaxSnapshots = 5
	}
	if cfg.Ingress.Adapter == "" {
		cfg.Ingress.Adapter = "noop"
	}
	if cfg.Ingress.AdminURL == "" {
		cfg.Ingress.AdminURL = "http://127.0.0.1:2019"
	}
	if cfg.Ingress.PortPoolStart == 0 {
		cfg.Ingress.PortPoolStart = 9000
	}
	if cfg.Ingress.PortPoolEnd == 0 {
		cfg.Ingress.PortPoolEnd = 9999
	}
	if cfg.Ingress.DomainSuffix == "" {
		cfg.Ingress.DomainSuffix = ".mesh.local"
	}
	if cfg.AgentsDir == "" {
		home, err := os.UserHomeDir()
		if err == nil {
			cfg.AgentsDir = filepath.Join(home, ".mesh", "agents")
		}
	}

	// Initialize maps
	if cfg.Orchestrators == nil {
		cfg.Orchestrators = make(map[string]map[string]string)
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
	if addr := cfg.Orchestrators["nomad"]["address"]; addr != "" {
		u, err := url.Parse(addr)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return fmt.Errorf("config: nomad address %q is not a valid URL", addr)
		}
	}
	// Validate registry type. Empty or "none" = no registry (valid).
	// S3 requires a bucket. The S3 plugin is initialized at runtime via the API.
	switch cfg.Registry.Type {
	case "", "none":
		// No registry configured — migrations use same-machine transfer.
	case "s3":
		if cfg.Registry.Bucket == "" {
			return fmt.Errorf("config: registry bucket is required when type is s3")
		}
		// If provided in config file, it will also be initialized at daemon start
		// (see daemon.go SetRegistry / restorePersistedRegistry).
	default:
		return fmt.Errorf("config: registry type %q is invalid — must be \"s3\", \"none\", or empty", cfg.Registry.Type)
	}

	// Validate auth_mode
	switch cfg.Daemon.AuthMode {
	case "jwt", "both":
		if cfg.Daemon.Auth0Domain == "" {
			return fmt.Errorf("config: auth0_domain is required when auth_mode is %q", cfg.Daemon.AuthMode)
		}
		if cfg.Daemon.Auth0Audience == "" {
			return fmt.Errorf("config: auth0_audience is required when auth_mode is %q", cfg.Daemon.AuthMode)
		}
	case "token", "":
		// auth_mode defaults to "token", no JWT fields required
	default:
		return fmt.Errorf("config: auth_mode %q is invalid — must be \"token\", \"jwt\", or \"both\"", cfg.Daemon.AuthMode)
	}

	return nil
}

// EnsureDirs creates parent directories for all paths the daemon needs to write.
// It is called after config validation succeeds and before the daemon starts.
func EnsureDirs(cfg *Config) error {
	if cfg.Plugin.Dir != "" {
		if err := os.MkdirAll(cfg.Plugin.Dir, 0755); err != nil {
			return fmt.Errorf("config: create plugin dir %q: %w", cfg.Plugin.Dir, err)
		}
	}
	if cfg.Store.Path != "" {
		if err := os.MkdirAll(filepath.Dir(cfg.Store.Path), 0755); err != nil {
			return fmt.Errorf("config: create store parent dir %q: %w", filepath.Dir(cfg.Store.Path), err)
		}
	}
	if cfg.Daemon.PIDFile != "" {
		if err := os.MkdirAll(filepath.Dir(cfg.Daemon.PIDFile), 0755); err != nil {
			return fmt.Errorf("config: create pid_file parent dir %q: %w", filepath.Dir(cfg.Daemon.PIDFile), err)
		}
	}
	return nil
}
