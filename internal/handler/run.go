package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/thesouldev/goboxd/internal/config"
	"github.com/thesouldev/goboxd/internal/runner"
	"github.com/thesouldev/goboxd/internal/stats"
	"github.com/thesouldev/goboxd/internal/status"
	"github.com/thesouldev/goboxd/internal/validate"
)

// ---------------------------------------------------------------------------
// Request / Response types
// ---------------------------------------------------------------------------

// RunRequest is the POST /run request body.
type RunRequest struct {
	Language         string          `json:"language"`
	Source           string          `json:"source"`
	SourceFilename   string          `json:"source_filename,omitempty"`
	ArtifactFilename string          `json:"artifact_filename,omitempty"`
	Build            *BuildOptions   `json:"build,omitempty"`
	Run              *RunOptions     `json:"run,omitempty"`
	Tests            []TestCaseInput `json:"tests"`
}

// BuildOptions are optional per-request overrides for the build phase.
type BuildOptions struct {
	Flags  []string     `json:"flags,omitempty"`
	Limits *LimitsInput `json:"limits,omitempty"`
}

// RunOptions are optional per-request overrides for the run phase.
type RunOptions struct {
	Limits *LimitsInput `json:"limits,omitempty"`
}

// LimitsInput holds per-request resource-limit overrides.
type LimitsInput struct {
	WallTimeS    int `json:"wall_time_s,omitempty"`
	MemoryKB     int `json:"memory_kb,omitempty"`
	MaxProcesses int `json:"max_processes,omitempty"`
}

// TestCaseInput is one test case in the request.
type TestCaseInput struct {
	Stdin          string `json:"stdin"`
	ExpectedStdout string `json:"expected_stdout"`
}

// RunResponse is the POST /run response body (always HTTP 200).
type RunResponse struct {
	Status string           `json:"status"`
	Build  *BuildResult     `json:"build,omitempty"`
	Tests  []TestCaseResult `json:"tests"`
}

// BuildResult holds the outcome of the compilation step.
type BuildResult struct {
	Status     string `json:"status"`
	Stdout     string `json:"stdout"`
	Stderr     string `json:"stderr"`
	WallTimeMs int64  `json:"wall_time_ms"`
}

// TestCaseResult holds the outcome of running one test case.
type TestCaseResult struct {
	Status     string `json:"status"`
	Stdout     string `json:"stdout"`
	Stderr     string `json:"stderr"`
	WallTimeMs int64  `json:"wall_time_ms"`
}

// errorResponse is the JSON shape for 400 errors.
type errorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ---------------------------------------------------------------------------
// RunHandler
// ---------------------------------------------------------------------------

// RunHandler handles POST /run requests.
type RunHandler struct {
	cfg *config.Config
	st  *stats.Stats
	sem chan struct{} // concurrency semaphore
}

// NewRunHandler creates a RunHandler with a concurrency semaphore sized to cfg.MaxConcurrent.
func NewRunHandler(cfg *config.Config, st *stats.Stats) *RunHandler {
	sem := make(chan struct{}, cfg.MaxConcurrent)
	return &RunHandler{cfg: cfg, st: st, sem: sem}
}

// ---------------------------------------------------------------------------
// Helper: write a 400 JSON error
// ---------------------------------------------------------------------------

func writeError(w http.ResponseWriter, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	json.NewEncoder(w).Encode(errorResponse{Code: code, Message: message})
}

// ---------------------------------------------------------------------------
// Run — the main HTTP handler
// ---------------------------------------------------------------------------

// Run handles POST /run requests.
func (h *RunHandler) Run(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// ── 1. Decode request ───────────────────────────────────────────────────
	var req RunRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields() // strict; ignore unknown fields is also acceptable
	if err := dec.Decode(&req); err != nil {
		writeError(w, "invalid_json", fmt.Sprintf("failed to decode request: %v", err))
		return
	}

	// ── 2. Validate language ────────────────────────────────────────────────
	lang, ok := h.cfg.Languages[req.Language]
	if !ok {
		writeError(w, "unknown_language", fmt.Sprintf("language %q is not configured", req.Language))
		return
	}

	// ── 3. Validate source size ─────────────────────────────────────────────
	if len(req.Source) > h.cfg.MaxSourceBytes {
		writeError(w, "source_too_large",
			fmt.Sprintf("source exceeds limit of %d bytes (got %d)", h.cfg.MaxSourceBytes, len(req.Source)))
		return
	}

	// ── 4. Validate test count ──────────────────────────────────────────────
	if len(req.Tests) > h.cfg.MaxTests {
		writeError(w, "too_many_tests",
			fmt.Sprintf("too many test cases: limit is %d, got %d", h.cfg.MaxTests, len(req.Tests)))
		return
	}
	if len(req.Tests) == 0 {
		writeError(w, "no_tests", "at least one test case is required")
		return
	}

	// ── 5. Validate source_filename if provided ──────────────────────────────
	if req.SourceFilename != "" {
		if err := validate.ValidateFilename(req.SourceFilename); err != nil {
			writeError(w, "invalid_filename", err.Error())
			return
		}
	}

	// ── 6. Validate artifact_filename if provided ────────────────────────────
	if req.ArtifactFilename != "" {
		if err := validate.ValidateFilename(req.ArtifactFilename); err != nil {
			writeError(w, "invalid_filename", err.Error())
			return
		}
	}

	// ── 7. Validate build flags ──────────────────────────────────────────────
	if req.Build != nil && len(req.Build.Flags) > 0 {
		if lang.Build == nil {
			writeError(w, "disallowed_flag",
				fmt.Sprintf("language %q does not support build flags", req.Language))
			return
		}
		if err := validate.ValidateFlags(req.Build.Flags, lang.Build.FlagAllowlist); err != nil {
			// Always emit "disallowed_flag" code so tests can find it
			writeError(w, "disallowed_flag", err.Error())
			return
		}
	}

	// ── 8. Acquire concurrency semaphore (queues; never rejects) ────────────
	h.sem <- struct{}{}
	defer func() { <-h.sem }()

	// ── 9. Track stats ───────────────────────────────────────────────────────
	atomic.AddInt64(&h.st.InFlightJobs, 1)
	defer atomic.AddInt64(&h.st.InFlightJobs, -1)
	atomic.AddInt64(&h.st.JobsTotal, 1)

	// ── 10. Create per-job temp directory ────────────────────────────────────
	jobID := runner.GenerateJobID()
	jobDir, err := os.MkdirTemp("", "goboxd-"+jobID+"-")
	if err != nil {
		atomic.AddInt64(&h.st.JobsFailedInternal, 1)
		h.st.SetLastError(time.Now())
		http.Error(w, `{"code":"internal_error","message":"failed to create job directory"}`,
			http.StatusInternalServerError)
		return
	}
	defer os.RemoveAll(jobDir)

	// ── 11. Determine source filename ─────────────────────────────────────────
	sourceFilename := lang.SourceFilename
	if req.SourceFilename != "" && lang.SourceFilenameStrategy == "from_request" {
		sourceFilename = req.SourceFilename
	}
	if sourceFilename == "" {
		sourceFilename = "solution"
	}

	// Write source to disk
	sourcePath := filepath.Join(jobDir, sourceFilename)
	if err := os.WriteFile(sourcePath, []byte(req.Source), 0644); err != nil {
		atomic.AddInt64(&h.st.JobsFailedInternal, 1)
		h.st.SetLastError(time.Now())
		http.Error(w, `{"code":"internal_error","message":"failed to write source file"}`,
			http.StatusInternalServerError)
		return
	}

	// ── 12. Build step (compiled languages only) ──────────────────────────────
	var buildResult *BuildResult

	if lang.Build != nil {
		// Determine artifact filename
		artifactFilename := lang.ArtifactFilename
		if artifactFilename == "" {
			artifactFilename = "solution"
		}
		artifactPath := filepath.Join(jobDir, artifactFilename)

		// Collect user flags
		userFlags := []string{}
		if req.Build != nil {
			userFlags = req.Build.Flags
		}

		// Build the compiler argument list, expanding template placeholders
		buildArgs := expandArgs(lang.Build.Args, sourcePath, artifactPath, userFlags)

		// Determine wall-time limit
		wallTimeS := lang.Build.Limits.WallTimeS
		if wallTimeS <= 0 {
			wallTimeS = 30 // generous default for compilation
		}
		if req.Build != nil && req.Build.Limits != nil && req.Build.Limits.WallTimeS > 0 {
			wallTimeS = req.Build.Limits.WallTimeS
		}

		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(wallTimeS)*time.Second)
		defer cancel()

		start := time.Now()
		cmd := exec.CommandContext(ctx, lang.Build.Cmd, buildArgs...)
		cmd.Dir = jobDir
		var buildStdout, buildStderr bytes.Buffer
		cmd.Stdout = &buildStdout
		cmd.Stderr = &buildStderr
		buildErr := cmd.Run()
		elapsed := time.Since(start).Milliseconds()

		buildResult = &BuildResult{
			Stdout:     buildStdout.String(),
			Stderr:     buildStderr.String(),
			WallTimeMs: elapsed,
		}

		if buildErr != nil {
			buildResult.Status = "failed"
			// All tests are not_executed when build fails
			notExec := make([]TestCaseResult, len(req.Tests))
			for i := range notExec {
				notExec[i] = TestCaseResult{Status: status.NotExecuted}
			}
			writeJSON(w, http.StatusOK, RunResponse{
				Status: status.BuildFailed,
				Build:  buildResult,
				Tests:  notExec,
			})
			return
		}
		buildResult.Status = "ok"
	}

	// ── 13. Run each test case ────────────────────────────────────────────────
	testResults := make([]TestCaseResult, 0, len(req.Tests))
	overallStatus := status.Accepted

	for _, tc := range req.Tests {
		result := h.runTestCase(lang, jobDir, sourceFilename, sourcePath, req, tc)
		testResults = append(testResults, result)
		overallStatus = worstStatus(overallStatus, result.Status)
	}

	writeJSON(w, http.StatusOK, RunResponse{
		Status: overallStatus,
		Build:  buildResult,
		Tests:  testResults,
	})
}

// ---------------------------------------------------------------------------
// runTestCase executes a single test case and returns its result.
// ---------------------------------------------------------------------------

func (h *RunHandler) runTestCase(
	lang config.Language,
	jobDir string,
	sourceFilename string,
	sourcePath string,
	req RunRequest,
	tc TestCaseInput,
) TestCaseResult {

	// Determine artifact path for compiled languages
	artifactFilename := lang.ArtifactFilename
	if artifactFilename == "" {
		artifactFilename = "solution"
	}
	artifactPath := filepath.Join(jobDir, artifactFilename)

	// Expand run args
	runArgs := expandArgs(lang.Run.Args, sourcePath, artifactPath, nil)

	// Determine run command — expand {{artifact}} using the bare filename
	// (relative to jobDir), not the absolute path, so "./{{artifact}}" → "./solution".
	runCmd := lang.Run.Cmd
	if strings.Contains(runCmd, config.TemplateArtifact) {
		runCmd = strings.ReplaceAll(runCmd, config.TemplateArtifact, artifactFilename)
	}
	if strings.Contains(runCmd, config.TemplateSource) {
		runCmd = strings.ReplaceAll(runCmd, config.TemplateSource, sourceFilename)
	}

	// Determine wall-time limit
	wallTimeS := lang.Run.Limits.WallTimeS
	if wallTimeS <= 0 {
		wallTimeS = 10
	}
	if req.Run != nil && req.Run.Limits != nil && req.Run.Limits.WallTimeS > 0 {
		wallTimeS = req.Run.Limits.WallTimeS
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(wallTimeS)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, runCmd, runArgs...)
	cmd.Dir = jobDir
	cmd.Stdin = strings.NewReader(tc.Stdin)

	// Limit stdout capture to 1 MiB to avoid OOM on huge output
	var stdoutBuf limitedBuffer
	stdoutBuf.limit = 1 << 20 // 1 MiB
	var stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	start := time.Now()
	runErr := cmd.Run()
	elapsed := time.Since(start).Milliseconds()

	stdout := stdoutBuf.String()
	stderr := stderrBuf.String()

	var testStatus string
	switch {
	case ctx.Err() == context.DeadlineExceeded:
		testStatus = status.TimeExceeded

	case runErr != nil:
		// Non-zero exit code → runtime error
		testStatus = status.RuntimeError

	case stdout == tc.ExpectedStdout:
		testStatus = status.Accepted

	case strings.TrimSpace(stdout) == strings.TrimSpace(tc.ExpectedStdout):
		testStatus = status.OutputWhitespaceMismatch

	default:
		testStatus = status.WrongOutput
	}

	return TestCaseResult{
		Status:     testStatus,
		Stdout:     stdout,
		Stderr:     stderr,
		WallTimeMs: elapsed,
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// expandArgs expands template placeholders in a command argument list.
//
// Templates:
//   - {{source}}   → absolute path to the source file
//   - {{artifact}} → absolute path to the compiled artifact
//   - {{flags}}    → expanded into individual flag strings (one per element)
func expandArgs(args []string, sourcePath, artifactPath string, flags []string) []string {
	out := make([]string, 0, len(args)+len(flags))
	for _, arg := range args {
		switch arg {
		case config.TemplateFlags:
			out = append(out, flags...)
		case config.TemplateSource:
			out = append(out, sourcePath)
		case config.TemplateArtifact:
			out = append(out, artifactPath)
		default:
			// Inline substitution for partial matches
			arg = strings.ReplaceAll(arg, config.TemplateSource, sourcePath)
			arg = strings.ReplaceAll(arg, config.TemplateArtifact, artifactPath)
			out = append(out, arg)
		}
	}
	return out
}

// statusPrecedence maps each status string to a priority (higher = worse).
var statusPrecedence = map[string]int{
	status.Accepted:               0,
	status.OutputWhitespaceMismatch: 1,
	status.WrongOutput:            2,
	status.TimeExceeded:           3,
	status.MemoryExceeded:         4,
	status.RuntimeError:           5,
	status.BuildFailed:            6,
	status.InternalError:          7,
}

// worstStatus returns whichever status has the higher precedence.
func worstStatus(a, b string) string {
	if statusPrecedence[b] > statusPrecedence[a] {
		return b
	}
	return a
}

// ---------------------------------------------------------------------------
// limitedBuffer — a bytes.Buffer that stops writing after `limit` bytes.
// ---------------------------------------------------------------------------

type limitedBuffer struct {
	bytes.Buffer
	limit int
}

func (lb *limitedBuffer) Write(p []byte) (int, error) {
	remaining := lb.limit - lb.Len()
	if remaining <= 0 {
		return len(p), nil // silently discard
	}
	if len(p) > remaining {
		p = p[:remaining]
	}
	n, err := lb.Buffer.Write(p)
	return n + (len(p) - n), err // report full len so callers don't error
}

// readAll reads the entire body of the request but limits to maxBytes.
func readAll(r io.Reader, maxBytes int) ([]byte, error) {
	return io.ReadAll(io.LimitReader(r, int64(maxBytes)+1))
}
