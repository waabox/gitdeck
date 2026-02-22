#!/usr/bin/env bash
set -euo pipefail

REPO="https://github.com/waabox/gitdeck.git"
INSTALL_DIR="/usr/local/bin"
TMP_DIR="$(mktemp -d)"
BINARY="gitdeck"
MIN_GO_VERSION="1.24"

cleanup() {
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT

info()  { printf "\033[1;34m==>\033[0m %s\n" "$1"; }
error() { printf "\033[1;31merror:\033[0m %s\n" "$1" >&2; exit 1; }

# --- Pre-flight checks -------------------------------------------------------

if ! command -v git &>/dev/null; then
  error "git is required but not installed."
fi

if ! command -v go &>/dev/null; then
  error "go is required but not installed. Get it at https://go.dev/dl/"
fi

go_version="$(go version | grep -oE 'go[0-9]+\.[0-9]+' | head -1 | sed 's/go//')"
if [ "$(printf '%s\n' "$MIN_GO_VERSION" "$go_version" | sort -V | head -1)" != "$MIN_GO_VERSION" ]; then
  error "Go >= $MIN_GO_VERSION is required (found $go_version)."
fi

# --- Clone & build ------------------------------------------------------------

info "Cloning gitdeck into $TMP_DIR ..."
git clone --depth 1 "$REPO" "$TMP_DIR/gitdeck"

info "Building gitdeck ..."
cd "$TMP_DIR/gitdeck"
go build -o "$BINARY" ./cmd/gitdeck

# --- Install ------------------------------------------------------------------

info "Installing gitdeck to $INSTALL_DIR ..."
if [ -w "$INSTALL_DIR" ]; then
  mv "$BINARY" "$INSTALL_DIR/$BINARY"
else
  sudo mv "$BINARY" "$INSTALL_DIR/$BINARY"
fi

# macOS: remove quarantine attribute if present
if [ "$(uname)" = "Darwin" ] && command -v xattr &>/dev/null; then
  xattr -d com.apple.quarantine "$INSTALL_DIR/$BINARY" 2>/dev/null || true
fi

info "gitdeck installed successfully! Run 'gitdeck' inside any git repo."
