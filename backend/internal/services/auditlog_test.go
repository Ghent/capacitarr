package services

import (
	"testing"
	"time"

	"capacitarr/internal/db"
)

func TestAuditLogService_Create(t *testing.T) {
	database := setupTestDB(t)
	svc := NewAuditLogService(database)

	entry := db.AuditLogEntry{
		MediaName: "Firefly",
		MediaType: "show",
		Action:    db.ActionDeleted,
		SizeBytes: 5069636198,
		Score:     0.85,
	}

	if err := svc.Create(entry); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	var count int64
	database.Model(&db.AuditLogEntry{}).Count(&count)
	if count != 1 {
		t.Errorf("expected 1 entry, got %d", count)
	}

	var saved db.AuditLogEntry
	database.First(&saved)
	if saved.MediaName != "Firefly" {
		t.Errorf("expected media name 'Firefly', got %q", saved.MediaName)
	}
	if saved.Action != db.ActionDeleted {
		t.Errorf("expected action %q, got %q", db.ActionDeleted, saved.Action)
	}
	if saved.Score != 0.85 {
		t.Errorf("expected score 0.85, got %f", saved.Score)
	}
}

func TestAuditLogService_UpsertDryRun_Create(t *testing.T) {
	database := setupTestDB(t)
	svc := NewAuditLogService(database)

	entry := db.AuditLogEntry{
		MediaName: "Firefly",
		MediaType: "show",
		Action:    db.ActionDryDelete,
		SizeBytes: 3000000000,
		Score:     0.70,
	}

	if err := svc.UpsertDryRun(entry); err != nil {
		t.Fatalf("UpsertDryRun returned error: %v", err)
	}

	var count int64
	database.Model(&db.AuditLogEntry{}).Count(&count)
	if count != 1 {
		t.Errorf("expected 1 entry, got %d", count)
	}

	// Verify score is stored
	var saved db.AuditLogEntry
	database.First(&saved)
	if saved.Score != 0.70 {
		t.Errorf("expected score 0.70, got %f", saved.Score)
	}
}

func TestAuditLogService_UpsertDryRun_Update(t *testing.T) {
	database := setupTestDB(t)
	svc := NewAuditLogService(database)

	// Create initial dry-run entry
	entry := db.AuditLogEntry{
		MediaName: "Firefly",
		MediaType: "show",
		Action:    db.ActionDryDelete,
		SizeBytes: 3000000000,
		Score:     0.70,
	}
	if err := svc.UpsertDryRun(entry); err != nil {
		t.Fatalf("First UpsertDryRun failed: %v", err)
	}

	// Upsert same media with updated score
	entry.Score = 0.85
	entry.SizeBytes = 3500000000
	entry.Score = 0.85
	if err := svc.UpsertDryRun(entry); err != nil {
		t.Fatalf("Second UpsertDryRun failed: %v", err)
	}

	// Should still have only 1 entry
	var count int64
	database.Model(&db.AuditLogEntry{}).Count(&count)
	if count != 1 {
		t.Errorf("expected 1 entry after upsert, got %d", count)
	}

	// Verify updated values
	var saved db.AuditLogEntry
	database.First(&saved)
	if saved.Score != 0.85 {
		t.Errorf("expected updated score 0.85, got %f", saved.Score)
	}
	if saved.SizeBytes != 3500000000 {
		t.Errorf("expected updated size, got %d", saved.SizeBytes)
	}
	if saved.Score != 0.85 {
		t.Errorf("expected updated score 0.85, got %f", saved.Score)
	}
}

func TestAuditLogService_PruneOlderThan(t *testing.T) {
	database := setupTestDB(t)
	svc := NewAuditLogService(database)

	// Create entries: one old, one recent
	now := time.Now().UTC()
	old := db.AuditLogEntry{
		MediaName: "Old Movie", MediaType: "movie",
		Action: db.ActionDeleted, SizeBytes: 1000,
		CreatedAt: now.AddDate(0, 0, -60),
	}
	recent := db.AuditLogEntry{
		MediaName: "Recent Movie", MediaType: "movie",
		Action: db.ActionDeleted, SizeBytes: 2000,
		CreatedAt: now.AddDate(0, 0, -5),
	}

	database.Create(&old)
	database.Create(&recent)

	pruned, err := svc.PruneOlderThan(30)
	if err != nil {
		t.Fatalf("PruneOlderThan returned error: %v", err)
	}
	if pruned != 1 {
		t.Errorf("expected 1 entry pruned, got %d", pruned)
	}

	// Recent entry should remain
	var remaining []db.AuditLogEntry
	database.Find(&remaining)
	if len(remaining) != 1 {
		t.Fatalf("expected 1 remaining entry, got %d", len(remaining))
	}
	if remaining[0].MediaName != "Recent Movie" {
		t.Errorf("expected recent movie to remain, got %q", remaining[0].MediaName)
	}
}

func TestAuditLogService_ListRecent(t *testing.T) {
	database := setupTestDB(t)
	svc := NewAuditLogService(database)

	// Create 3 entries
	for i := 0; i < 3; i++ {
		_ = svc.Create(db.AuditLogEntry{
			MediaName: "Firefly", MediaType: "show",
			Action: db.ActionDeleted, SizeBytes: 1000,
		})
	}

	logs, err := svc.ListRecent(2)
	if err != nil {
		t.Fatalf("ListRecent returned error: %v", err)
	}
	if len(logs) != 2 {
		t.Errorf("expected 2 entries, got %d", len(logs))
	}
}

func TestAuditLogService_ListGrouped(t *testing.T) {
	database := setupTestDB(t)
	svc := NewAuditLogService(database)

	// Create a show season entry and a movie
	_ = svc.Create(db.AuditLogEntry{
		MediaName: "Firefly - Season 1", MediaType: "season",
		Action: db.ActionDeleted, SizeBytes: 5000,
	})
	_ = svc.Create(db.AuditLogEntry{
		MediaName: "Serenity", MediaType: "movie",
		Action: db.ActionDeleted, SizeBytes: 3000,
	})

	result, err := svc.ListGrouped(100)
	if err != nil {
		t.Fatalf("ListGrouped returned error: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("expected 2 results (1 group + 1 single), got %d", len(result))
	}

	// First should be the group
	foundGroup := false
	foundSingle := false
	for _, r := range result {
		if r.Type == "group" && r.Group != nil && r.Group.ShowTitle == "Firefly" {
			foundGroup = true
		}
		if r.Type == "single" && r.Entry != nil && r.Entry.MediaName == "Serenity" {
			foundSingle = true
		}
	}
	if !foundGroup {
		t.Error("expected a group for 'Firefly'")
	}
	if !foundSingle {
		t.Error("expected a single entry for 'Serenity'")
	}
}

func TestAuditLogService_ListPaginated(t *testing.T) {
	database := setupTestDB(t)
	svc := NewAuditLogService(database)

	// Create entries
	for i := 0; i < 5; i++ {
		_ = svc.Create(db.AuditLogEntry{
			MediaName: "Firefly", MediaType: "show",
			Action: db.ActionDeleted, SizeBytes: 1000,
		})
	}

	result, err := svc.ListPaginated(AuditListParams{
		Limit: 3, Offset: 0, SortBy: "created_at", SortDir: "desc",
	})
	if err != nil {
		t.Fatalf("ListPaginated returned error: %v", err)
	}
	if result.Total != 5 {
		t.Errorf("expected total 5, got %d", result.Total)
	}
	if len(result.Data) != 3 {
		t.Errorf("expected 3 entries in data, got %d", len(result.Data))
	}
}

func TestAuditLogService_ListPaginated_Search(t *testing.T) {
	database := setupTestDB(t)
	svc := NewAuditLogService(database)

	_ = svc.Create(db.AuditLogEntry{MediaName: "Firefly", MediaType: "show", Action: db.ActionDeleted})
	_ = svc.Create(db.AuditLogEntry{MediaName: "Serenity", MediaType: "movie", Action: db.ActionDeleted})

	result, err := svc.ListPaginated(AuditListParams{
		Limit: 10, Search: "Serenity",
	})
	if err != nil {
		t.Fatalf("ListPaginated returned error: %v", err)
	}
	if result.Total != 1 {
		t.Errorf("expected total 1 for search, got %d", result.Total)
	}
}

func TestAuditLogService_PruneOlderThan_ZeroKeepsForever(t *testing.T) {
	database := setupTestDB(t)
	svc := NewAuditLogService(database)

	// Create an old entry
	old := db.AuditLogEntry{
		MediaName: "Ancient Movie", MediaType: "movie",
		Action: db.ActionDeleted, SizeBytes: 1000,
		CreatedAt: time.Now().UTC().AddDate(-1, 0, 0),
	}
	database.Create(&old)

	pruned, err := svc.PruneOlderThan(0)
	if err != nil {
		t.Fatalf("PruneOlderThan(0) returned error: %v", err)
	}
	if pruned != 0 {
		t.Errorf("expected 0 entries pruned with retention=0, got %d", pruned)
	}
}

func TestAuditLogService_BulkUpsertDryRun(t *testing.T) {
	database := setupTestDB(t)
	svc := NewAuditLogService(database)

	t.Run("empty slice is a no-op", func(t *testing.T) {
		if err := svc.BulkUpsertDryRun(nil); err != nil {
			t.Fatalf("BulkUpsertDryRun returned error: %v", err)
		}
	})

	t.Run("creates new dry-run entries", func(t *testing.T) {
		entries := []db.AuditLogEntry{
			{MediaName: "Firefly", MediaType: "show", SizeBytes: 5000, Score: 0.9, Trigger: "engine", DryRunReason: "mode"},
			{MediaName: "Serenity", MediaType: "movie", SizeBytes: 3000, Score: 0.7, Trigger: "engine", DryRunReason: "mode"},
		}
		if err := svc.BulkUpsertDryRun(entries); err != nil {
			t.Fatalf("BulkUpsertDryRun returned error: %v", err)
		}

		var count int64
		database.Model(&db.AuditLogEntry{}).Where("action = ?", db.ActionDryDelete).Count(&count)
		if count != 2 {
			t.Errorf("expected 2 dry-run entries in DB, got %d", count)
		}
	})

	t.Run("updates existing dry-run entries on second call", func(t *testing.T) {
		entries := []db.AuditLogEntry{
			{MediaName: "Firefly", MediaType: "show", SizeBytes: 6000, Score: 0.95, Trigger: "engine", DryRunReason: "mode"},
		}
		if err := svc.BulkUpsertDryRun(entries); err != nil {
			t.Fatalf("BulkUpsertDryRun returned error: %v", err)
		}

		// Verify updated values
		var entry db.AuditLogEntry
		database.Where("media_name = ? AND action = ?", "Firefly", db.ActionDryDelete).First(&entry)
		if entry.SizeBytes != 6000 {
			t.Errorf("expected SizeBytes=6000 after update, got %d", entry.SizeBytes)
		}
		if entry.Score != 0.95 {
			t.Errorf("expected Score=0.95 after update, got %f", entry.Score)
		}

		// Total count should still be 2 (not 3)
		var count int64
		database.Model(&db.AuditLogEntry{}).Where("action = ?", db.ActionDryDelete).Count(&count)
		if count != 2 {
			t.Errorf("expected 2 dry-run entries (upsert, not insert), got %d", count)
		}
	})

	t.Run("handles mixed creates and updates", func(t *testing.T) {
		entries := []db.AuditLogEntry{
			{MediaName: "Serenity", MediaType: "movie", SizeBytes: 4000, Score: 0.8, Trigger: "engine", DryRunReason: "mode"},      // update
			{MediaName: "New Movie", MediaType: "movie", SizeBytes: 2000, Score: 0.5, Trigger: "engine", DryRunReason: "disabled"}, // create
		}
		if err := svc.BulkUpsertDryRun(entries); err != nil {
			t.Fatalf("BulkUpsertDryRun returned error: %v", err)
		}

		var count int64
		database.Model(&db.AuditLogEntry{}).Where("action = ?", db.ActionDryDelete).Count(&count)
		if count != 3 {
			t.Errorf("expected 3 dry-run entries, got %d", count)
		}
	})
}
