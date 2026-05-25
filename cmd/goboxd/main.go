package main

import (
	"flag"
	"log/slog"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/thesouldev/goboxd/internal/handler"
)

var (
	version = "dev"
	commit  = "none"
)

func main() {
	port := flag.String("port", "8080", "HTTP listen port")
	flag.Parse()

	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(log)

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/healthz", handler.Health)
	r.Get("/readyz", handler.Ready)
	r.Get("/info", handler.Info)
	r.Post("/run", handler.Run)

	slog.Info("starting goboxd", "port", *port, "version", version, "commit", commit)
	if err := http.ListenAndServe(":"+*port, r); err != nil {
		slog.Error("server failed", "err", err)
		os.Exit(1)
	}
}
