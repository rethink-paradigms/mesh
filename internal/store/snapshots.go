package store

import (
	"context"
	"database/sql"
	"fmt"
)

// --- Snapshot CRUD ---

// CreateSnapshot inserts a new snapshot record without a cluster_id (legacy compat).
func (s *Store) CreateSnapshot(ctx context.Context, id, bodyID, manifestJSON, storagePath string, sizeBytes int64) error {
	return s.CreateSnapshotWithCluster(ctx, id, bodyID, manifestJSON, storagePath, sizeBytes, "")
}

// CreateSnapshotWithCluster inserts a new snapshot record with a cluster_id.
func (s *Store) CreateSnapshotWithCluster(ctx context.Context, id, bodyID, manifestJSON, storagePath string, sizeBytes int64, clusterID string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO snapshots (id, body_id, manifest_json, storage_path, size_bytes, cluster_id, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id, bodyID, manifestJSON, storagePath, sizeBytes, clusterID, now(),
	)
	if err != nil {
		return fmt.Errorf("create snapshot %s: %w", id, err)
	}
	return nil
}

func (s *Store) ListSnapshots(ctx context.Context, bodyID string) ([]*SnapshotRecord, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, body_id, manifest_json, storage_path, size_bytes, cluster_id, created_at
		 FROM snapshots WHERE body_id = ? ORDER BY created_at`, bodyID)
	if err != nil {
		return nil, fmt.Errorf("list snapshots for body %s: %w", bodyID, err)
	}
	defer rows.Close()

	var snaps []*SnapshotRecord
	for rows.Next() {
		var snap SnapshotRecord
		var clusterID sql.NullString
		if err := rows.Scan(&snap.ID, &snap.BodyID, &snap.ManifestJSON, &snap.StoragePath, &snap.SizeBytes, &clusterID, &snap.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan snapshot: %w", err)
		}
		snap.ClusterID = clusterID.String
		snaps = append(snaps, &snap)
	}
	return snaps, rows.Err()
}

func (s *Store) ListSnapshotsByCluster(ctx context.Context, bodyID, clusterID string) ([]*SnapshotRecord, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, body_id, manifest_json, storage_path, size_bytes, cluster_id, created_at
		 FROM snapshots WHERE body_id = ? AND (cluster_id = ? OR cluster_id IS NULL) ORDER BY created_at`, bodyID, clusterID)
	if err != nil {
		return nil, fmt.Errorf("list snapshots for body %s by cluster %s: %w", bodyID, clusterID, err)
	}
	defer rows.Close()

	var snaps []*SnapshotRecord
	for rows.Next() {
		var snap SnapshotRecord
		var cid sql.NullString
		if err := rows.Scan(&snap.ID, &snap.BodyID, &snap.ManifestJSON, &snap.StoragePath, &snap.SizeBytes, &cid, &snap.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan snapshot: %w", err)
		}
		snap.ClusterID = cid.String
		snaps = append(snaps, &snap)
	}
	return snaps, rows.Err()
}

func (s *Store) GetSnapshot(ctx context.Context, id string) (*SnapshotRecord, error) {
	var snap SnapshotRecord
	var clusterID sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT id, body_id, manifest_json, storage_path, size_bytes, cluster_id, created_at
		 FROM snapshots WHERE id = ?`, id,
	).Scan(&snap.ID, &snap.BodyID, &snap.ManifestJSON, &snap.StoragePath, &snap.SizeBytes, &clusterID, &snap.CreatedAt)
	snap.ClusterID = clusterID.String
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("snapshot %s: not found", id)
	}
	if err != nil {
		return nil, fmt.Errorf("get snapshot %s: %w", id, err)
	}
	return &snap, nil
}

func (s *Store) GetSnapshotByCluster(ctx context.Context, id, clusterID string) (*SnapshotRecord, error) {
	var snap SnapshotRecord
	var cid sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT id, body_id, manifest_json, storage_path, size_bytes, cluster_id, created_at
		 FROM snapshots WHERE id = ? AND (cluster_id = ? OR cluster_id IS NULL)`, id, clusterID,
	).Scan(&snap.ID, &snap.BodyID, &snap.ManifestJSON, &snap.StoragePath, &snap.SizeBytes, &cid, &snap.CreatedAt)
	snap.ClusterID = cid.String
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("snapshot %s: not found", id)
	}
	if err != nil {
		return nil, fmt.Errorf("get snapshot %s by cluster %s: %w", id, clusterID, err)
	}
	return &snap, nil
}

func (s *Store) DeleteSnapshot(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM snapshots WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete snapshot %s: %w", id, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("snapshot %s: not found", id)
	}
	return nil
}

func (s *Store) DeleteSnapshotByCluster(ctx context.Context, id, clusterID string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM snapshots WHERE id = ? AND cluster_id = ?`, id, clusterID)
	if err != nil {
		return fmt.Errorf("delete snapshot %s by cluster %s: %w", id, clusterID, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("snapshot %s: not found", id)
	}
	return nil
}
