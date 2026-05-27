package status

import (
	"strings"
)

const (
	StatusAccepted                 = "accepted"
	StatusBuildFailed              = "build_failed"
	StatusWrongOutput              = "wrong_output"
	StatusOutputWhitespaceMismatch = "output_whitespace_mismatch"
	StatusTimeExceeded             = "time_exceeded"
	StatusMemoryExceeded           = "memory_exceeded"
	StatusRuntimeError             = "runtime_error"
	StatusNotExecuted              = "not_executed"
	StatusInternalError            = "internal_error"

	// Build specific statuses
	StatusOK     = "ok"
	StatusFailed = "failed"
)

// TopLevelStatus computes top-level status from build + test results.
// Rule: accepted only if build ok AND all tests accepted.
// If build fails: build_failed, all tests not_executed.
// Otherwise: first non-accepted test status.
func TopLevelStatus(buildStatus string, testStatuses []string) string {
	if buildStatus != StatusOK {
		return StatusBuildFailed
	}

	for _, status := range testStatuses {
		if status != StatusAccepted {
			return status // return the FIRST non-accepted status
		}
	}

	return StatusAccepted
}

// CompareOutput compares actual vs expected stdout.
// Returns accepted, wrong_output, or output_whitespace_mismatch.
func CompareOutput(actual, expected string) string {
	if actual == expected {
		return StatusAccepted
	}
	// Normalize all whitespace (leading, trailing, internal runs of spaces/tabs/newlines)
	// by splitting on whitespace and rejoining with single spaces.
	if normalizeWhitespace(actual) == normalizeWhitespace(expected) {
		return StatusOutputWhitespaceMismatch
	}
	return StatusWrongOutput
}

// normalizeWhitespace collapses all whitespace runs into single spaces and trims.
func normalizeWhitespace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// StatusFromExitCode computes test status from execution results.
func StatusFromExitCode(exitCode int, timedOut, memKilled bool) string {
	if timedOut {
		return StatusTimeExceeded
	}
	if memKilled {
		return StatusMemoryExceeded
	}
	if exitCode == 0 {
		return StatusAccepted // (caller should check output too)
	}
	return StatusRuntimeError
}
