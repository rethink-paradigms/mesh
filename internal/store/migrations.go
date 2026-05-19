package store

import (
	"context"
	"database/sql"
	"fmt"
)

// --- Migration CRUD ---

// CreateMigration inserts a new migration record without a cluster_id (legacy compat).
func (s *Store) CreateMigration(ctx context.Context, id, bodyID, targetSubstrate, snapshotID string) error {
	return s.CreateMigrationWithCluster(ctx, id, bodyID, targetSubstrate, snapshotID, "")
}

// CreateMigrationWithCluster inserts a new migration record with a cluster_id.
func (s *Store) CreateMigrationWithCluster(ctx context.Context, id, bodyID, targetSubstrate, snapshotID, clusterID string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO migrations (id, body_id, target_substrate, current_step, snapshot_id, cluster_id, started_at)
		 VALUES (?, ?, ?, 0, ?, ?, ?)`,
		id, bodyID, targetSubstrate, snapshotID, clusterID, now(),
	)
	if err != nil {
		return fmt.Errorf("create migration %s: %w", id, err)
	}
	return nil
}

func (s *Store) UpdateMigration(ctx context.Context, id string, currentStep int, errStr string) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE migrations SET current_step = ?, error = ? WHERE id = ?`,
		currentStep, errStr, id,
	)
	if err != nil {
		return fmt.Errorf("update migration %s: %w", id, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("migration %s: not found", id)
	}
	return nil
}

func (s *Store) UpdateMigrationByCluster(ctx context.Context, id string, currentStep int, errStr, clusterID string) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE migrations SET current_step = ?, error = ? WHERE id = ? AND cluster_id = ?`,
		currentStep, errStr, id, clusterID,
	)
	if err != nil {
		return fmt.Errorf("update migration %s by cluster %s: %w", id, clusterID, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("migration %s: not found", id)
	}
	return nil
}

func (s *Store) GetMigration(ctx context.Context, id string) (*MigrationRecord, error) {
	var m MigrationRecord
	var snapID, clusterID, errStr sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT id, body_id, target_substrate, current_step, snapshot_id, cluster_id, started_at, error
		 FROM migrations WHERE id = ?`, id,
	).Scan(&m.ID, &m.BodyID, &m.TargetSubstrate, &m.CurrentStep, &snapID, &clusterID, &m.StartedAt, &errStr)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("migration %s: not found", id)
	}
	if err != nil {
		return nil, fmt.Errorf("get migration %s: %w", id, err)
	}
	m.SnapshotID = snapID.String
	m.ClusterID = clusterID.String
	m.Error = errStr.String
	return &m, nil
}

func (s *Store) GetMigrationByCluster(ctx context.Context, id, clusterID string) (*MigrationRecord, error) {
	var m MigrationRecord
	var snapID, cid, errStr sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT id, body_id, target_substrate, current_step, snapshot_id, cluster_id, started_at, error
		 FROM migrations WHERE id = ? AND (cluster_id = ? OR cluster_id IS NULL)`, id, clusterID,
	).Scan(&m.ID, &m.BodyID, &m.TargetSubstrate, &m.CurrentStep, &snapID, &cid, &m.StartedAt, &errStr)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("migration %s: not found", id)
	}
	if err != nil {
		return nil, fmt.Errorf("get migration %s by cluster %s: %w", id, clusterID, err)
	}
	m.SnapshotID = snapID.String
	m.ClusterID = cid.String
	m.Error = errStr.String
	return &m, nil
}

// DeleteMigration deletes a migration record by id.
func (s *Store) DeleteMigration(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM migrations WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete migration %s: %w", id, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("migration %s: not found", id)
	}
	return nil
}

func (s *Store) DeleteMigrationByCluster(ctx context.Context, id, clusterID string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM migrations WHERE id = ? AND cluster_id = ?`, id, clusterID)
	if err != nil {
		return fmt.Errorf("delete migration %s by cluster %s: %w", id, clusterID, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("migration %s: not found", id)
	}
	return nil
}
