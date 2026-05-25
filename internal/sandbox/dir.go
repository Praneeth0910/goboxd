package sandbox

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
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

	cleanupFn := func() {
		_ = os.RemoveAll(targetPath)
	}

	return targetPath, cleanupFn, nil
}
