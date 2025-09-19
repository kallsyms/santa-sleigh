# Santa Sleigh

Santa Sleigh is a cross-platform daemon that ships telemetry bundles collected by the [Santa](https://github.com/google/santa) endpoint security agent to an S3-compatible object store.
It tails Santa's JSON file output and uploads chunks to S3.
For other Santa-compatible daemons, native parquet upload is also supported.

## Configuration

Santa Sleigh reads its configuration from a TOML file. The default location is:

- macOS: `/Library/Application Support/SantaSleigh/config.toml`
- Linux: `/etc/santa-sleigh/config.toml`

A sample file is provided at `configs/santa-sleigh.sample.toml`.

### Santa Configuration

Santa must be configured to emit JSON telemetry. See the [sample Santa configuration profile](./configs/sample.mobileconfig) for reference on how to configure Santa for this.

### Upload Modes

- **parquet** (default): Treats `upload.queue` as a directory of pre-segmented parquet bundles. Files are uploaded concurrently and deleted after a successful transfer.
- **json**: Treats `upload.queue` as the path to the live JSON telemetry file written by Santa (e.g. `/var/log/santa/telemetry.jsonl`). Santa Sleigh tails the file and, every `upload.json_max_interval` (default 5 minutes) or after `upload.json_max_bytes` (default 10 MB) comprising new data, compresses, uploads, rotates the file on disk, and deletes the processed chunk once the upload succeeds.

## Installation

### macOS
The repository doubles as a Homebrew tap once a tagged release lands:

```bash
brew tap kallsyms/santa-sleigh https://github.com/kallsyms/santa-sleigh
brew install --cask kallsyms/santa-sleigh/santa-sleigh
```

### Linux
Grab the .deb from the latest [release](https://github.com/kallsyms/santa-sleigh/releases).

## Building From Source

The project targets Go 1.22 or later.

```bash
make build          # build for the current platform into dist/santa-sleigh
make build-linux    # cross-compile for linux/amd64
make build-macos    # cross-compile for darwin/arm64 and darwin/amd64
```

Each build embeds the version derived from `git describe` (override with `VERSION=<tag>`).

### Running as a Daemon

#### macOS

1. Build or download the `santa-sleigh` binary.
2. Run the installer script as root:
   ```bash
   sudo scripts/macos/install.sh /path/to/santa-sleigh
   ```
3. Edit `/Library/Application Support/SantaSleigh/config.toml` with your S3 settings.
4. Logs are written to `/Library/Logs/SantaSleigh/` and the configured log file.

The launchd property list lives at `/Library/LaunchDaemons/com.kallsyms.santa-sleigh.plist`.

#### Linux (systemd)

1. Build or download the `santa-sleigh` binary.
2. Run the installer script as root:
   ```bash
   sudo scripts/linux/install.sh /path/to/santa-sleigh
   ```
3. Update `/etc/santa-sleigh/config.toml` and optionally `/etc/default/santa-sleigh` for environment overrides.
4. Manage the service with `systemctl status|start|stop santa-sleigh`.

The systemd unit is installed to `/lib/systemd/system/santa-sleigh.service` and runs under the dedicated `santa-sleigh` service account.
