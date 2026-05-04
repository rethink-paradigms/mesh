// Package main implements the mesh CLI with Cobra subcommands.
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/rethink-paradigms/mesh/internal/config"
	configtoml "github.com/rethink-paradigms/mesh/internal/config-toml"
	"github.com/rethink-paradigms/mesh/internal/daemon"
	"github.com/rethink-paradigms/mesh/internal/manifest"
	"github.com/rethink-paradigms/mesh/internal/restore"
	"github.com/rethink-paradigms/mesh/internal/snapshot"
)

func main() {
	rootCmd := newRootCmd()
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		if isUsageError(err) {
			os.Exit(2)
		}
		os.Exit(1)
	}
}

func isUsageError(err error) bool {
	return strings.Contains(err.Error(), "required") ||
		strings.Contains(err.Error(), "accepts") ||
		strings.Contains(err.Error(), "arg(s)")
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "mesh",
		Short:         "Portable agent-body runtime for AI agents",
		Long:          "Mesh gives an agent a persistent compute identity that can live on any substrate and move between them without losing itself.",
		Version:       version(),
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.PersistentFlags().String("config", "", "path to config file (default: ~/.mesh/config.toml or $MESH_CONFIG)")
	root.PersistentFlags().Bool("verbose", false, "enable debug output to stderr")
	root.PersistentFlags().Bool("quiet", false, "suppress progress output")

	root.AddCommand(
		newSnapshotCmd(),
		newRestoreCmd(),
		newListCmd(),
		newInspectCmd(),
		newPruneCmd(),
		newInitCmd(),
		newServeCmd(),
		newStopCmd(),
		newStatusCmd(),
	)

	return root
}

// version returns the build version from Go build info, or a fallback.
func version() string {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return "v0.0.0-dev"
	}
	// Use VCS info if available for a meaningful version.
	var revision, modified string
	for _, s := range bi.Settings {
		switch s.Key {
		case "vcs.revision":
			revision = s.Value
		case "vcs.modified":
			modified = s.Value
		}
	}
	if revision != "" {
		v := revision[:8]
		if modified == "true" {
			v += "-dirty"
		}
		return v
	}
	if bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		return bi.Main.Version
	}
	return "v0.0.0-dev"
}

// loadConfig reads the --config flag or falls back to DefaultPath, then loads and validates.
func loadConfig(cmd *cobra.Command) (*configtoml.Config, error) {
	configPath, _ := cmd.Flags().GetString("config")
	if configPath == "" {
		configPath = configtoml.DefaultPath()
	}
	cfg, err := configtoml.Load(configPath)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	return cfg, nil
}

// findLatestSnapshot returns the path to the most recent snapshot for the agent.
func findLatestSnapshot(agentName string) (string, error) {
	cacheDir, err := snapshot.SnapshotCacheDir(agentName)
	if err != nil {
		return "", err
	}

	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		return "", fmt.Errorf("read snapshot dir: %w", err)
	}

	var snaps []string
	prefix := agentName + "-"
	for _, e := range entries {
		if !e.IsDir() && strings.HasPrefix(e.Name(), prefix) && strings.HasSuffix(e.Name(), ".tar.zst") {
			snaps = append(snaps, e.Name())
		}
	}

	if len(snaps) == 0 {
		return "", fmt.Errorf("no snapshots found for agent %q", agentName)
	}

	sort.Strings(snaps)
	return filepath.Join(cacheDir, snaps[len(snaps)-1]), nil
}

func newSnapshotCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "snapshot <agent>",
		Short: "Create a filesystem snapshot of an agent",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			agentName := args[0]
			cfg, err := loadConfig(cmd)
			if err != nil {
				return err
			}

			ctx := context.Background()
			if err := snapshot.Run(ctx, cfg, agentName, ""); err != nil {
				fmt.Fprintln(cmd.ErrOrStderr(), err)
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Snapshot created for %s\n", agentName)
			return nil
		},
	}
}

func newRestoreCmd() *cobra.Command {
	var snapshotPath string

	cmd := &cobra.Command{
		Use:   "restore <agent>",
		Short: "Restore an agent from a snapshot",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			agentName := args[0]
			cfg, err := loadConfig(cmd)
			if err != nil {
				return err
			}

			agentDef, err := snapshot.ResolveAgent(cfg, agentName)
			if err != nil {
				return err
			}

			workdir, err := configtoml.ExpandPath(agentDef.Workdir)
			if err != nil {
				return fmt.Errorf("expand workdir: %w", err)
			}

			snapPath := snapshotPath
			if snapPath == "" {
				latest, err := findLatestSnapshot(agentName)
				if err != nil {
					return err
				}
				snapPath = latest
			}

			ctx := context.Background()

			if agentDef.PostRestoreCmd != "" {
				opts := restore.RestoreOpts{
					PostRestoreCmd: agentDef.PostRestoreCmd,
					HookTimeout:    parseHookTimeout(agentDef.StopTimeout),
				}
				if err := restore.RestoreWithOpts(ctx, snapPath, workdir, opts); err != nil {
					fmt.Fprintln(cmd.ErrOrStderr(), err)
					return err
				}
			} else {
				if err := restore.Restore(ctx, snapPath, workdir); err != nil {
					fmt.Fprintln(cmd.ErrOrStderr(), err)
					return err
				}
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Restored %s from %s\n", agentName, filepath.Base(snapPath))
			return nil
		},
	}

	cmd.Flags().StringVar(&snapshotPath, "snapshot", "", "specific snapshot path (default: latest)")

	return cmd
}

func newListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list [agent]",
		Short: "List snapshots",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			agentFilter := ""
			if len(args) > 0 {
				agentFilter = args[0]
			}

			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("get home dir: %w", err)
			}
			snapRoot := filepath.Join(home, ".mesh", "snapshots")

			var agentDirs []string
			if agentFilter != "" {
				agentDirs = []string{filepath.Join(snapRoot, agentFilter)}
			} else {
				entries, err := os.ReadDir(snapRoot)
				if err != nil {
					if os.IsNotExist(err) {
						return nil
					}
					return fmt.Errorf("read snapshots dir: %w", err)
				}
				for _, e := range entries {
					if e.IsDir() {
						agentDirs = append(agentDirs, filepath.Join(snapRoot, e.Name()))
					}
				}
			}

			for _, dir := range agentDirs {
				entries, err := os.ReadDir(dir)
				if err != nil {
					if os.IsNotExist(err) {
						continue
					}
					return fmt.Errorf("read dir %q: %w", dir, err)
				}

				var snapEntries []string
				for _, e := range entries {
					if !e.IsDir() && strings.HasSuffix(e.Name(), ".tar.zst") {
						snapEntries = append(snapEntries, e.Name())
					}
				}
				sort.Strings(snapEntries)

				agentName := filepath.Base(dir)
				for _, s := range snapEntries {
					snapPath := filepath.Join(dir, s)
					ts, _ := parseTimestampFromFilename(s, agentName)

					info, infoErr := os.Stat(snapPath)
					sizeStr := "?"
					if infoErr == nil {
						sizeStr = humanSize(info.Size())
					}

					manifestPath := manifest.ManifestPath(snapPath)
					machineStr := ""
					m, readErr := manifest.Read(manifestPath)
					if readErr == nil && m.SourceMachine != "" {
						machineStr = " from " + m.SourceMachine
					}

					if ts != "" {
						fmt.Fprintf(cmd.OutOrStdout(), "%s  %s%s  %s\n", snapPath, sizeStr, machineStr, ts)
					} else {
						fmt.Fprintf(cmd.OutOrStdout(), "%s  %s%s\n", snapPath, sizeStr, machineStr)
					}
				}
			}

			return nil
		},
	}
}

func newInspectCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "inspect <snapshot>",
		Short: "Show snapshot manifest details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			snapPath := args[0]

			if _, err := os.Stat(snapPath); err != nil {
				if os.IsNotExist(err) {
					home, homeErr := os.UserHomeDir()
					if homeErr != nil {
						return fmt.Errorf("snapshot %q not found", snapPath)
					}
					cacheDir := filepath.Join(home, ".mesh", "snapshots", snapPath)
					latest, findErr := findLatestInDir(cacheDir, snapPath)
					if findErr != nil {
						return fmt.Errorf("snapshot %q not found", snapPath)
					}
					snapPath = latest
				} else {
					return fmt.Errorf("stat snapshot: %w", err)
				}
			}

			manifestPath := manifest.ManifestPath(snapPath)
			m, err := manifest.Read(manifestPath)
			if err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Agent: %s\n", m.AgentName)
			fmt.Fprintf(cmd.OutOrStdout(), "Timestamp: %s\n", m.Timestamp.Format(time.RFC3339))
			fmt.Fprintf(cmd.OutOrStdout(), "Source machine: %s\n", m.SourceMachine)
			fmt.Fprintf(cmd.OutOrStdout(), "Source workdir: %s\n", m.SourceWorkdir)
			fmt.Fprintf(cmd.OutOrStdout(), "Start cmd: %s\n", m.StartCmd)
			fmt.Fprintf(cmd.OutOrStdout(), "Stop timeout: %s\n", m.StopTimeout)
			fmt.Fprintf(cmd.OutOrStdout(), "Checksum: %s\n", m.Checksum)
			fmt.Fprintf(cmd.OutOrStdout(), "Size: %s\n", humanSize(m.Size))
			return nil
		},
	}
}

func newPruneCmd() *cobra.Command {
	var keep int

	cmd := &cobra.Command{
		Use:   "prune <agent>",
		Short: "Remove old snapshots, keeping the most recent",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			agentName := args[0]

			cacheDir, err := snapshot.SnapshotCacheDir(agentName)
			if err != nil {
				return err
			}

			entries, err := os.ReadDir(cacheDir)
			if err != nil {
				if os.IsNotExist(err) {
					fmt.Fprintf(cmd.OutOrStdout(), "No snapshots for agent %s\n", agentName)
					return nil
				}
				return fmt.Errorf("read snapshot dir: %w", err)
			}

			var snaps []string
			for _, e := range entries {
				if !e.IsDir() && strings.HasPrefix(e.Name(), agentName+"-") && strings.HasSuffix(e.Name(), ".tar.zst") {
					snaps = append(snaps, e.Name())
				}
			}

			sort.Strings(snaps)

			if len(snaps) <= keep {
				fmt.Fprintf(cmd.OutOrStdout(), "Only %d snapshots, nothing to prune (keep=%d)\n", len(snaps), keep)
				return nil
			}

			toDelete := snaps[:len(snaps)-keep]
			for _, name := range toDelete {
				tarPath := filepath.Join(cacheDir, name)
				shaPath := tarPath + ".sha256"
				jsonPath := manifest.ManifestPath(tarPath)
				os.Remove(tarPath)
				os.Remove(shaPath)
				os.Remove(jsonPath)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Pruned %d snapshot(s) for %s (kept %d)\n", len(toDelete), agentName, keep)
			return nil
		},
	}

	cmd.Flags().IntVar(&keep, "keep", 5, "number of snapshots to keep")

	return cmd
}

func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Initialize Mesh configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("get home dir: %w", err)
			}
			meshDir := filepath.Join(home, ".mesh")
			if err := os.MkdirAll(meshDir, 0755); err != nil {
				return fmt.Errorf("create mesh dir: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Mesh initialized. Run 'mesh serve' to start.\n")
			return nil
		},
	}
}

func newServeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Start the Mesh daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			configPath, _ := cmd.Flags().GetString("config")
			if configPath == "" {
				configPath = config.DefaultPath()
			}
			cfg, err := config.Load(configPath)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			d, err := daemon.New(cfg)
			if err != nil {
				return fmt.Errorf("create daemon: %w", err)
			}

			if err := d.Start(cmd.Context()); err != nil {
				if strings.Contains(err.Error(), "already running") {
					fmt.Fprintln(cmd.ErrOrStderr(), "Error: daemon is already running")
					return fmt.Errorf("daemon already running")
				}
				return err
			}
			return nil
		},
	}
}

func newStopCmd() *cobra.Command {
	var timeout time.Duration

	cmd := &cobra.Command{
		Use:   "stop",
		Short: "Stop the Mesh daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			configPath, _ := cmd.Flags().GetString("config")
			if configPath == "" {
				configPath = config.DefaultPath()
			}
			cfg, err := config.Load(configPath)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			if cfg.Daemon.PIDFile == "" {
				return fmt.Errorf("no pid_file configured")
			}

			data, err := os.ReadFile(cfg.Daemon.PIDFile)
			if err != nil {
				if os.IsNotExist(err) {
					fmt.Fprintln(cmd.ErrOrStderr(), "Error: daemon is not running (no PID file)")
					return fmt.Errorf("daemon not running")
				}
				return fmt.Errorf("read PID file: %w", err)
			}

			pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
			if err != nil {
				return fmt.Errorf("invalid PID file: %w", err)
			}

			proc, err := os.FindProcess(pid)
			if err != nil {
				fmt.Fprintln(cmd.ErrOrStderr(), "Error: daemon is not running (process not found)")
				return fmt.Errorf("daemon not running")
			}

			if err := proc.Signal(syscall.SIGTERM); err != nil {
				fmt.Fprintln(cmd.ErrOrStderr(), "Error: daemon is not running (cannot signal)")
				return fmt.Errorf("daemon not running")
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Stopping mesh daemon (pid %d)...\n", pid)

			deadline := time.Now().Add(timeout)
			for time.Now().Before(deadline) {
				if err := proc.Signal(syscall.Signal(0)); err != nil {
					fmt.Fprintln(cmd.OutOrStdout(), "Stopped mesh daemon")
					return nil
				}
				time.Sleep(100 * time.Millisecond)
			}

			fmt.Fprintln(cmd.ErrOrStderr(), "Warning: daemon did not stop within timeout, sending SIGKILL")
			proc.Signal(syscall.SIGKILL)
			return nil
		},
	}

	cmd.Flags().DurationVar(&timeout, "timeout", 30*time.Second, "timeout to wait for daemon to stop")

	return cmd
}

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show Mesh daemon status",
		RunE: func(cmd *cobra.Command, args []string) error {
			configPath, _ := cmd.Flags().GetString("config")
			if configPath == "" {
				configPath = config.DefaultPath()
			}
			cfg, err := config.Load(configPath)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			if cfg.Daemon.PIDFile == "" {
				fmt.Fprintln(cmd.OutOrStdout(), "Mesh daemon: stopped (no pid_file configured)")
				return nil
			}

			data, err := os.ReadFile(cfg.Daemon.PIDFile)
			if err != nil {
				if os.IsNotExist(err) {
					fmt.Fprintln(cmd.OutOrStdout(), "Mesh daemon: stopped")
					return nil
				}
				return fmt.Errorf("read PID file: %w", err)
			}

			pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
			if err != nil {
				fmt.Fprintln(cmd.OutOrStdout(), "Mesh daemon: stopped (invalid PID file)")
				return nil
			}

			proc, err := os.FindProcess(pid)
			if err != nil {
				fmt.Fprintln(cmd.OutOrStdout(), "Mesh daemon: stopped")
				return nil
			}

			if err := proc.Signal(syscall.Signal(0)); err != nil {
				fmt.Fprintln(cmd.OutOrStdout(), "Mesh daemon: stopped")
				return nil
			}

			// Process is alive — try to query health endpoint
			healthAddr := cfg.Daemon.SocketPath
			if healthAddr == "" {
				healthAddr = "/tmp/mesh.sock"
			}

			// The daemon's health server listens on a TCP port, not the socket path.
			// We need to discover it. Since we can't easily know the port, we'll just
			// report the PID and basic status.
			fmt.Fprintf(cmd.OutOrStdout(), "Mesh daemon: running (pid %d)\n", pid)

			// Try to query health endpoint via HTTP on localhost with common ports
			// The daemon binds to 127.0.0.1:0 (random port), so we can't know it from
			// config alone. We skip the health query for now and just show PID.
			// In a real implementation, the daemon could write its HTTP addr to a file.

			return nil
		},
	}
}

// parseHookTimeout parses a duration string, falling back to 30s.
func parseHookTimeout(s string) time.Duration {
	if d, err := time.ParseDuration(s); err == nil && d > 0 {
		return d
	}
	return 30 * time.Second
}

// parseTimestampFromFilename extracts the timestamp from a snapshot filename.
// Filename format: {agentName}-{YYYYMMDD-HHMMSS}.tar.zst
func parseTimestampFromFilename(filename, agentName string) (string, error) {
	prefix := agentName + "-"
	if !strings.HasPrefix(filename, prefix) {
		return "", fmt.Errorf("filename %q missing prefix %q", filename, prefix)
	}
	trimmed := strings.TrimPrefix(filename, prefix)
	trimmed = strings.TrimSuffix(trimmed, ".tar.zst")

	parts := strings.SplitN(trimmed, "-", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("filename %q has unexpected format", filename)
	}
	datePart := parts[0]
	timePart := parts[1]

	if len(datePart) != 8 || len(timePart) != 6 {
		return "", fmt.Errorf("filename %q has unexpected timestamp format", filename)
	}

	t, err := time.Parse("20060102-150405", datePart+"-"+timePart)
	if err != nil {
		return "", err
	}
	return t.Format("2006-01-02 15:04:05"), nil
}

func humanSize(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)
	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.1f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.1f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.1f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

func findLatestInDir(cacheDir, agentName string) (string, error) {
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		return "", err
	}

	prefix := agentName + "-"
	var snaps []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasPrefix(e.Name(), prefix) && strings.HasSuffix(e.Name(), ".tar.zst") {
			snaps = append(snaps, e.Name())
		}
	}

	if len(snaps) == 0 {
		return "", fmt.Errorf("no snapshots found for agent %q", agentName)
	}

	sort.Strings(snaps)
	return filepath.Join(cacheDir, snaps[len(snaps)-1]), nil
}
