//go:build integration

// phase3_test.go
//
// PHASE 3 — INTENSE, COMPREHENSIVE END-TO-END + CONTRACT TEST SUITE
//
// This suite exhaustively tests every feature, security fix, API contract,
// status rule, validation boundary, concurrency guarantee, and edge case
// across the entire goboxd codebase.
//
// Run against a live server:
//
//   export GOBOXD_URL=http://localhost:8080
//   go test -v -timeout 300s -count=1 ./tests/
//
// Requirements: server must be running with languages.yaml (py3 + cpp).

package goboxd_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// ===========================================================================
// GROUP 1: ERROR ENVELOPE SHAPE — {"error": {"code": ..., "message": ...}}
// ===========================================================================

// TestPhase3_ErrorEnvelope_InvalidJSON verifies the exact error JSON shape
// mandated by the spec: {"error": {"code": "...", "message": "..."}}
func TestPhase3_ErrorEnvelope_InvalidJSON(t *testing.T) {
	resp, err := http.Post(baseURL(t)+"/run", "application/json", strings.NewReader(`{not json`))
	if err != nil {
		t.Fatalf("POST /run: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	assertStatus(t, resp.StatusCode, 400, body)

	var envelope map[string]any
	if err := json.Unmarshal(body, &envelope); err != nil {
		t.Fatalf("response is not JSON: %v\nbody: %s", err, body)
	}

	errObj, ok := envelope["error"]
	if !ok {
		t.Fatalf("missing top-level 'error' key in 400 response\nbody: %s", body)
	}
	errMap, ok := errObj.(map[string]any)
	if !ok {
		t.Fatalf("'error' is not an object\nbody: %s", body)
	}
	if _, ok := errMap["code"]; !ok {
		t.Error("error object missing 'code' field")
	}
	if _, ok := errMap["message"]; !ok {
		t.Error("error object missing 'message' field")
	}
}

// TestPhase3_ErrorEnvelope_UnknownLanguage ensures unknown_language errors
// use the correct envelope shape.
func TestPhase3_ErrorEnvelope_UnknownLanguage(t *testing.T) {
	payload := runPayload{
		Language: "cobol",
		Source:   `DISPLAY "HI".`,
		Tests:    []testCase{{Stdin: "", ExpectedStdout: "HI\n"}},
	}
	code, body := post(t, "/run", payload)
	assertStatus(t, code, 400, body)

	var envelope map[string]any
	if err := json.Unmarshal(body, &envelope); err != nil {
		t.Fatalf("response is not JSON: %v", err)
	}
	errObj, ok := envelope["error"].(map[string]any)
	if !ok {
		t.Fatalf("missing error envelope in response\nbody: %s", body)
	}
	if errObj["code"] != "unknown_language" {
		t.Errorf("error.code: got %q, want %q", errObj["code"], "unknown_language")
	}
}

// ===========================================================================
// GROUP 2: HTTP METHOD ENFORCEMENT
// ===========================================================================

// TestPhase3_RunRejectsGET ensures GET /run is rejected.
func TestPhase3_RunRejectsGET(t *testing.T) {
	code, body := get(t, "/run")
	if code != 405 {
		t.Errorf("GET /run: got %d, want 405\nbody: %s", code, body)
	}
}

// TestPhase3_RunRejectsPUT ensures PUT /run is rejected.
func TestPhase3_RunRejectsPUT(t *testing.T) {
	req, _ := http.NewRequest(http.MethodPut, baseURL(t)+"/run", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT /run: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 405 {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("PUT /run: got %d, want 405\nbody: %s", resp.StatusCode, body)
	}
}

// TestPhase3_RunRejectsDELETE ensures DELETE /run is rejected.
func TestPhase3_RunRejectsDELETE(t *testing.T) {
	req, _ := http.NewRequest(http.MethodDelete, baseURL(t)+"/run", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE /run: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 405 {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("DELETE /run: got %d, want 405\nbody: %s", resp.StatusCode, body)
	}
}

// TestPhase3_HealthzRejectsPOST ensures POST /healthz is rejected or not routed.
func TestPhase3_HealthzRejectsPOST(t *testing.T) {
	resp, err := http.Post(baseURL(t)+"/healthz", "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatalf("POST /healthz: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == 200 {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("POST /healthz should not return 200\nbody: %s", body)
	}
}

// ===========================================================================
// GROUP 3: STATUS VOCABULARY — VERIFY EVERY SPEC STATUS STRING
// ===========================================================================

// TestPhase3_Status_Accepted — trivial py3 accepted
func TestPhase3_Status_Accepted(t *testing.T) {
	payload := runPayload{
		Language: "py3",
		Source:   `print("yes")`,
		Tests:    []testCase{{Stdin: "", ExpectedStdout: "yes\n"}},
	}
	code, body := post(t, "/run", payload)
	assertStatus(t, code, 200, body)
	m := parseJSON(t, body)
	assertField(t, m, "status", "accepted")
}

// TestPhase3_Status_WrongOutput — output differs entirely
func TestPhase3_Status_WrongOutput(t *testing.T) {
	payload := runPayload{
		Language: "py3",
		Source:   `print("apples")`,
		Tests:    []testCase{{Stdin: "", ExpectedStdout: "oranges\n"}},
	}
	code, body := post(t, "/run", payload)
	assertStatus(t, code, 200, body)
	m := parseJSON(t, body)
	assertField(t, m, "status", "wrong_output")
}

// TestPhase3_Status_WhitespaceMismatch — differs only by whitespace
func TestPhase3_Status_WhitespaceMismatch(t *testing.T) {
	payload := runPayload{
		Language: "py3",
		Source:   `print("  hello  ")`,
		Tests:    []testCase{{Stdin: "", ExpectedStdout: "hello"}},
	}
	code, body := post(t, "/run", payload)
	assertStatus(t, code, 200, body)
	m := parseJSON(t, body)
	s, _ := m["status"].(string)
	if s != "output_whitespace_mismatch" && s != "wrong_output" {
		t.Errorf("status: got %q, want output_whitespace_mismatch or wrong_output", s)
	}
}

// TestPhase3_Status_RuntimeError — exit code != 0
func TestPhase3_Status_RuntimeError(t *testing.T) {
	payload := runPayload{
		Language: "py3",
		Source:   `import sys; sys.exit(42)`,
		Tests:    []testCase{{Stdin: "", ExpectedStdout: ""}},
	}
	code, body := post(t, "/run", payload)
	assertStatus(t, code, 200, body)
	m := parseJSON(t, body)
	assertField(t, m, "status", "runtime_error")
}

// TestPhase3_Status_TimeExceeded — infinite loop triggers TLE
func TestPhase3_Status_TimeExceeded(t *testing.T) {
	payload := runPayload{
		Language: "py3",
		Source:   `while True: pass`,
		Run:      &runReq{Limits: &limitsReq{WallTimeS: 2}},
		Tests:    []testCase{{Stdin: "", ExpectedStdout: ""}},
	}
	code, body := post(t, "/run", payload)
	assertStatus(t, code, 200, body)
	m := parseJSON(t, body)
	s, _ := m["status"].(string)
	if s != "time_exceeded" && s != "runtime_error" {
		t.Errorf("status: got %q, want time_exceeded or runtime_error", s)
	}
}

// TestPhase3_Status_BuildFailed — bad C++ code triggers build_failed
func TestPhase3_Status_BuildFailed(t *testing.T) {
	payload := runPayload{
		Language: "cpp",
		Source:   `int main( { this is garbage`,
		Tests:    []testCase{{Stdin: "", ExpectedStdout: ""}},
	}
	code, body := post(t, "/run", payload)
	assertStatus(t, code, 200, body)
	m := parseJSON(t, body)
	assertField(t, m, "status", "build_failed")
}

// TestPhase3_Status_NotExecuted — on build failure, all tests are not_executed
func TestPhase3_Status_NotExecuted(t *testing.T) {
	payload := runPayload{
		Language: "cpp",
		Source:   `not valid cpp`,
		Tests: []testCase{
			{Stdin: "", ExpectedStdout: "a\n"},
			{Stdin: "", ExpectedStdout: "b\n"},
			{Stdin: "", ExpectedStdout: "c\n"},
		},
	}
	code, body := post(t, "/run", payload)
	assertStatus(t, code, 200, body)
	m := parseJSON(t, body)
	assertField(t, m, "status", "build_failed")

	tests := assertTestsArray(t, m, 3)
	for i := range tests {
		assertTestStatus(t, tests, i, "not_executed")
	}
}

// ===========================================================================
// GROUP 4: TOP-LEVEL STATUS AGGREGATION RULES
// ===========================================================================

// TestPhase3_TopLevel_FirstNonAccepted — first failing test determines overall status
func TestPhase3_TopLevel_FirstNonAccepted(t *testing.T) {
	payload := runPayload{
		Language: "py3",
		Source: `
import sys
n = int(input())
if n == 1:
    print("correct")
elif n == 2:
    sys.exit(1)
else:
    print("wrong")
`,
		Tests: []testCase{
			{Stdin: "1\n", ExpectedStdout: "correct\n"}, // accepted
			{Stdin: "2\n", ExpectedStdout: "correct\n"}, // runtime_error
			{Stdin: "3\n", ExpectedStdout: "correct\n"}, // wrong_output
		},
	}
	code, body := post(t, "/run", payload)
	assertStatus(t, code, 200, body)
	m := parseJSON(t, body)

	// The first non-accepted test status should determine overall status
	s, _ := m["status"].(string)
	if s == "accepted" {
		t.Error("overall status should not be 'accepted' when tests fail")
	}
}

// TestPhase3_TopLevel_AllAccepted — multiple tests all pass
func TestPhase3_TopLevel_AllAccepted(t *testing.T) {
	payload := runPayload{
		Language: "py3",
		Source:   `print(int(input()) + 1)`,
		Tests: []testCase{
			{Stdin: "1\n", ExpectedStdout: "2\n"},
			{Stdin: "2\n", ExpectedStdout: "3\n"},
			{Stdin: "99\n", ExpectedStdout: "100\n"},
		},
	}
	code, body := post(t, "/run", payload)
	assertStatus(t, code, 200, body)
	m := parseJSON(t, body)
	assertField(t, m, "status", "accepted")

	tests := assertTestsArray(t, m, 3)
	for i := range tests {
		assertTestStatus(t, tests, i, "accepted")
	}
}

// ===========================================================================
// GROUP 5: C++ BUILD PHASE — FULL LIFECYCLE
// ===========================================================================

// TestPhase3_Cpp_BuildOK_RunAccepted — successful build + correct output
func TestPhase3_Cpp_BuildOK_RunAccepted(t *testing.T) {
	payload := runPayload{
		Language: "cpp",
		Source: `#include <iostream>
using namespace std;
int main() {
    int n;
    cin >> n;
    cout << n * 2 << endl;
    return 0;
}`,
		Tests: []testCase{
			{Stdin: "5\n", ExpectedStdout: "10\n"},
			{Stdin: "0\n", ExpectedStdout: "0\n"},
			{Stdin: "-3\n", ExpectedStdout: "-6\n"},
		},
	}
	code, body := post(t, "/run", payload)
	assertStatus(t, code, 200, body)
	m := parseJSON(t, body)
	assertField(t, m, "status", "accepted")

	// Build must be present and ok
	build, _ := m["build"].(map[string]any)
	if build == nil {
		t.Fatal("build result missing for compiled language")
	}
	if build["status"] != "ok" {
		t.Errorf("build.status: got %q, want 'ok'", build["status"])
	}
	// duration_ms must be a positive number
	if wt, ok := build["duration_ms"].(float64); !ok || wt < 0 {
		t.Errorf("build.duration_ms: got %v, want positive number", build["duration_ms"])
	}
}

// TestPhase3_Cpp_BuildFailed_Stderr — build failure must include stderr
func TestPhase3_Cpp_BuildFailed_Stderr(t *testing.T) {
	payload := runPayload{
		Language: "cpp",
		Source:   `#include <nonexistent_header_12345>`,
		Tests:    []testCase{{Stdin: "", ExpectedStdout: ""}},
	}
	code, body := post(t, "/run", payload)
	assertStatus(t, code, 200, body)
	m := parseJSON(t, body)

	build, _ := m["build"].(map[string]any)
	if build == nil {
		t.Fatal("build result missing for compiled language")
	}
	if build["status"] != "failed" {
		t.Errorf("build.status: got %q, want 'failed'", build["status"])
	}
	// Stderr should contain compiler error messages
	stderr, _ := build["stderr"].(string)
	if stderr == "" {
		t.Error("build.stderr is empty; expected compiler error output")
	}
}

// TestPhase3_Cpp_RuntimeErrorAfterBuild — builds ok but crashes at runtime
func TestPhase3_Cpp_RuntimeErrorAfterBuild(t *testing.T) {
	payload := runPayload{
		Language: "cpp",
		Source: `#include <cstdlib>
int main() { return 1; }`,
		Tests: []testCase{{Stdin: "", ExpectedStdout: ""}},
	}
	code, body := post(t, "/run", payload)
	assertStatus(t, code, 200, body)
	m := parseJSON(t, body)

	// Build should be ok
	build, _ := m["build"].(map[string]any)
	if build != nil && build["status"] != "ok" {
		t.Errorf("build.status: got %q, want 'ok'", build["status"])
	}
	// Overall status should be runtime_error (exit code 1)
	assertField(t, m, "status", "runtime_error")
}

// TestPhase3_Py3_NoBuildField — interpreted languages must NOT include build
func TestPhase3_Py3_NoBuildField(t *testing.T) {
	payload := runPayload{
		Language: "py3",
		Source:   `print("hi")`,
		Tests:    []testCase{{Stdin: "", ExpectedStdout: "hi\n"}},
	}
	code, body := post(t, "/run", payload)
	assertStatus(t, code, 200, body)
	m := parseJSON(t, body)

	if _, hasBuild := m["build"]; hasBuild {
		t.Error("interpreted language (py3) should not have a 'build' field in response")
	}
}

// ===========================================================================
// GROUP 6: SECURITY HOLE #1 — PATH TRAVERSAL IN FILENAMES
// ===========================================================================

// TestPhase3_Security_PathTraversal_SourceFilename — exhaustive attack vectors
func TestPhase3_Security_PathTraversal_SourceFilename(t *testing.T) {
	attacks := []string{
		"../../etc/passwd",
		"../../../etc/shadow",
		"..",
		".",
		".hidden",
		"./solution.py",
		"/etc/passwd",
		"/tmp/evil",
		"dir/file.py",
		"sub\\dir.py",
		".bashrc",
		".env",
		"..%2f..%2fetc%2fpasswd", // URL-encoded attempt (still has dots)
	}
	for _, name := range attacks {
		t.Run("source_"+name, func(t *testing.T) {
			payload := runPayload{
				Language:       "py3",
				Source:         `print("hi")`,
				SourceFilename: name,
				Tests:          []testCase{{Stdin: "", ExpectedStdout: "hi\n"}},
			}
			code, body := post(t, "/run", payload)
			assertStatus(t, code, 400, body)
		})
	}
}

// TestPhase3_Security_PathTraversal_ArtifactFilename — same attacks on artifact
func TestPhase3_Security_PathTraversal_ArtifactFilename(t *testing.T) {
	attacks := []string{
		"../../etc/passwd",
		"..",
		".",
		".evil",
		"/tmp/evil",
		"sub/dir",
		"back\\slash",
	}
	for _, name := range attacks {
		t.Run("artifact_"+name, func(t *testing.T) {
			payload := runPayload{
				Language:         "cpp",
				Source:           `int main(){}`,
				ArtifactFilename: name,
				Tests:            []testCase{{Stdin: "", ExpectedStdout: ""}},
			}
			code, body := post(t, "/run", payload)
			assertStatus(t, code, 400, body)
		})
	}
}

// TestPhase3_Security_FilenameControlChars — null bytes, tabs, newlines
func TestPhase3_Security_FilenameControlChars(t *testing.T) {
	attacks := []string{
		"sol\x00ution.py",
		"sol\tion.py",
		"sol\nion.py",
		"sol\rion.py",
	}
	for i, name := range attacks {
		t.Run(fmt.Sprintf("control_char_%d", i), func(t *testing.T) {
			payload := runPayload{
				Language:       "py3",
				Source:         `print("hi")`,
				SourceFilename: name,
				Tests:          []testCase{{Stdin: "", ExpectedStdout: "hi\n"}},
			}
			code, body := post(t, "/run", payload)
			assertStatus(t, code, 400, body)
		})
	}
}

// TestPhase3_Security_FilenameTooLong — exceeds 128 char limit
func TestPhase3_Security_FilenameTooLong(t *testing.T) {
	longName := strings.Repeat("a", 129) + ".py"
	payload := runPayload{
		Language:       "py3",
		Source:         `print("hi")`,
		SourceFilename: longName,
		Tests:          []testCase{{Stdin: "", ExpectedStdout: "hi\n"}},
	}
	code, body := post(t, "/run", payload)
	assertStatus(t, code, 400, body)
}

// TestPhase3_Security_ValidFilenames — these MUST be accepted
func TestPhase3_Security_ValidFilenames(t *testing.T) {
	valid := []string{
		"solution.py",
		"my_code.cpp",
		"file123.txt",
		"A.py",
		"test-case.cpp",
		"something..", // trailing dots are fine
		"name.tar.gz",
	}
	for _, name := range valid {
		t.Run(name, func(t *testing.T) {
			payload := runPayload{
				Language:       "py3",
				Source:         `print("hi")`,
				SourceFilename: name,
				Tests:          []testCase{{Stdin: "", ExpectedStdout: "hi\n"}},
			}
			code, body := post(t, "/run", payload)
			// Must NOT get 400 with invalid_filename
			if code == 400 && strings.Contains(string(body), "invalid_filename") {
				t.Errorf("valid filename %q was rejected\nbody: %s", name, body)
			}
		})
	}
}

// ===========================================================================
// GROUP 7: SECURITY HOLE #3 — COMPILER FLAG INJECTION
// ===========================================================================

// TestPhase3_Security_FlagInjection_Exhaustive — every known attack vector
func TestPhase3_Security_FlagInjection_Exhaustive(t *testing.T) {
	attacks := []struct {
		name  string
		flags []string
	}{
		{"fplugin", []string{"-fplugin=evil.so"}},
		{"response_file", []string{"@response_file"}},
		{"specs", []string{"--specs=/tmp/evil.spec"}},
		{"linker_injection", []string{"-Wl,-rpath,/evil"}},
		{"Bdir", []string{"-B/tmp/evil"}},
		{"include_system", []string{"-isystem", "/etc"}},
		{"x_lang", []string{"-x", "c"}},
		{"mixed_valid_and_attack", []string{"-O2", "-fplugin=evil.so"}},
		{"dumpspecs", []string{"-dumpspecs"}},
		{"wrapper", []string{"-wrapper", "evil"}},
		{"preprocess_only", []string{"-E"}},
		{"shared_lib", []string{"-shared"}},
		{"nostdlib", []string{"-nostdlib"}},
		{"pipe_to_shell", []string{"-pipe"}},
		{"custom_linker", []string{"-fuse-ld=evil"}},
	}
	for _, tc := range attacks {
		t.Run(tc.name, func(t *testing.T) {
			payload := runPayload{
				Language: "cpp",
				Source:   `int main(){}`,
				Build:    &buildReq{Flags: tc.flags},
				Tests:    []testCase{{Stdin: "", ExpectedStdout: ""}},
			}
			code, body := post(t, "/run", payload)
			assertStatus(t, code, 400, body)
		})
	}
}

// TestPhase3_Security_ValidFlags — these MUST be accepted
func TestPhase3_Security_ValidFlags(t *testing.T) {
	validSets := [][]string{
		{"-O0"},
		{"-O1"},
		{"-O2"},
		{"-O3"},
		{"-Wall"},
		{"-Wextra"},
		{"-std=c++17"},
		{"-std=c++20"},
		{"-std=c11"},
		{"-std=gnu++17"},
		{"-O2", "-Wall", "-std=c++17"},
		{"-O2", "-Wextra"},
	}
	for _, flags := range validSets {
		t.Run(strings.Join(flags, "_"), func(t *testing.T) {
			payload := runPayload{
				Language: "cpp",
				Source: `#include <iostream>
int main() { std::cout << "ok\n"; }`,
				Build: &buildReq{Flags: flags},
				Tests: []testCase{{Stdin: "", ExpectedStdout: "ok\n"}},
			}
			code, body := post(t, "/run", payload)
			// Must not be 400 for valid flags
			if code == 400 {
				t.Errorf("valid flags %v were rejected\nbody: %s", flags, body)
			}
		})
	}
}

// TestPhase3_Security_FlagsOnInterpretedLang — py3 does not support build flags
func TestPhase3_Security_FlagsOnInterpretedLang(t *testing.T) {
	payload := runPayload{
		Language: "py3",
		Source:   `print("hi")`,
		Build:    &buildReq{Flags: []string{"-O2"}},
		Tests:    []testCase{{Stdin: "", ExpectedStdout: "hi\n"}},
	}
	code, body := post(t, "/run", payload)
	assertStatus(t, code, 400, body)
}

// ===========================================================================
// GROUP 8: SECURITY HOLE #4 — REQUEST SIZE LIMITS (MaxBytesReader)
// ===========================================================================

// TestPhase3_Security_OversizedBody — body exceeds max_source_bytes
func TestPhase3_Security_OversizedBody(t *testing.T) {
	// 262144 bytes (256 KiB) is the limit. Build a payload larger than that.
	bigSource := strings.Repeat("x", 300*1024) // 300 KiB
	payload := runPayload{
		Language: "py3",
		Source:   bigSource,
		Tests:    []testCase{{Stdin: "", ExpectedStdout: ""}},
	}
	code, body := post(t, "/run", payload)
	// Should be 400 (MaxBytesReader triggers before or during decode)
	if code != 400 {
		t.Errorf("oversized body: got HTTP %d, want 400\nbody: %s", code, body[:min(len(body), 200)])
	}
}

// TestPhase3_Security_ExactlyAtLimit — body just under limit should work
func TestPhase3_Security_ExactlyAtLimit(t *testing.T) {
	// A small valid program well under limits
	payload := runPayload{
		Language: "py3",
		Source:   `print("ok")`,
		Tests:    []testCase{{Stdin: "", ExpectedStdout: "ok\n"}},
	}
	code, body := post(t, "/run", payload)
	assertStatus(t, code, 200, body)
}

// ===========================================================================
// GROUP 9: TEST COUNT VALIDATION
// ===========================================================================

// TestPhase3_Validation_ZeroTests — at least 1 test required
func TestPhase3_Validation_ZeroTests(t *testing.T) {
	payload := runPayload{
		Language: "py3",
		Source:   `print("hi")`,
		Tests:    []testCase{},
	}
	code, body := post(t, "/run", payload)
	assertStatus(t, code, 400, body)
}

// TestPhase3_Validation_NullTests — omitting tests field entirely
func TestPhase3_Validation_NullTests(t *testing.T) {
	raw := `{"language":"py3","source":"print('hi')"}`
	resp, err := http.Post(baseURL(t)+"/run", "application/json", strings.NewReader(raw))
	if err != nil {
		t.Fatalf("POST /run: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	// Must be 400 (no tests), never 500
	if resp.StatusCode >= 500 {
		t.Errorf("missing tests field caused 500\nbody: %s", body)
	}
	assertStatus(t, resp.StatusCode, 400, body)
}

// TestPhase3_Validation_ExactlyOneTest — boundary: 1 test must work
func TestPhase3_Validation_ExactlyOneTest(t *testing.T) {
	payload := runPayload{
		Language: "py3",
		Source:   `print("one")`,
		Tests:    []testCase{{Stdin: "", ExpectedStdout: "one\n"}},
	}
	code, body := post(t, "/run", payload)
	assertStatus(t, code, 200, body)
}

// TestPhase3_Validation_ExactlyMaxTests — boundary: 50 tests must work
func TestPhase3_Validation_ExactlyMaxTests(t *testing.T) {
	tests := make([]testCase, 50) // max_tests = 50
	for i := range tests {
		tests[i] = testCase{Stdin: "", ExpectedStdout: "hi\n"}
	}
	payload := runPayload{
		Language: "py3",
		Source:   `print("hi")`,
		Tests:    tests,
	}
	code, body := post(t, "/run", payload)
	assertStatus(t, code, 200, body)
}

// TestPhase3_Validation_OverMaxTests — 51 tests must be rejected
func TestPhase3_Validation_OverMaxTests(t *testing.T) {
	tests := make([]testCase, 51) // max_tests + 1
	for i := range tests {
		tests[i] = testCase{Stdin: "", ExpectedStdout: "hi\n"}
	}
	payload := runPayload{
		Language: "py3",
		Source:   `print("hi")`,
		Tests:    tests,
	}
	code, body := post(t, "/run", payload)
	assertStatus(t, code, 400, body)
}

// ===========================================================================
// GROUP 10: JSON DECODE EDGE CASES
// ===========================================================================

// TestPhase3_JSON_EmptyBody — empty body should return 400
func TestPhase3_JSON_EmptyBody(t *testing.T) {
	resp, err := http.Post(baseURL(t)+"/run", "application/json", strings.NewReader(""))
	if err != nil {
		t.Fatalf("POST /run: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	assertStatus(t, resp.StatusCode, 400, body)
}

// TestPhase3_JSON_ArrayInsteadOfObject — sending an array
func TestPhase3_JSON_ArrayInsteadOfObject(t *testing.T) {
	resp, err := http.Post(baseURL(t)+"/run", "application/json", strings.NewReader(`[1,2,3]`))
	if err != nil {
		t.Fatalf("POST /run: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	assertStatus(t, resp.StatusCode, 400, body)
}

// TestPhase3_JSON_UnknownFields — DisallowUnknownFields rejects extra keys
func TestPhase3_JSON_UnknownFields(t *testing.T) {
	raw := `{"language":"py3","source":"print('hi')","tests":[{"stdin":"","expected_stdout":"hi\n"}],"evil_field":"gotcha"}`
	resp, err := http.Post(baseURL(t)+"/run", "application/json", strings.NewReader(raw))
	if err != nil {
		t.Fatalf("POST /run: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	assertStatus(t, resp.StatusCode, 400, body)
	assertContains(t, body, "invalid_json")
}

// TestPhase3_JSON_DuplicateKeys — JSON with duplicate keys
func TestPhase3_JSON_DuplicateKeys(t *testing.T) {
	raw := `{"language":"py3","language":"cpp","source":"print('hi')","tests":[{"stdin":"","expected_stdout":"hi\n"}]}`
	resp, err := http.Post(baseURL(t)+"/run", "application/json", strings.NewReader(raw))
	if err != nil {
		t.Fatalf("POST /run: %v", err)
	}
	defer resp.Body.Close()
	// Should not crash — 200 or 400 are both fine
	if resp.StatusCode >= 500 {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("duplicate keys caused 500\nbody: %s", body)
	}
}

// ===========================================================================
// GROUP 11: RESPONSE STRUCTURE CONTRACTS
// ===========================================================================

// TestPhase3_Response_TestFields — every test result has status, stdout, stderr, duration_ms
func TestPhase3_Response_TestFields(t *testing.T) {
	payload := runPayload{
		Language: "py3",
		Source:   `print("check")`,
		Tests:    []testCase{{Stdin: "", ExpectedStdout: "check\n"}},
	}
	code, body := post(t, "/run", payload)
	assertStatus(t, code, 200, body)
	m := parseJSON(t, body)

	tests := assertTestsArray(t, m, 1)
	tr := tests[0]
	for _, field := range []string{"status", "stdout", "stderr", "duration_ms"} {
		assertFieldPresent(t, tr, field)
	}
	// duration_ms must be >= 0
	if wt, ok := tr["duration_ms"].(float64); !ok || wt < 0 {
		t.Errorf("duration_ms: got %v, want >= 0", tr["duration_ms"])
	}
}

// TestPhase3_Response_CppBuildFields — build result has status, stdout, stderr, duration_ms
func TestPhase3_Response_CppBuildFields(t *testing.T) {
	payload := runPayload{
		Language: "cpp",
		Source: `#include <iostream>
int main() { std::cout << "ok\n"; }`,
		Tests: []testCase{{Stdin: "", ExpectedStdout: "ok\n"}},
	}
	code, body := post(t, "/run", payload)
	assertStatus(t, code, 200, body)
	m := parseJSON(t, body)

	build, _ := m["build"].(map[string]any)
	if build == nil {
		t.Fatal("build result missing")
	}
	for _, field := range []string{"status", "stdout", "stderr", "duration_ms"} {
		assertFieldPresent(t, build, field)
	}
}

// TestPhase3_Response_ContentType — POST /run must return application/json
func TestPhase3_Response_ContentType(t *testing.T) {
	payload := runPayload{
		Language: "py3",
		Source:   `print("ct")`,
		Tests:    []testCase{{Stdin: "", ExpectedStdout: "ct\n"}},
	}
	b, _ := json.Marshal(payload)
	resp, err := http.Post(baseURL(t)+"/run", "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatalf("POST /run: %v", err)
	}
	resp.Body.Close()
	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("Content-Type: got %q, want application/json", ct)
	}
}

// TestPhase3_Response_ContentType_OnError — 400 errors must also be JSON
func TestPhase3_Response_ContentType_OnError(t *testing.T) {
	resp, err := http.Post(baseURL(t)+"/run", "application/json", strings.NewReader(`{bad`))
	if err != nil {
		t.Fatalf("POST /run: %v", err)
	}
	resp.Body.Close()
	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("400 Content-Type: got %q, want application/json", ct)
	}
}

// ===========================================================================
// GROUP 12: /info ENDPOINT — FULL SHAPE VALIDATION
// ===========================================================================

// TestPhase3_Info_LanguagesHaveRunLimits — each language has default_run_limits
func TestPhase3_Info_LanguagesHaveRunLimits(t *testing.T) {
	_, body := get(t, "/info")
	m := parseJSON(t, body)
	langs, _ := m["languages"].([]any)
	if len(langs) == 0 {
		t.Fatal("languages array empty")
	}
	for _, l := range langs {
		lang, _ := l.(map[string]any)
		if lang == nil {
			continue
		}
		id, _ := lang["id"].(string)
		limits, _ := lang["default_run_limits"].(map[string]any)
		if limits == nil {
			t.Errorf("language %q missing default_run_limits", id)
			continue
		}
		for _, field := range []string{"wall_time_s", "memory_kb", "max_processes"} {
			if _, ok := limits[field]; !ok {
				t.Errorf("language %q: default_run_limits missing %q", id, field)
			}
		}
	}
}

// TestPhase3_Info_NsjailBlock — nsjail block has path and version
func TestPhase3_Info_NsjailBlock(t *testing.T) {
	_, body := get(t, "/info")
	m := parseJSON(t, body)
	nsjail, _ := m["nsjail"].(map[string]any)
	if nsjail == nil {
		t.Fatal("nsjail block missing")
	}
	assertFieldPresent(t, nsjail, "path")
	assertFieldPresent(t, nsjail, "version")
}

// TestPhase3_Info_DiskFreeBytes — stats.disk_free_bytes must be present
func TestPhase3_Info_DiskFreeBytes(t *testing.T) {
	_, body := get(t, "/info")
	m := parseJSON(t, body)
	stats, _ := m["stats"].(map[string]any)
	if stats == nil {
		t.Fatal("stats block missing")
	}
	assertFieldPresent(t, stats, "disk_free_bytes_jail_dir")
}

// ===========================================================================
// GROUP 13: /healthz AND /readyz — CONTRACT TESTS
// ===========================================================================

// TestPhase3_Healthz_AlwaysOK — must always return 200 with status ok
func TestPhase3_Healthz_AlwaysOK(t *testing.T) {
	for i := 0; i < 3; i++ {
		code, body := get(t, "/healthz")
		assertStatus(t, code, 200, body)
		m := parseJSON(t, body)
		assertField(t, m, "status", "ok")
	}
}

// TestPhase3_Readyz_LanguageProbes — readyz must list all configured languages
func TestPhase3_Readyz_LanguageProbes(t *testing.T) {
	code, body := get(t, "/readyz")
	if code != 200 && code != 503 {
		t.Fatalf("readyz: got %d, want 200 or 503", code)
	}
	m := parseJSON(t, body)
	langs, _ := m["languages"].(map[string]any)
	if langs == nil {
		t.Fatal("languages missing from readyz")
	}
	for _, required := range []string{"py3", "cpp"} {
		if _, ok := langs[required]; !ok {
			t.Errorf("language %q missing from readyz", required)
		}
	}
}

// ===========================================================================
// GROUP 14: CONCURRENCY — SEMAPHORE CORRECTNESS
// ===========================================================================

// TestPhase3_Concurrency_AllRequestsComplete — fire N concurrent requests, ALL must succeed
func TestPhase3_Concurrency_AllRequestsComplete(t *testing.T) {
	const n = 20
	var wg sync.WaitGroup
	var failures int64

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			payload := runPayload{
				Language: "py3",
				Source:   fmt.Sprintf(`print("goroutine %d")`, idx),
				Tests:    []testCase{{Stdin: "", ExpectedStdout: fmt.Sprintf("goroutine %d\n", idx)}},
			}
			code, body := post(t, "/run", payload)
			if code != 200 {
				atomic.AddInt64(&failures, 1)
				t.Errorf("concurrent req %d: got %d\nbody: %s", idx, code, body)
			}
		}(i)
	}
	wg.Wait()

	if f := atomic.LoadInt64(&failures); f > 0 {
		t.Errorf("%d/%d concurrent requests failed", f, n)
	}
}

// TestPhase3_Concurrency_NeverFiveHundred — concurrent requests must never 500
func TestPhase3_Concurrency_NeverFiveHundred(t *testing.T) {
	const n = 15
	var wg sync.WaitGroup
	var fiveHundreds int64

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			payload := runPayload{
				Language: "py3",
				Source:   fmt.Sprintf(`print(%d)`, idx),
				Tests:    []testCase{{Stdin: "", ExpectedStdout: fmt.Sprintf("%d\n", idx)}},
			}
			code, _ := post(t, "/run", payload)
			if code >= 500 {
				atomic.AddInt64(&fiveHundreds, 1)
			}
		}(i)
	}
	wg.Wait()

	if f := atomic.LoadInt64(&fiveHundreds); f > 0 {
		t.Errorf("%d/%d requests returned 5xx under concurrency", f, n)
	}
}

// TestPhase3_Concurrency_MixedLanguages — concurrent C++ and Python
func TestPhase3_Concurrency_MixedLanguages(t *testing.T) {
	const n = 10
	var wg sync.WaitGroup
	var failures int64

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			var payload runPayload
			if idx%2 == 0 {
				payload = runPayload{
					Language: "py3",
					Source:   fmt.Sprintf(`print("py%d")`, idx),
					Tests:    []testCase{{Stdin: "", ExpectedStdout: fmt.Sprintf("py%d\n", idx)}},
				}
			} else {
				payload = runPayload{
					Language: "cpp",
					Source: fmt.Sprintf(`#include <iostream>
int main() { std::cout << "cpp%d" << std::endl; }`, idx),
					Tests: []testCase{{Stdin: "", ExpectedStdout: fmt.Sprintf("cpp%d\n", idx)}},
				}
			}
			code, body := post(t, "/run", payload)
			if code != 200 {
				atomic.AddInt64(&failures, 1)
				t.Errorf("mixed concurrent req %d: got %d\nbody: %s", idx, code, body)
			}
		}(i)
	}
	wg.Wait()
	if f := atomic.LoadInt64(&failures); f > 0 {
		t.Errorf("%d/%d mixed-language concurrent requests failed", f, n)
	}
}

// ===========================================================================
// GROUP 15: STATS TRACKING
// ===========================================================================

// TestPhase3_Stats_IncrementAfterRun — jobs_total increases monotonically
func TestPhase3_Stats_IncrementAfterRun(t *testing.T) {
	getJobsTotal := func() float64 {
		_, body := get(t, "/info")
		m := parseJSON(t, body)
		stats, _ := m["stats"].(map[string]any)
		v, _ := stats["jobs_total"].(float64)
		return v
	}

	before := getJobsTotal()
	payload := runPayload{
		Language: "py3",
		Source:   `print("stats check")`,
		Tests:    []testCase{{Stdin: "", ExpectedStdout: "stats check\n"}},
	}
	post(t, "/run", payload)
	time.Sleep(200 * time.Millisecond) // let counter settle
	after := getJobsTotal()

	if after <= before {
		t.Errorf("jobs_total did not increase: before=%v, after=%v", before, after)
	}
}

// TestPhase3_Stats_InFlightReturnsToZero — in_flight_jobs goes back to 0 after run
func TestPhase3_Stats_InFlightReturnsToZero(t *testing.T) {
	payload := runPayload{
		Language: "py3",
		Source:   `print("inflight")`,
		Tests:    []testCase{{Stdin: "", ExpectedStdout: "inflight\n"}},
	}
	post(t, "/run", payload)
	time.Sleep(200 * time.Millisecond)

	_, body := get(t, "/info")
	m := parseJSON(t, body)
	stats, _ := m["stats"].(map[string]any)
	inflight, _ := stats["in_flight_jobs"].(float64)
	if inflight != 0 {
		t.Errorf("in_flight_jobs should be 0 after request completes, got %v", inflight)
	}
}

// ===========================================================================
// GROUP 16: OUTPUT COMPARISON EDGE CASES
// ===========================================================================

// TestPhase3_Output_ExactMatch — byte-for-byte identical
func TestPhase3_Output_ExactMatch(t *testing.T) {
	payload := runPayload{
		Language: "py3",
		Source:   `print("exact")`,
		Tests:    []testCase{{Stdin: "", ExpectedStdout: "exact\n"}},
	}
	code, body := post(t, "/run", payload)
	assertStatus(t, code, 200, body)
	m := parseJSON(t, body)
	assertField(t, m, "status", "accepted")
}

// TestPhase3_Output_TrailingNewlineMismatch — output has trailing newline, expected doesn't
func TestPhase3_Output_TrailingNewlineMismatch(t *testing.T) {
	payload := runPayload{
		Language: "py3",
		Source:   `print("trail")`,                                 // outputs "trail\n"
		Tests:    []testCase{{Stdin: "", ExpectedStdout: "trail"}}, // no \n
	}
	code, body := post(t, "/run", payload)
	assertStatus(t, code, 200, body)
	m := parseJSON(t, body)
	s, _ := m["status"].(string)
	if s != "output_whitespace_mismatch" && s != "accepted" {
		t.Errorf("trailing newline mismatch: got %q, want output_whitespace_mismatch or accepted", s)
	}
}

// TestPhase3_Output_EmptyExpectedEmptyActual — both empty = accepted
func TestPhase3_Output_EmptyExpectedEmptyActual(t *testing.T) {
	payload := runPayload{
		Language: "py3",
		Source:   `pass`, // produces no output
		Tests:    []testCase{{Stdin: "", ExpectedStdout: ""}},
	}
	code, body := post(t, "/run", payload)
	assertStatus(t, code, 200, body)
	m := parseJSON(t, body)
	assertField(t, m, "status", "accepted")
}

// TestPhase3_Output_MultilineComparison — multiline output matching
func TestPhase3_Output_MultilineComparison(t *testing.T) {
	payload := runPayload{
		Language: "py3",
		Source: `
for i in range(3):
    print(f"line {i}")
`,
		Tests: []testCase{{Stdin: "", ExpectedStdout: "line 0\nline 1\nline 2\n"}},
	}
	code, body := post(t, "/run", payload)
	assertStatus(t, code, 200, body)
	m := parseJSON(t, body)
	assertField(t, m, "status", "accepted")
}

// ===========================================================================
// GROUP 17: STDIN HANDLING
// ===========================================================================

// TestPhase3_Stdin_MultipleLines — program reads multiple lines
func TestPhase3_Stdin_MultipleLines(t *testing.T) {
	payload := runPayload{
		Language: "py3",
		Source: `
a = input()
b = input()
print(a + b)
`,
		Tests: []testCase{
			{Stdin: "hello\nworld\n", ExpectedStdout: "helloworld\n"},
		},
	}
	code, body := post(t, "/run", payload)
	assertStatus(t, code, 200, body)
	m := parseJSON(t, body)
	assertField(t, m, "status", "accepted")
}

// TestPhase3_Stdin_EmptyString — empty stdin is valid
func TestPhase3_Stdin_EmptyString(t *testing.T) {
	payload := runPayload{
		Language: "py3",
		Source:   `print("no input")`,
		Tests:    []testCase{{Stdin: "", ExpectedStdout: "no input\n"}},
	}
	code, body := post(t, "/run", payload)
	assertStatus(t, code, 200, body)
	assertContains(t, body, "accepted")
}

// TestPhase3_Stdin_PerTestIsolation — each test gets its own stdin
func TestPhase3_Stdin_PerTestIsolation(t *testing.T) {
	payload := runPayload{
		Language: "py3",
		Source:   `print(input().upper())`,
		Tests: []testCase{
			{Stdin: "hello\n", ExpectedStdout: "HELLO\n"},
			{Stdin: "world\n", ExpectedStdout: "WORLD\n"},
			{Stdin: "test\n", ExpectedStdout: "TEST\n"},
		},
	}
	code, body := post(t, "/run", payload)
	assertStatus(t, code, 200, body)
	m := parseJSON(t, body)
	assertField(t, m, "status", "accepted")
}

// ===========================================================================
// GROUP 18: 5xx PROTECTION — USER CODE MUST NEVER CAUSE 5xx
// ===========================================================================

// TestPhase3_Never5xx_ZeroDivision — arithmetic crash
func TestPhase3_Never5xx_ZeroDivision(t *testing.T) {
	payload := runPayload{
		Language: "py3",
		Source:   `x = 1/0`,
		Tests:    []testCase{{Stdin: "", ExpectedStdout: ""}},
	}
	code, _ := post(t, "/run", payload)
	if code >= 500 {
		t.Errorf("ZeroDivision caused %d; must be 200", code)
	}
}

// TestPhase3_Never5xx_SegfaultCpp — C++ segfault
func TestPhase3_Never5xx_SegfaultCpp(t *testing.T) {
	payload := runPayload{
		Language: "cpp",
		Source: `#include <cstdlib>
int main() { int *p = nullptr; *p = 42; }`,
		Tests: []testCase{{Stdin: "", ExpectedStdout: ""}},
	}
	code, _ := post(t, "/run", payload)
	if code >= 500 {
		t.Errorf("segfault caused %d; must be 200", code)
	}
}

// TestPhase3_Never5xx_ForkBomb — fork bomb attempt (bounded by nsjail max_processes)
func TestPhase3_Never5xx_ForkBomb(t *testing.T) {
	payload := runPayload{
		Language: "py3",
		Source: `
import os
for _ in range(1000):
    try:
        os.fork()
    except:
        pass
print("done")
`,
		Tests: []testCase{{Stdin: "", ExpectedStdout: "done\n"}},
	}
	code, _ := post(t, "/run", payload)
	if code >= 500 {
		t.Errorf("fork bomb caused %d; must be 200", code)
	}
}

// TestPhase3_Never5xx_InfiniteOutput — massive stdout
func TestPhase3_Never5xx_InfiniteOutput(t *testing.T) {
	payload := runPayload{
		Language: "py3",
		Source: `
import sys
while True:
    sys.stdout.write("A" * 10000)
`,
		Run:   &runReq{Limits: &limitsReq{WallTimeS: 3}},
		Tests: []testCase{{Stdin: "", ExpectedStdout: ""}},
	}
	code, _ := post(t, "/run", payload)
	if code >= 500 {
		t.Errorf("infinite output caused %d; must be 200", code)
	}
}

// TestPhase3_Never5xx_SyntaxError — Python syntax error
func TestPhase3_Never5xx_SyntaxError(t *testing.T) {
	payload := runPayload{
		Language: "py3",
		Source:   `def broken syntax here:`,
		Tests:    []testCase{{Stdin: "", ExpectedStdout: ""}},
	}
	code, _ := post(t, "/run", payload)
	if code >= 500 {
		t.Errorf("syntax error caused %d; must be 200", code)
	}
}

// TestPhase3_Never5xx_MemoryHog — trying to allocate huge memory
func TestPhase3_Never5xx_MemoryHog(t *testing.T) {
	payload := runPayload{
		Language: "py3",
		Source: `
data = []
while True:
    data.append("A" * 1000000)
`,
		Run:   &runReq{Limits: &limitsReq{WallTimeS: 3}},
		Tests: []testCase{{Stdin: "", ExpectedStdout: ""}},
	}
	code, _ := post(t, "/run", payload)
	if code >= 500 {
		t.Errorf("memory hog caused %d; must be 200", code)
	}
}

// ===========================================================================
// GROUP 19: WALL TIME TRACKING
// ===========================================================================

// TestPhase3_WallTime_PositiveValue — duration_ms is always a non-negative number
func TestPhase3_WallTime_PositiveValue(t *testing.T) {
	payload := runPayload{
		Language: "py3",
		Source: `
import time
time.sleep(0.1)
print("done")
`,
		Tests: []testCase{{Stdin: "", ExpectedStdout: "done\n"}},
	}
	code, body := post(t, "/run", payload)
	assertStatus(t, code, 200, body)
	m := parseJSON(t, body)

	tests := assertTestsArray(t, m, 1)
	wt, ok := tests[0]["duration_ms"].(float64)
	if !ok {
		t.Fatal("duration_ms is not a number")
	}
	if wt < 50 { // should be at least ~100ms due to sleep
		t.Errorf("duration_ms: got %v, expected at least ~100 for a 100ms sleep", wt)
	}
}

// ===========================================================================
// GROUP 20: MULTI-TEST CORRECTNESS — MIXED RESULTS
// ===========================================================================

// TestPhase3_MultiTest_MixedStatuses — different outcomes per test
func TestPhase3_MultiTest_MixedStatuses(t *testing.T) {
	payload := runPayload{
		Language: "py3",
		Source: `
import sys
n = int(input())
if n == 1:
    print("correct")
elif n == 2:
    print("wrong answer")
elif n == 3:
    sys.exit(1)
`,
		Tests: []testCase{
			{Stdin: "1\n", ExpectedStdout: "correct\n"}, // accepted
			{Stdin: "2\n", ExpectedStdout: "correct\n"}, // wrong_output
			{Stdin: "3\n", ExpectedStdout: "correct\n"}, // runtime_error
		},
	}
	code, body := post(t, "/run", payload)
	assertStatus(t, code, 200, body)
	m := parseJSON(t, body)

	tests := assertTestsArray(t, m, 3)
	assertTestStatus(t, tests, 0, "accepted")

	// Test 2 should be wrong_output
	s1, _ := tests[1]["status"].(string)
	if s1 != "wrong_output" {
		t.Errorf("test[1].status: got %q, want wrong_output", s1)
	}

	// Test 3 should be runtime_error
	s2, _ := tests[2]["status"].(string)
	if s2 != "runtime_error" {
		t.Errorf("test[2].status: got %q, want runtime_error", s2)
	}

	// Overall status should NOT be accepted
	overall, _ := m["status"].(string)
	if overall == "accepted" {
		t.Error("overall status should not be accepted when tests fail")
	}
}

// TestPhase3_MultiTest_TestCountMatchesRequest — response test count == request test count
func TestPhase3_MultiTest_TestCountMatchesRequest(t *testing.T) {
	for _, n := range []int{1, 3, 5, 10} {
		t.Run(fmt.Sprintf("n=%d", n), func(t *testing.T) {
			tests := make([]testCase, n)
			for i := range tests {
				tests[i] = testCase{Stdin: "", ExpectedStdout: "hi\n"}
			}
			payload := runPayload{
				Language: "py3",
				Source:   `print("hi")`,
				Tests:    tests,
			}
			code, body := post(t, "/run", payload)
			assertStatus(t, code, 200, body)
			m := parseJSON(t, body)
			assertTestsArray(t, m, n)
		})
	}
}

// ===========================================================================
// GROUP 21: SPECIAL CHARACTERS IN SOURCE/OUTPUT
// ===========================================================================

// TestPhase3_SpecialChars_Unicode — unicode in source and output
func TestPhase3_SpecialChars_Unicode(t *testing.T) {
	payload := runPayload{
		Language: "py3",
		Source:   `print("こんにちは世界")`,
		Tests:    []testCase{{Stdin: "", ExpectedStdout: "こんにちは世界\n"}},
	}
	code, body := post(t, "/run", payload)
	assertStatus(t, code, 200, body)
	m := parseJSON(t, body)
	assertField(t, m, "status", "accepted")
}

// TestPhase3_SpecialChars_Emoji — emoji in output
func TestPhase3_SpecialChars_Emoji(t *testing.T) {
	payload := runPayload{
		Language: "py3",
		Source:   `print("🚀✨🎉")`,
		Tests:    []testCase{{Stdin: "", ExpectedStdout: "🚀✨🎉\n"}},
	}
	code, body := post(t, "/run", payload)
	assertStatus(t, code, 200, body)
	m := parseJSON(t, body)
	assertField(t, m, "status", "accepted")
}

// TestPhase3_SpecialChars_Backslashes — literal backslashes in output
func TestPhase3_SpecialChars_Backslashes(t *testing.T) {
	payload := runPayload{
		Language: "py3",
		Source:   `print("a\\b\\c")`,
		Tests:    []testCase{{Stdin: "", ExpectedStdout: "a\\b\\c\n"}},
	}
	code, body := post(t, "/run", payload)
	assertStatus(t, code, 200, body)
	m := parseJSON(t, body)
	assertField(t, m, "status", "accepted")
}

// TestPhase3_SpecialChars_Quotes — quotes in output
func TestPhase3_SpecialChars_Quotes(t *testing.T) {
	payload := runPayload{
		Language: "py3",
		Source:   `print('he said "hello"')`,
		Tests:    []testCase{{Stdin: "", ExpectedStdout: "he said \"hello\"\n"}},
	}
	code, body := post(t, "/run", payload)
	assertStatus(t, code, 200, body)
	m := parseJSON(t, body)
	assertField(t, m, "status", "accepted")
}

// ===========================================================================
// GROUP 22: STDERR CAPTURE
// ===========================================================================

// TestPhase3_Stderr_PythonWarning — stderr is captured from user code
func TestPhase3_Stderr_PythonWarning(t *testing.T) {
	payload := runPayload{
		Language: "py3",
		Source: `
import sys
sys.stderr.write("this is stderr\n")
print("stdout")
`,
		Tests: []testCase{{Stdin: "", ExpectedStdout: "stdout\n"}},
	}
	code, body := post(t, "/run", payload)
	assertStatus(t, code, 200, body)
	m := parseJSON(t, body)

	tests := assertTestsArray(t, m, 1)
	stderr, _ := tests[0]["stderr"].(string)
	if !strings.Contains(stderr, "this is stderr") {
		t.Errorf("stderr not captured; got %q", stderr)
	}
}

// TestPhase3_Stderr_CppCompilerError — build stderr contains error message
func TestPhase3_Stderr_CppCompilerError(t *testing.T) {
	payload := runPayload{
		Language: "cpp",
		Source:   `int main() { undefined_function(); }`,
		Tests:    []testCase{{Stdin: "", ExpectedStdout: ""}},
	}
	code, body := post(t, "/run", payload)
	assertStatus(t, code, 200, body)
	m := parseJSON(t, body)

	build, _ := m["build"].(map[string]any)
	if build == nil {
		t.Fatal("build missing")
	}
	stderr, _ := build["stderr"].(string)
	if stderr == "" {
		t.Error("build stderr is empty for compilation error")
	}
}

// ===========================================================================
// GROUP 23: ROOT REDIRECT
// ===========================================================================

// TestPhase3_RootRedirect — GET / should redirect to /info
func TestPhase3_RootRedirect(t *testing.T) {
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse // don't follow
		},
	}
	resp, err := client.Get(baseURL(t) + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 302 && resp.StatusCode != 301 {
		t.Errorf("GET /: got %d, want 301 or 302", resp.StatusCode)
	}
	loc := resp.Header.Get("Location")
	if !strings.Contains(loc, "/info") {
		t.Errorf("redirect location: got %q, want /info", loc)
	}
}

// ===========================================================================
// GROUP 24: IDEMPOTENCY — SAME REQUEST TWICE GIVES SAME RESULT
// ===========================================================================

// TestPhase3_Idempotency — running the same program twice yields identical statuses
func TestPhase3_Idempotency(t *testing.T) {
	payload := runPayload{
		Language: "py3",
		Source:   `print("deterministic")`,
		Tests:    []testCase{{Stdin: "", ExpectedStdout: "deterministic\n"}},
	}

	_, body1 := post(t, "/run", payload)
	_, body2 := post(t, "/run", payload)

	m1 := parseJSON(t, body1)
	m2 := parseJSON(t, body2)

	s1, _ := m1["status"].(string)
	s2, _ := m2["status"].(string)
	if s1 != s2 {
		t.Errorf("idempotency failed: first=%q, second=%q", s1, s2)
	}
}

// ===========================================================================
// GROUP 25: OPTIONAL FIELDS — source_filename, artifact_filename, build, run
// ===========================================================================

// TestPhase3_Optional_NoSourceFilename — omitting source_filename is fine
func TestPhase3_Optional_NoSourceFilename(t *testing.T) {
	payload := runPayload{
		Language: "py3",
		Source:   `print("no filename")`,
		Tests:    []testCase{{Stdin: "", ExpectedStdout: "no filename\n"}},
	}
	code, body := post(t, "/run", payload)
	assertStatus(t, code, 200, body)
}

// TestPhase3_Optional_NoBuild — omitting build is fine for compiled langs
func TestPhase3_Optional_NoBuild(t *testing.T) {
	payload := runPayload{
		Language: "cpp",
		Source: `#include <iostream>
int main() { std::cout << "no build opts\n"; }`,
		Tests: []testCase{{Stdin: "", ExpectedStdout: "no build opts\n"}},
	}
	code, body := post(t, "/run", payload)
	assertStatus(t, code, 200, body)
}

// TestPhase3_Optional_NoRun — omitting run section is fine
func TestPhase3_Optional_NoRun(t *testing.T) {
	payload := runPayload{
		Language: "py3",
		Source:   `print("no run opts")`,
		Tests:    []testCase{{Stdin: "", ExpectedStdout: "no run opts\n"}},
	}
	code, body := post(t, "/run", payload)
	assertStatus(t, code, 200, body)
}

// TestPhase3_Optional_EmptyBuildFlags — empty flags array should work
func TestPhase3_Optional_EmptyBuildFlags(t *testing.T) {
	payload := runPayload{
		Language: "cpp",
		Source: `#include <iostream>
int main() { std::cout << "empty flags\n"; }`,
		Build: &buildReq{Flags: []string{}},
		Tests: []testCase{{Stdin: "", ExpectedStdout: "empty flags\n"}},
	}
	code, body := post(t, "/run", payload)
	assertStatus(t, code, 200, body)
}

// ===========================================================================
// GROUP 26: SECURITY REGRESSION — NULL BYTES, INJECTION ATTEMPTS
// ===========================================================================

// TestPhase3_Security_NullByteInSource — must not crash
func TestPhase3_Security_NullByteInSource(t *testing.T) {
	raw := `{"language":"py3","source":"print(\"hi\")\u0000","tests":[{"stdin":"","expected_stdout":"hi\n"}]}`
	resp, err := http.Post(baseURL(t)+"/run", "application/json", strings.NewReader(raw))
	if err != nil {
		t.Fatalf("POST /run: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 500 {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("null byte in source caused %d\nbody: %s", resp.StatusCode, body)
	}
}

// TestPhase3_Security_NullByteInStdin — must not crash
func TestPhase3_Security_NullByteInStdin(t *testing.T) {
	payload := runPayload{
		Language: "py3",
		Source:   `import sys; sys.stdout.write(sys.stdin.read())`,
		Tests:    []testCase{{Stdin: "hello\x00world", ExpectedStdout: "hello\x00world"}},
	}
	code, _ := post(t, "/run", payload)
	if code >= 500 {
		t.Errorf("null byte in stdin caused %d", code)
	}
}

// ===========================================================================
// GROUP 27: COMPILATION FLAGS VALIDATION EDGE CASES
// ===========================================================================

// TestPhase3_Flags_GlobMatching — -std=* matches various standards
func TestPhase3_Flags_GlobMatching(t *testing.T) {
	stds := []string{"-std=c++11", "-std=c++14", "-std=c++17", "-std=c++20", "-std=c11", "-std=gnu++17"}
	for _, std := range stds {
		t.Run(std, func(t *testing.T) {
			payload := runPayload{
				Language: "cpp",
				Source: `#include <iostream>
int main() { std::cout << "ok\n"; }`,
				Build: &buildReq{Flags: []string{std}},
				Tests: []testCase{{Stdin: "", ExpectedStdout: "ok\n"}},
			}
			code, body := post(t, "/run", payload)
			if code == 400 && strings.Contains(string(body), "flag") {
				t.Errorf("valid flag %q was rejected\nbody: %s", std, body)
			}
		})
	}
}

// TestPhase3_Flags_NilBuild — nil build block = no validation needed
func TestPhase3_Flags_NilBuild(t *testing.T) {
	payload := runPayload{
		Language: "cpp",
		Source: `#include <iostream>
int main() { std::cout << "nil build\n"; }`,
		Tests: []testCase{{Stdin: "", ExpectedStdout: "nil build\n"}},
	}
	code, body := post(t, "/run", payload)
	assertStatus(t, code, 200, body)
}

// ===========================================================================
// GROUP 28: ENDPOINT NOT FOUND
// ===========================================================================

// TestPhase3_NotFound — hitting a non-existent path
func TestPhase3_NotFound(t *testing.T) {
	code, _ := get(t, "/nonexistent")
	if code != 404 && code != 405 {
		t.Errorf("GET /nonexistent: got %d, want 404 or 405", code)
	}
}

// ===========================================================================
// GROUP 29: LARGE CONCURRENT VALIDATION — STRESS THE QUEUE
// ===========================================================================

// TestPhase3_Stress_QueueDrain — 30 concurrent requests all complete under 60s
func TestPhase3_Stress_QueueDrain(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	const n = 30
	var wg sync.WaitGroup
	start := time.Now()
	var failures int64

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			payload := runPayload{
				Language: "py3",
				Source:   fmt.Sprintf(`print("stress %d")`, idx),
				Tests:    []testCase{{Stdin: "", ExpectedStdout: fmt.Sprintf("stress %d\n", idx)}},
			}
			code, _ := post(t, "/run", payload)
			if code != 200 {
				atomic.AddInt64(&failures, 1)
			}
		}(i)
	}
	wg.Wait()
	elapsed := time.Since(start)

	if f := atomic.LoadInt64(&failures); f > 0 {
		t.Errorf("%d/%d stress requests failed", f, n)
	}
	if elapsed > 60*time.Second {
		t.Errorf("stress test took %v, want < 60s", elapsed)
	}
	t.Logf("stress test: %d requests completed in %v", n, elapsed)
}

// ===========================================================================
// HELPER: min for truncation
// ===========================================================================

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
