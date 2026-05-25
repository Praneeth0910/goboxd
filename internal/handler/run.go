package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/thesouldev/goboxd/internal/config"
	"github.com/thesouldev/goboxd/internal/runner"
	"github.com/thesouldev/goboxd/internal/stats"
	"github.com/thesouldev/goboxd/internal/status"
	"github.com/thesouldev/goboxd/internal/validate"
)

// ---------------------------------------------------------------------------
// HTTP Request / Response types
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
// Helpers
// ---------------------------------------------------------------------------

func writeError(w http.ResponseWriter, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	json.NewEncoder(w).Encode(errorResponse{Code: code, Message: message})
}

func writeJSON(w http.ResponseWriter, httpStatus int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpStatus)
	json.NewEncoder(w).Encode(v)
}

// ---------------------------------------------------------------------------
// Run — HTTP handler (validation + stats only; execution delegated to runner)
// ---------------------------------------------------------------------------

// Run handles POST /run requests.
func (h *RunHandler) Run(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// ── 1. Decode request ────────────────────────────────────────────────────
	var req RunRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeError(w, "invalid_json", fmt.Sprintf("failed to decode request: %v", err))
		return
	}

	// ── 2. Validate language ─────────────────────────────────────────────────
	lang, ok := h.cfg.Languages[req.Language]
	if !ok {
		writeError(w, "unknown_language", fmt.Sprintf("language %q is not configured", req.Language))
		return
	}

	// ── 3. Validate source size ──────────────────────────────────────────────
	if len(req.Source) > h.cfg.MaxSourceBytes {
		writeError(w, "source_too_large",
			fmt.Sprintf("source exceeds limit of %d bytes (got %d)", h.cfg.MaxSourceBytes, len(req.Source)))
		return
	}

	// ── 4. Validate test count ───────────────────────────────────────────────
	if len(req.Tests) > h.cfg.MaxTests {
		writeError(w, "too_many_tests",
			fmt.Sprintf("too many test cases: limit is %d, got %d", h.cfg.MaxTests, len(req.Tests)))
		return
	}
	if len(req.Tests) == 0 {
		writeError(w, "no_tests", "at least one test case is required")
		return
	}

	// ── 5. Validate source_filename ──────────────────────────────────────────
	if req.SourceFilename != "" {
		if err := validate.ValidateFilename(req.SourceFilename); err != nil {
			writeError(w, "invalid_filename", err.Error())
			return
		}
	}

	// ── 6. Validate artifact_filename ────────────────────────────────────────
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

	// ── 10. Build runner request and delegate execution to runner.RunSandbox ─
	runnerReq := toRunnerRequest(req)
	result, err := runner.RunSandbox(lang, runnerReq)
	if err != nil {
		atomic.AddInt64(&h.st.JobsFailedInternal, 1)
		h.st.SetLastError(time.Now())
		writeJSON(w, http.StatusInternalServerError,
			errorResponse{Code: "internal_error", Message: err.Error()})
		return
	}

	// ── 11. Map runner result → HTTP response ────────────────────────────────
	writeJSON(w, http.StatusOK, toHTTPResponse(result))
}

// ---------------------------------------------------------------------------
// Mapping helpers: HTTP types ↔ runner types
// ---------------------------------------------------------------------------

// toRunnerRequest converts the HTTP-layer RunRequest into runner.RunRequest.
func toRunnerRequest(req RunRequest) runner.RunRequest {
	tests := make([]runner.TestCase, len(req.Tests))
	for i, tc := range req.Tests {
		tests[i] = runner.TestCase{
			Stdin:          tc.Stdin,
			ExpectedStdout: tc.ExpectedStdout,
		}
	}

	rr := runner.RunRequest{
		Language:         req.Language,
		Source:           req.Source,
		SourceFilename:   req.SourceFilename,
		ArtifactFilename: req.ArtifactFilename,
		Tests:            tests,
	}

	if req.Build != nil {
		pc := &runner.PhaseConfig{Flags: req.Build.Flags}
		if req.Build.Limits != nil {
			pc.Limits = config.ResourceLimits{
				WallTimeS: req.Build.Limits.WallTimeS,
				MemoryKB:  req.Build.Limits.MemoryKB,
			}
		}
		rr.Build = pc
	}

	if req.Run != nil && req.Run.Limits != nil {
		rr.Run = runner.PhaseConfig{
			Limits: config.ResourceLimits{
				WallTimeS: req.Run.Limits.WallTimeS,
				MemoryKB:  req.Run.Limits.MemoryKB,
			},
		}
	}

	return rr
}

// toHTTPResponse converts runner.RunResult into the HTTP RunResponse.
func toHTTPResponse(r runner.RunResult) RunResponse {
	resp := RunResponse{
		Status: r.Status,
		Tests:  make([]TestCaseResult, len(r.Tests)),
	}

	// Include build result only if a build phase actually ran
	if r.Build.Status != "" {
		resp.Build = &BuildResult{
			Status:     r.Build.Status,
			Stdout:     r.Build.Stdout,
			Stderr:     r.Build.Stderr,
			WallTimeMs: r.Build.DurationMS,
		}
	}

	for i, tr := range r.Tests {
		resp.Tests[i] = TestCaseResult{
			Status:     tr.Status,
			Stdout:     tr.Stdout,
			Stderr:     tr.Stderr,
			WallTimeMs: tr.DurationMS,
		}
	}

	// Treat overall "not_executed" (all tests skipped due to build failure)
	if r.Status == status.BuildFailed {
		resp.Status = status.BuildFailed
	}

	return resp
}
