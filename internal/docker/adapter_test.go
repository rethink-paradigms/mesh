package docker_test

import (
	"context"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/rethink-paradigms/mesh/internal/docker"
	"github.com/rethink-paradigms/mesh/internal/orchestrator"
)

func dockerAvailable() bool {
	socketPath := os.Getenv("DOCKER_HOST")
	if socketPath == "" {
		socketPath = "/var/run/docker.sock"
	}
	if strings.HasPrefix(socketPath, "unix://") {
		socketPath = strings.TrimPrefix(socketPath, "unix://")
	}
	_, err := os.Stat(socketPath)
	return err == nil
}

func skipIfNoDocker(t *testing.T) {
	if !dockerAvailable() {
		t.Skip("Docker socket not available; skipping integration test")
	}
}

// ensureAlpineImage pulls alpine:latest via the Docker API if not already available.
// This prevents test failures in CI runners where the image hasn't been cached.
func ensureAlpineImage(t *testing.T) {
	t.Helper()

	socketPath := os.Getenv("DOCKER_HOST")
	if socketPath == "" {
		socketPath = "/var/run/docker.sock"
	}

	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return net.Dial("unix", strings.TrimPrefix(socketPath, "unix://"))
			},
		},
	}

	// Check if image already exists
	ctx := context.Background()
	req, _ := http.NewRequestWithContext(ctx, "GET", "http://localhost/images/alpine:latest/json", nil)
	resp, err := client.Do(req)
	if err == nil {
		resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			return // image already available
		}
	}

	// Pull alpine:latest
	t.Log("pulling alpine:latest image...")
	req, _ = http.NewRequestWithContext(ctx, "POST", "http://localhost/images/create?fromImage=alpine:latest", nil)
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("pull alpine:latest: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("pull alpine:latest: status %d", resp.StatusCode)
	}
	t.Log("alpine:latest pulled successfully")
}

func TestNew(t *testing.T) {
	a := docker.New(docker.Config{SocketPath: "/var/run/docker.sock"})
	if a == nil {
		t.Fatal("New() returned nil")
	}
}

func TestNewFromEnv(t *testing.T) {
	t.Setenv("DOCKER_HOST", "unix:///tmp/docker.sock")

	a := docker.NewFromEnv()
	if a == nil {
		t.Fatal("NewFromEnv() returned nil")
	}
}

func TestName(t *testing.T) {
	a := docker.New(docker.Config{})
	if a.Name() != "docker" {
		t.Errorf("Name() = %q, want %q", a.Name(), "docker")
	}
}

func TestIsHealthy(t *testing.T) {
	a := docker.New(docker.Config{})
	ctx := context.Background()

	if dockerAvailable() {
		if !a.IsHealthy(ctx) {
			t.Error("IsHealthy should return true when Docker socket is reachable")
		}
	} else {
		if a.IsHealthy(ctx) {
			t.Error("IsHealthy should return false when Docker socket is not reachable")
		}
	}
}

func TestDockerAdapterLifecycle(t *testing.T) {
	skipIfNoDocker(t)
	ensureAlpineImage(t)

	a := docker.New(docker.Config{})
	ctx := context.Background()

	spec := orchestrator.BodySpec{
		Image:     "alpine:latest",
		Cmd:       []string{"sh", "-c", "sleep 300"},
		MemoryMB:  64,
		CPUShares: 100,
		Env:       map[string]string{"TEST": "value"},
	}

	handle, err := a.ScheduleBody(ctx, spec)
	if err != nil {
		t.Fatalf("ScheduleBody failed: %v", err)
	}
	if handle == "" {
		t.Fatal("ScheduleBody returned empty handle")
	}
	if !strings.HasPrefix(string(handle), "mesh-") {
		t.Errorf("handle = %q, expected prefix 'mesh-'", handle)
	}

	err = a.StartBody(ctx, handle)
	if err != nil {
		t.Fatalf("StartBody failed: %v", err)
	}

	status, err := a.GetBodyStatus(ctx, handle)
	if err != nil {
		t.Fatalf("GetBodyStatus failed: %v", err)
	}
	if status.State != orchestrator.StateRunning {
		t.Errorf("status.State = %q, want %q", status.State, orchestrator.StateRunning)
	}

	err = a.StopBody(ctx, handle)
	if err != nil {
		t.Fatalf("StopBody failed: %v", err)
	}

	time.Sleep(2 * time.Second)

	status, err = a.GetBodyStatus(ctx, handle)
	if err != nil {
		t.Fatalf("GetBodyStatus after stop failed: %v", err)
	}
	if status.State != orchestrator.StateStopped {
		t.Errorf("status.State after stop = %q, want %q", status.State, orchestrator.StateStopped)
	}

	err = a.DestroyBody(ctx, handle)
	if err != nil {
		t.Fatalf("DestroyBody failed: %v", err)
	}
}

func TestDockerAdapterGetBodyStatus(t *testing.T) {
	skipIfNoDocker(t)
	ensureAlpineImage(t)

	a := docker.New(docker.Config{})
	ctx := context.Background()

	spec := orchestrator.BodySpec{
		Image:    "alpine:latest",
		Cmd:      []string{"sh", "-c", "sleep 300"},
		MemoryMB: 64,
	}

	handle, err := a.ScheduleBody(ctx, spec)
	if err != nil {
		t.Fatalf("ScheduleBody failed: %v", err)
	}
	defer a.DestroyBody(ctx, handle)

	status, err := a.GetBodyStatus(ctx, handle)
	if err != nil {
		t.Fatalf("GetBodyStatus failed: %v", err)
	}
	if status.State != orchestrator.StateCreated {
		t.Errorf("status.State before start = %q, want %q", status.State, orchestrator.StateCreated)
	}

	err = a.StartBody(ctx, handle)
	if err != nil {
		t.Fatalf("StartBody failed: %v", err)
	}

	status, err = a.GetBodyStatus(ctx, handle)
	if err != nil {
		t.Fatalf("GetBodyStatus after start failed: %v", err)
	}
	if status.State != orchestrator.StateRunning {
		t.Errorf("status.State after start = %q, want %q", status.State, orchestrator.StateRunning)
	}
}

func TestDockerAdapterWithPorts(t *testing.T) {
	skipIfNoDocker(t)
	ensureAlpineImage(t)

	a := docker.New(docker.Config{})
	ctx := context.Background()

	spec := orchestrator.BodySpec{
		Image:    "alpine:latest",
		Cmd:      []string{"sh", "-c", "sleep 300"},
		MemoryMB: 64,
		Ports: []orchestrator.BodyPort{
			{
				Name:          "http",
				ContainerPort: 8080,
				HostPort:      0,
				Protocol:      "tcp",
				Expose:        true,
			},
		},
	}

	handle, err := a.ScheduleBody(ctx, spec)
	if err != nil {
		t.Fatalf("ScheduleBody with ports failed: %v", err)
	}
	defer a.DestroyBody(ctx, handle)

	err = a.StartBody(ctx, handle)
	if err != nil {
		t.Fatalf("StartBody failed: %v", err)
	}

	status, err := a.GetBodyStatus(ctx, handle)
	if err != nil {
		t.Fatalf("GetBodyStatus failed: %v", err)
	}
	if status.State != orchestrator.StateRunning {
		t.Errorf("status.State = %q, want %q", status.State, orchestrator.StateRunning)
	}
}

func TestDockerAdapterExec(t *testing.T) {
	skipIfNoDocker(t)
	ensureAlpineImage(t)

	a := docker.New(docker.Config{})
	ctx := context.Background()

	spec := orchestrator.BodySpec{
		Image:    "alpine:latest",
		Cmd:      []string{"sh", "-c", "sleep 300"},
		MemoryMB: 64,
	}

	handle, err := a.ScheduleBody(ctx, spec)
	if err != nil {
		t.Fatalf("ScheduleBody failed: %v", err)
	}
	defer a.DestroyBody(ctx, handle)

	err = a.StartBody(ctx, handle)
	if err != nil {
		t.Fatalf("StartBody failed: %v", err)
	}

	time.Sleep(1 * time.Second)

	result, err := a.Exec(ctx, handle, []string{"echo", "hello"})
	if err != nil {
		t.Fatalf("Exec failed: %v", err)
	}
	if result.ExitCode != 0 {
		t.Errorf("Exec exit code = %d, want 0", result.ExitCode)
	}
	if !strings.Contains(result.Stdout, "hello") {
		t.Errorf("Exec stdout = %q, want to contain 'hello'", result.Stdout)
	}
}

func TestDockerAdapterExportImport(t *testing.T) {
	skipIfNoDocker(t)
	ensureAlpineImage(t)

	a := docker.New(docker.Config{})
	ctx := context.Background()

	spec := orchestrator.BodySpec{
		Image:    "alpine:latest",
		Cmd:      []string{"sh", "-c", "sleep 300"},
		MemoryMB: 64,
	}

	handle, err := a.ScheduleBody(ctx, spec)
	if err != nil {
		t.Fatalf("ScheduleBody failed: %v", err)
	}
	defer a.DestroyBody(ctx, handle)

	err = a.StartBody(ctx, handle)
	if err != nil {
		t.Fatalf("StartBody failed: %v", err)
	}

	rc, err := a.ExportFilesystem(ctx, handle)
	if err != nil {
		t.Fatalf("ExportFilesystem failed: %v", err)
	}
	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("Read export failed: %v", err)
	}
	if len(data) == 0 {
		t.Error("ExportFilesystem returned empty data")
	}
}

func TestCompileTimeInterfaceCheck(t *testing.T) {
	var _ orchestrator.OrchestratorAdapter = (*docker.Adapter)(nil)
}

func TestDockerExtensions(t *testing.T) {
	a := docker.New(docker.Config{})

	if !orchestrator.HasCapability[orchestrator.Exporter](a) {
		t.Error("Adapter should implement Exporter")
	}
	if !orchestrator.HasCapability[orchestrator.Importer](a) {
		t.Error("Adapter should implement Importer")
	}
	if !orchestrator.HasCapability[orchestrator.Executor](a) {
		t.Error("Adapter should implement Executor")
	}
}

func TestExtensionCompileTimeInterfaceChecks(t *testing.T) {
	var _ orchestrator.Exporter = (*docker.Adapter)(nil)
	var _ orchestrator.Importer = (*docker.Adapter)(nil)
	var _ orchestrator.Executor = (*docker.Adapter)(nil)
}

func TestScheduleBodyErrorWithoutDocker(t *testing.T) {
	a := docker.New(docker.Config{SocketPath: "/nonexistent/docker.sock"})
	ctx := context.Background()

	_, err := a.ScheduleBody(ctx, orchestrator.BodySpec{Image: "alpine"})
	if err == nil {
		t.Error("ScheduleBody should fail when Docker is not available")
	}
}

func TestStartBodyErrorWithoutDocker(t *testing.T) {
	a := docker.New(docker.Config{SocketPath: "/nonexistent/docker.sock"})
	ctx := context.Background()

	err := a.StartBody(ctx, orchestrator.Handle("nonexistent"))
	if err == nil {
		t.Error("StartBody should fail when Docker is not available")
	}
}

func TestStopBodyErrorWithoutDocker(t *testing.T) {
	a := docker.New(docker.Config{SocketPath: "/nonexistent/docker.sock"})
	ctx := context.Background()

	err := a.StopBody(ctx, orchestrator.Handle("nonexistent"))
	if err == nil {
		t.Error("StopBody should fail when Docker is not available")
	}
}

func TestDestroyBodyErrorWithoutDocker(t *testing.T) {
	a := docker.New(docker.Config{SocketPath: "/nonexistent/docker.sock"})
	ctx := context.Background()

	err := a.DestroyBody(ctx, orchestrator.Handle("nonexistent"))
	if err == nil {
		t.Error("DestroyBody should fail when Docker is not available")
	}
}

func TestGetBodyStatusErrorWithoutDocker(t *testing.T) {
	a := docker.New(docker.Config{SocketPath: "/nonexistent/docker.sock"})
	ctx := context.Background()

	_, err := a.GetBodyStatus(ctx, orchestrator.Handle("nonexistent"))
	if err == nil {
		t.Error("GetBodyStatus should fail when Docker is not available")
	}
}

func TestExecErrorWithoutDocker(t *testing.T) {
	a := docker.New(docker.Config{SocketPath: "/nonexistent/docker.sock"})
	ctx := context.Background()

	_, err := a.Exec(ctx, orchestrator.Handle("nonexistent"), []string{"echo", "hi"})
	if err == nil {
		t.Error("Exec should fail when Docker is not available")
	}
}

func TestExportFilesystemErrorWithoutDocker(t *testing.T) {
	a := docker.New(docker.Config{SocketPath: "/nonexistent/docker.sock"})
	ctx := context.Background()

	_, err := a.ExportFilesystem(ctx, orchestrator.Handle("nonexistent"))
	if err == nil {
		t.Error("ExportFilesystem should fail when Docker is not available")
	}
}

func TestImportFilesystemErrorWithoutDocker(t *testing.T) {
	a := docker.New(docker.Config{SocketPath: "/nonexistent/docker.sock"})
	ctx := context.Background()

	err := a.ImportFilesystem(ctx, orchestrator.Handle("nonexistent"), strings.NewReader(""))
	if err == nil {
		t.Error("ImportFilesystem should fail when Docker is not available")
	}
}
