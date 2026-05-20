package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
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
	installer    api.Installer
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
				slog.Warn("mcp set auth", "error", err)
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
	// Wire up the agent installer so install_agent MCP tool works
	if d.installer != nil {
		if ms, ok := srv.(interface{ SetInstaller(api.Installer) }); ok {
			ms.SetInstaller(d.installer)
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

// wire sets up all daemon dependencies: store, orchestrator adapters,
// body manager, plugin manager, ingress, and installer.
// It is called once at startup by Start(). Extracted from Start() to make
// the dependency graph explicit and testable.
func (d *Daemon) wire(ctx context.Context) error {
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

	dockerCfg := docker.Config{}
	if dockerSettings, ok := d.cfg.Orchestrators["docker"]; ok {
		if sp, ok := dockerSettings["socket_path"]; ok {
			dockerCfg.SocketPath = sp
		}
	}
	dockerAdp := docker.New(dockerCfg)
	if err := orchRegistry.Register("docker", dockerAdp); err != nil {
		slog.Warn("register docker orchestrator", "error", err)
	}

	for name, settings := range d.cfg.Orchestrators {
		switch name {
		case "docker":
			continue
		case "nomad":
			adp := nomad.New(nomad.Config{
				Address:   settings["address"],
				Token:     settings["token"],
				Region:    settings["region"],
				Namespace: settings["namespace"],
			})
			if err := orchRegistry.Register("nomad", adp); err != nil {
				slog.Warn("register orchestrator", "name", name, "error", err)
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

	d.tier = d.detectTier(ctx, primaryOrch)

	d.bodyMgr = body.NewBodyManager(d.store, primaryOrch, d.cfg.Daemon.ClusterID)
	d.bodySvc = service.NewBodyService(d.bodyMgr, d.store, d.orchRegistry)

	d.migrator = body.NewMigrationCoordinator(d.store, d.bodyMgr, d.orchRegistry, nil)
	d.restoreRegistryConfig(ctx)

	pm := plugin.NewPluginManager(d.cfg.Plugin.Dir, d.cfg.Plugin.Enabled)
	if err := pm.StartScanAndLoad(); err != nil {
		slog.Warn("plugin scan and load", "error", err)
	}
	pm.StartHealthChecks()
	d.pluginMgr = pm

	if err := d.reconcile(ctx); err != nil {
		return fmt.Errorf("daemon: reconcile: %w", err)
	}

	caddyOnSystem := caddyDetected()
	if d.cfg.Ingress.Adapter == "" || d.cfg.Ingress.Adapter == "noop" {
		if caddyOnSystem {
			if d.cfg.Ingress.AdminURL == "" {
				d.cfg.Ingress.AdminURL = "http://127.0.0.1:2019"
			}
			d.cfg.Ingress.Adapter = "caddy"
			slog.Info("caddy detected, using caddy ingress adapter", "admin_url", d.cfg.Ingress.AdminURL)
		}
	} else if d.cfg.Ingress.Adapter == "caddy" && !caddyOnSystem {
		slog.Warn("caddy configured but not installed, falling back to noop ingress adapter")
		d.cfg.Ingress.Adapter = "noop"
	}

	d.ingress = d.createIngressAdapter()
	d.bodyMgr.SetIngress(d.ingress)

	// Warm the port pool from existing body allocations so restarts don't
	// conflict with ports already bound by running Docker containers.
	d.warmPortPool(ctx)

	var descriptors map[string]*agent.Descriptor
	if d.cfg.AgentsDir != "" {
		loaded, err := agent.LoadDescriptors(d.cfg.AgentsDir)
		if err != nil {
			slog.Warn("load agent descriptors", "dir", d.cfg.AgentsDir, "error", err)
		}
		descriptors = loaded
	}
	d.installer = agent.NewInstaller(d.bodyMgr, d.ingress, d.orchRegistry, descriptors)

	return nil
}

func (d *Daemon) Start(ctx context.Context) error {
	if err := d.wire(ctx); err != nil {
		return err
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

	slog.Info("API server listening", "addr", d.httpAddr)

	// Start heartbeat goroutine if gateway URL is configured
	if d.cfg.Daemon.GatewayURL != "" && d.cfg.Daemon.HeartbeatIntervalSeconds > 0 {
		if d.cfg.Daemon.AuthMode == "jwt" && d.cfg.Daemon.AuthToken == "" {
			slog.Info("heartbeat disabled: JWT-only mode requires auth_token for heartbeat")
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
			slog.Info("heartbeat started", "gateway_url", d.cfg.Daemon.GatewayURL, "interval", interval)
		}
	}

	signal.Notify(d.sigs, syscall.SIGTERM, syscall.SIGINT)

	d.mu.Lock()
	d.ready = true
	d.mu.Unlock()

	// Start background reconciler to sync Docker container state back to store.
	go d.reconcileLoop(ctx)

	select {
	case sig := <-d.sigs:
		slog.Info("received signal, shutting down", "signal", sig)
	case <-ctx.Done():
	}

	stopCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return d.Stop(stopCtx)
}

// warmPortPool pre-populates the port pool from persisted body port allocations.
// This prevents port conflicts after a daemon restart when Docker containers
// are still running with ports bound.
func (d *Daemon) warmPortPool(ctx context.Context) {
	warmable, ok := d.ingress.(interface{ WarmPorts([]int) })
	if !ok {
		return // only adapters with an in-memory port pool need warming
	}

	bodies, err := d.store.ListBodies(ctx)
	if err != nil {
		slog.Warn("warm port pool: list bodies", "error", err)
		return
	}

	var ports []int
	for _, rec := range bodies {
		if rec.AllocatedPortsJSON == "" {
			continue
		}
		var allocs []body.AllocatedPort
		if err := json.Unmarshal([]byte(rec.AllocatedPortsJSON), &allocs); err != nil {
			slog.Warn("warm port pool: parse allocations", "body_id", rec.ID, "error", err)
			continue
		}
		for _, alloc := range allocs {
			if alloc.HostPort > 0 {
				ports = append(ports, alloc.HostPort)
			}
		}
	}

	if len(ports) > 0 {
		warmable.WarmPorts(ports)
		slog.Info("warmed port pool from existing bodies", "ports", len(ports), "bodies", len(bodies))
	}
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
			slog.Warn("mcp stop", "error", err)
		}
	}
	d.mcpServerMu.Unlock()

	if d.pluginMgr != nil {
		_ = d.pluginMgr.Stop()
	}

	if d.store != nil {
		d.store.Close() //nolint:errcheck
	}

	d.removePIDFile()
	d.removeAddrFile()

	d.doneOnce.Do(func() { close(d.done) })
	return nil
}
