package routes_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"

	"capacitarr/internal/events"
	"capacitarr/internal/services"
	"capacitarr/internal/testutil"
	"capacitarr/routes"
)

func setupHealthServer(t *testing.T) (*echo.Echo, *services.Registry) {
	t.Helper()
	database := testutil.SetupTestDB(t)
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	bus := events.NewEventBus()
	t.Cleanup(func() { bus.Close() })

	reg := services.NewRegistry(database, bus, testutil.TestConfig())
	api := e.Group("/api/v1")
	routes.RegisterAPIRoutes(api, reg, "test", "abc", "now", nil)
	return e, reg
}

func TestHealth_OK(t *testing.T) {
	e, _ := setupHealthServer(t)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/health", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	if resp["status"] != "ok" {
		t.Errorf("expected status ok, got %v", resp["status"])
	}
}

func TestHealth_UnhealthyWhenDBClosed(t *testing.T) {
	e, reg := setupHealthServer(t)

	sqlDB, err := reg.DB.DB()
	if err != nil {
		t.Fatalf("failed to get sql.DB: %v", err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatalf("failed to close database: %v", err)
	}

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/health", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("Expected 503, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	if resp["status"] != "unhealthy" {
		t.Errorf("expected status unhealthy, got %v", resp["status"])
	}
}
