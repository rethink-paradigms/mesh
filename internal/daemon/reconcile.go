package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/rethink-paradigms/mesh/internal/orchestrator"
)

func (d *Daemon) reconcile(ctx context.Context) error {
	bodies, err := d.store.ListBodies(ctx)
	if err != nil {
		if ctx.Err() == context.Canceled {
			return nil
		}
		return fmt.Errorf("reconcile: list bodies: %w", err)
	}

	for _, rec := range bodies {
		if rec.InstanceID == "" {
			continue
		}

		adp, err := d.orchRegistry.Open(rec.Substrate)
		// Fallback: older bodies were stored with substrate "local"
		// which maps to the docker adapter (the only local orchestrator).
		if err != nil && rec.Substrate == "local" {
			adp, err = d.orchRegistry.Open("docker")
		}
		if err != nil {
			switch rec.State {
			case orchestrator.StateRunning, orchestrator.StateStarting, orchestrator.StateStopping:
				slog.Warn("reconcile: body substrate not found, transitioning to Error", "body_id", rec.ID, "substrate", rec.Substrate)
				if transErr := d.bodyMgr.TransitionBody(ctx, rec.ID, orchestrator.StateError); transErr != nil {
					slog.Warn("reconcile: failed to transition body to Error", "body_id", rec.ID, "error", transErr)
				}
				d.mu.Lock()
				d.reconcileSteps++
				d.mu.Unlock()
			default:
				slog.Warn("reconcile: body substrate not found, skipping", "body_id", rec.ID, "substrate", rec.Substrate)
			}
			continue
		}

		_, err = adp.GetBodyStatus(ctx, orchestrator.Handle(rec.InstanceID))
		containerExists := err == nil

		switch rec.State {
		case orchestrator.StateRunning, orchestrator.StateStarting, orchestrator.StateStopping:
			if !containerExists {
				if transErr := d.bodyMgr.TransitionBody(ctx, rec.ID, orchestrator.StateError); transErr != nil {
					slog.Warn("reconcile: failed to transition body to Error", "body_id", rec.ID, "error", transErr)
				} else {
					slog.Warn("reconcile: body container not found, transitioned to Error", "body_id", rec.ID)
				}
				d.mu.Lock()
				d.reconcileSteps++
				d.mu.Unlock()
			} else {
				// Container exists — sync Docker state back to store
				status, err := adp.GetBodyStatus(ctx, orchestrator.Handle(rec.InstanceID))
				if err == nil && status.State != rec.State {
					// Map Docker exited → Exited (clean) or Error (crash) depending on exit code
					target := status.State
					if target == orchestrator.StateStopped {
						if status.ExitCode == 0 {
							target = orchestrator.StateExited
						} else {
							target = orchestrator.StateError
						}
					}
					slog.Info("reconcile: container state changed",
						"body_id", rec.ID,
						"stored_state", rec.State,
						"docker_state", status.State,
						"exit_code", status.ExitCode,
						"new_state", target)
					if transErr := d.bodyMgr.TransitionBody(ctx, rec.ID, target); transErr != nil {
						slog.Warn("reconcile: failed to transition body", "body_id", rec.ID, "error", transErr)
					}
					d.mu.Lock()
					d.reconcileSteps++
					d.mu.Unlock()
				}
			}

		case orchestrator.StateError:
			// GC: destroy Error bodies older than 1 hour
			createdAt, parseErr := time.Parse(time.RFC3339, rec.CreatedAt)
			if parseErr == nil && time.Since(createdAt) > 1*time.Hour {
				slog.Info("reconcile: garbage collecting stale error body", "body_id", rec.ID, "name", rec.Name)
				if err := d.bodyMgr.Destroy(ctx, rec.ID); err != nil {
					slog.Warn("reconcile: failed to destroy stale body", "body_id", rec.ID, "error", err)
				}
				d.mu.Lock()
				d.reconcileSteps++
				d.mu.Unlock()
			}
			if containerExists {
				status, _ := adp.GetBodyStatus(ctx, orchestrator.Handle(rec.InstanceID))
				if status.State == orchestrator.StateRunning {
					if transErr := d.bodyMgr.TransitionBody(ctx, rec.ID, orchestrator.StateRunning); transErr != nil {
						slog.Warn("reconcile: failed to transition body to Running", "body_id", rec.ID, "error", transErr)
					} else {
						slog.Warn("reconcile: body verified running, transitioned to Running", "body_id", rec.ID)
					}
					d.mu.Lock()
					d.reconcileSteps++
					d.mu.Unlock()
				}
			}

		case orchestrator.StateMigrating:
			if !d.hasActiveMigration(ctx, rec.ID) {
				if transErr := d.bodyMgr.TransitionBody(ctx, rec.ID, orchestrator.StateError); transErr != nil {
					slog.Warn("reconcile: failed to transition body from Migrating to Error", "body_id", rec.ID, "error", transErr)
				} else {
					slog.Warn("reconcile: body migration record missing, transitioned to Error", "body_id", rec.ID)
				}
				d.mu.Lock()
				d.reconcileSteps++
				d.mu.Unlock()
			}
		}
	}

	return nil
}

func (d *Daemon) hasActiveMigration(ctx context.Context, bodyID string) bool {
	var count int
	err := d.store.QueryRow(ctx, `SELECT COUNT(*) FROM migrations WHERE body_id = ? AND error = ''`, bodyID).Scan(&count)
	if err != nil {
		return false
	}
	return count > 0
}

// reconcileLoop runs periodic reconciliation in the background.
// It syncs Docker container state back to the daemon store every 30 seconds.
func (d *Daemon) reconcileLoop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := d.reconcile(ctx); err != nil {
				slog.Warn("background reconcile failed", "error", err)
			}
		case <-d.done:
			return
		}
	}
}
