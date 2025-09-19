cask "santa-sleigh" do
  version "0.0.4"
  sha256 "32471a9119e36c6c49c0ca427618808e11e3eb19c5fdac6edafaad4d16656000"

  url "https://github.com/kallsyms/santa-sleigh/releases/download/v0.0.4/santa-sleigh-0.0.4.pkg"
  name "Santa Sleigh"
  desc "Telemetry uploader for Santa endpoint security agent"
  homepage "https://github.com/kallsyms/santa-sleigh"

  pkg "santa-sleigh-0.0.4.pkg"

  uninstall launchctl: "com.kallsyms.santa-sleigh",
            pkgutil:   "com.kallsyms.santa-sleigh"
end
