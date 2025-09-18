#!/usr/bin/env bash
set -euo pipefail

if [[ $EUID -ne 0 ]]; then
  echo "This installer must be run as root" >&2
  exit 1
fi

PROJECT_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
BINARY_SRC="${1:-$PROJECT_ROOT/dist/linux/santa-sleigh}" 
SERVICE_SRC="$PROJECT_ROOT/packaging/linux/systemd/santa-sleigh.service"
CONFIG_SRC="$PROJECT_ROOT/configs/santa-sleigh.sample.toml"

if [[ ! -x "$BINARY_SRC" ]]; then
  echo "Binary $BINARY_SRC does not exist or is not executable" >&2
  exit 1
fi

id -u santa-sleigh &>/dev/null || useradd --system --no-create-home --shell /usr/sbin/nologin santa-sleigh

install -d -m 0755 /usr/local/bin
install -m 0755 "$BINARY_SRC" /usr/local/bin/santa-sleigh

install -d -m 0755 /etc/santa-sleigh
if [[ ! -f /etc/santa-sleigh/config.toml ]]; then
  install -m 0640 "$CONFIG_SRC" /etc/santa-sleigh/config.toml
fi

install -d -m 0750 /var/lib/santa-sleigh
chown santa-sleigh:santa-sleigh /var/lib/santa-sleigh

install -d -m 0750 /var/log/santa-sleigh
chown santa-sleigh:santa-sleigh /var/log/santa-sleigh

install -m 0644 "$SERVICE_SRC" /lib/systemd/system/santa-sleigh.service

systemctl daemon-reload
systemctl enable santa-sleigh.service
systemctl restart santa-sleigh.service

echo "santa-sleigh installed. Adjust /etc/santa-sleigh/config.toml and /etc/default/santa-sleigh as needed."
