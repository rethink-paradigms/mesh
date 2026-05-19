package plugin

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/hashicorp/go-plugin"
	"google.golang.org/grpc"
)

const PluginName = "mesh-plugin"

var Handshake = plugin.HandshakeConfig{
	MagicCookieKey:   "MESH_PLUGIN",
	MagicCookieValue: "mesh-v1",
	ProtocolVersion:  1,
}

type stubPlugin struct{ plugin.Plugin }

func (s *stubPlugin) GRPCClient(_ context.Context, _ *plugin.GRPCBroker, _ *grpc.ClientConn) (any, error) {
	return nil, fmt.Errorf("plugin loading disabled: gRPC transport removed, redesign pending")
}

func (s *stubPlugin) GRPCServer(_ *plugin.GRPCBroker, _ *grpc.Server) error {
	return fmt.Errorf("plugin loading disabled: gRPC transport removed, redesign pending")
}

type PluginState string

const (
	StateLoaded    PluginState = "Loaded"
	StateHealthy   PluginState = "Healthy"
	StateUnhealthy PluginState = "Unhealthy"
	StateCrashed   PluginState = "Crashed"
	StateRemoved   PluginState = "Removed"
)

type PluginRecord struct {
	Meta       PluginMeta
	Path       string
	State      PluginState
	Client     *plugin.Client
	RPCClient  plugin.ClientProtocol
	Impl       MeshPlugin
	FailCount  int
	RetryCount int
	mu         sync.RWMutex
}

type PluginManager struct {
	dir            string
	enabled        map[string]bool
	plugins        map[string]*PluginRecord
	pluginsMu      sync.RWMutex
	healthInterval time.Duration
	stopCh         chan struct{}
	wg             sync.WaitGroup
}

func NewPluginManager(dir string, enabled []string) *PluginManager {
	enabledMap := make(map[string]bool, len(enabled))
	for _, name := range enabled {
		enabledMap[name] = true
	}
	return &PluginManager{
		dir:            dir,
		enabled:        enabledMap,
		plugins:        make(map[string]*PluginRecord),
		healthInterval: 30 * time.Second,
		stopCh:         make(chan struct{}),
	}
}

func (pm *PluginManager) Scan() (map[string]string, error) {
	found := make(map[string]string)

	entries, err := os.ReadDir(pm.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return found, nil
		}
		return nil, fmt.Errorf("plugin manager: scan %q: %w", pm.dir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		path := filepath.Join(pm.dir, name)

		info, err := entry.Info()
		if err != nil {
			continue
		}
		if info.Mode()&0o111 == 0 {
			continue
		}

		// Use a context-based timeout instead of a goroutine race.
		// A real plugin binary survives the 800ms probe because it
		// implements the go-plugin protocol and waits for a parent gRPC
		// connection. Non-plugin binaries exit (or error out) quickly.
		//
		// exec.CommandContext + Run() avoids the goroutine scheduling race
		// present in the old goroutine+select+time.After pattern:
		// under CPU load, cmd.Wait() in the spawned goroutine might not
		// get scheduled before the timeout fires, causing non-plugin
		// binaries to be incorrectly identified as plugins.
		// See https://go.dev/issues/50187, https://go.dev/issues/68308.
		probeCtx, probeCancel := context.WithTimeout(context.Background(), 800*time.Millisecond)
		cmd := exec.CommandContext(probeCtx, path)
		cmd.Env = append(os.Environ(), Handshake.MagicCookieKey+"="+Handshake.MagicCookieValue)

		err = cmd.Run()
		probeCancel()

		if err != nil {
			// Run() returned an error. If the context timed out, the
			// process was still running after 800ms — it's a plugin candidate.
			if probeCtx.Err() == context.DeadlineExceeded {
				found[name] = path
			}
			// Otherwise (process exited before timeout with non-zero exit
			// code, or some other error) — not a plugin, skip it.
			continue
		}

		// Process exited with code 0 before the 800ms timeout.
		// Definitely not a plugin — skip it.
	}

	return found, nil
}

func (pm *PluginManager) Load(name, path string) error {
	pm.pluginsMu.Lock()
	defer pm.pluginsMu.Unlock()

	if _, exists := pm.plugins[name]; exists {
		return fmt.Errorf("plugin manager: %q already loaded", name)
	}

	if !pm.enabled[name] {
		return fmt.Errorf("plugin manager: %q not in enabled list", name)
	}

	client := plugin.NewClient(&plugin.ClientConfig{
		HandshakeConfig: Handshake,
		Plugins:         plugin.PluginSet{PluginName: &stubPlugin{}},
		Cmd:             exec.Command(path),
		AllowedProtocols: []plugin.Protocol{
			plugin.ProtocolGRPC,
		},
	})

	rpcClient, err := client.Client()
	if err != nil {
		client.Kill()
		return fmt.Errorf("plugin manager: %q client init: %w", name, err)
	}

	raw, err := rpcClient.Dispense(PluginName)
	if err != nil {
		client.Kill()
		return fmt.Errorf("plugin manager: %q dispense: %w", name, err)
	}

	impl, ok := raw.(MeshPlugin)
	if !ok {
		client.Kill()
		return fmt.Errorf("plugin manager: %q does not implement MeshPlugin", name)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	meta, err := impl.PluginInfo(ctx)
	cancel()
	if err != nil {
		client.Kill()
		return fmt.Errorf("plugin manager: %q PluginInfo: %w", name, err)
	}

	rec := &PluginRecord{
		Meta:      meta,
		Path:      path,
		State:     StateLoaded,
		Client:    client,
		RPCClient: rpcClient,
		Impl:      impl,
	}
	pm.plugins[name] = rec
	rec.SetState(StateHealthy)
	rec.SetFailCount(0)

	return nil
}

func (pm *PluginManager) StartScanAndLoad() error {
	found, err := pm.Scan()
	if err != nil {
		return err
	}

	for name, path := range found {
		if !pm.enabled[name] {
			continue
		}
		if err := pm.Load(name, path); err != nil {
			slog.Error("failed to load plugin", "name", name, "error", err)
		}
	}

	return nil
}

func (pm *PluginManager) StartHealthChecks() {
	pm.wg.Add(1)
	go func() {
		defer pm.wg.Done()
		ticker := time.NewTicker(pm.healthInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				pm.checkAll()
			case <-pm.stopCh:
				return
			}
		}
	}()
}

func (pm *PluginManager) checkAll() {
	pm.pluginsMu.RLock()
	records := make([]*PluginRecord, 0, len(pm.plugins))
	names := make([]string, 0, len(pm.plugins))
	for name, rec := range pm.plugins {
		records = append(records, rec)
		names = append(names, name)
	}
	pm.pluginsMu.RUnlock()

	for i, rec := range records {
		name := names[i]
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_, err := rec.Impl.PluginInfo(ctx)
		cancel()

		if err != nil {
			rec.mu.Lock()
			rec.FailCount++
			if rec.FailCount >= 3 {
				rec.State = StateUnhealthy
			}
			rec.mu.Unlock()
			slog.Warn("plugin health check failed", "name", name, "fail_count", rec.FailCount, "error", err)

			if rec.Client.Exited() {
				rec.SetState(StateCrashed)
				pm.attemptRestart(name, rec)
			}
		} else {
			rec.mu.Lock()
			rec.FailCount = 0
			if rec.State == StateUnhealthy || rec.State == StateCrashed {
				rec.State = StateHealthy
			}
			rec.mu.Unlock()
		}
	}
}

func (pm *PluginManager) attemptRestart(name string, rec *PluginRecord) {
	rec.mu.Lock()
	if rec.RetryCount >= 3 {
		rec.State = StateUnhealthy
		rec.mu.Unlock()
		slog.Warn("plugin exceeded max retries, marking unhealthy", "name", name)
		return
	}
	rec.RetryCount++
	rec.mu.Unlock()

	if rec.Client != nil {
		rec.Client.Kill()
	}

	for attempt := 1; attempt <= 3; attempt++ {
		slog.Info("restarting plugin", "name", name, "attempt", attempt)

		client := plugin.NewClient(&plugin.ClientConfig{
			HandshakeConfig: Handshake,
			Plugins:         plugin.PluginSet{PluginName: &stubPlugin{}},
			Cmd:             exec.Command(rec.Path),
			AllowedProtocols: []plugin.Protocol{
				plugin.ProtocolGRPC,
			},
		})

		rpcClient, err := client.Client()
		if err != nil {
			client.Kill()
			if attempt < 3 {
				time.Sleep(time.Second)
			}
			continue
		}

		raw, err := rpcClient.Dispense(PluginName)
		if err != nil {
			client.Kill()
			if attempt < 3 {
				time.Sleep(time.Second)
			}
			continue
		}

		impl, ok := raw.(MeshPlugin)
		if !ok {
			client.Kill()
			if attempt < 3 {
				time.Sleep(time.Second)
			}
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		meta, err := impl.PluginInfo(ctx)
		cancel()
		if err != nil {
			client.Kill()
			if attempt < 3 {
				time.Sleep(time.Second)
			}
			continue
		}

		rec.mu.Lock()
		rec.Client = client
		rec.RPCClient = rpcClient
		rec.Impl = impl
		rec.Meta = meta
		rec.State = StateHealthy
		rec.FailCount = 0
		rec.mu.Unlock()
		slog.Info("plugin restarted successfully", "name", name)
		return
	}

	rec.mu.Lock()
	rec.State = StateUnhealthy
	rec.mu.Unlock()
	slog.Warn("plugin restart failed after retries", "name", name)
}

func (pm *PluginManager) Stop() error {
	close(pm.stopCh)
	pm.wg.Wait()

	pm.pluginsMu.Lock()
	defer pm.pluginsMu.Unlock()

	var firstErr error
	for name, rec := range pm.plugins {
		if rec.Client != nil {
			rec.Client.Kill()
		}
		rec.State = StateRemoved
		if rec.Client != nil && !rec.Client.Exited() {
			slog.Warn("plugin process may be orphaned", "name", name)
		}
	}

	if runtime.GOOS != "windows" {
		for name, rec := range pm.plugins {
			if rec.Client == nil {
				continue
			}
			proc := rec.Client.ReattachConfig()
			if proc == nil {
				continue
			}
			_ = name
		}
	}

	return firstErr
}

func (pm *PluginManager) Get(name string) *PluginRecord {
	pm.pluginsMu.RLock()
	defer pm.pluginsMu.RUnlock()
	return pm.plugins[name]
}

func (pm *PluginManager) List() []string {
	pm.pluginsMu.RLock()
	defer pm.pluginsMu.RUnlock()
	names := make([]string, 0, len(pm.plugins))
	for name := range pm.plugins {
		names = append(names, name)
	}
	return names
}

func (rec *PluginRecord) SetState(s PluginState) {
	rec.mu.Lock()
	defer rec.mu.Unlock()
	rec.State = s
}

func (rec *PluginRecord) GetState() PluginState {
	rec.mu.RLock()
	defer rec.mu.RUnlock()
	return rec.State
}

func (rec *PluginRecord) SetFailCount(n int) {
	rec.mu.Lock()
	defer rec.mu.Unlock()
	rec.FailCount = n
}

func (rec *PluginRecord) GetFailCount() int {
	rec.mu.RLock()
	defer rec.mu.RUnlock()
	return rec.FailCount
}

func (rec *PluginRecord) SetRetryCount(n int) {
	rec.mu.Lock()
	defer rec.mu.Unlock()
	rec.RetryCount = n
}

func (rec *PluginRecord) GetRetryCount() int {
	rec.mu.RLock()
	defer rec.mu.RUnlock()
	return rec.RetryCount
}
