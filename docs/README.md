# Mesh Documentation

Welcome to the Mesh documentation. Mesh is a portable agent-body runtime for AI agents — giving them a persistent compute identity that can live on any substrate and move between them.

## What is Mesh?

Mesh provides two core abstractions:

- **Body**: A portable compute identity with persistent filesystem state. Bodies can move between substrates without losing themselves.
- **Form**: The physical instantiation of a body on a substrate. A body can take many forms over its lifetime (laptop, VM, container).

The snapshot primitive is `docker export | zstd` — a flat filesystem tarball. No memory state, fully portable.

## Features

- **Daemon with Docker + Nomad multi-adapter routing**
- **16 MCP tools** for body CRUD and migration
- **7-step cold migration coordinator** with S3 registry
- **Plugin system** (go-plugin + gRPC + protobuf)
- **CLI** (`mesh serve`, `mesh stop`, `mesh status`, `mesh init`)
- **Bootstrap** (goreleaser, install.sh, Homebrew formula coming soon)
- **CI** (GitHub Actions with integration tests)
- **17 packages** test-passing

## Quick Links

- [Architecture Overview](architecture.md) — System design and component diagrams
- [CLI Reference](cli-reference.md) — Command-line interface documentation
- [MCP API](mcp-api.md) — Model Context Protocol API reference
- [REST API](rest-api.md) — HTTP REST API for scripting and tooling
- [Migration Guide](migration.md) — Migrating from v0 to v1
- [Package Documentation](internal/packages.md) — Internal package reference

## Getting Started

### Install from Source

```bash
git clone https://github.com/rethink-paradigms/mesh.git
cd mesh
go build -o /usr/local/bin/mesh ./cmd/mesh/
```

Requires Go 1.25 or later. No CGo, no system dependencies beyond a working Go toolchain and Docker.

### Download a Release

Prebuilt binaries for Linux and macOS (amd64 and arm64) are available on the [GitHub Releases page](https://github.com/rethink-paradigms/mesh/releases). Download, extract, and place the `mesh` binary in your `$PATH`.

### Quick Start

```bash
# Initialize config
mesh init

# Start the daemon
mesh serve

# Check daemon status
mesh status
```

## Substrate Pools

Mesh supports three substrate pools:

- **Local**: Laptop, workstation, or Raspberry Pi
- **Fleet**: BYO VMs scheduled via Nomad (not Kubernetes)
- **Sandbox**: Cloud environments like Daytona, E2B, Fly, Modal, Cloudflare

## Primary Interface

The primary interface is the **MCP server** over stdio. AI agents communicate with Mesh via JSON-RPC. A CLI is provided for human operators.

## Deployment

### Systemd

A systemd unit file is provided at [`contrib/systemd/mesh.service`](https://github.com/rethink-paradigms/mesh/blob/main/contrib/systemd/mesh.service). Install it to run Mesh as a system service:

```bash
sudo cp contrib/systemd/mesh.service /etc/systemd/system/
sudo systemctl enable --now mesh
```

### Authentication

The REST API uses Bearer token authentication. Configure the token in `mesh.yaml` under `auth_token`. See the [REST API authentication docs](rest-api.md#authentication) for details.

## License

MIT. See [GitHub repository](https://github.com/rethink-paradigms/mesh) for full license text.
