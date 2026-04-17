# CLAUDE.md — Session Context for Claude Code / opencode

Mesh is a lightweight infrastructure orchestration platform that turns any collection of VMs across 50+ cloud providers into a unified container deployment target. It uses Pulumi for provisioning, Tailscale for encrypted mesh networking, Nomad for container scheduling, and Consul for service discovery — all driven by a Typer-based CLI. The open-source core lives here; enterprise features (GPU, monitoring, backups, AI agent orchestration) register as plugins via Python entry_points.

## Key Directories

```
src/mesh/
├── cli/              # Typer CLI commands, plugin discovery, Rich UI
│   ├── commands/     # init, status, deploy, destroy, logs, ssh, agent
│   ├── plugins.py    # Plugin discovery via entry_points
│   └── ui/           # Rich panels and themes
├── infrastructure/   # Node provisioning, boot scripts, provider registry
│   ├── provision_node/         # Multi-provider VM provisioning
│   ├── provision_local_cluster/ # Multipass local clusters
│   ├── provision_cloud_cluster/ # Pulumi Automation API (own venv)
│   ├── boot_consul_nomad/      # Jinja2 boot scripts
│   ├── configure_tailscale/    # Auth key generation
│   ├── providers/              # Libcloud provider implementations
│   └── progressive_activation/ # Tier detection (lite/standard/production)
├── workloads/        # Application deployment and ingress
│   ├── deploy_app/              # Tier-aware unified deployment API
│   ├── deploy_web_service/      # Nomad web app (Traefik routing)
│   ├── deploy_lite_web_service/ # Lite web service (Caddy routing)
│   ├── deploy_lite_ingress/     # Caddy HTTPS ingress
│   ├── deploy_traefik/          # Traefik TLS ingress
│   └── manage_secrets/          # GitHub Secrets → Nomad sync
└── verification/     # E2E test suites
    ├── e2e_app_deployment/      # Full cluster deployment tests
    ├── e2e_lite_mode/           # Lite mode validation
    └── e2e_multi_node_scenarios/ # Fault tolerance tests
```

## Common Workflows

```bash
# Setup
python -m venv .venv && source .venv/bin/activate
pip install -e ".[dev]"

# Run unit tests (do this before pushing)
pytest src/mesh -m "not e2e"

# Run all non-E2E tests with coverage
./run_tests.sh

# Format and lint
black src/mesh && flake8 src/mesh && mypy src/mesh --ignore-missing-imports

# Run CLI in demo mode (no real infrastructure needed)
mesh status --demo
mesh init --demo
```

## Gotchas

- **Pulumi typing**: `pulumi.*` has incomplete type stubs. Covered by mypy overrides in `pyproject.toml`. Don't fight it.
- **Typer/Click version conflict**: Typer >=0.9 requires Click >=8. Pin `click>=8.0.0` if import errors appear.
- **Test markers**: `e2e` tests need a live Nomad cluster. Always use `-m "not e2e"` for local dev.
- **provision_cloud_cluster**: Has its own virtualenv. Excluded from Black in `pyproject.toml`.
- **Memory budget**: 530MB control plane target. Justify any new RAM cost.
- **Demo mode**: All CLI commands accept `--demo` for testing without infrastructure.

## Before Modifying a Feature

1. Read its `CONTEXT.md` first — it is the design contract.
2. Tests are co-located: `test_foo.py` next to `foo.py`.
3. Follow conventional commits: `feat(scope):`, `fix(scope):`, etc.
4. Run `pytest src/mesh -m "not e2e"` before pushing.
