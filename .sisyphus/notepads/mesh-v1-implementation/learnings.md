

## Task 22-followup: Fix S3RegistryPlugin to implement body.Registry interface

### Implementation approach
- Updated `S3RegistryPlugin.Push` signature from `(ctx, key, r) (string, error)` to `(ctx, key, r, size, sha256) error`.
- Push no longer computes SHA-256 internally; it accepts the caller-provided sha256 and sets it as S3 metadata directly.
- Updated `S3RegistryPlugin.Pull` signature from `(ctx, key) (*PullResult, error)` to `(ctx, key) (io.ReadCloser, string, error)`.
- Removed `PullResult` struct since it's no longer needed.
- Added `S3RegistryPlugin.Verify(ctx, key, expectedSHA256) error` which uses `HeadObject` to read metadata without downloading the full object body.
- Updated `registry_test.go` testPlugin overrides and all test calls to match new signatures.
- Removed unused `crypto/sha256` and `encoding/hex` imports from registry_test.go.

### Key learnings
- The `body.Registry` interface is the contract; `S3RegistryPlugin` must adapt to it, not the other way around.
- Using `HeadObject` for Verify is more efficient than `GetObject` because it doesn't download the body.
- The testPlugin in registry_test.go duplicates the real plugin logic; keeping it in sync is necessary but brittle. Consider using the real S3RegistryPlugin with a mock S3 client interface instead.
- The `CopyObject` call in Push is still needed to set metadata after multipart upload completes.

### Files changed
- `internal/registry/push.go`: Push signature changed, removed internal SHA-256 computation
- `internal/registry/pull.go`: Pull signature changed, removed PullResult struct, added Verify method
- `internal/registry/registry_test.go`: Updated testPlugin overrides, all test calls, removed unused imports

### Verification
- `go test ./internal/registry/` - PASS (13 tests)
- `go test ./...` - PASS (all packages, no regressions)
- `go test -tags=integration -race ./integration/ -run "Migration|CrossMachine|SameMachine"` - PASS (3 tests, no race conditions)
