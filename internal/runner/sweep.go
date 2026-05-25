package runner

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// SweepOrphanedDirectories removes goboxd-* dirs older than maxAge.
func SweepOrphanedDirectories(baseDir string, maxAge time.Duration) {
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !e.IsDir() || !strings.HasPrefix(e.Name(), "goboxd-") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if time.Since(info.ModTime()) > maxAge {
			path := filepath.Join(baseDir, e.Name())
			if err := os.RemoveAll(path); err != nil {
				slog.Warn("sweep: failed to remove", "path", path, "err", err)
			} else {
				slog.Info("sweep: removed orphan", "path", path)
			}
		}
	}
}
