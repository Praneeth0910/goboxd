package validate

import (
	"encoding/json"
	"strings"
	"testing"
)

// ============ ValidateFilename Tests ============

func TestValidateFilenameAccepts(t *testing.T) {
	valid := []string{
		"solution.cpp",
		"Solution.java",
		"main.go",
		"solution_v2.py",
		"test.c",
		"README.txt",
		"file123.py",
		"my-file.cpp",
		"solution.h",
		"something..", // Trailing dots are OK
		"file....py",  // Multiple dots are OK
	}
	for _, name := range valid {
		if err := ValidateFilename(name); err != nil {
			t.Errorf("ValidateFilename(%q) should be valid, got: %v", name, err)
		}
	}
}

func TestValidateFilenameRejectsEmpty(t *testing.T) {
	err := ValidateFilename("")
	if err == nil {
		t.Fatal("ValidateFilename(\"\") should reject empty string")
	}
	assertErrorCode(t, err, "invalid_filename", "empty")
	if !strings.Contains(err.Error(), "empty") {
		t.Errorf("expected 'empty' in message, got: %v", err)
	}
}

func TestValidateFilenameRejectsPathSeparators(t *testing.T) {
	attacks := []string{
		"../../etc/passwd",
		"..\\..\\windows\\system32",
		"subdir/solution.py",
		"dir\\file.cpp",
		"a/b/c.py",
	}
	for _, name := range attacks {
		err := ValidateFilename(name)
		if err == nil {
			t.Errorf("ValidateFilename(%q) should reject path separators", name)
		}
		assertErrorCode(t, err, "invalid_filename", "separator")
	}

	// Separately test files starting with ./ or ../
	leadingDotCases := []string{
		"./solution.cpp",
		"../solution.cpp",
		"./.hidden",
	}
	for _, name := range leadingDotCases {
		err := ValidateFilename(name)
		if err == nil {
			t.Errorf("ValidateFilename(%q) should reject leading dot or separators", name)
		}
	}
}

func TestValidateFilenameRejectsLeadingDot(t *testing.T) {
	attacks := []string{
		".hidden",
		".bashrc",
		".config",
		"..",
		"...",
	}
	for _, name := range attacks {
		err := ValidateFilename(name)
		if err == nil {
			t.Errorf("ValidateFilename(%q) should reject leading dot", name)
		}
		assertErrorCode(t, err, "invalid_filename", "dot")
	}
}

func TestValidateFilenameRejectsAbsolutePath(t *testing.T) {
	attacks := []string{
		"/etc/passwd",
		"/usr/bin/python",
		"/var/log/access.log",
		"/root/.ssh/id_rsa",
	}
	for _, name := range attacks {
		err := ValidateFilename(name)
		if err == nil {
			t.Errorf("ValidateFilename(%q) should reject absolute path", name)
		}
		// Absolute paths start with / which is a path separator, so they're rejected for that
		assertErrorCode(t, err, "invalid_filename", "separator")
	}
}

func TestValidateFilenameRejectsDirectoryTraversal(t *testing.T) {
	attacks := []string{
		"..",
		"..something",
		".setup",
	}
	for _, name := range attacks {
		err := ValidateFilename(name)
		if err == nil {
			t.Errorf("ValidateFilename(%q) should reject directory traversal/leading dot", name)
		}
		assertErrorCode(t, err, "invalid_filename", "dot")
	}
}

func TestValidateFilenameRejectsLongNames(t *testing.T) {
	// Create a 129-character valid-looking filename
	longName := strings.Repeat("a", 129)
	err := ValidateFilename(longName)
	if err == nil {
		t.Fatal("ValidateFilename(very_long_name) should reject names > 128 chars")
	}
	assertErrorCode(t, err, "invalid_filename", "exceeds")
	if !strings.Contains(err.Error(), "128") {
		t.Errorf("expected '128' in error message for length check")
	}
}

func TestValidateFilenameAccepts128Chars(t *testing.T) {
	// 128 characters should be accepted
	name128 := strings.Repeat("a", 128) + ".py"
	if len(name128) > 128 {
		name128 = strings.Repeat("a", 124) + ".py" // Adjust to exactly fit
	}
	// Just verify 128-char name without extension works
	name128exact := strings.Repeat("a", 128)
	err := ValidateFilename(name128exact)
	if err != nil {
		t.Errorf("ValidateFilename(128-char name) should accept, got: %v", err)
	}
}

func TestValidateFilenameRejectsNullBytes(t *testing.T) {
	attacks := []string{
		"solution\x00.py",
		"file.cpp\x00\x00",
		"\x00solution.py",
	}
	for _, name := range attacks {
		err := ValidateFilename(name)
		if err == nil {
			t.Errorf("ValidateFilename should reject null bytes")
		}
		assertErrorCode(t, err, "invalid_filename", "control character")
	}
}

func TestValidateFilenameRejectsControlCharacters(t *testing.T) {
	// Test various control characters
	attacks := []string{
		"solution\x01.py", // SOH (Start of Heading)
		"file\x1b.cpp",    // ESC
		"test\t.py",       // TAB (0x09)
		"solution\x0d.py", // CR (0x0D)
		"test\x0a.py",     // LF (0x0A)
		"file\x7f.cpp",    // DEL (0x7F)
	}
	for _, name := range attacks {
		err := ValidateFilename(name)
		if err == nil {
			t.Errorf("ValidateFilename should reject control character in %q", name)
		}
		assertErrorCode(t, err, "invalid_filename", "control character")
	}
}

func TestValidateFilenameErrorFormat(t *testing.T) {
	// Verify that errors are JSON-serializable and have correct structure
	err := ValidateFilename("")
	if err == nil {
		t.Fatal("expected error")
	}

	// Try to unmarshal the error message as JSON
	errStr := err.Error()
	var errJSON ValidationError
	if err := json.Unmarshal([]byte(errStr), &errJSON); err != nil {
		t.Errorf("error message should be valid JSON: %v (got: %q)", err, errStr)
	}

	if errJSON.Code != "invalid_filename" {
		t.Errorf("expected code='invalid_filename', got %q", errJSON.Code)
	}
	if errJSON.Message == "" {
		t.Error("expected non-empty message")
	}
}

func TestValidateFilenameRealWorldAttacks(t *testing.T) {
	// Real-world attack attempts
	attacks := map[string]string{
		"../../../etc/passwd":      "path traversal",
		"..\\..\\windows\\win.ini": "path traversal",
		"/etc/shadow":              "absolute path",
		".bashrc":                  "hidden file",
		".ssh/id_rsa":              "hidden with path separator",
		"solution\x00.py":          "null byte injection",
		"file\r\nInjection":        "CRLF injection",
	}
	for attack, reason := range attacks {
		err := ValidateFilename(attack)
		if err == nil {
			t.Errorf("should reject %s attack: %q", reason, attack)
		}
	}
}

// ============ Legacy Filename Function Tests ============

func TestFilenameValid(t *testing.T) {
	if err := Filename("solution.cpp"); err != nil {
		t.Fatalf("expected valid filename, got: %v", err)
	}
}

func TestFilenameTraversal(t *testing.T) {
	if err := Filename("../../etc/passwd"); err == nil {
		t.Fatal("expected error for path traversal")
	}
}

// ============ Flags Tests ============

func TestFlagsEmpty(t *testing.T) {
	if err := Flags("go", ""); err != nil {
		t.Fatalf("expected no error for empty flags, got: %v", err)
	}
}

// ============ Attack Test Suite ============

func TestRealWorldAttackVectors(t *testing.T) {
	// Comprehensive attack test suite
	type attackTest struct {
		filename    string
		shouldFail  bool
		description string
	}

	attacks := []attackTest{
		// Path traversal attacks
		{"../../etc/passwd", true, "Classic path traversal"},
		{"../../../etc/shadow", true, "Deep path traversal"},
		{"..\\..\\windows\\system32\\config\\sam", true, "Windows path traversal"},

		// Absolute paths
		{"/etc/passwd", true, "Absolute path to passwd"},
		{"/root/.ssh/id_rsa", true, "Absolute path to SSH key"},
		{"/var/log/auth.log", true, "Absolute path to log"},

		// Hidden files and dot traversal
		{".bashrc", true, "Hidden bash config"},
		{".ssh", true, "Hidden SSH directory"},
		{"..", true, "Parent directory reference"},
		{"..passwd", true, "Dot prefix with traversal"},

		// Null byte injection
		{"solution.py\x00.txt", true, "Null byte injection"},
		{"file\x00etc\x00passwd", true, "Multiple null bytes"},

		// Control character injection
		{"solution.py\r\nInjected", true, "CRLF injection"},
		{"solution\x1bpy", true, "ESC character"},
		{"file.cpp\ttab", true, "Tab injection"},

		// Length overflow
		{strings.Repeat("a", 129), true, "Length exceeds 128 chars"},

		// Mixed attacks
		{"../../.bashrc", true, "Traversal + hidden"},
		{"./solution.cpp", true, "Relative path with dot"},
		{"./.hidden", true, "Dot directory reference"},

		// Valid filenames - should all pass
		{"solution.cpp", false, "Valid C++ file"},
		{"solution.py", false, "Valid Python file"},
		{"Solution.java", false, "Valid Java file"},
		{"main.go", false, "Valid Go file"},
		{"solution_v2.py", false, "Valid with underscore and number"},
		{"test-file.cpp", false, "Valid with dash"},
		{"README.md", false, "Valid README"},
		{"file123.txt", false, "Valid with numbers"},
		{"something..", false, "Valid trailing dots"},
	}

	for _, attack := range attacks {
		err := ValidateFilename(attack.filename)
		rejected := err != nil

		if rejected != attack.shouldFail {
			if attack.shouldFail {
				t.Errorf("ATTACK NOT BLOCKED - %s: filename=%q, expected rejection but was accepted",
					attack.description, truncateForLog(attack.filename))
			} else {
				t.Errorf("FALSE POSITIVE - %s: filename=%q, expected acceptance but was rejected: %v",
					attack.description, truncateForLog(attack.filename), err)
			}
		}
	}
}

func truncateForLog(s string) string {
	if len(s) > 40 {
		return s[:40] + "..."
	}
	return s
}

// ============ Test Helpers ============

func assertErrorCode(t *testing.T, err error, expectedCode, contextKeyword string) {
	t.Helper()
	if err == nil {
		t.Errorf("expected error for %s check", contextKeyword)
		return
	}

	errStr := err.Error()
	var errJSON ValidationError
	if err := json.Unmarshal([]byte(errStr), &errJSON); err != nil {
		// If not JSON, check error string contains the keyword
		if !strings.Contains(strings.ToLower(errStr), strings.ToLower(contextKeyword)) {
			t.Errorf("expected error containing %q, got: %q", contextKeyword, errStr)
		}
		return
	}

	if errJSON.Code != expectedCode {
		t.Errorf("expected code=%q, got code=%q", expectedCode, errJSON.Code)
	}
	if !strings.Contains(strings.ToLower(errJSON.Message), strings.ToLower(contextKeyword)) {
		t.Errorf("expected message containing %q, got: %q", contextKeyword, errJSON.Message)
	}
}
