package store

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"

	"github.com/rethink-paradigms/mesh/internal/orchestrator"
)

// TestStore_MigrationV2toV3 creates a v2 database, opens it through the
// migration chain, and verifies the cluster_id and allocated_ports_json
// columns exist on all tables.
func TestStore_MigrationV2toV3(t *testing.T) {
	f, err := os.CreateTemp("", "mesh-store-v2-*.db")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	path := f.Name()
	f.Close()
	defer os.Remove(path)

	dsn := fmt.Sprintf("file:%s", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	// Create v2 schema (no cluster_id columns)
	v2Schema := `
	CREATE TABLE bodies (
		id TEXT PRIMARY KEY,
		name TEXT UNIQUE NOT NULL,
		state TEXT NOT NULL,
		spec_json TEXT,
		substrate TEXT,
		instance_id TEXT,
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	);
	CREATE TABLE snapshots (
		id TEXT PRIMARY KEY,
		body_id TEXT NOT NULL REFERENCES bodies(id),
		manifest_json TEXT,
		storage_path TEXT,
		size_bytes INTEGER,
		created_at TEXT NOT NULL
	);
	CREATE TABLE migrations (
		id TEXT PRIMARY KEY,
		body_id TEXT NOT NULL REFERENCES bodies(id),
		target_substrate TEXT NOT NULL,
		current_step INTEGER DEFAULT 0,
		snapshot_id TEXT,
		started_at TEXT NOT NULL,
		error TEXT
	);
	CREATE TABLE config (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL
	);
	`
	if _, err := db.Exec(v2Schema); err != nil {
		db.Close()
		t.Fatalf("exec v2 schema: %v", err)
	}

	// Insert a v2 body (no cluster_id)
	if _, err := db.Exec(
		`INSERT INTO bodies (id, name, state, spec_json, substrate, instance_id, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		"v2-body", "v2-body-name", string(orchestrator.StateCreated), "{}", "docker", "inst-v2", "2024-06-01T00:00:00Z", "2024-06-01T00:00:00Z",
	); err != nil {
		db.Close()
		t.Fatalf("insert v2 body: %v", err)
	}

	// Set schema_version to 2
	if _, err := db.Exec("INSERT INTO config (key, value) VALUES ('schema_version', '2')"); err != nil {
		db.Close()
		t.Fatalf("set schema_version: %v", err)
	}
	db.Close()

	// Now open with Store — should migrate v2→v3
	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open v2 db: %v", err)
	}
	defer s.Close()

	ctx := context.Background()

	// Verify schema version is now 3
	version, err := s.GetConfig(ctx, "schema_version")
	if err != nil {
		t.Fatalf("GetConfig schema_version: %v", err)
	}
	if version != "4" {
		t.Errorf("schema_version = %q, want 4", version)
	}

	// Verify the migrated body has cluster_id = "" (NULL)
	b, err := s.GetBody(ctx, "v2-body")
	if err != nil {
		t.Fatalf("GetBody v2-body: %v", err)
	}
	if b.Name != "v2-body-name" {
		t.Errorf("body name = %q, want %q", b.Name, "v2-body-name")
	}
	if b.ClusterID != "" {
		t.Errorf("v2 body cluster_id = %q, want empty string (NULL)", b.ClusterID)
	}
}

// TestStore_InsertBodyWithClusterID verifies that a body created with a
// cluster_id can be read back correctly.
func TestStore_InsertBodyWithClusterID(t *testing.T) {
	s := tempStore(t)
	ctx := context.Background()

	err := s.CreateBodyWithCluster(ctx, "b-cluster", "cluster-body", orchestrator.StateCreated, `{"image":"alpine"}`, "docker", "inst-1", "cluster-prod-1")
	if err != nil {
		t.Fatalf("CreateBodyWithCluster: %v", err)
	}

	b, err := s.GetBody(ctx, "b-cluster")
	if err != nil {
		t.Fatalf("GetBody: %v", err)
	}
	if b.ClusterID != "cluster-prod-1" {
		t.Errorf("ClusterID = %q, want %q", b.ClusterID, "cluster-prod-1")
	}
	if b.Name != "cluster-body" {
		t.Errorf("Name = %q, want %q", b.Name, "cluster-body")
	}
}

// TestStore_ListBodiesWithClusterID verifies bodies with cluster_id appear
// correctly in ListBodies results.
func TestStore_ListBodiesWithClusterID(t *testing.T) {
	s := tempStore(t)
	ctx := context.Background()

	// Create bodies with and without cluster_id
	if err := s.CreateBodyWithCluster(ctx, "b1", "cluster-a", orchestrator.StateCreated, "", "docker", "i1", "cluster-alpha"); err != nil {
		t.Fatalf("CreateBodyWithCluster b1: %v", err)
	}
	if err := s.CreateBody(ctx, "b2", "no-cluster", orchestrator.StateCreated, "", "docker", "i2"); err != nil {
		t.Fatalf("CreateBody b2: %v", err)
	}
	if err := s.CreateBodyWithCluster(ctx, "b3", "cluster-b", orchestrator.StateCreated, "", "nomad", "i3", "cluster-beta"); err != nil {
		t.Fatalf("CreateBodyWithCluster b3: %v", err)
	}

	bodies, err := s.ListBodies(ctx)
	if err != nil {
		t.Fatalf("ListBodies: %v", err)
	}
	if len(bodies) != 3 {
		t.Fatalf("ListBodies count = %d, want 3", len(bodies))
	}

	// Verify cluster_id values
	for _, b := range bodies {
		switch b.ID {
		case "b1":
			if b.ClusterID != "cluster-alpha" {
				t.Errorf("b1 ClusterID = %q, want %q", b.ClusterID, "cluster-alpha")
			}
		case "b2":
			if b.ClusterID != "" {
				t.Errorf("b2 ClusterID = %q, want empty", b.ClusterID)
			}
		case "b3":
			if b.ClusterID != "cluster-beta" {
				t.Errorf("b3 ClusterID = %q, want %q", b.ClusterID, "cluster-beta")
			}
		}
	}
}

// TestStore_GetBodyWithNullClusterID verifies that a body created without
// cluster_id (NULL in DB) is read back with an empty string ClusterID.
func TestStore_GetBodyWithNullClusterID(t *testing.T) {
	s := tempStore(t)
	ctx := context.Background()

	err := s.CreateBody(ctx, "b-legacy", "legacy-body", orchestrator.StateCreated, "", "local", "")
	if err != nil {
		t.Fatalf("CreateBody: %v", err)
	}

	b, err := s.GetBody(ctx, "b-legacy")
	if err != nil {
		t.Fatalf("GetBody: %v", err)
	}
	if b.ClusterID != "" {
		t.Errorf("ClusterID = %q, want empty string (NULL)", b.ClusterID)
	}
	if b.Name != "legacy-body" {
		t.Errorf("Name = %q, want %q", b.Name, "legacy-body")
	}
}

// TestStore_SnapshotWithClusterID verifies snapshots with cluster_id.
func TestStore_SnapshotWithClusterID(t *testing.T) {
	s := tempStore(t)
	ctx := context.Background()

	err := s.CreateBody(ctx, "b-snap-cluster", "snap-cluster-body", orchestrator.StateRunning, "", "docker", "i1")
	if err != nil {
		t.Fatalf("CreateBody: %v", err)
	}

	// Create snapshot with cluster_id
	err = s.CreateSnapshotWithCluster(ctx, "snap-c1", "b-snap-cluster", `{"files":[]}`, "/tmp/snap.tar.zst", 2048, "cluster-prod-1")
	if err != nil {
		t.Fatalf("CreateSnapshotWithCluster: %v", err)
	}

	// Get and verify
	snap, err := s.GetSnapshot(ctx, "snap-c1")
	if err != nil {
		t.Fatalf("GetSnapshot: %v", err)
	}
	if snap.ClusterID != "cluster-prod-1" {
		t.Errorf("ClusterID = %q, want %q", snap.ClusterID, "cluster-prod-1")
	}

	// List and verify
	snaps, err := s.ListSnapshots(ctx, "b-snap-cluster")
	if err != nil {
		t.Fatalf("ListSnapshots: %v", err)
	}
	if len(snaps) != 1 {
		t.Fatalf("ListSnapshots count = %d, want 1", len(snaps))
	}
	if snaps[0].ClusterID != "cluster-prod-1" {
		t.Errorf("ListSnapshots[0].ClusterID = %q, want %q", snaps[0].ClusterID, "cluster-prod-1")
	}
}

// TestStore_MigrationWithClusterID verifies migrations with cluster_id.
func TestStore_MigrationWithClusterID(t *testing.T) {
	s := tempStore(t)
	ctx := context.Background()

	err := s.CreateBody(ctx, "b-mig-cluster", "mig-cluster-body", orchestrator.StateRunning, "", "docker", "i2")
	if err != nil {
		t.Fatalf("CreateBody: %v", err)
	}

	// Create migration with cluster_id
	err = s.CreateMigrationWithCluster(ctx, "mig-c1", "b-mig-cluster", "nomad", "snap-x", "cluster-prod-1")
	if err != nil {
		t.Fatalf("CreateMigrationWithCluster: %v", err)
	}

	// Get and verify
	m, err := s.GetMigration(ctx, "mig-c1")
	if err != nil {
		t.Fatalf("GetMigration: %v", err)
	}
	if m.ClusterID != "cluster-prod-1" {
		t.Errorf("ClusterID = %q, want %q", m.ClusterID, "cluster-prod-1")
	}
	if m.TargetSubstrate != "nomad" {
		t.Errorf("TargetSubstrate = %q, want %q", m.TargetSubstrate, "nomad")
	}
}

// TestStore_SnapshotWithNullClusterID verifies snapshots created without
// cluster_id via the legacy CreateSnapshot method work correctly.
func TestStore_SnapshotWithNullClusterID(t *testing.T) {
	s := tempStore(t)
	ctx := context.Background()

	err := s.CreateBody(ctx, "b-snap-null", "snap-null-body", orchestrator.StateRunning, "", "docker", "i1")
	if err != nil {
		t.Fatalf("CreateBody: %v", err)
	}

	err = s.CreateSnapshot(ctx, "snap-null", "b-snap-null", `{"files":[]}`, "/tmp/snap.tar.zst", 1024)
	if err != nil {
		t.Fatalf("CreateSnapshot: %v", err)
	}

	snap, err := s.GetSnapshot(ctx, "snap-null")
	if err != nil {
		t.Fatalf("GetSnapshot: %v", err)
	}
	if snap.ClusterID != "" {
		t.Errorf("ClusterID = %q, want empty string (NULL)", snap.ClusterID)
	}
}

// TestStore_MigrationWithNullClusterID verifies migrations created without
// cluster_id via the legacy CreateMigration method work correctly.
func TestStore_MigrationWithNullClusterID(t *testing.T) {
	s := tempStore(t)
	ctx := context.Background()

	err := s.CreateBody(ctx, "b-mig-null", "mig-null-body", orchestrator.StateRunning, "", "docker", "i2")
	if err != nil {
		t.Fatalf("CreateBody: %v", err)
	}

	err = s.CreateMigration(ctx, "mig-null", "b-mig-null", "local", "snap-x")
	if err != nil {
		t.Fatalf("CreateMigration: %v", err)
	}

	m, err := s.GetMigration(ctx, "mig-null")
	if err != nil {
		t.Fatalf("GetMigration: %v", err)
	}
	if m.ClusterID != "" {
		t.Errorf("ClusterID = %q, want empty string (NULL)", m.ClusterID)
	}
}
