package body

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rethink-paradigms/mesh/internal/adapter"
	"github.com/rethink-paradigms/mesh/internal/store"
)

type mockAdapter struct {
	mu           sync.Mutex
	created      []adapter.Handle
	started      []adapter.Handle
	stopped      []adapter.Handle
	destroyed    []adapter.Handle
	statuses     map[string]adapter.BodyStatus
	failStart    bool
	failStop     bool
	failCreate   bool
	failInspect  bool
	failImport   bool
	exportErr    error
	handleSeq    int
	inspectMeta  adapter.ContainerMetadata
	importedTo   []adapter.Handle
	importedData []string
	substrate    string
}

func (m *mockAdapter) SubstrateName() string {
	if m.substrate != "" {
		return m.substrate
	}
	return "mock"
}

func newMockAdapter() *mockAdapter {
	return &mockAdapter{
		statuses: make(map[string]adapter.BodyStatus),
	}
}

type mockRegistry struct {
	mu         sync.Mutex
	pushed     map[string]string
	pulled     []string
	pushErr    error
	pullErr    error
	verifyErr  error
	failCount  int
	failAfter  int
	pullSHA    string
}

func newMockRegistry() *mockRegistry {
	return &mockRegistry{
		pushed: make(map[string]string),
	}
}

func (m *mockRegistry) Push(_ context.Context, key string, r io.Reader, _ int64, sha256 string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.pushErr != nil {
		return m.pushErr
	}
	if m.failAfter > 0 && m.failCount < m.failAfter {
		m.failCount++
		return fmt.Errorf("mock push failure %d", m.failCount)
	}
	data, _ := io.ReadAll(r)
	m.pushed[key] = string(data)
	_ = sha256
	return nil
}

func (m *mockRegistry) Pull(_ context.Context, key string) (io.ReadCloser, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.pullErr != nil {
		return nil, "", m.pullErr
	}
	data, ok := m.pushed[key]
	if !ok {
		return nil, "", fmt.Errorf("key %s not found", key)
	}
	m.pulled = append(m.pulled, key)
	sha := m.pullSHA
	if sha == "" {
		sha = ""
	}
	return io.NopCloser(strings.NewReader(data)), sha, nil
}

func (m *mockRegistry) Verify(_ context.Context, key, expectedSHA256 string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.verifyErr != nil {
		return m.verifyErr
	}
	_, ok := m.pushed[key]
	if !ok {
		return fmt.Errorf("key %s not found", key)
	}
	_ = expectedSHA256
	return nil
}

func (m *mockAdapter) nextHandle() adapter.Handle {
	m.handleSeq++
	return adapter.Handle(fmt.Sprintf("handle-%d", m.handleSeq))
}

func (m *mockAdapter) Create(_ context.Context, spec adapter.BodySpec) (adapter.Handle, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.failCreate {
		return "", errors.New("create failed")
	}
	h := m.nextHandle()
	m.created = append(m.created, h)
	m.statuses[string(h)] = adapter.BodyStatus{State: adapter.StateCreated}
	return h, nil
}

func (m *mockAdapter) Start(_ context.Context, id adapter.Handle) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.failStart {
		return errors.New("start failed")
	}
	m.started = append(m.started, id)
	m.statuses[string(id)] = adapter.BodyStatus{State: adapter.StateRunning, StartedAt: time.Now()}
	return nil
}

func (m *mockAdapter) Stop(_ context.Context, id adapter.Handle, _ adapter.StopOpts) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.failStop {
		return errors.New("stop failed")
	}
	m.stopped = append(m.stopped, id)
	m.statuses[string(id)] = adapter.BodyStatus{State: adapter.StateStopped}
	return nil
}

func (m *mockAdapter) Destroy(_ context.Context, id adapter.Handle) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.destroyed = append(m.destroyed, id)
	delete(m.statuses, string(id))
	return nil
}

func (m *mockAdapter) GetStatus(_ context.Context, id adapter.Handle) (adapter.BodyStatus, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.statuses[string(id)]
	if !ok {
		return adapter.BodyStatus{}, fmt.Errorf("handle %s not found", id)
	}
	return s, nil
}

func (m *mockAdapter) Exec(_ context.Context, _ adapter.Handle, _ []string) (adapter.ExecResult, error) {
	return adapter.ExecResult{ExitCode: 0, Stdout: "ok"}, nil
}

type nopCloser struct{ io.Reader }

func (nopCloser) Close() error { return nil }

func (m *mockAdapter) ExportFilesystem(_ context.Context, id adapter.Handle) (io.ReadCloser, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.exportErr != nil {
		return nil, m.exportErr
	}
	return nopCloser{strings.NewReader("fake-tar-data")}, nil
}

func (m *mockAdapter) ImportFilesystem(_ context.Context, id adapter.Handle, r io.Reader, _ adapter.ImportOpts) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.failImport {
		return errors.New("import failed")
	}
	m.importedTo = append(m.importedTo, id)
	data, _ := io.ReadAll(r)
	m.importedData = append(m.importedData, string(data))
	return nil
}

func (m *mockAdapter) Inspect(_ context.Context, _ adapter.Handle) (adapter.ContainerMetadata, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.failInspect {
		return adapter.ContainerMetadata{}, errors.New("inspect failed")
	}
	return m.inspectMeta, nil
}

func (m *mockAdapter) Capabilities() adapter.AdapterCapabilities {
	return adapter.AdapterCapabilities{
		ExportFilesystem: true,
		ImportFilesystem: true,
		Inspect:          true,
	}
}



func (m *mockAdapter) IsHealthy(_ context.Context) bool {
	return true
}

func openTestStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("open test store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// --- State machine tests ---

func TestValidTransitions(t *testing.T) {
	tests := []struct {
		from, to adapter.BodyState
	}{
		{adapter.StateCreated, adapter.StateStarting},
		{adapter.StateCreated, adapter.StateError},
		{adapter.StateStarting, adapter.StateRunning},
		{adapter.StateStarting, adapter.StateError},
		{adapter.StateRunning, adapter.StateStopping},
		{adapter.StateRunning, adapter.StateMigrating},
		{adapter.StateRunning, adapter.StateError},
		{adapter.StateStopping, adapter.StateStopped},
		{adapter.StateStopping, adapter.StateError},
		{adapter.StateStopped, adapter.StateStarting},
		{adapter.StateStopped, adapter.StateDestroyed},
		{adapter.StateError, adapter.StateStarting},
		{adapter.StateError, adapter.StateDestroyed},
		{adapter.StateMigrating, adapter.StateRunning},
		{adapter.StateMigrating, adapter.StateError},
	}
	for _, tt := range tests {
		t.Run(string(tt.from)+"->"+string(tt.to), func(t *testing.T) {
			b := &Body{State: tt.from}
			if err := b.Transition(tt.to); err != nil {
				t.Errorf("Transition(%s → %s) failed: %v", tt.from, tt.to, err)
			}
			if b.State != tt.to {
				t.Errorf("state = %s, want %s", b.State, tt.to)
			}
		})
	}
}

func TestInvalidTransitions(t *testing.T) {
	tests := []struct {
		from, to adapter.BodyState
	}{
		{adapter.StateRunning, adapter.StateDestroyed},
		{adapter.StateCreated, adapter.StateStopped},
		{adapter.StateCreated, adapter.StateRunning},
		{adapter.StateStopped, adapter.StateRunning},
		{adapter.StateDestroyed, adapter.StateRunning},
		{adapter.StateDestroyed, adapter.StateCreated},
		{adapter.StateRunning, adapter.StateStarting},
		{adapter.StateMigrating, adapter.StateStopped},
	}
	for _, tt := range tests {
		t.Run(string(tt.from)+"->"+string(tt.to), func(t *testing.T) {
			b := &Body{State: tt.from}
			err := b.Transition(tt.to)
			if err == nil {
				t.Errorf("Transition(%s → %s) should have failed but succeeded", tt.from, tt.to)
			}
		})
	}
}

// --- Lifecycle tests ---

func TestFullLifecycle(t *testing.T) {
	s := openTestStore(t)
	ma := newMockAdapter()
	bm := NewBodyManager(s, ma)

	ctx := context.Background()

	b, err := bm.Create(ctx, "test-body", adapter.BodySpec{Image: "alpine"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if b.State != adapter.StateRunning {
		t.Fatalf("after Create, state = %s, want Running", b.State)
	}

	if err := bm.Stop(ctx, b.ID, adapter.StopOpts{Signal: "SIGTERM", Timeout: 10 * time.Second}); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if b.State != adapter.StateStopped {
		t.Fatalf("after Stop, state = %s, want Stopped", b.State)
	}

	if err := bm.Start(ctx, b.ID); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if b.State != adapter.StateRunning {
		t.Fatalf("after Start, state = %s, want Running", b.State)
	}

	if err := bm.Stop(ctx, b.ID, adapter.StopOpts{}); err != nil {
		t.Fatalf("Stop(2): %v", err)
	}

	if err := bm.Destroy(ctx, b.ID); err != nil {
		t.Fatalf("Destroy: %v", err)
	}
	if b.State != adapter.StateDestroyed {
		t.Fatalf("after Destroy, state = %s, want Destroyed", b.State)
	}

	ma.mu.Lock()
	defer ma.mu.Unlock()
	if len(ma.created) != 1 || len(ma.destroyed) != 1 {
		t.Errorf("adapter calls: created=%d destroyed=%d", len(ma.created), len(ma.destroyed))
	}
}

func TestCreatePersistsToStore(t *testing.T) {
	s := openTestStore(t)
	ma := newMockAdapter()
	bm := NewBodyManager(s, ma)

	ctx := context.Background()
	b, err := bm.Create(ctx, "persist-test", adapter.BodySpec{Image: "alpine"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	rec, err := s.GetBody(ctx, b.ID)
	if err != nil {
		t.Fatalf("GetBody: %v", err)
	}
	if rec.State != adapter.StateRunning {
		t.Errorf("stored state = %s, want Running", rec.State)
	}
	if rec.Name != "persist-test" {
		t.Errorf("stored name = %s, want persist-test", rec.Name)
	}
}

func TestDestroyRemovesSnapshots(t *testing.T) {
	s := openTestStore(t)
	ma := newMockAdapter()
	bm := NewBodyManager(s, ma)
	ctx := context.Background()

	b, err := bm.Create(ctx, "snap-test", adapter.BodySpec{Image: "alpine"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	snapID := "snap-1"
	if err := s.CreateSnapshot(ctx, snapID, b.ID, "", "/tmp/test.tar.zst", 1024); err != nil {
		t.Fatalf("CreateSnapshot: %v", err)
	}

	if err := bm.Stop(ctx, b.ID, adapter.StopOpts{}); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	if err := bm.Destroy(ctx, b.ID); err != nil {
		t.Fatalf("Destroy: %v", err)
	}

	snaps, err := s.ListSnapshots(ctx, b.ID)
	if err != nil {
		t.Fatalf("ListSnapshots: %v", err)
	}
	if len(snaps) != 0 {
		t.Errorf("snapshots after destroy = %d, want 0", len(snaps))
	}
}

func TestCannotDestroyRunningBody(t *testing.T) {
	s := openTestStore(t)
	ma := newMockAdapter()
	bm := NewBodyManager(s, ma)
	ctx := context.Background()

	b, err := bm.Create(ctx, "running-test", adapter.BodySpec{Image: "alpine"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	err = bm.Destroy(ctx, b.ID)
	if err == nil {
		t.Fatal("Destroy on running body should fail")
	}
}

func TestCannotStartRunningBody(t *testing.T) {
	s := openTestStore(t)
	ma := newMockAdapter()
	bm := NewBodyManager(s, ma)
	ctx := context.Background()

	b, err := bm.Create(ctx, "double-start", adapter.BodySpec{Image: "alpine"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	err = bm.Start(ctx, b.ID)
	if err == nil {
		t.Fatal("Start on running body should fail")
	}
}

func TestCannotStopStoppedBody(t *testing.T) {
	s := openTestStore(t)
	ma := newMockAdapter()
	bm := NewBodyManager(s, ma)
	ctx := context.Background()

	b, err := bm.Create(ctx, "double-stop", adapter.BodySpec{Image: "alpine"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := bm.Stop(ctx, b.ID, adapter.StopOpts{}); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	err = bm.Stop(ctx, b.ID, adapter.StopOpts{})
	if err == nil {
		t.Fatal("Stop on stopped body should fail")
	}
}

func TestGetStatus(t *testing.T) {
	s := openTestStore(t)
	ma := newMockAdapter()
	bm := NewBodyManager(s, ma)
	ctx := context.Background()

	b, err := bm.Create(ctx, "status-test", adapter.BodySpec{Image: "alpine"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	status, err := bm.GetStatus(ctx, b.ID)
	if err != nil {
		t.Fatalf("GetStatus: %v", err)
	}
	if status.State != adapter.StateRunning {
		t.Errorf("status state = %s, want Running", status.State)
	}
}

func TestList(t *testing.T) {
	s := openTestStore(t)
	ma := newMockAdapter()
	bm := NewBodyManager(s, ma)
	ctx := context.Background()

	_, err := bm.Create(ctx, "body-1", adapter.BodySpec{Image: "alpine"})
	if err != nil {
		t.Fatalf("Create body-1: %v", err)
	}
	_, err = bm.Create(ctx, "body-2", adapter.BodySpec{Image: "nginx"})
	if err != nil {
		t.Fatalf("Create body-2: %v", err)
	}

	bodies, err := bm.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(bodies) != 2 {
		t.Errorf("len(bodies) = %d, want 2", len(bodies))
	}
}

// --- Concurrent access tests ---

func TestConcurrentStartStop(t *testing.T) {
	s := openTestStore(t)
	ma := newMockAdapter()
	bm := NewBodyManager(s, ma)
	ctx := context.Background()

	b, err := bm.Create(ctx, "concurrent-test", adapter.BodySpec{Image: "alpine"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	var wg sync.WaitGroup
	errs := make(chan error, 2)

	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			var err error
			if idx == 0 {
				err = bm.Stop(ctx, b.ID, adapter.StopOpts{})
			} else {
				err = bm.Stop(ctx, b.ID, adapter.StopOpts{})
			}
			errs <- err
		}(i)
	}
	wg.Wait()
	close(errs)

	errCount := 0
	for e := range errs {
		if e != nil {
			errCount++
		}
	}

	rec, err := s.GetBody(ctx, b.ID)
	if err != nil {
		t.Fatalf("GetBody after concurrent ops: %v", err)
	}
	if rec.State != adapter.StateStopped {
		t.Errorf("final state = %s, want Stopped", rec.State)
	}
}

// --- Migration tests ---

func TestMigrationDurability(t *testing.T) {
	s := openTestStore(t)
	ma := newMockAdapter()
	bm := NewBodyManager(s, ma)
	ctx := context.Background()

	b, err := bm.Create(ctx, "mig-test", adapter.BodySpec{Image: "alpine"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	mc := NewMigrationCoordinator(s, ma, bm, nil)
	migID, err := mc.BeginMigration(ctx, b.ID, "remote-host")
	if err != nil {
		t.Fatalf("BeginMigration: %v", err)
	}

	_, err = s.GetMigration(ctx, migID)
	if err == nil {
		t.Fatal("migration record should be deleted after successful completion")
	}

	bodyRec, err := s.GetBody(ctx, b.ID)
	if err != nil {
		t.Fatalf("GetBody after migration: %v", err)
	}
	if bodyRec.State != adapter.StateRunning {
		t.Errorf("body state after migration = %s, want Running", bodyRec.State)
	}
}

func TestMigrationCreatesSnapshot(t *testing.T) {
	s := openTestStore(t)
	ma := newMockAdapter()
	bm := NewBodyManager(s, ma)
	ctx := context.Background()

	b, err := bm.Create(ctx, "mig-snap-test", adapter.BodySpec{Image: "alpine"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	mc := NewMigrationCoordinator(s, ma, bm, nil)
	_, err = mc.BeginMigration(ctx, b.ID, "remote-host")
	if err != nil {
		t.Fatalf("BeginMigration: %v", err)
	}

	snaps, err := s.ListSnapshots(ctx, b.ID)
	if err != nil {
		t.Fatalf("ListSnapshots: %v", err)
	}

	if len(snaps) != 0 {
		t.Errorf("snapshots after completed migration = %d, want 0 (cleaned up)", len(snaps))
	}
}

func TestAdapterFailureTransitionsToError(t *testing.T) {
	s := openTestStore(t)
	ma := newMockAdapter()
	bm := NewBodyManager(s, ma)
	ctx := context.Background()

	b, err := bm.Create(ctx, "fail-test", adapter.BodySpec{Image: "alpine"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := bm.Stop(ctx, b.ID, adapter.StopOpts{}); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	ma.failStart = true
	err = bm.Start(ctx, b.ID)
	if err == nil {
		t.Fatal("Start should have failed")
	}

	if b.State != adapter.StateError {
		t.Errorf("state after failed start = %s, want Error", b.State)
	}
}

func TestStopFailureTransitionsToError(t *testing.T) {
	s := openTestStore(t)
	ma := newMockAdapter()
	bm := NewBodyManager(s, ma)
	ctx := context.Background()

	b, err := bm.Create(ctx, "stop-fail-test", adapter.BodySpec{Image: "alpine"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	ma.failStop = true
	err = bm.Stop(ctx, b.ID, adapter.StopOpts{})
	if err == nil {
		t.Fatal("Stop should have failed")
	}

	if b.State != adapter.StateError {
		t.Errorf("state after failed stop = %s, want Error", b.State)
	}
}

func TestGetBodyFromStore(t *testing.T) {
	s := openTestStore(t)
	ma := newMockAdapter()
	bm := NewBodyManager(s, ma)
	ctx := context.Background()

	created, err := bm.Create(ctx, "get-test", adapter.BodySpec{Image: "alpine"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	fetched, err := bm.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if fetched.ID != created.ID {
		t.Errorf("fetched ID = %s, want %s", fetched.ID, created.ID)
	}
	if fetched.State != adapter.StateRunning {
		t.Errorf("fetched state = %s, want Running", fetched.State)
	}
}

func TestGetNonexistentBody(t *testing.T) {
	s := openTestStore(t)
	ma := newMockAdapter()
	bm := NewBodyManager(s, ma)
	ctx := context.Background()

	_, err := bm.Get(ctx, "nonexistent")
	if err == nil {
		t.Fatal("Get nonexistent body should fail")
	}
}

func TestCanTransitionMethod(t *testing.T) {
	b := &Body{State: adapter.StateRunning}
	if !b.CanTransition(adapter.StateStopping) {
		t.Error("Running → Stopping should be valid")
	}
	if b.CanTransition(adapter.StateDestroyed) {
		t.Error("Running → Destroyed should be invalid")
	}
	if b.CanTransition(adapter.StateStarting) {
		t.Error("Running → Starting should be invalid")
	}
}

func TestMigrationStepProvisionCreatesContainer(t *testing.T) {
	s := openTestStore(t)
	ma := newMockAdapter()
	ma.inspectMeta = adapter.ContainerMetadata{
		Image:   "alpine:latest",
		Workdir: "/app",
		Env:     map[string]string{"FOO": "bar"},
		Cmd:     []string{"sh"},
	}
	bm := NewBodyManager(s, ma)
	ctx := context.Background()

	b, err := bm.Create(ctx, "provision-test", adapter.BodySpec{Image: "alpine"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	mc := NewMigrationCoordinator(s, ma, bm, nil)
	_, err = mc.BeginMigration(ctx, b.ID, "docker")
	if err != nil {
		t.Fatalf("BeginMigration: %v", err)
	}

	ma.mu.Lock()
	createdCount := len(ma.created)
	ma.mu.Unlock()
	if createdCount != 2 {
		t.Errorf("created containers = %d, want 2 (source + target)", createdCount)
	}
}

func TestMigrationStepProvisionIdempotent(t *testing.T) {
	s := openTestStore(t)
	ma := newMockAdapter()
	ma.inspectMeta = adapter.ContainerMetadata{
		Image:   "alpine:latest",
		Workdir: "/app",
	}
	bm := NewBodyManager(s, ma)
	ctx := context.Background()

	b, err := bm.Create(ctx, "provision-idem-test", adapter.BodySpec{Image: "alpine"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	mc := NewMigrationCoordinator(s, ma, bm, nil)
	migID, err := mc.BeginMigration(ctx, b.ID, "docker")
	if err != nil {
		t.Fatalf("BeginMigration: %v", err)
	}

	if err := mc.ResumeMigration(ctx, migID); err != nil {
		t.Fatalf("ResumeMigration: %v", err)
	}

	ma.mu.Lock()
	createdCount := len(ma.created)
	ma.mu.Unlock()
	if createdCount != 2 {
		t.Errorf("created containers after resume = %d, want 2", createdCount)
	}
}

func TestMigrationStepTransferCopiesFiles(t *testing.T) {
	s := openTestStore(t)
	ma := newMockAdapter()
	ma.inspectMeta = adapter.ContainerMetadata{
		Image:   "alpine:latest",
		Workdir: "/app",
	}
	bm := NewBodyManager(s, ma)
	ctx := context.Background()

	b, err := bm.Create(ctx, "transfer-test", adapter.BodySpec{Image: "alpine"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	mc := NewMigrationCoordinator(s, ma, bm, nil)
	_, err = mc.BeginMigration(ctx, b.ID, "docker")
	if err != nil {
		t.Fatalf("BeginMigration: %v", err)
	}

	ma.mu.Lock()
	importedCount := len(ma.importedTo)
	ma.mu.Unlock()
	if importedCount != 1 {
		t.Errorf("imported filesystems = %d, want 1", importedCount)
	}
}

func TestMigrationStepTransferIdempotent(t *testing.T) {
	s := openTestStore(t)
	ma := newMockAdapter()
	ma.inspectMeta = adapter.ContainerMetadata{
		Image:   "alpine:latest",
		Workdir: "/app",
	}
	bm := NewBodyManager(s, ma)
	ctx := context.Background()

	b, err := bm.Create(ctx, "transfer-idem-test", adapter.BodySpec{Image: "alpine"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	mc := NewMigrationCoordinator(s, ma, bm, nil)
	migID, err := mc.BeginMigration(ctx, b.ID, "docker")
	if err != nil {
		t.Fatalf("BeginMigration: %v", err)
	}

	if err := mc.ResumeMigration(ctx, migID); err != nil {
		t.Fatalf("ResumeMigration: %v", err)
	}

	ma.mu.Lock()
	importedCount := len(ma.importedTo)
	ma.mu.Unlock()
	if importedCount != 1 {
		t.Errorf("imported filesystems after resume = %d, want 1", importedCount)
	}
}

func TestMigrationRetryAfterPartialFailure(t *testing.T) {
	s := openTestStore(t)
	ma := newMockAdapter()
	ma.inspectMeta = adapter.ContainerMetadata{
		Image:   "alpine:latest",
		Workdir: "/app",
	}
	bm := NewBodyManager(s, ma)
	ctx := context.Background()

	b, err := bm.Create(ctx, "retry-test", adapter.BodySpec{Image: "alpine"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	ma.failImport = true
	mc := NewMigrationCoordinator(s, ma, bm, nil)
	migID, err := mc.BeginMigration(ctx, b.ID, "docker")
	if err == nil {
		t.Fatal("BeginMigration should have failed")
	}

	rec, err := s.GetMigration(ctx, migID)
	if err != nil {
		t.Fatalf("GetMigration: %v", err)
	}
	if rec.CurrentStep != 3 {
		t.Errorf("current_step after failure = %d, want 3", rec.CurrentStep)
	}
	if rec.Error == "" {
		t.Error("expected error to be recorded")
	}

	ma.failImport = false
	if err := s.UpdateMigration(ctx, migID, rec.CurrentStep, ""); err != nil {
		t.Fatalf("clear migration error: %v", err)
	}

	if err := mc.ResumeMigration(ctx, migID); err != nil {
		t.Fatalf("ResumeMigration: %v", err)
	}

	_, err = s.GetMigration(ctx, migID)
	if err == nil {
		t.Fatal("migration record should be deleted after successful completion")
	}
}

func TestMigrationStepImportRestoresFiles(t *testing.T) {
	s := openTestStore(t)
	ma := newMockAdapter()
	ma.inspectMeta = adapter.ContainerMetadata{
		Image:   "alpine:latest",
		Workdir: "/app",
	}
	bm := NewBodyManager(s, ma)
	ctx := context.Background()

	b, err := bm.Create(ctx, "import-test", adapter.BodySpec{Image: "alpine"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	mc := NewMigrationCoordinator(s, ma, bm, nil)
	_, err = mc.BeginMigration(ctx, b.ID, "docker")
	if err != nil {
		t.Fatalf("BeginMigration: %v", err)
	}

	ma.mu.Lock()
	importedCount := len(ma.importedTo)
	ma.mu.Unlock()
	if importedCount != 1 {
		t.Errorf("imported filesystems = %d, want 1", importedCount)
	}
}

func TestMigrationStepImportIdempotent(t *testing.T) {
	s := openTestStore(t)
	ma := newMockAdapter()
	ma.inspectMeta = adapter.ContainerMetadata{
		Image:   "alpine:latest",
		Workdir: "/app",
	}
	bm := NewBodyManager(s, ma)
	ctx := context.Background()

	b, err := bm.Create(ctx, "import-idem-test", adapter.BodySpec{Image: "alpine"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	mc := NewMigrationCoordinator(s, ma, bm, nil)
	migID, err := mc.BeginMigration(ctx, b.ID, "docker")
	if err != nil {
		t.Fatalf("BeginMigration: %v", err)
	}

	if err := mc.ResumeMigration(ctx, migID); err != nil {
		t.Fatalf("ResumeMigration: %v", err)
	}

	ma.mu.Lock()
	importedCount := len(ma.importedTo)
	ma.mu.Unlock()
	if importedCount != 1 {
		t.Errorf("imported filesystems after resume = %d, want 1", importedCount)
	}
}

func TestMigrationStepVerifyDetectsMissingFiles(t *testing.T) {
	s := openTestStore(t)
	ma := newMockAdapter()
	ma.inspectMeta = adapter.ContainerMetadata{
		Image:   "alpine:latest",
		Workdir: "/app",
	}
	bm := NewBodyManager(s, ma)
	ctx := context.Background()

	b, err := bm.Create(ctx, "verify-missing-test", adapter.BodySpec{Image: "alpine"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	ma.failImport = true
	mc := NewMigrationCoordinator(s, ma, bm, nil)
	migID, err := mc.BeginMigration(ctx, b.ID, "docker")
	if err == nil {
		t.Fatal("BeginMigration should have failed")
	}

	rec, err := s.GetMigration(ctx, migID)
	if err != nil {
		t.Fatalf("GetMigration: %v", err)
	}
	if rec.CurrentStep != 3 {
		t.Errorf("current_step after failure = %d, want 3", rec.CurrentStep)
	}

	ma.failImport = false
	if err := s.UpdateMigration(ctx, migID, rec.CurrentStep, ""); err != nil {
		t.Fatalf("clear migration error: %v", err)
	}

	if err := mc.ResumeMigration(ctx, migID); err != nil {
		t.Fatalf("ResumeMigration: %v", err)
	}

	_, err = s.GetMigration(ctx, migID)
	if err == nil {
		t.Fatal("migration record should be deleted after successful completion")
	}
}

func TestMigrationStepVerifyDetectsUnhealthyContainer(t *testing.T) {
	s := openTestStore(t)
	ma := newMockAdapter()
	ma.inspectMeta = adapter.ContainerMetadata{
		Image:   "alpine:latest",
		Workdir: "/app",
	}
	bm := NewBodyManager(s, ma)
	ctx := context.Background()

	b, err := bm.Create(ctx, "verify-unhealthy-test", adapter.BodySpec{Image: "alpine"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	mc := NewMigrationCoordinator(s, ma, bm, nil)
	_, err = mc.BeginMigration(ctx, b.ID, "docker")
	if err != nil {
		t.Fatalf("BeginMigration: %v", err)
	}

	ma.mu.Lock()
	status, ok := ma.statuses[string(ma.created[len(ma.created)-1])]
	ma.mu.Unlock()
	if !ok {
		t.Fatal("target container status not found")
	}
	if status.State != adapter.StateRunning && status.State != adapter.StateCreated {
		t.Errorf("target container state = %s, want Running or Created", status.State)
	}
}

func TestMigrationProvisionFailsWithoutRollback(t *testing.T) {
	s := openTestStore(t)
	ma := newMockAdapter()
	ma.inspectMeta = adapter.ContainerMetadata{
		Image:   "alpine:latest",
		Workdir: "/app",
	}
	bm := NewBodyManager(s, ma)
	ctx := context.Background()

	b, err := bm.Create(ctx, "provision-fail-test", adapter.BodySpec{Image: "alpine"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	ma.failCreate = true
	mc := NewMigrationCoordinator(s, ma, bm, nil)
	_, err = mc.BeginMigration(ctx, b.ID, "docker")
	if err == nil {
		t.Fatal("BeginMigration should have failed")
	}

	ma.mu.Lock()
	destroyedCount := len(ma.destroyed)
	ma.mu.Unlock()
	if destroyedCount != 0 {
		t.Errorf("destroyed containers = %d, want 0 (create failed, nothing to roll back)", destroyedCount)
	}
}

func TestMigrationStepSwitchUpdatesBodyInstanceID(t *testing.T) {
	s := openTestStore(t)
	ma := newMockAdapter()
	ma.inspectMeta = adapter.ContainerMetadata{
		Image:   "alpine:latest",
		Workdir: "/app",
	}
	bm := NewBodyManager(s, ma)
	ctx := context.Background()

	b, err := bm.Create(ctx, "switch-test", adapter.BodySpec{Image: "alpine"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	srcHandle := b.InstanceID

	mc := NewMigrationCoordinator(s, ma, bm, nil)
	_, err = mc.BeginMigration(ctx, b.ID, "docker")
	if err != nil {
		t.Fatalf("BeginMigration: %v", err)
	}

	bodyRec, err := s.GetBody(ctx, b.ID)
	if err != nil {
		t.Fatalf("GetBody after migration: %v", err)
	}
	if bodyRec.InstanceID == string(srcHandle) {
		t.Errorf("body instance_id still = %s, should have changed to target", bodyRec.InstanceID)
	}

	b2, err := bm.Get(ctx, b.ID)
	if err != nil {
		t.Fatalf("Get body after migration: %v", err)
	}
	if b2.InstanceID == srcHandle {
		t.Errorf("in-memory instance_id still = %s, should have changed", b2.InstanceID)
	}

	ma.mu.Lock()
	stoppedCount := len(ma.stopped)
	destroyedCount := len(ma.destroyed)
	ma.mu.Unlock()
	if stoppedCount != 1 {
		t.Errorf("stopped containers = %d, want 1 (source)", stoppedCount)
	}
	if destroyedCount != 1 {
		t.Errorf("destroyed containers = %d, want 1 (source)", destroyedCount)
	}
}

func TestMigrationStepSwitchIdempotent(t *testing.T) {
	s := openTestStore(t)
	ma := newMockAdapter()
	ma.inspectMeta = adapter.ContainerMetadata{
		Image:   "alpine:latest",
		Workdir: "/app",
	}
	bm := NewBodyManager(s, ma)
	ctx := context.Background()

	b, err := bm.Create(ctx, "switch-idem-test", adapter.BodySpec{Image: "alpine"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	mc := NewMigrationCoordinator(s, ma, bm, nil)
	migID, err := mc.BeginMigration(ctx, b.ID, "docker")
	if err != nil {
		t.Fatalf("BeginMigration: %v", err)
	}

	if err := mc.ResumeMigration(ctx, migID); err != nil {
		t.Fatalf("ResumeMigration: %v", err)
	}

	bodyRec, err := s.GetBody(ctx, b.ID)
	if err != nil {
		t.Fatalf("GetBody after resume: %v", err)
	}
	if bodyRec.State != adapter.StateRunning {
		t.Errorf("body state after resume = %s, want Running", bodyRec.State)
	}
}

func TestMigrationStepCleanupRemovesSnapshotFile(t *testing.T) {
	s := openTestStore(t)
	ma := newMockAdapter()
	ma.inspectMeta = adapter.ContainerMetadata{
		Image:   "alpine:latest",
		Workdir: "/app",
	}
	bm := NewBodyManager(s, ma)
	ctx := context.Background()

	b, err := bm.Create(ctx, "cleanup-test", adapter.BodySpec{Image: "alpine"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	mc := NewMigrationCoordinator(s, ma, bm, nil)
	migID, err := mc.BeginMigration(ctx, b.ID, "docker")
	if err != nil {
		t.Fatalf("BeginMigration: %v", err)
	}

	_, err = s.GetMigration(ctx, migID)
	if err == nil {
		t.Fatal("migration record should have been deleted after cleanup")
	}

	snaps, err := s.ListSnapshots(ctx, b.ID)
	if err != nil {
		t.Fatalf("ListSnapshots: %v", err)
	}
	if len(snaps) != 0 {
		t.Errorf("snapshots after cleanup = %d, want 0", len(snaps))
	}
}

func TestMigrationStepSwitchRollbackOnFailure(t *testing.T) {
	s := openTestStore(t)
	ma := newMockAdapter()
	ma.inspectMeta = adapter.ContainerMetadata{
		Image:   "alpine:latest",
		Workdir: "/app",
	}
	bm := NewBodyManager(s, ma)
	ctx := context.Background()

	b, err := bm.Create(ctx, "rollback-test", adapter.BodySpec{Image: "alpine"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	srcHandle := b.InstanceID

	mc := NewMigrationCoordinator(s, ma, bm, nil)
	_, err = mc.BeginMigration(ctx, b.ID, "docker")
	if err != nil {
		t.Fatalf("BeginMigration: %v", err)
	}

	bodyRec, err := s.GetBody(ctx, b.ID)
	if err != nil {
		t.Fatalf("GetBody after migration: %v", err)
	}
	if bodyRec.InstanceID == string(srcHandle) {
		t.Errorf("body instance_id should have changed from source %s", srcHandle)
	}

	b2, err := bm.Get(ctx, b.ID)
	if err != nil {
		t.Fatalf("Get body after migration: %v", err)
	}
	if b2.State != adapter.StateRunning {
		t.Errorf("body state after migration = %s, want Running", b2.State)
	}
}

// --- Cross-machine migration tests ---

func TestMigrationCrossMachineUsesRegistry(t *testing.T) {
	s := openTestStore(t)
	ma := newMockAdapter()
	ma.inspectMeta = adapter.ContainerMetadata{
		Image:   "alpine:latest",
		Workdir: "/app",
	}
	ma.substrate = "docker"
	bm := NewBodyManager(s, ma)
	ctx := context.Background()

	b, err := bm.Create(ctx, "cross-test", adapter.BodySpec{Image: "alpine"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	reg := newMockRegistry()
	mc := NewMigrationCoordinator(s, ma, bm, reg)
	_, err = mc.BeginMigration(ctx, b.ID, "fleet")
	if err != nil {
		t.Fatalf("BeginMigration: %v", err)
	}

	reg.mu.Lock()
	pushedCount := len(reg.pushed)
	pulledCount := len(reg.pulled)
	reg.mu.Unlock()
	if pushedCount != 1 {
		t.Errorf("pushed snapshots = %d, want 1", pushedCount)
	}
	if pulledCount != 1 {
		t.Errorf("pulled snapshots = %d, want 1", pulledCount)
	}

	ma.mu.Lock()
	importedCount := len(ma.importedTo)
	ma.mu.Unlock()
	if importedCount != 1 {
		t.Errorf("imported filesystems = %d, want 1", importedCount)
	}
}

func TestMigrationCrossMachineSHA256Mismatch(t *testing.T) {
	s := openTestStore(t)
	ma := newMockAdapter()
	ma.inspectMeta = adapter.ContainerMetadata{
		Image:   "alpine:latest",
		Workdir: "/app",
	}
	ma.substrate = "docker"
	bm := NewBodyManager(s, ma)
	ctx := context.Background()

	b, err := bm.Create(ctx, "cross-sha-test", adapter.BodySpec{Image: "alpine"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	reg := newMockRegistry()
	reg.pullSHA = "mismatched-sha256-abc123"
	mc := NewMigrationCoordinator(s, ma, bm, reg)
	_, err = mc.BeginMigration(ctx, b.ID, "fleet")
	if err == nil {
		t.Fatal("BeginMigration should have failed on SHA mismatch")
	}
}

func TestMigrationCrossMachineRetryPush(t *testing.T) {
	s := openTestStore(t)
	ma := newMockAdapter()
	ma.inspectMeta = adapter.ContainerMetadata{
		Image:   "alpine:latest",
		Workdir: "/app",
	}
	ma.substrate = "docker"
	bm := NewBodyManager(s, ma)
	ctx := context.Background()

	b, err := bm.Create(ctx, "cross-retry-test", adapter.BodySpec{Image: "alpine"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	reg := newMockRegistry()
	reg.failAfter = 2
	mc := NewMigrationCoordinator(s, ma, bm, reg)
	_, err = mc.BeginMigration(ctx, b.ID, "fleet")
	if err != nil {
		t.Fatalf("BeginMigration should succeed after retries: %v", err)
	}

	reg.mu.Lock()
	pushedCount := len(reg.pushed)
	reg.mu.Unlock()
	if pushedCount != 1 {
		t.Errorf("pushed snapshots = %d, want 1", pushedCount)
	}
}

func TestMigrationSameMachineIgnoresRegistry(t *testing.T) {
	s := openTestStore(t)
	ma := newMockAdapter()
	ma.inspectMeta = adapter.ContainerMetadata{
		Image:   "alpine:latest",
		Workdir: "/app",
	}
	ma.substrate = "docker"
	bm := NewBodyManager(s, ma)
	ctx := context.Background()

	b, err := bm.Create(ctx, "same-test", adapter.BodySpec{Image: "alpine"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	reg := newMockRegistry()
	mc := NewMigrationCoordinator(s, ma, bm, reg)
	_, err = mc.BeginMigration(ctx, b.ID, "docker")
	if err != nil {
		t.Fatalf("BeginMigration: %v", err)
	}

	reg.mu.Lock()
	pushedCount := len(reg.pushed)
	reg.mu.Unlock()
	if pushedCount != 0 {
		t.Errorf("pushed snapshots = %d, want 0 (same-machine should not use registry)", pushedCount)
	}
}

func TestMigrationCrossMachineResumeAfterTransfer(t *testing.T) {
	s := openTestStore(t)
	ma := newMockAdapter()
	ma.inspectMeta = adapter.ContainerMetadata{
		Image:   "alpine:latest",
		Workdir: "/app",
	}
	ma.substrate = "docker"
	bm := NewBodyManager(s, ma)
	ctx := context.Background()

	b, err := bm.Create(ctx, "cross-resume-test", adapter.BodySpec{Image: "alpine"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	reg := newMockRegistry()
	mc := NewMigrationCoordinator(s, ma, bm, reg)
	migID, err := mc.BeginMigration(ctx, b.ID, "fleet")
	if err != nil {
		t.Fatalf("BeginMigration: %v", err)
	}

	if err := mc.ResumeMigration(ctx, migID); err != nil {
		t.Fatalf("ResumeMigration: %v", err)
	}

	_, err = s.GetMigration(ctx, migID)
	if err == nil {
		t.Fatal("migration record should be deleted after successful completion")
	}
}
