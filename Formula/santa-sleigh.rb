class SantaSleigh < Formula
  desc "Telemetry uploader for Santa endpoint security agent"
  homepage "https://github.com/kallsyms/santa-sleigh"
  url "https://github.com/kallsyms/santa-sleigh/archive/refs/tags/v0.0.5.tar.gz"
  sha256 "03afcbb401c11a065d32f526a8b3ae7be401f02928da5055eca37aae7aff74f2"
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

  def caveats
    <<~EOS
      Santa Sleigh needs root privileges to read Santa telemetry logs and write to /var/log.
      Start the service with sudo so it runs as a LaunchDaemon:

        sudo brew services start #{tap}/#{name}

      Logs are written to /var/log/santa-sleigh/santa-sleigh.log by default. Update
      #{etc}/santa-sleigh/config.toml if you want a different location.
    EOS
  end

  test do
    assert_match "Usage of", shell_output("#{bin}/santa-sleigh -h 2>&1")
  end
end
