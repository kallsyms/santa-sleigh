class SantaSleigh < Formula
  desc "Telemetry uploader for Santa endpoint security agent"
  homepage "https://github.com/kallsyms/santa-sleigh"
  url "https://github.com/kallsyms/santa-sleigh/archive/refs/tags/v0.0.3.tar.gz"
  sha256 "e341159edbb8fb7cb9098411ba927332e745cb762af46e48cb18db1e5a9618a1"
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
