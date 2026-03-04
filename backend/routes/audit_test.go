package routes_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"gorm.io/gorm"

	"capacitarr/internal/db"
	"capacitarr/internal/testutil"
)

// seedAuditLogs inserts n audit log records into the database for testing.
// Half are "Deleted" actions and half are "Dry-Run" actions.
func seedAuditLogs(t *testing.T, database *gorm.DB, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		action := "Deleted"
		if i%2 == 0 {
			action = "Dry-Run"
		}
		log := db.AuditLog{
			MediaName: fmt.Sprintf("Test Media %d", i),
			MediaType: "movie",
			Reason:    "Score: 0.85",
			Action:    action,
			SizeBytes: int64(1000000 * (i + 1)),
			CreatedAt: time.Now().Add(-time.Duration(i) * time.Minute),
		}
		if err := database.Create(&log).Error; err != nil {
			t.Fatalf("Failed to seed audit log %d: %v", i, err)
		}
	}
}

func TestGetAuditLogs_Empty(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	req := testutil.AuthenticatedRequest(t, http.MethodGet, "/api/audit", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Data   []db.AuditLog `json:"data"`
		Total  int64         `json:"total"`
		Limit  int           `json:"limit"`
		Offset int           `json:"offset"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if resp.Total != 0 {
		t.Errorf("Expected total 0 for empty state, got %d", resp.Total)
	}
	if len(resp.Data) != 0 {
		t.Errorf("Expected empty data slice, got %d items", len(resp.Data))
	}
	if resp.Limit != 50 {
		t.Errorf("Expected default limit 50, got %d", resp.Limit)
	}
	if resp.Offset != 0 {
		t.Errorf("Expected default offset 0, got %d", resp.Offset)
	}
}

func TestGetAuditLogs_WithData(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	seedAuditLogs(t, database, 5)

	req := testutil.AuthenticatedRequest(t, http.MethodGet, "/api/audit", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Data   []db.AuditLog `json:"data"`
		Total  int64         `json:"total"`
		Limit  int           `json:"limit"`
		Offset int           `json:"offset"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if resp.Total != 5 {
		t.Errorf("Expected total 5, got %d", resp.Total)
	}
	if len(resp.Data) != 5 {
		t.Errorf("Expected 5 items, got %d", len(resp.Data))
	}
}

func TestGetAuditLogs_Pagination(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	seedAuditLogs(t, database, 10)

	tests := []struct {
		name           string
		query          string
		expectedCount  int
		expectedTotal  int64
		expectedLimit  int
		expectedOffset int
	}{
		{
			name:           "limit 3",
			query:          "?limit=3",
			expectedCount:  3,
			expectedTotal:  10,
			expectedLimit:  3,
			expectedOffset: 0,
		},
		{
			name:           "limit 3 offset 5",
			query:          "?limit=3&offset=5",
			expectedCount:  3,
			expectedTotal:  10,
			expectedLimit:  3,
			expectedOffset: 5,
		},
		{
			name:           "offset past end",
			query:          "?offset=20",
			expectedCount:  0,
			expectedTotal:  10,
			expectedLimit:  50,
			expectedOffset: 20,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := testutil.AuthenticatedRequest(t, http.MethodGet, "/api/audit"+tc.query, nil)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("Expected 200, got %d: %s", rec.Code, rec.Body.String())
			}

			var resp struct {
				Data   []db.AuditLog `json:"data"`
				Total  int64         `json:"total"`
				Limit  int           `json:"limit"`
				Offset int           `json:"offset"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Fatalf("Failed to parse response: %v", err)
			}

			if len(resp.Data) != tc.expectedCount {
				t.Errorf("Expected %d items, got %d", tc.expectedCount, len(resp.Data))
			}
			if resp.Total != tc.expectedTotal {
				t.Errorf("Expected total %d, got %d", tc.expectedTotal, resp.Total)
			}
			if resp.Limit != tc.expectedLimit {
				t.Errorf("Expected limit %d, got %d", tc.expectedLimit, resp.Limit)
			}
			if resp.Offset != tc.expectedOffset {
				t.Errorf("Expected offset %d, got %d", tc.expectedOffset, resp.Offset)
			}
		})
	}
}

func TestGetAuditLogs_FilterByAction(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	seedAuditLogs(t, database, 10)

	tests := []struct {
		name          string
		action        string
		expectedCount int
	}{
		{"filter Deleted", "Deleted", 5},
		{"filter Dry-Run", "Dry-Run", 5},
		{"filter nonexistent", "Unknown", 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := testutil.AuthenticatedRequest(t, http.MethodGet, "/api/audit?action="+tc.action, nil)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("Expected 200, got %d: %s", rec.Code, rec.Body.String())
			}

			var resp struct {
				Data  []db.AuditLog `json:"data"`
				Total int64         `json:"total"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Fatalf("Failed to parse response: %v", err)
			}

			if int(resp.Total) != tc.expectedCount {
				t.Errorf("Expected total %d for action %q, got %d", tc.expectedCount, tc.action, resp.Total)
			}
		})
	}
}

func TestGetAuditLogs_SearchFilter(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	// Seed specific named items
	logs := []db.AuditLog{
		{MediaName: "Inception", MediaType: "movie", Reason: "test", Action: "Deleted", SizeBytes: 1000},
		{MediaName: "Interstellar", MediaType: "movie", Reason: "test", Action: "Deleted", SizeBytes: 2000},
		{MediaName: "The Dark Knight", MediaType: "movie", Reason: "test", Action: "Dry-Run", SizeBytes: 3000},
	}
	for _, log := range logs {
		if err := database.Create(&log).Error; err != nil {
			t.Fatalf("Failed to seed: %v", err)
		}
	}

	req := testutil.AuthenticatedRequest(t, http.MethodGet, "/api/audit?search=Inter", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Data  []db.AuditLog `json:"data"`
		Total int64         `json:"total"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if resp.Total != 1 {
		t.Errorf("Expected 1 result for search 'Inter', got %d", resp.Total)
	}
	if len(resp.Data) > 0 && resp.Data[0].MediaName != "Interstellar" {
		t.Errorf("Expected 'Interstellar', got %q", resp.Data[0].MediaName)
	}
}

func TestGetAuditLogs_Sorting(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	// Seed items with distinct sizes
	logs := []db.AuditLog{
		{MediaName: "Small", MediaType: "movie", Reason: "test", Action: "Deleted", SizeBytes: 100},
		{MediaName: "Large", MediaType: "movie", Reason: "test", Action: "Deleted", SizeBytes: 9000},
		{MediaName: "Medium", MediaType: "movie", Reason: "test", Action: "Deleted", SizeBytes: 5000},
	}
	for _, log := range logs {
		if err := database.Create(&log).Error; err != nil {
			t.Fatalf("Failed to seed: %v", err)
		}
	}

	req := testutil.AuthenticatedRequest(t, http.MethodGet, "/api/audit?sort_by=size_bytes&sort_dir=asc", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Data []db.AuditLog `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if len(resp.Data) != 3 {
		t.Fatalf("Expected 3 items, got %d", len(resp.Data))
	}
	if resp.Data[0].SizeBytes > resp.Data[1].SizeBytes || resp.Data[1].SizeBytes > resp.Data[2].SizeBytes {
		t.Errorf("Expected ascending size order, got %d, %d, %d",
			resp.Data[0].SizeBytes, resp.Data[1].SizeBytes, resp.Data[2].SizeBytes)
	}
}

func TestGetAuditLogs_LimitCap(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	// Request a limit above the 1000 cap
	req := testutil.AuthenticatedRequest(t, http.MethodGet, "/api/audit?limit=5000", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Limit int `json:"limit"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if resp.Limit != 1000 {
		t.Errorf("Expected limit capped to 1000, got %d", resp.Limit)
	}
}

func TestGetAuditActivity_Empty(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	req := testutil.AuthenticatedRequest(t, http.MethodGet, "/api/audit/activity", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var result []struct {
		Timestamp string `json:"timestamp"`
		Flagged   int    `json:"flagged"`
		Deleted   int    `json:"deleted"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if len(result) != 0 {
		t.Errorf("Expected empty activity for empty DB, got %d points", len(result))
	}
}

func TestGetAuditActivity_WithData(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	// Seed recent audit logs (within default 24h window)
	now := time.Now().UTC()
	logs := []db.AuditLog{
		{MediaName: "Movie A", MediaType: "movie", Reason: "test", Action: "Deleted", SizeBytes: 1000, CreatedAt: now.Add(-1 * time.Hour)},
		{MediaName: "Movie B", MediaType: "movie", Reason: "test", Action: "Dry-Run", SizeBytes: 2000, CreatedAt: now.Add(-1 * time.Hour)},
		{MediaName: "Movie C", MediaType: "movie", Reason: "test", Action: "Deleted", SizeBytes: 3000, CreatedAt: now.Add(-2 * time.Hour)},
	}
	for _, log := range logs {
		if err := database.Create(&log).Error; err != nil {
			t.Fatalf("Failed to seed: %v", err)
		}
	}

	req := testutil.AuthenticatedRequest(t, http.MethodGet, "/api/audit/activity?since=24h", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var result []struct {
		Timestamp string `json:"timestamp"`
		Flagged   int    `json:"flagged"`
		Deleted   int    `json:"deleted"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if len(result) == 0 {
		t.Fatal("Expected at least one activity point, got 0")
	}

	// Verify totals across all buckets
	totalFlagged := 0
	totalDeleted := 0
	for _, point := range result {
		totalFlagged += point.Flagged
		totalDeleted += point.Deleted
	}

	if totalDeleted != 2 {
		t.Errorf("Expected 2 total deleted across buckets, got %d", totalDeleted)
	}
	if totalFlagged != 1 {
		t.Errorf("Expected 1 total flagged (Dry-Run) across buckets, got %d", totalFlagged)
	}
}

func TestGetAuditActivity_InvalidSince(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	req := testutil.AuthenticatedRequest(t, http.MethodGet, "/api/audit/activity?since=invalid", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for invalid since parameter, got %d", rec.Code)
	}
}

func TestGetAuditLogs_Unauthenticated(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	req := httptest.NewRequest(http.MethodGet, "/api/audit", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code == http.StatusOK {
		t.Error("Expected non-200 for unauthenticated request to audit endpoint")
	}
}

// seedApprovalEntry creates an integration and a "Queued for Approval" audit log entry.
// Returns the audit log ID and integration ID.
func seedApprovalEntry(t *testing.T, database *gorm.DB) (auditID uint, integrationID uint) {
	t.Helper()

	// Create an integration config (needed for approve to look up the client)
	integration := db.IntegrationConfig{
		Type:    "radarr",
		Name:    "Test Radarr",
		URL:     "http://localhost:7878",
		APIKey:  "test-api-key",
		Enabled: true,
	}
	if err := database.Create(&integration).Error; err != nil {
		t.Fatalf("Failed to seed integration: %v", err)
	}

	entry := db.AuditLog{
		MediaName:     "Approval Test Movie",
		MediaType:     "movie",
		Reason:        "Score: 0.75 (WatchHistory: 0.5, Size: 0.8)",
		ScoreDetails:  `[{"name":"WatchHistory","rawScore":0.5,"weight":10},{"name":"Size","rawScore":0.8,"weight":6}]`,
		Action:        "Queued for Approval",
		SizeBytes:     5000000,
		IntegrationID: &integration.ID,
		ExternalID:    "123",
		CreatedAt:     time.Now(),
	}
	if err := database.Create(&entry).Error; err != nil {
		t.Fatalf("Failed to seed approval audit entry: %v", err)
	}

	return entry.ID, integration.ID
}

func TestApproveAuditEntry_HappyPath(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	// Set the global db.DB so the background deletion worker (started by
	// poller.init) can access the database when it processes the queued job.
	// We intentionally do NOT restore db.DB to nil in cleanup because the
	// worker goroutine processes asynchronously and would panic on a nil DB.
	db.DB = database

	auditID, _ := seedApprovalEntry(t, database)

	req := testutil.AuthenticatedRequest(t, http.MethodPost,
		fmt.Sprintf("/api/audit/%d/approve", auditID), nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	if resp["status"] != "approved" {
		t.Errorf("Expected status 'approved', got %q", resp["status"])
	}

	// Verify the audit entry was updated to "Approved"
	var updated db.AuditLog
	if err := database.First(&updated, auditID).Error; err != nil {
		t.Fatalf("Failed to find updated audit entry: %v", err)
	}
	if updated.Action != "Approved" {
		t.Errorf("Expected action 'Approved', got %q", updated.Action)
	}
}

func TestApproveAuditEntry_NotFound(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	req := testutil.AuthenticatedRequest(t, http.MethodPost, "/api/audit/99999/approve", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("Expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestApproveAuditEntry_NotQueued(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	// Create an entry with action "Dry-Run" (not "Queued for Approval")
	entry := db.AuditLog{
		MediaName: "Not Queued Movie",
		MediaType: "movie",
		Reason:    "Score: 0.50",
		Action:    "Dry-Run",
		SizeBytes: 1000000,
		CreatedAt: time.Now(),
	}
	if err := database.Create(&entry).Error; err != nil {
		t.Fatalf("Failed to seed: %v", err)
	}

	req := testutil.AuthenticatedRequest(t, http.MethodPost,
		fmt.Sprintf("/api/audit/%d/approve", entry.ID), nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for non-queued entry, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestRejectAuditEntry_HappyPath(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	auditID, _ := seedApprovalEntry(t, database)

	req := testutil.AuthenticatedRequest(t, http.MethodPost,
		fmt.Sprintf("/api/audit/%d/reject", auditID), nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	if resp["status"] != "rejected" {
		t.Errorf("Expected status 'rejected', got %q", resp["status"])
	}

	// Verify the audit entry was updated to "Rejected"
	var updated db.AuditLog
	if err := database.First(&updated, auditID).Error; err != nil {
		t.Fatalf("Failed to find updated audit entry: %v", err)
	}
	if updated.Action != "Rejected" {
		t.Errorf("Expected action 'Rejected', got %q", updated.Action)
	}
}

func TestRejectAuditEntry_NotFound(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	req := testutil.AuthenticatedRequest(t, http.MethodPost, "/api/audit/99999/reject", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("Expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}
