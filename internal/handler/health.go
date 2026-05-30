package handler

import (
	"encoding/json"
	"net/http"
	"os"
	"runtime"
	"sort"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/thesouldev/goboxd/internal/config"
	"github.com/thesouldev/goboxd/internal/runner"
	"github.com/thesouldev/goboxd/internal/stats"
)

// writeJSON writes an indented JSON response (human-readable in browser).
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		http.Error(w, `{"error":"internal json error"}`, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(status)
	if _, err := w.Write(b); err != nil {
		http.Error(w, "write error", http.StatusInternalServerError)
		return
	}
	if _, err := w.Write([]byte("\n")); err != nil {
		http.Error(w, "write error", http.StatusInternalServerError)
		return
	}
}

// HealthHandler manages the health, readiness, and info checks.
type HealthHandler struct {
	buildVersion string
	buildCommit  string
	nsjailProbe  runner.ProbeResult

	mu         sync.RWMutex
	langProbes map[string]runner.ProbeResult

	cfg *config.Config
	st  *stats.Stats
}

// NewHealthHandler creates a new HealthHandler with the given dependencies.
func NewHealthHandler(deps Deps) *HealthHandler {
	h := &HealthHandler{
		buildVersion: deps.Version,
		buildCommit:  deps.Commit,
		nsjailProbe:  deps.NsjailProbe,
		langProbes:   deps.LangProbes,
		cfg:          deps.Config,
		st:           deps.Stats,
	}

	go h.startBackgroundProbes()

	return h
}

func (h *HealthHandler) startBackgroundProbes() {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		newProbes := make(map[string]runner.ProbeResult)
		for langID := range h.cfg.Languages {
			newProbes[langID] = runner.ProbeLanguage(h.cfg.Languages[langID])
		}

		h.mu.Lock()
		h.nsjailProbe = runner.ProbeNsjail()
		h.langProbes = newProbes
		h.mu.Unlock()
	}
}

// Readyz handles GET /readyz requests.
func (h *HealthHandler) Readyz(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	allOK := h.nsjailProbe.OK
	langResp := make(map[string]map[string]interface{})

	h.mu.RLock()
	defer h.mu.RUnlock()

	for langID := range h.cfg.Languages {
		probe := h.langProbes[langID]
		ok := probe.OK

		if !ok {
			allOK = false
		}

		m := map[string]interface{}{
			"ok": ok,
		}
		if probe.Version != "" {
			m["version"] = probe.Version
		}
		if probe.Error != "" {
			m["error"] = probe.Error
		}
		langResp[langID] = m
	}

	nsjailResp := map[string]interface{}{
		"ok":      h.nsjailProbe.OK,
		"version": h.nsjailProbe.Version, // Always present, even if empty string
	}
	if h.nsjailProbe.Error != "" {
		nsjailResp["error"] = h.nsjailProbe.Error
	}

	status := "ok"
	if !allOK {
		status = "degraded"
	}

	resp := map[string]interface{}{
		"status":    status,
		"nsjail":    nsjailResp,
		"languages": langResp,
	}

	httpStatus := http.StatusOK
	if !allOK {
		httpStatus = http.StatusServiceUnavailable
	}

	writeJSON(w, httpStatus, resp)
}

// Info handlers types
type BuildInfo struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	GoVersion string `json:"go_version"`
}

type NsjailInfo struct {
	Path    string `json:"path"`
	Version string `json:"version"`
}

type DefaultRunLimits struct {
	WallTimeS    int `json:"wall_time_s"`
	MemoryKB     int `json:"memory_kb"`
	MaxProcesses int `json:"max_processes"`
}

type LanguageInfo struct {
	ID               string           `json:"id"`
	Name             string           `json:"name"`
	Version          string           `json:"version"`
	DefaultRunLimits DefaultRunLimits `json:"default_run_limits"`
}

type LimitsInfo struct {
	MaxSourceBytes    int `json:"max_source_bytes"`
	MaxTests          int `json:"max_tests"`
	MaxConcurrentJobs int `json:"max_concurrent_jobs"`
}

type StatsInfo struct {
	InFlightJobs       int64      `json:"in_flight_jobs"`
	JobsTotal          int64      `json:"jobs_total"`
	JobsFailedInternal int64      `json:"jobs_failed_internal"`
	LastInternalError  *time.Time `json:"last_internal_error_at,omitempty"`
	DiskFreeBytes      uint64     `json:"disk_free_bytes_jail_dir"`
}

type InfoResponse struct {
	BuildInfo BuildInfo      `json:"build_info"`
	Nsjail    NsjailInfo     `json:"nsjail"`
	Languages []LanguageInfo `json:"languages"`
	Limits    LimitsInfo     `json:"limits"`
	Stats     StatsInfo      `json:"stats"`
}

// Info handles GET /info requests.
// Always 200.
// Returns build_info, nsjail, languages[], limits, stats
func (h *HealthHandler) Info(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	nsjailPath := os.Getenv("NSJAIL_PATH")
	if nsjailPath == "" {
		nsjailPath = "/usr/sbin/nsjail"
	}

	// Prepare Languages
	langs := make([]LanguageInfo, 0, len(h.cfg.Languages))
	func() {
		h.mu.RLock()
		defer h.mu.RUnlock()
		for id := range h.cfg.Languages {
			probe := h.langProbes[id]

			limits := DefaultRunLimits{
				WallTimeS:    h.cfg.Languages[id].Run.Limits.WallTimeS,
				MemoryKB:     h.cfg.Languages[id].Run.Limits.MemoryKB,
				MaxProcesses: h.cfg.Languages[id].Run.Limits.MaxProcesses,
			}

			langs = append(langs, LanguageInfo{
				ID:               id,
				Name:             h.cfg.Languages[id].Name,
				Version:          probe.Version,
				DefaultRunLimits: limits,
			})
		}
	}()

	// Sort languages by ID to make JSON response deterministic
	sort.Slice(langs, func(i, j int) bool {
		return langs[i].ID < langs[j].ID
	})

	// Prepare Disk free bytes
	var stat syscall.Statfs_t
	var diskFree uint64
	if err := syscall.Statfs(os.TempDir(), &stat); err == nil {
		diskFree = stat.Bavail * uint64(stat.Bsize)
	}

	// Prepare Stats
	lastErrTime := h.st.GetLastError()
	var lastErrPtr *time.Time
	if !lastErrTime.IsZero() {
		lastErrPtr = &lastErrTime
	}

	statsInfo := StatsInfo{
		InFlightJobs:       atomic.LoadInt64(&h.st.InFlightJobs),
		JobsTotal:          atomic.LoadInt64(&h.st.JobsTotal),
		JobsFailedInternal: atomic.LoadInt64(&h.st.JobsFailedInternal),
		LastInternalError:  lastErrPtr,
		DiskFreeBytes:      diskFree,
	}

	resp := InfoResponse{
		BuildInfo: BuildInfo{
			Version:   h.buildVersion,
			Commit:    h.buildCommit,
			GoVersion: runtime.Version(),
		},
		Nsjail: NsjailInfo{
			Path:    nsjailPath,
			Version: h.nsjailProbe.Version,
		},
		Languages: langs,
		Limits: LimitsInfo{
			MaxSourceBytes:    h.cfg.MaxSourceBytes,
			MaxTests:          h.cfg.MaxTests,
			MaxConcurrentJobs: h.cfg.MaxConcurrent,
		},
		Stats: statsInfo,
	}

	writeJSON(w, http.StatusOK, resp)
}
