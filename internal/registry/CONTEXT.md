# registry — S3 Snapshot Registry Plugin

## What it does
Implements the `body.Registry` interface for cross-machine migration. Pushes/pulls body snapshots to S3-compatible storage. Used by MigrationCoordinator for the transfer step when source and target are on different machines.

## Key types

| Type | File | Purpose |
|------|------|---------|
| `S3RegistryPlugin` | `plugin.go` | S3 client implementing body.Registry |
| `RegistryConfig` | `plugin.go` | S3 credentials: bucket, region, endpoint, access keys |

## Dependencies
- `github.com/aws/aws-sdk-go-v2/service/s3` — AWS S3 SDK
- `github.com/aws/aws-sdk-go-v2/config` — AWS credential loading
- `internal/body` — Registry interface

## Invariants
- Supports any S3-compatible endpoint (MinIO, CloudFlare R2, etc.)
- SHA-256 verification on pull
- Hot-swappable at runtime via daemon API
- Config is persisted to store for restart survival
- Falls back gracefully to same-machine migration when disconnected
