package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/thesouldev/goboxd/internal/config"
	"github.com/thesouldev/goboxd/internal/handler"
	mw "github.com/thesouldev/goboxd/internal/middleware"
	"github.com/thesouldev/goboxd/internal/runner"
	"github.com/thesouldev/goboxd/internal/stats"
)

var (
	version = "dev"
	commit  = "none"
)

func main() {
	port := flag.Int("port", 8080, "port to listen on")
	configPath := flag.String("config", "languages.yaml", "path to configuration file")
	flag.Parse()

	// Setup structured JSON logging
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	slog.Info("starting goboxd", "version", version, "commit", commit)

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("failed to load configuration", "error", err, "path", *configPath)
		os.Exit(1)
	}

	// Setup Stats
	st := &stats.Stats{}

	// Clean up stale jail directories from the temp directory (using 10 minutes as maxAge)
	slog.Info("sweeping orphaned sandbox directories")
	runner.SweepOrphanedDirectories(os.TempDir(), 10*time.Minute)

	// Read GOBOXD_MAX_CONCURRENT from env, fall back to runtime.NumCPU()
	maxConcurrent := runtime.NumCPU()
	if v := os.Getenv("GOBOXD_MAX_CONCURRENT"); v != "" {
		if val, err := strconv.Atoi(v); err == nil && val > 0 {
			maxConcurrent = val
		}
	}
	cfg.MaxConcurrent = maxConcurrent

	// Concurrency Limiting Tradeoffs:
	// 1. sync.Mutex:
	//    - Only allows a concurrency limit of 1 (strictly serial execution).
	//    - Does not support arbitrary bounded concurrency (N > 1) natively.
	// 2. Worker Pool:
	//    - Requires managing a pool of worker goroutines and a job queue channel.
	//    - Introduces complexity in shutdown coordination, task routing, and handling timeouts.
	//    - Redundant here because Go's standard HTTP server (`net/http`) already spawns
	//      and manages a goroutine per incoming request.
	// 3. Channel Semaphore (Chosen):
	//    - Simple, elegant, and leverages Go's native channel blocking semantics.
	//    - By using a buffered channel `sem := make(chan struct{}, maxConcurrent)`, each HTTP
	//      request goroutine can block on `sem <- struct{}{}`.
	//    - This naturally queues requests when the limit is reached without requiring extra
	//      goroutines, workers, or complex job dispatching.
	//    - It prevents goroutine leaks as long as we properly release resources using `defer`.
	sem := make(chan struct{}, maxConcurrent)

	// Run startup probes
	slog.Info("running startup probes")
	nsjailProbe := runner.ProbeNsjail()
	langProbes := make(map[string]runner.ProbeResult)
	for langID, langCfg := range cfg.Languages {
		langProbes[langID] = runner.ProbeLanguage(langCfg)
	}
	healthHandler := handler.NewHealthHandler(version, commit, nsjailProbe, langProbes, cfg, st)
	runHandler := handler.NewRunHandler(cfg, st, sem)

	// Setup router with structured logging and recovery
	r := chi.NewRouter()
	r.Use(mw.Logger) // Custom structured JSON logger (replaces chi's middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/info", http.StatusFound)
	})
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	r.Get("/readyz", healthHandler.Readyz)
	r.Get("/info", healthHandler.Info)
	r.Post("/run", runHandler.Run)

	// Start server
	addr := fmt.Sprintf(":%d", *port)
	slog.Info("starting server", "addr", addr)
	if err := http.ListenAndServe(addr, r); err != nil {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
}
