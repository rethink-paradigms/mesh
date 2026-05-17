package service

import (
	"context"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/rethink-paradigms/mesh/internal/body"
	"github.com/rethink-paradigms/mesh/internal/orchestrator"
	"github.com/rethink-paradigms/mesh/internal/store"
	"github.com/stretchr/testify/assert"
)

type mockOrchAdapter struct {
	handle      orchestrator.Handle
	status      orchestrator.BodyStatus
	execOutputs map[string]orchestrator.ExecResult
}

func (m *mockOrchAdapter) ScheduleBody(_ context.Context, _ orchestrator.BodySpec) (orchestrator.Handle, error) {
	if m.handle == "" {
		return "mock-handle-1", nil
	}
	return m.handle, nil
}
func (m *mockOrchAdapter) StartBody(_ context.Context, _ orchestrator.Handle) error   { return nil }
func (m *mockOrchAdapter) StopBody(_ context.Context, _ orchestrator.Handle) error    { return nil }
func (m *mockOrchAdapter) DestroyBody(_ context.Context, _ orchestrator.Handle) error { return nil }
func (m *mockOrchAdapter) GetBodyStatus(_ context.Context, _ orchestrator.Handle) (orchestrator.BodyStatus, error) {
	if m.status.State != "" {
		return m.status, nil
	}
	return orchestrator.BodyStatus{State: orchestrator.StateRunning}, nil
}
func (m *mockOrchAdapter) Name() string                     { return "mock" }
func (m *mockOrchAdapter) IsHealthy(_ context.Context) bool { return true }

func (m *mockOrchAdapter) ExportFilesystem(_ context.Context, _ orchestrator.Handle) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("")), nil
}
func (m *mockOrchAdapter) ImportFilesystem(_ context.Context, _ orchestrator.Handle, _ io.Reader) error {
	return nil
}
func (m *mockOrchAdapter) Inspect(_ context.Context, _ orchestrator.Handle) (orchestrator.ContainerMetadata, error) {
	return orchestrator.ContainerMetadata{}, nil
}
func (m *mockOrchAdapter) Exec(_ context.Context, _ orchestrator.Handle, cmd []string) (orchestrator.ExecResult, error) {
	key := strings.Join(cmd, " ")
	if m.execOutputs != nil {
		if result, ok := m.execOutputs[key]; ok {
			return result, nil
		}
	}
	return orchestrator.ExecResult{Stdout: "ok", ExitCode: 0}, nil
}

func tempStore(t *testing.T) *store.Store {
	t.Helper()
	f, err := os.CreateTemp("", "body-service-test-*.db")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	path := f.Name()
	f.Close()
	s, err := store.Open(path)
	if err != nil {
		os.Remove(path)
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() {
		s.Close()
		os.Remove(path)
	})
	return s
}

func testBodyManager(t *testing.T, s *store.Store) *body.BodyManager {
	t.Helper()
	return body.NewBodyManager(s, &mockOrchAdapter{}, "")
}

func TestStop_NotRunning_ReturnsConflictError(t *testing.T) {
	s := tempStore(t)
	bm := testBodyManager(t, s)
	ctx := context.Background()

	created, err := bm.Create(ctx, "test-stop", orchestrator.BodySpec{Image: "alpine"})
	assert.NoError(t, err)

	err = bm.Stop(ctx, created.ID, orchestrator.StopOpts{})
	assert.NoError(t, err)

	svc := NewBodyService(bm, s, nil)
	err = svc.Stop(ctx, created.ID)

	var conflictErr *ConflictError
	assert.ErrorAs(t, err, &conflictErr)
	assert.Equal(t, "Stopped", conflictErr.State)
	assert.Equal(t, "Running or Starting", conflictErr.Required)
}

func TestStart_NotStopped_ReturnsConflictError(t *testing.T) {
	s := tempStore(t)
	bm := testBodyManager(t, s)
	ctx := context.Background()

	created, err := bm.Create(ctx, "test-start", orchestrator.BodySpec{Image: "alpine"})
	assert.NoError(t, err)

	svc := NewBodyService(bm, s, nil)
	err = svc.Start(ctx, created.ID)

	var conflictErr *ConflictError
	assert.ErrorAs(t, err, &conflictErr)
	assert.Equal(t, "Running", conflictErr.State)
	assert.Equal(t, "Stopped", conflictErr.Required)
}

func TestCreate_EmptyName_ReturnsValidationError(t *testing.T) {
	s := tempStore(t)
	bm := testBodyManager(t, s)
	svc := NewBodyService(bm, s, nil)
	ctx := context.Background()

	_, err := svc.Create(ctx, "", "alpine", orchestrator.BodySpec{})

	var valErr *ValidationError
	assert.ErrorAs(t, err, &valErr)
	assert.Equal(t, "name", valErr.Field)
}

func TestCreate_EmptyImage_ReturnsValidationError(t *testing.T) {
	s := tempStore(t)
	bm := testBodyManager(t, s)
	svc := NewBodyService(bm, s, nil)
	ctx := context.Background()

	_, err := svc.Create(ctx, "test-body", "", orchestrator.BodySpec{})

	var valErr *ValidationError
	assert.ErrorAs(t, err, &valErr)
	assert.Equal(t, "image", valErr.Field)
}

func TestExec_NotRunning_ReturnsConflictError(t *testing.T) {
	s := tempStore(t)
	bm := testBodyManager(t, s)
	ctx := context.Background()

	created, err := bm.Create(ctx, "test-exec", orchestrator.BodySpec{Image: "alpine"})
	assert.NoError(t, err)

	err = bm.Stop(ctx, created.ID, orchestrator.StopOpts{})
	assert.NoError(t, err)

	svc := NewBodyService(bm, s, nil)
	_, err = svc.Exec(ctx, created.ID, []string{"echo", "hello"})

	var conflictErr *ConflictError
	assert.ErrorAs(t, err, &conflictErr)
	assert.Equal(t, "Stopped", conflictErr.State)
	assert.Equal(t, "Running", conflictErr.Required)
}

func TestGet_NotFound_ReturnsNotFoundError(t *testing.T) {
	s := tempStore(t)
	bm := testBodyManager(t, s)
	svc := NewBodyService(bm, s, nil)
	ctx := context.Background()

	_, err := svc.Get(ctx, "nonexistent")

	var notFoundErr *NotFoundError
	assert.ErrorAs(t, err, &notFoundErr)
	assert.Equal(t, "nonexistent", notFoundErr.ID)
}

func TestDestroy_Running_ReturnsConflictError(t *testing.T) {
	s := tempStore(t)
	bm := testBodyManager(t, s)
	ctx := context.Background()

	created, err := bm.Create(ctx, "test-destroy", orchestrator.BodySpec{Image: "alpine"})
	assert.NoError(t, err)

	svc := NewBodyService(bm, s, nil)
	err = svc.Destroy(ctx, created.ID)

	var conflictErr *ConflictError
	assert.ErrorAs(t, err, &conflictErr)
	assert.Equal(t, "Running", conflictErr.State)
	assert.Equal(t, "Stopped or Error", conflictErr.Required)
}

func TestStop_BodyNotFound_ReturnsNotFoundError(t *testing.T) {
	s := tempStore(t)
	bm := testBodyManager(t, s)
	svc := NewBodyService(bm, s, nil)
	ctx := context.Background()

	err := svc.Stop(ctx, "nonexistent")

	var notFoundErr *NotFoundError
	assert.ErrorAs(t, err, &notFoundErr)
	assert.Equal(t, "nonexistent", notFoundErr.ID)
}

func TestStart_BodyNotFound_ReturnsNotFoundError(t *testing.T) {
	s := tempStore(t)
	bm := testBodyManager(t, s)
	svc := NewBodyService(bm, s, nil)
	ctx := context.Background()

	err := svc.Start(ctx, "nonexistent")

	var notFoundErr *NotFoundError
	assert.ErrorAs(t, err, &notFoundErr)
	assert.Equal(t, "nonexistent", notFoundErr.ID)
}

func TestDestroy_BodyNotFound_ReturnsNotFoundError(t *testing.T) {
	s := tempStore(t)
	bm := testBodyManager(t, s)
	svc := NewBodyService(bm, s, nil)
	ctx := context.Background()

	err := svc.Destroy(ctx, "nonexistent")

	var notFoundErr *NotFoundError
	assert.ErrorAs(t, err, &notFoundErr)
	assert.Equal(t, "nonexistent", notFoundErr.ID)
}

func TestExec_BodyNotFound_ReturnsNotFoundError(t *testing.T) {
	s := tempStore(t)
	bm := testBodyManager(t, s)
	svc := NewBodyService(bm, s, nil)
	ctx := context.Background()

	_, err := svc.Exec(ctx, "nonexistent", []string{"echo", "hello"})

	var notFoundErr *NotFoundError
	assert.ErrorAs(t, err, &notFoundErr)
	assert.Equal(t, "nonexistent", notFoundErr.ID)
}

func TestCreateBodyWithDefaultOrchestrator(t *testing.T) {
	s := tempStore(t)

	reg := orchestrator.NewRegistry()
	mockDocker := &mockOrchAdapter{}
	mockNomad := &mockOrchAdapter{}
	_ = reg.Register("docker", mockDocker)
	_ = reg.Register("nomad", mockNomad)
	_ = reg.SetDefault("docker")

	bm := body.NewBodyManager(s, mockDocker, "")
	svc := NewBodyService(bm, s, reg)
	ctx := context.Background()

	b, err := svc.Create(ctx, "test-multi", "alpine", orchestrator.BodySpec{})
	assert.NoError(t, err)
	assert.Equal(t, "mock", b.Substrate)
}

func TestCreateBodyMultiOrchNoDefault(t *testing.T) {
	s := tempStore(t)

	reg := orchestrator.NewRegistry()
	mockDocker := &mockOrchAdapter{}
	mockNomad := &mockOrchAdapter{}
	_ = reg.Register("docker", mockDocker)
	_ = reg.Register("nomad", mockNomad)

	bm := body.NewBodyManager(s, mockDocker, "")
	svc := NewBodyService(bm, s, reg)
	ctx := context.Background()

	_, err := svc.Create(ctx, "test-no-default", "alpine", orchestrator.BodySpec{})

	var valErr *ValidationError
	assert.ErrorAs(t, err, &valErr)
	assert.Equal(t, "substrate", valErr.Field)
	assert.Contains(t, valErr.Message, "docker")
	assert.Contains(t, valErr.Message, "nomad")
}
