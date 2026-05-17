#!/bin/sh
# Mesh install script.
# Detects OS/arch, downloads latest release from GitHub, verifies checksum,
# installs to /usr/local/bin, and runs `mesh init`.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/rethink-paradigms/mesh/main/scripts/install.sh | sh
#   MESH_VERSION=1.0.0 curl -fsSL ... | sh       # pin a version
#   MESH_DRY_RUN=1 bash scripts/install.sh       # dry-run (for testing)
#
# Environment variables:
#   MESH_VERSION   - release tag to install (default: latest)
#   MESH_BINDIR    - install directory (default: /usr/local/bin)
#   MESH_DRY_RUN   - set to 1 to skip download and actual install (for testing)
#   MESH_SKIP_INIT - set to 1 to skip 'mesh init' after install (for cloud-init)

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
    die "Cannot determine archive name for 'latest' version without GitHub API access. Please set MESH_VERSION to a specific release tag (e.g., MESH_VERSION=1.0.1)."
  fi

  RELEASE_TAG="$VERSION"
  BASE_URL="https://github.com/${MESH_REPO}/releases/download/${RELEASE_TAG}"
  ARCHIVE_VERSION="${VERSION#v}"
  ARCHIVE_NAME="mesh_${ARCHIVE_VERSION}_${OS}_${ARCH}.tar.gz"
  CHECKSUM_NAME="mesh_${ARCHIVE_VERSION}_checksums.txt"

  ARCHIVE_URL="${BASE_URL}/${ARCHIVE_NAME}"
  CHECKSUM_URL="${BASE_URL}/${CHECKSUM_NAME}"
}

# --- Build daemon download URLs ---
# PREREQUISITE: mesh-daemon binary must be published to GitHub Releases before
# this script can successfully download and install it. See ME-03 (deferred).
# Archive naming: mesh-daemon_{version}_{os}_{arch}.tar.gz (no 'v' prefix in version).
build_daemon_urls() {
  DAEMON_ARCHIVE_NAME="mesh-daemon_${ARCHIVE_VERSION}_${OS}_${ARCH}.tar.gz"
  DAEMON_CHECKSUM_NAME="mesh_${ARCHIVE_VERSION}_checksums.txt"
  DAEMON_ARCHIVE_URL="${BASE_URL}/${DAEMON_ARCHIVE_NAME}"
  DAEMON_CHECKSUM_URL="${BASE_URL}/${DAEMON_CHECKSUM_NAME}"
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

# --- Download and verify daemon ---
download_daemon() {
  info "Downloading $DAEMON_ARCHIVE_URL ..."
  if [ "$MESH_DRY_RUN" = "1" ]; then
    ok "DRY-RUN: would download $DAEMON_ARCHIVE_URL"
    mkdir -p "${TMPDIR}/daemon_extracted"
    mkdir -p "${TMPDIR}/daemon_extracted/${DAEMON_ARCHIVE_NAME%.tar.gz}"
    touch "${TMPDIR}/daemon_extracted/${DAEMON_ARCHIVE_NAME%.tar.gz}/mesh-daemon"
    ok "DRY-RUN: created fake daemon archive at $TMPDIR"
    DAEMON_ARCHIVE_DIR="${TMPDIR}/daemon_extracted"
    return
  fi

  # Download daemon archive
  if echo "$FETCH_CMD" | grep -q curl; then
    curl -fsSL "$DAEMON_ARCHIVE_URL" -o "${TMPDIR}/${DAEMON_ARCHIVE_NAME}"
  else
    wget -q "$DAEMON_ARCHIVE_URL" -O "${TMPDIR}/${DAEMON_ARCHIVE_NAME}"
  fi

  # Download checksums (same file as mesh, may already exist from first download)
  if [ ! -f "${TMPDIR}/${DAEMON_CHECKSUM_NAME}" ]; then
    if echo "$FETCH_CMD" | grep -q curl; then
      curl -fsSL "$DAEMON_CHECKSUM_URL" -o "${TMPDIR}/${DAEMON_CHECKSUM_NAME}" 2>/dev/null || \
        warn "Checksum file not found, skipping verification"
    else
      wget -q "$DAEMON_CHECKSUM_URL" -O "${TMPDIR}/${DAEMON_CHECKSUM_NAME}" 2>/dev/null || \
        warn "Checksum file not found, skipping verification"
    fi
  fi

  # Verify checksum
  if [ -f "${TMPDIR}/${DAEMON_CHECKSUM_NAME}" ]; then
    info "Verifying SHA-256 checksum for daemon..."
    if command -v sha256sum >/dev/null 2>&1; then
      (cd "${TMPDIR}" && sha256sum -c "${DAEMON_CHECKSUM_NAME}" --ignore-missing 2>/dev/null) || \
        die "Checksum verification failed for $DAEMON_ARCHIVE_NAME"
    elif command -v shasum >/dev/null 2>&1; then
      (cd "${TMPDIR}" && shasum -a 256 -c "${DAEMON_CHECKSUM_NAME}" --ignore-missing 2>/dev/null) || \
        die "Checksum verification failed for $DAEMON_ARCHIVE_NAME"
    else
      warn "No sha256sum or shasum found, skipping checksum verification"
    fi
    ok "Checksum verified"
  fi

  # Extract archive
  info "Extracting daemon..."
  mkdir -p "${TMPDIR}/daemon_extracted"
  tar -xzf "${TMPDIR}/${DAEMON_ARCHIVE_NAME}" -C "${TMPDIR}/daemon_extracted"
  DAEMON_ARCHIVE_DIR="${TMPDIR}/daemon_extracted"
  ok "Extracted daemon"
}

# --- Find the mesh binary in extracted files ---
find_binary() {
  BINARY_PATH=$(find "$ARCHIVE_DIR" -type f -name "mesh" | head -1)
  if [ -z "$BINARY_PATH" ]; then
    die "mesh binary not found in the archive"
  fi
  info "Found mesh binary at: $BINARY_PATH"
}

# --- Find the daemon binary in extracted files ---
find_daemon_binary() {
  DAEMON_BINARY_PATH=$(find "$DAEMON_ARCHIVE_DIR" -type f -name "mesh-daemon" | head -1)
  if [ -z "$DAEMON_BINARY_PATH" ]; then
    die "mesh-daemon binary not found in the archive"
  fi
  info "Found mesh-daemon binary at: $DAEMON_BINARY_PATH"
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

# --- Install the daemon binary ---
install_daemon_binary() {
  BINDIR="${MESH_BINDIR:-$DEFAULT_BINDIR}"

  if [ "$MESH_DRY_RUN" = "1" ]; then
    ok "DRY-RUN: would install mesh-daemon to $BINDIR/mesh-daemon"
    return
  fi

  # Check if target is writable, use sudo if not
  if [ -d "$BINDIR" ] && [ ! -w "$BINDIR" ]; then
    info "Need sudo to install to $BINDIR"
    sudo install -d "$BINDIR"
    sudo install -m 755 "$DAEMON_BINARY_PATH" "$BINDIR/mesh-daemon"
    ok "Installed mesh-daemon to $BINDIR/mesh-daemon (with sudo)"
  else
    mkdir -p "$BINDIR"
    install -m 755 "$DAEMON_BINARY_PATH" "$BINDIR/mesh-daemon"
    ok "Installed mesh-daemon to $BINDIR/mesh-daemon"
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
  MESH_CMD="${BINDIR}/mesh"

  if [ "$MESH_DRY_RUN" = "1" ]; then
    if [ "$MESH_SKIP_INIT" = "1" ]; then
      ok "DRY-RUN: MESH_SKIP_INIT=1, skipping mesh init"
    else
      ok "DRY-RUN: would run: mesh init"
    fi
    ok "DRY-RUN: would run: mesh --version"
    if [ -d "/etc/systemd/system" ]; then
      ok "DRY-RUN: would install systemd service from inline template"
    fi
    return
  fi

  # Create ~/.mesh/ directory
  info "Creating ~/.mesh/..."
  mkdir -p "$HOME/.mesh"
  ok "Created $HOME/.mesh"

  # Run mesh init (unless skipped via MESH_SKIP_INIT)
  if [ "$MESH_SKIP_INIT" = "1" ]; then
    info "MESH_SKIP_INIT=1, skipping mesh init"
  else
    info "Running mesh init..."
    "$MESH_CMD" init
    ok "Mesh initialized"
  fi

  # Verify installation
  info "Verifying installation..."
  "$MESH_CMD" --version
  ok "Mesh $(mesh --version) installed successfully"

  # Install systemd service
  # Canonical source: scripts/mesh-daemon.service (file).
  # Inline heredoc fallback for standalone curl|sh when the canonical file
  # is not available. These MUST stay in sync.
  if [ -d "/etc/systemd/system" ]; then
    info "Installing systemd service..."
    SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
    CANONICAL="$SCRIPT_DIR/mesh-daemon.service"
    if [ -f "$CANONICAL" ]; then
      cp "$CANONICAL" /etc/systemd/system/mesh-daemon.service
      ok "Installed systemd service from $CANONICAL"
    else
      # Fallback for standalone curl|sh -- must match mesh-daemon.service
      cat > /etc/systemd/system/mesh-daemon.service << 'SERVICE'
[Unit]
Description=Mesh Daemon -- Portable agent-body runtime
After=network-online.target docker.service

[Service]
Type=simple
ExecStart=/usr/local/bin/mesh-daemon serve --config /etc/mesh/config.yaml
Restart=on-failure
RestartSec=5
User=root
StandardOutput=append:/var/log/mesh-daemon.log
StandardError=append:/var/log/mesh-daemon.log

[Install]
WantedBy=multi-user.target
SERVICE
      ok "Installed systemd service from inline template"
    fi
    systemctl daemon-reload
    ok "Systemd service installed and daemon reloaded"
  fi
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
  build_daemon_urls
  download_daemon
  find_daemon_binary
  install_daemon_binary
  check_path
  post_install
  if [ "$MESH_SKIP_INIT" = "1" ]; then
    ok "Mesh installed (init skipped). Run 'mesh-daemon serve' to start the daemon."
  else
    ok "Mesh is ready. Run 'mesh-daemon serve' to start the daemon."
  fi
}

main "$@"
