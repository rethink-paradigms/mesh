package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
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
	"github.com/rethink-paradigms/mesh/internal/registry"
	"github.com/rethink-paradigms/mesh/internal/service"
	"github.com/rethink-paradigms/mesh/internal/store"
	"github.com/rethink-paradigms/mesh/internal/version"
)

type Daemon struct {
	cfg   *config.Config
	store *store.Store

	orchRegistry *orchestrator.Registry
	bodyMgr      *body.BodyManager
	bodySvc      *service.BodyService
	pluginMgr    *plugin.PluginManager
	installer    *agent.Installer
	migrator     *body.MigrationCoordinator

	registryMu sync.Mutex
	registry   body.Registry // hot-swappable S3 registry plugin (nil = disabled)

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
	// Wire up the migration coordinator so migrate_body MCP tool works
	if d.migrator != nil {
		if ms, ok := srv.(interface {
			SetMigrator(*body.MigrationCoordinator)
		}); ok {
			ms.SetMigrator(d.migrator)
		}
	}
}

// ─── Registry Management ──────────────────────────────────────────────────

// ConfigureS3 validates S3 credentials and hot-swaps the registry plugin at runtime.
// Implements api.RegistryManager.
func (d *Daemon) ConfigureS3(ctx context.Context, cfg api.S3RegistryConfig) error {
	d.registryMu.Lock()
	defer d.registryMu.Unlock()

	// Create the S3 plugin — validates config is syntactically correct
	p, err := registry.NewS3RegistryPlugin(registry.RegistryConfig{
		Bucket:          cfg.Bucket,
		Region:          cfg.Region,
		Endpoint:        cfg.Endpoint,
		AccessKeyID:     cfg.AccessKeyID,
		SecretAccessKey: cfg.SecretAccessKey,
	})
	if err != nil {
		return fmt.Errorf("invalid S3 config: %w", err)
	}

	d.registry = p
	if d.migrator != nil {
		d.migrator.SetRegistry(p)
	}

	// Persist to store so it survives daemon restart
	if err := d.persistRegistryConfig(ctx, &cfg); err != nil {
		fmt.Fprintf(os.Stderr, "daemon: warn: failed to persist registry config: %v\n", err)
	}

	fmt.Fprintf(os.Stderr, "daemon: S3 registry configured: bucket=%s region=%s\n", cfg.Bucket, cfg.Region)
	return nil
}

// DisconnectS3 clears the registry plugin. Falls back to same-machine migration.
// Implements api.RegistryManager.
func (d *Daemon) DisconnectS3(ctx context.Context) error {
	d.registryMu.Lock()
	defer d.registryMu.Unlock()

	d.registry = nil
	if d.migrator != nil {
		d.migrator.SetRegistry(nil)
	}

	if err := d.clearRegistryConfig(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "daemon: warn: failed to clear persisted registry config: %v\n", err)
	}

	fmt.Fprintf(os.Stderr, "daemon: S3 registry disconnected, fallback to same-machine migration\n")
	return nil
}

// RegistryStatus returns the current registry state.
// Implements api.RegistryManager.
func (d *Daemon) RegistryStatus(_ context.Context) map[string]interface{} {
	d.registryMu.Lock()
	defer d.registryMu.Unlock()

	if d.registry == nil {
		return map[string]interface{}{
			"configured": false,
			"type":       "none",
		}
	}

	// Try to extract bucket/region from the plugin if possible
	// S3RegistryPlugin doesn't expose its config, so we return basic info
	return map[string]interface{}{
		"configured": true,
		"type":       "s3",
		"healthy":    true,
	}
}

// persistRegistryConfig stores the S3 config as JSON in the config table.
func (d *Daemon) persistRegistryConfig(ctx context.Context, cfg *api.S3RegistryConfig) error {
	if d.store == nil {
		return nil
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal registry config: %w", err)
	}
	return d.store.SetConfig(ctx, "registry_s3", string(data))
}

// clearRegistryConfig removes persisted S3 config.
func (d *Daemon) clearRegistryConfig(ctx context.Context) error {
	if d.store == nil {
		return nil
	}
	return d.store.SetConfig(ctx, "registry_s3", "")
}

// restoreRegistryConfig loads a previously persisted S3 config from the store
// and initializes the registry plugin. Called at daemon startup.
func (d *Daemon) restoreRegistryConfig(ctx context.Context) {
	if d.store == nil {
		return
	}
	val, err := d.store.GetConfig(ctx, "registry_s3")
	if err != nil || val == "" {
		return // no persisted config
	}

	var cfg api.S3RegistryConfig
	if err := json.Unmarshal([]byte(val), &cfg); err != nil {
		fmt.Fprintf(os.Stderr, "daemon: warn: invalid persisted registry config: %v\n", err)
		return
	}
	if cfg.Bucket == "" || cfg.Region == "" {
		return // incomplete config, skip
	}

	p, err := registry.NewS3RegistryPlugin(registry.RegistryConfig{
		Bucket:          cfg.Bucket,
		Region:          cfg.Region,
		Endpoint:        cfg.Endpoint,
		AccessKeyID:     cfg.AccessKeyID,
		SecretAccessKey: cfg.SecretAccessKey,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "daemon: warn: failed to restore S3 registry: %v\n", err)
		return
	}

	d.registry = p
	if d.migrator != nil {
		d.migrator.SetRegistry(p)
	}
	fmt.Fprintf(os.Stderr, "daemon: restored S3 registry: bucket=%s region=%s\n", cfg.Bucket, cfg.Region)
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
	if err := config.EnsureDirs(d.cfg); err != nil {
		return fmt.Errorf("daemon: ensure dirs: %w", err)
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
		_ = orchRegistry.SetDefault("nomad")
		primaryOrch = nomadAdp
	} else {
		_ = orchRegistry.SetDefault("docker")
		primaryOrch = dockerAdp
	}

	// Determine tier: explicit config wins, otherwise derive from Nomad node count
	d.tier = d.detectTier(ctx, primaryOrch)

	d.bodyMgr = body.NewBodyManager(d.store, primaryOrch, d.cfg.Daemon.ClusterID)
	d.bodySvc = service.NewBodyService(d.bodyMgr, d.store, d.orchRegistry)

	// Create migration coordinator (registry starts nil — hot-swapped later)
	d.migrator = body.NewMigrationCoordinator(d.store, d.bodyMgr, d.orchRegistry, nil)
	// Restore any previously persisted S3 registry config
	d.restoreRegistryConfig(ctx)

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
		descriptors, err := agent.LoadDescriptors(d.cfg.AgentsDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "daemon: load agent descriptors from %s: %v\n", d.cfg.AgentsDir, err)
			// Not fatal — daemon can operate without agent installer
		} else if len(descriptors) > 0 {
			d.installer = agent.NewInstaller(d.bodyMgr, d.ingress, d.orchRegistry, descriptors)
		} else {
			fmt.Fprintf(os.Stderr, "daemon: info: no agent descriptors found in %s\n", d.cfg.AgentsDir)
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

	if err := d.writeAddrFile(); err != nil {
		return fmt.Errorf("daemon: write addr file: %w", err)
	}
	defer d.removeAddrFile()

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
				func(ctx context.Context) string {
					if defOrch, err := d.orchRegistry.Default(); err == nil && defOrch.IsHealthy(ctx) {
						return "healthy"
					}
					return "degraded"
				},
				func() []heartbeat.HeartbeatBodyInfo {
					records, err := d.store.ListBodiesByCluster(ctx, d.cfg.Daemon.ClusterID)
					if err != nil || len(records) == 0 {
						return nil
					}
					result := make([]heartbeat.HeartbeatBodyInfo, 0, len(records))
					for _, r := range records {
						result = append(result, heartbeat.HeartbeatBodyInfo{
							ID:    r.ID,
							Name:  r.Name,
							State: string(r.State),
						})
					}
					return result
				},
				func() string {
					addr := d.HTTPAddr()
					if addr == "" {
						return ""
					}
					// addr is host:port from net.Listen
					_, port, err := net.SplitHostPort(addr)
					if err != nil {
						// Try parsing as URL (127.0.0.1:8080 without brackets case)
						if u, uErr := url.Parse("//" + addr); uErr == nil && u.Port() != "" {
							return u.Port()
						}
						// If MESH_PORT is set, use that
						if p := os.Getenv("MESH_PORT"); p != "" {
							return p
						}
						return ""
					}
					return port
				},
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
		_ = d.pluginMgr.Stop()
	}

	if d.store != nil {
		d.store.Close()
	}

	d.removePIDFile()
	d.removeAddrFile()

	d.doneOnce.Do(func() { close(d.done) })
	return nil
}

func (d *Daemon) reconcile(ctx context.Context) error {
	bodies, err := d.store.ListBodies(ctx)
	if err != nil {
		if ctx.Err() == context.Canceled {
			return nil
		}
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
			PublicDomain:  d.cfg.Ingress.PublicDomain,
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

	// Build heartbeat status function. d.heartbeat may be nil at router
	// creation time but is set later by Start() before the loop runs.
	// The closure captures d (not d.heartbeat), so it reads the current
	// value on each call.
	heartbeatStatusFn := func() api.GatewayHeartbeatStatus {
		if d.heartbeat == nil {
			return api.GatewayHeartbeatStatus{}
		}
		s := d.heartbeat.Status()
		return api.GatewayHeartbeatStatus{
			Reachable:           s.Reachable,
			LastSuccess:         s.LastSuccess,
			ConsecutiveFailures: s.ConsecutiveFailures,
			LastError:           s.LastError,
		}
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
		RegistryManager:          d, // Daemon implements api.RegistryManager
		StopDaemon:               d.Stop,
		HeartbeatStatusFn:        heartbeatStatusFn,
	})

	listenAddr := d.cfg.Daemon.ListenAddr
	if listenAddr == "" {
		port := os.Getenv("MESH_PORT")
		if port == "" {
			port = "8080"
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

	go func() { _ = srv.Serve(ln) }()
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

func (d *Daemon) addrFilePath() string {
	if d.cfg.Daemon.PIDFile == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(d.cfg.Daemon.PIDFile), "daemon.addr")
}

func (d *Daemon) writeAddrFile() error {
	path := d.addrFilePath()
	if path == "" {
		return nil
	}
	return os.WriteFile(path, []byte(d.httpAddr), 0o644)
}

func (d *Daemon) removeAddrFile() {
	path := d.addrFilePath()
	if path != "" {
		os.Remove(path)
	}
}

// detectTier determines the cluster tier for this daemon.
// Priority: 1) explicit config, 2) Nomad node count, 3) default "solo".
func (d *Daemon) detectTier(ctx context.Context, primaryOrch orchestrator.OrchestratorAdapter) string {
	// Explicit config tier takes precedence
	if d.cfg.Tier != "" {
		fmt.Fprintf(os.Stderr, "daemon: tier from config: %s\n", d.cfg.Tier)
		return d.cfg.Tier
	}

	// Derive from Nomad: count nodes; 1 node → solo, >1 → cluster
	if lister, ok := primaryOrch.(orchestrator.NodeLister); ok {
		nodes, err := lister.ListNodes(ctx)
		if err == nil {
			if len(nodes) > 1 {
				fmt.Fprintf(os.Stderr, "daemon: tier auto-detected (%d nodes): cluster\n", len(nodes))
				return "CLUSTER"
			}
			fmt.Fprintf(os.Stderr, "daemon: tier auto-detected (%d nodes): solo\n", len(nodes))
			return "SOLO"
		}
		fmt.Fprintf(os.Stderr, "daemon: tier: failed to list nodes: %v, defaulting to solo\n", err)
	}

	return "SOLO"
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
