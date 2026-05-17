package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// AgentManifest describes a built-in agent that can be installed on Mesh.
type AgentManifest struct {
	Name        string         `yaml:"name"`
	Image       string         `yaml:"image"`
	Command     []string       `yaml:"command"`
	Ports       []ManifestPort `yaml:"ports"`
	Env         EnvConfig      `yaml:"env"`
	HealthCheck *HealthCheck   `yaml:"health_check"`
	Resources   ResourceLimits `yaml:"resources"`
}

// ManifestPort describes a single port mapping for an agent manifest.
type ManifestPort struct {
	Name          string `yaml:"name"`
	ContainerPort int    `yaml:"container_port"`
	Protocol      string `yaml:"protocol"`
	Expose        bool   `yaml:"expose"`
}

// EnvConfig describes required and optional environment variables.
type EnvConfig struct {
	Required []string `yaml:"required"`
	Optional []string `yaml:"optional"`
}

// HealthCheck describes a health check configuration.
type HealthCheck struct {
	Type            string `yaml:"type"`
	Path            string `yaml:"path"`
	Port            string `yaml:"port"`
	IntervalSeconds int    `yaml:"interval_seconds"`
}

// ResourceLimits describes compute resource limits.
type ResourceLimits struct {
	MemoryMB  int `yaml:"memory_mb"`
	CPUShares int `yaml:"cpu_shares"`
}

// ParseManifest parses a YAML manifest from raw bytes.
func ParseManifest(data []byte) (*AgentManifest, error) {
	var m AgentManifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}
	if err := validateManifest(&m); err != nil {
		return nil, fmt.Errorf("validate manifest: %w", err)
	}
	return &m, nil
}

// LoadManifest reads and parses a single YAML manifest file.
func LoadManifest(path string) (*AgentManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read manifest %s: %w", path, err)
	}
	return ParseManifest(data)
}

// LoadManifestDir loads all .yaml files from a directory, keyed by manifest name.
func LoadManifestDir(dir string) (map[string]*AgentManifest, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read manifest dir %s: %w", dir, err)
	}

	manifests := make(map[string]*AgentManifest)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
			continue
		}

		path := filepath.Join(dir, name)
		m, err := LoadManifest(path)
		if err != nil {
			return nil, err
		}
		manifests[m.Name] = m
	}

	return manifests, nil
}

// ValidateEnv checks that all required environment variables are present.
func ValidateEnv(manifest *AgentManifest, provided map[string]string) error {
	for _, key := range manifest.Env.Required {
		if _, ok := provided[key]; !ok {
			return fmt.Errorf("required env var %q not provided", key)
		}
	}
	return nil
}

func validateManifest(m *AgentManifest) error {
	if m.Name == "" {
		return fmt.Errorf("name is required")
	}
	if m.Image == "" {
		return fmt.Errorf("image is required")
	}
	return nil
}
