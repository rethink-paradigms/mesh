#!/bin/sh
# Validate that the inline systemd unit heredoc in install.sh
# matches the canonical scripts/mesh-daemon.service file.
#
# Exit codes:
#   0 — match (or no diff tool available)
#   1 — mismatch
#   2 — missing required files

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
INSTALL_SH="$SCRIPT_DIR/install.sh"
SERVICE_FILE="$SCRIPT_DIR/mesh-daemon.service"

# Verify both files exist
if [ ! -f "$INSTALL_SH" ]; then
  echo "ERROR: $INSTALL_SH not found" >&2
  exit 2
fi
if [ ! -f "$SERVICE_FILE" ]; then
  echo "ERROR: $SERVICE_FILE not found" >&2
  exit 2
fi

# Extract the heredoc from install.sh:
# Find lines between the 'cat > ... << 'SERVICE'' line and the terminating 'SERVICE' line.
# Works with any indentation (inside if/else blocks).
HEREDOC=$(awk '
  /cat > \/etc\/systemd\/system\/mesh-daemon\.service << .SERVICE.$/ { capture=1; next }
  /^[[:space:]]*SERVICE$/ && capture { capture=0; exit }
  capture { print }
' "$INSTALL_SH")

if [ -z "$HEREDOC" ]; then
  echo "ERROR: could not extract systemd unit heredoc from install.sh" >&2
  exit 2
fi

# Diff against the canonical file
if echo "$HEREDOC" | diff -q - "$SERVICE_FILE" >/dev/null 2>&1; then
  echo "OK: inline heredoc matches scripts/mesh-daemon.service"
  exit 0
else
  echo "FAIL: inline heredoc in install.sh does NOT match scripts/mesh-daemon.service" >&2
  echo "" >&2
  echo "Differences:" >&2
  echo "$HEREDOC" | diff -u - "$SERVICE_FILE" 2>&1 || true
  echo "" >&2
  echo "Fix: update the heredoc fallback in install.sh to match scripts/mesh-daemon.service" >&2
  echo "Or: update scripts/mesh-daemon.service if that is the intended source of truth." >&2
  exit 1
fi
