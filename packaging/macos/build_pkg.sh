#!/usr/bin/env bash
set -euo pipefail

if [[ $# -lt 2 ]]; then
  echo "Usage: $0 <version> <path-to-universal-or-arch-binary>" >&2
  echo "Example: $0 1.0.0 dist/darwin/arm64/santa-sleigh" >&2
  exit 1
fi

VERSION="$1"
BINARY_SRC="$2"
MACOS_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$MACOS_DIR/../.." && pwd)"
WORKDIR="$(mktemp -d)"
trap 'rm -rf "$WORKDIR"' EXIT

if [[ ! -x "$BINARY_SRC" ]]; then
  echo "Binary $BINARY_SRC not found or not executable" >&2
  exit 1
fi

PAYLOAD="$WORKDIR/payload"
mkdir -p "$PAYLOAD/usr/local/bin"
install -m 0755 "$BINARY_SRC" "$PAYLOAD/usr/local/bin/santa-sleigh"

mkdir -p "$PAYLOAD/Library/Application Support/SantaSleigh"
install -m 0644 "$REPO_ROOT/configs/santa-sleigh.sample.toml" "$PAYLOAD/Library/Application Support/SantaSleigh/config.toml.sample"

mkdir -p "$PAYLOAD/Library/LaunchDaemons"
install -m 0644 "$MACOS_DIR/com.kallsyms.santa-sleigh.plist" "$PAYLOAD/Library/LaunchDaemons/com.kallsyms.santa-sleigh.plist"

SCRIPT_DIR="$WORKDIR/scripts"
mkdir -p "$SCRIPT_DIR"
install -m 0755 "$MACOS_DIR/scripts/postinstall" "$SCRIPT_DIR/postinstall"

OUTPUT_DIR="$REPO_ROOT/dist/macos"
mkdir -p "$OUTPUT_DIR"
PKG_PATH="$OUTPUT_DIR/santa-sleigh-${VERSION}.pkg"

pkgbuild \
  --root "$PAYLOAD" \
  --scripts "$SCRIPT_DIR" \
  --identifier "com.kallsyms.santa-sleigh" \
  --version "$VERSION" \
  --install-location "/" \
  "$PKG_PATH"

echo "Created $PKG_PATH"
