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

	"github.com/rethink-paradigms/mesh/internal/agent"
	"github.com/rethink-paradigms/mesh/internal/api"
	"github.com/rethink-paradigms/mesh/internal/body"
	"github.com/rethink-paradigms/mesh/internal/config"
	"github.com/rethink-paradigms/mesh/internal/docker"
	"github.com/rethink-paradigms/mesh/internal/heartbeat"
	"github.com/rethink-paradigms/mesh/internal/ingress"
	"github.com/rethink-paradigms/mesh/internal/nomad"
	"github.com/rethink-paradigms/mesh/internal/orchestrator"
	"github.com/rethink-paradigms/mesh/internal/plugin"
	"github.com/rethink-paradigms/mesh/internal/provisioner"
	"github.com/rethink-paradigms/mesh/internal/service"
	"github.com/rethink-paradigms/mesh/internal/store"
	"github.com/rethink-paradigms/mesh/internal/version"
)

type Daemon struct {
	cfg   *config.Config
	store *store.Store

	orchRegistry *orchestrator.Registry
	provRegistry *provisioner.Registry
	bodyMgr      *body.BodyManager
	bodySvc      *service.BodyService
	pluginMgr    *plugin.PluginManager
	installer    *agent.Installer

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
	tier           string

	ingress ingress.IngressAdapter

	heartbeat *heartbeat.Client
}

func New(cfg *config.Config) (*Daemon, error) {
	d := &Daemon{
		cfg:       cfg,
		sigs:      make(chan os.Signal, 1),
		done:      make(chan struct{}),
		startedAt: time.Now(),
		version:   version.Version,
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
	if d.ingress != nil {
		if is, ok := srv.(interface{ SetIngress(ingress.IngressAdapter) }); ok {
			is.SetIngress(d.ingress)
		}
	}
	if d.cfg != nil && d.cfg.Daemon.Auth0Domain != "" && d.cfg.Daemon.Auth0Audience != "" {
		if ms, ok := srv.(interface {
			SetAuth(string, string, string) error
		}); ok {
			if err := ms.SetAuth(d.cfg.Daemon.Auth0Domain, d.cfg.Daemon.Auth0Audience, d.cfg.Daemon.ClusterOwnerID); err != nil {
				fmt.Fprintf(os.Stderr, "daemon: mcp set auth: %v\n", err)
			}
		}
	}
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

	dockerAdp := docker.New(docker.Config{})
	if err := orchRegistry.Register("docker", dockerAdp); err != nil {
		fmt.Fprintf(os.Stderr, "daemon: register docker orchestrator: %v\n", err)
	}

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

	var primaryOrch orchestrator.OrchestratorAdapter
	if nomadAdp, err := orchRegistry.Open("nomad"); err == nil && nomadAdp.IsHealthy(ctx) {
		d.tier = "STANDARD"
		_ = orchRegistry.SetDefault("nomad")
		primaryOrch = nomadAdp
	} else {
		d.tier = "LITE"
		_ = orchRegistry.SetDefault("docker")
		primaryOrch = dockerAdp
	}

	provRegistry := provisioner.NewRegistry()
	d.provRegistry = provRegistry

	if len(d.cfg.Provisioners) == 0 {
		fmt.Fprintf(os.Stderr, "daemon: info: no provisioners registered\n")
	}

	d.bodyMgr = body.NewBodyManager(d.store, primaryOrch)
	d.bodySvc = service.NewBodyService(d.bodyMgr, d.store, d.orchRegistry)

	pm := plugin.NewPluginManager(d.cfg.Plugin.Dir, d.cfg.Plugin.Enabled)
	if err := pm.StartScanAndLoad(); err != nil {
		fmt.Fprintf(os.Stderr, "daemon: plugin scan and load: %v\n", err)
	}
	pm.StartHealthChecks()
	d.pluginMgr = pm

	if err := d.reconcile(ctx); err != nil {
		return fmt.Errorf("daemon: reconcile: %w", err)
	}

	// Auto-detect Caddy ingress adapter when not explicitly configured
	if d.cfg.Ingress.Adapter == "" || d.cfg.Ingress.Adapter == "noop" {
		if caddyDetected() {
			if d.cfg.Ingress.AdminURL == "" {
				d.cfg.Ingress.AdminURL = "http://127.0.0.1:2019"
			}
			d.cfg.Ingress.Adapter = "caddy"
			fmt.Fprintf(os.Stderr, "daemon: info: caddy detected at %s, using caddy ingress adapter\n", d.cfg.Ingress.AdminURL)
		}
	}

	// Create ingress adapter early so both installer and API server share it
	d.ingress = d.createIngressAdapter()

	// Wire agent installer from agents directory
	if d.cfg.AgentsDir != "" {
		manifests, err := agent.LoadManifestDir(d.cfg.AgentsDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "daemon: load agent manifests from %s: %v\n", d.cfg.AgentsDir, err)
			// Not fatal — daemon can operate without agent installer
		} else if len(manifests) > 0 {
			d.installer = agent.NewInstaller(d.bodyMgr, d.ingress, d.orchRegistry, manifests)
		} else {
			fmt.Fprintf(os.Stderr, "daemon: info: no agent manifests found in %s\n", d.cfg.AgentsDir)
		}
	}

	if err := d.writePIDFile(); err != nil {
		return fmt.Errorf("daemon: write PID: %w", err)
	}
	defer d.removePIDFile()

	// Validate authentication configuration based on auth_mode
	switch d.cfg.Daemon.AuthMode {
	case "jwt", "both":
		if d.cfg.Daemon.Auth0Domain == "" || d.cfg.Daemon.Auth0Audience == "" {
			return fmt.Errorf("daemon: auth_mode %q requires auth0_domain and auth0_audience in config", d.cfg.Daemon.AuthMode)
		}
		if d.cfg.Daemon.AuthToken == "" && d.cfg.Daemon.AuthMode == "both" {
			return fmt.Errorf("daemon: auth_mode %q requires auth_token and auth0 config", d.cfg.Daemon.AuthMode)
		}
	case "token", "":
		if d.cfg.Daemon.AuthToken == "" {
			return fmt.Errorf("daemon: auth_token is not set in config — refusing to start with unprotected API")
		}
	default:
		return fmt.Errorf("daemon: auth_mode %q is invalid", d.cfg.Daemon.AuthMode)
	}

	if err := d.startAPIServer(); err != nil {
		return fmt.Errorf("daemon: API server: %w", err)
	}
	defer d.stopAPIServer()

	fmt.Fprintf(os.Stderr, "daemon: API server listening on %s\n", d.httpAddr)

	// Start heartbeat goroutine if gateway URL is configured
	if d.cfg.Daemon.GatewayURL != "" && d.cfg.Daemon.HeartbeatIntervalSeconds > 0 {
		if d.cfg.Daemon.AuthMode == "jwt" && d.cfg.Daemon.AuthToken == "" {
			fmt.Fprintf(os.Stderr, "daemon: heartbeat disabled: JWT-only mode requires auth_token for heartbeat\n")
		} else {
			var orchName string
			if defOrch, err := d.orchRegistry.Default(); err == nil {
				orchName = defOrch.Name()
			}
			d.heartbeat = heartbeat.NewClient(
				d.cfg.Daemon.GatewayURL,
				d.cfg.Daemon.AuthToken,
				d.cfg.Daemon.AuthMode,
				d.cfg.Daemon.ClusterID,
				d.version,
				d.tier,
				orchName,
				func() int { return d.bodyMgr.Count() },
			)
			interval := time.Duration(d.cfg.Daemon.HeartbeatIntervalSeconds) * time.Second
			d.heartbeat.Start(ctx, interval)
			fmt.Fprintf(os.Stderr, "daemon: heartbeat started to %s every %v\n", d.cfg.Daemon.GatewayURL, interval)
		}
	}

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

func (d *Daemon) createIngressAdapter() ingress.IngressAdapter {
	switch d.cfg.Ingress.Adapter {
	case "caddy":
		return ingress.NewCaddyAdapter(ingress.CaddyConfig{
			AdminURL:      d.cfg.Ingress.AdminURL,
			PortPoolStart: d.cfg.Ingress.PortPoolStart,
			PortPoolEnd:   d.cfg.Ingress.PortPoolEnd,
			DomainSuffix:  d.cfg.Ingress.DomainSuffix,
		})
	case "noop", "":
		return ingress.NewNoopAdapter()
	default:
		fmt.Fprintf(os.Stderr, "daemon: unknown ingress adapter %q, falling back to noop\n", d.cfg.Ingress.Adapter)
		return ingress.NewNoopAdapter()
	}
}

func (d *Daemon) startAPIServer() error {
	var primaryOrch orchestrator.OrchestratorAdapter
	if adp, err := d.orchRegistry.Default(); err == nil {
		primaryOrch = adp
	}

	if d.ingress == nil {
		d.ingress = d.createIngressAdapter()
	}

	router := api.NewRouter(api.RouterConfig{
		BodyManager:              d.bodyMgr,
		BodyService:              d.bodySvc,
		Store:                    d.store,
		Orchestrator:             primaryOrch,
		Ingress:                  d.ingress,
		AuthToken:                d.cfg.Daemon.AuthToken,
		AuthMode:                 d.cfg.Daemon.AuthMode,
		Auth0Domain:              d.cfg.Daemon.Auth0Domain,
		Auth0Audience:            d.cfg.Daemon.Auth0Audience,
		ClusterOwnerID:           d.cfg.Daemon.ClusterOwnerID,
		ClusterID:                d.cfg.Daemon.ClusterID,
		Version:                  d.version,
		Tier:                     d.tier,
		OrchRegistry:             d.orchRegistry,
		Features:                 d.cfg.Features,
		Uptime:                   d.startedAt,
		Installer:                d.installer,
		GatewayURL:               d.cfg.Daemon.GatewayURL,
		HeartbeatIntervalSeconds: d.cfg.Daemon.HeartbeatIntervalSeconds,
	})

	listenAddr := d.cfg.Daemon.ListenAddr
	if listenAddr == "" {
		port := os.Getenv("MESH_PORT")
		if port == "" {
			return fmt.Errorf("MESH_PORT env var is required (or set daemon.listen_addr in config)")
		}
		listenAddr = "127.0.0.1:" + port
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

// caddyDetected checks whether a Caddy instance is running on this host
// by querying the Caddy admin API at http://127.0.0.1:2019/config/.
// Returns true only when the API responds with HTTP 200.
func caddyDetected() bool {
	client := &http.Client{
		Timeout: 2 * time.Second,
	}
	resp, err := client.Get("http://127.0.0.1:2019/config/")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}
