#!/usr/bin/env bash
set -euo pipefail

if [[ $# -lt 1 ]]; then
  echo "Usage: $0 <version> [sha256]" >&2
  echo "Example: $0 1.2.3" >&2
  exit 1
fi

VERSION="$1"
SHA256="${2:-}"
REPO="https://github.com/kallsyms/santa-sleigh"
TAG="v${VERSION}"
TARBALL_URL="${REPO}/archive/refs/tags/${TAG}.tar.gz"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

if [[ -z "$SHA256" ]]; then
  tmpfile="$(mktemp)"
  trap 'rm -f "$tmpfile"' EXIT
  curl -Ls "$TARBALL_URL" -o "$tmpfile"
  if command -v shasum >/dev/null 2>&1; then
    SHA256="$(shasum -a 256 "$tmpfile" | awk '{print $1}')"
  else
    SHA256="$(sha256sum "$tmpfile" | awk '{print $1}')"
  fi
  rm -f "$tmpfile"
  trap - EXIT
fi

cat > "${REPO_ROOT}/Formula/santa-sleigh.rb" <<RUBY
class SantaSleigh < Formula
  desc "Telemetry uploader for Santa endpoint security agent"
  homepage "https://github.com/kallsyms/santa-sleigh"
  url "${TARBALL_URL}"
  sha256 "${SHA256}"
  license "Apache-2.0"
  head "https://github.com/kallsyms/santa-sleigh.git", branch: "main"

  depends_on "go" => :build

  def install
    build_version = build.head? ? "head" : version
    ldflags = "-s -w -X github.com/kallsyms/santa-sleigh/internal/daemon.version=#{build_version}"
    system "go", "build", *std_go_args(output: bin/"santa-sleigh", ldflags: ldflags)
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

echo "Updated Formula/santa-sleigh.rb for version ${VERSION}"
