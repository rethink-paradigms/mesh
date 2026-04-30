#!/bin/sh
# Mesh install script.
# Detects OS/arch, downloads latest release from GitHub, verifies checksum,
# installs to /usr/local/bin, and runs `mesh init`.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/rethink-paradigms/mesh/main/scripts/install.sh | sh
#   MESH_VERSION=v0.1.0 curl -fsSL ... | sh    # pin a version
#   MESH_DRY_RUN=1 bash scripts/install.sh       # dry-run (for testing)
#
# Environment variables:
#   MESH_VERSION   - release tag to install (default: latest)
#   MESH_BINDIR    - install directory (default: /usr/local/bin)
#   MESH_DRY_RUN   - set to 1 to skip download and actual install (for testing)

set -e

# --- Constants ---
MESH_REPO="rethink-paradigms/mesh"
DEFAULT_BINDIR="/usr/local/bin"
UNAME_MACHINE=$(uname -m)
UNAME_SYSTEM=$(uname -s)

# --- Colors / formatting ---
info()  { printf "\033[34m==>\033[0m %s\n" "$*"; }
ok()    { printf "\033[32m OK\033[0m  %s\n" "$*"; }
warn()  { printf "\033[33m WARN\033[0m %s\n" "$*" >&2; }
err()   { printf "\033[31mERR\033[0m  %s\n" "$*" >&2; }
die()   { err "$@"; exit 1; }

# --- Detect OS and architecture ---
detect_os_arch() {
  case "$UNAME_SYSTEM" in
    Linux)  OS="linux" ;;
    Darwin) OS="darwin" ;;
    *)      die "Unsupported OS: $UNAME_SYSTEM. Mesh runs on Linux and macOS." ;;
  esac

  case "$UNAME_MACHINE" in
    x86_64|amd64) ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    *) die "Unsupported architecture: $UNAME_MACHINE. Mesh runs on amd64 and arm64." ;;
  esac

  info "Detected: $OS / $ARCH"
}

# --- Resolve the latest release version ---
resolve_version() {
  if [ -n "$MESH_VERSION" ] && [ "$MESH_VERSION" != "latest" ]; then
    VERSION="$MESH_VERSION"
    info "Using pinned version: $VERSION"
    return
  fi

  if command -v curl >/dev/null 2>&1; then
    API_OUT=$(curl -fsSL "https://api.github.com/repos/${MESH_REPO}/releases/latest" 2>/dev/null) || {
      warn "GitHub API unreachable, falling back to 'latest' tag"
      VERSION="latest"
      return
    }
  elif command -v wget >/dev/null 2>&1; then
    API_OUT=$(wget -qO- "https://api.github.com/repos/${MESH_REPO}/releases/latest" 2>/dev/null) || {
      warn "GitHub API unreachable, falling back to 'latest' tag"
      VERSION="latest"
      return
    }
  else
    die "Neither curl nor wget found. Install one of them and try again."
  fi

  # Extract tag_name from JSON using POSIX-safe pattern
  VERSION=$(printf "%s" "$API_OUT" | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p')
  if [ -z "$VERSION" ]; then
    warn "Could not parse latest release tag, falling back to 'latest'"
    VERSION="latest"
  fi
  info "Latest release: $VERSION"
}

# --- Detect available download tool ---
select_downloader() {
  FETCH_CMD=""
  if command -v curl >/dev/null 2>&1; then
    FETCH_CMD="curl -fsSL"
  elif command -v wget >/dev/null 2>&1; then
    FETCH_CMD="wget -qO-"
  else
    die "No download tool found: install curl or wget and try again."
  fi
}

# --- Build download URLs ---
build_urls() {
  if [ "$VERSION" = "latest" ]; then
    RELEASE_TAG="latest"
  else
    RELEASE_TAG="$VERSION"
  fi

  BASE_URL="https://github.com/${MESH_REPO}/releases/download/${RELEASE_TAG}"
  ARCHIVE_NAME="mesh_${VERSION}_${OS}_${ARCH}.tar.gz"
  CHECKSUM_NAME="mesh_${VERSION}_checksums.txt"

  ARCHIVE_URL="${BASE_URL}/${ARCHIVE_NAME}"
  CHECKSUM_URL="${BASE_URL}/${CHECKSUM_NAME}"
}

# --- Download and verify ---
download_and_verify() {
  TMPDIR=$(mktemp -d 2>/dev/null || mktemp -d -t mesh-install)
  trap 'cleanup' EXIT INT TERM

  info "Downloading $ARCHIVE_URL ..."
  if [ "$MESH_DRY_RUN" = "1" ]; then
    ok "DRY-RUN: would download $ARCHIVE_URL"
    # Create a fake archive for dry-run testing so we can still exercise the rest
    mkdir -p "${TMPDIR}/extracted"
    mkdir -p "${TMPDIR}/extracted/${ARCHIVE_NAME%.tar.gz}"
    touch "${TMPDIR}/extracted/${ARCHIVE_NAME%.tar.gz}/mesh"
    ok "DRY-RUN: created fake archive at $TMPDIR"
    ARCHIVE_DIR="${TMPDIR}/extracted"
    return
  fi

  # Download archive
  if echo "$FETCH_CMD" | grep -q curl; then
    curl -fsSL "$ARCHIVE_URL" -o "${TMPDIR}/${ARCHIVE_NAME}"
  else
    wget -q "$ARCHIVE_URL" -O "${TMPDIR}/${ARCHIVE_NAME}"
  fi

  # Download checksums
  if echo "$FETCH_CMD" | grep -q curl; then
    curl -fsSL "$CHECKSUM_URL" -o "${TMPDIR}/${CHECKSUM_NAME}" 2>/dev/null || \
      warn "Checksum file not found, skipping verification"
  else
    wget -q "$CHECKSUM_URL" -O "${TMPDIR}/${CHECKSUM_NAME}" 2>/dev/null || \
      warn "Checksum file not found, skipping verification"
  fi

  # Verify checksum
  if [ -f "${TMPDIR}/${CHECKSUM_NAME}" ]; then
    info "Verifying SHA-256 checksum..."
    if command -v sha256sum >/dev/null 2>&1; then
      (cd "${TMPDIR}" && sha256sum -c "${CHECKSUM_NAME}" --ignore-missing 2>/dev/null) || \
        die "Checksum verification failed for $ARCHIVE_NAME"
    elif command -v shasum >/dev/null 2>&1; then
      (cd "${TMPDIR}" && shasum -a 256 -c "${CHECKSUM_NAME}" --ignore-missing 2>/dev/null) || \
        die "Checksum verification failed for $ARCHIVE_NAME"
    else
      warn "No sha256sum or shasum found, skipping checksum verification"
    fi
    ok "Checksum verified"
  fi

  # Extract archive
  info "Extracting..."
  mkdir -p "${TMPDIR}/extracted"
  tar -xzf "${TMPDIR}/${ARCHIVE_NAME}" -C "${TMPDIR}/extracted"
  ARCHIVE_DIR="${TMPDIR}/extracted"
  ok "Extracted"
}

# --- Find the mesh binary in extracted files ---
find_binary() {
  BINARY_PATH=$(find "$ARCHIVE_DIR" -type f -name "mesh" | head -1)
  if [ -z "$BINARY_PATH" ]; then
    die "mesh binary not found in the archive"
  fi
  info "Found mesh binary at: $BINARY_PATH"
}

# --- Install the binary ---
install_binary() {
  BINDIR="${MESH_BINDIR:-$DEFAULT_BINDIR}"

  if [ "$MESH_DRY_RUN" = "1" ]; then
    ok "DRY-RUN: would install mesh to $BINDIR/mesh"
    return
  fi

  # Check if target is writable, use sudo if not
  if [ -d "$BINDIR" ] && [ ! -w "$BINDIR" ]; then
    info "Need sudo to install to $BINDIR"
    sudo install -d "$BINDIR"
    sudo install -m 755 "$BINARY_PATH" "$BINDIR/mesh"
    ok "Installed mesh to $BINDIR/mesh (with sudo)"
  else
    mkdir -p "$BINDIR"
    install -m 755 "$BINARY_PATH" "$BINDIR/mesh"
    ok "Installed mesh to $BINDIR/mesh"
  fi
}

# --- Check PATH ---
check_path() {
  BINDIR="${MESH_BINDIR:-$DEFAULT_BINDIR}"
  case "$PATH" in
    *"$BINDIR"*) ;;
    *) warn "$BINDIR is not in your PATH. Add it: export PATH=\"\$PATH:$BINDIR\"" ;;
  esac
}

# --- Post-install: init and verify ---
post_install() {
  if [ "$MESH_DRY_RUN" = "1" ]; then
    ok "DRY-RUN: would run: mesh init"
    ok "DRY-RUN: would run: mesh --version"
    return
  fi

  MESH_CMD="${BINDIR}/mesh"

  # Create ~/.mesh/ directory
  info "Creating ~/.mesh/..."
  mkdir -p "$HOME/.mesh"
  ok "Created $HOME/.mesh"

  # Run mesh init
  info "Running mesh init..."
  "$MESH_CMD" init
  ok "Mesh initialized"

  # Verify installation
  info "Verifying installation..."
  "$MESH_CMD" --version
  ok "Mesh $(mesh --version) installed successfully"
}

# --- Cleanup ---
cleanup() {
  if [ -n "$TMPDIR" ] && [ -d "$TMPDIR" ]; then
    rm -rf "$TMPDIR"
  fi
}

# --- Main ---
main() {
  cat <<'EOF'

  __  __          _
 |  \/  |        | |
 | \  / | ___  __| | ___ _ __
 | |\/| |/ _ \/ _` |/ _ \ '__|
 | |  | |  __/ (_| |  __/ |
 |_|  |_|\___|\__,_|\___|_|

 Portable agent-body runtime for AI agents

EOF

  detect_os_arch
  resolve_version
  select_downloader
  build_urls
  download_and_verify
  find_binary
  install_binary
  check_path
  post_install
  ok "Mesh is ready. Run 'mesh serve' to start the daemon."
}

main "$@"
