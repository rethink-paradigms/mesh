# Mesh — Portable Agent-Body Runtime

## Project Structure

Mesh is a Go binary (v0 built, v1 in progress). Two products:
- **Mesh** (open source): CLI + MCP server + skills. Go binary. Manages agent compute bodies (provision, execute, snapshot, migrate, destroy).
- **AgentBodies** (commercial UI): React web app on top of Mesh. AI-first agent body management interface.

```
cmd/mesh/        — CLI entry point (Cobra)
internal/
  agent/          — Process management
  clone/          — Cross-machine body cloning
  config/         — TOML config loading
  manifest/       — Snapshot manifest read/write
  restore/        — Filesystem restore from snapshot
  snapshot/       — Filesystem snapshot (tar + zstd)
  transport/      — SSH transport layer
  provider/       — (v1) Substrate adapter interface + implementations
  mcp/            — (v1) MCP server for agent tools
  orchestration/  — (v1) Body state machine
discovery/        — Design docs, constraints, decisions
DESIGN.md         — AgentBodies design system (fonts, colors, spacing, layout, motion)
```

## Architecture Philosophy

Library-first. Internal packages are the library with clean contracts. CLI is a thin wrapper. MCP server (v1) wraps the same library. No shelling out between components.

## Design System

Always read DESIGN.md before any UI work. AgentBodies uses "The Craftsman's Bench" aesthetic: dark warm palette, Instrument Serif display, Geist Sans body, Berkeley Mono code, copper accent (#C8956C). Chat-first layout with three zones (Prompt, Canvas, Rail). No dashboard default view.

## Key Constraints

- Single Go binary, no CGo, no daemon in v0
- Provider-agnostic: adding a new provider = one Go interface + config entry
- MCP + skills as primary interface (not SDK)
- v0 (snapshot/restore/clone) is the Persistence layer, it doesn't change, it extends

## Current State

v0 is done and working. v1 Stage 1 (Provider interface + Daytona provider) is in progress in a separate workspace. AgentBodies UI is pre-implementation, design system just shipped.

## Build & Test

```bash
go build -o /usr/local/bin/mesh ./cmd/mesh/
go test ./...
```

Go 1.25+. No external dependencies beyond cobra, toml, compress (zstd).