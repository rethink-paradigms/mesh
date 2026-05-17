# Releasing Mesh

Mesh releases are **fully automated**. Every merge to `main` cuts a new release — no human steps required.

## How it works

```
merge to main
    │
    ▼
┌─────────────────┐
│  CI: vet + test │  ← gate: must pass before any release
└────────┬────────┘
         │
         ▼
┌─────────────────────────────┐
│  Auto-bump patch version    │  ← internal/version/version.go
│  Commit with [skip ci]      │  ← avoids infinite loop
│  Tag (semver, no v prefix)  │  ← e.g. 1.0.1
│  Push commit + tag          │
└────────┬────────────────────┘
         │
         ▼
┌─────────────────────────────┐
│  GoReleaser builds + publishes │
│  • mesh binary (Linux/macOS) │
│  • mesh-daemon binary        │
│  • archives + checksums      │
│  • GitHub Release            │
└─────────────────────────────┘
```

## Version source of truth

The version lives in one place:

```go
// internal/version/version.go
const Version = "1.0.0"
```

Every binary, API response, and MCP capability report reads from this constant. The release workflow auto-increments the patch component on every merge.

## Tag format

Tags are **semver without a `v` prefix**:

- ✅ `1.0.0`
- ✅ `1.0.1`
- ✅ `1.1.0-rc1`
- ❌ `v1.0.0`

This keeps archive names, checksums, and install URLs clean and consistent.

## What triggers a release

Any push to `main` that passes vet + test triggers the pipeline. This includes:

- Merged pull requests
- Direct pushes (discouraged but handled)

The workflow uses `[skip ci]` in the version-bump commit to prevent infinite loops.

## What the VM gets

The install script queries GitHub for the latest release:

```bash
curl -fsSL https://raw.githubusercontent.com/rethink-paradigms/mesh/main/scripts/install.sh | sh
```

Because releases are cut on every merge, `releases/latest` always points to the code at `main` HEAD. There is no drift between working tree and deployed binary.

## Manual override (emergency only)

If you need to cut a release outside the normal merge flow (e.g. hotfix on a release branch):

```bash
# 1. Bump the version constant manually
vim internal/version/version.go   # edit const Version

# 2. Commit and push
git add internal/version/version.go
git commit -m "chore(release): bump version to 1.0.1"
git push origin main

# 3. Tag and push (the workflow is already triggered by the push,
#    but if you need to tag a specific commit without pushing to main)
git tag -a 1.0.1 -m "Release 1.0.1"
git push origin 1.0.1
```

## For downstream provisioning tools

Provisioning systems that need the latest stable version query the GitHub API:

```bash
curl -fsSL https://api.github.com/repos/rethink-paradigms/mesh/releases/latest | jq -r '.tag_name'
# → 1.0.1
```

No version is hard-coded in provisioning. Mesh is independent; provisioners query it.
