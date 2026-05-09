package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTestConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

func TestConfig_JWTMissingFieldsRefusesStartup(t *testing.T) {
	cfg := `
registry:
  type: s3
  bucket: test-bucket
plugin:
  dir: /tmp
daemon:
  auth_mode: "jwt"
  auth_token: "test"
store:
  path: ":memory:"
`
	path := writeTestConfig(t, cfg)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for missing auth0_domain in jwt mode, got nil")
	}
}

func TestConfig_TokenModeDoesNotRequireJWT(t *testing.T) {
	cfg := `
registry:
  type: s3
  bucket: test-bucket
plugin:
  dir: /tmp
daemon:
  auth_mode: "token"
  auth_token: "test"
store:
  path: ":memory:"
`
	path := writeTestConfig(t, cfg)
	_, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestConfig_AuthModeDefaults(t *testing.T) {
	cfg := `
registry:
  type: s3
  bucket: test-bucket
plugin:
  dir: /tmp
daemon:
  auth_token: "test"
store:
  path: ":memory:"
`
	path := writeTestConfig(t, cfg)
	c, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.Daemon.AuthMode != "" {
		t.Fatalf("AuthMode = %q, want empty (default)", c.Daemon.AuthMode)
	}
}

func TestConfig_JWTAudienceRequired(t *testing.T) {
	cfg := `
registry:
  type: s3
  bucket: test-bucket
plugin:
  dir: /tmp
daemon:
  auth_mode: "jwt"
  auth_token: "test"
  auth0_domain: "my-tenant.auth0.com"
store:
  path: ":memory:"
`
	path := writeTestConfig(t, cfg)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for missing auth0_audience in jwt mode, got nil")
	}
	if !strings.Contains(err.Error(), "auth0_audience") {
		t.Errorf("error = %v, want error containing 'auth0_audience'", err)
	}
}

func TestConfig_BothModeRequiresAllConfigs(t *testing.T) {
	cfg := `
registry:
  type: s3
  bucket: test-bucket
plugin:
  dir: /tmp
daemon:
  auth_mode: "both"
  auth_token: "test"
store:
  path: ":memory:"
`
	path := writeTestConfig(t, cfg)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for missing auth0 fields in both mode, got nil")
	}
}

func TestConfig_InvalidAuthMode(t *testing.T) {
	cfg := `
registry:
  type: s3
  bucket: test-bucket
plugin:
  dir: /tmp
daemon:
  auth_mode: "bogus"
  auth_token: "test"
store:
  path: ":memory:"
`
	path := writeTestConfig(t, cfg)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid auth_mode, got nil")
	}
}

func TestConfig_AuthConfigHelper(t *testing.T) {
	cfg := &Config{
		Daemon: DaemonConfig{
			AuthMode:       "jwt",
			AuthToken:      "test-token",
			Auth0Domain:    "my-tenant.auth0.com",
			Auth0Audience:  "mesh-api",
			ClusterOwnerID: "auth0|user123",
			ClusterID:      "cluster-abc",
		},
	}
	ac := cfg.AuthConfig()
	if ac.Mode != "jwt" {
		t.Fatalf("Mode = %q, want jwt", ac.Mode)
	}
	if ac.Token != "test-token" {
		t.Fatalf("Token = %q", ac.Token)
	}
	if ac.ClusterID != "cluster-abc" {
		t.Fatalf("ClusterID = %q", ac.ClusterID)
	}
}
