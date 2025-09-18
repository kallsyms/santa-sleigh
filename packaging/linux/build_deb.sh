#!/usr/bin/env bash
set -euo pipefail

if [[ $# -lt 3 ]]; then
  echo "Usage: $0 <version> <arch> <path-to-linux-binary>" >&2
  echo "Example: $0 1.0.0 amd64 dist/linux/santa-sleigh" >&2
  exit 1
fi

VERSION="$1"
ARCH="$2"
BINARY_SRC="$3"
LINUX_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$LINUX_DIR/../.." && pwd)"

if [[ ! -x "$BINARY_SRC" ]]; then
  echo "Binary $BINARY_SRC not found or not executable" >&2
  exit 1
fi

WORKDIR="$(mktemp -d)"
trap 'rm -rf "$WORKDIR"' EXIT
PKGDIR="$WORKDIR/santa-sleigh_${VERSION}_${ARCH}"

mkdir -p "$PKGDIR/DEBIAN"
mkdir -p "$PKGDIR/usr/local/bin"
mkdir -p "$PKGDIR/etc/santa-sleigh"
mkdir -p "$PKGDIR/etc/default"
mkdir -p "$PKGDIR/lib/systemd/system"

install -m 0755 "$BINARY_SRC" "$PKGDIR/usr/local/bin/santa-sleigh"
install -m 0644 "$REPO_ROOT/configs/santa-sleigh.sample.toml" "$PKGDIR/etc/santa-sleigh/config.toml.sample"
install -m 0644 "$REPO_ROOT/configs/linux/santa-sleigh.default" "$PKGDIR/etc/default/santa-sleigh.sample"
install -m 0644 "$LINUX_DIR/systemd/santa-sleigh.service" "$PKGDIR/lib/systemd/system/santa-sleigh.service"

install -m 0755 "$LINUX_DIR/debian/postinst" "$PKGDIR/DEBIAN/postinst"
install -m 0755 "$LINUX_DIR/debian/prerm" "$PKGDIR/DEBIAN/prerm"
install -m 0755 "$LINUX_DIR/debian/postrm" "$PKGDIR/DEBIAN/postrm"

cat > "$PKGDIR/DEBIAN/control" <<CONTROL
Package: santa-sleigh
Version: $VERSION
Section: admin
Priority: optional
Architecture: $ARCH
Maintainer: Kallsyms <ops@kallsyms.invalid>
Depends: adduser, systemd
Description: Santa telemetry uploader daemon
 Santa Sleigh ships telemetry bundles from Santa endpoints to S3.
CONTROL

OUTPUT_DIR="$REPO_ROOT/dist/linux"
mkdir -p "$OUTPUT_DIR"

dpkg-deb --build "$PKGDIR" "$OUTPUT_DIR/santa-sleigh_${VERSION}_${ARCH}.deb"

echo "Created $OUTPUT_DIR/santa-sleigh_${VERSION}_${ARCH}.deb"
