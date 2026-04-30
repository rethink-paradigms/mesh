# Internal Packages Reference

Consolidated reference for all packages in `internal/`.

## internal/adapter

SubstrateAdapter interface definition for substrate-agnostic body provisioning.

## internal/body

Body state machine (8 states), lifecycle operations, and migration coordinator.

## internal/config

YAML configuration parsing and validation for Mesh daemon.

## internal/config-toml

Parses and validates TOML configuration for agents, machines, and hooks.

## internal/daemon

Long-running process with signal handling, PID file, and graceful shutdown.

## internal/docker

Docker adapter implementing SubstrateAdapter interface for container lifecycle management.

## internal/manifest

Reads and writes JSON manifest sidecar files for each snapshot.

## internal/mcp

MCP server (stdio transport) with tool registration and JSON-RPC request routing.

## internal/restore

Handles snapshot restoration: hash verification + atomic extraction.

## internal/snapshot

Handles filesystem snapshot creation: tar + zstd compression + SHA-256 hashing.

## internal/store

SQLite wrapper with WAL mode, body CRUD, snapshot metadata, and schema migrations.
