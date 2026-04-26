# F2 Code Quality Review — Learnings

## Codebase Quality
- Codebase is clean, idiomatic Go. No AI slop, no over-commenting.
- Error handling is consistent: `%w` wrapping, meaningful error messages with context prefixes.
- Test coverage is good: every internal/ package has tests (except transport which has no test file).
- All cmd/ output goes through `cmd.OutOrStdout()` / `cmd.ErrOrStderr()` — correct Cobra pattern.

## Patterns to Note
- `internal/` packages correctly return errors, never print directly.
- Streaming pipeline in snapshot uses `io.Pipe` — no full tarball in memory.
- Path traversal protection in restore's `extractTar` — good security practice.
- Atomic rename in restore with EXDEV fallback — robust across filesystems.
- `golangci-lint` not installed on this machine — could not run lint check.

## Issues Found
- Shell injection in clone.go remote commands (HIGH) — needs shellescape.
- Dead flags --verbose/--quiet in main.go (MEDIUM).
- Silently ignored errors in main.go:99 and snapshot.go:209 (LOW).
- Hand-rolled contains() in config_test.go instead of strings.Contains (LOW).
