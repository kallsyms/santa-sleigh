package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"
)

// Duration wraps time.Duration to support TOML decoding from strings like "15s".
type Duration struct {
	time.Duration
}

// UnmarshalText implements encoding.TextUnmarshaler for TOML duration fields.
func (d *Duration) UnmarshalText(text []byte) error {
	trimmed := strings.TrimSpace(string(text))
	if trimmed == "" {
		return nil
	}
	parsed, err := time.ParseDuration(trimmed)
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", trimmed, err)
	}
	d.Duration = parsed
	return nil
}

// MarshalText implements encoding.TextMarshaler to ensure round-tripping durations.
func (d Duration) MarshalText() ([]byte, error) {
	return []byte(d.Duration.String()), nil
}

// Config captures runtime configuration loaded from TOML.
type Config struct {
	AWS     AWSConfig     `toml:"aws"`
	Paths   PathsConfig   `toml:"paths"`
	Logging LoggingConfig `toml:"logging"`
	Upload  UploadConfig  `toml:"upload"`
}

// AWSConfig carries S3 connection parameters.
type AWSConfig struct {
	AccessKey    string `toml:"access_key"`
	SecretKey    string `toml:"secret_key"`
	SessionToken string `toml:"session_token"`
	Profile      string `toml:"profile"`
	Region       string `toml:"region"`
	Bucket       string `toml:"bucket"`
	CustomURL    string `toml:"custom_url"`
	UsePathStyle bool   `toml:"use_path_style"`
	S3Prefix     string `toml:"key_prefix"`
}

// PathsConfig holds filesystem locations used by the daemon.
type PathsConfig struct {
	QueueDir   string `toml:"queue_dir"`
	ArchiveDir string `toml:"archive_dir"`
	LogDir     string `toml:"log_dir"`
}

// LoggingConfig sets the log destination and verbosity.
type LoggingConfig struct {
	Level string `toml:"level"`
	File  string `toml:"file"`
}

// UploadConfig tunes the daemon behaviour for uploads.
type UploadConfig struct {
	Concurrency   int      `toml:"concurrency"`
	PollInterval  Duration `toml:"poll_interval"`
	MaxRetries    int      `toml:"max_retries"`
	StagingSuffix string   `toml:"staging_suffix"`
}

// DefaultConfigPath returns the OS-specific default config path.
func DefaultConfigPath() string {
	if runtime.GOOS == "darwin" {
		return "/Library/Application Support/SantaSleigh/config.toml"
	}
	return "/etc/santa-sleigh/config.toml"
}

// DefaultQueueDir returns the OS-specific directory for pending telemetry.
func DefaultQueueDir() string {
	if runtime.GOOS == "darwin" {
		return "/Library/Application Support/SantaSleigh/queue"
	}
	return "/var/lib/santa-sleigh/queue"
}

// DefaultArchiveDir returns the OS-specific directory for archived telemetry.
func DefaultArchiveDir() string {
	if runtime.GOOS == "darwin" {
		return "/Library/Application Support/SantaSleigh/archive"
	}
	return "/var/lib/santa-sleigh/archive"
}

// DefaultLogDir returns the OS-specific directory for logs.
func DefaultLogDir() string {
	if runtime.GOOS == "darwin" {
		return "/Library/Logs/SantaSleigh"
	}
	return "/var/log/santa-sleigh"
}

// Load reads a TOML configuration file, applies defaults, and validates the result.
func Load(overridePath string) (*Config, error) {
	path := overridePath
	if path == "" {
		path = DefaultConfigPath()
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config %s: %w", path, err)
	}
	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}
	cfg.applyDefaults(path)
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (c *Config) applyDefaults(configPath string) {
	if c.AWS.AccessKey == "" {
		c.AWS.AccessKey = os.Getenv("AWS_ACCESS_KEY_ID")
	}
	if c.AWS.SecretKey == "" {
		c.AWS.SecretKey = os.Getenv("AWS_SECRET_ACCESS_KEY")
	}
	if c.AWS.SessionToken == "" {
		c.AWS.SessionToken = os.Getenv("AWS_SESSION_TOKEN")
	}
	if c.AWS.Region == "" {
		c.AWS.Region = os.Getenv("AWS_REGION")
	}
	if c.Paths.QueueDir == "" {
		c.Paths.QueueDir = DefaultQueueDir()
	}
	if c.Paths.ArchiveDir == "" {
		c.Paths.ArchiveDir = DefaultArchiveDir()
	}
	if c.Paths.LogDir == "" {
		c.Paths.LogDir = DefaultLogDir()
	}
	if c.Logging.Level == "" {
		c.Logging.Level = "info"
	}
	if c.Logging.File == "" {
		base := filepath.Base(configPath)
		if base == "" {
			base = "config.toml"
		}
		c.Logging.File = filepath.Join(c.Paths.LogDir, "santa-sleigh.log")
	}
	if c.Upload.Concurrency <= 0 {
		c.Upload.Concurrency = 4
	}
	if c.Upload.MaxRetries <= 0 {
		c.Upload.MaxRetries = 3
	}
	if c.Upload.PollInterval.Duration == 0 {
		c.Upload.PollInterval = Duration{Duration: 15 * time.Second}
	}
	if c.Upload.StagingSuffix == "" {
		c.Upload.StagingSuffix = ".partial"
	}
}

func (c *Config) validate() error {
	var errs []string
	if c.AWS.Region == "" {
		errs = append(errs, "aws.region must be set")
	}
	if c.AWS.Bucket == "" {
		errs = append(errs, "aws.bucket must be set")
	}
	if c.Paths.QueueDir == "" {
		errs = append(errs, "paths.queue_dir must be set")
	}
	if c.Logging.File == "" {
		errs = append(errs, "logging.file must be set")
	}
	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}
