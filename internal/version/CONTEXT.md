# version — Build Version Injection

## What it does
Provides the build version string via a package-level variable. Injected at build time via `-ldflags "-X github.com/rethink-paradigms/mesh/internal/version.Version=..."`.

## Key types

| Type | File | Purpose |
|------|------|---------|
| `Version` (var) | `version.go` | Build version string, defaults to "dev" |

## Dependencies
None.

## Invariants
- Default value is "dev" (not empty)
- Injected via GoReleaser at release build time
- Read by both daemon and CLI for version reporting
