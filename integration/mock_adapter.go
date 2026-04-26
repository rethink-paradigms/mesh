//go:build integration

package integration

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/rethink-paradigms/mesh/internal/adapter"
)

type mockSubstrateAdapter struct {
	mu        sync.Mutex
	created   []adapter.Handle
	started   []adapter.Handle
	stopped   []adapter.Handle
	destroyed []adapter.Handle
}

var _ adapter.SubstrateAdapter = (*mockSubstrateAdapter)(nil)

func (m *mockSubstrateAdapter) Create(_ context.Context, _ adapter.BodySpec) (adapter.Handle, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	handle := adapter.Handle(fmt.Sprintf("mock-%d", len(m.created)+1))
	m.created = append(m.created, handle)
	return handle, nil
}

func (m *mockSubstrateAdapter) Start(_ context.Context, id adapter.Handle) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.started = append(m.started, id)
	return nil
}

func (m *mockSubstrateAdapter) Stop(_ context.Context, id adapter.Handle, _ adapter.StopOpts) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stopped = append(m.stopped, id)
	return nil
}

func (m *mockSubstrateAdapter) Destroy(_ context.Context, id adapter.Handle) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.destroyed = append(m.destroyed, id)
	return nil
}

func (m *mockSubstrateAdapter) GetStatus(_ context.Context, _ adapter.Handle) (adapter.BodyStatus, error) {
	return adapter.BodyStatus{State: adapter.StateRunning, Uptime: time.Minute}, nil
}

func (m *mockSubstrateAdapter) Exec(_ context.Context, _ adapter.Handle, _ []string) (adapter.ExecResult, error) {
	return adapter.ExecResult{Stdout: "mock output", Stderr: "", ExitCode: 0}, nil
}

func (m *mockSubstrateAdapter) ExportFilesystem(_ context.Context, _ adapter.Handle) (io.ReadCloser, error) {
	r, w := io.Pipe()
	go func() { w.Write([]byte("mock tar data")); w.Close() }()
	return r, nil
}

func (m *mockSubstrateAdapter) ImportFilesystem(_ context.Context, _ adapter.Handle, _ io.Reader, _ adapter.ImportOpts) error {
	return nil
}

func (m *mockSubstrateAdapter) Inspect(_ context.Context, _ adapter.Handle) (adapter.ContainerMetadata, error) {
	return adapter.ContainerMetadata{Image: "alpine:latest", Platform: "linux/amd64"}, nil
}

func (m *mockSubstrateAdapter) Capabilities() adapter.AdapterCapabilities {
	return adapter.AdapterCapabilities{
		ExportFilesystem: true,
		ImportFilesystem: true,
		Inspect:          true,
	}
}
