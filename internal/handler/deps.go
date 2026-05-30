package handler

import (
	"github.com/thesouldev/goboxd/internal/config"
	"github.com/thesouldev/goboxd/internal/runner"
	"github.com/thesouldev/goboxd/internal/stats"
)

// Deps holds the injected dependencies for all HTTP handlers to avoid long parameter lists.
type Deps struct {
	Config      *config.Config
	Stats       *stats.Stats
	Semaphore   chan struct{}
	Version     string
	Commit      string
	NsjailProbe runner.ProbeResult
	LangProbes  map[string]runner.ProbeResult
}
