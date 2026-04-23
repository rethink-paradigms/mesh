# PORTING GUIDE — Old Repo → New Repo

> This file captures everything worth carrying from the old Python repo to the new Go repo.
> The old codebase is fully scrapped. Only patterns, tooling configs, and process artifacts survive.
> The `discovery/` folder is NOT listed here — it moves as-is, untouched.

---

## 1. Changelog Management: Towncrier

**Tool used:** [towncrier](https://towncrier.readthedocs.io/) — fragment-based changelog management.

**Why it's good:** Instead of manually editing CHANGELOG.md, you drop a small markdown fragment per PR into `changelog.d/`. On release, towncrier assembles them into a formatted changelog. No merge conflicts on CHANGELOG.md. Each PR documents its own change.

**The pattern:**
```
changelog.d/
├── +snapshot-engine.added.md       ← "+" prefix = no issue number
├── 123.fix-login-bug.fixed.md      ← "123" = links to issue #123
└── ...
```

**Fragment naming:** `{issue_number}.{slug}.{type}.md`
- Types: `added`, `changed`, `fixed`, `deprecated`, `removed`, `security`
- If no issue, prefix with `+`

**Fragment content:** One sentence. That's it. Towncrier assembles them.

**Old config (from pyproject.toml):**
```toml
[tool.towncrier]
directory = "changelog.d"
filename = "CHANGELOG.md"
start_string = "<!-- towncrier release notes start -->\n"
underlines = ["", "", ""]
title_format = "## [{version}] - {project_date}"
issue_format = "[#{issue}](https://github.com/rethink-paradigms/mesh/issues/{issue})"
name = "rethink-mesh"

[[tool.towncrier.type]]
name = "Security"
[[tool.towncrier.type]]
name = "Removed"
[[tool.towncrier.type]]
name = "Deprecated"
[[tool.towncrier.type]]
name = "Added"
[[tool.towncrier.type]]
name = "Changed"
[[tool.towncrier.type]]
name = "Fixed"
```

**For the new Go repo:** Towncrier works with any `pyproject.toml` project. But since the new project is Go, the equivalent is **[git-cliff](https://git-cliff.norm.it/)** — same fragment-based approach but Go-native. Config goes in `cliff.toml`. Both tools do the same thing: fragment per PR → assembled changelog on release. Pick whichever fits the new repo's tooling.

---

## 2. CONTEXT.md Per Module (Vertical-Slice Documentation)

**Pattern:** Every module/directory gets a CONTEXT.md that self-documents its contract. An agent or developer reads ONE file to understand ONE module without reading the whole codebase.

**The shape:**
```markdown
# Domain: [Module Name]

**Description:**
[One paragraph: what this module does]

## Public Interface

| Feature | Input | Output | Description |
|---------|-------|--------|-------------|
| `FunctionName` | param1, param2 | returnType | What it does |

## Dependencies
- [External deps this module needs]

## Structure

module/
├── CONTEXT.md
├── file1.go
├── file2.go
└── file1_test.go

## Test Coverage
- [x] Test: [description]
- [ ] Test: [description]

## Design Decisions
- **Decision label**: Rationale
```

**Where to apply in new repo:** Each of the 6 modules (Interface, Orchestration, Provisioning, Persistence, Networking, Plugin Infrastructure) should have a CONTEXT.md following this shape.

**Why it matters for the new project:** You said that the design is the true artifact, code is compiled output. CONTEXT.md files are where the design intent of each module lives alongside the code. When an AI agent picks up a module to work on, it reads CONTEXT.md first.

---

## 3. AGENTS.md (Root-Level Agent Onboarding)

**Pattern:** A root-level file that tells any AI agent working in the repo:
- What the project is (one paragraph)
- Where to find things (discovery folder structure)
- Rules for working (check decisions before proposing, don't write code during discovery, etc.)
- Quick reference of core abstractions

**Current AGENTS.md** already points to the new system's discovery folder. It needs to be updated for:
- Go project structure (not Python)
- Implementation phase (not just discovery)
- The actual module layout

**Carry the concept. Rewrite the content for the new repo.**

---

## 4. PR Template

**Adapted for Go** (`.github/PULL_REQUEST_TEMPLATE.md`):
```markdown
## Description
<!-- Brief description of what this PR does and why. -->

## Type of Change
- [ ] Bug fix
- [ ] New feature
- [ ] Documentation update
- [ ] Refactor
- [ ] Test addition/update
- [ ] Chore (build, CI, etc.)

## Checklist
- [ ] Tests added or updated for new behavior
- [ ] CONTEXT.md updated if the feature interface changed
- [ ] `go test ./...` passes locally
- [ ] Lint passes (`gofmt`, `go vet`, `staticcheck`)
- [ ] Commit message follows Conventional Commits

## Related Issues
<!-- Link any related issues: Fixes #123 -->
```

---

## 5. Conventional Commits

**Pattern used:** `type(scope): description`
- `feat(orchestration): add body state machine`
- `fix(snapshot): handle partial export cleanup`
- `chore(persistence): towncrier fragment for streaming export`

**Scopes map to modules.** In the new repo:
- `feat(interface):`, `fix(orchestration):`, `chore(networking):`, etc.

**Carry as-is.**

---

## 6. CI/CD Shape (Go Equivalent)

```yaml
name: Test
on: [push, pull_request]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: '1.23' }
      - run: go fmt ./...
      - run: go vet ./...
      - run: go test -race -coverprofile=coverage.out ./...
      - uses: codecov/codecov-action@v4
```

**Same shape as old repo (lint → test → coverage), different tooling.**

---

## 7. .env.example Pattern

**Pattern:** Commented sections per credential type. User copies to `.env`, fills in values. Never committed.

**For the new repo:**
```bash
# Tailscale (for networking)
TAILSCALE_KEY=tskey-auth-...
TAILSCALE_TAILNET=example.tailnet.com

# Cloud provider (for substrate adapters)
# FLY_API_TOKEN=...
# or NOMAD_ADDR=...
# or DOCKER_HOST=...

# Storage (for snapshot persistence)
# SNAPSHOT_DIR=/var/lib/mesh/snapshots
# or S3_BUCKET=...
```

**Carry the pattern. Rewrite the variables.**

---

## 8. What NOT to Carry

Everything else in the old repo is dead:

- `src/mesh/` — All 85 Python files. Every CONTEXT.md (carry the pattern, not the files). All code.
- `docs/` — OLD Mesh documentation.
- `mkdocs.yml` — Docs site config.
- `conftest.py`, `run_tests.sh` — Python test infrastructure.
- `scripts/` — OLD utility scripts.
- `.github/ISSUE_TEMPLATE/` — Generic GitHub templates, recreate if needed.
- `.github/workflows/docs.yml`, `publish.yml`, `reusable-docker-build.yml`, `reusable-nomad-deploy.yml` — OLD Mesh CI.

---

## Summary: The Carry Checklist

When setting up the new repo:

- [ ] Copy `discovery/` folder as-is (the source of truth)
- [ ] Write AGENTS.md (new content, same pattern)
- [ ] Set up changelog tooling (git-cliff for Go, or towncrier if keeping pyproject.toml for tooling)
- [ ] Create `.github/PULL_REQUEST_TEMPLATE.md` (adapt the old one)
- [ ] Create `.github/workflows/test.yml` (Go equivalent)
- [ ] Create `.env.example` (new variables, same pattern)
- [ ] Add CONTEXT.md per module as you build each one (follow the shape above)
- [ ] Use conventional commits from day one
