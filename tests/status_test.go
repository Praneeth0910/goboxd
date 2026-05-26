package goboxd_test

import (
	"testing"

	"github.com/thesouldev/goboxd/internal/status"
)

// TestTopLevelStatus tests the TopLevelStatus function with table-driven tests.
func TestTopLevelStatus(t *testing.T) {
	tests := []struct {
		name           string
		buildStatus    string
		testStatuses   []string
		expectedStatus string
	}{
		// Build failed cases
		{
			name:           "build_failed, any tests",
			buildStatus:    status.StatusFailed,
			testStatuses:   []string{status.StatusAccepted, status.StatusAccepted},
			expectedStatus: status.StatusBuildFailed,
		},
		{
			name:           "build_failed, empty tests",
			buildStatus:    status.StatusFailed,
			testStatuses:   []string{},
			expectedStatus: status.StatusBuildFailed,
		},
		{
			name:           "internal_error build status",
			buildStatus:    status.StatusInternalError,
			testStatuses:   []string{status.StatusAccepted},
			expectedStatus: status.StatusBuildFailed,
		},

		// Build ok, all tests pass
		{
			name:           "build_ok, all tests_accepted",
			buildStatus:    status.StatusOK,
			testStatuses:   []string{status.StatusAccepted, status.StatusAccepted, status.StatusAccepted},
			expectedStatus: status.StatusAccepted,
		},
		{
			name:           "build_ok, no tests",
			buildStatus:    status.StatusOK,
			testStatuses:   []string{},
			expectedStatus: status.StatusAccepted,
		},

		// Build ok, mixed test results (returns first non-accepted)
		{
			name:           "build_ok, mixed [accepted, wrong_output, time_exceeded]",
			buildStatus:    status.StatusOK,
			testStatuses:   []string{status.StatusAccepted, status.StatusWrongOutput, status.StatusTimeExceeded},
			expectedStatus: status.StatusWrongOutput,
		},
		{
			name:           "build_ok, [time_exceeded, accepted]",
			buildStatus:    status.StatusOK,
			testStatuses:   []string{status.StatusTimeExceeded, status.StatusAccepted},
			expectedStatus: status.StatusTimeExceeded,
		},
		{
			name:           "build_ok, [accepted, runtime_error]",
			buildStatus:    status.StatusOK,
			testStatuses:   []string{status.StatusAccepted, status.StatusRuntimeError},
			expectedStatus: status.StatusRuntimeError,
		},
		{
			name:           "build_ok, single test_wrong_output",
			buildStatus:    status.StatusOK,
			testStatuses:   []string{status.StatusWrongOutput},
			expectedStatus: status.StatusWrongOutput,
		},
		{
			name:           "build_ok, [memory_exceeded, accepted, wrong_output]",
			buildStatus:    status.StatusOK,
			testStatuses:   []string{status.StatusMemoryExceeded, status.StatusAccepted, status.StatusWrongOutput},
			expectedStatus: status.StatusMemoryExceeded,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := status.TopLevelStatus(tt.buildStatus, tt.testStatuses)
			if got != tt.expectedStatus {
				t.Errorf("TopLevelStatus(%q, %v) = %q, want %q", tt.buildStatus, tt.testStatuses, got, tt.expectedStatus)
			}
		})
	}
}

// TestCompareOutput tests the CompareOutput function with table-driven tests.
func TestCompareOutput(t *testing.T) {
	tests := []struct {
		name           string
		actual         string
		expected       string
		expectedStatus string
	}{
		// Exact match cases
		{
			name:           "exact match",
			actual:         "hello world",
			expected:       "hello world",
			expectedStatus: status.StatusAccepted,
		},
		{
			name:           "exact match empty strings",
			actual:         "",
			expected:       "",
			expectedStatus: status.StatusAccepted,
		},
		{
			name:           "exact match with newline",
			actual:         "hello\nworld\n",
			expected:       "hello\nworld\n",
			expectedStatus: status.StatusAccepted,
		},

		// Whitespace mismatch cases
		{
			name:           "trailing newline difference",
			actual:         "hi\n",
			expected:       "hi",
			expectedStatus: status.StatusOutputWhitespaceMismatch,
		},
		{
			name:           "leading and trailing spaces",
			actual:         "  hi  ",
			expected:       "hi",
			expectedStatus: status.StatusOutputWhitespaceMismatch,
		},
		{
			name:           "multiple spaces between words",
			actual:         "hello    world",
			expected:       "hello world",
			expectedStatus: status.StatusOutputWhitespaceMismatch,
		},
		{
			name:           "tabs and spaces",
			actual:         "\thello\t",
			expected:       "hello",
			expectedStatus: status.StatusOutputWhitespaceMismatch,
		},
		{
			name:           "multiple newlines",
			actual:         "hello\n\n\nworld\n",
			expected:       "hello\nworld",
			expectedStatus: status.StatusOutputWhitespaceMismatch,
		},

		// Wrong output cases
		{
			name:           "case sensitivity",
			actual:         "HI",
			expected:       "hi",
			expectedStatus: status.StatusWrongOutput,
		},
		{
			name:           "empty vs non-empty",
			actual:         "",
			expected:       "hi",
			expectedStatus: status.StatusWrongOutput,
		},
		{
			name:           "non-empty vs empty",
			actual:         "hi",
			expected:       "",
			expectedStatus: status.StatusWrongOutput,
		},
		{
			name:           "different content",
			actual:         "hello",
			expected:       "world",
			expectedStatus: status.StatusWrongOutput,
		},
		{
			name:           "substring mismatch",
			actual:         "hello world",
			expected:       "hello",
			expectedStatus: status.StatusWrongOutput,
		},
		{
			name:           "numbers differ",
			actual:         "42",
			expected:       "43",
			expectedStatus: status.StatusWrongOutput,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := status.CompareOutput(tt.actual, tt.expected)
			if got != tt.expectedStatus {
				t.Errorf("CompareOutput(%q, %q) = %q, want %q", tt.actual, tt.expected, got, tt.expectedStatus)
			}
		})
	}
}

// TestStatusFromExitCode tests the StatusFromExitCode function.
func TestStatusFromExitCode(t *testing.T) {
	tests := []struct {
		name           string
		exitCode       int
		timedOut       bool
		memKilled      bool
		expectedStatus string
	}{
		// Time exceeded takes precedence
		{
			name:           "timeout regardless of exit code",
			exitCode:       0,
			timedOut:       true,
			memKilled:      false,
			expectedStatus: status.StatusTimeExceeded,
		},
		{
			name:           "timeout with non-zero exit",
			exitCode:       1,
			timedOut:       true,
			memKilled:      false,
			expectedStatus: status.StatusTimeExceeded,
		},

		// Memory exceeded
		{
			name:           "memory killed",
			exitCode:       0,
			timedOut:       false,
			memKilled:      true,
			expectedStatus: status.StatusMemoryExceeded,
		},
		{
			name:           "memory killed with non-zero exit",
			exitCode:       1,
			timedOut:       false,
			memKilled:      true,
			expectedStatus: status.StatusMemoryExceeded,
		},

		// Normal execution
		{
			name:           "zero exit code (success)",
			exitCode:       0,
			timedOut:       false,
			memKilled:      false,
			expectedStatus: status.StatusAccepted,
		},
		{
			name:           "non-zero exit code (runtime error)",
			exitCode:       1,
			timedOut:       false,
			memKilled:      false,
			expectedStatus: status.StatusRuntimeError,
		},
		{
			name:           "exit code 127 (not found)",
			exitCode:       127,
			timedOut:       false,
			memKilled:      false,
			expectedStatus: status.StatusRuntimeError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := status.StatusFromExitCode(tt.exitCode, tt.timedOut, tt.memKilled)
			if got != tt.expectedStatus {
				t.Errorf("StatusFromExitCode(%d, %v, %v) = %q, want %q", tt.exitCode, tt.timedOut, tt.memKilled, got, tt.expectedStatus)
			}
		})
	}
}
