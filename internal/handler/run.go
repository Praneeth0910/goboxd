package handler

import (
	"encoding/json"
	"fmt"
	"log/slog"
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
	Flags  []string     `json:"flags,omitempty"`
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
	DurationMs int64  `json:"duration_ms"`
}

// TestCaseResult holds the outcome of running one test case.
type TestCaseResult struct {
	Status       string `json:"status"`
	Stdout       string `json:"stdout"`
	Stderr       string `json:"stderr"`
	DurationMs   int64  `json:"duration_ms"`
	MemoryPeakKB int64  `json:"memory_peak_kb"`
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

// NewRunHandler creates a RunHandler from the injected dependencies.
func NewRunHandler(deps Deps) *RunHandler {
	return &RunHandler{cfg: deps.Config, st: deps.Stats, sem: deps.Semaphore}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func writeError(w http.ResponseWriter, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	if err := json.NewEncoder(w).Encode(map[string]any{
		"error": errorResponse{Code: code, Message: message},
	}); err != nil {
		slog.Error("failed to encode error response", "error", err)
	}
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

	// 1. Wrap r.Body with http.MaxBytesReader BEFORE decoding
	maxSourceBytes := int64(h.cfg.MaxSourceBytes)
	r.Body = http.MaxBytesReader(w, r.Body, maxSourceBytes)

	// 2. Decode JSON
	var req RunRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeError(w, "invalid_json", fmt.Sprintf("failed to decode request: %v", err))
		return
	}

	// 3. Validate language exists
	lang, ok := h.cfg.Languages[req.Language]
	if !ok {
		writeError(w, "unknown_language", fmt.Sprintf("language %q is not configured", req.Language))
		return
	}

	// 4. Validate source_filename
	if req.SourceFilename != "" {
		if err := validate.ValidateFilename(req.SourceFilename); err != nil {
			writeError(w, "invalid_filename", err.Error())
			return
		}
	}

	// 5. Validate artifact_filename
	if req.ArtifactFilename != "" {
		if err := validate.ValidateFilename(req.ArtifactFilename); err != nil {
			writeError(w, "invalid_filename", err.Error())
			return
		}
	}

	// 6. Validate build flags
	if req.Build != nil && len(req.Build.Flags) > 0 {
		if lang.Build == nil {
			writeError(w, "disallowed_flag", fmt.Sprintf("language %q does not support build flags", req.Language))
			return
		}
		if err := validate.ValidateFlags(req.Build.Flags, lang.Build.FlagAllowlist); err != nil {
			writeError(w, "disallowed_flag", err.Error())
			return
		}
	}

	if req.Run != nil && len(req.Run.Flags) > 0 {
		if err := validate.ValidateFlags(req.Run.Flags, lang.Run.FlagAllowlist); err != nil {
			writeError(w, "disallowed_flag", err.Error())
			return
		}
	}

	// 6.5 Validate limits
	if req.Build != nil && req.Build.Limits != nil {
		if lang.Build == nil {
			writeError(w, "invalid_limits", fmt.Sprintf("language %q does not support build limits", req.Language))
			return
		}
		buildLimits := lang.Build.Limits
		if buildLimits.WallTimeS <= 0 {
			buildLimits.WallTimeS = 30
		}
		if buildLimits.MemoryKB <= 0 {
			buildLimits.MemoryKB = 1024 * 1024
		}
		if buildLimits.MaxProcesses <= 0 {
			buildLimits.MaxProcesses = 100
		}
		override := config.ResourceLimits{
			WallTimeS:    req.Build.Limits.WallTimeS,
			MemoryKB:     req.Build.Limits.MemoryKB,
			MaxProcesses: req.Build.Limits.MaxProcesses,
		}
		if _, err := buildLimits.MergeWithCap(override); err != nil {
			writeError(w, "invalid_limits", fmt.Sprintf("build limits: %v", err))
			return
		}
	}

	if req.Run != nil && req.Run.Limits != nil {
		runLimits := lang.Run.Limits
		if runLimits.WallTimeS <= 0 {
			runLimits.WallTimeS = 10
		}
		if runLimits.MemoryKB <= 0 {
			runLimits.MemoryKB = 256 * 1024
		}
		if runLimits.MaxProcesses <= 0 {
			runLimits.MaxProcesses = 64
		}
		override := config.ResourceLimits{
			WallTimeS:    req.Run.Limits.WallTimeS,
			MemoryKB:     req.Run.Limits.MemoryKB,
			MaxProcesses: req.Run.Limits.MaxProcesses,
		}
		if _, err := runLimits.MergeWithCap(override); err != nil {
			writeError(w, "invalid_limits", fmt.Sprintf("run limits: %v", err))
			return
		}
	}

	// 7. Validate test count
	if len(req.Tests) < 1 || len(req.Tests) > h.cfg.MaxTests {
		writeError(w, "invalid_test_count", fmt.Sprintf("test count must be between 1 and %d", h.cfg.MaxTests))
		return
	}

	// 8. Acquire concurrency slot (queue behavior with timeout)
	// We use a select with time.After to prevent infinite blocking.
	select {
	case h.sem <- struct{}{}:
		defer func() { <-h.sem }()
	case <-time.After(time.Duration(h.cfg.QueueTimeoutS) * time.Second):
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		if err := json.NewEncoder(w).Encode(map[string]any{
			"error": errorResponse{Code: "queue_full", Message: fmt.Sprintf("server is busy, retry after %d seconds", h.cfg.QueueTimeoutS)},
		}); err != nil {
			slog.Error("failed to encode queue_full response", "error", err)
		}
		return
	}

	// Track stats
	atomic.AddInt64(&h.st.InFlightJobs, 1)
	defer atomic.AddInt64(&h.st.InFlightJobs, -1)
	atomic.AddInt64(&h.st.JobsTotal, 1)

	// 9. Call runner.RunSandbox
	runnerReq := toRunnerRequest(req)
	result, err := runner.RunSandbox(lang, runnerReq)
	if err != nil {
		atomic.AddInt64(&h.st.JobsFailedInternal, 1)
		h.st.SetLastError(time.Now())
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		if err := json.NewEncoder(w).Encode(map[string]any{
			"error": errorResponse{Code: "internal_error", Message: err.Error()},
		}); err != nil {
			slog.Error("failed to encode internal_error response", "error", err)
		}
		return
	}

	// 10. Respond 200 with result (never 5xx for user-code failure)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(toHTTPResponse(result)); err != nil {
		slog.Error("failed to encode run response", "error", err)
	}
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
				WallTimeS:    req.Build.Limits.WallTimeS,
				MemoryKB:     req.Build.Limits.MemoryKB,
				MaxProcesses: req.Build.Limits.MaxProcesses,
			}
		}
		rr.Build = pc
	}

	if req.Run != nil {
		rr.Run = runner.PhaseConfig{Flags: req.Run.Flags}
		if req.Run.Limits != nil {
			rr.Run.Limits = config.ResourceLimits{
				WallTimeS:    req.Run.Limits.WallTimeS,
				MemoryKB:     req.Run.Limits.MemoryKB,
				MaxProcesses: req.Run.Limits.MaxProcesses,
			}
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
			DurationMs: r.Build.DurationMS,
		}
	}

	for i, tr := range r.Tests {
		resp.Tests[i] = TestCaseResult{
			Status:       tr.Status,
			Stdout:       tr.Stdout,
			Stderr:       tr.Stderr,
			DurationMs:   tr.DurationMS,
			MemoryPeakKB: tr.MemoryPeakKB,
		}
	}

	// Treat overall "not_executed" (all tests skipped due to build failure)
	if r.Status == status.StatusBuildFailed {
		resp.Status = status.StatusBuildFailed
	}

	return resp
}
