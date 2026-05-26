package sandbox

import (
	"io"
	"strings"
)

const (
	DefaultMaxSourceBytes = 256 * 1024      // 256 KiB
	DefaultMaxStdinBytes  = 64 * 1024       // 64 KiB
	DefaultMaxOutputBytes = 1 * 1024 * 1024 // 1 MiB
	TruncationMarker      = "\n[output truncated]"
)

// Applying at the HTTP layer:
// "In the run handler, wrap r.Body with http.MaxBytesReader before json.Decode"
// For example:
// r.Body = http.MaxBytesReader(w, r.Body, maxRequestSize)
// This prevents large payload DoS attacks by closing the connection if the body exceeds the threshold.

// Applying at the child process layer:
// "Wrap the nsjail stdout/stderr pipes with CapReader before reading"
// This limits child process memory utilization by clipping logs as they are streamed, preventing runaway memory consumption.

// capReader enforces a limit and appends a truncation marker if the limit is reached.
type capReader struct {
	r     io.Reader
	limit int64
	read  int64
	eof   bool
	trunc io.Reader
}

// CapReader wraps an io.Reader, enforcing a byte limit.
// On limit exceeded, appends TruncationMarker.
func CapReader(r io.Reader, limit int64) io.Reader {
	return &capReader{
		r:     r,
		limit: limit,
	}
}

func (c *capReader) Read(p []byte) (n int, err error) {
	if c.eof {
		if c.trunc != nil {
			return c.trunc.Read(p)
		}
		return 0, io.EOF
	}

	// Remaining space for reading under the limit
	rem := c.limit - c.read
	if rem <= 0 {
		c.eof = true
		c.trunc = strings.NewReader(TruncationMarker)
		return c.trunc.Read(p)
	}

	// Adjust slice capacity to remaining budget
	sliceToRead := p
	if int64(len(p)) > rem {
		sliceToRead = p[:rem]
	}

	n, err = c.r.Read(sliceToRead)
	c.read += int64(n)

	if err == io.EOF {
		c.eof = true
		return n, io.EOF
	}

	// If we've reached exactly our limit, we must check if there is more data left in the source reader
	if c.read >= c.limit {
		// Read one extra byte to verify if truncation is needed
		buf := make([]byte, 1)
		_, checkErr := c.r.Read(buf)
		c.eof = true
		if checkErr == io.EOF {
			return n, io.EOF
		}
		// There is indeed more data to read, so we append the truncation marker
		c.trunc = strings.NewReader(TruncationMarker)
	}

	return n, err
}

// CapOutput limits the output string to the specified size in bytes.
// If it is longer, returns the truncated output with the TruncationMarker.
func CapOutput(output string, limit int) string {
	if len(output) <= limit {
		return output
	}
	return output[:limit] + TruncationMarker
}
