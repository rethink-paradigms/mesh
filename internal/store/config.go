package store

import (
	"context"
	"database/sql"
	"fmt"
)

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

func (s *Store) QueryRow(ctx context.Context, query string, args ...any) *sql.Row {
	return s.db.QueryRowContext(ctx, query, args...)
}
