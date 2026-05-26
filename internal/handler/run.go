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

// NewRunHandler creates a RunHandler with the provided concurrency semaphore.
func NewRunHandler(cfg *config.Config, st *stats.Stats, sem chan struct{}) *RunHandler {
	return &RunHandler{cfg: cfg, st: st, sem: sem}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func writeError(w http.ResponseWriter, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	json.NewEncoder(w).Encode(map[string]any{
		"error": errorResponse{Code: code, Message: message},
	})
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

	// 7. Validate test count
	if len(req.Tests) < 1 || len(req.Tests) > h.cfg.MaxTests {
		writeError(w, "invalid_test_count", fmt.Sprintf("test count must be between 1 and %d", h.cfg.MaxTests))
		return
	}

	// 8. Acquire concurrency slot (blocking queue behavior)
	// Why this is correct and safe:
	// - Channel Blocking (Queueing): Writing to a buffered channel `sem <- struct{}{}`
	//   blocks the sending goroutine if the channel is full. Since Go's net/http server
	//   executes every incoming request in its own goroutine, blocking here puts the request
	//   in a natural waiting queue, matching the requirement that requests queue and do not fail.
	// - Immediate Defer: The 'defer' statement must be registered *immediately* after
	//   successful acquisition. This guarantees that no matter how the rest of the
	//   function returns (e.g., normal response, runner failure, internal server errors, or panics),
	//   the slot is released by reading from the channel `<-h.sem`.
	// - No Goroutine Leaks: Because the release is guaranteed via the deferred read on any
	//   exit path, request goroutines are guaranteed to eventually exit and free their slots.
	//   They are not leaked or left hanging indefinitely.
	h.sem <- struct{}{}
	defer func() { <-h.sem }()

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
		json.NewEncoder(w).Encode(map[string]any{
			"error": errorResponse{Code: "internal_error", Message: err.Error()},
		})
		return
	}

	// 10. Respond 200 with result (never 5xx for user-code failure)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(toHTTPResponse(result))
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

	if req.Run != nil && req.Run.Limits != nil {
		rr.Run = runner.PhaseConfig{
			Limits: config.ResourceLimits{
				WallTimeS:    req.Run.Limits.WallTimeS,
				MemoryKB:     req.Run.Limits.MemoryKB,
				MaxProcesses: req.Run.Limits.MaxProcesses,
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
	if r.Status == status.StatusBuildFailed {
		resp.Status = status.StatusBuildFailed
	}

	return resp
}
