package runner

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/thesouldev/goboxd/internal/sandbox"
)



// TestCapReaderBoundsOutput verifies that CapReader correctly limits child process output
// to prevent OOM attacks from runaway programs writing gigabytes of data.
// This test does NOT use nsjail; it directly tests the CapReader wrapping behavior.
func TestCapReaderBoundsOutput(t *testing.T) {
	const testLimit = 10 * 1024 // 10 KiB test limit (smaller for fast test)

	// Create a mock reader that produces testLimit + 1024 bytes of data
	excessData := make([]byte, testLimit+1024)
	for i := range excessData {
		excessData[i] = 'A'
	}
	mockReader := bytes.NewReader(excessData)

	// Wrap with CapReader just like runCommand does
	capReader := sandbox.CapReader(mockReader, int64(testLimit))

	// Read all output using io.Copy (the same pattern as runner.go)
	var output bytes.Buffer
	n, err := io.Copy(&output, capReader)

	// Verify no read error (CapReader.Read should not error on normal flow)
	if err != nil && !errors.Is(err, io.EOF) {
		t.Fatalf("unexpected error reading from CapReader: %v", err)
	}

	// The output should be exactly testLimit bytes + the truncation marker
	expectedLen := testLimit + len(sandbox.TruncationMarker)
	if n != int64(expectedLen) {
		t.Fatalf("expected %d bytes read, got %d", expectedLen, n)
	}

	outputBytes := output.Bytes()
	if len(outputBytes) != expectedLen {
		t.Fatalf("expected output length %d, got %d", expectedLen, len(outputBytes))
	}

	// Verify the output ends with the truncation marker
	if !strings.HasSuffix(output.String(), sandbox.TruncationMarker) {
		t.Fatalf("output should end with %q, got: %q", sandbox.TruncationMarker, output.String()[len(output.String())-30:])
	}

	// Verify the output starts with the original data (first part untouched)
	firstPart := outputBytes[:testLimit]
	if !bytes.Equal(firstPart, excessData[:testLimit]) {
		t.Fatalf("first %d bytes of output should match input data", testLimit)
	}
}

// TestCapReaderNoTruncationWhenExactLimit verifies that CapReader does NOT append
// the truncation marker when the output is exactly at the limit (no excess).
func TestCapReaderNoTruncationWhenExactLimit(t *testing.T) {
	const testLimit = 1024

	// Create exactly testLimit bytes of data (no excess)
	exactData := make([]byte, testLimit)
	for i := range exactData {
		exactData[i] = 'B'
	}
	mockReader := bytes.NewReader(exactData)

	capReader := sandbox.CapReader(mockReader, int64(testLimit))

	var output bytes.Buffer
	_, err := io.Copy(&output, capReader)

	if err != nil && !errors.Is(err, io.EOF) {
		t.Fatalf("unexpected error: %v", err)
	}

	// Output should be exactly testLimit bytes (no truncation marker appended)
	if output.Len() != testLimit {
		t.Fatalf("expected output length %d, got %d", testLimit, output.Len())
	}

	// Should NOT contain truncation marker
	if strings.Contains(output.String(), sandbox.TruncationMarker) {
		t.Fatalf("output should NOT contain truncation marker when at exact limit")
	}
}

// TestCapReaderWithDefaultMaxOutputBytes tests with the production default limit
// to ensure a program producing DefaultMaxOutputBytes + 1024 bytes gets capped.
func TestCapReaderWithDefaultMaxOutputBytes(t *testing.T) {
	// Create data that exceeds the default limit by 1 KiB
	excessData := make([]byte, sandbox.DefaultMaxOutputBytes+1024)
	for i := range excessData {
		excessData[i] = byte('X')
	}
	mockReader := bytes.NewReader(excessData)

	capReader := sandbox.CapReader(mockReader, sandbox.DefaultMaxOutputBytes)

	var output bytes.Buffer
	_, err := io.Copy(&output, capReader)

	if err != nil && !errors.Is(err, io.EOF) {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify output is bounded to DefaultMaxOutputBytes + truncation marker
	expectedLen := sandbox.DefaultMaxOutputBytes + int64(len(sandbox.TruncationMarker))
	if output.Len() != int(expectedLen) {
		t.Fatalf("expected output length %d, got %d", expectedLen, output.Len())
	}

	// Verify truncation marker is present
	if !strings.HasSuffix(output.String(), sandbox.TruncationMarker) {
		t.Fatalf("output should end with truncation marker")
	}
}
