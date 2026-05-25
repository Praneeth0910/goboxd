package scripts
package main

import (
	"encoding/json"
	"fmt"
	"github.com/thesouldev/goboxd/internal/validate"
)

type AttackTest struct {
	Filename  string
	ShouldFail bool
	Description string
}

func main() {
	tests := []AttackTest{
		// Path traversal attacks
		{"../../etc/passwd", true, "Classic path traversal"},
		{"../../../etc/shadow", true, "Deep path traversal"},
		{"..\\..\\windows\\system32\\config\\sam", true, "Windows path traversal"},

		// Absolute paths
		{"/etc/passwd", true, "Absolute path to passwd"},
		{"/root/.ssh/id_rsa", true, "Absolute path to SSH key"},

		// Hidden files and dot traversal
		{".bashrc", true, "Hidden bash config"},
		{".ssh", true, "Hidden SSH directory"},
		{"..", true, "Parent directory reference"},
		{"..passwd", true, "Dot prefix with traversal"},

		// Null byte injection (string representation for display)
		{"solution.py\x00.txt", true, "Null byte injection"},

		// Control character injection
		{"solution.py\r\nInjected", true, "CRLF injection"},
		{"solution\x1bpy", true, "ESC character"},

		// Length overflow
		{string(make([]byte, 129)), true, "Length exceeds 128 chars"},

		// Mixed attacks
		{"../../.bashrc", true, "Traversal + hidden"},
		{"./solution.cpp", true, "Relative path with dot"},

		// Valid filenames
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

	passed := 0
	failed := 0

	fmt.Println("=" + string(make([]byte, 78)) + "=")
	fmt.Println("FILENAME VALIDATION ATTACK TEST SUITE")
	fmt.Println("=" + string(make([]byte, 78)) + "=")
	fmt.Println()

	for _, test := range tests {
		err := validate.ValidateFilename(test.Filename)

		rejected := err != nil
		expectedToFail := test.ShouldFail

		if rejected == expectedToFail {
			passed++
			if rejected {
				fmt.Printf("✓ PASS: %-40s REJECTED\n", test.Description)
			} else {
				fmt.Printf("✓ PASS: %-40s ACCEPTED\n", test.Description)
			}
		} else {
			failed++
			if rejected {
				fmt.Printf("✗ FAIL: %-40s REJECTED (expected ACCEPTED)\n", test.Description)
			} else {
				fmt.Printf("✗ FAIL: %-40s ACCEPTED (expected REJECTED)\n", test.Description)
			}
		}

		if err != nil {
			var errJSON validate.ValidationError
			json.Unmarshal([]byte(err.Error()), &errJSON)
			if errJSON.Code != "" {
				fmt.Printf("       Error: %s\n", errJSON.Message[:min(len(errJSON.Message), 60)])
			}
		}

		fmt.Printf("       Filename: %q\n", truncate(test.Filename, 50))
		fmt.Println()
	}

	fmt.Println("=" + string(make([]byte, 78)) + "=")
	fmt.Printf("RESULTS: %d passed, %d failed out of %d tests\n", passed, failed, len(tests))
	fmt.Println("=" + string(make([]byte, 78)) + "=")

	if failed > 0 {
		fmt.Println("\n❌ ATTACKS NOT BLOCKED - VULNERABILITIES FOUND!")
	} else {
		fmt.Println("\n✅ ALL ATTACKS BLOCKED - NO VULNERABILITIES!")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func truncate(s string, maxLen int) string {
	if len(s) > maxLen {
		return s[:maxLen] + "..."
	}
	return s
}
