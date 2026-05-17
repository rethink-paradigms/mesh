package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Descriptor describes a built-in agent that can be installed on Mesh.
type Descriptor struct {
	Name        string         `yaml:"name"`
	Image       string         `yaml:"image"`
	Command     []string       `yaml:"command"`
	Ports       []PortMapping  `yaml:"ports"`
	Env         EnvConfig      `yaml:"env"`
	HealthCheck *HealthCheck   `yaml:"health_check"`
	Resources   ResourceLimits `yaml:"resources"`
}

// PortMapping describes a single port mapping for an agent descriptor.
type PortMapping struct {
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

// ParseDescriptor parses a YAML descriptor from raw bytes.
func ParseDescriptor(data []byte) (*Descriptor, error) {
	var m Descriptor
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse descriptor: %w", err)
	}
	if err := validateDescriptor(&m); err != nil {
		return nil, fmt.Errorf("validate descriptor: %w", err)
	}
	return &m, nil
}

// LoadDescriptor reads and parses a single YAML descriptor file.
func LoadDescriptor(path string) (*Descriptor, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read descriptor %s: %w", path, err)
	}
	return ParseDescriptor(data)
}

// LoadDescriptors loads all .yaml files from a directory, keyed by descriptor name.
func LoadDescriptors(dir string) (map[string]*Descriptor, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read descriptor dir %s: %w", dir, err)
	}

	descriptors := make(map[string]*Descriptor)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
			continue
		}

		path := filepath.Join(dir, name)
		m, err := LoadDescriptor(path)
		if err != nil {
			return nil, err
		}
		descriptors[m.Name] = m
	}

	return descriptors, nil
}

// ValidateEnv checks that all required environment variables are present.
func ValidateEnv(descriptor *Descriptor, provided map[string]string) error {
	for _, key := range descriptor.Env.Required {
		if _, ok := provided[key]; !ok {
			return fmt.Errorf("required env var %q not provided", key)
		}
	}
	return nil
}

func validateDescriptor(m *Descriptor) error {
	if m.Name == "" {
		return fmt.Errorf("name is required")
	}
	if m.Image == "" {
		return fmt.Errorf("image is required")
	}
	return nil
}
