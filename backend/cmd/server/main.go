package main

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/capacitarr/capacitarr/backend/internal/api"
	"github.com/capacitarr/capacitarr/backend/internal/config"
	"github.com/capacitarr/capacitarr/backend/internal/db"
	"github.com/capacitarr/capacitarr/backend/internal/logger"
)

func main() {
	cfg := config.Load()
	logger.Init(cfg.Debug)

	slog.Info("Starting Capacitarr backend", "port", cfg.Port, "base_url", cfg.BaseURL)

	if err := db.Init(cfg); err != nil {
		slog.Error("Failed to initialize database", "error", err)
		os.Exit(1)
	}

	mux := api.SetupRouter(cfg)

	server := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: mux,
	}

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("Server failed", "error", err)
		os.Exit(1)
	}
}
