package routes_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"capacitarr/internal/db"
	"capacitarr/internal/testutil"
)

func TestGetSunsetQueue(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	// Seed data
	dg := db.DiskGroup{MountPath: "/data", TotalBytes: 100000, ThresholdPct: 85, TargetPct: 75, Mode: db.ModeSunset}
	database.Create(&dg)
	ic := db.IntegrationConfig{Type: "sonarr", Name: "Test", URL: "http://localhost:8989", APIKey: "key"}
	database.Create(&ic)
	database.Create(&db.SunsetQueueItem{
		MediaName: "Firefly", MediaType: "show", IntegrationID: ic.ID,
		SizeBytes: 5000000000, Score: 0.85, DiskGroupID: dg.ID,
		Trigger: db.TriggerEngine, DeletionDate: time.Now().UTC().AddDate(0, 0, 30),
	})

	req := testutil.AuthenticatedRequest(t, http.MethodGet, "/api/sunset-queue", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var items []map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &items); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("Expected 1 item, got %d", len(items))
	}
	if items[0]["mediaName"] != "Firefly" {
		t.Errorf("Expected mediaName 'Firefly', got %v", items[0]["mediaName"])
	}
	if _, ok := items[0]["daysRemaining"]; !ok {
		t.Error("Expected daysRemaining field in response")
	}
}

func TestCancelSunsetItem(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	// Seed data
	dg := db.DiskGroup{MountPath: "/data", TotalBytes: 100000, ThresholdPct: 85, TargetPct: 75, Mode: db.ModeSunset}
	database.Create(&dg)
	ic := db.IntegrationConfig{Type: "sonarr", Name: "Test", URL: "http://localhost:8989", APIKey: "key"}
	database.Create(&ic)
	item := db.SunsetQueueItem{
		MediaName: "Firefly", MediaType: "show", IntegrationID: ic.ID,
		SizeBytes: 5000000000, DiskGroupID: dg.ID,
		Trigger: db.TriggerEngine, DeletionDate: time.Now().UTC().AddDate(0, 0, 30),
	}
	database.Create(&item)

	req := testutil.AuthenticatedRequest(t, http.MethodDelete, fmt.Sprintf("/api/sunset-queue/%d", item.ID), nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify item was deleted
	var count int64
	database.Model(&db.SunsetQueueItem{}).Count(&count)
	if count != 0 {
		t.Errorf("Expected 0 items after cancel, got %d", count)
	}
}

func TestCancelSunsetItem_NotFound(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	req := testutil.AuthenticatedRequest(t, http.MethodDelete, "/api/sunset-queue/99999", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("Expected 404, got %d", rec.Code)
	}
}

func TestRescheduleSunsetItem(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	// Seed data
	dg := db.DiskGroup{MountPath: "/data", TotalBytes: 100000, ThresholdPct: 85, TargetPct: 75, Mode: db.ModeSunset}
	database.Create(&dg)
	ic := db.IntegrationConfig{Type: "sonarr", Name: "Test", URL: "http://localhost:8989", APIKey: "key"}
	database.Create(&ic)
	item := db.SunsetQueueItem{
		MediaName: "Firefly", MediaType: "show", IntegrationID: ic.ID,
		SizeBytes: 5000000000, DiskGroupID: dg.ID,
		Trigger: db.TriggerEngine, DeletionDate: time.Now().UTC().AddDate(0, 0, 30),
	}
	database.Create(&item)

	newDate := time.Now().UTC().AddDate(0, 0, 60).Format("2006-01-02")
	body := fmt.Sprintf(`{"deletionDate":"%s"}`, newDate)
	req := testutil.AuthenticatedRequest(t, http.MethodPatch, fmt.Sprintf("/api/sunset-queue/%d", item.ID), strings.NewReader(body))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var result map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	if result["deletionDate"] != newDate {
		t.Errorf("Expected deletionDate %s, got %v", newDate, result["deletionDate"])
	}
}

func TestRescheduleSunsetItem_PastDate(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	dg := db.DiskGroup{MountPath: "/data", TotalBytes: 100000, ThresholdPct: 85, TargetPct: 75, Mode: db.ModeSunset}
	database.Create(&dg)
	ic := db.IntegrationConfig{Type: "sonarr", Name: "Test", URL: "http://localhost:8989", APIKey: "key"}
	database.Create(&ic)
	item := db.SunsetQueueItem{
		MediaName: "Firefly", MediaType: "show", IntegrationID: ic.ID,
		SizeBytes: 5000000000, DiskGroupID: dg.ID,
		Trigger: db.TriggerEngine, DeletionDate: time.Now().UTC().AddDate(0, 0, 30),
	}
	database.Create(&item)

	pastDate := time.Now().UTC().AddDate(0, 0, -5).Format("2006-01-02")
	body := fmt.Sprintf(`{"deletionDate":"%s"}`, pastDate)
	req := testutil.AuthenticatedRequest(t, http.MethodPatch, fmt.Sprintf("/api/sunset-queue/%d", item.ID), strings.NewReader(body))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for past date, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestClearSunsetQueue(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	// Seed data
	dg := db.DiskGroup{MountPath: "/data", TotalBytes: 100000, ThresholdPct: 85, TargetPct: 75, Mode: db.ModeSunset}
	database.Create(&dg)
	ic := db.IntegrationConfig{Type: "sonarr", Name: "Test", URL: "http://localhost:8989", APIKey: "key"}
	database.Create(&ic)
	for i := 0; i < 3; i++ {
		database.Create(&db.SunsetQueueItem{
			MediaName: "Firefly", MediaType: "show", IntegrationID: ic.ID,
			ExternalID: fmt.Sprintf("ext-%d", i),
			SizeBytes: 1000000, DiskGroupID: dg.ID,
			Trigger: db.TriggerEngine, DeletionDate: time.Now().UTC().AddDate(0, 0, 30),
		})
	}

	req := testutil.AuthenticatedRequest(t, http.MethodPost, "/api/sunset-queue/clear", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var result map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	if cancelled, ok := result["cancelled"].(float64); !ok || int(cancelled) != 3 {
		t.Errorf("Expected cancelled=3, got %v", result["cancelled"])
	}
}
