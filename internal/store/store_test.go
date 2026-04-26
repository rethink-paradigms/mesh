package store

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"

	"github.com/rethink-paradigms/mesh/internal/adapter"
)

func tempStore(t *testing.T) *Store {
	t.Helper()
	f, err := os.CreateTemp("", "mesh-store-*.db")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	path := f.Name()
	f.Close()
	s, err := Open(path)
	if err != nil {
		os.Remove(path)
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() {
		s.Close()
		os.Remove(path)
	})
	return s
}

func TestBodyCRUD(t *testing.T) {
	s := tempStore(t)
	ctx := context.Background()

	// Create
	err := s.CreateBody(ctx, "b1", "test-body", adapter.StateCreated, `{"image":"alpine"}`, "docker", "inst-1")
	if err != nil {
		t.Fatalf("CreateBody: %v", err)
	}

	// Get
	b, err := s.GetBody(ctx, "b1")
	if err != nil {
		t.Fatalf("GetBody: %v", err)
	}
	if b.ID != "b1" {
		t.Errorf("ID = %q, want %q", b.ID, "b1")
	}
	if b.Name != "test-body" {
		t.Errorf("Name = %q, want %q", b.Name, "test-body")
	}
	if b.State != adapter.StateCreated {
		t.Errorf("State = %q, want %q", b.State, adapter.StateCreated)
	}
	if b.SpecJSON != `{"image":"alpine"}` {
		t.Errorf("SpecJSON = %q, want %q", b.SpecJSON, `{"image":"alpine"}`)
	}
	if b.Substrate != "docker" {
		t.Errorf("Substrate = %q, want %q", b.Substrate, "docker")
	}
	if b.InstanceID != "inst-1" {
		t.Errorf("InstanceID = %q, want %q", b.InstanceID, "inst-1")
	}
	if b.CreatedAt == "" {
		t.Error("CreatedAt is empty")
	}
	if b.UpdatedAt == "" {
		t.Error("UpdatedAt is empty")
	}

	// Update state
	err = s.UpdateBodyState(ctx, "b1", adapter.StateRunning)
	if err != nil {
		t.Fatalf("UpdateBodyState: %v", err)
	}

	// Verify state changed
	b2, err := s.GetBody(ctx, "b1")
	if err != nil {
		t.Fatalf("GetBody after update: %v", err)
	}
	if b2.State != adapter.StateRunning {
		t.Errorf("State after update = %q, want %q", b2.State, adapter.StateRunning)
	}

	// List
	bodies, err := s.ListBodies(ctx)
	if err != nil {
		t.Fatalf("ListBodies: %v", err)
	}
	if len(bodies) != 1 {
		t.Fatalf("ListBodies count = %d, want 1", len(bodies))
	}

	// Delete
	err = s.DeleteBody(ctx, "b1")
	if err != nil {
		t.Fatalf("DeleteBody: %v", err)
	}

	// Verify deleted
	_, err = s.GetBody(ctx, "b1")
	if err == nil {
		t.Fatal("GetBody after delete should error")
	}
}

func TestBodyStateTransitions(t *testing.T) {
	s := tempStore(t)
	ctx := context.Background()

	err := s.CreateBody(ctx, "b-states", "state-tester", adapter.StateCreated, "", "local", "")
	if err != nil {
		t.Fatalf("CreateBody: %v", err)
	}

	transitions := []adapter.BodyState{
		adapter.StateStarting,
		adapter.StateRunning,
		adapter.StateStopping,
		adapter.StateStopped,
		adapter.StateDestroyed,
	}

	for _, state := range transitions {
		err := s.UpdateBodyState(ctx, "b-states", state)
		if err != nil {
			t.Fatalf("UpdateBodyState to %s: %v", state, err)
		}

		b, err := s.GetBody(ctx, "b-states")
		if err != nil {
			t.Fatalf("GetBody after transition to %s: %v", state, err)
		}
		if b.State != state {
			t.Errorf("State = %q, want %q", b.State, state)
		}
	}
}

func TestSnapshotCRUD(t *testing.T) {
	s := tempStore(t)
	ctx := context.Background()

	err := s.CreateBody(ctx, "b-snap", "snap-body", adapter.StateRunning, "", "docker", "i1")
	if err != nil {
		t.Fatalf("CreateBody: %v", err)
	}

	// Create snapshot
	err = s.CreateSnapshot(ctx, "snap1", "b-snap", `{"files":[]}`, "/tmp/snap1.tar.zst", 1024)
	if err != nil {
		t.Fatalf("CreateSnapshot: %v", err)
	}

	// List snapshots
	snaps, err := s.ListSnapshots(ctx, "b-snap")
	if err != nil {
		t.Fatalf("ListSnapshots: %v", err)
	}
	if len(snaps) != 1 {
		t.Fatalf("ListSnapshots count = %d, want 1", len(snaps))
	}
	if snaps[0].ID != "snap1" {
		t.Errorf("snap ID = %q, want %q", snaps[0].ID, "snap1")
	}
	if snaps[0].BodyID != "b-snap" {
		t.Errorf("snap BodyID = %q, want %q", snaps[0].BodyID, "b-snap")
	}
	if snaps[0].SizeBytes != 1024 {
		t.Errorf("snap SizeBytes = %d, want 1024", snaps[0].SizeBytes)
	}

	// Get snapshot
	snap, err := s.GetSnapshot(ctx, "snap1")
	if err != nil {
		t.Fatalf("GetSnapshot: %v", err)
	}
	if snap.StoragePath != "/tmp/snap1.tar.zst" {
		t.Errorf("StoragePath = %q, want %q", snap.StoragePath, "/tmp/snap1.tar.zst")
	}

	// Delete snapshot
	err = s.DeleteSnapshot(ctx, "snap1")
	if err != nil {
		t.Fatalf("DeleteSnapshot: %v", err)
	}

	// Verify deleted
	snaps, err = s.ListSnapshots(ctx, "b-snap")
	if err != nil {
		t.Fatalf("ListSnapshots after delete: %v", err)
	}
	if len(snaps) != 0 {
		t.Fatalf("ListSnapshots count after delete = %d, want 0", len(snaps))
	}
}

func TestMigrationCRUD(t *testing.T) {
	s := tempStore(t)
	ctx := context.Background()

	err := s.CreateBody(ctx, "b-mgr", "mgr-body", adapter.StateRunning, "", "docker", "i2")
	if err != nil {
		t.Fatalf("CreateBody: %v", err)
	}

	// Create migration
	err = s.CreateMigration(ctx, "mgr1", "b-mgr", "nomad", "snap-x")
	if err != nil {
		t.Fatalf("CreateMigration: %v", err)
	}

	// Get migration
	mgr, err := s.GetMigration(ctx, "mgr1")
	if err != nil {
		t.Fatalf("GetMigration: %v", err)
	}
	if mgr.ID != "mgr1" {
		t.Errorf("ID = %q, want %q", mgr.ID, "mgr1")
	}
	if mgr.BodyID != "b-mgr" {
		t.Errorf("BodyID = %q, want %q", mgr.BodyID, "b-mgr")
	}
	if mgr.TargetSubstrate != "nomad" {
		t.Errorf("TargetSubstrate = %q, want %q", mgr.TargetSubstrate, "nomad")
	}
	if mgr.CurrentStep != 0 {
		t.Errorf("CurrentStep = %d, want 0", mgr.CurrentStep)
	}
	if mgr.SnapshotID != "snap-x" {
		t.Errorf("SnapshotID = %q, want %q", mgr.SnapshotID, "snap-x")
	}
	if mgr.Error != "" {
		t.Errorf("Error = %q, want empty", mgr.Error)
	}

	// Update migration
	err = s.UpdateMigration(ctx, "mgr1", 2, "partial failure")
	if err != nil {
		t.Fatalf("UpdateMigration: %v", err)
	}

	mgr2, err := s.GetMigration(ctx, "mgr1")
	if err != nil {
		t.Fatalf("GetMigration after update: %v", err)
	}
	if mgr2.CurrentStep != 2 {
		t.Errorf("CurrentStep = %d, want 2", mgr2.CurrentStep)
	}
	if mgr2.Error != "partial failure" {
		t.Errorf("Error = %q, want %q", mgr2.Error, "partial failure")
	}
}

func TestWALMode(t *testing.T) {
	s := tempStore(t)

	var mode string
	err := s.db.QueryRow("PRAGMA journal_mode").Scan(&mode)
	if err != nil {
		t.Fatalf("query journal_mode: %v", err)
	}
	if mode != "wal" {
		t.Errorf("journal_mode = %q, want %q", mode, "wal")
	}
}

func TestConcurrentAccess(t *testing.T) {
	s := tempStore(t)
	ctx := context.Background()

	var wg sync.WaitGroup
	const n = 10
	errc := make(chan error, n)

	for i := range n {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			id := fmt.Sprintf("concurrent-%d", idx)
			name := fmt.Sprintf("body-%d", idx)
			err := s.CreateBody(ctx, id, name, adapter.StateCreated, "", "local", "")
			errc <- err
		}(i)
	}
	wg.Wait()
	close(errc)

	for err := range errc {
		if err != nil {
			t.Fatalf("concurrent CreateBody: %v", err)
		}
	}

	bodies, err := s.ListBodies(ctx)
	if err != nil {
		t.Fatalf("ListBodies: %v", err)
	}
	if len(bodies) != n {
		t.Errorf("ListBodies count = %d, want %d", len(bodies), n)
	}
}

func TestPerBodyMutex(t *testing.T) {
	s := tempStore(t)
	ctx := context.Background()

	err := s.CreateBody(ctx, "b-mutex", "mutex-body", adapter.StateCreated, "", "local", "")
	if err != nil {
		t.Fatalf("CreateBody: %v", err)
	}

	var wg sync.WaitGroup
	const n = 50

	states := []adapter.BodyState{
		adapter.StateStarting,
		adapter.StateRunning,
		adapter.StateStopping,
		adapter.StateStopped,
	}

	for i := range n {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			state := states[idx%len(states)]
			_ = s.UpdateBodyState(ctx, "b-mutex", state)
		}(i)
	}
	wg.Wait()

	b, err := s.GetBody(ctx, "b-mutex")
	if err != nil {
		t.Fatalf("GetBody after concurrent updates: %v", err)
	}
	found := false
	for _, st := range states {
		if b.State == st {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("final state %q is not one of the expected states", b.State)
	}
}

func TestForeignKeyEnforcement(t *testing.T) {
	s := tempStore(t)
	ctx := context.Background()

	err := s.CreateSnapshot(ctx, "snap-fk", "nonexistent-body", "{}", "/tmp/x", 0)
	if err == nil {
		t.Fatal("CreateSnapshot with non-existent body_id should fail with foreign key error")
	}
}

func TestConfigCRUD(t *testing.T) {
	s := tempStore(t)
	ctx := context.Background()

	// Get non-existent key
	_, err := s.GetConfig(ctx, "nope")
	if err == nil {
		t.Fatal("GetConfig for missing key should error")
	}

	// Set and get
	err = s.SetConfig(ctx, "test_key", "test_value")
	if err != nil {
		t.Fatalf("SetConfig: %v", err)
	}

	val, err := s.GetConfig(ctx, "test_key")
	if err != nil {
		t.Fatalf("GetConfig: %v", err)
	}
	if val != "test_value" {
		t.Errorf("value = %q, want %q", val, "test_value")
	}

	// Upsert
	err = s.SetConfig(ctx, "test_key", "updated")
	if err != nil {
		t.Fatalf("SetConfig upsert: %v", err)
	}
	val, err = s.GetConfig(ctx, "test_key")
	if err != nil {
		t.Fatalf("GetConfig after upsert: %v", err)
	}
	if val != "updated" {
		t.Errorf("value after upsert = %q, want %q", val, "updated")
	}
}
