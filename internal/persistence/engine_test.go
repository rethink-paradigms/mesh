package persistence

import (
	"bytes"
	"context"
	"io"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rethink-paradigms/mesh/internal/adapter"
)

type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func newFakeClock() *fakeClock {
	return &fakeClock{now: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
}

func (c *fakeClock) nowFunc() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

type mockAdapter struct {
	mu        sync.Mutex
	importBuf bytes.Buffer
	exportErr error
	importErr error
}

type nopCloser struct{ io.Reader }

func (nopCloser) Close() error { return nil }

func (m *mockAdapter) Create(_ context.Context, _ adapter.BodySpec) (adapter.Handle, error) {
	return "mock-handle", nil
}
func (m *mockAdapter) Start(_ context.Context, _ adapter.Handle) error        { return nil }
func (m *mockAdapter) Stop(_ context.Context, _ adapter.Handle, _ adapter.StopOpts) error { return nil }
func (m *mockAdapter) Destroy(_ context.Context, _ adapter.Handle) error      { return nil }
func (m *mockAdapter) GetStatus(_ context.Context, _ adapter.Handle) (adapter.BodyStatus, error) {
	return adapter.BodyStatus{State: adapter.StateRunning}, nil
}
func (m *mockAdapter) Exec(_ context.Context, _ adapter.Handle, _ []string) (adapter.ExecResult, error) {
	return adapter.ExecResult{ExitCode: 0}, nil
}
func (m *mockAdapter) Inspect(_ context.Context, _ adapter.Handle) (adapter.ContainerMetadata, error) {
	return adapter.ContainerMetadata{}, nil
}
func (m *mockAdapter) Capabilities() adapter.AdapterCapabilities {
	return adapter.AdapterCapabilities{
		ExportFilesystem: true,
		ImportFilesystem: true,
		Inspect:          true,
	}
}

func (m *mockAdapter) ExportFilesystem(_ context.Context, _ adapter.Handle) (io.ReadCloser, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.exportErr != nil {
		return nil, m.exportErr
	}
	return nopCloser{bytes.NewReader([]byte("fake-tar-content-for-testing"))}, nil
}

func (m *mockAdapter) IsHealthy(_ context.Context) bool { return true }
func (m *mockAdapter) SubstrateName() string { return "mock" }

func (m *mockAdapter) ImportFilesystem(_ context.Context, _ adapter.Handle, tarball io.Reader, _ adapter.ImportOpts) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.importErr != nil {
		return m.importErr
	}
	_, err := io.Copy(&m.importBuf, tarball)
	return err
}

func TestCaptureRestore(t *testing.T) {
	tmpDir := t.TempDir()
	mock := &mockAdapter{}
	clock := newFakeClock()
	engine := NewSnapshotEngine(mock, tmpDir)
	engine.nowFunc = clock.nowFunc
	ctx := context.Background()

	m, err := engine.Capture(ctx, "test-body-123", "test-body")
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}

	if m.BodyID != "test-body-123" {
		t.Errorf("manifest BodyID = %q, want %q", m.BodyID, "test-body-123")
	}
	if m.Checksum == "" {
		t.Error("manifest Checksum is empty")
	}
	if m.Size == 0 {
		t.Error("manifest Size is 0")
	}

	snapDir := tmpDir + "/test-body"
	entries, err := entriesWithSuffix(snapDir, ".tar.zst")
	if err != nil {
		t.Fatalf("reading snapshot dir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 .tar.zst file, got %d", len(entries))
	}

	snapshotPath := snapDir + "/" + entries[0]

	mock.importBuf.Reset()
	err = engine.Restore(ctx, "test-body-123", snapshotPath)
	if err != nil {
		t.Fatalf("Restore: %v", err)
	}

	received := mock.importBuf.String()
	if received == "" {
		t.Error("import received empty data")
	}
	if received != "fake-tar-content-for-testing" {
		t.Errorf("import data mismatch: got %q, want %q", received, "fake-tar-content-for-testing")
	}
}

func TestList(t *testing.T) {
	tmpDir := t.TempDir()
	mock := &mockAdapter{}
	clock := newFakeClock()
	engine := NewSnapshotEngine(mock, tmpDir)
	engine.nowFunc = clock.nowFunc
	ctx := context.Background()

	_, err := engine.Capture(ctx, "body-1", "my-agent")
	if err != nil {
		t.Fatalf("first Capture: %v", err)
	}
	clock.advance(time.Second)

	_, err = engine.Capture(ctx, "body-1", "my-agent")
	if err != nil {
		t.Fatalf("second Capture: %v", err)
	}

	manifests, err := engine.List(ctx, "my-agent")
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	if len(manifests) != 2 {
		t.Fatalf("expected 2 manifests, got %d", len(manifests))
	}

	if !manifests[0].Timestamp.After(manifests[1].Timestamp) {
		t.Error("manifests not sorted newest-first")
	}
}

func TestPrune(t *testing.T) {
	tmpDir := t.TempDir()
	mock := &mockAdapter{}
	clock := newFakeClock()
	engine := NewSnapshotEngine(mock, tmpDir)
	engine.nowFunc = clock.nowFunc
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		_, err := engine.Capture(ctx, "body-1", "prune-agent")
		if err != nil {
			t.Fatalf("Capture %d: %v", i, err)
		}
		clock.advance(time.Second)
	}

	err := engine.Prune(ctx, "prune-agent", 2)
	if err != nil {
		t.Fatalf("Prune: %v", err)
	}

	manifests, err := engine.List(ctx, "prune-agent")
	if err != nil {
		t.Fatalf("List after prune: %v", err)
	}

	if len(manifests) != 2 {
		t.Errorf("expected 2 manifests after prune, got %d", len(manifests))
	}

	snapDir := tmpDir + "/prune-agent"
	tarEntries, _ := entriesWithSuffix(snapDir, ".tar.zst")
	shaEntries, _ := entriesWithSuffix(snapDir, ".sha256")
	jsonEntries, _ := entriesWithSuffix(snapDir, ".json")

	if len(tarEntries) != 2 {
		t.Errorf("expected 2 .tar.zst files, got %d", len(tarEntries))
	}
	if len(shaEntries) != 2 {
		t.Errorf("expected 2 .sha256 files, got %d", len(shaEntries))
	}
	if len(jsonEntries) != 2 {
		t.Errorf("expected 2 .json files, got %d", len(jsonEntries))
	}
}

func TestListEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	mock := &mockAdapter{}
	engine := NewSnapshotEngine(mock, tmpDir)

	manifests, err := engine.List(context.Background(), "nonexistent")
	if err != nil {
		t.Fatalf("List nonexistent: %v", err)
	}
	if len(manifests) != 0 {
		t.Errorf("expected 0 manifests, got %d", len(manifests))
	}
}

func entriesWithSuffix(dir, suffix string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var matched []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), suffix) {
			matched = append(matched, e.Name())
		}
	}
	return matched, nil
}
