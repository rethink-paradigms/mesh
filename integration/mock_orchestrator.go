//go:build integration

package integration

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/rethink-paradigms/mesh/internal/orchestrator"
	"github.com/rethink-paradigms/mesh/internal/provisioner"
)

type mockOrchestratorAdapter struct {
	mu         sync.Mutex
	created    []orchestrator.Handle
	started    []orchestrator.Handle
	stopped    []orchestrator.Handle
	destroyed  []orchestrator.Handle
	importedTo []orchestrator.Handle
	substrate  string
}

var _ orchestrator.OrchestratorAdapter = (*mockOrchestratorAdapter)(nil)
var _ orchestrator.Exporter = (*mockOrchestratorAdapter)(nil)
var _ orchestrator.Importer = (*mockOrchestratorAdapter)(nil)
var _ orchestrator.Inspector = (*mockOrchestratorAdapter)(nil)
var _ orchestrator.Executor = (*mockOrchestratorAdapter)(nil)

func (m *mockOrchestratorAdapter) ScheduleBody(_ context.Context, _ orchestrator.BodySpec) (orchestrator.Handle, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	handle := orchestrator.Handle(fmt.Sprintf("mock-%d", len(m.created)+1))
	m.created = append(m.created, handle)
	return handle, nil
}

func (m *mockOrchestratorAdapter) StartBody(_ context.Context, id orchestrator.Handle) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.started = append(m.started, id)
	return nil
}

func (m *mockOrchestratorAdapter) StopBody(_ context.Context, id orchestrator.Handle) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stopped = append(m.stopped, id)
	return nil
}

func (m *mockOrchestratorAdapter) DestroyBody(_ context.Context, id orchestrator.Handle) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.destroyed = append(m.destroyed, id)
	return nil
}

func (m *mockOrchestratorAdapter) GetBodyStatus(_ context.Context, _ orchestrator.Handle) (orchestrator.BodyStatus, error) {
	return orchestrator.BodyStatus{State: orchestrator.StateRunning, Uptime: time.Minute}, nil
}

func (m *mockOrchestratorAdapter) Exec(_ context.Context, _ orchestrator.Handle, _ []string) (orchestrator.ExecResult, error) {
	return orchestrator.ExecResult{Stdout: "mock output", Stderr: "", ExitCode: 0}, nil
}

func (m *mockOrchestratorAdapter) ExportFilesystem(_ context.Context, _ orchestrator.Handle) (io.ReadCloser, error) {
	var tarBuf bytes.Buffer
	tw := tar.NewWriter(&tarBuf)
	content := []byte("hello from mesh")
	hdr := &tar.Header{
		Name: "test.txt",
		Mode: 0644,
		Size: int64(len(content)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return nil, err
	}
	if _, err := tw.Write(content); err != nil {
		return nil, err
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}

	return io.NopCloser(bytes.NewReader(tarBuf.Bytes())), nil
}

func (m *mockOrchestratorAdapter) ImportFilesystem(_ context.Context, id orchestrator.Handle, _ io.Reader) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.importedTo = append(m.importedTo, id)
	return nil
}

func (m *mockOrchestratorAdapter) Inspect(_ context.Context, _ orchestrator.Handle) (orchestrator.ContainerMetadata, error) {
	return orchestrator.ContainerMetadata{Image: "alpine:latest", Platform: "linux/amd64"}, nil
}

func (m *mockOrchestratorAdapter) Name() string {
	if m.substrate != "" {
		return m.substrate
	}
	return "mock"
}

func (m *mockOrchestratorAdapter) IsHealthy(_ context.Context) bool {
	return true
}

type mockProvisionerAdapter struct {
	name string
}

var _ provisioner.ProvisionerAdapter = (*mockProvisionerAdapter)(nil)

func (m *mockProvisionerAdapter) CreateMachine(_ context.Context, _ provisioner.MachineSpec, _ string) (provisioner.MachineID, error) {
	return provisioner.MachineID("mock-machine-1"), nil
}

func (m *mockProvisionerAdapter) DestroyMachine(_ context.Context, _ provisioner.MachineID) error {
	return nil
}

func (m *mockProvisionerAdapter) GetMachineStatus(_ context.Context, _ provisioner.MachineID) (provisioner.MachineStatus, error) {
	return provisioner.MachineStatus{State: "running", ID: "mock-machine-1"}, nil
}

func (m *mockProvisionerAdapter) ListMachines(_ context.Context) ([]provisioner.MachineInfo, error) {
	return nil, nil
}

func (m *mockProvisionerAdapter) Name() string {
	if m.name != "" {
		return m.name
	}
	return "mock-provisioner"
}

func (m *mockProvisionerAdapter) IsHealthy(_ context.Context) bool {
	return true
}
