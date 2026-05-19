# store — SQLite Persistence Layer

## What it does
Provides SQLite-backed CRUD for bodies, snapshots, migrations, and key-value config. Uses WAL mode, foreign keys, and busy timeout. Includes schema migration system (v1→v2→v3).

## Key types

| Type | File | Purpose |
|------|------|---------|
| `Store` | `store.go` | Database handle with per-body mutex pool |
| `BodyRecord` | `store.go` | Row type for bodies table |
| `SnapshotRecord` | `store.go` | Row type for snapshots table |
| `MigrationRecord` | `store.go` | Row type for migrations table |

## Schema (v3)
- `bodies` — id, name, state, spec_json, substrate, instance_id, cluster_id, timestamps
- `snapshots` — id, body_id, manifest_json, storage_path, size_bytes, cluster_id, created_at
- `migrations` — id, body_id, target_substrate, current_step, snapshot_id, cluster_id, started_at, error
- `config` — key-value store for schema_version and runtime config

## Dependencies
- `internal/orchestrator` — BodyState type
- `modernc.org/sqlite` — CGo-free SQLite driver

## Invariants
- `MaxOpenConns(1)` — single writer, serialized access
- WAL mode with 5s busy timeout
- Foreign keys enforced
- Per-body mutexes prevent concurrent mutations on same body
- Schema version tracked in config table
- `NULL` and empty string are semantically equivalent for cluster_id
