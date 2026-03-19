// Package testutil provides helpers for integration tests that require
// an in-memory SQLite database with Goose migrations applied, an Echo
// server with routes registered, and authenticated HTTP requests with
// valid JWT tokens.
package testutil

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"io"
	"log"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
	_ "github.com/ncruces/go-sqlite3/embed" // load the embedded SQLite WASM binary

	"capacitarr/internal/integrations"
	"github.com/ncruces/go-sqlite3/gormlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"capacitarr/internal/config"
	"capacitarr/internal/db"
	"capacitarr/internal/events"
	"capacitarr/internal/services"
	"capacitarr/routes"
)

// GenerateTestCSPNonce produces a cryptographically random, base64url-encoded
// nonce for Content-Security-Policy headers. This mirrors the production
// generateCSPNonce function in main.go (which is in package main and cannot
// be imported directly).
func GenerateTestCSPNonce() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

// SilenceLogs suppresses all slog and standard log output for the duration
// of the current test. This prevents config.Load(), db.Init(), and Goose
// migration log messages from polluting test output with spurious warnings
// (e.g., "No JWT_SECRET set", "AUTH_HEADER is set", "UNIQUE constraint failed").
// These messages are expected during tests but confuse users reading test output.
func SilenceLogs(t *testing.T) {
	t.Helper()

	// Suppress slog (structured logging used by Capacitarr's application code)
	prevSlog := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	t.Cleanup(func() { slog.SetDefault(prevSlog) })

	// Suppress standard log (used by Goose migrations and GORM)
	prevOutput := log.Writer()
	prevFlags := log.Flags()
	log.SetOutput(io.Discard)
	log.SetFlags(0)
	t.Cleanup(func() {
		log.SetOutput(prevOutput)
		log.SetFlags(prevFlags)
	})
}

// TestJWTSecret is the secret used for signing JWT tokens in tests.
const TestJWTSecret = "test-jwt-secret-for-unit-tests"

// TestUsername is the default username used in authenticated requests.
const TestUsername = "testadmin"

// SetupTestDB creates an in-memory SQLite database with all Goose
// migrations applied and the default PreferenceSet seeded. It calls
// t.Fatal if anything fails.
func SetupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	SilenceLogs(t)

	database, err := gorm.Open(gormlite.Open(":memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("Failed to open in-memory SQLite: %v", err)
	}

	sqlDB, err := database.DB()
	if err != nil {
		t.Fatalf("Failed to get underlying sql.DB: %v", err)
	}

	// Force a single connection so every operation uses the same in-memory
	// database.  Without this, the connection pool may hand out different
	// connections, each with its own (empty) :memory: database, causing
	// "no such table" errors.
	sqlDB.SetMaxOpenConns(1)

	if err := db.RunMigrations(sqlDB); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	// Seed default preferences (mirrors db.Init behaviour)
	pref := db.PreferenceSet{
		ID:                    1,
		ExecutionMode:         "dry-run",
		LogLevel:              "info",
		AuditLogRetentionDays: 30,
		PollIntervalSeconds:   300,
		WatchHistoryWeight:    10,
		LastWatchedWeight:     8,
		FileSizeWeight:        6,
		RatingWeight:          5,
		TimeInLibraryWeight:   4,
		SeriesStatusWeight:    3,
		TiebreakerMethod:      "size_desc",
		DeletionsEnabled:      true,
		SnoozeDurationHours:   24,
		CheckForUpdates:       true,
	}
	if err := database.FirstOrCreate(&pref, db.PreferenceSet{ID: 1}).Error; err != nil {
		t.Fatalf("Failed to seed preferences: %v", err)
	}

	return database
}

// TestConfig returns a Config suitable for tests with a known JWT secret.
func TestConfig() *config.Config {
	return &config.Config{
		Port:          "0",
		BaseURL:       "/",
		Database:      ":memory:",
		Debug:         true,
		JWTSecret:     TestJWTSecret,
		CORSOrigins:   []string{"*"},
		SecureCookies: false,
		AuthHeader:    "",
	}
}

// SetupTestServer creates an Echo instance with auth, preferences,
// integrations, and rule routes registered. It uses the test config
// with a known JWT secret.
func SetupTestServer(t *testing.T, database *gorm.DB) *echo.Echo {
	t.Helper()
	e, _ := SetupTestServerWithRegistry(t, database)
	return e
}

// SetupTestServerWithRegistry creates an Echo instance and returns both the
// server and the service registry. Use this when tests need to access the
// registry directly (e.g., to configure VersionService's mock URL).
func SetupTestServerWithRegistry(t *testing.T, database *gorm.DB) (*echo.Echo, *services.Registry) {
	t.Helper()

	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	cfg := TestConfig()

	// Ensure integration factories are registered for any tests that involve
	// client creation (approval, deletion, etc.)
	integrations.RegisterAllFactories()

	// Apply the same security headers middleware as main.go.
	// This ensures security regression tests verify production behavior.
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			h := c.Response().Header()
			h.Set("X-Content-Type-Options", "nosniff")
			h.Set("X-Frame-Options", "DENY")
			h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
			h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
			h.Set("X-Permitted-Cross-Domain-Policies", "none")
			h.Set("Cross-Origin-Opener-Policy", "same-origin")
			h.Set("Cross-Origin-Resource-Policy", "same-origin")

			// Generate a per-request CSP nonce, mirroring main.go behavior.
			nonce := GenerateTestCSPNonce()
			c.Set("cspNonce", nonce)
			h.Set("Content-Security-Policy",
				"default-src 'self'; "+
					"script-src 'self' 'nonce-"+nonce+"'; "+
					"style-src 'self' 'unsafe-inline'; "+
					"img-src 'self' data: https:; "+
					"font-src 'self'; "+
					"connect-src 'self'; "+
					"frame-ancestors 'none'; "+
					"base-uri 'self'; "+
					"form-action 'self'")
			if cfg.SecureCookies {
				h.Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
			}
			return next(c)
		}
	})

	api := e.Group("/api")

	// Create a test event bus (no subscribers needed for most tests)
	bus := events.NewEventBus()
	t.Cleanup(func() { bus.Close() })

	// Create a test service registry
	reg := services.NewRegistry(database, bus, cfg)
	reg.InitVersion("v0.0.0-test")

	// Public routes
	api.GET("/health", func(c echo.Context) error {
		return c.String(http.StatusOK, "OK")
	})

	// Protected routes
	protected := api.Group("")
	protected.Use(routes.RequireAuth(reg))

	routes.RegisterAuthRoutes(api, protected, reg)
	routes.RegisterMetricsRoutes(protected, reg)
	routes.RegisterIntegrationRoutes(protected, reg)
	routes.RegisterRuleRoutes(protected, reg)
	routes.RegisterDiskGroupRoutes(protected, reg)
	routes.RegisterAuditRoutes(protected, reg)
	routes.RegisterApprovalRoutes(protected, reg)
	routes.RegisterActivityRoutes(protected, reg)
	routes.RegisterEngineRoutes(protected, reg)
	routes.RegisterDataRoutes(protected, reg)
	routes.RegisterNotificationRoutes(protected, reg)
	routes.RegisterVersionRoutes(protected, reg)
	routes.RegisterBackupRoutes(protected, reg, "v0.0.0-test")
	routes.RegisterDeletionQueueRoutes(protected, reg)
	routes.RegisterAnalyticsRoutes(protected, reg)
	routes.RegisterLibraryRoutes(protected, reg)
	routes.RegisterPreviewRoutes(protected, reg)
	routes.RegisterMigrationRoutes(api, protected, reg)

	return e, reg
}

// GenerateTestJWT creates a valid JWT token string signed with TestJWTSecret
// and the given subject (username). The token expires in 24 hours.
func GenerateTestJWT(t *testing.T, username string) string {
	t.Helper()

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": username,
		"exp": time.Now().Add(24 * time.Hour).Unix(),
	})

	tokenString, err := token.SignedString([]byte(TestJWTSecret)) // nosemgrep: go.jwt-go.security.jwt.hardcoded-jwt-key — test-only constant, not a production secret
	if err != nil {
		t.Fatalf("Failed to sign JWT: %v", err)
	}
	return tokenString
}

// AuthenticatedRequest creates an *http.Request with a valid JWT Bearer token
// in the Authorization header. Uses TestUsername as the subject.
func AuthenticatedRequest(t *testing.T, method, path string, body io.Reader) *http.Request {
	t.Helper()

	req, err := http.NewRequestWithContext(context.Background(), method, path, body)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+GenerateTestJWT(t, TestUsername))
	req.Header.Set("Content-Type", "application/json")
	return req
}
