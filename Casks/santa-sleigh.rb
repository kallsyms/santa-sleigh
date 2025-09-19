cask "santa-sleigh" do
  version "0.0.4"
  sha256 "0019dfc4b32d63c1392aa264aed2253c1e0c2fb09216f8e2cc269bbfb8bb49b5"

  url "https://github.com/kallsyms/santa-sleigh/releases/download/v0.0.4/santa-sleigh-0.0.4.pkg"
  name "Santa Sleigh"
  desc "Telemetry uploader for Santa endpoint security agent"
  homepage "https://github.com/kallsyms/santa-sleigh"

  pkg "santa-sleigh-0.0.4.pkg"

  uninstall launchctl: "com.kallsyms.santa-sleigh",
            pkgutil:   "com.kallsyms.santa-sleigh"
end
