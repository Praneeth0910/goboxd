package status

import (
	"testing"
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
			buildStatus:    StatusFailed,
			testStatuses:   []string{StatusAccepted, StatusAccepted},
			expectedStatus: StatusBuildFailed,
		},
		{
			name:           "build_failed, empty tests",
			buildStatus:    StatusFailed,
			testStatuses:   []string{},
			expectedStatus: StatusBuildFailed,
		},
		{
			name:           "internal_error build status",
			buildStatus:    StatusInternalError,
			testStatuses:   []string{StatusAccepted},
			expectedStatus: StatusBuildFailed,
		},

		// Build ok, all tests pass
		{
			name:           "build_ok, all tests_accepted",
			buildStatus:    StatusOK,
			testStatuses:   []string{StatusAccepted, StatusAccepted, StatusAccepted},
			expectedStatus: StatusAccepted,
		},
		{
			name:           "build_ok, no tests",
			buildStatus:    StatusOK,
			testStatuses:   []string{},
			expectedStatus: StatusAccepted,
		},

		// Build ok, mixed test results (returns first non-accepted)
		{
			name:           "build_ok, mixed [accepted, wrong_output, time_exceeded]",
			buildStatus:    StatusOK,
			testStatuses:   []string{StatusAccepted, StatusWrongOutput, StatusTimeExceeded},
			expectedStatus: StatusWrongOutput,
		},
		{
			name:           "build_ok, [time_exceeded, accepted]",
			buildStatus:    StatusOK,
			testStatuses:   []string{StatusTimeExceeded, StatusAccepted},
			expectedStatus: StatusTimeExceeded,
		},
		{
			name:           "build_ok, [accepted, runtime_error]",
			buildStatus:    StatusOK,
			testStatuses:   []string{StatusAccepted, StatusRuntimeError},
			expectedStatus: StatusRuntimeError,
		},
		{
			name:           "build_ok, single test_wrong_output",
			buildStatus:    StatusOK,
			testStatuses:   []string{StatusWrongOutput},
			expectedStatus: StatusWrongOutput,
		},
		{
			name:           "build_ok, [memory_exceeded, accepted, wrong_output]",
			buildStatus:    StatusOK,
			testStatuses:   []string{StatusMemoryExceeded, StatusAccepted, StatusWrongOutput},
			expectedStatus: StatusMemoryExceeded,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TopLevelStatus(tt.buildStatus, tt.testStatuses)
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
			expectedStatus: StatusAccepted,
		},
		{
			name:           "exact match empty strings",
			actual:         "",
			expected:       "",
			expectedStatus: StatusAccepted,
		},
		{
			name:           "exact match with newline",
			actual:         "hello\nworld\n",
			expected:       "hello\nworld\n",
			expectedStatus: StatusAccepted,
		},

		// Whitespace mismatch cases
		{
			name:           "trailing newline difference",
			actual:         "hi\n",
			expected:       "hi",
			expectedStatus: StatusOutputWhitespaceMismatch,
		},
		{
			name:           "leading and trailing spaces",
			actual:         "  hi  ",
			expected:       "hi",
			expectedStatus: StatusOutputWhitespaceMismatch,
		},
		{
			name:           "multiple spaces between words",
			actual:         "hello    world",
			expected:       "hello world",
			expectedStatus: StatusOutputWhitespaceMismatch,
		},
		{
			name:           "tabs and spaces",
			actual:         "\thello\t",
			expected:       "hello",
			expectedStatus: StatusOutputWhitespaceMismatch,
		},
		{
			name:           "multiple newlines",
			actual:         "hello\n\n\nworld\n",
			expected:       "hello\nworld",
			expectedStatus: StatusOutputWhitespaceMismatch,
		},

		// Wrong output cases
		{
			name:           "case sensitivity",
			actual:         "HI",
			expected:       "hi",
			expectedStatus: StatusWrongOutput,
		},
		{
			name:           "empty vs non-empty",
			actual:         "",
			expected:       "hi",
			expectedStatus: StatusWrongOutput,
		},
		{
			name:           "non-empty vs empty",
			actual:         "hi",
			expected:       "",
			expectedStatus: StatusWrongOutput,
		},
		{
			name:           "different content",
			actual:         "hello",
			expected:       "world",
			expectedStatus: StatusWrongOutput,
		},
		{
			name:           "substring mismatch",
			actual:         "hello world",
			expected:       "hello",
			expectedStatus: StatusWrongOutput,
		},
		{
			name:           "numbers differ",
			actual:         "42",
			expected:       "43",
			expectedStatus: StatusWrongOutput,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CompareOutput(tt.actual, tt.expected)
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
			expectedStatus: StatusTimeExceeded,
		},
		{
			name:           "timeout with non-zero exit",
			exitCode:       1,
			timedOut:       true,
			memKilled:      false,
			expectedStatus: StatusTimeExceeded,
		},

		// Memory exceeded
		{
			name:           "memory killed",
			exitCode:       0,
			timedOut:       false,
			memKilled:      true,
			expectedStatus: StatusMemoryExceeded,
		},
		{
			name:           "memory killed with non-zero exit",
			exitCode:       1,
			timedOut:       false,
			memKilled:      true,
			expectedStatus: StatusMemoryExceeded,
		},

		// Normal execution
		{
			name:           "zero exit code (success)",
			exitCode:       0,
			timedOut:       false,
			memKilled:      false,
			expectedStatus: StatusAccepted,
		},
		{
			name:           "non-zero exit code (runtime error)",
			exitCode:       1,
			timedOut:       false,
			memKilled:      false,
			expectedStatus: StatusRuntimeError,
		},
		{
			name:           "exit code 127 (not found)",
			exitCode:       127,
			timedOut:       false,
			memKilled:      false,
			expectedStatus: StatusRuntimeError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StatusFromExitCode(tt.exitCode, tt.timedOut, tt.memKilled)
			if got != tt.expectedStatus {
				t.Errorf("StatusFromExitCode(%d, %v, %v) = %q, want %q", tt.exitCode, tt.timedOut, tt.memKilled, got, tt.expectedStatus)
			}
		})
	}
}
