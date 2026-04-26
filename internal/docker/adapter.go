// Package docker provides a Docker adapter implementing the SubstrateAdapter interface
// for container lifecycle management. It uses the Docker SDK (moby) to manage containers
// with lazy client connection and thread-safe access.
package docker

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/rethink-paradigms/mesh/internal/adapter"
)

const defaultStopTimeout = 30

// Adapter implements adapter.SubstrateAdapter using the Docker SDK.
type Adapter struct {
	mu     sync.Mutex
	client *client.Client
}

// New creates a new Docker adapter with lazy client initialization.
func New() *Adapter {
	return &Adapter{}
}

// getClient returns a Docker client, creating one lazily on first use.
func (a *Adapter) getClient(ctx context.Context) (*client.Client, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.client != nil {
		return a.client, nil
	}
	c, err := client.NewClientWithOpts(
		client.FromEnv,
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return nil, fmt.Errorf("docker: create client: %w", err)
	}
	a.client = c
	return c, nil
}

// Create creates a new container from the given body spec and returns its handle.
func (a *Adapter) Create(ctx context.Context, spec adapter.BodySpec) (adapter.Handle, error) {
	cli, err := a.getClient(ctx)
	if err != nil {
		return "", err
	}

	var env []string
	for k, v := range spec.Env {
		env = append(env, k+"="+v)
	}

	config := &container.Config{
		Image:      spec.Image,
		Env:        env,
		Cmd:        spec.Cmd,
		WorkingDir: spec.Workdir,
	}

	hostConfig := &container.HostConfig{}
	if spec.MemoryMB > 0 {
		hostConfig.Memory = int64(spec.MemoryMB) * 1024 * 1024
	}
	if spec.CPUShares > 0 {
		hostConfig.CPUShares = int64(spec.CPUShares)
	}

	resp, err := cli.ContainerCreate(ctx, config, hostConfig, nil, nil, "")
	if err != nil {
		return "", fmt.Errorf("docker: create container: %w", err)
	}
	return adapter.Handle(resp.ID), nil
}

// Start starts a created container.
func (a *Adapter) Start(ctx context.Context, id adapter.Handle) error {
	cli, err := a.getClient(ctx)
	if err != nil {
		return err
	}
	if err := cli.ContainerStart(ctx, string(id), container.StartOptions{}); err != nil {
		return fmt.Errorf("docker: start container %s: %w", id, err)
	}
	return nil
}

// Stop stops a running container with a timeout.
func (a *Adapter) Stop(ctx context.Context, id adapter.Handle, opts adapter.StopOpts) error {
	cli, err := a.getClient(ctx)
	if err != nil {
		return err
	}

	timeoutSec := int(defaultStopTimeout)
	if opts.Timeout > 0 {
		timeoutSec = int(opts.Timeout.Seconds())
	}

	if err := cli.ContainerStop(ctx, string(id), container.StopOptions{Timeout: &timeoutSec}); err != nil {
		return fmt.Errorf("docker: stop container %s: %w", id, err)
	}
	return nil
}

// Destroy removes a container forcefully, including its volumes.
func (a *Adapter) Destroy(ctx context.Context, id adapter.Handle) error {
	cli, err := a.getClient(ctx)
	if err != nil {
		return err
	}
	if err := cli.ContainerRemove(ctx, string(id), container.RemoveOptions{
		Force:         true,
		RemoveVolumes: true,
	}); err != nil {
		return fmt.Errorf("docker: destroy container %s: %w", id, err)
	}
	return nil
}

// GetStatus returns the current status of a container.
func (a *Adapter) GetStatus(ctx context.Context, id adapter.Handle) (adapter.BodyStatus, error) {
	cli, err := a.getClient(ctx)
	if err != nil {
		return adapter.BodyStatus{}, err
	}

	inspect, err := cli.ContainerInspect(ctx, string(id))
	if err != nil {
		return adapter.BodyStatus{}, fmt.Errorf("docker: inspect container %s: %w", id, err)
	}

	state := mapDockerState(inspect.State.Status)
	status := adapter.BodyStatus{
		State: state,
	}

	if inspect.State.StartedAt != "" {
		startedAt, err := time.Parse(time.RFC3339Nano, inspect.State.StartedAt)
		if err == nil {
			status.StartedAt = startedAt
			if state == adapter.StateRunning {
				status.Uptime = time.Since(startedAt)
			} else if inspect.State.FinishedAt != "" {
				finishedAt, err := time.Parse(time.RFC3339Nano, inspect.State.FinishedAt)
				if err == nil {
					status.Uptime = finishedAt.Sub(startedAt)
				}
			}
		}
	}

	if inspect.HostConfig != nil && inspect.HostConfig.Memory > 0 {
		status.MemoryMB = inspect.HostConfig.Memory / (1024 * 1024)
	}

	return status, nil
}

// Exec runs a command inside a running container and returns the output.
func (a *Adapter) Exec(ctx context.Context, id adapter.Handle, cmd []string) (adapter.ExecResult, error) {
	cli, err := a.getClient(ctx)
	if err != nil {
		return adapter.ExecResult{}, err
	}

	execCreate, err := cli.ContainerExecCreate(ctx, string(id), container.ExecOptions{
		Cmd:          cmd,
		AttachStdout: true,
		AttachStderr: true,
	})
	if err != nil {
		return adapter.ExecResult{}, fmt.Errorf("docker: exec create in %s: %w", id, err)
	}

	hijacked, err := cli.ContainerExecAttach(ctx, execCreate.ID, container.ExecAttachOptions{})
	if err != nil {
		return adapter.ExecResult{}, fmt.Errorf("docker: exec attach in %s: %w", id, err)
	}
	defer hijacked.Close()

	output, err := io.ReadAll(hijacked.Reader)
	if err != nil {
		return adapter.ExecResult{}, fmt.Errorf("docker: exec read output from %s: %w", id, err)
	}

	execInspect, err := cli.ContainerExecInspect(ctx, execCreate.ID)
	if err != nil {
		return adapter.ExecResult{}, fmt.Errorf("docker: exec inspect in %s: %w", id, err)
	}

	// Docker multiplexes stdout and stderr in the hijacked response using
	// a simple framing protocol (8-byte header per frame). For a basic split,
	// we return the full output as stdout. Integration tests can verify exact
	// stream separation if needed.
	stdout, stderr := demuxDockerOutput(output)

	return adapter.ExecResult{
		Stdout:   stdout,
		Stderr:   stderr,
		ExitCode: execInspect.ExitCode,
	}, nil
}

// ExportFilesystem exports the container's filesystem as a tar archive.
func (a *Adapter) ExportFilesystem(ctx context.Context, id adapter.Handle) (io.ReadCloser, error) {
	cli, err := a.getClient(ctx)
	if err != nil {
		return nil, err
	}

	rc, err := cli.ContainerExport(ctx, string(id))
	if err != nil {
		return nil, fmt.Errorf("docker: export filesystem from %s: %w", id, err)
	}
	return rc, nil
}

// ImportFilesystem imports a tar archive into the container's root filesystem.
func (a *Adapter) ImportFilesystem(ctx context.Context, id adapter.Handle, tarball io.Reader, opts adapter.ImportOpts) error {
	cli, err := a.getClient(ctx)
	if err != nil {
		return err
	}

	copyOpts := container.CopyToContainerOptions{
		AllowOverwriteDirWithFile: opts.Overwrite,
	}
	if err := cli.CopyToContainer(ctx, string(id), "/", tarball, copyOpts); err != nil {
		return fmt.Errorf("docker: import filesystem to %s: %w", id, err)
	}
	return nil
}

// Inspect returns metadata about a container.
func (a *Adapter) Inspect(ctx context.Context, id adapter.Handle) (adapter.ContainerMetadata, error) {
	cli, err := a.getClient(ctx)
	if err != nil {
		return adapter.ContainerMetadata{}, err
	}

	inspect, err := cli.ContainerInspect(ctx, string(id))
	if err != nil {
		return adapter.ContainerMetadata{}, fmt.Errorf("docker: inspect container %s: %w", id, err)
	}

	envMap := make(map[string]string)
	for _, e := range inspect.Config.Env {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) == 2 {
			envMap[parts[0]] = parts[1]
		} else {
			envMap[parts[0]] = ""
		}
	}

	return adapter.ContainerMetadata{
		Image:    inspect.Config.Image,
		Env:      envMap,
		Cmd:      inspect.Config.Cmd,
		Workdir:  inspect.Config.WorkingDir,
		Platform: inspect.Platform,
	}, nil
}

// Capabilities returns the adapter's supported optional features.
func (a *Adapter) Capabilities() adapter.AdapterCapabilities {
	return adapter.AdapterCapabilities{
		ExportFilesystem: true,
		ImportFilesystem: true,
		Inspect:          true,
	}
}

// mapDockerState converts Docker container status strings to adapter BodyState.
func mapDockerState(status string) adapter.BodyState {
	switch status {
	case "created":
		return adapter.StateCreated
	case "running":
		return adapter.StateRunning
	case "paused":
		return adapter.StateRunning
	case "restarting":
		return adapter.StateStarting
	case "removing":
		return adapter.StateError
	case "exited":
		return adapter.StateStopped
	case "dead":
		return adapter.StateError
	default:
		return adapter.StateError
	}
}

// demuxDockerOutput splits Docker multiplexed output into stdout and stderr.
// Docker uses an 8-byte header per frame: [streamType(1)][padding(3)][size(4)].
// Stream type: 1=stdout, 2=stderr.
func demuxDockerOutput(data []byte) (stdout, stderr string) {
	if len(data) == 0 {
		return "", ""
	}

	if !looksLikeMultiplexed(data) {
		return string(data), ""
	}

	var outBuf, errBuf strings.Builder
	offset := 0
	for offset+8 <= len(data) {
		streamType := data[offset]
		size := int(data[offset+4])<<24 | int(data[offset+5])<<16 | int(data[offset+6])<<8 | int(data[offset+7])
		offset += 8

		if offset+size > len(data) {
			outBuf.Write(data[offset:])
			break
		}

		payload := data[offset : offset+size]
		offset += size

		switch streamType {
		case 1:
			outBuf.Write(payload)
		case 2:
			errBuf.Write(payload)
		default:
			outBuf.Write(payload)
		}
	}

	return outBuf.String(), errBuf.String()
}

// looksLikeMultiplexed checks whether data starts with a valid Docker
// multiplexed frame header: streamType in {0,1,2} and padding bytes are zero.
func looksLikeMultiplexed(data []byte) bool {
	if len(data) < 8 {
		return false
	}
	streamType := data[0]
	if streamType > 2 {
		return false
	}
	return data[1] == 0 && data[2] == 0 && data[3] == 0
}
