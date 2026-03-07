// Package main is the entry point for the Capacitarr application server.
package main

import (
	"context"
	"crypto/rand"
	"embed"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"mime"
	"net/http"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"capacitarr/internal/config"
	"capacitarr/internal/db"
	"capacitarr/internal/events"
	"capacitarr/internal/jobs"
	"capacitarr/internal/logger"
	"capacitarr/internal/poller"
	"capacitarr/internal/services"
	"capacitarr/routes"
)

//go:embed all:frontend/dist
var embeddedFiles embed.FS

// getSubFS strips the "frontend/dist" prefix from the embedded filesystem
func getSubFS() fs.FS {
	fsys, err := fs.Sub(embeddedFiles, "frontend/dist")
	if err != nil {
		panic(fmt.Errorf("error stripping prefix from embedded fs: %v", err))
	}
	return fsys
}

// serveEmbeddedFile reads a file from the embedded FS and writes it to the response
// with the correct Content-Type based on the file extension.
func serveEmbeddedFile(c echo.Context, fsys fs.FS, filePath string) error {
	f, err := fsys.Open(filePath)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound)
	}
	defer func() { _ = f.Close() }()

	data, err := io.ReadAll(f)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError)
	}

	contentType := mime.TypeByExtension(filepath.Ext(filePath))
	if contentType == "" {
		contentType = http.DetectContentType(data)
	}

	// Set Cache-Control based on whether the file is a hashed asset
	if strings.HasPrefix(filePath, "_assets/") {
		// Hashed assets from Nuxt build — immutable, cache forever
		c.Response().Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else if filePath == "index.html" || filePath == "200.html" {
		// Entry points — always revalidate
		c.Response().Header().Set("Cache-Control", "no-cache")
	}

	return c.Blob(http.StatusOK, contentType, data)
}

// spaHandler serves static files from the embedded filesystem and falls back
// to 200.html (Nuxt's SPA catch-all) for any path that doesn't match a real
// file. This allows the client-side Vue Router to handle navigation.
func spaHandler(fsys fs.FS, stripPrefix string) echo.HandlerFunc {
	return func(c echo.Context) error {
		// Get the requested path and strip the prefix if configured
		reqPath := c.Request().URL.Path
		if stripPrefix != "" {
			reqPath = strings.TrimPrefix(reqPath, stripPrefix)
		}

		// Clean the path and remove leading slash for fs.Open
		reqPath = path.Clean("/" + reqPath)
		reqPath = strings.TrimPrefix(reqPath, "/")

		// If the path is empty, serve index.html
		if reqPath == "" || reqPath == "." {
			reqPath = "index.html"
		}

		// Try to open the requested file
		f, err := fsys.Open(reqPath)
		if err == nil {
			// Check if it's a directory — if so, look for index.html inside it
			stat, statErr := f.Stat()
			_ = f.Close()
			if statErr == nil && stat.IsDir() {
				indexPath := path.Join(reqPath, "index.html")
				if idxFile, idxErr := fsys.Open(indexPath); idxErr == nil {
					_ = idxFile.Close()
					return serveEmbeddedFile(c, fsys, indexPath)
				}
				// Directory exists but no index.html — fall through to SPA fallback
			} else if statErr == nil {
				// It's a real file, serve it from the embedded FS
				return serveEmbeddedFile(c, fsys, reqPath)
			}
		}

		// If the requested path looks like a static resource (has a file extension),
		// return 404 instead of the SPA fallback. This prevents requests for
		// non-existent .json, .js, .css files from receiving HTML responses
		// (which causes JSON parse errors on the client).
		if ext := filepath.Ext(reqPath); ext != "" {
			return echo.NewHTTPError(http.StatusNotFound)
		}

		// File not found and has no extension — serve the SPA fallback
		// (200.html or index.html). Nuxt generates 200.html specifically for
		// SPA catch-all hosting (client-side Vue Router handles the route).
		if fallback, fbErr := fsys.Open("200.html"); fbErr == nil {
			_ = fallback.Close()
			return serveEmbeddedFile(c, fsys, "200.html")
		}
		return serveEmbeddedFile(c, fsys, "index.html")
	}
}

// generateRequestID produces a short random hex ID for request tracing.
func generateRequestID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x", b)
}

// Build-time injected via -ldflags
var (
	version   = "dev"
	commit    = "unknown"
	buildDate = "unknown"
)

func main() {
	cfg := config.Load()
	logger.Init(cfg.Debug)

	// ─── Startup configuration logging ──────────────────────────────────────
	slog.Info("Starting Capacitarr backend",
		"component", "main",
		"version", version,
		"commit", commit,
		"buildDate", buildDate,
		"port", cfg.Port,
		"baseURL", cfg.BaseURL,
		"debug", cfg.Debug,
		"dbPath", cfg.Database,
	)
	if len(cfg.CORSOrigins) > 0 {
		slog.Info("CORS origins configured", "component", "main", "origins", cfg.CORSOrigins)
	}
	if cfg.AuthHeader != "" {
		slog.Info("Reverse proxy auth header enabled", "component", "main", "header", cfg.AuthHeader)
	}

	if err := db.Init(cfg); err != nil {
		slog.Error("Failed to initialize database", "component", "main", "operation", "init_database", "error", err)
		os.Exit(1)
	}

	// ─── Event Bus + Subscribers ───────────────────────────────────────────
	bus := events.NewEventBus()

	// SSE Broadcaster — fans out events to connected browser tabs
	sseBroadcaster := events.NewSSEBroadcaster(bus)
	sseBroadcaster.Start()

	slog.Info("Event bus started (partial)", "component", "main", "subscribers", "sse_broadcaster")

	// ─── Service Registry ──────────────────────────────────────────────────
	reg := services.NewRegistry(db.DB, bus, cfg)
	reg.InitVersion(version)

	// Activity Persister — writes all events to the activity_events table via SettingsService.
	// Must be created after the service registry since it depends on SettingsService.
	activityPersister := events.NewActivityPersister(reg.Settings, bus)
	activityPersister.Start()

	// Notification Dispatch Service — subscribes to events, dispatches digests + alerts
	reg.NotificationDispatch.Start()

	slog.Info("Event subscribers started", "component", "main", "subscribers", "activity_persister, notification_dispatch")

	// Start the background deletion worker (replaces old init() goroutine)
	reg.Deletion.Start()

	// Recover any approvals that were orphaned by a previous shutdown
	if count, err := reg.Approval.RecoverOrphans(); err != nil {
		slog.Error("Failed to recover orphaned approvals", "component", "main", "error", err)
	} else if count > 0 {
		slog.Info("Recovered orphaned approvals on startup", "component", "main", "count", count)
	}

	// Initialize background jobs
	pollerInstance := poller.New(reg)
	pollerInstance.Start()
	cronScheduler := jobs.Start(reg)

	// Non-blocking startup self-test — check connectivity to all enabled integrations
	go func() {
		configs, err := reg.Integration.ListEnabled()
		if err != nil {
			slog.Warn("Startup self-test: failed to list integrations", "component", "main", "error", err)
			return
		}
		if len(configs) == 0 {
			slog.Info("Startup self-test: no integrations configured", "component", "main")
			return
		}
		for _, cfg := range configs {
			result := reg.Integration.TestConnection(cfg.Type, cfg.URL, cfg.APIKey, nil)
			if result.Success {
				slog.Info("Startup self-test: connection OK",
					"component", "main",
					"integration", cfg.Name,
					"type", cfg.Type,
				)
			} else {
				slog.Warn("Startup self-test: connection failed",
					"component", "main",
					"integration", cfg.Name,
					"type", cfg.Type,
					"error", result.Error,
				)
			}
		}
		slog.Info("Startup self-test complete", "component", "main", "integrations", len(configs))
	}()

	// Initialize Echo instance
	e := echo.New()

	// Request ID middleware — generates a unique ID for each request
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			reqID := c.Request().Header.Get("X-Request-ID")
			if reqID == "" {
				reqID = generateRequestID()
			}
			c.Set("requestId", reqID)
			c.Response().Header().Set("X-Request-ID", reqID)
			return next(c)
		}
	})

	// Request logger middleware
	e.Use(middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		LogURI:    true,
		LogStatus: true,
		LogMethod: true,
		LogValuesFunc: func(c echo.Context, v middleware.RequestLoggerValues) error {
			reqID, _ := c.Get("requestId").(string)
			slog.Info("request",
				"component", "middleware",
				"method", v.Method,
				"uri", v.URI,
				"status", v.Status,
				"requestId", reqID,
			)
			return nil
		},
	}))
	e.Use(middleware.Recover())

	// Security headers middleware
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Response().Header().Set("X-Content-Type-Options", "nosniff")
			c.Response().Header().Set("X-Frame-Options", "DENY")
			c.Response().Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
			return next(c)
		}
	})

	// Add CORS middleware — only if origins are configured
	if len(cfg.CORSOrigins) > 0 {
		e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
			AllowOrigins: cfg.CORSOrigins,
			AllowMethods: []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete},
		}))
	}

	// API Routing group mapping to /api/v1
	// Respect configuration's BaseURL for any proxy magic
	prefix := cfg.BaseURL
	if prefix == "/" {
		prefix = "" // allow mapping directly to routes without double slashing
	}
	// Remove trailing slash from prefix for clean route joining
	prefix = strings.TrimRight(prefix, "/")
	apiGroup := e.Group(prefix + "/api/v1")
	routes.RegisterAPIRoutes(apiGroup, reg, version, commit, buildDate, sseBroadcaster)

	// Serve the embedded Nuxt static frontend with SPA fallback
	fsys := getSubFS()

	if cfg.BaseURL != "" && cfg.BaseURL != "/" {
		baseURL := strings.TrimRight(cfg.BaseURL, "/")
		uiGroup := e.Group(baseURL)
		uiGroup.GET("/*", spaHandler(fsys, baseURL))
	} else {
		e.GET("/*", spaHandler(fsys, ""))
	}

	// Publish server start event to the event bus
	bus.Publish(events.ServerStartedEvent{Version: version})

	slog.Info("Server initialized, starting listener", "component", "main", "port", cfg.Port)

	// Graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		sig := <-sigChan
		slog.Info("Received shutdown signal", "component", "main", "signal", sig)

		// Stop background jobs
		pollerInstance.Stop()
		cronScheduler.Stop()
		reg.Integration.CloseCache()

		// Stop services
		reg.Deletion.Stop()

		// Stop event bus infrastructure
		reg.NotificationDispatch.Stop()
		sseBroadcaster.Stop()
		activityPersister.Stop()
		bus.Close()

		// Shutdown HTTP server with 10s deadline
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := e.Shutdown(ctx); err != nil {
			slog.Error("Server shutdown error", "component", "main", "operation", "shutdown", "error", err)
		}
	}()

	// Start Server
	if err := e.Start(":" + cfg.Port); err != nil && err != http.ErrServerClosed {
		slog.Error("Server error", "component", "main", "operation", "start_server", "error", err)
		os.Exit(1)
	}

	slog.Info("Capacitarr shut down gracefully", "component", "main")
}
