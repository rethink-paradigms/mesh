package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/rethink-paradigms/mesh/internal/api"
	"github.com/rethink-paradigms/mesh/internal/ingress"
	"github.com/rethink-paradigms/mesh/internal/orchestrator"
)

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
		slog.Warn("unknown ingress adapter, falling back to noop", "adapter", d.cfg.Ingress.Adapter)
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
		AgentVaultToken:          d.cfg.Daemon.AgentVaultToken,
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
			slog.Warn("api server shutdown", "error", err)
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
		slog.Info("tier from config", "tier", d.cfg.Tier)
		return d.cfg.Tier
	}

	// Derive from Nomad: count nodes; 1 node → solo, >1 → cluster
	if lister, ok := primaryOrch.(orchestrator.NodeLister); ok {
		nodes, err := lister.ListNodes(ctx)
		if err == nil {
			if len(nodes) > 1 {
				slog.Info("tier auto-detected", "nodes", len(nodes), "tier", "CLUSTER")
				return "CLUSTER"
			}
			slog.Info("tier auto-detected", "nodes", len(nodes), "tier", "SOLO")
			return "SOLO"
		}
		slog.Warn("tier: failed to list nodes, defaulting to solo", "error", err)
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
