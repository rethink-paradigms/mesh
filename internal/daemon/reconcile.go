package daemon

import (
	"context"
	"fmt"
	"log/slog"

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
			}

		case orchestrator.StateError:
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
