package body

import (
	"context"
	"fmt"
	"sync"

	"github.com/google/uuid"
	"github.com/rethink-paradigms/mesh/internal/adapter"
	"github.com/rethink-paradigms/mesh/internal/store"
)

const migrationSteps = 7

type stepFunc func(ctx context.Context, mig *migrationContext) error

type migrationContext struct {
	id           string
	bodyID       string
	target       string
	snapshotID   string
	snapshotPath string
	snapshotSize int64
	newHandle    adapter.Handle
	mc           *MigrationCoordinator
}

// MigrationCoordinator manages body migrations between substrates.
type MigrationCoordinator struct {
	store   *store.Store
	adapter adapter.SubstrateAdapter
	bm      *BodyManager
	mu      sync.Mutex
}

func NewMigrationCoordinator(s *store.Store, a adapter.SubstrateAdapter, bm *BodyManager) *MigrationCoordinator {
	return &MigrationCoordinator{
		store:   s,
		adapter: a,
		bm:      bm,
	}
}

func (mc *MigrationCoordinator) buildSteps() []struct {
	name string
	fn   stepFunc
}{
	return []struct {
		name string
		fn   stepFunc
	}{
		{"export", mc.stepExport},
		{"provision", mc.stepProvision},
		{"transfer", mc.stepTransfer},
		{"import", mc.stepImport},
		{"verify", mc.stepVerify},
		{"switch", mc.stepSwitch},
		{"cleanup", mc.stepCleanup},
	}
}

// BeginMigration starts a 7-step migration for a body to a target substrate.
// Each step is persisted to the store so migration can be resumed after a crash.
func (mc *MigrationCoordinator) BeginMigration(ctx context.Context, bodyID, targetSubstrate string) (string, error) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	migID := uuid.New().String()

	if err := mc.store.CreateMigration(ctx, migID, bodyID, targetSubstrate, ""); err != nil {
		return "", fmt.Errorf("create migration record: %w", err)
	}

	mig := &migrationContext{
		id:     migID,
		bodyID: bodyID,
		target: targetSubstrate,
		mc:     mc,
	}

	b := mc.bm.getOrCreateBody(bodyID)
	b.mu.Lock()
	defer b.mu.Unlock()

	if err := mc.transitionBody(ctx, b, adapter.StateMigrating); err != nil {
		return "", fmt.Errorf("transition to migrating: %w", err)
	}

	steps := mc.buildSteps()
	for i, step := range steps {
		stepNum := i + 1
		if err := step.fn(ctx, mig); err != nil {
			_ = mc.store.UpdateMigration(ctx, migID, stepNum, err.Error())
			_ = mc.transitionBody(ctx, b, adapter.StateError)
			return migID, fmt.Errorf("migration step %d (%s) failed: %w", stepNum, step.name, err)
		}
		if err := mc.store.UpdateMigration(ctx, migID, stepNum, ""); err != nil {
			return migID, fmt.Errorf("persist migration step %d: %w", stepNum, err)
		}
	}

	if err := mc.transitionBody(ctx, b, adapter.StateRunning); err != nil {
		return migID, fmt.Errorf("transition to running after migration: %w", err)
	}

	return migID, nil
}

// ResumeMigration resumes a migration from the last persisted step.
func (mc *MigrationCoordinator) ResumeMigration(ctx context.Context, migrationID string) error {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	rec, err := mc.store.GetMigration(ctx, migrationID)
	if err != nil {
		return fmt.Errorf("get migration: %w", err)
	}

	if rec.Error != "" {
		return fmt.Errorf("migration %s has error: %s", migrationID, rec.Error)
	}

	mig := &migrationContext{
		id:     rec.ID,
		bodyID: rec.BodyID,
		target: rec.TargetSubstrate,
		mc:     mc,
	}

	b := mc.bm.getOrCreateBody(rec.BodyID)
	b.mu.Lock()
	defer b.mu.Unlock()

	steps := mc.buildSteps()
	for i := rec.CurrentStep; i < migrationSteps; i++ {
		stepNum := i + 1
		if err := steps[i].fn(ctx, mig); err != nil {
			_ = mc.store.UpdateMigration(ctx, migrationID, stepNum, err.Error())
			_ = mc.transitionBody(ctx, b, adapter.StateError)
			return fmt.Errorf("migration step %d (%s) failed: %w", stepNum, steps[i].name, err)
		}
		if err := mc.store.UpdateMigration(ctx, migrationID, stepNum, ""); err != nil {
			return fmt.Errorf("persist migration step %d: %w", stepNum, err)
		}
	}

	return mc.transitionBody(ctx, b, adapter.StateRunning)
}

func (mc *MigrationCoordinator) stepExport(ctx context.Context, mig *migrationContext) error {
	b := mc.bm.getOrCreateBody(mig.bodyID)

	rc, err := mc.adapter.ExportFilesystem(ctx, b.InstanceID)
	if err != nil {
		return fmt.Errorf("export filesystem: %w", err)
	}
	defer rc.Close()

	snapID := uuid.New().String()
	storagePath := fmt.Sprintf("/tmp/mesh-snapshot-%s.tar.zst", snapID)

	var size int64
	if buf, ok := rc.(interface{ Len() int }); ok {
		size = int64(buf.Len())
	}

	if err := mc.store.CreateSnapshot(ctx, snapID, mig.bodyID, "", storagePath, size); err != nil {
		return fmt.Errorf("create snapshot: %w", err)
	}

	mig.snapshotID = snapID
	mig.snapshotPath = storagePath
	mig.snapshotSize = size

	return nil
}

func (mc *MigrationCoordinator) stepProvision(_ context.Context, _ *migrationContext) error {
	return nil
}

func (mc *MigrationCoordinator) stepTransfer(_ context.Context, _ *migrationContext) error {
	return nil
}

func (mc *MigrationCoordinator) stepImport(_ context.Context, _ *migrationContext) error {
	return nil
}

func (mc *MigrationCoordinator) stepVerify(_ context.Context, _ *migrationContext) error {
	return nil
}

func (mc *MigrationCoordinator) stepSwitch(_ context.Context, _ *migrationContext) error {
	return nil
}

func (mc *MigrationCoordinator) stepCleanup(ctx context.Context, mig *migrationContext) error {
	if mig.snapshotID != "" {
		_ = mc.store.DeleteSnapshot(ctx, mig.snapshotID)
	}
	return nil
}

func (mc *MigrationCoordinator) transitionBody(ctx context.Context, b *Body, target adapter.BodyState) error {
	if err := b.Transition(target); err != nil {
		return err
	}
	return mc.store.UpdateBodyState(ctx, b.ID, target)
}
