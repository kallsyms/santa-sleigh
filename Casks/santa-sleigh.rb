cask "santa-sleigh" do
  version "0.0.5"
  sha256 "3e322e5099a4540bd2b7d390a62ab945d4276d641f3229fa88369b2773c0c2be"

  url "https://github.com/kallsyms/santa-sleigh/releases/download/v0.0.5/santa-sleigh-0.0.5.pkg"
  name "Santa Sleigh"
  desc "Telemetry uploader for Santa endpoint security agent"
  homepage "https://github.com/kallsyms/santa-sleigh"

  pkg "santa-sleigh-0.0.5.pkg"

  uninstall launchctl: "com.kallsyms.santa-sleigh",
            pkgutil:   "com.kallsyms.santa-sleigh"
end
