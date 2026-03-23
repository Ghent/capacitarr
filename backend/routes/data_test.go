package routes_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"gorm.io/gorm"

	"capacitarr/internal/db"
	"capacitarr/internal/testutil"
)

// ---------- helpers ----------

// seedDataForReset populates the database with records that the reset endpoint
// should clear: audit logs, library histories, engine run stats, disk groups,
// and integration configs with transient fields set.
func seedDataForReset(t *testing.T, database *gorm.DB) {
	t.Helper()

	// Audit logs
	for i := 0; i < 3; i++ {
		if err := database.Create(&db.AuditLogEntry{
			MediaName: "Test Movie",
			MediaType: "movie",
			Action:    "deleted",
			SizeBytes: 1000000,
		}).Error; err != nil {
			t.Fatalf("Failed to seed audit log: %v", err)
		}
	}

	// Library histories
	now := time.Now()
	for i := 0; i < 2; i++ {
		if err := database.Create(&db.LibraryHistory{
			Timestamp:     now.Add(-time.Duration(i) * time.Hour),
			TotalCapacity: 100000000,
			UsedCapacity:  80000000,
			Resolution:    "raw",
		}).Error; err != nil {
			t.Fatalf("Failed to seed library history: %v", err)
		}
	}

	// Engine run stats
	if err := database.Create(&db.EngineRunStats{
		RunAt:         now,
		Evaluated:     10,
		Candidates:    3,
		FreedBytes:    5000000,
		ExecutionMode: db.ModeDryRun,
		DurationMs:    150,
	}).Error; err != nil {
		t.Fatalf("Failed to seed engine run stats: %v", err)
	}

	// Disk groups
	if err := database.Create(&db.DiskGroup{
		MountPath:    "/mnt/media",
		TotalBytes:   1000000000,
		UsedBytes:    800000000,
		ThresholdPct: 85,
		TargetPct:    75,
	}).Error; err != nil {
		t.Fatalf("Failed to seed disk group: %v", err)
	}

	// Integration config with transient fields
	syncTime := now
	if err := database.Create(&db.IntegrationConfig{
		Type:           "sonarr",
		Name:           "Test Sonarr",
		URL:            "http://localhost:8989",
		APIKey:         "testkey123456789",
		Enabled:        true,
		MediaSizeBytes: 50000000,
		MediaCount:     100,
		LastSync:       &syncTime,
		LastError:      "previous error",
	}).Error; err != nil {
		t.Fatalf("Failed to seed integration config: %v", err)
	}
}

// ---------- DELETE /api/data/reset ----------

func TestDataReset_ClearsAllData(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	seedDataForReset(t, database)

	// Verify data was seeded
	var auditCount, historyCount, statsCount, diskCount int64
	database.Model(&db.AuditLogEntry{}).Count(&auditCount)
	database.Model(&db.LibraryHistory{}).Count(&historyCount)
	database.Model(&db.EngineRunStats{}).Count(&statsCount)
	database.Model(&db.DiskGroup{}).Count(&diskCount)

	if auditCount == 0 || historyCount == 0 || statsCount == 0 || diskCount == 0 {
		t.Fatal("Seed data missing; cannot test reset")
	}

	// Perform reset
	req := testutil.AuthenticatedRequest(t, http.MethodDelete, "/api/data/reset", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify response structure
	var resp struct {
		Status  string         `json:"status"`
		Message string         `json:"message"`
		Cleared map[string]int `json:"cleared"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if resp.Status != "success" {
		t.Errorf("Expected status 'success', got %q", resp.Status)
	}
	if resp.Message == "" {
		t.Error("Expected non-empty message")
	}

	// Verify counts in response
	if resp.Cleared["auditLog"] != 3 {
		t.Errorf("Expected 3 audit logs cleared, got %d", resp.Cleared["auditLog"])
	}
	if resp.Cleared["libraryHistories"] != 2 {
		t.Errorf("Expected 2 library histories cleared, got %d", resp.Cleared["libraryHistories"])
	}
	if resp.Cleared["engineRunStats"] != 1 {
		t.Errorf("Expected 1 engine run stat cleared, got %d", resp.Cleared["engineRunStats"])
	}
	if resp.Cleared["diskGroupsReset"] != 1 {
		t.Errorf("Expected 1 disk group reset, got %d", resp.Cleared["diskGroupsReset"])
	}
	if resp.Cleared["integrationsReset"] != 1 {
		t.Errorf("Expected 1 integration reset, got %d", resp.Cleared["integrationsReset"])
	}

	// Verify deleted tables are actually empty
	database.Model(&db.AuditLogEntry{}).Count(&auditCount)
	database.Model(&db.LibraryHistory{}).Count(&historyCount)
	database.Model(&db.EngineRunStats{}).Count(&statsCount)

	if auditCount != 0 {
		t.Errorf("Expected 0 audit logs after reset, got %d", auditCount)
	}
	if historyCount != 0 {
		t.Errorf("Expected 0 library histories after reset, got %d", historyCount)
	}
	if statsCount != 0 {
		t.Errorf("Expected 0 engine run stats after reset, got %d", statsCount)
	}

	// Verify disk groups still exist with thresholds preserved but transient fields zeroed
	var diskGroups []db.DiskGroup
	database.Find(&diskGroups)
	if len(diskGroups) != 1 {
		t.Fatalf("Expected 1 disk group to still exist after reset, got %d", len(diskGroups))
	}
	dg := diskGroups[0]
	if dg.ThresholdPct != 85 {
		t.Errorf("Expected ThresholdPct preserved at 85, got %v", dg.ThresholdPct)
	}
	if dg.TargetPct != 75 {
		t.Errorf("Expected TargetPct preserved at 75, got %v", dg.TargetPct)
	}
	if dg.TotalBytes != 0 {
		t.Errorf("Expected TotalBytes reset to 0, got %d", dg.TotalBytes)
	}
	if dg.UsedBytes != 0 {
		t.Errorf("Expected UsedBytes reset to 0, got %d", dg.UsedBytes)
	}
	if dg.MountPath != "/mnt/media" {
		t.Errorf("Expected MountPath preserved, got %q", dg.MountPath)
	}

	// Verify integration transient fields were reset but configs still exist
	var intConfigs []db.IntegrationConfig
	database.Find(&intConfigs)
	if len(intConfigs) != 1 {
		t.Fatalf("Expected 1 integration config to still exist, got %d", len(intConfigs))
	}
	cfg := intConfigs[0]
	if cfg.MediaSizeBytes != 0 {
		t.Errorf("Expected MediaSizeBytes reset to 0, got %d", cfg.MediaSizeBytes)
	}
	if cfg.MediaCount != 0 {
		t.Errorf("Expected MediaCount reset to 0, got %d", cfg.MediaCount)
	}
	if cfg.LastError != "" {
		t.Errorf("Expected LastError cleared, got %q", cfg.LastError)
	}
	// Name, URL, APIKey should be preserved
	if cfg.Name != "Test Sonarr" {
		t.Errorf("Expected integration name preserved, got %q", cfg.Name)
	}
}

func TestDataReset_EmptyState(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	// Reset on empty database should succeed with 0 counts
	req := testutil.AuthenticatedRequest(t, http.MethodDelete, "/api/data/reset", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Status  string         `json:"status"`
		Cleared map[string]int `json:"cleared"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if resp.Status != "success" {
		t.Errorf("Expected status 'success', got %q", resp.Status)
	}
	if resp.Cleared["auditLog"] != 0 {
		t.Errorf("Expected 0 audit logs cleared on empty state, got %d", resp.Cleared["auditLog"])
	}
}

func TestDataReset_Unauthenticated(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/api/data/reset", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code == http.StatusOK {
		t.Error("Expected non-200 for unauthenticated request to data reset endpoint")
	}
}

func TestDataReset_PreservesCustomRules(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	// Seed a protection rule
	rule := db.CustomRule{
		Field:    "title",
		Operator: "contains",
		Value:    "Serenity",
		Effect:   "always_keep",
		Enabled:  true,
	}
	if err := database.Create(&rule).Error; err != nil {
		t.Fatalf("Failed to seed protection rule: %v", err)
	}

	// Seed some data that should be cleared
	if err := database.Create(&db.AuditLogEntry{
		MediaName: "Test", MediaType: "movie", Action: "deleted", SizeBytes: 100,
	}).Error; err != nil {
		t.Fatalf("Failed to seed audit log: %v", err)
	}

	// Perform reset
	req := testutil.AuthenticatedRequest(t, http.MethodDelete, "/api/data/reset", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Protection rules should NOT be deleted by data reset
	var ruleCount int64
	database.Model(&db.CustomRule{}).Count(&ruleCount)
	if ruleCount != 1 {
		t.Errorf("Expected 1 protection rule preserved after reset, got %d", ruleCount)
	}
}

func TestDataReset_PreservesPreferences(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	// Perform reset
	req := testutil.AuthenticatedRequest(t, http.MethodDelete, "/api/data/reset", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Preferences should NOT be deleted by data reset
	var prefs db.PreferenceSet
	if err := database.First(&prefs, 1).Error; err != nil {
		t.Errorf("Expected preferences to be preserved after reset: %v", err)
	}
}

func TestDataReset_WrongHTTPMethod(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	// The endpoint is DELETE only — other methods should return 405
	methods := []string{http.MethodGet, http.MethodPost, http.MethodPut}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := testutil.AuthenticatedRequest(t, method, "/api/data/reset", nil)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			if rec.Code == http.StatusOK {
				t.Errorf("Expected non-200 for %s on data/reset, got %d", method, rec.Code)
			}
		})
	}
}
