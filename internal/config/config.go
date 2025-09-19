package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
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

// LoggingConfig sets the log destination and verbosity.
type LoggingConfig struct {
	Level string `toml:"level"`
	File  string `toml:"file"`
}

// UploadConfig tunes the daemon behaviour for uploads.
type UploadConfig struct {
	Concurrency     int        `toml:"concurrency"`
	PollInterval    Duration   `toml:"poll_interval"`
	MaxRetries      int        `toml:"max_retries"`
	StagingSuffix   string     `toml:"staging_suffix"`
	Mode            UploadMode `toml:"mode"`
	Queue           string     `toml:"queue"`
	JSONMaxBytes    int64      `toml:"json_max_bytes"`
	JSONMaxInterval Duration   `toml:"json_max_interval"`
}

// UploadMode selects between parquet directory processing and JSON tailing.
type UploadMode string

const (
	ModeParquet UploadMode = "parquet"
	ModeJSON    UploadMode = "json"
)

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
		return "/var/db/santa/spool"
	}
	return "/var/log/pedro/spool"
}

// DefaultJSONInputPath returns the default telemetry log file for JSON mode.
func DefaultJSONInputPath() string {
	if runtime.GOOS == "darwin" {
		return "/var/db/santa/log.ndjson"
	}
	return "/var/log/pedro/log.ndjson"
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
	if c.Upload.Mode == "" {
		c.Upload.Mode = ModeParquet
	} else {
		c.Upload.Mode = UploadMode(strings.ToLower(string(c.Upload.Mode)))
	}
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
	if c.Upload.Queue == "" {
		if c.Upload.Mode == ModeJSON {
			c.Upload.Queue = DefaultJSONInputPath()
		} else {
			c.Upload.Queue = DefaultQueueDir()
		}
	}
	if c.Logging.Level == "" {
		c.Logging.Level = "info"
	}
	if c.Logging.File == "" {
		base := filepath.Base(configPath)
		if base == "" {
			base = "config.toml"
		}
		if runtime.GOOS == "darwin" {
			c.Logging.File = "/Library/Logs/SantaSleigh/santa-sleigh.log"
		} else {
			c.Logging.File = "/var/log/santa-sleigh/santa-sleigh.log"
		}
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
	if c.Upload.JSONMaxBytes <= 0 {
		c.Upload.JSONMaxBytes = 10 * 1024 * 1024
	}
	if c.Upload.JSONMaxInterval.Duration == 0 {
		c.Upload.JSONMaxInterval = Duration{Duration: 5 * time.Minute}
	}
}

func (c *Config) validate() error {
	var errs []string
	switch c.Upload.Mode {
	case ModeParquet, ModeJSON:
	default:
		errs = append(errs, "upload.mode must be \"json\" or \"parquet\"")
	}
	if c.AWS.Region == "" {
		errs = append(errs, "aws.region must be set")
	}
	if c.AWS.Bucket == "" {
		errs = append(errs, "aws.bucket must be set")
	}
	if c.Upload.Queue == "" {
		errs = append(errs, "upload.queue must be set")
	}
	if c.Upload.JSONMaxBytes <= 0 {
		errs = append(errs, "upload.json_max_bytes must be greater than 0")
	}
	if c.Upload.JSONMaxInterval.Duration <= 0 {
		errs = append(errs, "upload.json_max_interval must be greater than 0")
	}
	if c.Logging.File == "" {
		errs = append(errs, "logging.file must be set")
	}
	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}
