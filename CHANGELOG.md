# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/)
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [1.0.0] - 2026-05-01

### Added

- Daemon with Docker + Nomad multi-adapter routing
- 16 MCP tools for body CRUD and migration
- 7-step cold migration coordinator with S3 registry
- Plugin system (go-plugin + gRPC + protobuf)
- CLI (`mesh serve`, `mesh stop`, `mesh status`, `mesh init`)
- Bootstrap (goreleaser, install.sh, Homebrew formula)
- CI (GitHub Actions with integration tests)
- 17 packages test-passing

### Changed

- Complete redesign — Python v0 → Go v1

[Unreleased]: https://github.com/rethink-paradigms/mesh/compare/v1.0.0...HEAD
[1.0.0]: https://github.com/rethink-paradigms/mesh/releases/tag/v1.0.0
