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
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/rethink-paradigms/mesh/internal/adapter"
	"github.com/rethink-paradigms/mesh/internal/body"
	"github.com/rethink-paradigms/mesh/internal/config"
	"github.com/rethink-paradigms/mesh/internal/orchestrator"
	"github.com/rethink-paradigms/mesh/internal/plugin"
	"github.com/rethink-paradigms/mesh/internal/store"
)

// Daemon is the long-running mesh process that manages bodies and exposes MCP tools.
type Daemon struct {
	cfg   *config.Config
	store *store.Store

	adapters    *adapter.MultiAdapter
	bodyMgr     *body.BodyManager
	pluginMgr   *plugin.PluginManager

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

	reconcileSteps int
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

// Adapters returns the daemon's multi-adapter router.
func (d *Daemon) Adapters() *adapter.MultiAdapter {
	return d.adapters
}

// BodyManager returns the daemon's body manager.
func (d *Daemon) BodyManager() *body.BodyManager {
	return d.bodyMgr
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

// Start begins the daemon's main loop. It opens the store, initializes adapters,
// writes the PID file, starts the health server, registers signal handlers, and
// blocks until a termination signal or context cancellation.
func (d *Daemon) Start(ctx context.Context) error {
	// 1. Check PID file for conflicts
	if err := d.checkPIDConflict(); err != nil {
		return fmt.Errorf("daemon: PID conflict: %w", err)
	}

	// 2. Open SQLite store
	s, err := store.Open(d.cfg.Store.Path)
	if err != nil {
		return fmt.Errorf("daemon: open store: %w", err)
	}
	d.store = s

	// 3. Initialize MultiAdapter (adapters registered by plugin system)
	multi := adapter.NewMultiAdapter()
	d.adapters = multi

	// 4. Initialize BodyManager
	d.bodyMgr = body.NewBodyManager(d.store, &multiAdapterOrchestrator{multi: d.adapters})

	// 5. Initialize PluginManager
	pm := plugin.NewPluginManager(d.cfg.Plugin.Dir, d.cfg.Plugin.Enabled)
	if err := pm.StartScanAndLoad(); err != nil {
		fmt.Fprintf(os.Stderr, "daemon: plugin scan and load: %v\n", err)
	}
	pm.StartHealthChecks()
	d.pluginMgr = pm

	// 6. Startup reconciliation
	if err := d.reconcile(ctx); err != nil {
		return fmt.Errorf("daemon: reconcile: %w", err)
	}

	// 7. Write PID file
	if err := d.writePIDFile(); err != nil {
		return fmt.Errorf("daemon: write PID: %w", err)
	}
	defer d.removePIDFile()

	// 8. Start health check HTTP server
	if err := d.startHealthServer(); err != nil {
		return fmt.Errorf("daemon: health server: %w", err)
	}
	defer d.stopHealthServer()

	// 9. Register signal handlers
	signal.Notify(d.sigs, syscall.SIGTERM, syscall.SIGINT)

	d.mu.Lock()
	d.ready = true
	d.mu.Unlock()

	// 10. Block until signal or context cancellation
	select {
	case sig := <-d.sigs:
		fmt.Fprintf(os.Stderr, "daemon: received signal %v, shutting down\n", sig)
	case <-ctx.Done():
	}

	// 10. Graceful shutdown
	stopCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return d.Stop(stopCtx)
}

// PluginManager returns the daemon's plugin manager.
func (d *Daemon) PluginManager() *plugin.PluginManager {
	return d.pluginMgr
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

	// Stop PluginManager
	if d.pluginMgr != nil {
		d.pluginMgr.Stop()
	}

	// Close store
	if d.store != nil {
		d.store.Close()
	}

	d.removePIDFile()

	d.doneOnce.Do(func() { close(d.done) })
	return nil
}

func (d *Daemon) reconcile(ctx context.Context) error {
	bodies, err := d.store.ListBodies(ctx)
	if err != nil {
		return fmt.Errorf("reconcile: list bodies: %w", err)
	}

	for _, rec := range bodies {
		if rec.InstanceID == "" {
			continue
		}

		adp, err := d.adapters.GetAdapter(rec.Substrate)
		if err != nil {
			continue
		}

		_, err = adp.GetStatus(ctx, adapter.Handle(rec.InstanceID))
		containerExists := err == nil

		switch rec.State {
		case adapter.StateRunning, adapter.StateStarting, adapter.StateStopping:
			if !containerExists {
				if transErr := d.bodyMgr.TransitionBody(ctx, rec.ID, adapter.StateError); transErr != nil {
					fmt.Fprintf(os.Stderr, "reconcile: failed to transition body %s to Error: %v\n", rec.ID, transErr)
				} else {
					fmt.Fprintf(os.Stderr, "reconcile: body %s container not found, transitioned to Error\n", rec.ID)
				}
				d.mu.Lock()
				d.reconcileSteps++
				d.mu.Unlock()
			}

		case adapter.StateError:
			if containerExists {
				status, _ := adp.GetStatus(ctx, adapter.Handle(rec.InstanceID))
				if status.State == adapter.StateRunning {
					if transErr := d.bodyMgr.TransitionBody(ctx, rec.ID, adapter.StateRunning); transErr != nil {
						fmt.Fprintf(os.Stderr, "reconcile: failed to transition body %s to Running: %v\n", rec.ID, transErr)
					} else {
						fmt.Fprintf(os.Stderr, "reconcile: body %s verified running, transitioned to Running\n", rec.ID)
					}
					d.mu.Lock()
					d.reconcileSteps++
					d.mu.Unlock()
				}
			}

		case adapter.StateMigrating:
			if !d.hasActiveMigration(ctx, rec.ID) {
				if transErr := d.bodyMgr.TransitionBody(ctx, rec.ID, adapter.StateError); transErr != nil {
					fmt.Fprintf(os.Stderr, "reconcile: failed to transition body %s from Migrating to Error: %v\n", rec.ID, transErr)
				} else {
					fmt.Fprintf(os.Stderr, "reconcile: body %s migration record missing, transitioned to Error\n", rec.ID)
				}
				d.mu.Lock()
				d.reconcileSteps++
				d.mu.Unlock()
			}
		}
	}

	return nil
}

func (d *Daemon) hasActiveMigration(ctx context.Context, bodyID string) bool {
	var count int
	err := d.store.QueryRow(ctx, `SELECT COUNT(*) FROM migrations WHERE body_id = ? AND error = ''`, bodyID).Scan(&count)
	if err != nil {
		return false
	}
	return count > 0
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

	d.mu.RLock()
	steps := d.reconcileSteps
	d.mu.RUnlock()
	resp["reconcile_steps"] = steps

	if r.URL.Query().Get("verbose") == "true" {
		resp["config"] = d.cfg
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

// --- PID file ---

func (d *Daemon) checkPIDConflict() error {
	if d.cfg.Daemon.PIDFile == "" {
		return nil
	}
	data, err := os.ReadFile(d.cfg.Daemon.PIDFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read PID file: %w", err)
	}
	pid, err := strconv.Atoi(string(data))
	if err != nil {
		// Corrupt PID file — overwrite it.
		return nil
	}
	if pid == os.Getpid() {
		return nil
	}
	// Check if process is still alive.
	proc, err := os.FindProcess(pid)
	if err != nil {
		// Cannot find process — safe to start.
		return nil
	}
	if err := proc.Signal(syscall.Signal(0)); err != nil {
		// Process not alive — safe to start.
		return nil
	}
	return fmt.Errorf("daemon already running (pid %d)", pid)
}

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

// multiAdapterOrchestrator wraps *adapter.MultiAdapter to satisfy orchestrator.OrchestratorAdapter.
// This is a temporary bridge until Task 11 fully rewires the daemon.
type multiAdapterOrchestrator struct {
	multi *adapter.MultiAdapter
}

func (m *multiAdapterOrchestrator) ScheduleBody(ctx context.Context, spec orchestrator.BodySpec) (orchestrator.Handle, error) {
	adpSpec := adapter.BodySpec{
		Image:     spec.Image,
		Workdir:   spec.Workdir,
		Env:       spec.Env,
		Cmd:       spec.Cmd,
		MemoryMB:  spec.MemoryMB,
		CPUShares: spec.CPUShares,
	}
	h, err := m.multi.Create(ctx, adpSpec)
	return orchestrator.Handle(h), err
}

func (m *multiAdapterOrchestrator) StartBody(ctx context.Context, id orchestrator.Handle) error {
	return m.multi.Start(ctx, adapter.Handle(id))
}

func (m *multiAdapterOrchestrator) StopBody(ctx context.Context, id orchestrator.Handle) error {
	return m.multi.Stop(ctx, adapter.Handle(id), adapter.StopOpts{})
}

func (m *multiAdapterOrchestrator) DestroyBody(ctx context.Context, id orchestrator.Handle) error {
	return m.multi.Destroy(ctx, adapter.Handle(id))
}

func (m *multiAdapterOrchestrator) GetBodyStatus(ctx context.Context, id orchestrator.Handle) (orchestrator.BodyStatus, error) {
	s, err := m.multi.GetStatus(ctx, adapter.Handle(id))
	if err != nil {
		return orchestrator.BodyStatus{}, err
	}
	return orchestrator.BodyStatus{
		State:      orchestrator.BodyState(s.State),
		Uptime:     s.Uptime,
		MemoryMB:   s.MemoryMB,
		CPUPercent: s.CPUPercent,
		StartedAt:  s.StartedAt,
	}, nil
}

func (m *multiAdapterOrchestrator) Name() string {
	return m.multi.SubstrateName()
}

func (m *multiAdapterOrchestrator) IsHealthy(ctx context.Context) bool {
	return m.multi.IsHealthy(ctx)
}
