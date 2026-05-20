// Package version provides the single source of truth for the Mesh version.
package version

// Version is the current release version of Mesh.
// Set via -ldflags during build (defaults to "1.0.8" for release commits).
var Version = "1.0.8"

// Commit is the git commit hash the binary was built from.
// Set via -ldflags: -X 'github.com/rethink-paradigms/mesh/internal/version.Commit=$(git rev-parse --short HEAD)'
var Commit = "unknown"

// BuildTime is the UTC timestamp when the binary was built.
// Set via -ldflags: -X 'github.com/rethink-paradigms/mesh/internal/version.BuildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)'
var BuildTime = "unknown"
