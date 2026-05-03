package body

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rethink-paradigms/mesh/internal/adapter"
	"github.com/rethink-paradigms/mesh/internal/orchestrator"
	"github.com/rethink-paradigms/mesh/internal/provisioner"
	"github.com/rethink-paradigms/mesh/internal/store"
)

// Registry defines the interface for snapshot storage operations used during
// cross-machine migrations. Implementations include S3 registry plugins.
type Registry interface {
	// Push uploads a snapshot to the registry with the given key.
	// The sha256 parameter is the expected checksum of the content.
	Push(ctx context.Context, key string, r io.Reader, size int64, sha256 string) error
	// Pull downloads a snapshot from the registry by key.
	// Returns the content reader and the SHA-256 checksum stored with the object.
	Pull(ctx context.Context, key string) (io.ReadCloser, string, error)
	// Verify checks that the object at key matches the expected SHA-256.
	Verify(ctx context.Context, key, expectedSHA256 string) error
}

const migrationSteps = 7

type stepFunc func(ctx context.Context, mig *migrationContext) error

type migrationContext struct {
	id           string
	bodyID       string
	target       string
	snapshotID   string
	snapshotPath string
	snapshotSize int64
	snapshotSHA  string
	newHandle    adapter.Handle
	mc           *MigrationCoordinator
}

// MigrationCoordinator manages body migrations between substrates.
type MigrationCoordinator struct {
	store        *store.Store
	orchRegistry *orchestrator.Registry
	provRegistry *provisioner.Registry
	bm           *BodyManager
	registry     Registry
	mu           sync.Mutex
}

// NewMigrationCoordinator creates a migration coordinator.
// If registry is non-nil, cross-machine migrations will push/pull snapshots via S3.
func NewMigrationCoordinator(s *store.Store, bm *BodyManager, orchRegistry *orchestrator.Registry, provRegistry *provisioner.Registry, registry Registry) *MigrationCoordinator {
	return &MigrationCoordinator{
		store:        s,
		orchRegistry: orchRegistry,
		provRegistry: provRegistry,
		bm:           bm,
		registry:     registry,
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
		if stepNum == migrationSteps {
			continue
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
// If the migration record is missing, it assumes the migration completed successfully.
func (mc *MigrationCoordinator) ResumeMigration(ctx context.Context, migrationID string) error {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	rec, err := mc.store.GetMigration(ctx, migrationID)
	if err != nil {
		// Migration record missing — assume already completed and cleaned up.
		return nil
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

	bodyRec, err := mc.store.GetBody(ctx, rec.BodyID)
	if err == nil && bodyRec.InstanceID != "" {
		mig.newHandle = adapter.Handle(bodyRec.InstanceID)
	}

	b := mc.bm.getOrCreateBody(rec.BodyID)
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.State == adapter.StateError {
		if err := mc.transitionBody(ctx, b, adapter.StateMigrating); err != nil {
			return fmt.Errorf("transition from error to migrating: %w", err)
		}
	}

	steps := mc.buildSteps()
	for i := rec.CurrentStep; i < migrationSteps; i++ {
		stepNum := i + 1
		if err := steps[i].fn(ctx, mig); err != nil {
			_ = mc.store.UpdateMigration(ctx, migrationID, stepNum, err.Error())
			_ = mc.transitionBody(ctx, b, adapter.StateError)
			return fmt.Errorf("migration step %d (%s) failed: %w", stepNum, steps[i].name, err)
		}
		if stepNum == migrationSteps {
			continue
		}
		if err := mc.store.UpdateMigration(ctx, migrationID, stepNum, ""); err != nil {
			return fmt.Errorf("persist migration step %d: %w", stepNum, err)
		}
	}

	return mc.transitionBody(ctx, b, adapter.StateRunning)
}

func (mc *MigrationCoordinator) stepExport(ctx context.Context, mig *migrationContext) error {
	b := mc.bm.getOrCreateBody(mig.bodyID)

	srcOrch, err := mc.orchRegistry.Open(b.Substrate)
	if err != nil {
		return fmt.Errorf("open source orchestrator for %q: %w", b.Substrate, err)
	}

	exporter, ok := srcOrch.(orchestrator.Exporter)
	if !ok {
		return fmt.Errorf("source orchestrator %q does not support ExportFilesystem", b.Substrate)
	}

	rc, err := exporter.ExportFilesystem(ctx, orchestrator.Handle(b.InstanceID))
	if err != nil {
		return fmt.Errorf("export filesystem: %w", err)
	}
	defer rc.Close()

	snapID := uuid.New().String()
	storagePath := fmt.Sprintf("/tmp/mesh-snapshot-%s.tar.zst", snapID)

	out, err := os.Create(storagePath)
	if err != nil {
		return fmt.Errorf("create snapshot file: %w", err)
	}

	hasher := sha256.New()
	tee := io.TeeReader(rc, hasher)

	size, copyErr := io.Copy(out, tee)
	closeErr := out.Close()
	if copyErr != nil {
		os.Remove(storagePath)
		return fmt.Errorf("write snapshot file: %w", copyErr)
	}
	if closeErr != nil {
		os.Remove(storagePath)
		return fmt.Errorf("close snapshot file: %w", closeErr)
	}

	if err := mc.store.CreateSnapshot(ctx, snapID, mig.bodyID, "", storagePath, size); err != nil {
		os.Remove(storagePath)
		return fmt.Errorf("create snapshot: %w", err)
	}

	mig.snapshotID = snapID
	mig.snapshotPath = storagePath
	mig.snapshotSize = size
	mig.snapshotSHA = hex.EncodeToString(hasher.Sum(nil))

	return nil
}

func (mc *MigrationCoordinator) stepProvision(ctx context.Context, mig *migrationContext) error {
	if mig.newHandle != "" {
		return nil
	}

	rec, err := mc.store.GetMigration(ctx, mig.id)
	if err != nil {
		return fmt.Errorf("get migration record: %w", err)
	}
	if rec.CurrentStep >= 2 {
		bodyRec, err := mc.store.GetBody(ctx, mig.bodyID)
		if err == nil && bodyRec.InstanceID != "" {
			mig.newHandle = adapter.Handle(bodyRec.InstanceID)
			return nil
		}
	}

	b := mc.bm.getOrCreateBody(mig.bodyID)

	// Same-substrate: use source orchestrator for provisioning
	if b.Substrate == mig.target {
		srcOrch, err := mc.orchRegistry.Open(b.Substrate)
		if err != nil {
			return fmt.Errorf("open source orchestrator for %q: %w", b.Substrate, err)
		}

		inspector, ok := srcOrch.(orchestrator.Inspector)
		if !ok {
			return fmt.Errorf("source orchestrator %q does not support Inspect", b.Substrate)
		}

		srcMeta, err := inspector.Inspect(ctx, orchestrator.Handle(b.InstanceID))
		if err != nil {
			return fmt.Errorf("inspect source container: %w", err)
		}

		spec := orchestrator.BodySpec{
			Image:   srcMeta.Image,
			Workdir: srcMeta.Workdir,
			Env:     srcMeta.Env,
			Cmd:     srcMeta.Cmd,
		}

		targetHandle, err := srcOrch.ScheduleBody(ctx, spec)
		if err != nil {
			return fmt.Errorf("create target container: %w", err)
		}

		mig.newHandle = adapter.Handle(targetHandle)

		if err := mc.store.UpdateBodyState(ctx, mig.bodyID, adapter.StateMigrating); err != nil {
			_ = srcOrch.DestroyBody(ctx, targetHandle)
			mig.newHandle = ""
			return fmt.Errorf("persist target handle: %w", err)
		}

		return nil
	}

	// Cross-substrate: look up provisioner for target
	prov, err := mc.provRegistry.Open(mig.target)
	if err != nil {
		available := mc.provRegistry.List()
		return fmt.Errorf("no provisioner for substrate %q (available: %s)", mig.target, strings.Join(available, ", "))
	}

	// Provision a machine on the target substrate
	machineSpec := provisioner.MachineSpec{
		Image:    "mesh-agent",
		MemoryMB: 512,
		CPUShares: 256,
		Region:   "default",
	}
	_, err = prov.CreateMachine(ctx, machineSpec, "")
	if err != nil {
		return fmt.Errorf("provision target machine on %q: %w", mig.target, err)
	}

	// After provisioning, schedule body on target orchestrator
	targetOrch, err := mc.orchRegistry.Open(mig.target)
	if err != nil {
		return fmt.Errorf("open target orchestrator for %q: %w", mig.target, err)
	}

	srcOrch, err := mc.orchRegistry.Open(b.Substrate)
	if err != nil {
		return fmt.Errorf("open source orchestrator for %q: %w", b.Substrate, err)
	}

	inspector, ok := srcOrch.(orchestrator.Inspector)
	if !ok {
		return fmt.Errorf("source orchestrator %q does not support Inspect", b.Substrate)
	}

	srcMeta, err := inspector.Inspect(ctx, orchestrator.Handle(b.InstanceID))
	if err != nil {
		return fmt.Errorf("inspect source container: %w", err)
	}

	spec := orchestrator.BodySpec{
		Image:   srcMeta.Image,
		Workdir: srcMeta.Workdir,
		Env:     srcMeta.Env,
		Cmd:     srcMeta.Cmd,
	}

	targetHandle, err := targetOrch.ScheduleBody(ctx, spec)
	if err != nil {
		return fmt.Errorf("schedule body on target orchestrator %q: %w", mig.target, err)
	}

	mig.newHandle = adapter.Handle(targetHandle)

	if err := mc.store.UpdateBodyState(ctx, mig.bodyID, adapter.StateMigrating); err != nil {
		_ = targetOrch.DestroyBody(ctx, targetHandle)
		mig.newHandle = ""
		return fmt.Errorf("persist target handle: %w", err)
	}

	return nil
}

func (mc *MigrationCoordinator) stepTransfer(ctx context.Context, mig *migrationContext) error {
	rec, err := mc.store.GetMigration(ctx, mig.id)
	if err != nil {
		return fmt.Errorf("get migration record: %w", err)
	}
	if rec.CurrentStep >= 3 {
		return nil
	}

	if mig.snapshotPath == "" {
		return fmt.Errorf("snapshot path not set (step 1 not completed?)")
	}
	if mig.newHandle == "" {
		return fmt.Errorf("target handle not set (step 2 not completed?)")
	}

	b := mc.bm.getOrCreateBody(mig.bodyID)
	isCrossMachine := mc.registry != nil && b.Substrate != mig.target
	if isCrossMachine {
		return mc.transferCrossMachine(ctx, mig)
	}
	return mc.transferSameMachine(ctx, mig)
}

func (mc *MigrationCoordinator) transferSameMachine(ctx context.Context, mig *migrationContext) error {
	f, err := os.Open(mig.snapshotPath)
	if err != nil {
		return fmt.Errorf("open snapshot %s: %w", mig.snapshotPath, err)
	}
	defer f.Close()

	targetOrch, err := mc.orchRegistry.Open(mig.target)
	if err != nil {
		return fmt.Errorf("open target orchestrator for %q: %w", mig.target, err)
	}

	importer, ok := targetOrch.(orchestrator.Importer)
	if !ok {
		return fmt.Errorf("target orchestrator %q does not support ImportFilesystem", mig.target)
	}

	if err := importer.ImportFilesystem(ctx, orchestrator.Handle(mig.newHandle), f); err != nil {
		return fmt.Errorf("import filesystem to target: %w", err)
	}

	return nil
}

func (mc *MigrationCoordinator) transferCrossMachine(ctx context.Context, mig *migrationContext) error {
	key := "mesh-snapshot-" + mig.snapshotID

	pushErr := withRetry(ctx, 3, func() error {
		f, err := os.Open(mig.snapshotPath)
		if err != nil {
			return fmt.Errorf("open snapshot for push: %w", err)
		}
		defer f.Close()
		return mc.registry.Push(ctx, key, f, mig.snapshotSize, mig.snapshotSHA)
	})
	if pushErr != nil {
		return fmt.Errorf("push snapshot to registry: %w", pushErr)
	}

	var pulled io.ReadCloser
	var pulledSHA string
	pullErr := withRetry(ctx, 3, func() error {
		var err error
		pulled, pulledSHA, err = mc.registry.Pull(ctx, key)
		return err
	})
	if pullErr != nil {
		return fmt.Errorf("pull snapshot from registry: %w", pullErr)
	}
	defer pulled.Close()

	if pulledSHA != "" && pulledSHA != mig.snapshotSHA {
		return fmt.Errorf("sha256 mismatch after cross-machine transfer: source=%s pulled=%s", mig.snapshotSHA, pulledSHA)
	}

	targetOrch, err := mc.orchRegistry.Open(mig.target)
	if err != nil {
		return fmt.Errorf("open target orchestrator for %q: %w", mig.target, err)
	}

	importer, ok := targetOrch.(orchestrator.Importer)
	if !ok {
		return fmt.Errorf("target orchestrator %q does not support ImportFilesystem", mig.target)
	}

	if err := importer.ImportFilesystem(ctx, orchestrator.Handle(mig.newHandle), pulled); err != nil {
		return fmt.Errorf("import filesystem to target: %w", err)
	}

	return nil
}

func withRetry(ctx context.Context, maxRetries int, fn func() error) error {
	var lastErr error
	for i := 0; i <= maxRetries; i++ {
		if err := fn(); err == nil {
			return nil
		} else {
			lastErr = err
		}
		if i < maxRetries {
			delay := time.Duration(1<<i) * time.Second
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}
	}
	return lastErr
}

func (mc *MigrationCoordinator) stepImport(ctx context.Context, mig *migrationContext) error {
	rec, err := mc.store.GetMigration(ctx, mig.id)
	if err != nil {
		return fmt.Errorf("get migration record: %w", err)
	}
	if rec.CurrentStep >= 4 {
		return nil
	}

	if mig.newHandle == "" {
		return fmt.Errorf("target handle not set (step 2 not completed?)")
	}

	return nil
}

func (mc *MigrationCoordinator) stepVerify(ctx context.Context, mig *migrationContext) error {
	rec, err := mc.store.GetMigration(ctx, mig.id)
	if err != nil {
		return fmt.Errorf("get migration record: %w", err)
	}
	if rec.CurrentStep >= 5 {
		return nil
	}

	if mig.newHandle == "" {
		return fmt.Errorf("target handle not set (step 2 not completed?)")
	}

	targetOrch, err := mc.orchRegistry.Open(mig.target)
	if err != nil {
		return fmt.Errorf("open target orchestrator for %q: %w", mig.target, err)
	}

	status, err := targetOrch.GetBodyStatus(ctx, orchestrator.Handle(mig.newHandle))
	if err != nil {
		return fmt.Errorf("health check (GetBodyStatus) failed: %w", err)
	}
	if status.State != orchestrator.StateRunning && status.State != orchestrator.StateCreated {
		return fmt.Errorf("target container unhealthy: state=%s", status.State)
	}

	executor, ok := targetOrch.(orchestrator.Executor)
	if !ok {
		return fmt.Errorf("target orchestrator %q does not support Exec", mig.target)
	}

	execRes, err := executor.Exec(ctx, orchestrator.Handle(mig.newHandle), []string{"echo", "ok"})
	if err != nil {
		return fmt.Errorf("health check (Exec) failed: %w", err)
	}
	if execRes.ExitCode != 0 {
		return fmt.Errorf("health check command failed: exit code %d, stderr=%s", execRes.ExitCode, execRes.Stderr)
	}

	lsRes, err := executor.Exec(ctx, orchestrator.Handle(mig.newHandle), []string{"ls", "/"})
	if err != nil {
		return fmt.Errorf("filesystem check (ls) failed: %w", err)
	}
	if lsRes.ExitCode != 0 {
		return fmt.Errorf("filesystem check command failed: exit code %d, stderr=%s", lsRes.ExitCode, lsRes.Stderr)
	}
	if lsRes.Stdout == "" {
		return fmt.Errorf("filesystem check: root directory appears empty")
	}

	return nil
}

func (mc *MigrationCoordinator) stepSwitch(ctx context.Context, mig *migrationContext) error {
	rec, err := mc.store.GetMigration(ctx, mig.id)
	if err != nil {
		return fmt.Errorf("get migration record: %w", err)
	}
	if rec.CurrentStep >= 6 {
		return nil
	}

	b := mc.bm.getOrCreateBody(mig.bodyID)
	srcHandle := b.InstanceID

	if mig.newHandle == "" {
		return fmt.Errorf("target handle not set (step 2 not completed?)")
	}

	targetOrch, err := mc.orchRegistry.Open(mig.target)
	if err != nil {
		return fmt.Errorf("open target orchestrator for %q: %w", mig.target, err)
	}

	status, err := targetOrch.GetBodyStatus(ctx, orchestrator.Handle(mig.newHandle))
	if err != nil {
		return fmt.Errorf("verify target health: %w", err)
	}
	if status.State != orchestrator.StateRunning && status.State != orchestrator.StateCreated {
		return fmt.Errorf("target container unhealthy: state=%s", status.State)
	}

	if srcHandle != "" {
		srcOrch, err := mc.orchRegistry.Open(b.Substrate)
		if err != nil {
			return fmt.Errorf("open source orchestrator for %q: %w", b.Substrate, err)
		}
		if err := srcOrch.StopBody(ctx, orchestrator.Handle(srcHandle)); err != nil {
			return fmt.Errorf("stop source container: %w", err)
		}
	}

	if err := mc.store.UpdateBodyInstanceID(ctx, mig.bodyID, string(mig.newHandle)); err != nil {
		if srcHandle != "" {
			srcOrch, _ := mc.orchRegistry.Open(b.Substrate)
			if srcOrch != nil {
				_ = srcOrch.StartBody(ctx, orchestrator.Handle(srcHandle))
			}
		}
		return fmt.Errorf("update body instance_id: %w", err)
	}

	b.InstanceID = mig.newHandle
	b.Substrate = mig.target

	if srcHandle != "" {
		srcOrch, _ := mc.orchRegistry.Open(b.Substrate)
		if srcOrch != nil {
			_ = srcOrch.DestroyBody(ctx, orchestrator.Handle(srcHandle))
		}
	}

	return nil
}

func (mc *MigrationCoordinator) stepCleanup(ctx context.Context, mig *migrationContext) error {
	rec, err := mc.store.GetMigration(ctx, mig.id)
	if err != nil {
		return fmt.Errorf("get migration record: %w", err)
	}
	if rec.CurrentStep >= 7 {
		return nil
	}

	if mig.snapshotPath != "" {
		_ = os.Remove(mig.snapshotPath)
	}

	if mig.snapshotID != "" {
		_ = mc.store.DeleteSnapshot(ctx, mig.snapshotID)
	}

	if err := mc.store.DeleteMigration(ctx, mig.id); err != nil {
		return fmt.Errorf("delete migration record: %w", err)
	}

	return nil
}

func (mc *MigrationCoordinator) transitionBody(ctx context.Context, b *Body, target adapter.BodyState) error {
	if err := b.Transition(target); err != nil {
		return err
	}
	return mc.store.UpdateBodyState(ctx, b.ID, target)
}
