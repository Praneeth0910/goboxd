package runner

import (
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

// ProbeNsjail checks if nsjail is available.
func ProbeNsjail() ProbeResult {
	nsjailPath := os.Getenv("NSJAIL_PATH")
	if nsjailPath == "" {
		nsjailPath = "/usr/sbin/nsjail"
	}

	info, err := os.Stat(nsjailPath)
	if err != nil {
		return ProbeResult{OK: false, Error: err.Error()}
	}

	// Check if it's executable
	if info.Mode()&0o111 == 0 {
		return ProbeResult{OK: false, Error: "nsjail is not executable"}
	}

	return ProbeResult{OK: true, Version: "unknown"}
}

// ProbeLanguage checks if the language compiler/interpreter is available.
func ProbeLanguage(lang config.Language) ProbeResult {
	cmdName := lang.Run.Cmd
	// For compiled languages, we want to probe the compiler instead of the runtime artifact
	if lang.Build != nil && lang.Build.Cmd != "" {
		cmdName = lang.Build.Cmd
	}

	if cmdName == "" {
		return ProbeResult{OK: false, Error: "no command defined for language"}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	versionArg := "--version"
	cmdBase := cmdName
	if idx := strings.LastIndex(cmdBase, "/"); idx != -1 {
		cmdBase = cmdBase[idx+1:]
	}
	switch cmdBase {
	case "go":
		versionArg = "version"
	case "kotlinc":
		versionArg = "-version"
	case "iverilog":
		versionArg = "-V"
	}

	cmd := exec.CommandContext(ctx, cmdName, versionArg)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return ProbeResult{OK: false, Error: err.Error()}
	}

	// Return the first non-warning line of output as the version
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	version := ""
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.Contains(line, "warning:") && !strings.Contains(line, "Warning:") {
			version = line
			break
		}
	}

	return ProbeResult{OK: true, Version: version}
}
