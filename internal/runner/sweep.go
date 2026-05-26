package runner

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// SweepOrphanedDirectories removes goboxd-* dirs in baseDir
// that are older than maxAge. Called once at startup.
//
// Why we use mtime-based age rather than trying to match against currently running processes:
//  1. Accuracy & Simplicity: Matching folders against currently running processes (e.g., checking active PIDs)
//     is highly prone to false positives/negatives due to PID reuse (PID rollover) by the OS. A PID that was used
//     by a crashed goboxd instance hours ago might now belong to an entirely unrelated system process, making the
//     folder appear active when it is actually an orphan.
//  2. Cross-Host/Server Isolation: If the filesystem is shared or if server instances are quickly restarted,
//     relying on local process lists can lead to premature or incorrect cleanup decisions.
//  3. Absolute Bounds on Lifetime: In a code-execution sandbox, runs have a strict execution time limit (typically
//     seconds). If a directory's modification time (mtime) is significantly older than the maximum possible execution
//     age (e.g., 10 minutes), it is mathematically guaranteed to be orphaned. Even if a process was somehow still alive,
//     it has exceeded its maximum wall-time limit and should be cleaned up regardless.
func SweepOrphanedDirectories(baseDir string, maxAge time.Duration) {
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		slog.Warn("sweep: failed to read base directory", "baseDir", baseDir, "err", err)
		return
	}

	for _, e := range entries {
		// Only sweep directories with the "goboxd-" prefix
		if !e.IsDir() || !strings.HasPrefix(e.Name(), "goboxd-") {
			continue
		}

		info, err := e.Info()
		if err != nil {
			slog.Warn("sweep: failed to get directory info", "name", e.Name(), "err", err)
			continue
		}

		// If the directory is older than maxAge, it's safe to sweep (no active run could be this old)
		if time.Since(info.ModTime()) > maxAge {
			path := filepath.Join(baseDir, e.Name())
			if err := os.RemoveAll(path); err != nil {
				slog.Warn("sweep: failed to remove orphaned directory", "path", path, "err", err)
			} else {
				slog.Info("sweep: successfully removed orphaned directory", "path", path)
			}
		}
	}
}
