package runner

// ProbeResult holds the result of a binary probe.
type ProbeResult struct {
	OK      bool
	Version string
	Error   string
}
