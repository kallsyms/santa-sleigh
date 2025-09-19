package daemon

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/kallsyms/santa-sleigh/internal/config"
	"github.com/kallsyms/santa-sleigh/internal/uploader"
)

// Daemon coordinates scanning a queue directory and shipping entries to S3.
type Daemon struct {
	cfg           *config.Config
	uploader      uploader.Uploader
	logger        *slog.Logger
	mode          config.UploadMode
	lastJSONFlush time.Time
	hostname      string
}

// New builds a daemon instance with the supplied configuration and dependencies.
func New(cfg *config.Config, up uploader.Uploader, logger *slog.Logger) *Daemon {
	host, err := os.Hostname()
	if err != nil || host == "" {
		host = "unknown"
	}
	host = strings.ToLower(host)
	host = strings.ReplaceAll(host, " ", "-")
	host = strings.ReplaceAll(host, "/", "-")
	host = strings.ReplaceAll(host, ":", "-")

	return &Daemon{
		cfg:           cfg,
		uploader:      up,
		logger:        logger,
		mode:          cfg.Upload.Mode,
		lastJSONFlush: time.Now(),
		hostname:      host,
	}
}

// Run blocks until the context is cancelled or a fatal error occurs.
func (d *Daemon) Run(ctx context.Context) error {
	d.logger.Info("starting santa-sleigh", slog.String("version", Version()))
	if err := d.ensureDirectories(); err != nil {
		return err
	}
	ticker := time.NewTicker(d.cfg.Upload.PollInterval.Duration)
	defer ticker.Stop()

	// Kick off an initial scan immediately.
	if err := d.scanAndUpload(ctx); err != nil {
		d.logger.Error("initial scan failed", slog.String("error", err.Error()))
	}

	for {
		select {
		case <-ctx.Done():
			d.logger.Info("shutdown signal received")
			return nil
		case <-ticker.C:
			if err := d.scanAndUpload(ctx); err != nil {
				d.logger.Error("scan failed", slog.String("error", err.Error()))
			}
		}
	}
}

func (d *Daemon) ensureDirectories() error {
	var dirs []string
	queuePath := d.cfg.Upload.Queue
	if d.mode == config.ModeJSON {
		dirs = append(dirs, filepath.Dir(queuePath))
	} else {
		dirs = append(dirs, queuePath)
	}
	dirs = append(dirs, filepath.Dir(d.cfg.Logging.File))
	for _, dir := range dirs {
		if dir == "" {
			continue
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create directory %s: %w", dir, err)
		}
	}
	if d.mode == config.ModeJSON {
		filePath := queuePath
		f, err := os.OpenFile(filePath, os.O_CREATE|os.O_RDWR, 0o640)
		if err != nil {
			return fmt.Errorf("create json input file %s: %w", filePath, err)
		}
		if err := f.Close(); err != nil {
			return fmt.Errorf("close json input file %s: %w", filePath, err)
		}
	}
	return nil
}

func (d *Daemon) scanAndUpload(ctx context.Context) error {
	switch d.mode {
	case config.ModeJSON:
		return d.processJSON(ctx)
	default:
		return d.processParquet(ctx)
	}
}

func (d *Daemon) processParquet(ctx context.Context) error {
	entries, err := os.ReadDir(d.cfg.Upload.Queue)
	if err != nil {
		return fmt.Errorf("read queue dir: %w", err)
	}
	var files []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if d.cfg.Upload.StagingSuffix != "" && strings.HasSuffix(name, d.cfg.Upload.StagingSuffix) {
			continue
		}
		files = append(files, filepath.Join(d.cfg.Upload.Queue, name))
	}

	if len(files) == 0 {
		d.logger.Debug("queue empty")
		return nil
	}

	sort.Slice(files, func(i, j int) bool {
		iInfo, _ := os.Stat(files[i])
		jInfo, _ := os.Stat(files[j])
		if iInfo == nil || jInfo == nil {
			return files[i] < files[j]
		}
		return iInfo.ModTime().Before(jInfo.ModTime())
	})

	sem := make(chan struct{}, d.cfg.Upload.Concurrency)
	errCh := make(chan error, len(files))
	var wg sync.WaitGroup

	for _, file := range files {
		file := file
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-sem }()
			if err := d.processFile(ctx, file); err != nil {
				errCh <- err
			}
		}()
	}

	wg.Wait()
	close(errCh)

	var combined error
	for err := range errCh {
		combined = errors.Join(combined, err)
	}
	return combined
}

func (d *Daemon) processFile(ctx context.Context, path string) error {
	claimedPath, release, err := d.claim(path)
	if err != nil {
		return err
	}
	defer func() {
		if releaseErr := release(); releaseErr != nil {
			d.logger.Error("release failed", slog.String("file", path), slog.String("error", releaseErr.Error()))
		}
	}()

	info, err := os.Stat(claimedPath)
	if err != nil {
		return fmt.Errorf("stat claimed file %s: %w", claimedPath, err)
	}
	file, err := os.Open(claimedPath)
	if err != nil {
		return fmt.Errorf("open file %s: %w", claimedPath, err)
	}
	defer file.Close()

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	key := d.objectKey(filepath.Base(path), info.ModTime())
	if err := d.uploader.Upload(ctx, key, file, info.Size()); err != nil {
		return fmt.Errorf("upload %s: %w", claimedPath, err)
	}

	if err := os.Remove(claimedPath); err != nil {
		return fmt.Errorf("remove %s: %w", claimedPath, err)
	}

	d.logger.Info("uploaded", slog.String("key", key), slog.Int64("size_bytes", info.Size()))
	return nil
}

func (d *Daemon) objectKey(filename string, ts time.Time) string {
	parts := make([]string, 0, 4)
	if prefix := strings.Trim(d.cfg.AWS.S3Prefix, "/"); prefix != "" {
		parts = append(parts, prefix)
	}
	parts = append(parts,
		fmt.Sprintf("hostname=%s", d.hostname),
		fmt.Sprintf("date=%s", ts.UTC().Format("20060102")),
		filename,
	)
	return path.Join(parts...)
}

func (d *Daemon) claim(path string) (string, func() error, error) {
	stagingSuffix := d.cfg.Upload.StagingSuffix
	if stagingSuffix == "" {
		return path, func() error { return nil }, nil
	}
	stagingPath := path + stagingSuffix
	if err := os.Rename(path, stagingPath); err != nil {
		return "", nil, fmt.Errorf("rename %s to staging: %w", path, err)
	}

	released := false
	release := func() error {
		if released {
			return nil
		}
		released = true
		if _, err := os.Stat(stagingPath); err == nil {
			return os.Rename(stagingPath, path)
		}
		return nil
	}

	return stagingPath, release, nil
}

func (d *Daemon) processJSON(ctx context.Context) error {
	path := d.cfg.Upload.Queue
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("stat json input %s: %w", path, err)
	}
	size := info.Size()
	if size == 0 {
		return nil
	}

	now := time.Now()
	maxBytes := d.cfg.Upload.JSONMaxBytes
	if maxBytes <= 0 {
		maxBytes = 10 * 1024 * 1024
	}
	maxInterval := d.cfg.Upload.JSONMaxInterval.Duration
	if maxInterval <= 0 {
		maxInterval = 5 * time.Minute
	}
	if size < maxBytes && now.Sub(d.lastJSONFlush) < maxInterval {
		return nil
	}

	rotatedPath, err := d.rotateJSONInput(path, info, now)
	if err != nil {
		return err
	}
	if rotatedPath == "" {
		return nil
	}

	data, err := os.ReadFile(rotatedPath)
	if err != nil {
		return fmt.Errorf("read rotated json input %s: %w", rotatedPath, err)
	}
	if len(data) == 0 {
		_ = os.Remove(rotatedPath)
		return nil
	}

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write(data); err != nil {
		return fmt.Errorf("compress json payload: %w", err)
	}
	if err := gz.Close(); err != nil {
		return fmt.Errorf("finalise compression: %w", err)
	}

	base := filepath.Base(path)
	nameRoot := strings.TrimSuffix(base, filepath.Ext(base))
	if nameRoot == "" {
		nameRoot = "telemetry"
	}
	objectName := fmt.Sprintf("%s-%s.json.gz", nameRoot, now.UTC().Format("20060102T150405Z"))
	objectKey := d.objectKey(objectName, now)

	compressed := buf.Bytes()
	if err := d.uploader.Upload(ctx, objectKey, bytes.NewReader(compressed), int64(len(compressed))); err != nil {
		return fmt.Errorf("upload json payload: %w", err)
	}
	d.lastJSONFlush = now

	if err := os.Remove(rotatedPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		d.logger.Warn("remove rotated json input", slog.String("path", rotatedPath), slog.String("error", err.Error()))
	}

	d.logger.Info("uploaded json chunk", slog.String("key", objectKey), slog.Int("raw_bytes", len(data)), slog.Int("compressed_bytes", len(compressed)))
	return nil
}

func (d *Daemon) rotateJSONInput(path string, info os.FileInfo, ts time.Time) (string, error) {
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	rotatedName := fmt.Sprintf("%s.%s", base, ts.UTC().Format("20060102T150405Z"))
	rotatedPath := filepath.Join(dir, rotatedName)

	if err := os.Rename(path, rotatedPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", fmt.Errorf("rotate json input: %w", err)
	}

	return rotatedPath, nil
}
