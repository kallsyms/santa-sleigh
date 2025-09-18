#!/usr/bin/env bash
set -euo pipefail

if [[ $EUID -ne 0 ]]; then
  echo "This installer must be run as root" >&2
  exit 1
fi

PROJECT_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
BINARY_SRC="${1:-$PROJECT_ROOT/dist/darwin/santa-sleigh}" 
CONFIG_SRC="$PROJECT_ROOT/configs/santa-sleigh.sample.toml"
PLIST_SRC="$PROJECT_ROOT/packaging/macos/com.kallsyms.santa-sleigh.plist"

if [[ ! -x "$BINARY_SRC" ]]; then
  echo "Binary $BINARY_SRC does not exist or is not executable" >&2
  exit 1
fi

install -d -m 0755 /usr/local/bin
install -m 0755 "$BINARY_SRC" /usr/local/bin/santa-sleigh

install -d -m 0755 "/Library/Application Support/SantaSleigh"
if [[ ! -f "/Library/Application Support/SantaSleigh/config.toml" ]]; then
  install -m 0640 "$CONFIG_SRC" "/Library/Application Support/SantaSleigh/config.toml"
fi
install -d -m 0755 /Library/Logs/SantaSleigh

install -m 0644 "$PLIST_SRC" /Library/LaunchDaemons/com.kallsyms.santa-sleigh.plist
chmod 0644 /Library/LaunchDaemons/com.kallsyms.santa-sleigh.plist

launchctl bootout system /Library/LaunchDaemons/com.kallsyms.santa-sleigh.plist 2>/dev/null || true
launchctl bootstrap system /Library/LaunchDaemons/com.kallsyms.santa-sleigh.plist
launchctl enable system/com.kallsyms.santa-sleigh

cat <<'EONOTE'
santa-sleigh installed.
Edit /Library/Application Support/SantaSleigh/config.toml to configure credentials.
Logs: /Library/Logs/SantaSleigh
EONOTE
