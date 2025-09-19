#!/usr/bin/env bash
set -euo pipefail

if [[ $# -lt 1 ]]; then
  echo "Usage: $0 <version> [source_sha256] [pkg_path]" >&2
  echo "Example: $0 1.2.3" >&2
  exit 1
fi

VERSION="$1"
SOURCE_SHA="${2:-}"
PKG_PATH="${3:-}"
REPO="https://github.com/kallsyms/santa-sleigh"
TAG="v${VERSION}"
TARBALL_URL="${REPO}/archive/refs/tags/${TAG}.tar.gz"
PKG_URL="${REPO}/releases/download/${TAG}/santa-sleigh-${VERSION}.pkg"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

cleanup_tmp_tar=false
cleanup_tmp_pkg=false

if [[ -z "$SOURCE_SHA" ]]; then
  tmp_tar="$(mktemp)"
  cleanup_tmp_tar=true
  curl -Ls "$TARBALL_URL" -o "$tmp_tar"
  if command -v shasum >/dev/null 2>&1; then
    SOURCE_SHA="$(shasum -a 256 "$tmp_tar" | awk '{print $1}')"
  else
    SOURCE_SHA="$(sha256sum "$tmp_tar" | awk '{print $1}')"
  fi
fi

if [[ -z "$PKG_PATH" ]]; then
  tmp_pkg="$(mktemp)"
  cleanup_tmp_pkg=true
  curl -Ls "$PKG_URL" -o "$tmp_pkg"
  PKG_PATH="$tmp_pkg"
fi

if command -v shasum >/dev/null 2>&1; then
  PKG_SHA="$(shasum -a 256 "$PKG_PATH" | awk '{print $1}')"
else
  PKG_SHA="$(sha256sum "$PKG_PATH" | awk '{print $1}')"
fi

cat > "${REPO_ROOT}/Formula/santa-sleigh.rb" <<RUBY
class SantaSleigh < Formula
  desc "Telemetry uploader for Santa endpoint security agent"
  homepage "https://github.com/kallsyms/santa-sleigh"
  url "${TARBALL_URL}"
  sha256 "${SOURCE_SHA}"
  license "Apache-2.0"
  head "https://github.com/kallsyms/santa-sleigh.git", branch: "main"

  depends_on "go" => :build

  def install
    build_version = build.head? ? "head" : version
    ldflags = "-s -w -X github.com/kallsyms/santa-sleigh/internal/daemon.version=#{build_version}"
    system "go", "build", *std_go_args(ldflags: ldflags, output: bin/"santa-sleigh"), "./cmd/santa-sleigh"
    pkgshare.install "configs/santa-sleigh.sample.toml"
  end

  def post_install
    config_dir = etc/"santa-sleigh"
    config_dir.mkpath
    sample = pkgshare/"santa-sleigh.sample.toml"
    config_dir.install sample => "config.toml" unless (config_dir/"config.toml").exist?
    (var/"log/santa-sleigh").mkpath
  end

  service do
    run [opt_bin/"santa-sleigh", "-config", etc/"santa-sleigh/config.toml"]
    keep_alive true
    working_dir etc/"santa-sleigh"
    log_path var/"log/santa-sleigh/homebrew.log"
    error_log_path var/"log/santa-sleigh/homebrew.log"
  end

  test do
    assert_match "Usage of", shell_output("#{bin}/santa-sleigh -h 2>&1")
  end
end
RUBY

cat > "${REPO_ROOT}/Casks/santa-sleigh.rb" <<RUBY
cask "santa-sleigh" do
  version "${VERSION}"
  sha256 "${PKG_SHA}"

  url "https://github.com/kallsyms/santa-sleigh/releases/download/v${VERSION}/santa-sleigh-${VERSION}.pkg"
  name "Santa Sleigh"
  desc "Telemetry uploader for Santa endpoint security agent"
  homepage "https://github.com/kallsyms/santa-sleigh"

  pkg "santa-sleigh-${VERSION}.pkg"

  uninstall launchctl: "com.kallsyms.santa-sleigh",
            pkgutil:   "com.kallsyms.santa-sleigh"
end
RUBY

if [[ "$cleanup_tmp_tar" == true ]]; then
  rm -f "$tmp_tar"
fi
if [[ "$cleanup_tmp_pkg" == true ]]; then
  rm -f "$tmp_pkg"
fi

echo "Updated Formula/santa-sleigh.rb for version ${VERSION}"
echo "Updated Casks/santa-sleigh.rb for version ${VERSION}"
