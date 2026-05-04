package daemon

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/rethink-paradigms/mesh/internal/api"
	"github.com/rethink-paradigms/mesh/internal/body"
	"github.com/rethink-paradigms/mesh/internal/config"
	"github.com/rethink-paradigms/mesh/internal/ingress"
	"github.com/rethink-paradigms/mesh/internal/nomad"
	"github.com/rethink-paradigms/mesh/internal/orchestrator"
	"github.com/rethink-paradigms/mesh/internal/plugin"
	"github.com/rethink-paradigms/mesh/internal/provisioner"
	"github.com/rethink-paradigms/mesh/internal/store"
)

type Daemon struct {
	cfg   *config.Config
	store *store.Store

	orchRegistry *orchestrator.Registry
	provRegistry *provisioner.Registry
	bodyMgr      *body.BodyManager
	pluginMgr    *plugin.PluginManager

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
	version        string
}

func New(cfg *config.Config) (*Daemon, error) {
	d := &Daemon{
		cfg:       cfg,
		sigs:      make(chan os.Signal, 1),
		done:      make(chan struct{}),
		startedAt: time.Now(),
		version:   "dev",
	}
	return d, nil
}

func (d *Daemon) OrchRegistry() *orchestrator.Registry {
	return d.orchRegistry
}

func (d *Daemon) ProvRegistry() *provisioner.Registry {
	return d.provRegistry
}

func (d *Daemon) BodyManager() *body.BodyManager {
	return d.bodyMgr
}

func (d *Daemon) SetMCP(srv interface{ Stop(context.Context) error }) {
	d.mcpServerMu.Lock()
	defer d.mcpServerMu.Unlock()
	d.mcpServer = srv
}

func (d *Daemon) SetVersion(v string) {
	d.version = v
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

func (d *Daemon) Done() <-chan struct{} {
	return d.done
}

func (d *Daemon) Start(ctx context.Context) error {
	if err := d.checkPIDConflict(); err != nil {
		return fmt.Errorf("daemon: PID conflict: %w", err)
	}

	s, err := store.Open(d.cfg.Store.Path)
	if err != nil {
		return fmt.Errorf("daemon: open store: %w", err)
	}
	d.store = s

	orchRegistry := orchestrator.NewRegistry()
	d.orchRegistry = orchRegistry

	for name, settings := range d.cfg.Orchestrators {
		switch name {
		case "nomad":
			adp := nomad.New(nomad.Config{
				Address:   settings["address"],
				Token:     settings["token"],
				Region:    settings["region"],
				Namespace: settings["namespace"],
			})
			if err := orchRegistry.Register("nomad", adp); err != nil {
				fmt.Fprintf(os.Stderr, "daemon: register orchestrator %q: %v\n", name, err)
			}
		}
	}

	if len(orchRegistry.List()) == 0 {
		fmt.Fprintf(os.Stderr, "daemon: warning: no orchestrators registered\n")
	}

	provRegistry := provisioner.NewRegistry()
	d.provRegistry = provRegistry

	if len(d.cfg.Provisioners) == 0 {
		fmt.Fprintf(os.Stderr, "daemon: info: no provisioners registered\n")
	}

	var primaryOrch orchestrator.OrchestratorAdapter
	if names := orchRegistry.List(); len(names) > 0 {
		primaryOrch, _ = orchRegistry.Open(names[0])
	}
	if primaryOrch == nil {
		primaryOrch = &noopOrchestrator{}
	}
	d.bodyMgr = body.NewBodyManager(d.store, primaryOrch)

	pm := plugin.NewPluginManager(d.cfg.Plugin.Dir, d.cfg.Plugin.Enabled)
	if err := pm.StartScanAndLoad(); err != nil {
		fmt.Fprintf(os.Stderr, "daemon: plugin scan and load: %v\n", err)
	}
	pm.StartHealthChecks()
	d.pluginMgr = pm

	if err := d.reconcile(ctx); err != nil {
		return fmt.Errorf("daemon: reconcile: %w", err)
	}

	if err := d.writePIDFile(); err != nil {
		return fmt.Errorf("daemon: write PID: %w", err)
	}
	defer d.removePIDFile()

	if d.cfg.Daemon.AuthToken == "" {
		fmt.Fprintf(os.Stderr, "WARNING: auth_token not set, API endpoints are unprotected\n")
	}

	if err := d.startAPIServer(); err != nil {
		return fmt.Errorf("daemon: API server: %w", err)
	}
	defer d.stopAPIServer()

	fmt.Fprintf(os.Stderr, "daemon: API server listening on %s\n", d.httpAddr)

	signal.Notify(d.sigs, syscall.SIGTERM, syscall.SIGINT)

	d.mu.Lock()
	d.ready = true
	d.mu.Unlock()

	select {
	case sig := <-d.sigs:
		fmt.Fprintf(os.Stderr, "daemon: received signal %v, shutting down\n", sig)
	case <-ctx.Done():
	}

	stopCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return d.Stop(stopCtx)
}

func (d *Daemon) PluginManager() *plugin.PluginManager {
	return d.pluginMgr
}

func (d *Daemon) Stop(ctx context.Context) error {
	d.mu.Lock()
	d.ready = false
	d.mu.Unlock()
	signal.Reset(syscall.SIGTERM, syscall.SIGINT)

	d.stopAPIServer()

	d.mcpServerMu.Lock()
	if d.mcpServer != nil {
		if err := d.mcpServer.Stop(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "daemon: mcp stop: %v\n", err)
		}
	}
	d.mcpServerMu.Unlock()

	if d.pluginMgr != nil {
		d.pluginMgr.Stop()
	}

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

		adp, err := d.orchRegistry.Open(rec.Substrate)
		if err != nil {
			switch rec.State {
			case orchestrator.StateRunning, orchestrator.StateStarting, orchestrator.StateStopping:
				fmt.Fprintf(os.Stderr, "reconcile: body %s substrate %q not found, transitioning to Error\n", rec.ID, rec.Substrate)
				if transErr := d.bodyMgr.TransitionBody(ctx, rec.ID, orchestrator.StateError); transErr != nil {
					fmt.Fprintf(os.Stderr, "reconcile: failed to transition body %s to Error: %v\n", rec.ID, transErr)
				}
				d.mu.Lock()
				d.reconcileSteps++
				d.mu.Unlock()
			default:
				fmt.Fprintf(os.Stderr, "reconcile: body %s substrate %q not found, skipping\n", rec.ID, rec.Substrate)
			}
			continue
		}

		_, err = adp.GetBodyStatus(ctx, orchestrator.Handle(rec.InstanceID))
		containerExists := err == nil

		switch rec.State {
		case orchestrator.StateRunning, orchestrator.StateStarting, orchestrator.StateStopping:
			if !containerExists {
				if transErr := d.bodyMgr.TransitionBody(ctx, rec.ID, orchestrator.StateError); transErr != nil {
					fmt.Fprintf(os.Stderr, "reconcile: failed to transition body %s to Error: %v\n", rec.ID, transErr)
				} else {
					fmt.Fprintf(os.Stderr, "reconcile: body %s container not found, transitioned to Error\n", rec.ID)
				}
				d.mu.Lock()
				d.reconcileSteps++
				d.mu.Unlock()
			}

		case orchestrator.StateError:
			if containerExists {
				status, _ := adp.GetBodyStatus(ctx, orchestrator.Handle(rec.InstanceID))
				if status.State == orchestrator.StateRunning {
					if transErr := d.bodyMgr.TransitionBody(ctx, rec.ID, orchestrator.StateRunning); transErr != nil {
						fmt.Fprintf(os.Stderr, "reconcile: failed to transition body %s to Running: %v\n", rec.ID, transErr)
					} else {
						fmt.Fprintf(os.Stderr, "reconcile: body %s verified running, transitioned to Running\n", rec.ID)
					}
					d.mu.Lock()
					d.reconcileSteps++
					d.mu.Unlock()
				}
			}

		case orchestrator.StateMigrating:
			if !d.hasActiveMigration(ctx, rec.ID) {
				if transErr := d.bodyMgr.TransitionBody(ctx, rec.ID, orchestrator.StateError); transErr != nil {
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

func (d *Daemon) startAPIServer() error {
	var primaryOrch orchestrator.OrchestratorAdapter
	if names := d.orchRegistry.List(); len(names) > 0 {
		primaryOrch, _ = d.orchRegistry.Open(names[0])
	}
	if primaryOrch == nil {
		primaryOrch = &noopOrchestrator{}
	}

	router := api.NewRouter(api.RouterConfig{
		BodyManager:  d.bodyMgr,
		Store:        d.store,
		Orchestrator: primaryOrch,
		Ingress:      ingress.NewNoopAdapter(),
		AuthToken:    d.cfg.Daemon.AuthToken,
		Version:      d.version,
	})

	listenAddr := d.cfg.Daemon.ListenAddr
	if listenAddr == "" {
		listenAddr = "127.0.0.1:8080"
	}

	srv := &http.Server{Addr: listenAddr, Handler: router}

	ln, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return fmt.Errorf("api listen on %s: %w", listenAddr, err)
	}

	d.mu.Lock()
	d.httpServer = srv
	d.httpAddr = ln.Addr().String()
	d.mu.Unlock()

	go srv.Serve(ln)
	return nil
}

func (d *Daemon) stopAPIServer() {
	if d.httpServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := d.httpServer.Shutdown(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "daemon: api server shutdown: %v\n", err)
		}
	}
}

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
		return nil
	}
	if pid == os.Getpid() {
		return nil
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return nil
	}
	if err := proc.Signal(syscall.Signal(0)); err != nil {
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

type noopOrchestrator struct{}

func (n *noopOrchestrator) ScheduleBody(_ context.Context, _ orchestrator.BodySpec) (orchestrator.Handle, error) {
	return "", fmt.Errorf("no orchestrator configured")
}

func (n *noopOrchestrator) StartBody(_ context.Context, _ orchestrator.Handle) error {
	return fmt.Errorf("no orchestrator configured")
}

func (n *noopOrchestrator) StopBody(_ context.Context, _ orchestrator.Handle) error {
	return fmt.Errorf("no orchestrator configured")
}

func (n *noopOrchestrator) DestroyBody(_ context.Context, _ orchestrator.Handle) error {
	return fmt.Errorf("no orchestrator configured")
}

func (n *noopOrchestrator) GetBodyStatus(_ context.Context, _ orchestrator.Handle) (orchestrator.BodyStatus, error) {
	return orchestrator.BodyStatus{}, fmt.Errorf("no orchestrator configured")
}

func (n *noopOrchestrator) Name() string { return "noop" }

func (n *noopOrchestrator) IsHealthy(_ context.Context) bool { return false }
