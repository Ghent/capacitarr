package routes_test

import (
	"context"
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
		action := "deleted"
		if i%2 == 0 {
			action = "dry_delete"
		}
		log := db.AuditLogEntry{
			MediaName: fmt.Sprintf("Test Media %d", i),
			MediaType: "movie",
			Action:    action,
			SizeBytes: int64(1000000 * (i + 1)),
			Score:     0.85,
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

	req := testutil.AuthenticatedRequest(t, http.MethodGet, "/api/audit-log", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Data   []db.AuditLogEntry `json:"data"`
		Total  int64              `json:"total"`
		Limit  int                `json:"limit"`
		Offset int                `json:"offset"`
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

	req := testutil.AuthenticatedRequest(t, http.MethodGet, "/api/audit-log", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Data   []db.AuditLogEntry `json:"data"`
		Total  int64              `json:"total"`
		Limit  int                `json:"limit"`
		Offset int                `json:"offset"`
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

	// Verify score field appears in API response
	for _, entry := range resp.Data {
		if entry.Score != 0.85 {
			t.Errorf("Expected score 0.85 in API response, got %f for %q", entry.Score, entry.MediaName)
		}
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
			req := testutil.AuthenticatedRequest(t, http.MethodGet, "/api/audit-log"+tc.query, nil)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("Expected 200, got %d: %s", rec.Code, rec.Body.String())
			}

			var resp struct {
				Data   []db.AuditLogEntry `json:"data"`
				Total  int64              `json:"total"`
				Limit  int                `json:"limit"`
				Offset int                `json:"offset"`
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
		{"filter deleted", "deleted", 5},
		{"filter dry_delete", "dry_delete", 5},
		{"filter nonexistent", "Unknown", 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := testutil.AuthenticatedRequest(t, http.MethodGet, "/api/audit-log?action="+tc.action, nil)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("Expected 200, got %d: %s", rec.Code, rec.Body.String())
			}

			var resp struct {
				Data  []db.AuditLogEntry `json:"data"`
				Total int64              `json:"total"`
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
	logs := []db.AuditLogEntry{
		{MediaName: "Serenity", MediaType: "movie", Action: "deleted", SizeBytes: 1000},
		{MediaName: "Serenity 2", MediaType: "movie", Action: "deleted", SizeBytes: 2000},
		{MediaName: "Serenity 3", MediaType: "movie", Action: "dry_delete", SizeBytes: 3000},
	}
	for _, log := range logs {
		if err := database.Create(&log).Error; err != nil {
			t.Fatalf("Failed to seed: %v", err)
		}
	}

	req := testutil.AuthenticatedRequest(t, http.MethodGet, "/api/audit-log?search=Serenity+2", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Data  []db.AuditLogEntry `json:"data"`
		Total int64              `json:"total"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if resp.Total != 1 {
		t.Errorf("Expected 1 result for search 'Serenity 2', got %d", resp.Total)
	}
	if len(resp.Data) > 0 && resp.Data[0].MediaName != "Serenity 2" {
		t.Errorf("Expected 'Serenity 2', got %q", resp.Data[0].MediaName)
	}
}

func TestGetAuditLogs_Sorting(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	// Seed items with distinct sizes
	logs := []db.AuditLogEntry{
		{MediaName: "Small", MediaType: "movie", Action: "deleted", SizeBytes: 100},
		{MediaName: "Large", MediaType: "movie", Action: "deleted", SizeBytes: 9000},
		{MediaName: "Medium", MediaType: "movie", Action: "deleted", SizeBytes: 5000},
	}
	for _, log := range logs {
		if err := database.Create(&log).Error; err != nil {
			t.Fatalf("Failed to seed: %v", err)
		}
	}

	req := testutil.AuthenticatedRequest(t, http.MethodGet, "/api/audit-log?sort_by=size_bytes&sort_dir=asc", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Data []db.AuditLogEntry `json:"data"`
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
	req := testutil.AuthenticatedRequest(t, http.MethodGet, "/api/audit-log?limit=5000", nil)
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

func TestGetAuditLogs_Unauthenticated(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/audit-log", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code == http.StatusOK {
		t.Error("Expected non-200 for unauthenticated request to audit endpoint")
	}
}

// seedApprovalEntry creates an integration and a pending approval queue item.
// Returns the queue item ID.
func seedApprovalEntry(t *testing.T, database *gorm.DB) (itemID uint) {
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

	entry := db.ApprovalQueueItem{
		MediaName:     "Approval Test Movie",
		MediaType:     "movie",
		ScoreDetails:  `[{"name":"WatchHistory","rawScore":0.5,"weight":10},{"name":"Size","rawScore":0.8,"weight":6}]`,
		Status:        "pending",
		SizeBytes:     5000000,
		Score:         0.75,
		IntegrationID: integration.ID,
		ExternalID:    "ext-test-1",
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
	if err := database.Create(&entry).Error; err != nil {
		t.Fatalf("Failed to seed approval queue item: %v", err)
	}

	return entry.ID
}

func TestApproveEntry_HappyPath(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	itemID := seedApprovalEntry(t, database)

	req := testutil.AuthenticatedRequest(t, http.MethodPost,
		fmt.Sprintf("/api/approval-queue/%d/approve", itemID), nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	if resp["status"] != "approved" {
		t.Errorf("Expected status 'approved', got %v", resp["status"])
	}

	// Verify the queue item was updated to "approved"
	var updated db.ApprovalQueueItem
	if err := database.First(&updated, itemID).Error; err != nil {
		t.Fatalf("Failed to find updated queue item: %v", err)
	}
	if updated.Status != "approved" {
		t.Errorf("Expected status 'approved', got %q", updated.Status)
	}
}

func TestApproveEntry_NotFound(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	req := testutil.AuthenticatedRequest(t, http.MethodPost, "/api/approval-queue/99999/approve", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("Expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestApproveEntry_NotPending(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	// Create an integration first
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

	// Create an entry with status "approved" (not "pending")
	entry := db.ApprovalQueueItem{
		MediaName:     "Not Pending Movie",
		MediaType:     "movie",
		Status:        "approved",
		SizeBytes:     1000000,
		IntegrationID: integration.ID,
		ExternalID:    "ext-test-2",
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
	if err := database.Create(&entry).Error; err != nil {
		t.Fatalf("Failed to seed: %v", err)
	}

	req := testutil.AuthenticatedRequest(t, http.MethodPost,
		fmt.Sprintf("/api/approval-queue/%d/approve", entry.ID), nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for non-pending entry, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestApproveEntry_DeletionsDisabled(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	itemID := seedApprovalEntry(t, database)

	// Disable deletions in preferences — approval should still succeed
	// (the DeletionService will dry-delete via ForceDryRun)
	if err := database.Model(&db.PreferenceSet{}).Where("id = 1").
		Update("deletions_enabled", false).Error; err != nil {
		t.Fatalf("Failed to disable deletions: %v", err)
	}

	req := testutil.AuthenticatedRequest(t, http.MethodPost,
		fmt.Sprintf("/api/approval-queue/%d/approve", itemID), nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200 (approve with dry-run simulation), got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify the queue item was approved (DeletionService will dry-delete it)
	var entry db.ApprovalQueueItem
	if err := database.First(&entry, itemID).Error; err != nil {
		t.Fatalf("Failed to find queue item: %v", err)
	}
	if entry.Status != "approved" {
		t.Errorf("Expected status 'approved', got %q", entry.Status)
	}
}

func TestRejectEntry_HappyPath(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	itemID := seedApprovalEntry(t, database)

	req := testutil.AuthenticatedRequest(t, http.MethodPost,
		fmt.Sprintf("/api/approval-queue/%d/reject", itemID), nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	if resp["status"] != "rejected" {
		t.Errorf("Expected status 'rejected', got %v", resp["status"])
	}

	// Verify the queue item was updated to "rejected"
	var updated db.ApprovalQueueItem
	if err := database.First(&updated, itemID).Error; err != nil {
		t.Fatalf("Failed to find updated queue item: %v", err)
	}
	if updated.Status != "rejected" {
		t.Errorf("Expected status 'rejected', got %q", updated.Status)
	}
}

func TestRejectEntry_NotFound(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	req := testutil.AuthenticatedRequest(t, http.MethodPost, "/api/approval-queue/99999/reject", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("Expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

// --- /audit/recent tests ---

func TestGetRecentAuditLogs_Empty(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	req := testutil.AuthenticatedRequest(t, http.MethodGet, "/api/audit-log/recent", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var logs []db.AuditLogEntry
	if err := json.Unmarshal(rec.Body.Bytes(), &logs); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	if len(logs) != 0 {
		t.Errorf("Expected 0 items for empty state, got %d", len(logs))
	}
}

func TestGetRecentAuditLogs_DefaultLimit(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	seedAuditLogs(t, database, 10)

	req := testutil.AuthenticatedRequest(t, http.MethodGet, "/api/audit-log/recent", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var logs []db.AuditLogEntry
	if err := json.Unmarshal(rec.Body.Bytes(), &logs); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	if len(logs) != 5 {
		t.Errorf("Expected default limit of 5 items, got %d", len(logs))
	}
}

func TestGetRecentAuditLogs_CustomLimit(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	seedAuditLogs(t, database, 10)

	req := testutil.AuthenticatedRequest(t, http.MethodGet, "/api/audit-log/recent?limit=3", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var logs []db.AuditLogEntry
	if err := json.Unmarshal(rec.Body.Bytes(), &logs); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	if len(logs) != 3 {
		t.Errorf("Expected 3 items with limit=3, got %d", len(logs))
	}
}

func TestGetRecentAuditLogs_LimitCapped(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	seedAuditLogs(t, database, 10)

	req := testutil.AuthenticatedRequest(t, http.MethodGet, "/api/audit-log/recent?limit=100", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var logs []db.AuditLogEntry
	if err := json.Unmarshal(rec.Body.Bytes(), &logs); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	// Should return all 10 (capped at 50, but we only have 10)
	if len(logs) != 10 {
		t.Errorf("Expected 10 items (all available), got %d", len(logs))
	}
}

func TestGetRecentAuditLogs_OrderedByNewest(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	seedAuditLogs(t, database, 5)

	req := testutil.AuthenticatedRequest(t, http.MethodGet, "/api/audit-log/recent", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var logs []db.AuditLogEntry
	if err := json.Unmarshal(rec.Body.Bytes(), &logs); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// Verify entries are ordered newest first
	for i := 1; i < len(logs); i++ {
		if logs[i].CreatedAt.After(logs[i-1].CreatedAt) {
			t.Errorf("Entries not in descending order at index %d: %v > %v",
				i, logs[i].CreatedAt, logs[i-1].CreatedAt)
		}
	}
}
