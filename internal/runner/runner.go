package runner

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"sync/atomic"
)

var jobCounter atomic.Int64

// GenerateJobID creates a unique job identifier.
// Format: {counter}-{randomHex} — never reuses an ID.
func GenerateJobID() string {
	count := jobCounter.Add(1)
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%d-%s-%d", count, hex.EncodeToString(b), os.Getpid())
}
