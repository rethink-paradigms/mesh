package daemon

import (
	"context"
	"encoding/json"
	"net/http"
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

	var body map[string]interface{}
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
