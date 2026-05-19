# snapshot — Filesystem Snapshot Creation

## What it does
Creates deterministic, compressed filesystem snapshots for agent bodies. Streaming pipeline: sorted dir walk → tar → zstd → SHA-256 tee → output file. Output: `.tar.zst` + `.sha256` sidecar + `.meta.json` sidecar.

## Key types

| Type | File | Purpose |
|------|------|---------|
| (package-level functions) | `snapshot.go` | CreateSnapshot, Run, SnapshotCacheDir, ResolveAgent |

## Dependencies
- `github.com/klauspost/compress/zstd` — zstd compression
- `internal/snapshotmeta` — sidecar metadata
- `internal/config-toml` — agent config resolution

## Invariants
- Deterministic output via lexicographic entry sorting
- Streaming via io.Pipe — no full tarball in memory
- Context cancellation cleans up partial files
- Output filename: `{agent}-{YYYYMMDD-HHMMSS}.tar.zst`
- Cache directory: `~/.mesh/snapshots/{agent}/`
