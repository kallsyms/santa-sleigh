package daemon

import (
	"errors"
	"fmt"
	"io"
	"os"
	"syscall"
)

// moveFile attempts to rename src to dst and falls back to a copy+remove when needed.
func moveFile(src, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	} else if !isCrossDeviceLink(err) {
		return fmt.Errorf("rename %s -> %s: %w", src, dst, err)
	}

	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open src %s: %w", src, err)
	}
	defer srcFile.Close()

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("open dst %s: %w", dst, err)
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("copy file: %w", err)
	}
	if err := dstFile.Sync(); err != nil {
		return fmt.Errorf("sync dst: %w", err)
	}
	if err := os.Remove(src); err != nil {
		return fmt.Errorf("remove src: %w", err)
	}
	return nil
}

func isCrossDeviceLink(err error) bool {
	var linkErr *os.LinkError
	if errors.As(err, &linkErr) {
		return errors.Is(linkErr.Err, syscall.EXDEV)
	}
	return errors.Is(err, syscall.EXDEV)
}
