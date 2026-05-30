package sandbox

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
)

var jailCounter atomic.Int64

// Why this scheme prevents collisions even under concurrent load across multiple goroutines:
// 1. PID (os.Getpid()): Prevents collision between different concurrently running goboxd server processes.
// 2. jailCounter (atomic.Int64): An atomic increment guarantees that no two goroutines within the same
//    process will ever receive the same counter value, even under extremely high concurrent load.
// 3. crypto/rand (6-hex-random): Generates 3 bytes of cryptographically secure random numbers (6 hex characters).
//    This adds a final layer of entropy, preventing predictable patterns and guaranteeing uniqueness.
// Combine these three factors, and the namespace is guaranteed to be completely collision-free globally and locally.

// NewJailDir creates a unique directory under baseDir.
// Name format: goboxd-{PID}-{counter}-{randomHex}
// Never reuses a directory. Returns cleanup func.
func NewJailDir(baseDir string) (path string, cleanup func(), err error) {
	// Generate 3 bytes of secure random data -> 6 hex characters
	randomBytes := make([]byte, 3)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", nil, fmt.Errorf("failed to generate random bytes: %w", err)
	}
	randomHex := hex.EncodeToString(randomBytes)

	// Unique name pattern incorporating PID, thread-safe counter, and random hex
	dirName := fmt.Sprintf("goboxd-%d-%d-%s", os.Getpid(), jailCounter.Add(1), randomHex)

	// Using os.MkdirTemp as the foundation, under baseDir, using the unique pattern
	targetPath, err := os.MkdirTemp(baseDir, dirName)
	if err != nil {
		return "", nil, fmt.Errorf("failed to create temp jail dir: %w", err)
	}
	if err := os.Chmod(targetPath, 0o777); err != nil {
		return "", nil, fmt.Errorf("failed to chmod temp jail dir: %w", err)
	}

	cleanupFn := func() {
		if err := os.RemoveAll(targetPath); err != nil {
			slog.Error("failed to remove jail dir", "path", targetPath, "error", err)
		}
	}

	return targetPath, cleanupFn, nil
}

// SafeJoin joins a base directory with a filename and guarantees that the resulting
// path does not escape the base directory. It serves as a defense-in-depth mechanism.
func SafeJoin(base, name string) (string, error) {
	joined := filepath.Join(base, filepath.Base(name))
	absBase, err := filepath.Abs(base)
	if err != nil {
		return "", fmt.Errorf("failed to resolve base path: %w", err)
	}
	absJoined, err := filepath.Abs(joined)
	if err != nil {
		return "", fmt.Errorf("failed to resolve joined path: %w", err)
	}
	if !strings.HasPrefix(absJoined, absBase+string(filepath.Separator)) {
		return "", fmt.Errorf("path %q escapes sandbox", name)
	}
	return absJoined, nil
}
