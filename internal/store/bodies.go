package store

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/rethink-paradigms/mesh/internal/orchestrator"
)

// --- Body CRUD ---

// CreateBody inserts a new body record without a cluster_id (legacy compat).
func (s *Store) CreateBody(ctx context.Context, id, name string, state orchestrator.BodyState, specJSON, substrate, instanceID string) error {
	return s.CreateBodyWithCluster(ctx, id, name, state, specJSON, substrate, instanceID, "")
}

// CreateBodyWithCluster inserts a new body record with a cluster_id.
func (s *Store) CreateBodyWithCluster(ctx context.Context, id, name string, state orchestrator.BodyState, specJSON, substrate, instanceID, clusterID string) error {
	unlock := s.bodyLock(id)
	defer unlock.Unlock()

	ts := now()
	var cid any
	if clusterID != "" {
		cid = clusterID
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO bodies (id, name, state, spec_json, substrate, instance_id, cluster_id, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, name, string(state), specJSON, substrate, instanceID, cid, ts, ts,
	)
	if err != nil {
		return fmt.Errorf("create body %s: %w", id, err)
	}
	return nil
}

// GetBody retrieves a body record by id.
func (s *Store) GetBody(ctx context.Context, id string) (*BodyRecord, error) {
	var b BodyRecord
	var clusterID sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, state, spec_json, substrate, instance_id, cluster_id, allocated_ports_json, created_at, updated_at
		 FROM bodies WHERE id = ?`, id,
	).Scan(&b.ID, &b.Name, &b.State, &b.SpecJSON, &b.Substrate, &b.InstanceID, &clusterID, &b.AllocatedPortsJSON, &b.CreatedAt, &b.UpdatedAt)
	b.ClusterID = clusterID.String
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("body %s: not found", id)
	}
	if err != nil {
		return nil, fmt.Errorf("get body %s: %w", id, err)
	}
	return &b, nil
}

// ListBodiesBySubstrate returns all body records with the given substrate.
func (s *Store) ListBodiesBySubstrate(ctx context.Context, substrate string) ([]*BodyRecord, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, state, spec_json, substrate, instance_id, cluster_id, allocated_ports_json, created_at, updated_at
		 FROM bodies WHERE substrate = ? ORDER BY created_at`, substrate)
	if err != nil {
		return nil, fmt.Errorf("list bodies by substrate %s: %w", substrate, err)
	}
	defer rows.Close()

	var bodies []*BodyRecord
	for rows.Next() {
		var b BodyRecord
		var clusterID sql.NullString
		if err := rows.Scan(&b.ID, &b.Name, &b.State, &b.SpecJSON, &b.Substrate, &b.InstanceID, &clusterID, &b.AllocatedPortsJSON, &b.CreatedAt, &b.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan body: %w", err)
		}
		b.ClusterID = clusterID.String
		bodies = append(bodies, &b)
	}
	return bodies, rows.Err()
}

// ListBodies returns all body records.
func (s *Store) ListBodies(ctx context.Context) ([]*BodyRecord, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, state, spec_json, substrate, instance_id, cluster_id, allocated_ports_json, created_at, updated_at
		 FROM bodies ORDER BY created_at`)
	if err != nil {
		return nil, fmt.Errorf("list bodies: %w", err)
	}
	defer rows.Close()

	var bodies []*BodyRecord
	for rows.Next() {
		var b BodyRecord
		var clusterID sql.NullString
		if err := rows.Scan(&b.ID, &b.Name, &b.State, &b.SpecJSON, &b.Substrate, &b.InstanceID, &clusterID, &b.AllocatedPortsJSON, &b.CreatedAt, &b.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan body: %w", err)
		}
		b.ClusterID = clusterID.String
		bodies = append(bodies, &b)
	}
	return bodies, rows.Err()
}

func (s *Store) ListBodiesByCluster(ctx context.Context, clusterID string) ([]*BodyRecord, error) {
	if clusterID == "" {
		return s.ListBodies(ctx)
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, state, spec_json, substrate, instance_id, cluster_id, allocated_ports_json, created_at, updated_at
		 FROM bodies WHERE cluster_id = ? OR cluster_id IS NULL ORDER BY created_at`, clusterID)
	if err != nil {
		return nil, fmt.Errorf("list bodies by cluster %s: %w", clusterID, err)
	}
	defer rows.Close()

	var bodies []*BodyRecord
	for rows.Next() {
		var b BodyRecord
		var cid sql.NullString
		if err := rows.Scan(&b.ID, &b.Name, &b.State, &b.SpecJSON, &b.Substrate, &b.InstanceID, &cid, &b.AllocatedPortsJSON, &b.CreatedAt, &b.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan body: %w", err)
		}
		b.ClusterID = cid.String
		bodies = append(bodies, &b)
	}
	return bodies, rows.Err()
}

func (s *Store) GetBodyByCluster(ctx context.Context, id, clusterID string) (*BodyRecord, error) {
	if clusterID == "" {
		return s.GetBody(ctx, id)
	}
	var b BodyRecord
	var cid sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, state, spec_json, substrate, instance_id, cluster_id, allocated_ports_json, created_at, updated_at
		 FROM bodies WHERE id = ? AND (cluster_id = ? OR cluster_id IS NULL)`, id, clusterID,
	).Scan(&b.ID, &b.Name, &b.State, &b.SpecJSON, &b.Substrate, &b.InstanceID, &cid, &b.AllocatedPortsJSON, &b.CreatedAt, &b.UpdatedAt)
	b.ClusterID = cid.String
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("body %s: not found", id)
	}
	if err != nil {
		return nil, fmt.Errorf("get body %s by cluster %s: %w", id, clusterID, err)
	}
	return &b, nil
}

func (s *Store) UpdateBodyStateByCluster(ctx context.Context, id string, state orchestrator.BodyState, clusterID string) error {
	if clusterID == "" {
		return s.UpdateBodyState(ctx, id, state)
	}
	unlock := s.bodyLock(id)
	defer unlock.Unlock()

	res, err := s.db.ExecContext(ctx,
		`UPDATE bodies SET state = ?, updated_at = ? WHERE id = ? AND cluster_id = ?`,
		string(state), now(), id, clusterID,
	)
	if err != nil {
		return fmt.Errorf("update body state %s by cluster %s: %w", id, clusterID, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("body %s: not found", id)
	}
	return nil
}

func (s *Store) UpdateBodyInstanceIDByCluster(ctx context.Context, id, instanceID, clusterID string) error {
	if clusterID == "" {
		return s.UpdateBodyInstanceID(ctx, id, instanceID)
	}
	unlock := s.bodyLock(id)
	defer unlock.Unlock()

	res, err := s.db.ExecContext(ctx,
		`UPDATE bodies SET instance_id = ?, updated_at = ? WHERE id = ? AND cluster_id = ?`,
		instanceID, now(), id, clusterID,
	)
	if err != nil {
		return fmt.Errorf("update body instance_id %s by cluster %s: %w", id, clusterID, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("body %s: not found", id)
	}
	return nil
}

func (s *Store) UpdateBodySubstrateByCluster(ctx context.Context, id, substrate, clusterID string) error {
	if clusterID == "" {
		return s.UpdateBodySubstrate(ctx, id, substrate)
	}
	unlock := s.bodyLock(id)
	defer unlock.Unlock()

	res, err := s.db.ExecContext(ctx,
		`UPDATE bodies SET substrate = ?, updated_at = ? WHERE id = ? AND cluster_id = ?`,
		substrate, now(), id, clusterID,
	)
	if err != nil {
		return fmt.Errorf("update body substrate %s by cluster %s: %w", id, clusterID, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("body %s: not found", id)
	}
	return nil
}

func (s *Store) DeleteBodyByCluster(ctx context.Context, id, clusterID string) error {
	if clusterID == "" {
		return s.DeleteBody(ctx, id)
	}
	unlock := s.bodyLock(id)
	defer unlock.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.ExecContext(ctx, `DELETE FROM snapshots WHERE body_id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete snapshots for body %s: %w", id, err)
	}
	_, err = tx.ExecContext(ctx, `DELETE FROM migrations WHERE body_id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete migrations for body %s: %w", id, err)
	}
	res, err := tx.ExecContext(ctx, `DELETE FROM bodies WHERE id = ? AND cluster_id = ?`, id, clusterID)
	if err != nil {
		return fmt.Errorf("delete body %s by cluster %s: %w", id, clusterID, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("body %s: not found", id)
	}

	return tx.Commit()
}

// UpdateBodyState updates the state and updated_at timestamp of a body.
func (s *Store) UpdateBodyState(ctx context.Context, id string, state orchestrator.BodyState) error {
	unlock := s.bodyLock(id)
	defer unlock.Unlock()

	res, err := s.db.ExecContext(ctx,
		`UPDATE bodies SET state = ?, updated_at = ? WHERE id = ?`,
		string(state), now(), id,
	)
	if err != nil {
		return fmt.Errorf("update body state %s: %w", id, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("body %s: not found", id)
	}
	return nil
}

// UpdateBodyInstanceID updates the instance_id and updated_at timestamp of a body.
func (s *Store) UpdateBodyInstanceID(ctx context.Context, id, instanceID string) error {
	unlock := s.bodyLock(id)
	defer unlock.Unlock()

	res, err := s.db.ExecContext(ctx,
		`UPDATE bodies SET instance_id = ?, updated_at = ? WHERE id = ?`,
		instanceID, now(), id,
	)
	if err != nil {
		return fmt.Errorf("update body instance_id %s: %w", id, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("body %s: not found", id)
	}
	return nil
}

// UpdateBodySubstrate updates the substrate and updated_at timestamp of a body.
func (s *Store) UpdateBodySubstrate(ctx context.Context, id, substrate string) error {
	unlock := s.bodyLock(id)
	defer unlock.Unlock()

	res, err := s.db.ExecContext(ctx,
		`UPDATE bodies SET substrate = ?, updated_at = ? WHERE id = ?`,
		substrate, now(), id,
	)
	if err != nil {
		return fmt.Errorf("update body substrate %s: %w", id, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("body %s: not found", id)
	}
	return nil
}

// UpdateBodyAllocatedPorts persists the allocated ports JSON for a body.
// Used to warm the port pool on daemon restart.
func (s *Store) UpdateBodyAllocatedPorts(ctx context.Context, id, portsJSON string) error {
	unlock := s.bodyLock(id)
	defer unlock.Unlock()

	res, err := s.db.ExecContext(ctx,
		`UPDATE bodies SET allocated_ports_json = ?, updated_at = ? WHERE id = ?`,
		portsJSON, now(), id,
	)
	if err != nil {
		return fmt.Errorf("update body allocated_ports %s: %w", id, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("body %s: not found", id)
	}
	return nil
}

// DeleteBody deletes a body and its associated snapshots and migrations.
func (s *Store) DeleteBody(ctx context.Context, id string) error {
	unlock := s.bodyLock(id)
	defer unlock.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.ExecContext(ctx, `DELETE FROM snapshots WHERE body_id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete snapshots for body %s: %w", id, err)
	}
	_, err = tx.ExecContext(ctx, `DELETE FROM migrations WHERE body_id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete migrations for body %s: %w", id, err)
	}
	res, err := tx.ExecContext(ctx, `DELETE FROM bodies WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete body %s: %w", id, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("body %s: not found", id)
	}

	return tx.Commit()
}
