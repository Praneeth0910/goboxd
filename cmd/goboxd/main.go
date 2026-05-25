package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/thesouldev/goboxd/internal/config"
	"github.com/thesouldev/goboxd/internal/handler"
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

	// Run startup probes
	slog.Info("running startup probes")
	nsjailProbe := runner.ProbeNsjail()
	langProbes := make(map[string]runner.ProbeResult)
	for langID, langCfg := range cfg.Languages {
		langProbes[langID] = runner.ProbeLanguage(langCfg)
	}
	healthHandler := handler.NewHealthHandler(version, commit, nsjailProbe, langProbes, cfg, st)

	// Setup router
	r := chi.NewRouter()
	r.Use(middleware.Logger)
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

	// Start server
	addr := fmt.Sprintf(":%d", *port)
	slog.Info("starting server", "addr", addr)
	if err := http.ListenAndServe(addr, r); err != nil {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
}
