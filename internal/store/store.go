// Package store provides SQLite wrapper with WAL mode, body CRUD, snapshot metadata, and schema migrations.
package store

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/rethink-paradigms/mesh/internal/adapter"
	_ "modernc.org/sqlite"
)

// BodyRecord represents a row in the bodies table.
type BodyRecord struct {
	ID         string
	Name       string
	State      adapter.BodyState
	SpecJSON   string
	Substrate  string
	InstanceID string
	CreatedAt  string
	UpdatedAt  string
}

// SnapshotRecord represents a row in the snapshots table.
type SnapshotRecord struct {
	ID           string
	BodyID       string
	ManifestJSON string
	StoragePath  string
	SizeBytes    int64
	CreatedAt    string
}

// MigrationRecord represents a row in the migrations table.
type MigrationRecord struct {
	ID              string
	BodyID          string
	TargetSubstrate string
	CurrentStep     int
	SnapshotID      string
	StartedAt       string
	Error           string
}

// Store wraps a SQLite database with WAL mode, per-body mutexes, and CRUD operations.
type Store struct {
	db      *sql.DB
	mu      sync.Mutex
	bodyMux map[string]*sync.Mutex
}

const schemaV1 = `
CREATE TABLE IF NOT EXISTS bodies (
	id TEXT PRIMARY KEY,
	name TEXT UNIQUE NOT NULL,
	state TEXT NOT NULL,
	spec_json TEXT,
	substrate TEXT,
	instance_id TEXT,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS snapshots (
	id TEXT PRIMARY KEY,
	body_id TEXT NOT NULL REFERENCES bodies(id),
	manifest_json TEXT,
	storage_path TEXT,
	size_bytes INTEGER,
	created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS migrations (
	id TEXT PRIMARY KEY,
	body_id TEXT NOT NULL REFERENCES bodies(id),
	target_substrate TEXT NOT NULL,
	current_step INTEGER DEFAULT 0,
	snapshot_id TEXT,
	started_at TEXT NOT NULL,
	error TEXT
);

CREATE TABLE IF NOT EXISTS config (
	key TEXT PRIMARY KEY,
	value TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_snapshots_body_id ON snapshots(body_id);
CREATE INDEX IF NOT EXISTS idx_migrations_body_id ON migrations(body_id);
`

// Open opens (or creates) a SQLite database at path with WAL mode and foreign keys enabled.
func Open(path string) (*Store, error) {
	dsn := fmt.Sprintf("file:%s", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open db %s: %w", path, err)
	}

	db.SetMaxOpenConns(1)

	init := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA foreign_keys=ON",
		"PRAGMA busy_timeout=5000",
	}
	for _, stmt := range init {
		if _, err := db.Exec(stmt); err != nil {
			db.Close()
			return nil, fmt.Errorf("exec %q: %w", stmt, err)
		}
	}

	if err := migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return &Store{db: db}, nil
}

// Close closes the underlying database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// migrate runs schema migrations. Current version is tracked in the config table.
func migrate(db *sql.DB) error {
	// Create config table first so we can read schema_version
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS config (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL
	)`)
	if err != nil {
		return fmt.Errorf("create config table: %w", err)
	}

	var version string
	err = db.QueryRow("SELECT value FROM config WHERE key = 'schema_version'").Scan(&version)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("read schema_version: %w", err)
	}

	if version == "1" {
		return nil // already migrated
	}

	// Version 0 or missing: create all tables
	_, err = db.Exec(schemaV1)
	if err != nil {
		return fmt.Errorf("apply schema v1: %w", err)
	}

	_, err = db.Exec("INSERT OR REPLACE INTO config (key, value) VALUES ('schema_version', '1')")
	if err != nil {
		return fmt.Errorf("set schema_version: %w", err)
	}

	return nil
}

// bodyLock acquires the per-body mutex for the given id. Callers must unlock the returned mutex.
func (s *Store) bodyLock(id string) *sync.Mutex {
	s.mu.Lock()
	if s.bodyMux == nil {
		s.bodyMux = make(map[string]*sync.Mutex)
	}
	m, ok := s.bodyMux[id]
	if !ok {
		m = new(sync.Mutex)
		s.bodyMux[id] = m
	}
	s.mu.Unlock()
	m.Lock()
	return m
}

// now returns the current UTC timestamp as an ISO 8601 string.
func now() string {
	return time.Now().UTC().Format(time.RFC3339)
}

// --- Body CRUD ---

// CreateBody inserts a new body record.
func (s *Store) CreateBody(ctx context.Context, id, name string, state adapter.BodyState, specJSON, substrate, instanceID string) error {
	unlock := s.bodyLock(id)
	defer unlock.Unlock()

	ts := now()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO bodies (id, name, state, spec_json, substrate, instance_id, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		id, name, string(state), specJSON, substrate, instanceID, ts, ts,
	)
	if err != nil {
		return fmt.Errorf("create body %s: %w", id, err)
	}
	return nil
}

// GetBody retrieves a body record by id.
func (s *Store) GetBody(ctx context.Context, id string) (*BodyRecord, error) {
	var b BodyRecord
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, state, spec_json, substrate, instance_id, created_at, updated_at
		 FROM bodies WHERE id = ?`, id,
	).Scan(&b.ID, &b.Name, &b.State, &b.SpecJSON, &b.Substrate, &b.InstanceID, &b.CreatedAt, &b.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("body %s: not found", id)
	}
	if err != nil {
		return nil, fmt.Errorf("get body %s: %w", id, err)
	}
	return &b, nil
}

// ListBodies returns all body records.
func (s *Store) ListBodies(ctx context.Context) ([]*BodyRecord, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, state, spec_json, substrate, instance_id, created_at, updated_at
		 FROM bodies ORDER BY created_at`)
	if err != nil {
		return nil, fmt.Errorf("list bodies: %w", err)
	}
	defer rows.Close()

	var bodies []*BodyRecord
	for rows.Next() {
		var b BodyRecord
		if err := rows.Scan(&b.ID, &b.Name, &b.State, &b.SpecJSON, &b.Substrate, &b.InstanceID, &b.CreatedAt, &b.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan body: %w", err)
		}
		bodies = append(bodies, &b)
	}
	return bodies, rows.Err()
}

// UpdateBodyState updates the state and updated_at timestamp of a body.
func (s *Store) UpdateBodyState(ctx context.Context, id string, state adapter.BodyState) error {
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

// DeleteBody deletes a body and its associated snapshots and migrations.
func (s *Store) DeleteBody(ctx context.Context, id string) error {
	unlock := s.bodyLock(id)
	defer unlock.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

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

// --- Snapshot CRUD ---

// CreateSnapshot inserts a new snapshot record.
func (s *Store) CreateSnapshot(ctx context.Context, id, bodyID, manifestJSON, storagePath string, sizeBytes int64) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO snapshots (id, body_id, manifest_json, storage_path, size_bytes, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		id, bodyID, manifestJSON, storagePath, sizeBytes, now(),
	)
	if err != nil {
		return fmt.Errorf("create snapshot %s: %w", id, err)
	}
	return nil
}

// ListSnapshots returns all snapshots for a given body.
func (s *Store) ListSnapshots(ctx context.Context, bodyID string) ([]*SnapshotRecord, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, body_id, manifest_json, storage_path, size_bytes, created_at
		 FROM snapshots WHERE body_id = ? ORDER BY created_at`, bodyID)
	if err != nil {
		return nil, fmt.Errorf("list snapshots for body %s: %w", bodyID, err)
	}
	defer rows.Close()

	var snaps []*SnapshotRecord
	for rows.Next() {
		var snap SnapshotRecord
		if err := rows.Scan(&snap.ID, &snap.BodyID, &snap.ManifestJSON, &snap.StoragePath, &snap.SizeBytes, &snap.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan snapshot: %w", err)
		}
		snaps = append(snaps, &snap)
	}
	return snaps, rows.Err()
}

// GetSnapshot retrieves a snapshot record by id.
func (s *Store) GetSnapshot(ctx context.Context, id string) (*SnapshotRecord, error) {
	var snap SnapshotRecord
	err := s.db.QueryRowContext(ctx,
		`SELECT id, body_id, manifest_json, storage_path, size_bytes, created_at
		 FROM snapshots WHERE id = ?`, id,
	).Scan(&snap.ID, &snap.BodyID, &snap.ManifestJSON, &snap.StoragePath, &snap.SizeBytes, &snap.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("snapshot %s: not found", id)
	}
	if err != nil {
		return nil, fmt.Errorf("get snapshot %s: %w", id, err)
	}
	return &snap, nil
}

// DeleteSnapshot deletes a snapshot record by id.
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

// --- Migration CRUD ---

// CreateMigration inserts a new migration record.
func (s *Store) CreateMigration(ctx context.Context, id, bodyID, targetSubstrate, snapshotID string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO migrations (id, body_id, target_substrate, current_step, snapshot_id, started_at)
		 VALUES (?, ?, ?, 0, ?, ?)`,
		id, bodyID, targetSubstrate, snapshotID, now(),
	)
	if err != nil {
		return fmt.Errorf("create migration %s: %w", id, err)
	}
	return nil
}

// UpdateMigration updates the current step and error fields of a migration.
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

// GetMigration retrieves a migration record by id.
func (s *Store) GetMigration(ctx context.Context, id string) (*MigrationRecord, error) {
	var m MigrationRecord
	var snapID, errStr sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT id, body_id, target_substrate, current_step, snapshot_id, started_at, error
		 FROM migrations WHERE id = ?`, id,
	).Scan(&m.ID, &m.BodyID, &m.TargetSubstrate, &m.CurrentStep, &snapID, &m.StartedAt, &errStr)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("migration %s: not found", id)
	}
	if err != nil {
		return nil, fmt.Errorf("get migration %s: %w", id, err)
	}
	m.SnapshotID = snapID.String
	m.Error = errStr.String
	return &m, nil
}

// --- Config ---

// GetConfig retrieves a config value by key.
func (s *Store) GetConfig(ctx context.Context, key string) (string, error) {
	var val string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM config WHERE key = ?`, key).Scan(&val)
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("config %s: not found", key)
	}
	if err != nil {
		return "", fmt.Errorf("get config %s: %w", key, err)
	}
	return val, nil
}

// SetConfig upserts a config key-value pair.
func (s *Store) SetConfig(ctx context.Context, key, value string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO config (key, value) VALUES (?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		key, value,
	)
	if err != nil {
		return fmt.Errorf("set config %s: %w", key, err)
	}
	return nil
}
