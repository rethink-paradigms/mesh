# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/)
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.3.0] - 2026-04-17

First public open-source release.

### Added
- Tier-aware deployment via progressive activation: `lite` (1 VM, Caddy HTTPS),
  `standard` (multi-node), `ingress` and `production` (Traefik, Let's Encrypt).
- Multi-cloud provisioning across 50+ providers through Apache Libcloud, plus
  first-class AWS, Hetzner, DigitalOcean, and Multipass (local) adapters.
- `mesh` CLI with `init`, `deploy`, `status`, `logs`, `ssh`, `destroy` commands.
- Nomad + Consul + Tailscale control plane with a ~530MB memory budget.
- Plugin architecture via `mesh.plugins` entry point for enterprise extensions.
- Deployment templates: `deploy_web_service`, `deploy_lite_web_service`,
  `deploy_lite_ingress`, `deploy_traefik`.
- Secret management via `manage_secrets` (GitHub Secrets → Nomad Variables).
- E2E test suites for lite mode, single-cluster, and multi-node scenarios.

### Changed
- Package layout is now `src/mesh/…` (src-layout). Python imports are
  `from mesh.*` in both library code and consumers.

[Unreleased]: https://github.com/rethink-paradigms/mesh/compare/v0.3.0...HEAD
[0.3.0]: https://github.com/rethink-paradigms/mesh/releases/tag/v0.3.0
