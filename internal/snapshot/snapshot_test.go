package snapshot

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/klauspost/compress/zstd"
	configtoml "github.com/rethink-paradigms/mesh/internal/config-toml"
	"github.com/rethink-paradigms/mesh/internal/manifest"
)

func TestHashRoundTrip(t *testing.T) {
	srcDir := t.TempDir()

	mustWriteFile(t, filepath.Join(srcDir, "hello.txt"), []byte("hello world\n"), 0o644)
	mustMkdir(t, filepath.Join(srcDir, "subdir"))
	mustWriteFile(t, filepath.Join(srcDir, "subdir", "nested.txt"), []byte("nested content\n"), 0o644)
	mustSymlink(t, filepath.Join(srcDir, "link.txt"), "hello.txt")
	mustWriteFile(t, filepath.Join(srcDir, "executable.sh"), []byte("#!/bin/sh\necho hi\n"), 0o755)
	mustWriteFile(t, filepath.Join(srcDir, "secret.txt"), []byte("secret\n"), 0o600)
	mustWriteFile(t, filepath.Join(srcDir, "empty.txt"), []byte(""), 0o644)
	mustWriteFile(t, filepath.Join(srcDir, "日本語.txt"), []byte("unicode content\n"), 0o644)

	snapshotPath := filepath.Join(t.TempDir(), "test.tar.zst")
	if err := CreateSnapshot(context.Background(), srcDir, snapshotPath); err != nil {
		t.Fatalf("CreateSnapshot: %v", err)
	}

	t.Run("SidecarHashMatches", func(t *testing.T) {
		shaBytes, err := os.ReadFile(snapshotPath + ".sha256")
		if err != nil {
			t.Fatalf("read sha256 sidecar: %v", err)
		}
		storedHash := string(bytes.TrimSpace(shaBytes))

		f, err := os.Open(snapshotPath)
		if err != nil {
			t.Fatalf("open snapshot: %v", err)
		}
		defer f.Close()

		h := sha256.New()
		if _, err := io.Copy(h, f); err != nil {
			t.Fatalf("hash snapshot: %v", err)
		}
		computedHash := hex.EncodeToString(h.Sum(nil))

		if storedHash != computedHash {
			t.Errorf("hash mismatch: sidecar=%s computed=%s", storedHash, computedHash)
		}
	})

	t.Run("ExtractAndCompare", func(t *testing.T) {
		restoreDir := t.TempDir()
		if err := extractTarZst(snapshotPath, restoreDir); err != nil {
			t.Fatalf("extract: %v", err)
		}

		assertFileContent(t, restoreDir, "hello.txt", []byte("hello world\n"))
		assertFileContent(t, restoreDir, "subdir/nested.txt", []byte("nested content\n"))
		assertFileContent(t, restoreDir, "executable.sh", []byte("#!/bin/sh\necho hi\n"))
		assertFileContent(t, restoreDir, "secret.txt", []byte("secret\n"))
		assertFileContent(t, restoreDir, "empty.txt", []byte(""))
		assertFileContent(t, restoreDir, "日本語.txt", []byte("unicode content\n"))

		assertSymlink(t, restoreDir, "link.txt", "hello.txt")

		assertPerm(t, restoreDir, "executable.sh", 0o755)
		assertPerm(t, restoreDir, "secret.txt", 0o600)

		assertDirExists(t, restoreDir, "subdir")

		assertSameFileSet(t, srcDir, restoreDir)
	})
}

func TestDeterministicHash(t *testing.T) {
	srcDir := t.TempDir()
	mustWriteFile(t, filepath.Join(srcDir, "a.txt"), []byte("content a\n"), 0o644)
	mustWriteFile(t, filepath.Join(srcDir, "b.txt"), []byte("content b\n"), 0o644)
	mustMkdir(t, filepath.Join(srcDir, "dir"))
	mustWriteFile(t, filepath.Join(srcDir, "dir", "c.txt"), []byte("content c\n"), 0o644)

	outDir := t.TempDir()
	path1 := filepath.Join(outDir, "snap1.tar.zst")
	path2 := filepath.Join(outDir, "snap2.tar.zst")

	if err := CreateSnapshot(context.Background(), srcDir, path1); err != nil {
		t.Fatalf("snapshot 1: %v", err)
	}
	if err := CreateSnapshot(context.Background(), srcDir, path2); err != nil {
		t.Fatalf("snapshot 2: %v", err)
	}

	hash1 := mustReadTrimmedFile(t, path1+".sha256")
	hash2 := mustReadTrimmedFile(t, path2+".sha256")

	if hash1 != hash2 {
		t.Errorf("non-deterministic hashes:\n  snap1: %s\n  snap2: %s", hash1, hash2)
	}
}

func TestEmptyDirectory(t *testing.T) {
	srcDir := t.TempDir()

	snapshotPath := filepath.Join(t.TempDir(), "empty.tar.zst")
	if err := CreateSnapshot(context.Background(), srcDir, snapshotPath); err != nil {
		t.Fatalf("CreateSnapshot on empty dir: %v", err)
	}

	if _, err := os.Stat(snapshotPath); err != nil {
		t.Fatalf("snapshot file missing: %v", err)
	}
	if _, err := os.Stat(snapshotPath + ".sha256"); err != nil {
		t.Fatalf("sha256 sidecar missing: %v", err)
	}

	restoreDir := t.TempDir()
	if err := extractTarZst(snapshotPath, restoreDir); err != nil {
		t.Fatalf("extract empty snapshot: %v", err)
	}

	entries, err := os.ReadDir(restoreDir)
	if err != nil {
		t.Fatalf("read restored dir: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected empty restored dir, got %d entries", len(entries))
	}
}

func TestPermissionPreservation(t *testing.T) {
	srcDir := t.TempDir()
	mustWriteFile(t, filepath.Join(srcDir, "exec.sh"), []byte("#!/bin/sh\n"), 0o755)
	mustWriteFile(t, filepath.Join(srcDir, "private.txt"), []byte("secret\n"), 0o600)
	mustWriteFile(t, filepath.Join(srcDir, "readonly.txt"), []byte("ro\n"), 0o444)

	snapshotPath := filepath.Join(t.TempDir(), "perms.tar.zst")
	if err := CreateSnapshot(context.Background(), srcDir, snapshotPath); err != nil {
		t.Fatalf("CreateSnapshot: %v", err)
	}

	restoreDir := t.TempDir()
	if err := extractTarZst(snapshotPath, restoreDir); err != nil {
		t.Fatalf("extract: %v", err)
	}

	assertPerm(t, restoreDir, "exec.sh", 0o755)
	assertPerm(t, restoreDir, "private.txt", 0o600)
	assertPerm(t, restoreDir, "readonly.txt", 0o444)
}

func TestSymlinkPreservation(t *testing.T) {
	srcDir := t.TempDir()
	mustWriteFile(t, filepath.Join(srcDir, "target.txt"), []byte("target\n"), 0o644)
	mustSymlink(t, filepath.Join(srcDir, "link1.txt"), "target.txt")
	mustMkdir(t, filepath.Join(srcDir, "subdir"))
	mustWriteFile(t, filepath.Join(srcDir, "subdir", "other.txt"), []byte("other\n"), 0o644)
	mustSymlink(t, filepath.Join(srcDir, "link2.txt"), filepath.Join("subdir", "other.txt"))

	snapshotPath := filepath.Join(t.TempDir(), "symlinks.tar.zst")
	if err := CreateSnapshot(context.Background(), srcDir, snapshotPath); err != nil {
		t.Fatalf("CreateSnapshot: %v", err)
	}

	restoreDir := t.TempDir()
	if err := extractTarZst(snapshotPath, restoreDir); err != nil {
		t.Fatalf("extract: %v", err)
	}

	assertSymlink(t, restoreDir, "link1.txt", "target.txt")
	assertSymlink(t, restoreDir, "link2.txt", filepath.Join("subdir", "other.txt"))
}

func extractTarZst(archivePath, destDir string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()

	zr, err := zstd.NewReader(f)
	if err != nil {
		return err
	}
	defer zr.Close()

	tr := tar.NewReader(zr)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		target := filepath.Join(destDir, filepath.FromSlash(header.Name))

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, os.FileMode(header.Mode)); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return err
			}
			out.Close()
		case tar.TypeSymlink:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			if err := os.Symlink(header.Linkname, target); err != nil {
				return err
			}
		}
	}

	return nil
}

func assertSameFileSet(t *testing.T, srcDir, restoreDir string) {
	t.Helper()
	srcFiles := collectRelPaths(t, srcDir)
	restoreFiles := collectRelPaths(t, restoreDir)

	if len(srcFiles) != len(restoreFiles) {
		t.Errorf("file count mismatch: src=%d restore=%d", len(srcFiles), len(restoreFiles))
	}

	for name := range srcFiles {
		if !restoreFiles[name] {
			t.Errorf("missing in restore: %q", name)
		}
	}
	for name := range restoreFiles {
		if !srcFiles[name] {
			t.Errorf("extra in restore: %q", name)
		}
	}
}

func collectRelPaths(t *testing.T, root string) map[string]bool {
	t.Helper()
	paths := make(map[string]bool)
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		paths[rel] = true
		return nil
	})
	if err != nil {
		t.Fatalf("walk %q: %v", root, err)
	}
	return paths
}

func assertFileContent(t *testing.T, dir, name string, want []byte) {
	t.Helper()
	got, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Errorf("read %q: %v", name, err)
		return
	}
	if !bytes.Equal(got, want) {
		t.Errorf("content %q: got %q, want %q", name, got, want)
	}
}

func assertSymlink(t *testing.T, dir, name, wantTarget string) {
	t.Helper()
	target, err := os.Readlink(filepath.Join(dir, name))
	if err != nil {
		t.Errorf("readlink %q: %v", name, err)
		return
	}
	if target != wantTarget {
		t.Errorf("symlink %q: got target %q, want %q", name, target, wantTarget)
	}
}

func assertPerm(t *testing.T, dir, name string, want os.FileMode) {
	t.Helper()
	info, err := os.Lstat(filepath.Join(dir, name))
	if err != nil {
		t.Errorf("lstat %q: %v", name, err)
		return
	}
	got := info.Mode().Perm()
	if got != want {
		t.Errorf("perm %q: got %04o, want %04o", name, got, want)
	}
}

func assertDirExists(t *testing.T, dir, name string) {
	t.Helper()
	info, err := os.Lstat(filepath.Join(dir, name))
	if err != nil {
		t.Errorf("stat %q: %v", name, err)
		return
	}
	if !info.IsDir() {
		t.Errorf("%q is not a directory", name)
	}
}

func mustWriteFile(t *testing.T, path string, content []byte, perm os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, content, perm); err != nil {
		t.Fatalf("write %q: %v", path, err)
	}
	if err := os.Chmod(path, perm); err != nil {
		t.Fatalf("chmod %q: %v", path, err)
	}
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir %q: %v", path, err)
	}
}

func mustSymlink(t *testing.T, path, target string) {
	t.Helper()
	if err := os.Symlink(target, path); err != nil {
		t.Fatalf("symlink %q → %q: %v", path, target, err)
	}
}

func mustReadTrimmedFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %q: %v", path, err)
	}
	return string(bytes.TrimSpace(b))
}

func mustWriteManifest(t *testing.T, path string, m *manifest.Manifest) {
	t.Helper()
	if err := manifest.Write(path, m); err != nil {
		t.Fatal(err)
	}
}

func TestResolveAgentFound(t *testing.T) {
	cfg := &configtoml.Config{
		Agents: []configtoml.Agent{
			{Name: "alpha", Workdir: "/tmp/alpha"},
			{Name: "beta", Workdir: "/tmp/beta"},
		},
	}
	agent, err := ResolveAgent(cfg, "beta")
	if err != nil {
		t.Fatalf("ResolveAgent: %v", err)
	}
	if agent.Name != "beta" {
		t.Errorf("got name %q, want %q", agent.Name, "beta")
	}
	if agent.Workdir != "/tmp/beta" {
		t.Errorf("got workdir %q, want %q", agent.Workdir, "/tmp/beta")
	}
}

func TestResolveAgentNotFound(t *testing.T) {
	cfg := &configtoml.Config{
		Agents: []configtoml.Agent{
			{Name: "alpha", Workdir: "/tmp/alpha"},
		},
	}
	_, err := ResolveAgent(cfg, "missing")
	if err == nil {
		t.Fatal("expected error for missing agent, got nil")
	}
	if !strings.Contains(err.Error(), "missing") {
		t.Errorf("error should mention agent name: %v", err)
	}
}

func TestRunCreatesSnapshot(t *testing.T) {
	workdir := t.TempDir()
	mustWriteFile(t, filepath.Join(workdir, "data.txt"), []byte("test data\n"), 0o644)

	cacheDir := t.TempDir()
	cfg := &configtoml.Config{
		Agents: []configtoml.Agent{
			{Name: "test-agent", Workdir: workdir, MaxSnapshots: 10},
		},
	}

	err := Run(context.Background(), cfg, "test-agent", cacheDir)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		t.Fatalf("read cache dir: %v", err)
	}

	var tarFiles []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tar.zst") {
			tarFiles = append(tarFiles, e.Name())
		}
	}
	if len(tarFiles) != 1 {
		t.Fatalf("expected 1 .tar.zst file, got %d: %v", len(tarFiles), tarFiles)
	}

	expectedPrefix := "test-agent-"
	if !strings.HasPrefix(tarFiles[0], expectedPrefix) {
		t.Errorf("filename %q should start with %q", tarFiles[0], expectedPrefix)
	}
	if !strings.HasSuffix(tarFiles[0], ".tar.zst") {
		t.Errorf("filename %q should end with .tar.zst", tarFiles[0])
	}

	tarPath := filepath.Join(cacheDir, tarFiles[0])
	shaPath := tarPath + ".sha256"
	if _, err := os.Stat(shaPath); err != nil {
		t.Errorf("sha256 sidecar missing: %v", err)
	}

	jsonPath := manifest.ManifestPath(tarPath)
	if _, err := os.Stat(jsonPath); err != nil {
		t.Errorf("json manifest sidecar missing: %v", err)
	}

	restoreDir := t.TempDir()
	if err := extractTarZst(tarPath, restoreDir); err != nil {
		t.Fatalf("extract snapshot: %v", err)
	}
	assertFileContent(t, restoreDir, "data.txt", []byte("test data\n"))
}

func TestRunAgentNotFound(t *testing.T) {
	cacheDir := t.TempDir()
	cfg := &configtoml.Config{
		Agents: []configtoml.Agent{
			{Name: "other", Workdir: "/tmp/other"},
		},
	}

	err := Run(context.Background(), cfg, "nonexistent", cacheDir)
	if err == nil {
		t.Fatal("expected error for unknown agent, got nil")
	}
	if !strings.Contains(err.Error(), "nonexistent") {
		t.Errorf("error should mention agent name: %v", err)
	}
}

func TestRunNonExistentWorkdir(t *testing.T) {
	cacheDir := t.TempDir()
	cfg := &configtoml.Config{
		Agents: []configtoml.Agent{
			{Name: "test-agent", Workdir: "/nonexistent/path/that/does/not/exist", MaxSnapshots: 5},
		},
	}

	err := Run(context.Background(), cfg, "test-agent", cacheDir)
	if err == nil {
		t.Fatal("expected error for nonexistent workdir, got nil")
	}
	if !strings.Contains(err.Error(), "workdir") {
		t.Errorf("error should mention workdir: %v", err)
	}
}

func TestRunUnreadableWorkdir(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("skipping: running as root, cannot test permission denial")
	}

	workdir := t.TempDir()
	mustWriteFile(t, filepath.Join(workdir, "data.txt"), []byte("test\n"), 0o644)

	if err := os.Chmod(workdir, 0o000); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	defer os.Chmod(workdir, 0o755)

	cacheDir := t.TempDir()
	cfg := &configtoml.Config{
		Agents: []configtoml.Agent{
			{Name: "test-agent", Workdir: workdir, MaxSnapshots: 5},
		},
	}

	err := Run(context.Background(), cfg, "test-agent", cacheDir)
	if err == nil {
		t.Fatal("expected error for unreadable workdir, got nil")
	}
}

func TestRunMaxSnapshots(t *testing.T) {
	workdir := t.TempDir()
	mustWriteFile(t, filepath.Join(workdir, "data.txt"), []byte("test\n"), 0o644)

	cacheDir := t.TempDir()
	maxSnapshots := 3

	oldNames := []string{
		"agent-20260420-010000.tar.zst",
		"agent-20260421-010000.tar.zst",
		"agent-20260422-010000.tar.zst",
	}
	for _, name := range oldNames {
		tarPath := filepath.Join(cacheDir, name)
		if err := os.WriteFile(tarPath, []byte("fake"), 0o644); err != nil {
			t.Fatalf("write %q: %v", name, err)
		}
		if err := os.WriteFile(tarPath+".sha256", []byte("abcd\n"), 0o644); err != nil {
			t.Fatalf("write sha256: %v", err)
		}
		mustWriteManifest(t, manifest.ManifestPath(tarPath), &manifest.Manifest{
			AgentName: "agent",
			Checksum:  "abcd",
		})
	}

	for i := 0; i < 2; i++ {
		cfg := &configtoml.Config{
			Agents: []configtoml.Agent{
				{Name: "agent", Workdir: workdir, MaxSnapshots: maxSnapshots},
			},
		}
		err := Run(context.Background(), cfg, "agent", cacheDir)
		if err != nil {
			t.Fatalf("Run %d: %v", i, err)
		}
		if i == 0 {
			time.Sleep(1100 * time.Millisecond)
		}
	}

	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		t.Fatalf("read cache dir: %v", err)
	}

	var tarFiles []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tar.zst") {
			tarFiles = append(tarFiles, e.Name())
		}
	}

	if len(tarFiles) != maxSnapshots {
		t.Errorf("expected %d snapshots after pruning, got %d: %v", maxSnapshots, len(tarFiles), tarFiles)
	}

	var shaFiles []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".sha256") {
			shaFiles = append(shaFiles, e.Name())
		}
	}
	if len(shaFiles) != maxSnapshots {
		t.Errorf("expected %d sha256 sidecars, got %d: %v", maxSnapshots, len(shaFiles), shaFiles)
	}

	var jsonFiles []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".json") {
			jsonFiles = append(jsonFiles, e.Name())
		}
	}
	if len(jsonFiles) != maxSnapshots {
		t.Errorf("expected %d json sidecars, got %d: %v", maxSnapshots, len(jsonFiles), jsonFiles)
	}

	sort.Strings(tarFiles)
	for _, name := range tarFiles {
		if strings.HasPrefix(name, "agent-20260420") {
			t.Errorf("oldest snapshot should have been pruned: %q", name)
		}
	}
}

func TestTimestampedFilename(t *testing.T) {
	agentName := "myagent"
	ts := time.Date(2026, 4, 23, 14, 30, 45, 0, time.UTC)
	filename := fmt.Sprintf("%s-%s.tar.zst", agentName, ts.Format("20060102-150405"))
	expected := "myagent-20260423-143045.tar.zst"
	if filename != expected {
		t.Errorf("got %q, want %q", filename, expected)
	}
}

func TestPreSnapshotHook(t *testing.T) {
	workdir := t.TempDir()
	mustWriteFile(t, filepath.Join(workdir, "data.txt"), []byte("test\n"), 0o644)

	cacheDir := t.TempDir()
	cfg := &configtoml.Config{
		Agents: []configtoml.Agent{
			{
				Name:           "hook-agent",
				Workdir:        workdir,
				MaxSnapshots:   10,
				PreSnapshotCmd: "touch pre_hook_ran",
				StopTimeout:    "30s",
			},
		},
	}

	err := Run(context.Background(), cfg, "hook-agent", cacheDir)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if _, err := os.Stat(filepath.Join(workdir, "pre_hook_ran")); err != nil {
		t.Errorf("pre_hook_ran file not created by hook: %v", err)
	}

	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		t.Fatalf("read cache dir: %v", err)
	}
	var tarFiles []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tar.zst") {
			tarFiles = append(tarFiles, e.Name())
		}
	}
	if len(tarFiles) != 1 {
		t.Errorf("expected 1 snapshot, got %d", len(tarFiles))
	}
}

func TestPreSnapshotHookFailure(t *testing.T) {
	workdir := t.TempDir()
	mustWriteFile(t, filepath.Join(workdir, "data.txt"), []byte("test\n"), 0o644)

	cacheDir := t.TempDir()
	cfg := &configtoml.Config{
		Agents: []configtoml.Agent{
			{
				Name:           "fail-agent",
				Workdir:        workdir,
				MaxSnapshots:   10,
				PreSnapshotCmd: "exit 1",
				StopTimeout:    "30s",
			},
		},
	}

	err := Run(context.Background(), cfg, "fail-agent", cacheDir)
	if err == nil {
		t.Fatal("expected error from hook failure, got nil")
	}
	if !strings.Contains(err.Error(), "pre-snapshot hook failed") {
		t.Errorf("error = %q, want pre-snapshot hook failed", err.Error())
	}

	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		t.Fatalf("read cache dir: %v", err)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tar.zst") {
			t.Errorf("snapshot should not have been created after hook failure: %s", e.Name())
		}
	}
}

func TestPreSnapshotHookTimeout(t *testing.T) {
	workdir := t.TempDir()
	mustWriteFile(t, filepath.Join(workdir, "data.txt"), []byte("test\n"), 0o644)

	cacheDir := t.TempDir()
	cfg := &configtoml.Config{
		Agents: []configtoml.Agent{
			{
				Name:           "timeout-agent",
				Workdir:        workdir,
				MaxSnapshots:   10,
				PreSnapshotCmd: "sleep 60",
				StopTimeout:    "1s",
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := Run(ctx, cfg, "timeout-agent", cacheDir)
	if err == nil {
		t.Fatal("expected error from hook timeout, got nil")
	}
	if !strings.Contains(err.Error(), "hook timed out") {
		t.Errorf("error = %q, want hook timed out", err.Error())
	}
}

func TestNoHookConfigured(t *testing.T) {
	workdir := t.TempDir()
	mustWriteFile(t, filepath.Join(workdir, "data.txt"), []byte("test\n"), 0o644)

	cacheDir := t.TempDir()
	cfg := &configtoml.Config{
		Agents: []configtoml.Agent{
			{
				Name:         "nohook-agent",
				Workdir:      workdir,
				MaxSnapshots: 10,
				StopTimeout:  "30s",
			},
		},
	}

	err := Run(context.Background(), cfg, "nohook-agent", cacheDir)
	if err != nil {
		t.Fatalf("Run without hook: %v", err)
	}

	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		t.Fatalf("read cache dir: %v", err)
	}
	var tarFiles []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tar.zst") {
			tarFiles = append(tarFiles, e.Name())
		}
	}
	if len(tarFiles) != 1 {
		t.Errorf("expected 1 snapshot, got %d", len(tarFiles))
	}
}

func TestRunCreatesManifest(t *testing.T) {
	workdir := t.TempDir()
	mustWriteFile(t, filepath.Join(workdir, "data.txt"), []byte("hello manifest\n"), 0o644)

	cacheDir := t.TempDir()
	cfg := &configtoml.Config{
		Agents: []configtoml.Agent{
			{
				Name:         "manifest-agent",
				Workdir:      workdir,
				StartCmd:     "./run.sh",
				StopTimeout:  "45s",
				MaxSnapshots: 10,
			},
		},
	}

	before := time.Now()
	err := Run(context.Background(), cfg, "manifest-agent", cacheDir)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	after := time.Now()

	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		t.Fatalf("read cache dir: %v", err)
	}

	var tarFiles []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tar.zst") {
			tarFiles = append(tarFiles, e.Name())
		}
	}
	if len(tarFiles) != 1 {
		t.Fatalf("expected 1 tar file, got %d", len(tarFiles))
	}

	tarPath := filepath.Join(cacheDir, tarFiles[0])
	jsonPath := manifest.ManifestPath(tarPath)
	m, err := manifest.Read(jsonPath)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}

	if m.AgentName != "manifest-agent" {
		t.Errorf("AgentName = %q, want %q", m.AgentName, "manifest-agent")
	}
	if m.Timestamp.Before(before) || m.Timestamp.After(after) {
		t.Errorf("Timestamp = %v, want between %v and %v", m.Timestamp, before, after)
	}
	if m.SourceMachine == "" {
		t.Error("SourceMachine should not be empty")
	}
	if m.SourceWorkdir != workdir {
		t.Errorf("SourceWorkdir = %q, want %q", m.SourceWorkdir, workdir)
	}
	if m.StartCmd != "./run.sh" {
		t.Errorf("StartCmd = %q, want %q", m.StartCmd, "./run.sh")
	}
	if m.StopTimeout != "45s" {
		t.Errorf("StopTimeout = %q, want %q", m.StopTimeout, "45s")
	}

	shaBytes, err := os.ReadFile(tarPath + ".sha256")
	if err != nil {
		t.Fatalf("read sha256 sidecar: %v", err)
	}
	if m.Checksum != strings.TrimSpace(string(shaBytes)) {
		t.Errorf("Checksum = %q, want %q", m.Checksum, strings.TrimSpace(string(shaBytes)))
	}

	stat, err := os.Stat(tarPath)
	if err != nil {
		t.Fatalf("stat tarball: %v", err)
	}
	if m.Size != stat.Size() {
		t.Errorf("Size = %d, want %d", m.Size, stat.Size())
	}
}
