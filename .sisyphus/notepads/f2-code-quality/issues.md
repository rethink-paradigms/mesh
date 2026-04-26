# F2 Code Quality Review — Issues

## Open Issues
1. **Shell injection in clone.go** — remoteDir and snapshotFile need proper escaping before interpolation into SSH shell commands. Use `shellescape` or refactor to avoid shell string construction.
2. **Dead flags** — --verbose and --quiet defined but never consumed. Either implement or remove.
3. **golangci-lint not installed** — could not run lint checks. Consider adding to dev tooling.

## Resolved
- All 7 test packages pass with race detector enabled.
- No forbidden imports found.
- No AI slop detected.
