package ingress

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"
)

func TestCaddyAdapterName(t *testing.T) {
	ca := NewCaddyAdapter(CaddyConfig{})
	if ca.Name() != "caddy" {
		t.Errorf("Name() = %q, want %q", ca.Name(), "caddy")
	}
}

func TestCaddyAdapterAllocPort(t *testing.T) {
	ca := NewCaddyAdapter(CaddyConfig{
		PortPoolStart: 9000,
		PortPoolEnd:   9002,
	})

	port, err := ca.AllocPort(context.Background(), 8080)
	if err != nil {
		t.Fatalf("AllocPort returned unexpected error: %v", err)
	}
	if port != 9000 {
		t.Errorf("AllocPort returned port %d, want 9000", port)
	}
}

func TestCaddyAdapterFreePort(t *testing.T) {
	ca := NewCaddyAdapter(CaddyConfig{
		PortPoolStart: 9000,
		PortPoolEnd:   9002,
	})

	port, err := ca.AllocPort(context.Background(), 8080)
	if err != nil {
		t.Fatalf("AllocPort: %v", err)
	}

	if err := ca.FreePort(port); err != nil {
		t.Fatalf("FreePort returned unexpected error: %v", err)
	}

	port2, err := ca.AllocPort(context.Background(), 8080)
	if err != nil {
		t.Fatalf("AllocPort after free: %v", err)
	}
	if port2 != port {
		t.Errorf("AllocPort after free returned port %d, want %d", port2, port)
	}
}

func TestCaddyAdapterPoolExhaustion(t *testing.T) {
	ca := NewCaddyAdapter(CaddyConfig{
		PortPoolStart: 9000,
		PortPoolEnd:   9001,
	})

	_, err := ca.AllocPort(context.Background(), 8080)
	if err != nil {
		t.Fatalf("first AllocPort: %v", err)
	}
	_, err = ca.AllocPort(context.Background(), 8081)
	if err != nil {
		t.Fatalf("second AllocPort: %v", err)
	}

	_, err = ca.AllocPort(context.Background(), 8082)
	if err == nil {
		t.Fatal("expected error when pool exhausted, got nil")
	}
}

func TestPortPoolReserve(t *testing.T) {
	pool := NewPortPool(9000, 9002)

	// Reserve a valid port
	if err := pool.Reserve(9001); err != nil {
		t.Fatalf("Reserve(9001): %v", err)
	}

	// Alloc should skip reserved port
	port, err := pool.Alloc()
	if err != nil {
		t.Fatalf("Alloc: %v", err)
	}
	if port == 9001 {
		t.Fatal("Alloc returned reserved port 9001")
	}

	// Reserve already-allocated port should fail
	if err := pool.Reserve(port); err == nil {
		t.Fatal("expected error reserving already-allocated port, got nil")
	}

	// Reserve out-of-range port should fail
	if err := pool.Reserve(9999); err == nil {
		t.Fatal("expected error reserving out-of-range port, got nil")
	}

	// Reserve already-reserved port should fail
	if err := pool.Reserve(9001); err == nil {
		t.Fatal("expected error reserving already-reserved port, got nil")
	}
}

func TestCaddyAdapterPortPoolStats(t *testing.T) {
	ca := NewCaddyAdapter(CaddyConfig{
		PortPoolStart: 9000,
		PortPoolEnd:   9004,
	})

	start, end, used, free := ca.PortPoolStats()
	if start != 9000 {
		t.Errorf("start = %d, want 9000", start)
	}
	if end != 9004 {
		t.Errorf("end = %d, want 9004", end)
	}
	if used != 0 {
		t.Errorf("used = %d, want 0", used)
	}
	if free != 5 {
		t.Errorf("free = %d, want 5", free)
	}

	_, _ = ca.AllocPort(context.Background(), 8080)
	_, _ = ca.AllocPort(context.Background(), 8081)

	_, _, used, free = ca.PortPoolStats()
	if used != 2 {
		t.Errorf("used after alloc = %d, want 2", used)
	}
	if free != 3 {
		t.Errorf("free after alloc = %d, want 3", free)
	}
}

func TestCaddyAdapterConcurrentAlloc(t *testing.T) {
	ca := NewCaddyAdapter(CaddyConfig{
		PortPoolStart: 9000,
		PortPoolEnd:   9010,
	})

	var wg sync.WaitGroup
	ports := make(chan int, 20)
	var mu sync.Mutex
	var errs []error

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			port, err := ca.AllocPort(context.Background(), 8080)
			if err != nil {
				mu.Lock()
				errs = append(errs, err)
				mu.Unlock()
				return
			}
			ports <- port
		}()
	}

	wg.Wait()
	close(ports)

	seen := make(map[int]bool)
	for port := range ports {
		if seen[port] {
			t.Fatalf("duplicate port allocation: %d", port)
		}
		seen[port] = true
	}

	if len(seen) != 11 {
		t.Errorf("allocated %d ports, want 11", len(seen))
	}

	if len(errs) != 9 {
		t.Errorf("expected 9 errors from pool exhaustion, got %d", len(errs))
	}
}

func TestCaddyAdapterServerNameDiscovery(t *testing.T) {
	tests := []struct {
		name        string
		serversBody string
		wantName    string
		wantErr     bool
	}{
		{
			name:        "single server",
			serversBody: `{"srv0":{}}`,
			wantName:    "srv0",
		},
		{
			name:        "no servers",
			serversBody: `{}`,
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux := http.NewServeMux()
			mux.HandleFunc("/config/apps/http/servers", func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(tt.serversBody))
			})

			ln, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatalf("listen: %v", err)
			}
			defer ln.Close()

			srv := &http.Server{Handler: mux}
			go srv.Serve(ln)
			defer srv.Close()

			adminURL := fmt.Sprintf("http://%s", ln.Addr().String())
			ca := NewCaddyAdapter(CaddyConfig{
				AdminURL: adminURL,
			})

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			name, err := ca.getServerName(ctx)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("getServerName: %v", err)
			}
			if name != tt.wantName {
				t.Errorf("getServerName() = %q, want %q", name, tt.wantName)
			}

			// Second call should use cached value (no mock server needed)
			name2, err := ca.getServerName(ctx)
			if err != nil {
				t.Fatalf("getServerName second call: %v", err)
			}
			if name2 != tt.wantName {
				t.Errorf("cached getServerName() = %q, want %q", name2, tt.wantName)
			}
		})
	}
}

func TestCaddyAdapterAddRoute(t *testing.T) {
	mux := http.NewServeMux()
	var routeReq *http.Request
	var routeBody []byte
	mux.HandleFunc("/config/apps/http/servers", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"srv0":{}}`))
	})
	mux.HandleFunc("/config/apps/http/servers/srv0/routes/", func(w http.ResponseWriter, r *http.Request) {
		routeReq = r
		routeBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	})

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	srv := &http.Server{Handler: mux}
	go srv.Serve(ln)
	defer srv.Close()

	adminURL := fmt.Sprintf("http://%s", ln.Addr().String())
	ca := NewCaddyAdapter(CaddyConfig{
		AdminURL: adminURL,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err = ca.AddRoute(ctx, "test.mesh.local", "127.0.0.1", 8080)
	if err != nil {
		t.Fatalf("AddRoute: %v", err)
	}

	if routeReq == nil {
		t.Fatal("no request received by mock server")
	}
	if routeReq.Method != http.MethodPost {
		t.Errorf("method = %q, want POST", routeReq.Method)
	}
	_ = routeBody
}

func TestCaddyAdapterRemoveRoute(t *testing.T) {
	mux := http.NewServeMux()
	var deletePath string
	mux.HandleFunc("/config/apps/http/servers", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"srv0":{}}`))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			deletePath = r.URL.Path
		}
		w.WriteHeader(http.StatusOK)
	})

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	srv := &http.Server{Handler: mux}
	go srv.Serve(ln)
	defer srv.Close()

	adminURL := fmt.Sprintf("http://%s", ln.Addr().String())
	ca := NewCaddyAdapter(CaddyConfig{
		AdminURL: adminURL,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err = ca.RemoveRoute(ctx, "test.mesh.local")
	if err != nil {
		t.Fatalf("RemoveRoute: %v", err)
	}

	if deletePath == "" {
		t.Fatal("no DELETE request received by mock server")
	}
}

func TestCaddyAdapterListRoutes(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/config/apps/http/servers", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"srv0":{}}`))
	})
	mux.HandleFunc("/config/apps/http/servers/srv0/routes", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[
			{"@id": "route1", "match": [{"host": ["test1.mesh.local"]}], "handle": [{"handler": "reverse_proxy", "upstreams": [{"dial": "127.0.0.1:8080"}]}]},
			{"@id": "route2", "match": [{"host": ["test2.mesh.local"]}], "handle": [{"handler": "reverse_proxy", "upstreams": [{"dial": "127.0.0.1:8081"}]}]}
		]`))
	})

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	srv := &http.Server{Handler: mux}
	go srv.Serve(ln)
	defer srv.Close()

	adminURL := fmt.Sprintf("http://%s", ln.Addr().String())
	ca := NewCaddyAdapter(CaddyConfig{
		AdminURL: adminURL,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	routes, err := ca.ListRoutes(ctx)
	if err != nil {
		t.Fatalf("ListRoutes: %v", err)
	}

	if len(routes) != 2 {
		t.Errorf("len(routes) = %d, want 2", len(routes))
	}
}

func TestCaddyAdapterConnectionError(t *testing.T) {
	ca := NewCaddyAdapter(CaddyConfig{
		AdminURL: "http://127.0.0.1:1",
	})

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := ca.AddRoute(ctx, "test.mesh.local", "127.0.0.1", 8080)
	if err == nil {
		t.Fatal("expected error for connection failure, got nil")
	}
}

var _ IngressAdapter = (*CaddyAdapter)(nil)
