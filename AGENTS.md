# AGENTS.md — AI Contributor Instructions

Instructions for AI agents (Claude, Copilot, Cursor, etc.) working on the Mesh codebase.

## Build & Install

```bash
python -m venv .venv && source .venv/bin/activate
pip install -e ".[dev]"
```

## Test Commands

```bash
# Unit + integration (fast, no cluster needed)
pytest src/mesh -m "not e2e"

# Exclude slow/integration/cloud markers
pytest src/mesh -m "not e2e and not integration and not cloud_only"

# Full suite excluding live-cluster E2E
./run_tests.sh

# E2E tests (requires running local cluster)
RUN_E2E=1 ./run_tests.sh

# Specific test markers available:
#   slow, integration, e2e, local_only, destructive, cloud_only, cross_cloud
```

## Lint & Format

```bash
black src/mesh
flake8 src/mesh
mypy src/mesh --ignore-missing-imports
isort src/mesh
```

## Project Conventions

### Vertical Slice Architecture

Each feature lives in its own directory under `src/mesh/` with four domains:

| Domain | Directory | Purpose |
|:---|:---|:---|
| CLI | `src/mesh/cli/` | Typer commands, plugin discovery, Rich UI |
| Infrastructure | `src/mesh/infrastructure/` | Node provisioning, boot scripts, providers |
| Workloads | `src/mesh/workloads/` | App deployment, ingress, secrets |
| Verification | `src/mesh/verification/` | E2E test suites |

### CONTEXT.md Design Contracts

Every feature directory has a `CONTEXT.md` that defines its public interface, dependencies, and design decisions. **Read the CONTEXT.md before modifying a feature. Write the CONTEXT.md first when adding a new feature.**

### Co-located Tests

Tests live next to the code they test: `test_<thing>.py` beside `<thing>.py`. No separate `tests/` tree.

### Conventional Commits

```
feat(scope): description
fix(scope): description
refactor(scope): description
docs: description
test: description
chore: description
```

## Key Files

| File | Purpose |
|:---|:---|
| `pyproject.toml` | Build config, dependencies, tool settings |
| `conftest.py` | Test path setup |
| `run_tests.sh` | Test runner script |
| `src/mesh/cli/main.py` | CLI entry point (Typer app) |
| `src/mesh/cli/plugins.py` | Plugin discovery via entry_points |
| `src/mesh/infrastructure/providers/` | Multi-cloud provider implementations (Libcloud) |
| `.env.example` | Required environment variables template |

## Memory Budget

The control plane targets **~530MB RAM total**. New components must justify their RAM cost. The platform is deliberately lightweight — no heavyweight dependencies.

## Plugin Architecture

Mesh supports plugins via Python `entry_points`:

```toml
[project.entry-points."mesh.plugins"]
my-command = "my_package.cli:register"
```

Enterprise features (GPU, monitoring, backups, AI agent orchestration) register as plugins and live in a separate `mesh-enterprise` package. The integration surface is in `src/mesh/cli/plugins.py`.

## Gotchas

- **Pulumi typing**: `pulumi.*` modules have incomplete stubs. `mypy` ignores them via `pyproject.toml` overrides. Do not add strict type annotations to Pulumi resource calls.
- **Typer version conflict**: Typer >=0.9.0 requires Click >=8.0. If you see Click import errors, pin `click>=8.0.0`.
- **Test markers**: Always use `-m "not e2e"` for local runs. E2E tests require a live Nomad cluster.
- **Pulumi cloud cluster dir**: `src/mesh/infrastructure/provision_cloud_cluster/` has its own virtualenv and is excluded from Black formatting.
- **Demo mode**: All CLI commands support `--demo` flag for testing without real infrastructure.
