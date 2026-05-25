package handler

import (
	"encoding/json"
	"net/http"

	"github.com/thesouldev/goboxd/internal/config"
	"github.com/thesouldev/goboxd/internal/runner"
)

// HealthHandler manages the health and readiness checks.
type HealthHandler struct {
	nsjailProbe runner.ProbeResult
	langProbes  map[string]runner.ProbeResult
	cfg         *config.Config
}

// NewHealthHandler creates a new HealthHandler with the given probe results.
func NewHealthHandler(nsjail runner.ProbeResult, langs map[string]runner.ProbeResult, cfg *config.Config) *HealthHandler {
	return &HealthHandler{
		nsjailProbe: nsjail,
		langProbes:  langs,
		cfg:         cfg,
	}
}

// Readyz handles GET /readyz requests.
// Returns 200 only if nsjail OK + all language probes OK.
// Returns 503 with breakdown on any failure.
// JSON shape exactly as spec §03.
func (h *HealthHandler) Readyz(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	allOK := h.nsjailProbe.OK
	langResp := make(map[string]map[string]interface{})

	for langID, probe := range h.langProbes {
		if !probe.OK {
			allOK = false
		}

		m := map[string]interface{}{
			"ok": probe.OK,
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

	w.Header().Set("Content-Type", "application/json")
	if !allOK {
		w.WriteHeader(http.StatusServiceUnavailable)
	} else {
		w.WriteHeader(http.StatusOK)
	}

	json.NewEncoder(w).Encode(resp)
}
