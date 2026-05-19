# restore — Snapshot Restoration

## What it does
Restores a body filesystem from a compressed snapshot (`.tar.zst`). Decompresses and extracts to a target directory. Supports optional post-restore hook command execution.

## Key types

| Type | File | Purpose |
|------|------|---------|
| `RestoreOpts` | `restore.go` | Restoration options: post-restore command, hook timeout |

## Dependencies
- `github.com/klauspost/compress/zstd` — zstd decompression
- `archive/tar` — tar extraction

## Invariants
- Destination directory must exist
- Post-restore hooks run with configurable timeout (default 30s)
- Snapshot checksum is NOT verified during restore (verified at snapshot creation)
