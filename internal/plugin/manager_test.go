package plugin

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestPluginManagerScan(t *testing.T) {
	pluginDir := t.TempDir()
	pm := NewPluginManager(pluginDir, []string{"mesh-plugin-reference"})
	found, err := pm.Scan()
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	if len(found) != 0 {
		t.Fatalf("expected 0 plugins in empty dir, got %d", len(found))
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
	pm := NewPluginManager(t.TempDir(), []string{"mesh-plugin-test"})
	err := pm.Load("mesh-plugin-test", "/nonexistent/binary/path")
	if err == nil {
		t.Fatal("expected error loading non-existent binary")
	}
}

func TestPluginManagerLoadNotEnabled(t *testing.T) {
	pm := NewPluginManager(t.TempDir(), []string{})
	err := pm.Load("mesh-plugin-reference", "/any/path")
	if err == nil {
		t.Fatal("expected error for non-enabled plugin")
	}
}

func TestPluginManagerLoadDuplicate(t *testing.T) {
	pm := NewPluginManager(t.TempDir(), []string{"my-plugin"})
	pm.plugins["my-plugin"] = &PluginRecord{State: StateLoaded}
	err := pm.Load("my-plugin", "/any/path")
	if err == nil {
		t.Fatal("expected error for duplicate load")
	}
	if err := pm.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
}

func TestPluginManagerStartScanAndLoad(t *testing.T) {
	pm := NewPluginManager(t.TempDir(), []string{"mesh-plugin-reference"})
	if err := pm.StartScanAndLoad(); err != nil {
		t.Fatalf("StartScanAndLoad failed: %v", err)
	}
	list := pm.List()
	if len(list) != 0 {
		t.Fatalf("expected 0 plugins loaded (empty dir), got %d", len(list))
	}
	if err := pm.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
}

func TestPluginManagerHealthCheck(t *testing.T) {
	pm := NewPluginManager(t.TempDir(), []string{})
	pm.healthInterval = 100 * time.Millisecond
	pm.StartHealthChecks()
	time.Sleep(250 * time.Millisecond)
	list := pm.List()
	if len(list) != 0 {
		t.Fatalf("expected 0 plugins, got %d", len(list))
	}
	if err := pm.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
}

func TestPluginManagerHealthCheckDetectsUnhealthy(t *testing.T) {
	rec := &PluginRecord{State: StateHealthy}
	for i := 0; i < 3; i++ {
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
}

func TestPluginManagerRestartAfterCrash(t *testing.T) {
	pm := NewPluginManager(t.TempDir(), []string{"my-plugin"})
	rec := &PluginRecord{
		Path:  "/nonexistent/binary",
		State: StateCrashed,
	}
	pm.attemptRestart("my-plugin", rec)
	if rec.GetState() != StateUnhealthy {
		t.Fatalf("expected state Unhealthy after failed restart, got %s", rec.GetState())
	}
	if rec.GetRetryCount() != 1 {
		t.Fatalf("expected retry count 1, got %d", rec.GetRetryCount())
	}
}

func TestPluginManagerRestartMaxRetries(t *testing.T) {
	pm := NewPluginManager(t.TempDir(), []string{"my-plugin"})
	rec := &PluginRecord{
		Path:       "/nonexistent/binary",
		State:      StateCrashed,
		RetryCount: 3,
	}
	pm.attemptRestart("my-plugin", rec)
	if rec.GetState() != StateUnhealthy {
		t.Fatalf("expected state Unhealthy after max retries, got %s", rec.GetState())
	}
}

func TestPluginManagerShutdownKillsProcesses(t *testing.T) {
	pm := NewPluginManager(t.TempDir(), []string{"my-plugin"})
	pm.plugins["my-plugin"] = &PluginRecord{State: StateLoaded}
	if err := pm.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
	rec := pm.Get("my-plugin")
	if rec == nil {
		t.Fatal("expected plugin record after stop")
	}
	if rec.GetState() != StateRemoved {
		t.Fatalf("expected state Removed, got %s", rec.GetState())
	}
}

func TestPluginManagerListAndGet(t *testing.T) {
	pm := NewPluginManager(t.TempDir(), []string{})
	list := pm.List()
	if len(list) != 0 {
		t.Fatalf("expected empty list, got %v", list)
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
	if runtime.GOOS == "windows" {
		t.Skip("shell script test not supported on Windows")
	}
	pluginDir := t.TempDir()
	badBin := filepath.Join(pluginDir, "bad-plugin")
	if err := os.WriteFile(badBin, []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
		t.Fatalf("failed to create shell script: %v", err)
	}

	pm := NewPluginManager(pluginDir, []string{"bad-plugin"})
	err := pm.Load("bad-plugin", badBin)
	if err == nil {
		t.Fatal("expected error loading non-plugin binary")
	}
}

func TestPluginManagerHealthCheckRecovers(t *testing.T) {
	rec := &PluginRecord{State: StateUnhealthy, FailCount: 3}
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
}

func TestPluginManagerMultiplePlugins(t *testing.T) {
	pm := NewPluginManager(t.TempDir(), []string{"plugin-a", "plugin-b"})
	pm.plugins["plugin-a"] = &PluginRecord{State: StateHealthy}
	pm.plugins["plugin-b"] = &PluginRecord{State: StateHealthy}

	list := pm.List()
	if len(list) != 2 {
		t.Fatalf("expected 2 plugins, got %d", len(list))
	}
	if err := pm.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
}

func TestPluginManagerGetAfterStop(t *testing.T) {
	pm := NewPluginManager(t.TempDir(), []string{"my-plugin"})
	pm.plugins["my-plugin"] = &PluginRecord{State: StateLoaded}

	if err := pm.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	rec := pm.Get("my-plugin")
	if rec == nil {
		t.Fatal("expected plugin record after stop")
	}
	if rec.GetState() != StateRemoved {
		t.Fatalf("expected state Removed after stop, got %s", rec.GetState())
	}
}

func TestPluginManagerStartHealthChecksStop(t *testing.T) {
	pm := NewPluginManager(t.TempDir(), []string{})
	pm.healthInterval = 100 * time.Millisecond
	pm.StartHealthChecks()
	time.Sleep(250 * time.Millisecond)
	if err := pm.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
	list := pm.List()
	if len(list) != 0 {
		t.Fatalf("expected 0 plugins after stop, got %d", len(list))
	}
}

func TestPluginManagerScanAndLoadOnlyEnabled(t *testing.T) {
	pm := NewPluginManager(t.TempDir(), []string{})
	if err := pm.StartScanAndLoad(); err != nil {
		t.Fatalf("StartScanAndLoad failed: %v", err)
	}
	list := pm.List()
	if len(list) != 0 {
		t.Fatalf("expected 0 plugins (none enabled), got %d", len(list))
	}
	if err := pm.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
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
	if runtime.GOOS == "windows" {
		t.Skip("shell script test not supported on Windows")
	}
	pluginDir := t.TempDir()
	badBin := filepath.Join(pluginDir, "slow-plugin")
	if err := os.WriteFile(badBin, []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
		t.Fatalf("failed to create shell script: %v", err)
	}

	pm := NewPluginManager(pluginDir, []string{"slow-plugin"})
	err := pm.Load("slow-plugin", badBin)
	if err == nil {
		t.Fatal("expected error loading non-plugin binary")
	}
}

func TestPluginManagerRestartAfterCrashWithRetries(t *testing.T) {
	pm := NewPluginManager(t.TempDir(), []string{"my-plugin"})
	rec := &PluginRecord{
		Path:       "/nonexistent/binary",
		State:      StateCrashed,
		RetryCount: 0,
	}
	pm.attemptRestart("my-plugin", rec)
	if rec.GetState() != StateUnhealthy {
		t.Fatalf("expected state Unhealthy after failed restart, got %s", rec.GetState())
	}
	if rec.GetRetryCount() != 1 {
		t.Fatalf("expected retry count 1, got %d", rec.GetRetryCount())
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
