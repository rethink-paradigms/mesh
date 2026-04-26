# Contributing to Mesh

Thanks for your interest in contributing to Mesh!

## Prerequisites

- Go 1.25 or later
- Docker (for development and testing)
- Make (optional, for running targets in Makefile)

## Getting Started

```bash
# Clone the repository
git clone https://github.com/rethink-paradigms/mesh.git
cd mesh

# Build the binary
go build -o mesh ./cmd/mesh/

# Run tests
go test ./...

# Initialize config
./mesh init

# Start the daemon (in background)
./mesh serve &

# Check status
./mesh status

# Stop the daemon
./mesh stop
```

## Package Organization

Mesh is organized as a library-first codebase. All logic lives in `internal/` packages. The CLI is a thin Cobra wrapper.

```
cmd/mesh/           CLI entry point, Cobra commands
internal/
  adapter/           SubstrateAdapter interface (substrate abstraction)
  body/              Body state machine, lifecycle, transitions
  config/            YAML config parsing (v1)
  config-toml/       TOML config parsing (v0, backward compat)
  daemon/            Long-running daemon process
  docker/            Docker SubstrateAdapter implementation
  manifest/          Snapshot manifest read/write (v1 + v2)
  mcp/               MCP server (stdio JSON-RPC)
  restore/           Snapshot extraction and post-restore hooks
  snapshot/          Create filesystem snapshots (tar + zstd + SHA-256)
  store/             SQLite store with WAL mode
```

## Adding a New Internal Package

When adding a new internal package:

1. Create the package directory: `internal/<package>/`
2. Write a `CONTEXT.md` file describing:
   - What the package does
   - Key types and interfaces
   - Dependencies on other packages
   - Usage examples
3. Write the package code following Go conventions
4. Add tests alongside the code (`<package>_test.go`)
5. Document exported functions, types, and constants

Example `internal/adapter/CONTEXT.md`:

```markdown
# Adapter Package

Provides the SubstrateAdapter interface for abstracting different substrate types (Docker, Nomad, cloud providers).

## Interface

```go
type SubstrateAdapter interface {
    // Provision creates a new instance on the substrate
    Provision(ctx context.Context, spec InstanceSpec) (Instance, error)

    // Start launches an existing instance
    Start(ctx context.Context, id string) error

    // Stop stops a running instance
    Stop(ctx context.Context, id string) error

    // Snapshot captures instance state
    Snapshot(ctx context.Context, id string) (Snapshot, error)

    // Restore restores from a snapshot
    Restore(ctx context.Context, snapshot Snapshot) (Instance, error)
}
```

## Dependencies

- None (leaf package)

## Usage

See `internal/docker/` for a reference implementation.
```

## Code Style

- Follow [Effective Go](https://go.dev/doc/effective_go)
- Run `go fmt ./...` before committing
- Run `go vet ./...` to catch common errors
- Use `golangci-lint run` for comprehensive linting

### Naming Conventions

- Package names: lowercase, single word, descriptive
- Interface names: simple names ending in `er` (e.g., `Reader`, `Adapter`)
- Exported functions: PascalCase
- Private functions: camelCase
- Constants: UPPER_SNAKE_CASE for exported, camelCase for private
- Errors: returned as error values, not panics

## Testing

Write tests for all non-trivial code. Tests should be:

- Fast: avoid sleeping, use interfaces for external deps
- Isolated: don't rely on global state
- Deterministic: same input produces same output

```bash
# Run all tests
go test ./...

# Run with race detector
go test -race ./...

# Run with coverage
go test -cover ./...

# Run specific package
go test ./internal/store

# Run with verbose output
go test -v ./internal/daemon
```

## Commit Conventions

Follow [Conventional Commits](https://www.conventionalcommits.org/):

```
feat: add MCP server for agent communication
fix: resolve PID file race condition on startup
refactor: simplify body state machine transitions
docs: update README for v1
test: add migration coordinator tests
chore: upgrade gopkg.in/yaml.v3 to v3.0.1
```

## Pull Request Process

1. Fork the repository
2. Create a branch for your change: `git checkout -b feature/my-feature`
3. Make your changes and write tests
4. Ensure tests pass: `go test ./...`
5. Ensure linting passes: `golangci-lint run`
6. Commit with conventional commit messages
7. Push and create a pull request

### PR Checklist

- Tests added for new behavior
- Existing tests still pass
- Code is formatted (`go fmt ./...`)
- No linting errors (`golangci-lint run`)
- Documentation updated (README, CONTEXT.md, code comments)
- Commits follow conventional commit format

## Development Workflow

For daemon development:

```bash
# Build
go build -o mesh ./cmd/mesh/

# Run in foreground with debug logging
./mesh serve --config ~/.mesh/config.yaml --verbose

# In another terminal, test MCP
echo '{"jsonrpc":"2.0","id":1,"method":"ping"}' | ./mesh mcp
```

For snapshot/restore testing (v0 commands):

```bash
# Create a test workdir
mkdir -p /tmp/test-agent && echo "hello" > /tmp/test-agent/state.txt

# Take a snapshot
./mesh snapshot test-agent

# List snapshots
./mesh list

# Inspect snapshot
./mesh inspect test-agent

# Restore to a different location
rm -rf /tmp/test-agent-restored
./mesh restore test-agent --snapshot ~/.mesh/snapshots/test-agent/test-agent-*.tar.zst
```

## Memory Budget

The daemon targets minimal memory usage. The SQLite store uses WAL mode for efficient concurrent access. New features should justify their memory cost.

## Reporting Bugs and Security Issues

- Bugs: open an issue at [github.com/rethink-paradigms/mesh/issues](https://github.com/rethink-paradigms/mesh/issues)
- Security: please do not open public issues. Use GitHub's private vulnerability reporting — see [SECURITY.md](SECURITY.md)

## Code of Conduct

See [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md). Be kind, assume good faith, and critique the code, not the author.
