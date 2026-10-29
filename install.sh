#!/usr/bin/env bash
# liteharness-install — one-line installer for LiteHarness
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/justwaters/LiteHarness/master/install.sh | bash
#
# Detects OS + architecture, downloads the matching binary from GitHub
# releases, and installs it into ~/.local/bin (or $PREFIX/bin).

set -euo pipefail

REPO="justwaters/LiteHarness"
INSTALL_DIR="${PREFIX:-$HOME/.local/bin}"
LITEHARNESS_BIN="liteharness"

# ---------- helpers ----------
fail() { echo "error: $*" >&2; exit 1; }
has()  { command -v "$1" >/dev/null 2>&1; }

need_cmd() {
  if ! has "$1"; then
    fail "$1 is required but not installed"
  fi
}

# ---------- detect platform ----------
detect_os() {
  case "$(uname -s)" in
    Linux)  echo "linux" ;;
    Darwin) echo "darwin" ;;
    *)      fail "unsupported OS: $(uname -s)" ;;
  esac
}

detect_arch() {
  local arch
  arch="$(uname -m)"
  case "$arch" in
    x86_64|amd64)  echo "amd64" ;;
    i386|i686|386) echo "386" ;;
    arm64|aarch64) echo "arm64" ;;
    armv7l|armv7)  echo "arm-v7" ;;
    *)             fail "unsupported architecture: $arch" ;;
  esac
}

# ---------- main ----------
OS="$(detect_os)"
ARCH="$(detect_arch)"

case "$OS" in
  linux)   BIN="liteharness-linux-$ARCH" ;;
  darwin)  BIN="liteharness-darwin-$ARCH" ;;
esac

# Need curl or wget
if has curl; then
  DL="curl -fsSL"
elif has wget; then
  DL="wget -qO-"
else
  fail "curl or wget is required"
fi

# Find the latest release tag (fall back to "master" if no releases exist)
LATEST_TAG="$($DL "https://api.github.com/repos/$REPO/releases/latest" 2>/dev/null \
  | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/' || true)"

if [ -z "$LATEST_TAG" ]; then
  LATEST_TAG="master"
  echo "No GitHub releases found; downloading from master branch."
fi

URL="https://github.com/$REPO/releases/download/$LATEST_TAG/$BIN"
if [ "$LATEST_TAG" = "master" ]; then
  URL="https://raw.githubusercontent.com/$REPO/master/$BIN"
fi

echo "Installing LiteHarness ($BIN) -> $INSTALL_DIR"
mkdir -p "$INSTALL_DIR"

$DL "$URL" > "$INSTALL_DIR/$LITEHARNESS_BIN"
chmod +x "$INSTALL_DIR/$LITEHARNESS_BIN"

echo "Done. Run: $LITEHARNESS_BIN"
echo ""
echo "If $INSTALL_DIR is not on your PATH, add it:"
echo "  export PATH=\"\$PATH:$INSTALL_DIR\""
