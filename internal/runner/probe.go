package runner

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/thesouldev/goboxd/internal/config"
)

// ProbeResult holds the result of a binary probe.
type ProbeResult struct {
	OK      bool
	Version string
	Error   string
}

// ProbeNsjail checks if nsjail is available and returns its version.
func ProbeNsjail() ProbeResult {
	nsjailPath := os.Getenv("NSJAIL_PATH")
	if nsjailPath == "" {
		nsjailPath = "/usr/sbin/nsjail"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, nsjailPath, "--version")
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	if err := cmd.Run(); err != nil {
		return ProbeResult{OK: false, Error: err.Error()}
	}

	version := strings.TrimSpace(stderrBuf.String())
	return ProbeResult{OK: true, Version: version}
}

// ProbeLanguage checks if the language compiler/interpreter is available.
func ProbeLanguage(lang config.Language) ProbeResult {
	cmdName := lang.Run.Cmd
	// For Java, we want to probe the compiler (javac) instead of the runtime (java)
	if lang.ID == "java" && lang.Build != nil && lang.Build.Cmd != "" {
		cmdName = lang.Build.Cmd
	}

	if cmdName == "" {
		return ProbeResult{OK: false, Error: "no command defined for language"}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, cmdName, "--version")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return ProbeResult{OK: false, Error: err.Error()}
	}

	// Return the first line of output as the version
	lines := strings.SplitN(string(out), "\n", 2)
	version := ""
	if len(lines) > 0 {
		version = strings.TrimSpace(lines[0])
	}

	return ProbeResult{OK: true, Version: version}
}
