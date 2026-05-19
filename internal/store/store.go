// Package store provides SQLite wrapper with WAL mode, body CRUD, snapshot metadata, and schema migrations.
package store

import (
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/rethink-paradigms/mesh/internal/orchestrator"
	_ "modernc.org/sqlite"
)

// BodyRecord represents a row in the bodies table.
type BodyRecord struct {
	ID         string
	Name       string
	State      orchestrator.BodyState
	SpecJSON   string
	Substrate  string
	InstanceID string
	ClusterID  string
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
	ClusterID    string
	CreatedAt    string
}

// MigrationRecord represents a row in the migrations table.
type MigrationRecord struct {
	ID              string
	BodyID          string
	TargetSubstrate string
	CurrentStep     int
	SnapshotID      string
	ClusterID       string
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

	if version == "3" {
		return nil // already at latest
	}

	if version == "" {
		// Version 0 or missing: create all tables fresh
		_, err = db.Exec(schemaV1)
		if err != nil {
			return fmt.Errorf("apply schema v1: %w", err)
		}
		_, err = db.Exec("INSERT OR REPLACE INTO config (key, value) VALUES ('schema_version', '2')")
		if err != nil {
			return fmt.Errorf("set schema_version: %w", err)
		}
	}

	if version == "1" {
		// v1 → v2: add substrate column with default "docker"
		_, err = db.Exec(`ALTER TABLE bodies ADD COLUMN substrate TEXT DEFAULT 'docker'`)
		if err != nil {
			return fmt.Errorf("migrate v1→v2 add substrate column: %w", err)
		}
		_, err = db.Exec("INSERT OR REPLACE INTO config (key, value) VALUES ('schema_version', '2')")
		if err != nil {
			return fmt.Errorf("set schema_version to 2: %w", err)
		}
	}

	// v2 → v3: add cluster_id column to bodies, snapshots, migrations
	_, err = db.Exec(`ALTER TABLE bodies ADD COLUMN cluster_id TEXT DEFAULT NULL`)
	if err != nil {
		return fmt.Errorf("migrate v2→v3 add cluster_id to bodies: %w", err)
	}
	_, err = db.Exec(`ALTER TABLE snapshots ADD COLUMN cluster_id TEXT DEFAULT NULL`)
	if err != nil {
		return fmt.Errorf("migrate v2→v3 add cluster_id to snapshots: %w", err)
	}
	_, err = db.Exec(`ALTER TABLE migrations ADD COLUMN cluster_id TEXT DEFAULT NULL`)
	if err != nil {
		return fmt.Errorf("migrate v2→v3 add cluster_id to migrations: %w", err)
	}
	_, err = db.Exec("INSERT OR REPLACE INTO config (key, value) VALUES ('schema_version', '3')")
	if err != nil {
		return fmt.Errorf("set schema_version to 3: %w", err)
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
