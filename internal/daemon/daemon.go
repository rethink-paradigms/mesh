// Package daemon implements the long-running Mesh daemon process with signal
// handling, PID file management, and graceful shutdown orchestration.
package daemon

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/rethink-paradigms/mesh/internal/config"
)

// Daemon is the long-running mesh process that manages bodies and exposes MCP tools.
type Daemon struct {
	cfg    *config.Config
	sigs   chan os.Signal
	done   chan struct{}
	pidPath string
}

// New creates a new Daemon from the given config.
func New(cfg *config.Config) (*Daemon, error) {
	d := &Daemon{
		cfg:     cfg,
		sigs:    make(chan os.Signal, 1),
		done:    make(chan struct{}),
		pidPath: cfg.Daemon.PIDFile,
	}
	return d, nil
}

// Start begins the daemon's main loop. It blocks until a stop signal is received.
func (d *Daemon) Start(ctx context.Context) error {
	signal.Notify(d.sigs, syscall.SIGTERM, syscall.SIGINT)
	if err := d.writePIDFile(); err != nil {
		return fmt.Errorf("daemon: write PID file: %w", err)
	}
	defer d.removePIDFile()

	select {
	case <-d.sigs:
	case <-ctx.Done():
	}

	return nil
}

// Stop initiates graceful shutdown of the daemon.
func (d *Daemon) Stop(ctx context.Context) error {
	return nil
}

func (d *Daemon) writePIDFile() error {
	if d.pidPath == "" {
		return nil
	}
	return os.WriteFile(d.pidPath, []byte(fmt.Sprintf("%d", os.Getpid())), 0644)
}

func (d *Daemon) removePIDFile() {
	if d.pidPath != "" {
		os.Remove(d.pidPath)
	}
}
