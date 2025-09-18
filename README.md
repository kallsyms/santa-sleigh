# Santa Sleigh

Santa Sleigh is a cross-platform daemon that ships telemetry bundles collected by the [Santa](https://github.com/google/santa) endpoint security agent to an S3-compatible object store. It watches a queue directory for new files, uploads them to S3, and archives successfully transferred payloads.

## Features

- Runs as a long-lived daemon on macOS (launchd) and Linux (systemd).
- Configurable via TOML with sensible per-platform defaults.
- Concurrent uploads with retry policy and staging safeguards to prevent duplicate processing.
- Supports AWS credentials from the config file, environment variables, or ambient providers (IMDS, profile files).
- Packaging helpers for `.pkg` (macOS) and `.deb` (Debian-based Linux).
- GitHub Actions workflows to build, package, and publish release artifacts.

## Configuration

Santa Sleigh reads configuration from a TOML file. The default location is:

- macOS: `/Library/Application Support/SantaSleigh/config.toml`
- Linux: `/etc/santa-sleigh/config.toml`

A sample file is provided at `configs/santa-sleigh.sample.toml`. Key sections include:

- `[aws]` – bucket name, region, optional credentials and custom endpoints.
- `[paths]` – queue, archive, and log directories.
- `[logging]` – log level and log file location.
- `[upload]` – concurrency, polling interval, retry count, staging suffix.

Environment variables (`AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_SESSION_TOKEN`, `AWS_REGION`) are applied automatically when values are omitted from the configuration file.

## Building From Source

The project targets Go 1.22 or later.

```bash
make build          # build for the current platform into dist/santa-sleigh
make build-linux    # cross-compile for linux/amd64
make build-macos    # cross-compile for darwin/arm64 and darwin/amd64
```

Each build embeds the version derived from `git describe` (override with `VERSION=<tag>`).

## Running as a Daemon

### macOS (launchd)

1. Build or download the `santa-sleigh` binary.
2. Run the installer script as root:
   ```bash
   sudo scripts/macos/install.sh /path/to/santa-sleigh
   ```
3. Edit `/Library/Application Support/SantaSleigh/config.toml` with your S3 settings.
4. Logs are written to `/Library/Logs/SantaSleigh/` and the configured log file.

The launchd property list lives at `/Library/LaunchDaemons/com.kallsyms.santa-sleigh.plist`.

### Linux (systemd)

1. Build or download the `santa-sleigh` binary.
2. Run the installer script as root:
   ```bash
   sudo scripts/linux/install.sh /path/to/santa-sleigh
   ```
3. Update `/etc/santa-sleigh/config.toml` and optionally `/etc/default/santa-sleigh` for environment overrides.
4. Manage the service with `systemctl status|start|stop santa-sleigh`.

The systemd unit is installed to `/lib/systemd/system/santa-sleigh.service` and runs under the dedicated `santa-sleigh` service account.

## Packaging

### macOS `.pkg`

1. Cross-compile a macOS binary (arm64 or universal).
2. Run the packaging helper:
   ```bash
   packaging/macos/build_pkg.sh 1.0.0 dist/darwin/santa-sleigh
   ```
3. The package is emitted to `dist/macos/santa-sleigh-1.0.0.pkg` and installs the binary, launchd plist, and a sample config. The post-install script bootstraps the launchd service.

### Debian `.deb`

1. Cross-compile a linux/amd64 binary: `make build-linux`.
2. Build the package:
   ```bash
   packaging/linux/build_deb.sh 1.0.0 amd64 dist/linux/santa-sleigh
   ```
3. The resulting `dist/linux/santa-sleigh_1.0.0_amd64.deb` installs the binary, systemd unit, sample config, and service account scripts. Post-install hooks create the `santa-sleigh` user and restart the service if already enabled.

## Development Notes

- `go test ./...` exercises the codebase.
- The daemon logs to both stdout and the configured log file using structured JSON output (`log/slog`).
- Uploaded files are staged with a suffix (default `.partial`) to avoid duplicate work and archived on success.
- Version information can be overridden with `-ldflags "-X github.com/kallsyms/santa-sleigh/internal/daemon.version=<value>"`.

## Repository Layout

```
cmd/                Command entrypoint
internal/config     TOML configuration handling
internal/daemon     Daemon orchestration
internal/logging    Logging setup
internal/uploader   S3 uploader abstraction for AWS SDK v2
configs/            Sample configuration files
scripts/            Convenience installers for macOS/Linux
packaging/          Launchd, systemd, and packaging helpers
.github/workflows   CI workflows for build and release
```

