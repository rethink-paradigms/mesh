package docker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rethink-paradigms/mesh/internal/orchestrator"
)

var _ orchestrator.OrchestratorAdapter = (*Adapter)(nil)
var _ orchestrator.Exporter = (*Adapter)(nil)
var _ orchestrator.Importer = (*Adapter)(nil)
var _ orchestrator.Executor = (*Adapter)(nil)

type Adapter struct {
	mu     sync.Mutex
	client *http.Client
	config Config
}

type Config struct {
	SocketPath string
}

func New(cfg Config) *Adapter {
	return &Adapter{config: cfg}
}

func NewFromEnv() *Adapter {
	cfg := Config{}
	if host := os.Getenv("DOCKER_HOST"); host != "" {
		cfg.SocketPath = host
	}
	return New(cfg)
}

func (a *Adapter) getClient() (*http.Client, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.client != nil {
		return a.client, nil
	}

	socketPath := a.config.SocketPath
	if socketPath == "" {
		socketPath = "/var/run/docker.sock"
	}
	if strings.HasPrefix(socketPath, "unix://") {
		socketPath = strings.TrimPrefix(socketPath, "unix://")
	}

	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return net.Dial("unix", socketPath)
		},
	}

	a.client = &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
	}
	return a.client, nil
}

func (a *Adapter) apiURL(path string) string {
	return "http://localhost/v1.43" + path
}

func (a *Adapter) doRequest(ctx context.Context, method, url string, body io.Reader) (*http.Response, error) {
	client, err := a.getClient()
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("docker: create request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("docker: request failed: %w", err)
	}
	return resp, nil
}

func (a *Adapter) ScheduleBody(ctx context.Context, spec orchestrator.BodySpec) (orchestrator.Handle, error) {
	bodyUUID := uuid.New().String()
	containerName := "mesh-" + bodyUUID

	env := make([]string, 0, len(spec.Env))
	for k, v := range spec.Env {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}

	exposedPorts := make(map[string]struct{})
	portBindings := make(map[string][]map[string]string)

	for _, p := range spec.Ports {
		if !p.Expose {
			continue
		}
		proto := p.Protocol
		if proto == "" {
			proto = "tcp"
		}
		portKey := fmt.Sprintf("%d/%s", p.ContainerPort, proto)
		exposedPorts[portKey] = struct{}{}

		binding := map[string]string{
			"HostIp":   "0.0.0.0",
			"HostPort": strconv.Itoa(p.HostPort),
		}
		portBindings[portKey] = append(portBindings[portKey], binding)
	}

	createBody := map[string]interface{}{
		"Image": spec.Image,
		"Cmd":   spec.Cmd,
		"Env":   env,
		"HostConfig": map[string]interface{}{
			"Memory":     int64(spec.MemoryMB) * 1024 * 1024,
			"CpuShares":  spec.CPUShares,
			"PortBindings": portBindings,
		},
	}

	if spec.Workdir != "" {
		createBody["WorkingDir"] = spec.Workdir
	}

	if len(exposedPorts) > 0 {
		createBody["ExposedPorts"] = exposedPorts
	}

	jsonBody, err := json.Marshal(createBody)
	if err != nil {
		return "", fmt.Errorf("docker: marshal create body: %w", err)
	}

	url := a.apiURL(fmt.Sprintf("/containers/create?name=%s", containerName))
	resp, err := a.doRequest(ctx, "POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return "", fmt.Errorf("docker: schedule body: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("docker: schedule body: status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		ID string `json:"Id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("docker: decode create response: %w", err)
	}

	return orchestrator.Handle(containerName), nil
}

func (a *Adapter) StartBody(ctx context.Context, id orchestrator.Handle) error {
	containerID, err := a.resolveContainerID(ctx, id)
	if err != nil {
		return err
	}

	url := a.apiURL(fmt.Sprintf("/containers/%s/start", containerID))
	resp, err := a.doRequest(ctx, "POST", url, nil)
	if err != nil {
		return fmt.Errorf("docker: start body: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("docker: start body: status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

func (a *Adapter) StopBody(ctx context.Context, id orchestrator.Handle) error {
	containerID, err := a.resolveContainerID(ctx, id)
	if err != nil {
		return err
	}

	url := a.apiURL(fmt.Sprintf("/containers/%s/stop?t=30", containerID))
	resp, err := a.doRequest(ctx, "POST", url, nil)
	if err != nil {
		return fmt.Errorf("docker: stop body: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotModified {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("docker: stop body: status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

func (a *Adapter) DestroyBody(ctx context.Context, id orchestrator.Handle) error {
	containerID, err := a.resolveContainerID(ctx, id)
	if err != nil {
		return err
	}

	url := a.apiURL(fmt.Sprintf("/containers/%s?force=true", containerID))
	resp, err := a.doRequest(ctx, "DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("docker: destroy body: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("docker: destroy body: status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

func (a *Adapter) GetBodyStatus(ctx context.Context, id orchestrator.Handle) (orchestrator.BodyStatus, error) {
	containerID, err := a.resolveContainerID(ctx, id)
	if err != nil {
		return orchestrator.BodyStatus{}, err
	}

	url := a.apiURL(fmt.Sprintf("/containers/%s/json", containerID))
	resp, err := a.doRequest(ctx, "GET", url, nil)
	if err != nil {
		return orchestrator.BodyStatus{}, fmt.Errorf("docker: get body status: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return orchestrator.BodyStatus{}, fmt.Errorf("docker: get body status: status %d: %s", resp.StatusCode, string(body))
	}

	var inspect struct {
		State struct {
			Status     string    `json:"Status"`
			Running    bool      `json:"Running"`
			StartedAt  time.Time `json:"StartedAt"`
			FinishedAt time.Time `json:"FinishedAt"`
			ExitCode   int       `json:"ExitCode"`
		} `json:"State"`
		HostConfig struct {
			Memory int64 `json:"Memory"`
		} `json:"HostConfig"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&inspect); err != nil {
		return orchestrator.BodyStatus{}, fmt.Errorf("docker: decode inspect response: %w", err)
	}

	status := orchestrator.BodyStatus{
		State:    mapDockerState(inspect.State.Status),
		MemoryMB: inspect.HostConfig.Memory / (1024 * 1024),
	}

	if inspect.State.Running && !inspect.State.StartedAt.IsZero() {
		status.StartedAt = inspect.State.StartedAt
		status.Uptime = time.Since(inspect.State.StartedAt)
	}

	return status, nil
}

func (a *Adapter) Name() string {
	return "docker"
}

func (a *Adapter) IsHealthy(ctx context.Context) bool {
	url := a.apiURL("/info")
	resp, err := a.doRequest(ctx, "GET", url, nil)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func (a *Adapter) ExportFilesystem(ctx context.Context, id orchestrator.Handle) (io.ReadCloser, error) {
	containerID, err := a.resolveContainerID(ctx, id)
	if err != nil {
		return nil, err
	}

	url := a.apiURL(fmt.Sprintf("/containers/%s/export", containerID))
	resp, err := a.doRequest(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("docker: export filesystem: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("docker: export filesystem: status %d: %s", resp.StatusCode, string(body))
	}
	return resp.Body, nil
}

func (a *Adapter) ImportFilesystem(ctx context.Context, id orchestrator.Handle, tarball io.Reader) error {
	containerID, err := a.resolveContainerID(ctx, id)
	if err != nil {
		return err
	}

	url := a.apiURL(fmt.Sprintf("/containers/%s/archive?path=/", containerID))
	resp, err := a.doRequest(ctx, "PUT", url, tarball)
	if err != nil {
		return fmt.Errorf("docker: import filesystem: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("docker: import filesystem: status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

func (a *Adapter) Exec(ctx context.Context, id orchestrator.Handle, cmd []string) (orchestrator.ExecResult, error) {
	containerID, err := a.resolveContainerID(ctx, id)
	if err != nil {
		return orchestrator.ExecResult{}, err
	}

	execBody := map[string]interface{}{
		"Cmd":          cmd,
		"AttachStdout": true,
		"AttachStderr": true,
	}
	jsonBody, err := json.Marshal(execBody)
	if err != nil {
		return orchestrator.ExecResult{}, fmt.Errorf("docker: marshal exec body: %w", err)
	}

	url := a.apiURL(fmt.Sprintf("/containers/%s/exec", containerID))
	resp, err := a.doRequest(ctx, "POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return orchestrator.ExecResult{}, fmt.Errorf("docker: exec: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return orchestrator.ExecResult{}, fmt.Errorf("docker: exec: status %d: %s", resp.StatusCode, string(body))
	}

	var execResp struct {
		ID string `json:"Id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&execResp); err != nil {
		return orchestrator.ExecResult{}, fmt.Errorf("docker: decode exec response: %w", err)
	}

	startBody := map[string]interface{}{
		"Detach": false,
		"Tty":    false,
	}
	jsonStart, err := json.Marshal(startBody)
	if err != nil {
		return orchestrator.ExecResult{}, fmt.Errorf("docker: marshal exec start body: %w", err)
	}

	startURL := a.apiURL(fmt.Sprintf("/exec/%s/start", execResp.ID))
	startResp, err := a.doRequest(ctx, "POST", startURL, bytes.NewReader(jsonStart))
	if err != nil {
		return orchestrator.ExecResult{}, fmt.Errorf("docker: exec start: %w", err)
	}
	defer startResp.Body.Close()

	if startResp.StatusCode != http.StatusOK && startResp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(startResp.Body)
		return orchestrator.ExecResult{}, fmt.Errorf("docker: exec start: status %d: %s", startResp.StatusCode, string(body))
	}

	stdout, err := io.ReadAll(startResp.Body)
	if err != nil {
		return orchestrator.ExecResult{}, fmt.Errorf("docker: read exec output: %w", err)
	}

	inspectURL := a.apiURL(fmt.Sprintf("/exec/%s/json", execResp.ID))
	inspectResp, err := a.doRequest(ctx, "GET", inspectURL, nil)
	if err != nil {
		return orchestrator.ExecResult{}, fmt.Errorf("docker: exec inspect: %w", err)
	}
	defer inspectResp.Body.Close()

	var inspect struct {
		ExitCode int `json:"ExitCode"`
	}
	if err := json.NewDecoder(inspectResp.Body).Decode(&inspect); err != nil {
		return orchestrator.ExecResult{}, fmt.Errorf("docker: decode exec inspect: %w", err)
	}

	return orchestrator.ExecResult{
		Stdout:   string(stdout),
		ExitCode: inspect.ExitCode,
	}, nil
}

func (a *Adapter) resolveContainerID(ctx context.Context, id orchestrator.Handle) (string, error) {
	name := string(id)
	url := a.apiURL(fmt.Sprintf("/containers/%s/json", name))
	resp, err := a.doRequest(ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("docker: resolve container: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		var inspect struct {
			ID string `json:"Id"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&inspect); err != nil {
			return "", fmt.Errorf("docker: decode resolve response: %w", err)
		}
		return inspect.ID, nil
	}

	if resp.StatusCode == http.StatusNotFound {
		return "", fmt.Errorf("docker: container %q not found", name)
	}

	body, _ := io.ReadAll(resp.Body)
	return "", fmt.Errorf("docker: resolve container %q: status %d: %s", name, resp.StatusCode, string(body))
}

func mapDockerState(status string) orchestrator.BodyState {
	switch status {
	case "running":
		return orchestrator.StateRunning
	case "exited":
		return orchestrator.StateStopped
	case "created":
		return orchestrator.StateCreated
	case "paused":
		return orchestrator.StateStopped
	case "restarting":
		return orchestrator.StateStarting
	case "removing":
		return orchestrator.StateStopping
	case "dead":
		return orchestrator.StateError
	default:
		return orchestrator.StateCreated
	}
}
