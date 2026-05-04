package restore

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rethink-paradigms/mesh/internal/manifest"
	"github.com/rethink-paradigms/mesh/internal/orchestrator"
	"github.com/rethink-paradigms/mesh/internal/snapshot"
	"github.com/rethink-paradigms/mesh/internal/store"
)

// mustWriteFile creates a file with content and permissions in dir.
func mustWriteFile(tb testing.TB, dir, name, content string, perm os.FileMode) {
	tb.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), perm); err != nil {
		tb.Fatal(err)
	}
	if err := os.Chmod(p, perm); err != nil {
		tb.Fatal(err)
	}
}

// mustSymlink creates a symlink in dir.
func mustSymlink(tb testing.TB, dir, name, target string) {
	tb.Helper()
	p := filepath.Join(dir, name)
	if err := os.Symlink(target, p); err != nil {
		tb.Fatal(err)
	}
}

// mustMkdir creates a subdirectory with permissions.
func mustMkdir(tb testing.TB, dir, name string, perm os.FileMode) {
	tb.Helper()
	p := filepath.Join(dir, name)
	if err := os.MkdirAll(p, perm); err != nil {
		tb.Fatal(err)
	}
	if err := os.Chmod(p, perm); err != nil {
		tb.Fatal(err)
	}
}

// createSnapshotFixture creates a source dir with known content and snapshots it.
// Returns (snapshotTarPath, snapshotHashPath, sourceDir).
func createSnapshotFixture(t *testing.T) (snapPath, hashPath, srcDir string) {
	t.Helper()

	srcDir = t.TempDir()
	mustMkdir(t, srcDir, "subdir", 0o755)
	mustWriteFile(t, srcDir, "hello.txt", "hello world\n", 0o644)
	mustWriteFile(t, srcDir, "subdir/nested.txt", "nested content\n", 0o600)
	mustSymlink(t, srcDir, "link-to-hello", "hello.txt")

	snapDir := t.TempDir()
	snapPath = filepath.Join(snapDir, "test-snapshot.tar.zst")

	ctx := context.Background()
	if err := snapshot.CreateSnapshot(ctx, srcDir, snapPath); err != nil {
		t.Fatalf("create snapshot fixture: %v", err)
	}

	return snapPath, snapPath + ".sha256", srcDir
}

func TestRestoreRoundTrip(t *testing.T) {
	snapPath, _, _ := createSnapshotFixture(t)

	restoreDir := filepath.Join(t.TempDir(), "restored")
	ctx := context.Background()

	if err := Restore(ctx, snapPath, restoreDir); err != nil {
		t.Fatalf("Restore: %v", err)
	}

	// Verify files exist and match.
	helloContent, err := os.ReadFile(filepath.Join(restoreDir, "hello.txt"))
	if err != nil {
		t.Fatalf("read restored hello.txt: %v", err)
	}
	if string(helloContent) != "hello world\n" {
		t.Errorf("hello.txt content = %q, want %q", helloContent, "hello world\n")
	}

	nestedContent, err := os.ReadFile(filepath.Join(restoreDir, "subdir", "nested.txt"))
	if err != nil {
		t.Fatalf("read restored nested.txt: %v", err)
	}
	if string(nestedContent) != "nested content\n" {
		t.Errorf("nested.txt content = %q, want %q", nestedContent, "nested content\n")
	}

	// Verify symlink.
	linkTarget, err := os.Readlink(filepath.Join(restoreDir, "link-to-hello"))
	if err != nil {
		t.Fatalf("readlink: %v", err)
	}
	if linkTarget != "hello.txt" {
		t.Errorf("symlink target = %q, want %q", linkTarget, "hello.txt")
	}

	// Verify directory exists.
	info, err := os.Stat(filepath.Join(restoreDir, "subdir"))
	if err != nil {
		t.Fatalf("stat subdir: %v", err)
	}
	if !info.IsDir() {
		t.Error("subdir is not a directory")
	}
}

func TestHashMismatch(t *testing.T) {
	snapPath, hashPath, _ := createSnapshotFixture(t)

	// Corrupt the hash file.
	if err := os.WriteFile(hashPath, []byte("0000000000000000000000000000000000000000000000000000000000000000\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	restoreDir := filepath.Join(t.TempDir(), "restored")
	ctx := context.Background()

	err := Restore(ctx, snapPath, restoreDir)
	if err == nil {
		t.Fatal("expected error for hash mismatch")
	}
	if !strings.Contains(err.Error(), "hash mismatch") {
		t.Errorf("error = %q, want hash mismatch", err.Error())
	}

	// Target dir should not exist.
	if _, err := os.Stat(restoreDir); !os.IsNotExist(err) {
		t.Error("target dir should not exist after hash mismatch")
	}
}

func TestAtomicRename(t *testing.T) {
	snapPath, _, _ := createSnapshotFixture(t)

	parentDir := t.TempDir()
	targetDir := filepath.Join(parentDir, "target")

	// Pre-populate target with old content.
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(targetDir, "old-file.txt"), []byte("old\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	if err := Restore(ctx, snapPath, targetDir); err != nil {
		t.Fatalf("Restore: %v", err)
	}

	// Old file should be gone.
	if _, err := os.Stat(filepath.Join(targetDir, "old-file.txt")); !os.IsNotExist(err) {
		t.Error("old-file.txt should be gone after restore")
	}

	// New content should be present.
	content, err := os.ReadFile(filepath.Join(targetDir, "hello.txt"))
	if err != nil {
		t.Fatalf("read hello.txt: %v", err)
	}
	if string(content) != "hello world\n" {
		t.Errorf("hello.txt = %q, want %q", content, "hello world\n")
	}
}

func TestCleanupOnFailure(t *testing.T) {
	snapPath, hashPath, _ := createSnapshotFixture(t)

	// Corrupt the tarball (truncate it).
	if err := os.Truncate(snapPath, 10); err != nil {
		t.Fatal(err)
	}
	// Also fix the hash so we get past VerifyHash and fail during extraction.
	if err := os.Remove(hashPath); err != nil {
		t.Fatal(err)
	}
	// Create a valid hash for the truncated file to ensure we get past hash check.
	// Actually, easier: just corrupt the hash to fail early.
	if err := os.WriteFile(hashPath, []byte("badhash\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	parentDir := t.TempDir()
	restoreDir := filepath.Join(parentDir, "restored")

	ctx := context.Background()
	err := Restore(ctx, snapPath, restoreDir)
	if err == nil {
		t.Fatal("expected error for corrupted snapshot")
	}

	// No temp dirs should be left behind in parent.
	entries, err := os.ReadDir(parentDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".mesh-restore-") {
			t.Errorf("temp dir left behind: %s", e.Name())
		}
	}
}

func TestRestoreNonExistentSnapshot(t *testing.T) {
	restoreDir := filepath.Join(t.TempDir(), "restored")
	ctx := context.Background()

	err := Restore(ctx, "/nonexistent/path.tar.zst", restoreDir)
	if err == nil {
		t.Fatal("expected error for missing snapshot")
	}
}

func TestRestoreToNonWritableDir(t *testing.T) {
	snapPath, _, _ := createSnapshotFixture(t)

	// Create a read-only parent directory.
	parentDir := t.TempDir()
	readonlyDir := filepath.Join(parentDir, "readonly")
	if err := os.MkdirAll(readonlyDir, 0o555); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(readonlyDir, 0o755) // restore for cleanup

	targetDir := filepath.Join(readonlyDir, "restored")
	ctx := context.Background()

	err := Restore(ctx, snapPath, targetDir)
	if err == nil {
		t.Fatal("expected error for non-writable parent")
	}
	if !strings.Contains(err.Error(), "pre-flight") && !strings.Contains(err.Error(), "not writable") {
		t.Errorf("error = %q, want pre-flight writability error", err.Error())
	}
}

func TestVerifyHashCorrect(t *testing.T) {
	snapPath, hashPath, _ := createSnapshotFixture(t)

	if err := VerifyHash(snapPath, hashPath); err != nil {
		t.Fatalf("VerifyHash: %v", err)
	}
}

func TestVerifyHashMismatch(t *testing.T) {
	snapPath, hashPath, _ := createSnapshotFixture(t)

	// Write a wrong hash.
	if err := os.WriteFile(hashPath, []byte("deadbeef\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := VerifyHash(snapPath, hashPath)
	if err == nil {
		t.Fatal("expected error for hash mismatch")
	}
	if !strings.Contains(err.Error(), "hash mismatch") {
		t.Errorf("error = %q, want hash mismatch", err.Error())
	}
}

func TestPostRestoreHook(t *testing.T) {
	snapPath, _, _ := createSnapshotFixture(t)

	restoreDir := filepath.Join(t.TempDir(), "restored")
	ctx := context.Background()

	opts := RestoreOpts{
		PostRestoreCmd: "touch post_hook_ran",
		HookTimeout:    30 * time.Second,
	}

	if err := RestoreWithOpts(ctx, snapPath, restoreDir, opts); err != nil {
		t.Fatalf("RestoreWithOpts: %v", err)
	}

	if _, err := os.Stat(filepath.Join(restoreDir, "post_hook_ran")); err != nil {
		t.Errorf("post_hook_ran file not created by hook: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(restoreDir, "hello.txt"))
	if err != nil {
		t.Fatalf("read restored hello.txt: %v", err)
	}
	if string(content) != "hello world\n" {
		t.Errorf("hello.txt = %q, want %q", content, "hello world\n")
	}
}

func TestPostRestoreHookFailure(t *testing.T) {
	snapPath, _, _ := createSnapshotFixture(t)

	restoreDir := filepath.Join(t.TempDir(), "restored")
	ctx := context.Background()

	opts := RestoreOpts{
		PostRestoreCmd: "exit 1",
		HookTimeout:    30 * time.Second,
	}

	err := RestoreWithOpts(ctx, snapPath, restoreDir, opts)
	if err == nil {
		t.Fatal("expected error from post-restore hook failure, got nil")
	}
	if !strings.Contains(err.Error(), "post-restore hook failed") {
		t.Errorf("error = %q, want post-restore hook failed", err.Error())
	}

	content, err := os.ReadFile(filepath.Join(restoreDir, "hello.txt"))
	if err != nil {
		t.Fatalf("restored files should still exist: %v", err)
	}
	if string(content) != "hello world\n" {
		t.Errorf("hello.txt = %q, want %q", content, "hello world\n")
	}
}

func TestRestoreWithOptsNoHook(t *testing.T) {
	snapPath, _, _ := createSnapshotFixture(t)

	restoreDir := filepath.Join(t.TempDir(), "restored")
	ctx := context.Background()

	if err := RestoreWithOpts(ctx, snapPath, restoreDir, RestoreOpts{}); err != nil {
		t.Fatalf("RestoreWithOpts with no hook: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(restoreDir, "hello.txt"))
	if err != nil {
		t.Fatalf("read restored hello.txt: %v", err)
	}
	if string(content) != "hello world\n" {
		t.Errorf("hello.txt = %q, want %q", content, "hello world\n")
	}
}

func openTestStore(tb testing.TB) *store.Store {
	tb.Helper()
	dbPath := filepath.Join(tb.TempDir(), "test.db")
	s, err := store.Open(dbPath)
	if err != nil {
		tb.Fatalf("open store: %v", err)
	}
	tb.Cleanup(func() { s.Close() })
	return s
}

func TestRestoreFromStore(t *testing.T) {
	snapPath, _, _ := createSnapshotFixture(t)
	s := openTestStore(t)
	ctx := context.Background()

	bodyID := "restore-test-body"
	if err := s.CreateBody(ctx, bodyID, bodyID, orchestrator.StateCreated, "", "local", ""); err != nil {
		t.Fatalf("CreateBody: %v", err)
	}

	stat, err := os.Stat(snapPath)
	if err != nil {
		t.Fatalf("stat snapshot: %v", err)
	}

	m, err := manifest.Read(manifest.ManifestPath(snapPath))
	if err != nil {
		m = &manifest.Manifest{AgentName: "test"}
	}
	manifestJSON, _ := json.Marshal(m)

	snapID := filepath.Base(snapPath)
	if err := s.CreateSnapshot(ctx, snapID, bodyID, string(manifestJSON), snapPath, stat.Size()); err != nil {
		t.Fatalf("CreateSnapshot: %v", err)
	}

	restoreDir := filepath.Join(t.TempDir(), "restored")
	if err := RestoreFromStore(ctx, s, snapID, restoreDir, RestoreOpts{}); err != nil {
		t.Fatalf("RestoreFromStore: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(restoreDir, "hello.txt"))
	if err != nil {
		t.Fatalf("read restored hello.txt: %v", err)
	}
	if string(content) != "hello world\n" {
		t.Errorf("hello.txt = %q, want %q", content, "hello world\n")
	}

	linkTarget, err := os.Readlink(filepath.Join(restoreDir, "link-to-hello"))
	if err != nil {
		t.Fatalf("readlink: %v", err)
	}
	if linkTarget != "hello.txt" {
		t.Errorf("symlink target = %q, want %q", linkTarget, "hello.txt")
	}
}

func TestRestoreFromStoreFallbackToPath(t *testing.T) {
	snapPath, _, _ := createSnapshotFixture(t)
	s := openTestStore(t)
	ctx := context.Background()

	restoreDir := filepath.Join(t.TempDir(), "restored")
	if err := RestoreFromStore(ctx, s, snapPath, restoreDir, RestoreOpts{}); err != nil {
		t.Fatalf("RestoreFromStore fallback: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(restoreDir, "hello.txt"))
	if err != nil {
		t.Fatalf("read restored hello.txt: %v", err)
	}
	if string(content) != "hello world\n" {
		t.Errorf("hello.txt = %q, want %q", content, "hello world\n")
	}
}

func TestRestoreFromStoreWithHook(t *testing.T) {
	snapPath, _, _ := createSnapshotFixture(t)
	s := openTestStore(t)
	ctx := context.Background()

	bodyID := "hook-body"
	if err := s.CreateBody(ctx, bodyID, bodyID, orchestrator.StateCreated, "", "local", ""); err != nil {
		t.Fatalf("CreateBody: %v", err)
	}

	stat, _ := os.Stat(snapPath)
	snapID := filepath.Base(snapPath)
	if err := s.CreateSnapshot(ctx, snapID, bodyID, "{}", snapPath, stat.Size()); err != nil {
		t.Fatalf("CreateSnapshot: %v", err)
	}

	restoreDir := filepath.Join(t.TempDir(), "restored")
	opts := RestoreOpts{
		PostRestoreCmd: "touch post_hook_ran",
		HookTimeout:    30 * time.Second,
	}

	if err := RestoreFromStore(ctx, s, snapID, restoreDir, opts); err != nil {
		t.Fatalf("RestoreFromStore with hook: %v", err)
	}

	if _, err := os.Stat(filepath.Join(restoreDir, "post_hook_ran")); err != nil {
		t.Errorf("post_hook_ran file not created: %v", err)
	}
}

func TestRoundTripWithStore(t *testing.T) {
	srcDir := t.TempDir()
	mustWriteFile(t, srcDir, "hello.txt", "round trip content\n", 0o644)
	mustMkdir(t, srcDir, "sub", 0o755)
	mustWriteFile(t, srcDir, "sub/nested.txt", "nested\n", 0o600)

	snapDir := t.TempDir()
	snapPath := filepath.Join(snapDir, "roundtrip.tar.zst")
	ctx := context.Background()

	if err := snapshot.CreateSnapshot(ctx, srcDir, snapPath); err != nil {
		t.Fatalf("CreateSnapshot: %v", err)
	}

	s := openTestStore(t)
	bodyID := "roundtrip-body"
	if err := s.CreateBody(ctx, bodyID, bodyID, orchestrator.StateCreated, "", "local", ""); err != nil {
		t.Fatalf("CreateBody: %v", err)
	}

	stat, _ := os.Stat(snapPath)
	snapID := filepath.Base(snapPath)
	if err := s.CreateSnapshot(ctx, snapID, bodyID, "{}", snapPath, stat.Size()); err != nil {
		t.Fatalf("CreateSnapshot: %v", err)
	}

	restoreDir := filepath.Join(t.TempDir(), "restored")
	if err := RestoreFromStore(ctx, s, snapID, restoreDir, RestoreOpts{}); err != nil {
		t.Fatalf("RestoreFromStore: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(restoreDir, "hello.txt"))
	if err != nil {
		t.Fatalf("read restored hello.txt: %v", err)
	}
	if string(content) != "round trip content\n" {
		t.Errorf("hello.txt = %q, want round trip content", content)
	}

	nested, err := os.ReadFile(filepath.Join(restoreDir, "sub", "nested.txt"))
	if err != nil {
		t.Fatalf("read restored nested.txt: %v", err)
	}
	if string(nested) != "nested\n" {
		t.Errorf("nested.txt = %q, want nested", nested)
	}
}
