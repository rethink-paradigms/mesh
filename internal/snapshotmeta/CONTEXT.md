# snapshotmeta — Snapshot Sidecar Metadata

## What it does
Reads and writes JSON sidecar files (`.meta.json`) for snapshots. Records source machine identity, workdir path, start command, stop timeout, and checksum for traceability.

## Key types

| Type | File | Purpose |
|------|------|---------|
| `Meta` | `meta.go` | Snapshot metadata: AgentName, Timestamp, SourceMachine, SourceWorkdir, StartCmd, StopTimeout, Checksum, Size |

## Dependencies
None — stdlib only.

## Invariants
- Sidecar path: `{snapshot}.meta.json`
- Timestamps in RFC 3339
- Checksum is hex-encoded SHA-256 of the `.tar.zst`
