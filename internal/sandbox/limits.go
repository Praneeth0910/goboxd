package sandbox

import (
	"github.com/thesouldev/goboxd/internal/config"
)

// Limits represents merged resource limits
type Limits struct {
	Memory      int
	CPU         int
	Processes   int
	OpenFiles   int
	MaxDuration int
}

// MergeLimits combines config defaults with request-specific overrides
func MergeLimits(base config.ResourceLimits, overrides map[string]int) Limits {
	limits := Limits{
		Memory:      base.Memory,
		CPU:         base.CPU,
		Processes:   base.Processes,
		OpenFiles:   base.OpenFiles,
		MaxDuration: base.MaxDuration,
	}

	// Apply overrides
	if mem, ok := overrides["memory"]; ok && mem > 0 {
		limits.Memory = mem
	}
	if cpu, ok := overrides["cpu"]; ok && cpu > 0 {
		limits.CPU = cpu
	}
	if procs, ok := overrides["processes"]; ok && procs > 0 {
		limits.Processes = procs
	}
	if files, ok := overrides["open_files"]; ok && files > 0 {
		limits.OpenFiles = files
	}
	if duration, ok := overrides["max_duration"]; ok && duration > 0 {
		limits.MaxDuration = duration
	}

	return limits
}
