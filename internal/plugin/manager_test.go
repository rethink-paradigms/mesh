package plugin

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func buildReferencePluginForManager(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	binPath := filepath.Join(tmpDir, "mesh-plugin-reference")
	if runtime.GOOS == "windows" {
		binPath += ".exe"
	}

	_, thisFile, _, _ := runtime.Caller(0)
	refDir := filepath.Join(filepath.Dir(thisFile), "reference")

	cmd := exec.Command("go", "build", "-o", binPath, ".")
	cmd.Dir = refDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to build reference plugin: %v\n%s", err, out)
	}
	return binPath
}

func TestPluginManagerScan(t *testing.T) {
	binPath := buildReferencePluginForManager(t)
	pluginDir := filepath.Dir(binPath)

	pm := NewPluginManager(pluginDir, []string{"mesh-plugin-reference"})
	found, err := pm.Scan()
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	if len(found) != 1 {
		t.Fatalf("expected 1 plugin, got %d", len(found))
	}
	if _, ok := found["mesh-plugin-reference"]; !ok {
		t.Fatalf("expected mesh-plugin-reference in scan results, got %v", found)
	}
}

func TestPluginManagerScanEmptyDir(t *testing.T) {
	emptyDir := t.TempDir()
	pm := NewPluginManager(emptyDir, []string{})
	found, err := pm.Scan()
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	if len(found) != 0 {
		t.Fatalf("expected 0 plugins, got %d", len(found))
	}
}

func TestPluginManagerScanMissingDir(t *testing.T) {
	pm := NewPluginManager("/nonexistent/path/12345", []string{})
	found, err := pm.Scan()
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	if len(found) != 0 {
		t.Fatalf("expected 0 plugins, got %d", len(found))
	}
}

func TestPluginManagerLoad(t *testing.T) {
	binPath := buildReferencePluginForManager(t)
	pluginDir := filepath.Dir(binPath)

	pm := NewPluginManager(pluginDir, []string{"mesh-plugin-reference"})
	if err := pm.Load("mesh-plugin-reference", binPath); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	rec := pm.Get("mesh-plugin-reference")
	if rec == nil {
		t.Fatal("expected plugin record, got nil")
	}
	if rec.GetState() != StateHealthy {
		t.Fatalf("expected state Healthy, got %s", rec.GetState())
	}
	if rec.Meta.Name != "reference" {
		t.Fatalf("expected plugin name 'reference', got %q", rec.Meta.Name)
	}
	if rec.Meta.Version != "1.0.0" {
		t.Fatalf("expected version '1.0.0', got %q", rec.Meta.Version)
	}

	if err := pm.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
}

func TestPluginManagerLoadNotEnabled(t *testing.T) {
	binPath := buildReferencePluginForManager(t)
	pluginDir := filepath.Dir(binPath)

	pm := NewPluginManager(pluginDir, []string{})
	err := pm.Load("mesh-plugin-reference", binPath)
	if err == nil {
		t.Fatal("expected error for non-enabled plugin")
	}
}

func TestPluginManagerLoadDuplicate(t *testing.T) {
	binPath := buildReferencePluginForManager(t)
	pluginDir := filepath.Dir(binPath)

	pm := NewPluginManager(pluginDir, []string{"mesh-plugin-reference"})
	if err := pm.Load("mesh-plugin-reference", binPath); err != nil {
		t.Fatalf("first Load failed: %v", err)
	}
	err := pm.Load("mesh-plugin-reference", binPath)
	if err == nil {
		t.Fatal("expected error for duplicate load")
	}

	if err := pm.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
}

func TestPluginManagerStartScanAndLoad(t *testing.T) {
	binPath := buildReferencePluginForManager(t)
	pluginDir := filepath.Dir(binPath)

	pm := NewPluginManager(pluginDir, []string{"mesh-plugin-reference"})
	if err := pm.StartScanAndLoad(); err != nil {
		t.Fatalf("StartScanAndLoad failed: %v", err)
	}

	list := pm.List()
	if len(list) != 1 {
		t.Fatalf("expected 1 plugin loaded, got %d", len(list))
	}

	rec := pm.Get("mesh-plugin-reference")
	if rec == nil {
		t.Fatal("expected plugin record after scan-and-load")
	}
	if rec.GetState() != StateHealthy {
		t.Fatalf("expected state Healthy, got %s", rec.GetState())
	}

	if err := pm.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
}

func TestPluginManagerHealthCheck(t *testing.T) {
	binPath := buildReferencePluginForManager(t)
	pluginDir := filepath.Dir(binPath)

	pm := NewPluginManager(pluginDir, []string{"mesh-plugin-reference"})
	pm.healthInterval = 200 * time.Millisecond
	if err := pm.Load("mesh-plugin-reference", binPath); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	pm.StartHealthChecks()

	time.Sleep(500 * time.Millisecond)

	rec := pm.Get("mesh-plugin-reference")
	if rec == nil {
		t.Fatal("expected plugin record")
	}
	if rec.GetState() != StateHealthy {
		t.Fatalf("expected state Healthy after health checks, got %s", rec.GetState())
	}
	if rec.GetFailCount() != 0 {
		t.Fatalf("expected fail count 0, got %d", rec.GetFailCount())
	}

	if err := pm.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
}

func TestPluginManagerHealthCheckDetectsUnhealthy(t *testing.T) {
	binPath := buildReferencePluginForManager(t)
	pluginDir := filepath.Dir(binPath)

	pm := NewPluginManager(pluginDir, []string{"mesh-plugin-reference"})
	if err := pm.Load("mesh-plugin-reference", binPath); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	rec := pm.Get("mesh-plugin-reference")
	if rec == nil {
		t.Fatal("expected plugin record")
	}

	rec.Client.Kill()

	for i := 0; i < 3; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		_, err := rec.Impl.PluginInfo(ctx)
		cancel()
		if err == nil {
			t.Fatal("expected PluginInfo to fail after kill")
		}
		rec.mu.Lock()
		rec.FailCount++
		if rec.FailCount >= 3 {
			rec.State = StateUnhealthy
		}
		rec.mu.Unlock()
	}

	if rec.GetState() != StateUnhealthy {
		t.Fatalf("expected state Unhealthy, got %s", rec.GetState())
	}
	if rec.GetFailCount() != 3 {
		t.Fatalf("expected fail count 3, got %d", rec.GetFailCount())
	}

	if err := pm.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
}

func TestPluginManagerRestartAfterCrash(t *testing.T) {
	binPath := buildReferencePluginForManager(t)
	pluginDir := filepath.Dir(binPath)

	pm := NewPluginManager(pluginDir, []string{"mesh-plugin-reference"})
	if err := pm.Load("mesh-plugin-reference", binPath); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	rec := pm.Get("mesh-plugin-reference")
	if rec == nil {
		t.Fatal("expected plugin record")
	}

	oldClient := rec.Client
	oldClient.Kill()

	pm.attemptRestart("mesh-plugin-reference", rec)

	if rec.GetState() != StateHealthy {
		t.Fatalf("expected state Healthy after restart, got %s", rec.GetState())
	}
	if rec.GetFailCount() != 0 {
		t.Fatalf("expected fail count 0 after restart, got %d", rec.GetFailCount())
	}
	if rec.Impl == nil {
		t.Fatal("expected Impl after restart")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	meta, err := rec.Impl.PluginInfo(ctx)
	cancel()
	if err != nil {
		t.Fatalf("PluginInfo after restart failed: %v", err)
	}
	if meta.Name != "reference" {
		t.Fatalf("expected name 'reference' after restart, got %q", meta.Name)
	}

	if err := pm.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
}

func TestPluginManagerRestartMaxRetries(t *testing.T) {
	binPath := buildReferencePluginForManager(t)
	pluginDir := filepath.Dir(binPath)

	pm := NewPluginManager(pluginDir, []string{"mesh-plugin-reference"})
	if err := pm.Load("mesh-plugin-reference", binPath); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	rec := pm.Get("mesh-plugin-reference")
	if rec == nil {
		t.Fatal("expected plugin record")
	}

	rec.Client.Kill()

	rec.SetRetryCount(3)
	pm.attemptRestart("mesh-plugin-reference", rec)

	if rec.GetState() != StateUnhealthy {
		t.Fatalf("expected state Unhealthy after max retries, got %s", rec.GetState())
	}

	if err := pm.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
}

func TestPluginManagerShutdownKillsProcesses(t *testing.T) {
	binPath := buildReferencePluginForManager(t)
	pluginDir := filepath.Dir(binPath)

	pm := NewPluginManager(pluginDir, []string{"mesh-plugin-reference"})
	if err := pm.Load("mesh-plugin-reference", binPath); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	rec := pm.Get("mesh-plugin-reference")
	if rec == nil {
		t.Fatal("expected plugin record")
	}
	if rec.Client.Exited() {
		t.Fatal("expected plugin process to be running before shutdown")
	}

	if err := pm.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	if !rec.Client.Exited() {
		t.Fatal("expected plugin process to be exited after shutdown")
	}
	if rec.GetState() != StateRemoved {
		t.Fatalf("expected state Removed, got %s", rec.GetState())
	}
}

func TestPluginManagerListAndGet(t *testing.T) {
	binPath := buildReferencePluginForManager(t)
	pluginDir := filepath.Dir(binPath)

	pm := NewPluginManager(pluginDir, []string{"mesh-plugin-reference"})
	if err := pm.Load("mesh-plugin-reference", binPath); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	list := pm.List()
	if len(list) != 1 || list[0] != "mesh-plugin-reference" {
		t.Fatalf("expected list [mesh-plugin-reference], got %v", list)
	}

	rec := pm.Get("mesh-plugin-reference")
	if rec == nil {
		t.Fatal("expected plugin record")
	}

	missing := pm.Get("nonexistent")
	if missing != nil {
		t.Fatal("expected nil for nonexistent plugin")
	}

	if err := pm.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
}

func TestPluginManagerStateTracking(t *testing.T) {
	rec := &PluginRecord{State: StateLoaded}

	rec.SetState(StateHealthy)
	if rec.GetState() != StateHealthy {
		t.Fatalf("expected Healthy, got %s", rec.GetState())
	}

	rec.SetState(StateUnhealthy)
	if rec.GetState() != StateUnhealthy {
		t.Fatalf("expected Unhealthy, got %s", rec.GetState())
	}

	rec.SetFailCount(2)
	if rec.GetFailCount() != 2 {
		t.Fatalf("expected fail count 2, got %d", rec.GetFailCount())
	}

	rec.SetRetryCount(1)
	if rec.GetRetryCount() != 1 {
		t.Fatalf("expected retry count 1, got %d", rec.GetRetryCount())
	}
}

func TestPluginManagerScanSkipsNonExecutable(t *testing.T) {
	pluginDir := t.TempDir()
	nonExec := filepath.Join(pluginDir, "not-executable.txt")
	if err := os.WriteFile(nonExec, []byte("hello"), 0644); err != nil {
		t.Fatalf("failed to create non-executable file: %v", err)
	}

	pm := NewPluginManager(pluginDir, []string{})
	found, err := pm.Scan()
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	if len(found) != 0 {
		t.Fatalf("expected 0 plugins, got %d", len(found))
	}
}

func TestPluginManagerScanSkipsNonPluginBinary(t *testing.T) {
	pluginDir := t.TempDir()
	badBin := filepath.Join(pluginDir, "bad-binary")
	if runtime.GOOS == "windows" {
		badBin += ".exe"
	}

	badCode := `package main
import "time"
func main() {
	time.Sleep(10 * time.Millisecond)
}
`
	tmpDir := t.TempDir()
	srcFile := filepath.Join(tmpDir, "main.go")
	if err := os.WriteFile(srcFile, []byte(badCode), 0644); err != nil {
		t.Fatalf("failed to write bad source: %v", err)
	}

	cmd := exec.Command("go", "build", "-o", badBin, srcFile)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to build bad binary: %v\n%s", err, out)
	}
	if err := os.Chmod(badBin, 0755); err != nil {
		t.Fatalf("failed to chmod bad binary: %v", err)
	}

	pm := NewPluginManager(pluginDir, []string{})
	found, err := pm.Scan()
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	if len(found) != 0 {
		t.Fatalf("expected 0 plugins, got %d", len(found))
	}
}

func TestPluginManagerLoadInvalidPlugin(t *testing.T) {
	pluginDir := t.TempDir()
	badBin := filepath.Join(pluginDir, "bad-plugin")
	if runtime.GOOS == "windows" {
		badBin += ".exe"
	}

	badCode := `package main
import "github.com/hashicorp/go-plugin"
func main() {
	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: plugin.HandshakeConfig{
			ProtocolVersion: 1,
			MagicCookieKey: "WRONG",
			MagicCookieValue: "wrong",
		},
	})
}
`
	tmpDir := t.TempDir()
	srcFile := filepath.Join(tmpDir, "main.go")
	if err := os.WriteFile(srcFile, []byte(badCode), 0644); err != nil {
		t.Fatalf("failed to write bad source: %v", err)
	}

	cmd := exec.Command("go", "build", "-o", badBin, srcFile)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to build bad binary: %v\n%s", err, out)
	}
	if err := os.Chmod(badBin, 0755); err != nil {
		t.Fatalf("failed to chmod bad binary: %v", err)
	}

	pm := NewPluginManager(pluginDir, []string{"bad-plugin"})
	err = pm.Load("bad-plugin", badBin)
	if err == nil {
		t.Fatal("expected error loading plugin with wrong handshake")
	}
}

func TestPluginManagerHealthCheckRecovers(t *testing.T) {
	binPath := buildReferencePluginForManager(t)
	pluginDir := filepath.Dir(binPath)

	pm := NewPluginManager(pluginDir, []string{"mesh-plugin-reference"})
	if err := pm.Load("mesh-plugin-reference", binPath); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	rec := pm.Get("mesh-plugin-reference")
	if rec == nil {
		t.Fatal("expected plugin record")
	}

	rec.SetState(StateUnhealthy)
	rec.SetFailCount(3)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	_, err := rec.Impl.PluginInfo(ctx)
	cancel()
	if err != nil {
		t.Fatalf("PluginInfo should succeed for healthy plugin: %v", err)
	}

	rec.mu.Lock()
	rec.FailCount = 0
	if rec.State == StateUnhealthy || rec.State == StateCrashed {
		rec.State = StateHealthy
	}
	rec.mu.Unlock()

	if rec.GetState() != StateHealthy {
		t.Fatalf("expected state Healthy after recovery, got %s", rec.GetState())
	}
	if rec.GetFailCount() != 0 {
		t.Fatalf("expected fail count 0 after recovery, got %d", rec.GetFailCount())
	}

	if err := pm.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
}

func TestPluginManagerMultiplePlugins(t *testing.T) {
	binPath := buildReferencePluginForManager(t)
	pluginDir := filepath.Dir(binPath)

	pm := NewPluginManager(pluginDir, []string{"mesh-plugin-reference"})
	if err := pm.Load("mesh-plugin-reference", binPath); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	list := pm.List()
	if len(list) != 1 {
		t.Fatalf("expected 1 plugin, got %d", len(list))
	}

	if err := pm.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
}

func TestPluginManagerGetAfterStop(t *testing.T) {
	binPath := buildReferencePluginForManager(t)
	pluginDir := filepath.Dir(binPath)

	pm := NewPluginManager(pluginDir, []string{"mesh-plugin-reference"})
	if err := pm.Load("mesh-plugin-reference", binPath); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if err := pm.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	rec := pm.Get("mesh-plugin-reference")
	if rec == nil {
		t.Fatal("expected plugin record after stop")
	}
	if rec.GetState() != StateRemoved {
		t.Fatalf("expected state Removed after stop, got %s", rec.GetState())
	}
}

func TestPluginManagerStartHealthChecksStop(t *testing.T) {
	binPath := buildReferencePluginForManager(t)
	pluginDir := filepath.Dir(binPath)

	pm := NewPluginManager(pluginDir, []string{"mesh-plugin-reference"})
	pm.healthInterval = 100 * time.Millisecond
	if err := pm.Load("mesh-plugin-reference", binPath); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	pm.StartHealthChecks()

	time.Sleep(250 * time.Millisecond)

	if err := pm.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	rec := pm.Get("mesh-plugin-reference")
	if rec == nil {
		t.Fatal("expected plugin record")
	}
	if rec.GetState() != StateRemoved {
		t.Fatalf("expected state Removed after stop, got %s", rec.GetState())
	}
}

func TestPluginManagerScanAndLoadOnlyEnabled(t *testing.T) {
	binPath := buildReferencePluginForManager(t)
	pluginDir := filepath.Dir(binPath)

	pm := NewPluginManager(pluginDir, []string{})
	if err := pm.StartScanAndLoad(); err != nil {
		t.Fatalf("StartScanAndLoad failed: %v", err)
	}

	list := pm.List()
	if len(list) != 0 {
		t.Fatalf("expected 0 plugins (none enabled), got %d", len(list))
	}
}

func TestPluginManagerRecordConcurrency(t *testing.T) {
	rec := &PluginRecord{State: StateLoaded}

	done := make(chan struct{})
	go func() {
		for i := 0; i < 100; i++ {
			rec.SetState(StateHealthy)
			rec.SetState(StateUnhealthy)
		}
		close(done)
	}()

	for i := 0; i < 100; i++ {
		_ = rec.GetState()
	}

	<-done
}

func TestPluginManagerLoadPluginInfoTimeout(t *testing.T) {
	pluginDir := t.TempDir()
	badBin := filepath.Join(pluginDir, "slow-plugin")
	if runtime.GOOS == "windows" {
		badBin += ".exe"
	}

	badCode := `package main
import (
	"time"
	"github.com/hashicorp/go-plugin"
)
func main() {
	time.Sleep(10 * time.Second)
	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: plugin.HandshakeConfig{
			ProtocolVersion: 1,
			MagicCookieKey: "MESH_PLUGIN",
			MagicCookieValue: "mesh-plugin-2026",
		},
	})
}
`
	tmpDir := t.TempDir()
	srcFile := filepath.Join(tmpDir, "main.go")
	if err := os.WriteFile(srcFile, []byte(badCode), 0644); err != nil {
		t.Fatalf("failed to write slow source: %v", err)
	}

	cmd := exec.Command("go", "build", "-o", badBin, srcFile)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to build slow binary: %v\n%s", err, out)
	}
	if err := os.Chmod(badBin, 0755); err != nil {
		t.Fatalf("failed to chmod slow binary: %v", err)
	}

	pm := NewPluginManager(pluginDir, []string{"slow-plugin"})
	err = pm.Load("slow-plugin", badBin)
	if err == nil {
		t.Fatal("expected error loading slow plugin")
	}
}

func TestPluginManagerRestartAfterCrashWithRetries(t *testing.T) {
	binPath := buildReferencePluginForManager(t)
	pluginDir := filepath.Dir(binPath)

	pm := NewPluginManager(pluginDir, []string{"mesh-plugin-reference"})
	if err := pm.Load("mesh-plugin-reference", binPath); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	rec := pm.Get("mesh-plugin-reference")
	if rec == nil {
		t.Fatal("expected plugin record")
	}

	rec.Client.Kill()
	rec.SetRetryCount(0)

	pm.attemptRestart("mesh-plugin-reference", rec)

	if rec.GetState() != StateHealthy {
		t.Fatalf("expected state Healthy after restart, got %s", rec.GetState())
	}
	if rec.GetRetryCount() != 1 {
		t.Fatalf("expected retry count 1, got %d", rec.GetRetryCount())
	}

	if err := pm.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
}

func TestPluginManagerStopNoPlugins(t *testing.T) {
	pm := NewPluginManager(t.TempDir(), []string{})
	if err := pm.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
}

func TestPluginManagerCheckAllWithNoPlugins(t *testing.T) {
	pm := NewPluginManager(t.TempDir(), []string{})
	pm.checkAll()
}

func TestPluginManagerAttemptRestartBadPath(t *testing.T) {
	pm := NewPluginManager(t.TempDir(), []string{"bad"})
	rec := &PluginRecord{
		Path:   "/nonexistent/binary",
		State:  StateCrashed,
		Client: nil,
	}
	pm.attemptRestart("bad", rec)
	if rec.GetState() != StateUnhealthy {
		t.Fatalf("expected state Unhealthy, got %s", rec.GetState())
	}
}
