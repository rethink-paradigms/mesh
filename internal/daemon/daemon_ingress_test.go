package daemon

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rethink-paradigms/mesh/internal/ingress"
)

func TestDaemon_IngressAdapter_NoopDefault(t *testing.T) {
	cfg := testConfig(t)
	d, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- d.Start(ctx)
	}()

	var addr string
	for i := 0; i < 50; i++ {
		addr = d.HTTPAddr()
		if addr != "" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if addr == "" {
		cancel()
		t.Fatal("API server never started")
	}

	resp, err := http.Get("http://" + addr + "/healthz")
	if err != nil {
		cancel()
		t.Fatalf("GET /healthz: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		cancel()
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		cancel()
		t.Fatalf("decode response: %v", err)
	}
	if body["status"] == nil {
		cancel()
		t.Fatal("health endpoint returned no status field")
	}

	if _, ok := d.ingress.(*ingress.NoopAdapter); !ok {
		cancel()
		t.Fatalf("expected NoopAdapter, got %T", d.ingress)
	}

	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Start did not return within timeout")
	}
}

func TestDaemon_IngressAdapter_CaddyConfigured(t *testing.T) {
	// Start a fake Caddy admin API so caddyDetected() returns true.
	srv := startCaddyTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := testConfig(t)
	cfg.Ingress.Adapter = "caddy"
	cfg.Ingress.AdminURL = "http://127.0.0.1:2099"
	cfg.Ingress.PortPoolStart = 10000
	cfg.Ingress.PortPoolEnd = 10100
	cfg.Ingress.DomainSuffix = ".example.com"

	d, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- d.Start(ctx)
	}()

	var addr string
	for i := 0; i < 50; i++ {
		addr = d.HTTPAddr()
		if addr != "" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if addr == "" {
		cancel()
		t.Fatal("API server never started")
	}

	resp, err := http.Get("http://" + addr + "/healthz")
	if err != nil {
		cancel()
		t.Fatalf("GET /healthz: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		cancel()
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	adp, ok := d.ingress.(*ingress.CaddyAdapter)
	if !ok {
		cancel()
		t.Fatalf("expected CaddyAdapter, got %T", d.ingress)
	}

	if name := adp.Name(); name != "caddy" {
		cancel()
		t.Fatalf("adapter Name() = %q, want %q", name, "caddy")
	}

	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Start did not return within timeout")
	}
}

func TestDaemon_IngressAdapter_UnknownAdapter(t *testing.T) {
	cfg := testConfig(t)
	cfg.Ingress.Adapter = "bogus"

	d, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- d.Start(ctx)
	}()

	var addr string
	for i := 0; i < 50; i++ {
		addr = d.HTTPAddr()
		if addr != "" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if addr == "" {
		cancel()
		t.Fatal("API server never started")
	}

	resp, err := http.Get("http://" + addr + "/healthz")
	if err != nil {
		cancel()
		t.Fatalf("GET /healthz: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		cancel()
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	if _, ok := d.ingress.(*ingress.NoopAdapter); !ok {
		cancel()
		t.Fatalf("expected NoopAdapter fallback, got %T", d.ingress)
	}

	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Start did not return within timeout")
	}
}

// Regression test for ME-003: installer must receive the real ingress adapter,
// not a fresh NoopAdapter instance.
func TestDaemon_InstallerWiresRealIngressAdapter(t *testing.T) {
	// Start a fake Caddy admin API so caddyDetected() returns true.
	srv := startCaddyTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	agentsDir := filepath.Join(t.TempDir(), "agents")
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		t.Fatalf("mkdir agents: %v", err)
	}
	descriptor := `name: test-agent
image: test:latest
`
	if err := os.WriteFile(filepath.Join(agentsDir, "test.yaml"), []byte(descriptor), 0644); err != nil {
		t.Fatalf("write descriptor: %v", err)
	}

	cfg := testConfig(t)
	cfg.AgentsDir = agentsDir
	cfg.Ingress.Adapter = "caddy"
	cfg.Ingress.AdminURL = "http://127.0.0.1:2099"
	cfg.Ingress.PortPoolStart = 10000
	cfg.Ingress.PortPoolEnd = 10100
	cfg.Ingress.DomainSuffix = ".example.com"

	d, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- d.Start(ctx)
	}()

	var addr string
	for i := 0; i < 50; i++ {
		addr = d.HTTPAddr()
		if addr != "" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if addr == "" {
		cancel()
		t.Fatal("API server never started")
	}

	if d.installer == nil {
		cancel()
		t.Fatal("expected installer to be wired, got nil")
	}

	if d.installer.IngressAdapter() != d.ingress {
		cancel()
		t.Fatalf("installer ingress adapter %T != daemon ingress adapter %T", d.installer.IngressAdapter(), d.ingress)
	}

	if _, ok := d.ingress.(*ingress.CaddyAdapter); !ok {
		cancel()
		t.Fatalf("expected daemon ingress to be CaddyAdapter, got %T", d.ingress)
	}

	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Start did not return within timeout")
	}
}
