// Package restore handles filesystem restoration from snapshots created by the snapshot package.
//
// Restore pipeline: verify SHA-256 hash → decompress .tar.zst → extract tar to temp dir → atomic rename.
// Transactional: on any failure the temp extraction directory is cleaned up.
package restore

import (
	"archive/tar"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/klauspost/compress/zstd"
)

// VerifyHash reads the .sha256 sidecar and compares it against the actual SHA-256
// of the .tar.zst file at tarPath. Returns an error on mismatch or I/O failure.
func VerifyHash(tarPath string, hashPath string) error {
	expected, err := os.ReadFile(hashPath)
	if err != nil {
		return fmt.Errorf("restore: read hash file %q: %w", hashPath, err)
	}

	// Trim whitespace/newline from stored hash.
	expectedHex := strings.TrimSpace(string(expected))

	f, err := os.Open(tarPath)
	if err != nil {
		return fmt.Errorf("restore: open tarball %q: %w", tarPath, err)
	}
	defer f.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		return fmt.Errorf("restore: hash tarball: %w", err)
	}

	actualHex := hex.EncodeToString(hasher.Sum(nil))

	if actualHex != expectedHex {
		return fmt.Errorf("restore: hash mismatch: expected %s, got %s", expectedHex, actualHex)
	}

	return nil
}

// RestoreOpts holds optional parameters for restore operations.
type RestoreOpts struct {
	PostRestoreCmd string
	HookTimeout    time.Duration
}

// Restore restores a snapshot to targetDir using default options.
func Restore(ctx context.Context, snapshotPath string, targetDir string) error {
	return RestoreWithOpts(ctx, snapshotPath, targetDir, RestoreOpts{})
}

// RestoreWithOpts restores a snapshot to targetDir with optional hooks.
func RestoreWithOpts(ctx context.Context, snapshotPath string, targetDir string, opts RestoreOpts) error {
	// Pre-flight: parent of target must exist and be writable.
	parentDir := filepath.Dir(targetDir)
	if err := checkWritableDir(parentDir); err != nil {
		return fmt.Errorf("restore: pre-flight: %w", err)
	}

	// Verify integrity.
	hashPath := snapshotPath + ".sha256"
	if err := VerifyHash(snapshotPath, hashPath); err != nil {
		return err
	}

	// Open tarball.
	tarFile, err := os.Open(snapshotPath)
	if err != nil {
		return fmt.Errorf("restore: open snapshot %q: %w", snapshotPath, err)
	}
	defer tarFile.Close()

	// Create temp dir in same filesystem as target parent for atomic rename.
	tmpDir, err := os.MkdirTemp(parentDir, ".mesh-restore-*")
	if err != nil {
		return fmt.Errorf("restore: create temp dir: %w", err)
	}

	// Ensure cleanup on any failure.
	cleanup := true
	defer func() {
		if cleanup {
			os.RemoveAll(tmpDir)
		}
	}()

	// Decompress and extract.
	zr, err := zstd.NewReader(tarFile)
	if err != nil {
		return fmt.Errorf("restore: zstd reader: %w", err)
	}
	defer zr.Close()

	if err := extractTar(ctx, zr, tmpDir); err != nil {
		return fmt.Errorf("restore: extract: %w", err)
	}

	// Remove existing target if present.
	if _, err := os.Stat(targetDir); err == nil {
		if err := os.RemoveAll(targetDir); err != nil {
			return fmt.Errorf("restore: remove existing target %q: %w", targetDir, err)
		}
	}

	// Atomic rename.
	if err := os.Rename(tmpDir, targetDir); err != nil {
		// EXDEV fallback: cross-device rename.
		if isEXDEV(err) {
			if copyErr := recursiveCopy(tmpDir, targetDir); copyErr != nil {
				return fmt.Errorf("restore: cross-device copy: %w", copyErr)
			}
			os.RemoveAll(tmpDir)
		} else {
			return fmt.Errorf("restore: rename %q → %q: %w", tmpDir, targetDir, err)
		}
	}

	cleanup = false

	if opts.PostRestoreCmd != "" {
		timeout := opts.HookTimeout
		if timeout <= 0 {
			timeout = 30 * time.Second
		}
		if err := runPostRestoreHook(ctx, opts.PostRestoreCmd, targetDir, timeout); err != nil {
			return fmt.Errorf("restore: post-restore hook failed: %w", err)
		}
	}

	return nil
}

func runPostRestoreHook(ctx context.Context, cmd string, dir string, timeout time.Duration) error {
	hookCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	execCmd := exec.CommandContext(hookCtx, "sh", "-c", cmd)
	execCmd.Dir = dir
	execCmd.Stdout = os.Stderr
	execCmd.Stderr = os.Stderr

	if err := execCmd.Run(); err != nil {
		if hookCtx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("hook timed out after %s: %w", timeout, err)
		}
		return err
	}
	return nil
}

// extractTar extracts a tar stream into dstDir. Handles regular files, directories, and symlinks.
func extractTar(ctx context.Context, r io.Reader, dstDir string) error {
	tr := tar.NewReader(r)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("tar next: %w", err)
		}

		target := filepath.Join(dstDir, filepath.FromSlash(header.Name))

		// Security: prevent path traversal.
		if !strings.HasPrefix(target, filepath.Clean(dstDir)+string(os.PathSeparator)) && target != filepath.Clean(dstDir) {
			return fmt.Errorf("restore: path traversal: %q escapes target dir", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, fs.FileMode(header.Mode)); err != nil {
				return fmt.Errorf("mkdir %q: %w", target, err)
			}
			if err := os.Chmod(target, fs.FileMode(header.Mode)); err != nil {
				return fmt.Errorf("chmod dir %q: %w", target, err)
			}

		case tar.TypeReg:
			// Ensure parent directory exists.
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return fmt.Errorf("mkdir parent %q: %w", filepath.Dir(target), err)
			}

			f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, fs.FileMode(header.Mode))
			if err != nil {
				return fmt.Errorf("create file %q: %w", target, err)
			}

			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return fmt.Errorf("write file %q: %w", target, err)
			}
			f.Close()

			if err := os.Chmod(target, fs.FileMode(header.Mode)); err != nil {
				return fmt.Errorf("chmod file %q: %w", target, err)
			}

		case tar.TypeSymlink:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return fmt.Errorf("mkdir parent %q: %w", filepath.Dir(target), err)
			}
			if err := os.Symlink(header.Linkname, target); err != nil {
				return fmt.Errorf("symlink %q → %q: %w", target, header.Linkname, err)
			}

		default:
			return fmt.Errorf("restore: unsupported tar entry type %d for %q", header.Typeflag, header.Name)
		}
	}

	return nil
}

// checkWritableDir verifies that dir exists and is writable by creating and removing a temp file.
func checkWritableDir(dir string) error {
	info, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("stat %q: %w", dir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%q is not a directory", dir)
	}

	// Probe writability.
	f, err := os.CreateTemp(dir, ".mesh-write-test-*")
	if err != nil {
		return fmt.Errorf("%q is not writable: %w", dir, err)
	}
	f.Close()
	os.Remove(f.Name())

	return nil
}

// isEXDEV checks if err is a cross-device link error.
func isEXDEV(err error) bool {
	for ; err != nil; err = unwrap(err) {
		if errno, ok := err.(interface{ ErrorCode() int }); ok {
			return errno.ErrorCode() == 18 // syscall.EXDEV
		}
	}
	// Fallback: check error string.
	return strings.Contains(err.Error(), "invalid cross-device link")
}

// unwrap returns the underlying error, supporting both Unwrap() and raw errno types.
func unwrap(err error) error {
	if u, ok := err.(interface{ Unwrap() error }); ok {
		return u.Unwrap()
	}
	return nil
}

// recursiveCopy copies all entries from src to dst recursively, then removes src.
func recursiveCopy(src, dst string) error {
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return fmt.Errorf("mkdir %q: %w", dst, err)
	}

	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return fmt.Errorf("rel path: %w", err)
		}
		targetPath := filepath.Join(dst, rel)

		if d.IsDir() {
			info, err := d.Info()
			if err != nil {
				return err
			}
			return os.MkdirAll(targetPath, info.Mode())
		}

		info, err := d.Info()
		if err != nil {
			return err
		}

		// Handle symlinks.
		if info.Mode()&os.ModeSymlink != 0 {
			linkTarget, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return os.Symlink(linkTarget, targetPath)
		}

		// Regular file copy.
		return copyFile(path, targetPath, info.Mode())
	})
}

// copyFile copies a single file preserving permissions.
func copyFile(src, dst string, mode fs.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}

	_, err = io.Copy(out, in)
	out.Close()
	if err != nil {
		return err
	}

	return os.Chmod(dst, mode)
}
