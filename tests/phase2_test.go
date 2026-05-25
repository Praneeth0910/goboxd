// goboxd_phase2_test.go
//
// END-TO-END HTTP TEST SUITE — Phase 1 + Phase 2
//
// Run against a live server:
//
//   export GOBOXD_URL=http://localhost:8080
//   go test -v -timeout 120s ./goboxd_phase2_test.go
//
// Or point at a different host:
//
//   GOBOXD_URL=http://staging:8080 go test -v -timeout 120s ./goboxd_phase2_test.go
//
// Requirements: the server must be running with languages.yaml that has
// at least py3 and cpp configured (Stage 1 baseline).
//
// Each test group is clearly labelled. Failures print the raw response body
// so you can debug without guessing.

package goboxd_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func baseURL(t *testing.T) string {
	t.Helper()
	u := os.Getenv("GOBOXD_URL")
	if u == "" {
		u = "http://localhost:8080"
	}
	return strings.TrimRight(u, "/")
}

func get(t *testing.T, path string) (int, []byte) {
	t.Helper()
	resp, err := http.Get(baseURL(t) + path)
	if err != nil {
		t.Fatalf("GET %s failed: %v", path, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, body
}

func post(t *testing.T, path string, payload any) (int, []byte) {
	t.Helper()
	b, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	resp, err := http.Post(baseURL(t)+path, "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatalf("POST %s failed: %v", path, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, body
}

// parseJSON decodes body into a map; fails the test on error.
func parseJSON(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatalf("response is not JSON: %v\nbody: %s", err, body)
	}
	return m
}

func assertStatus(t *testing.T, got, want int, body []byte) {
	t.Helper()
	if got != want {
		t.Errorf("HTTP status: got %d, want %d\nbody: %s", got, want, body)
	}
}

func assertField(t *testing.T, m map[string]any, key, want string) {
	t.Helper()
	v, ok := m[key]
	if !ok {
		t.Errorf("missing field %q in response", key)
		return
	}
	if fmt.Sprint(v) != want {
		t.Errorf("field %q: got %q, want %q", key, fmt.Sprint(v), want)
	}
}

func assertFieldPresent(t *testing.T, m map[string]any, key string) {
	t.Helper()
	if _, ok := m[key]; !ok {
		t.Errorf("expected field %q to be present in response", key)
	}
}

func assertContains(t *testing.T, body []byte, substr string) {
	t.Helper()
	if !strings.Contains(string(body), substr) {
		t.Errorf("response body does not contain %q\nbody: %s", substr, body)
	}
}

// runPayload builds a minimal POST /run request body.
type runPayload struct {
	Language         string     `json:"language"`
	Source           string     `json:"source"`
	SourceFilename   string     `json:"source_filename,omitempty"`
	ArtifactFilename string     `json:"artifact_filename,omitempty"`
	Build            *buildReq  `json:"build,omitempty"`
	Run              *runReq    `json:"run,omitempty"`
	Tests            []testCase `json:"tests"`
}

type buildReq struct {
	Flags  []string   `json:"flags,omitempty"`
	Limits *limitsReq `json:"limits,omitempty"`
}

type runReq struct {
	Limits *limitsReq `json:"limits,omitempty"`
}

type limitsReq struct {
	WallTimeS    int `json:"wall_time_s,omitempty"`
	MemoryKB     int `json:"memory_kb,omitempty"`
	MaxProcesses int `json:"max_processes,omitempty"`
}

type testCase struct {
	Stdin          string `json:"stdin"`
	ExpectedStdout string `json:"expected_stdout"`
}

// ---------------------------------------------------------------------------
// GROUP 1: Health & Readiness
// ---------------------------------------------------------------------------

// TestHealthz — GET /healthz must always return 200 {"status":"ok"}
func TestHealthz(t *testing.T) {
	code, body := get(t, "/healthz")
	assertStatus(t, code, 200, body)

	m := parseJSON(t, body)
	assertField(t, m, "status", "ok")
}

// TestReadyz — GET /readyz must return 200 when the server is ready.
// A degraded response (503) is allowed only when nsjail or a language is broken.
func TestReadyz(t *testing.T) {
	code, body := get(t, "/readyz")
	if code != 200 && code != 503 {
		t.Fatalf("unexpected HTTP status %d from /readyz\nbody: %s", code, body)
	}

	m := parseJSON(t, body)
	assertFieldPresent(t, m, "status")

	// The "nsjail" key must exist and carry an "ok" boolean
	nsjailRaw, ok := m["nsjail"]
	if !ok {
		t.Error("missing 'nsjail' key in /readyz response")
	} else {
		nsjail, _ := nsjailRaw.(map[string]any)
		if nsjail == nil {
			t.Error("'nsjail' is not an object")
		} else {
			assertFieldPresent(t, nsjail, "ok")
		}
	}

	// The "languages" key must exist and be a non-empty object
	langsRaw, ok := m["languages"]
	if !ok {
		t.Error("missing 'languages' key in /readyz response")
	} else {
		langs, _ := langsRaw.(map[string]any)
		if len(langs) == 0 {
			t.Error("'languages' map is empty — at least py3 and cpp must be registered")
		}
	}

	if code == 200 {
		assertField(t, m, "status", "ok")
	} else {
		assertField(t, m, "status", "degraded")
		t.Logf("WARN: server is degraded — check nsjail installation\nbody: %s", body)
	}
}

// ---------------------------------------------------------------------------
// GROUP 2: /info endpoint
// ---------------------------------------------------------------------------

// TestInfo_Shape checks all required top-level keys are present.
func TestInfo_Shape(t *testing.T) {
	code, body := get(t, "/info")
	assertStatus(t, code, 200, body)

	m := parseJSON(t, body)
	for _, key := range []string{"build_info", "nsjail", "languages", "limits", "stats"} {
		assertFieldPresent(t, m, key)
	}
}

// TestInfo_BuildInfo checks version/commit/go_version fields.
func TestInfo_BuildInfo(t *testing.T) {
	_, body := get(t, "/info")
	m := parseJSON(t, body)

	bi, _ := m["build_info"].(map[string]any)
	if bi == nil {
		t.Fatal("build_info is not an object")
	}
	for _, field := range []string{"version", "commit", "go_version"} {
		assertFieldPresent(t, bi, field)
	}
	// go_version must start with "go"
	if gv, ok := bi["go_version"].(string); !ok || !strings.HasPrefix(gv, "go") {
		t.Errorf("go_version should start with 'go', got %v", bi["go_version"])
	}
}

// TestInfo_Languages checks at least py3 and cpp are listed.
func TestInfo_Languages(t *testing.T) {
	_, body := get(t, "/info")
	m := parseJSON(t, body)

	langsRaw, _ := m["languages"].([]any)
	if len(langsRaw) == 0 {
		t.Fatal("'languages' array is empty")
	}

	ids := map[string]bool{}
	for _, l := range langsRaw {
		lang, _ := l.(map[string]any)
		if lang == nil {
			continue
		}
		id, _ := lang["id"].(string)
		ids[id] = true
		// Each language entry must have these fields
		for _, field := range []string{"id", "name"} {
			assertFieldPresent(t, lang, field)
		}
	}

	for _, required := range []string{"py3", "cpp"} {
		if !ids[required] {
			t.Errorf("language %q not found in /info languages array", required)
		}
	}
}

// TestInfo_Limits checks the global limits block.
func TestInfo_Limits(t *testing.T) {
	_, body := get(t, "/info")
	m := parseJSON(t, body)

	limits, _ := m["limits"].(map[string]any)
	if limits == nil {
		t.Fatal("'limits' is not an object")
	}
	for _, field := range []string{"max_source_bytes", "max_tests", "max_concurrent_jobs"} {
		assertFieldPresent(t, limits, field)
	}

	// max_source_bytes must be 262144 (per languages.yaml)
	if v, ok := limits["max_source_bytes"].(float64); ok {
		if int(v) != 262144 {
			t.Errorf("max_source_bytes: got %v, want 262144", v)
		}
	}
}

// TestInfo_Stats checks the stats block has expected counters.
func TestInfo_Stats(t *testing.T) {
	_, body := get(t, "/info")
	m := parseJSON(t, body)

	stats, _ := m["stats"].(map[string]any)
	if stats == nil {
		t.Fatal("'stats' is not an object")
	}
	for _, field := range []string{"in_flight_jobs", "jobs_total", "jobs_failed_internal"} {
		assertFieldPresent(t, stats, field)
	}
}

// ---------------------------------------------------------------------------
// GROUP 3: POST /run — happy path (requires nsjail + toolchains)
// ---------------------------------------------------------------------------

// TestRun_Python3_HelloWorld — trivial py3 execution, single test.
func TestRun_Python3_HelloWorld(t *testing.T) {
	payload := runPayload{
		Language: "py3",
		Source:   `print("hello world")`,
		Tests: []testCase{
			{Stdin: "", ExpectedStdout: "hello world\n"},
		},
	}
	code, body := post(t, "/run", payload)
	assertStatus(t, code, 200, body)

	m := parseJSON(t, body)
	assertField(t, m, "status", "accepted")

	tests := assertTestsArray(t, m, 1)
	assertTestStatus(t, tests, 0, "accepted")
}

// TestRun_Python3_MultiTest — multiple test cases, all accepted.
func TestRun_Python3_MultiTest(t *testing.T) {
	payload := runPayload{
		Language: "py3",
		Source: `
n = int(input())
print(n * 2)
`,
		Tests: []testCase{
			{Stdin: "3\n", ExpectedStdout: "6\n"},
			{Stdin: "10\n", ExpectedStdout: "20\n"},
			{Stdin: "0\n", ExpectedStdout: "0\n"},
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

// TestRun_Python3_WrongOutput — program gives wrong answer.
func TestRun_Python3_WrongOutput(t *testing.T) {
	payload := runPayload{
		Language: "py3",
		Source:   `print("wrong")`,
		Tests: []testCase{
			{Stdin: "", ExpectedStdout: "correct\n"},
		},
	}
	code, body := post(t, "/run", payload)
	assertStatus(t, code, 200, body) // always 200

	m := parseJSON(t, body)
	status, _ := m["status"].(string)
	if status != "wrong_output" && status != "output_whitespace_mismatch" {
		t.Errorf("top-level status: got %q, want wrong_output or output_whitespace_mismatch", status)
	}
}

// TestRun_Python3_RuntimeError — program crashes.
func TestRun_Python3_RuntimeError(t *testing.T) {
	payload := runPayload{
		Language: "py3",
		Source:   `raise ValueError("crash")`,
		Tests: []testCase{
			{Stdin: "", ExpectedStdout: ""},
		},
	}
	code, body := post(t, "/run", payload)
	assertStatus(t, code, 200, body)

	m := parseJSON(t, body)
	assertField(t, m, "status", "runtime_error")
}

// TestRun_Python3_WhitespaceMismatch — output differs only by trailing newline.
func TestRun_Python3_WhitespaceMismatch(t *testing.T) {
	payload := runPayload{
		Language: "py3",
		Source:   `print("hi")`, // prints "hi\n"
		Tests: []testCase{
			{Stdin: "", ExpectedStdout: "hi"}, // expected has no newline
		},
	}
	code, body := post(t, "/run", payload)
	assertStatus(t, code, 200, body)

	m := parseJSON(t, body)
	status, _ := m["status"].(string)
	if status != "output_whitespace_mismatch" && status != "accepted" {
		t.Logf("INFO: whitespace mismatch handling returned %q (acceptable)", status)
	}
}

// TestRun_Cpp_HelloWorld — compiled language happy path.
func TestRun_Cpp_HelloWorld(t *testing.T) {
	payload := runPayload{
		Language: "cpp",
		Source: `#include <iostream>
int main() {
    std::cout << "hello" << std::endl;
    return 0;
}`,
		Tests: []testCase{
			{Stdin: "", ExpectedStdout: "hello\n"},
		},
	}
	code, body := post(t, "/run", payload)
	assertStatus(t, code, 200, body)

	m := parseJSON(t, body)
	assertField(t, m, "status", "accepted")

	// C++ should expose a build result
	assertFieldPresent(t, m, "build")
}

// TestRun_Cpp_BuildFailed — compilation error causes build_failed.
func TestRun_Cpp_BuildFailed(t *testing.T) {
	payload := runPayload{
		Language: "cpp",
		Source:   `this is not valid cpp code at all {{{`,
		Tests: []testCase{
			{Stdin: "", ExpectedStdout: "anything"},
		},
	}
	code, body := post(t, "/run", payload)
	assertStatus(t, code, 200, body)

	m := parseJSON(t, body)
	assertField(t, m, "status", "build_failed")

	// All tests must be not_executed when build fails
	tests := assertTestsArray(t, m, 1)
	assertTestStatus(t, tests, 0, "not_executed")
}

// TestRun_Cpp_WithFlags — valid compiler flags are accepted.
func TestRun_Cpp_WithFlags(t *testing.T) {
	payload := runPayload{
		Language: "cpp",
		Source: `#include <iostream>
int main() { std::cout << "ok\n"; }`,
		Build: &buildReq{Flags: []string{"-O2", "-Wall"}},
		Tests: []testCase{
			{Stdin: "", ExpectedStdout: "ok\n"},
		},
	}
	code, body := post(t, "/run", payload)
	assertStatus(t, code, 200, body)
	assertContains(t, body, "accepted")
}

// TestRun_Cpp_StdFlag — glob flag -std=* matches a specific standard.
func TestRun_Cpp_StdFlag(t *testing.T) {
	payload := runPayload{
		Language: "cpp",
		Source: `#include <iostream>
int main() { std::cout << "ok\n"; }`,
		Build: &buildReq{Flags: []string{"-std=c++17", "-O2"}},
		Tests: []testCase{
			{Stdin: "", ExpectedStdout: "ok\n"},
		},
	}
	code, body := post(t, "/run", payload)
	assertStatus(t, code, 200, body)
	assertContains(t, body, "accepted")
}

// ---------------------------------------------------------------------------
// GROUP 4: POST /run — 400 validation rejections
// ---------------------------------------------------------------------------

// TestRun_UnknownLanguage — should return 400.
func TestRun_UnknownLanguage(t *testing.T) {
	payload := runPayload{
		Language: "brainfuck",
		Source:   `++++`,
		Tests:    []testCase{{Stdin: "", ExpectedStdout: ""}},
	}
	code, body := post(t, "/run", payload)
	assertStatus(t, code, 400, body)
	assertContains(t, body, "unknown_language")
}

// TestRun_PathTraversalFilename — security hole #1 — must return 400.
func TestRun_PathTraversalFilename(t *testing.T) {
	cases := []string{
		"../../etc/passwd",
		"../solution.py",
		"/etc/passwd",
		".hidden",
		"..",
		"dir/solution.cpp",
		"sol\\nested.cpp",
	}
	for _, filename := range cases {
		t.Run(filename, func(t *testing.T) {
			payload := runPayload{
				Language:       "py3",
				Source:         `print("hi")`,
				SourceFilename: filename,
				Tests:          []testCase{{Stdin: "", ExpectedStdout: "hi\n"}},
			}
			code, body := post(t, "/run", payload)
			assertStatus(t, code, 400, body)
			// Must reference filename validation in the error
			if !strings.Contains(string(body), "invalid_filename") &&
				!strings.Contains(string(body), "filename") {
				t.Errorf("expected filename error, got: %s", body)
			}
		})
	}
}

// TestRun_FlagInjection — security hole #3 — must return 400.
func TestRun_FlagInjection(t *testing.T) {
	attacks := [][]string{
		{"-fplugin=evil.so"},
		{"@response_file"},
		{"--specs=/tmp/evil.spec"},
		{"-Wl,-rpath,/evil"},
		{"-B/tmp/evil"},
		{"-x", "c"},
		{"-isystem", "/etc"},
		{"-O2", "-fplugin=evil.so"}, // mixed: valid + attack
	}
	for _, flags := range attacks {
		t.Run(strings.Join(flags, " "), func(t *testing.T) {
			payload := runPayload{
				Language: "cpp",
				Source:   `int main(){}`,
				Build:    &buildReq{Flags: flags},
				Tests:    []testCase{{Stdin: "", ExpectedStdout: ""}},
			}
			code, body := post(t, "/run", payload)
			assertStatus(t, code, 400, body)
			if !strings.Contains(string(body), "disallowed_flag") &&
				!strings.Contains(string(body), "flag") {
				t.Errorf("expected flag rejection error, got: %s", body)
			}
		})
	}
}

// TestRun_SourceTooLarge — security hole #4 — must return 400.
func TestRun_SourceTooLarge(t *testing.T) {
	bigSource := strings.Repeat("x", 300*1024) // 300 KiB > 256 KiB limit
	payload := runPayload{
		Language: "py3",
		Source:   bigSource,
		Tests:    []testCase{{Stdin: "", ExpectedStdout: ""}},
	}
	code, body := post(t, "/run", payload)
	assertStatus(t, code, 400, body)
}

// TestRun_TooManyTests — exceeds max_tests=50.
func TestRun_TooManyTests(t *testing.T) {
	tests := make([]testCase, 51)
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

// TestRun_EmptyTests — at least one test case required.
func TestRun_EmptyTests(t *testing.T) {
	payload := runPayload{
		Language: "py3",
		Source:   `print("hi")`,
		Tests:    []testCase{},
	}
	code, body := post(t, "/run", payload)
	// Servers may differ: some return 400, some run with 0 tests and return accepted
	// Either is acceptable; we just check it doesn't 500.
	if code == 500 {
		t.Errorf("server returned 500 for empty tests, should be 400 or 200\nbody: %s", body)
	}
}

// TestRun_InvalidJSON — malformed request body.
func TestRun_InvalidJSON(t *testing.T) {
	resp, err := http.Post(baseURL(t)+"/run", "application/json", strings.NewReader(`{bad json`))
	if err != nil {
		t.Fatalf("POST /run: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	assertStatus(t, resp.StatusCode, 400, body)
}

// ---------------------------------------------------------------------------
// GROUP 5: POST /run — result structure validation
// ---------------------------------------------------------------------------

// TestRun_ResponseShape — every required field is present in a normal run.
func TestRun_ResponseShape(t *testing.T) {
	payload := runPayload{
		Language: "py3",
		Source:   `print("hi")`,
		Tests:    []testCase{{Stdin: "", ExpectedStdout: "hi\n"}},
	}
	code, body := post(t, "/run", payload)
	assertStatus(t, code, 200, body)

	m := parseJSON(t, body)
	assertFieldPresent(t, m, "status")
	assertFieldPresent(t, m, "tests")

	tests, _ := m["tests"].([]any)
	if len(tests) != 1 {
		t.Fatalf("expected 1 test result, got %d", len(tests))
	}

	tr, _ := tests[0].(map[string]any)
	if tr == nil {
		t.Fatal("test result is not an object")
	}
	for _, field := range []string{"status", "stdout", "stderr"} {
		assertFieldPresent(t, tr, field)
	}
}

// TestRun_CppResponseShape — compiled language exposes build result.
func TestRun_CppResponseShape(t *testing.T) {
	payload := runPayload{
		Language: "cpp",
		Source:   `#include<iostream>\nint main(){std::cout<<"ok\n";}`,
		Tests:    []testCase{{Stdin: "", ExpectedStdout: "ok\n"}},
	}
	code, body := post(t, "/run", payload)
	assertStatus(t, code, 200, body)

	m := parseJSON(t, body)
	// Build key must exist for compiled languages
	if _, ok := m["build"]; !ok {
		t.Error("'build' key missing from /run response for a compiled language")
	}
}

// TestRun_StatusNeverFiveHundred — user code errors must never return 5xx.
func TestRun_StatusNeverFiveHundred(t *testing.T) {
	cases := []struct {
		name   string
		source string
	}{
		{"crash", `raise SystemExit(1)`},
		{"infinite_loop_will_tle", `while True: pass`},
		{"syntax_error", `def bad syntax:`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			payload := runPayload{
				Language: "py3",
				Source:   tc.source,
				Tests:    []testCase{{Stdin: "", ExpectedStdout: ""}},
			}
			code, body := post(t, "/run", payload)
			if code >= 500 {
				t.Errorf("user code failure returned %d; must be 200\nbody: %s", code, body)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// GROUP 6: Concurrency — bounded queue
// ---------------------------------------------------------------------------

// TestConcurrency_QueueNotFail — concurrent requests must queue, not fail.
// Sends 10 simultaneous requests and asserts every one returns 200.
func TestConcurrency_QueueNotFail(t *testing.T) {
	const n = 10
	var wg sync.WaitGroup
	errors := make([]string, n)

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			payload := runPayload{
				Language: "py3",
				Source:   fmt.Sprintf(`print(%d)`, idx),
				Tests:    []testCase{{Stdin: "", ExpectedStdout: fmt.Sprintf("%d\n", idx)}},
			}
			code, body := post(t, "/run", payload)
			if code != 200 {
				errors[idx] = fmt.Sprintf("goroutine %d: got %d\nbody: %s", idx, code, body)
			}
		}(i)
	}
	wg.Wait()

	for i, e := range errors {
		if e != "" {
			t.Errorf("concurrent request %d failed: %s", i, e)
		}
	}
}

// ---------------------------------------------------------------------------
// GROUP 7: Stats counter increments
// ---------------------------------------------------------------------------

// TestStats_JobsTotalIncrement — jobs_total must increase after a run.
func TestStats_JobsTotalIncrement(t *testing.T) {
	_, body1 := get(t, "/info")
	m1 := parseJSON(t, body1)
	stats1, _ := m1["stats"].(map[string]any)
	before, _ := stats1["jobs_total"].(float64)

	// Fire a job
	payload := runPayload{
		Language: "py3",
		Source:   `print("counter test")`,
		Tests:    []testCase{{Stdin: "", ExpectedStdout: "counter test\n"}},
	}
	post(t, "/run", payload)

	// Allow counter to settle
	time.Sleep(100 * time.Millisecond)

	_, body2 := get(t, "/info")
	m2 := parseJSON(t, body2)
	stats2, _ := m2["stats"].(map[string]any)
	after, _ := stats2["jobs_total"].(float64)

	if after <= before {
		t.Errorf("jobs_total did not increase: before=%v, after=%v", before, after)
	}
}

// ---------------------------------------------------------------------------
// GROUP 8: Content-Type headers
// ---------------------------------------------------------------------------

// TestContentType_JSON — all endpoints must return application/json.
func TestContentType_JSON(t *testing.T) {
	endpoints := []struct {
		method string
		path   string
	}{
		{"GET", "/healthz"},
		{"GET", "/readyz"},
		{"GET", "/info"},
	}
	for _, ep := range endpoints {
		t.Run(ep.method+" "+ep.path, func(t *testing.T) {
			resp, err := http.Get(baseURL(t) + ep.path)
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			resp.Body.Close()
			ct := resp.Header.Get("Content-Type")
			if !strings.Contains(ct, "application/json") {
				t.Errorf("%s %s: Content-Type = %q, want application/json", ep.method, ep.path, ct)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// GROUP 9: Edge cases
// ---------------------------------------------------------------------------

// TestRun_EmptyStdin — stdin may be empty string, must not panic.
func TestRun_EmptyStdin(t *testing.T) {
	payload := runPayload{
		Language: "py3",
		Source:   `print("no stdin needed")`,
		Tests:    []testCase{{Stdin: "", ExpectedStdout: "no stdin needed\n"}},
	}
	code, body := post(t, "/run", payload)
	assertStatus(t, code, 200, body)
	assertContains(t, body, "accepted")
}

// TestRun_LargeOutput — program producing substantial output; must not OOM.
func TestRun_LargeOutput(t *testing.T) {
	payload := runPayload{
		Language: "py3",
		// Produces ~100 KiB; well under the 1 MiB cap
		Source: `
for i in range(1000):
    print("A" * 100)
`,
		Tests: []testCase{{Stdin: "", ExpectedStdout: ""}},
	}
	code, body := post(t, "/run", payload)
	// Output won't match (expected is ""), so wrong_output is fine.
	// The key check is that the server didn't crash (not 500).
	if code == 500 {
		t.Errorf("server crashed on large output: %s", body)
	}
	assertStatus(t, code, 200, body)
}

// TestRun_NullByteInSource — must not crash the server.
func TestRun_NullByteInSource(t *testing.T) {
	// A null byte in a JSON string is non-standard; server should handle gracefully.
	raw := `{"language":"py3","source":"print(\"hi\")\x00","tests":[{"stdin":"","expected_stdout":"hi\n"}]}`
	resp, err := http.Post(baseURL(t)+"/run", "application/json", strings.NewReader(raw))
	if err != nil {
		t.Fatalf("POST /run: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	// Either 400 (invalid JSON) or 200 (safe handling) — never 500
	if resp.StatusCode == 500 {
		t.Errorf("null byte in source caused 500\nbody: %s", body)
	}
}

// TestRun_NoTestsField — omitting tests entirely.
func TestRun_NoTestsField(t *testing.T) {
	raw := `{"language":"py3","source":"print('hi')"}`
	resp, err := http.Post(baseURL(t)+"/run", "application/json", strings.NewReader(raw))
	if err != nil {
		t.Fatalf("POST /run: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	// Accept either 400 (required) or 200 (lenient); never 500
	if resp.StatusCode == 500 {
		t.Errorf("missing tests field caused 500\nbody: %s", body)
	}
}

// ---------------------------------------------------------------------------
// GROUP 10: Security — probe that known good filenames pass validation
// ---------------------------------------------------------------------------

// TestRun_ValidFilenames — valid filenames must NOT be rejected.
func TestRun_ValidFilenames(t *testing.T) {
	// These must all be accepted at the validation layer.
	// The run itself may fail (wrong language match) but must not return 400 for the filename.
	cases := []string{
		"solution.cpp",
		"solution_v2.py",
		"test-file.cpp",
		"README.md",
		"file123.txt",
		"something..",
		"header.h",
	}
	for _, fn := range cases {
		t.Run(fn, func(t *testing.T) {
			payload := runPayload{
				Language:       "py3",
				Source:         `print("hi")`,
				SourceFilename: fn,
				Tests:          []testCase{{Stdin: "", ExpectedStdout: "hi\n"}},
			}
			code, body := post(t, "/run", payload)
			// A 400 with "invalid_filename" means the validator is too strict
			if code == 400 && strings.Contains(string(body), "invalid_filename") {
				t.Errorf("valid filename %q was rejected: %s", fn, body)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Assertion helpers
// ---------------------------------------------------------------------------

func assertTestsArray(t *testing.T, m map[string]any, expectedCount int) []map[string]any {
	t.Helper()
	raw, ok := m["tests"]
	if !ok {
		t.Fatalf("missing 'tests' key in response")
	}
	arr, _ := raw.([]any)
	if len(arr) != expectedCount {
		t.Fatalf("expected %d test result(s), got %d", expectedCount, len(arr))
	}
	results := make([]map[string]any, len(arr))
	for i, item := range arr {
		tr, _ := item.(map[string]any)
		if tr == nil {
			t.Fatalf("test[%d] is not an object", i)
		}
		results[i] = tr
	}
	return results
}

func assertTestStatus(t *testing.T, tests []map[string]any, idx int, want string) {
	t.Helper()
	if idx >= len(tests) {
		t.Fatalf("test index %d out of range (len=%d)", idx, len(tests))
	}
	got, _ := tests[idx]["status"].(string)
	if got != want {
		t.Errorf("tests[%d].status: got %q, want %q", idx, got, want)
	}
}