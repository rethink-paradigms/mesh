// Package snapshot handles filesystem snapshot creation: tar + zstd compression + SHA-256 hashing.
//
// Streaming pipeline: sorted dir walk → tar → io.Pipe → zstd → SHA-256 tee → output file.
// Deterministic output via lexicographic entry sorting within each directory.
package snapshot

import (
	"archive/tar"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/klauspost/compress/zstd"
	configtoml "github.com/rethink-paradigms/mesh/internal/config-toml"
	"github.com/rethink-paradigms/mesh/internal/manifest"
)

// CreateSnapshot creates a compressed, hashed snapshot of the filesystem rooted at workdir.
// Output: .tar.zst at outputPath, .sha256 sidecar with hex-encoded digest.
// Streams via io.Pipe — no full tarball buffered in memory. Context cancellation cleans up partial files.
func CreateSnapshot(ctx context.Context, workdir string, outputPath string) error {
	pipeReader, pipeWriter := io.Pipe()
	tarErrCh := make(chan error, 1)

	go func() {
		defer pipeWriter.Close()
		tarErrCh <- writeSortedTar(ctx, workdir, pipeWriter)
	}()

	outFile, err := os.Create(outputPath)
	if err != nil {
		pipeReader.Close()
		<-tarErrCh
		return fmt.Errorf("create output file: %w", err)
	}
	defer outFile.Close()

	hasher := sha256.New()
	mw := io.MultiWriter(outFile, hasher)
	zw, err := zstd.NewWriter(mw)
	if err != nil {
		pipeReader.Close()
		<-tarErrCh
		return fmt.Errorf("create zstd writer: %w", err)
	}

	if _, err := io.Copy(zw, pipeReader); err != nil {
		zw.Close()
		cleanup(outputPath)
		<-tarErrCh
		return fmt.Errorf("compress pipeline: %w", err)
	}

	if tarErr := <-tarErrCh; tarErr != nil {
		zw.Close()
		cleanup(outputPath)
		return fmt.Errorf("tar writing: %w", tarErr)
	}

	if err := zw.Close(); err != nil {
		cleanup(outputPath)
		return fmt.Errorf("flush zstd: %w", err)
	}

	digest := hex.EncodeToString(hasher.Sum(nil))
	shaPath := outputPath + ".sha256"
	if err := os.WriteFile(shaPath, []byte(digest+"\n"), 0o644); err != nil {
		cleanup(outputPath)
		cleanup(shaPath)
		return fmt.Errorf("write sha256 sidecar: %w", err)
	}

	return nil
}

func writeSortedTar(ctx context.Context, root string, w io.Writer) error {
	tw := tar.NewWriter(w)
	defer tw.Close()

	info, err := os.Lstat(root)
	if err != nil {
		return fmt.Errorf("stat root %q: %w", root, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("root %q is not a directory", root)
	}

	return walkDir(ctx, root, root, tw)
}

func walkDir(ctx context.Context, root, dirPath string, tw *tar.Writer) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return fmt.Errorf("read dir %q: %w", dirPath, err)
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	for _, entry := range entries {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		fullPath := filepath.Join(dirPath, entry.Name())
		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("info %q: %w", fullPath, err)
		}

		switch {
		case info.Mode().IsRegular():
			if err := writeFileEntry(tw, root, fullPath, info); err != nil {
				return err
			}
		case info.IsDir():
			if err := writeDirEntry(tw, root, fullPath, info); err != nil {
				return err
			}
			if err := walkDir(ctx, root, fullPath, tw); err != nil {
				return err
			}
		case info.Mode()&os.ModeSymlink != 0:
			if err := writeSymlinkEntry(tw, root, fullPath, info); err != nil {
				return err
			}
		}
	}

	return nil
}

func writeFileEntry(tw *tar.Writer, root, fullPath string, info os.FileInfo) error {
	header, err := tar.FileInfoHeader(info, "")
	if err != nil {
		return fmt.Errorf("header %q: %w", fullPath, err)
	}
	setRelName(header, root, fullPath, false)
	clearNonDeterministic(header)

	if err := tw.WriteHeader(header); err != nil {
		return fmt.Errorf("write header %q: %w", header.Name, err)
	}

	f, err := os.Open(fullPath)
	if err != nil {
		return fmt.Errorf("open %q: %w", fullPath, err)
	}
	defer f.Close()

	if _, err := io.Copy(tw, f); err != nil {
		return fmt.Errorf("copy %q: %w", fullPath, err)
	}
	return nil
}

func writeDirEntry(tw *tar.Writer, root, fullPath string, info os.FileInfo) error {
	header, err := tar.FileInfoHeader(info, "")
	if err != nil {
		return fmt.Errorf("header %q: %w", fullPath, err)
	}
	setRelName(header, root, fullPath, true)
	clearNonDeterministic(header)

	if err := tw.WriteHeader(header); err != nil {
		return fmt.Errorf("write header %q: %w", header.Name, err)
	}
	return nil
}

func writeSymlinkEntry(tw *tar.Writer, root, fullPath string, info os.FileInfo) error {
	target, err := os.Readlink(fullPath)
	if err != nil {
		return fmt.Errorf("readlink %q: %w", fullPath, err)
	}

	header, err := tar.FileInfoHeader(info, target)
	if err != nil {
		return fmt.Errorf("header %q: %w", fullPath, err)
	}
	setRelName(header, root, fullPath, false)
	header.Linkname = target
	clearNonDeterministic(header)

	if err := tw.WriteHeader(header); err != nil {
		return fmt.Errorf("write header %q: %w", header.Name, err)
	}
	return nil
}

func setRelName(header *tar.Header, root, fullPath string, isDir bool) {
	rel, _ := filepath.Rel(root, fullPath)
	rel = filepath.ToSlash(rel)
	if isDir && rel != "." {
		rel += "/"
	}
	header.Name = rel
}

// clearNonDeterministic zeroes fields that vary across machines/times for reproducible archives.
func clearNonDeterministic(header *tar.Header) {
	header.Uid = 0
	header.Gid = 0
	header.Uname = ""
	header.Gname = ""
	header.Devmajor = 0
	header.Devminor = 0
}

func cleanup(path string) {
	os.Remove(path)
}

// ResolveAgent finds an agent by name in the config. Returns error if not found.
func ResolveAgent(cfg *configtoml.Config, name string) (*configtoml.Agent, error) {
	for i := range cfg.Agents {
		if cfg.Agents[i].Name == name {
			return &cfg.Agents[i], nil
		}
	}
	return nil, fmt.Errorf("snapshot: agent %q not found in config", name)
}

// SnapshotCacheDir returns the directory where snapshots for the given agent are stored.
// Default: ~/.mesh/snapshots/{agentName}/
func SnapshotCacheDir(agentName string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("snapshot: get home dir: %w", err)
	}
	return filepath.Join(home, ".mesh", "snapshots", agentName), nil
}

// parseHookTimeout parses the agent's StopTimeout as a duration, falling back to 30s.
func parseHookTimeout(stopTimeout string) time.Duration {
	if d, err := time.ParseDuration(stopTimeout); err == nil && d > 0 {
		return d
	}
	return 30 * time.Second
}

// runHook executes a shell command in the given directory with a timeout.
// Hook stdout/stderr is forwarded to stderr for visibility.
func runHook(ctx context.Context, cmd string, dir string, timeout time.Duration) error {
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

// Run orchestrates the full snapshot workflow for a named agent:
// 1. Resolve agent from config by name
// 2. Expand ~ in workdir and verify it exists
// 3. Create snapshot cache directory
// 4. Generate timestamped snapshot filename
// 5. Create the snapshot via CreateSnapshot
// 6. Enforce max_snapshots limit by pruning oldest
//
// If cacheDir is empty, uses the default ~/.mesh/snapshots/{agentName}/.
func Run(ctx context.Context, cfg *configtoml.Config, agentName string, cacheDir string) error {
	agent, err := ResolveAgent(cfg, agentName)
	if err != nil {
		return err
	}

	workdir, err := configtoml.ExpandPath(agent.Workdir)
	if err != nil {
		return fmt.Errorf("snapshot: expand workdir: %w", err)
	}

	info, err := os.Stat(workdir)
	if err != nil {
		return fmt.Errorf("snapshot: stat workdir %q: %w", workdir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("snapshot: workdir %q is not a directory", workdir)
	}
	if _, err := os.ReadDir(workdir); err != nil {
		return fmt.Errorf("snapshot: workdir %q is not readable: %w", workdir, err)
	}

	if cacheDir == "" {
		cacheDir, err = SnapshotCacheDir(agentName)
		if err != nil {
			return err
		}
	}

	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return fmt.Errorf("snapshot: create cache dir %q: %w", cacheDir, err)
	}

	if agent.PreSnapshotCmd != "" {
		timeout := parseHookTimeout(agent.StopTimeout)
		if err := runHook(ctx, agent.PreSnapshotCmd, workdir, timeout); err != nil {
			return fmt.Errorf("snapshot: pre-snapshot hook failed: %w", err)
		}
	}

	timestamp := time.Now().Format("20060102-150405")
	filename := fmt.Sprintf("%s-%s.tar.zst", agentName, timestamp)
	outputPath := filepath.Join(cacheDir, filename)

	if err := CreateSnapshot(ctx, workdir, outputPath); err != nil {
		return fmt.Errorf("snapshot: create: %w", err)
	}

	stat, statErr := os.Stat(outputPath)
	if statErr != nil {
		return fmt.Errorf("snapshot: stat output: %w", statErr)
	}

	shaBytes, shaErr := os.ReadFile(outputPath + ".sha256")
	if shaErr != nil {
		return fmt.Errorf("snapshot: read checksum: %w", shaErr)
	}

	hostname, _ := os.Hostname()

	m := manifest.Manifest{
		AgentName:     agent.Name,
		Timestamp:     time.Now(),
		SourceMachine: hostname,
		SourceWorkdir: workdir,
		StartCmd:      agent.StartCmd,
		StopTimeout:   agent.StopTimeout,
		Checksum:      strings.TrimSpace(string(shaBytes)),
		Size:          stat.Size(),
	}

	if err := manifest.Write(manifest.ManifestPath(outputPath), &m); err != nil {
		return fmt.Errorf("snapshot: write manifest: %w", err)
	}

	if agent.MaxSnapshots > 0 {
		if err := pruneSnapshots(cacheDir, agent.MaxSnapshots); err != nil {
			// Log but don't fail — snapshot was created successfully.
			_ = err
		}
	}

	return nil
}

// pruneSnapshots removes the oldest snapshots (by timestamp in filename) when
// the count exceeds max. Deletes both .tar.zst and .sha256 sidecars.
func pruneSnapshots(cacheDir string, max int) error {
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		return fmt.Errorf("prune: read dir %q: %w", cacheDir, err)
	}

	var snapshots []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".tar.zst") {
			snapshots = append(snapshots, e.Name())
		}
	}

	// Sort by name — timestamp-encoded names sort chronologically.
	sort.Strings(snapshots)

	for len(snapshots) > max {
		oldest := snapshots[0]
		snapshots = snapshots[1:]

		tarPath := filepath.Join(cacheDir, oldest)
		shaPath := tarPath + ".sha256"
		jsonPath := manifest.ManifestPath(tarPath)

		os.Remove(tarPath)
		os.Remove(shaPath)
		os.Remove(jsonPath)
	}

	return nil
}
