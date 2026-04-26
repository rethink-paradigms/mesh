// Package daemon implements the long-running Mesh daemon process with signal
// handling, PID file management, startup reconciliation, health checks,
// and graceful shutdown orchestration.
package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/rethink-paradigms/mesh/internal/adapter"
	"github.com/rethink-paradigms/mesh/internal/config"
	"github.com/rethink-paradigms/mesh/internal/store"
)

// Daemon is the long-running mesh process that manages bodies and exposes MCP tools.
type Daemon struct {
	cfg   *config.Config
	store *store.Store

	docker adapter.SubstrateAdapter

	mcpServer   interface{ Stop(context.Context) error }
	mcpServerMu sync.Mutex

	sigs      chan os.Signal
	done      chan struct{}
	doneOnce  sync.Once
	startedAt time.Time

	mu         sync.RWMutex
	ready      bool
	httpServer *http.Server
	httpAddr   string
}

// New creates a new Daemon from the given config.
func New(cfg *config.Config) (*Daemon, error) {
	d := &Daemon{
		cfg:       cfg,
		sigs:      make(chan os.Signal, 1),
		done:      make(chan struct{}),
		startedAt: time.Now(),
	}
	return d, nil
}

// SetMCP injects an MCP server. Called by whoever wires the daemon together (Task 9).
func (d *Daemon) SetMCP(srv interface{ Stop(context.Context) error }) {
	d.mcpServerMu.Lock()
	defer d.mcpServerMu.Unlock()
	d.mcpServer = srv
}

func (d *Daemon) HTTPAddr() string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.httpAddr
}

func (d *Daemon) Ready() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.ready
}

// Done returns a channel that is closed when the daemon has fully stopped.
func (d *Daemon) Done() <-chan struct{} {
	return d.done
}

// Start begins the daemon's main loop. It opens the store, runs reconciliation,
// writes the PID file, starts the health server, registers signal handlers, and
// blocks until a termination signal or context cancellation.
func (d *Daemon) Start(ctx context.Context) error {
	// 1. Open SQLite store
	s, err := store.Open(d.cfg.Store.Path)
	if err != nil {
		return fmt.Errorf("daemon: open store: %w", err)
	}
	d.store = s

	// 2. Startup reconciliation
	if err := d.reconcile(ctx); err != nil {
		return fmt.Errorf("daemon: reconcile: %w", err)
	}

	// 3. Write PID file
	if err := d.writePIDFile(); err != nil {
		return fmt.Errorf("daemon: write PID: %w", err)
	}
	defer d.removePIDFile()

	// 4. Start health check HTTP server
	if err := d.startHealthServer(); err != nil {
		return fmt.Errorf("daemon: health server: %w", err)
	}
	defer d.stopHealthServer()

	// 5. Register signal handlers
	signal.Notify(d.sigs, syscall.SIGTERM, syscall.SIGINT)

	d.mu.Lock()
	d.ready = true
	d.mu.Unlock()

	// 6. Block until signal or context cancellation
	select {
	case sig := <-d.sigs:
		fmt.Fprintf(os.Stderr, "daemon: received signal %v, shutting down\n", sig)
	case <-ctx.Done():
	}

	// 7. Graceful shutdown
	stopCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return d.Stop(stopCtx)
}

// Stop performs graceful shutdown: stops MCP server, closes the store,
// removes PID file, and signals completion.
func (d *Daemon) Stop(ctx context.Context) error {
	d.mu.Lock()
	d.ready = false
	d.mu.Unlock()
	signal.Reset(syscall.SIGTERM, syscall.SIGINT)

	// Stop MCP server if set
	d.mcpServerMu.Lock()
	if d.mcpServer != nil {
		d.mcpServer.Stop(ctx)
	}
	d.mcpServerMu.Unlock()

	// Close store
	if d.store != nil {
		d.store.Close()
	}

	d.removePIDFile()

	d.doneOnce.Do(func() { close(d.done) })
	return nil
}

// reconcile checks stored body state against actual substrate state.
// For now this is a stub — full implementation when Docker adapter is wired.
func (d *Daemon) reconcile(ctx context.Context) error {
	bodies, err := d.store.ListBodies(ctx)
	if err != nil {
		return fmt.Errorf("reconcile: list bodies: %w", err)
	}
	for _, b := range bodies {
		// TODO: Check if instance_id is set and Docker container exists.
		// If container missing but state says Running → set to Error.
		_ = b
	}
	return nil
}

// --- Health server ---

func (d *Daemon) startHealthServer() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", d.handleHealth)

	addr := "127.0.0.1:0"
	srv := &http.Server{Addr: addr, Handler: mux}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("health listen: %w", err)
	}

	d.mu.Lock()
	d.httpServer = srv
	d.httpAddr = ln.Addr().String()
	d.mu.Unlock()

	go srv.Serve(ln)
	return nil
}

func (d *Daemon) stopHealthServer() {
	if d.httpServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		d.httpServer.Shutdown(ctx)
	}
}

func (d *Daemon) handleHealth(w http.ResponseWriter, r *http.Request) {
	d.mu.RLock()
	isReady := d.ready
	d.mu.RUnlock()

	if !isReady {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "not ready"})
		return
	}

	resp := map[string]interface{}{
		"status":     "ok",
		"uptime_sec": int(time.Since(d.startedAt).Seconds()),
	}

	if d.store != nil {
		bodies, err := d.store.ListBodies(r.Context())
		if err == nil {
			resp["bodies"] = len(bodies)
		}
	}

	if r.URL.Query().Get("verbose") == "true" {
		resp["config"] = d.cfg
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

// --- PID file ---

func (d *Daemon) writePIDFile() error {
	if d.cfg.Daemon.PIDFile == "" {
		return nil
	}
	return os.WriteFile(d.cfg.Daemon.PIDFile, []byte(fmt.Sprintf("%d", os.Getpid())), 0644)
}

func (d *Daemon) removePIDFile() {
	if d.cfg.Daemon.PIDFile != "" {
		os.Remove(d.cfg.Daemon.PIDFile)
	}
}
