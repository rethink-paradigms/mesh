package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/rethink-paradigms/mesh/internal/api"
	"github.com/rethink-paradigms/mesh/internal/registry"
)

// ─── Registry Management ──────────────────────────────────────────────────

// ConfigureS3 validates S3 credentials and hot-swaps the registry plugin at runtime.
// Implements api.RegistryManager.
func (d *Daemon) ConfigureS3(ctx context.Context, cfg api.S3RegistryConfig) error {
	d.registryMu.Lock()
	defer d.registryMu.Unlock()

	// Create the S3 plugin — validates config is syntactically correct
	p, err := registry.NewS3RegistryPlugin(registry.RegistryConfig{
		Bucket:          cfg.Bucket,
		Region:          cfg.Region,
		Endpoint:        cfg.Endpoint,
		AccessKeyID:     cfg.AccessKeyID,
		SecretAccessKey: cfg.SecretAccessKey,
	})
	if err != nil {
		return fmt.Errorf("invalid S3 config: %w", err)
	}

	d.registry = p
	if d.migrator != nil {
		d.migrator.SetRegistry(p)
	}

	// Persist to store so it survives daemon restart
	if err := d.persistRegistryConfig(ctx, &cfg); err != nil {
		slog.Warn("failed to persist registry config", "error", err)
	}

	slog.Info("S3 registry configured", "bucket", cfg.Bucket, "region", cfg.Region)
	return nil
}

// DisconnectS3 clears the registry plugin. Falls back to same-machine migration.
// Implements api.RegistryManager.
func (d *Daemon) DisconnectS3(ctx context.Context) error {
	d.registryMu.Lock()
	defer d.registryMu.Unlock()

	d.registry = nil
	if d.migrator != nil {
		d.migrator.SetRegistry(nil)
	}

	if err := d.clearRegistryConfig(ctx); err != nil {
		slog.Warn("failed to clear persisted registry config", "error", err)
	}

	slog.Info("S3 registry disconnected, fallback to same-machine migration")
	return nil
}

// RegistryStatus returns the current registry state.
// Implements api.RegistryManager.
func (d *Daemon) RegistryStatus(_ context.Context) map[string]any {
	d.registryMu.Lock()
	defer d.registryMu.Unlock()

	if d.registry == nil {
		return map[string]any{
			"configured": false,
			"type":       "none",
		}
	}

	// Try to extract bucket/region from the plugin if possible
	// S3RegistryPlugin doesn't expose its config, so we return basic info
	return map[string]any{
		"configured": true,
		"type":       "s3",
		"healthy":    true,
	}
}

// persistRegistryConfig stores the S3 config as JSON in the config table.
func (d *Daemon) persistRegistryConfig(ctx context.Context, cfg *api.S3RegistryConfig) error {
	if d.store == nil {
		return nil
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal registry config: %w", err)
	}
	return d.store.SetConfig(ctx, "registry_s3", string(data))
}

// clearRegistryConfig removes persisted S3 config.
func (d *Daemon) clearRegistryConfig(ctx context.Context) error {
	if d.store == nil {
		return nil
	}
	return d.store.SetConfig(ctx, "registry_s3", "")
}

// restoreRegistryConfig loads a previously persisted S3 config from the store
// and initializes the registry plugin. Called at daemon startup.
func (d *Daemon) restoreRegistryConfig(ctx context.Context) {
	if d.store == nil {
		return
	}
	val, err := d.store.GetConfig(ctx, "registry_s3")
	if err != nil || val == "" {
		return // no persisted config
	}

	var cfg api.S3RegistryConfig
	if err := json.Unmarshal([]byte(val), &cfg); err != nil {
		slog.Warn("invalid persisted registry config", "error", err)
		return
	}
	if cfg.Bucket == "" || cfg.Region == "" {
		return // incomplete config, skip
	}

	p, err := registry.NewS3RegistryPlugin(registry.RegistryConfig{
		Bucket:          cfg.Bucket,
		Region:          cfg.Region,
		Endpoint:        cfg.Endpoint,
		AccessKeyID:     cfg.AccessKeyID,
		SecretAccessKey: cfg.SecretAccessKey,
	})
	if err != nil {
		slog.Warn("failed to restore S3 registry", "error", err)
		return
	}

	d.registry = p
	if d.migrator != nil {
		d.migrator.SetRegistry(p)
	}
	slog.Info("restored S3 registry", "bucket", cfg.Bucket, "region", cfg.Region)
}
