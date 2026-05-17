# Releasing Mesh

Mesh uses [GoReleaser](https://goreleaser.com) and GitHub Actions for automated releases.

## Version source of truth

The version lives in one place:

```
internal/version/version.go
```

```go
const Version = "1.0.0"
```

Every binary, API response, and MCP capability report reads from this constant.

## Release tags

Tags are **semver without a `v` prefix**:

- ✅ `1.0.0`
- ✅ `1.0.1`
- ✅ `1.1.0-rc1`
- ❌ `v1.0.0`

This keeps archive names, checksums, and install URLs clean and consistent.

## Release checklist

1. **Bump the version constant**
   ```bash
   # Edit internal/version/version.go
   const Version = "1.0.1"  # or whatever the next version is
   ```

2. **Commit the bump**
   ```bash
   git add internal/version/version.go
   git commit -m "chore(release): bump version to 1.0.1"
   git push origin main
   ```

3. **Tag and push**
   ```bash
   git tag -a 1.0.1 -m "Release 1.0.1"
   git push origin 1.0.1
   ```

4. **GitHub Actions does the rest**
   - Builds `mesh` and `mesh-daemon` for Linux/macOS amd64/arm64
   - Creates archives: `mesh_1.0.1_linux_amd64.tar.gz`, `mesh-daemon_1.0.1_linux_amd64.tar.gz`, etc.
   - Generates checksums
   - Publishes the GitHub Release with auto-generated changelog

5. **Verify**
   ```bash
   curl -fsSL https://github.com/rethink-paradigms/mesh/releases/download/1.0.1/install.sh | sh
   mesh --version   # should print 1.0.1
   ```

## Install script

The install script auto-detects the latest release from the GitHub API:

```bash
curl -fsSL https://raw.githubusercontent.com/rethink-paradigms/mesh/main/scripts/install.sh | sh
```

To pin a version:

```bash
MESH_VERSION=1.0.1 curl -fsSL ... | sh
```

## For downstream provisioning tools

Provisioning systems that need to know the latest stable version can query the GitHub API:

```bash
curl -fsSL https://api.github.com/repos/rethink-paradigms/mesh/releases/latest | jq -r '.tag_name'
# → 1.0.1
```

No version is hard-coded in provisioning. Mesh is independent; provisioners query it.
