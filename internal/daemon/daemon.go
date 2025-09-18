package daemon

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/kallsyms/santa-sleigh/internal/config"
	"github.com/kallsyms/santa-sleigh/internal/uploader"
)

// Daemon coordinates scanning a queue directory and shipping entries to S3.
type Daemon struct {
	cfg      *config.Config
	uploader uploader.Uploader
	logger   *slog.Logger
}

// New builds a daemon instance with the supplied configuration and dependencies.
func New(cfg *config.Config, up uploader.Uploader, logger *slog.Logger) *Daemon {
	return &Daemon{cfg: cfg, uploader: up, logger: logger}
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
	dirs := []string{d.cfg.Paths.QueueDir, d.cfg.Paths.ArchiveDir, filepath.Dir(d.cfg.Logging.File)}
	for _, dir := range dirs {
		if dir == "" {
			continue
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create directory %s: %w", dir, err)
		}
	}
	return nil
}

func (d *Daemon) scanAndUpload(ctx context.Context) error {
	entries, err := os.ReadDir(d.cfg.Paths.QueueDir)
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
		files = append(files, filepath.Join(d.cfg.Paths.QueueDir, name))
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

	key := d.objectKey(filepath.Base(path))
	if err := d.uploader.Upload(ctx, key, file, info.Size()); err != nil {
		return fmt.Errorf("upload %s: %w", claimedPath, err)
	}

	if err := d.archive(claimedPath, filepath.Base(path)); err != nil {
		return fmt.Errorf("archive %s: %w", claimedPath, err)
	}

	d.logger.Info("uploaded", slog.String("key", key), slog.Int64("size_bytes", info.Size()))
	return nil
}

func (d *Daemon) objectKey(filename string) string {
	key := filename
	if d.cfg.AWS.S3Prefix != "" {
		key = filepath.Join(d.cfg.AWS.S3Prefix, filename)
	}
	if runtime.GOOS == "windows" {
		return strings.ReplaceAll(key, "\\", "/")
	}
	return filepath.ToSlash(key)
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

func (d *Daemon) archive(stagingPath, originalName string) error {
	destination := filepath.Join(d.cfg.Paths.ArchiveDir, originalName)
	if err := os.MkdirAll(d.cfg.Paths.ArchiveDir, 0o755); err != nil {
		return fmt.Errorf("create archive dir: %w", err)
	}
	if err := moveFile(stagingPath, destination); err == nil {
		return nil
	}

	// Fallback to timestamp-based name if collision occurs.
	timestamped := fmt.Sprintf("%s-%d", originalName, time.Now().UnixNano())
	return moveFile(stagingPath, filepath.Join(d.cfg.Paths.ArchiveDir, timestamped))
}
