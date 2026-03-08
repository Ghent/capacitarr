package poller

import (
	"testing"
	"time"

	_ "github.com/ncruces/go-sqlite3/embed" // load the embedded SQLite WASM binary
	"github.com/ncruces/go-sqlite3/gormlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"capacitarr/internal/config"
	"capacitarr/internal/db"
	"capacitarr/internal/events"
	"capacitarr/internal/services"
)

// setupEvaluateTestDB creates an in-memory SQLite database with migrations applied,
// seeds default preferences, and returns the database and a service registry.
func setupEvaluateTestDB(t *testing.T) (*gorm.DB, *services.Registry) {
	t.Helper()

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

	// Single connection for in-memory DB consistency
	sqlDB.SetMaxOpenConns(1)

	if err := db.RunMigrations(sqlDB); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	pref := db.PreferenceSet{
		ID:                  1,
		ExecutionMode:       "approval",
		LogLevel:            "info",
		PollIntervalSeconds: 300,
	}
	if err := database.FirstOrCreate(&pref, db.PreferenceSet{ID: 1}).Error; err != nil {
		t.Fatalf("Failed to seed preferences: %v", err)
	}

	// Seed a default integration (required for approval_queue FK constraint)
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

	bus := events.NewEventBus()
	t.Cleanup(func() { bus.Close() })
	cfg := &config.Config{JWTSecret: "test"}
	reg := services.NewRegistry(database, bus, cfg)

	return database, reg
}

// TestApprovalDedup_SingleEntry verifies that running the approval dedup logic
// twice for the same media item produces only one "pending" approval queue
// entry, with the second run updating the existing entry rather than creating
// a duplicate.
func TestApprovalDedup_SingleEntry(t *testing.T) {
	database, reg := setupEvaluateTestDB(t)

	mediaName := "Firefly - Season 1"
	mediaType := "season"
	integrationID := uint(1)

	// Simulate first engine run: create initial entry
	firstEntry := db.ApprovalQueueItem{
		MediaName:     mediaName,
		MediaType:     mediaType,
		Reason:        "Score: 5.50 (high score)",
		ScoreDetails:  `[{"name":"size","contribution":3.0},{"name":"age","contribution":2.5}]`,
		Status:        "pending",
		SizeBytes:     1000000000,
		IntegrationID: integrationID,
		ExternalID:    "ext-1",
		CreatedAt:     time.Now().Add(-1 * time.Hour),
		UpdatedAt:     time.Now().Add(-1 * time.Hour),
	}

	// Run the dedup logic (mirrors evaluate.go approval dedup path)
	var existing db.ApprovalQueueItem
	result := reg.DB.Where(
		"media_name = ? AND media_type = ? AND status = ?",
		mediaName, mediaType, "pending",
	).First(&existing)
	if result.Error == nil {
		reg.DB.Model(&existing).Updates(map[string]any{
			"reason":         firstEntry.Reason,
			"score_details":  firstEntry.ScoreDetails,
			"size_bytes":     firstEntry.SizeBytes,
			"integration_id": firstEntry.IntegrationID,
			"external_id":    firstEntry.ExternalID,
		})
	} else {
		reg.DB.Create(&firstEntry)
	}

	// Verify: one entry exists
	var count int64
	database.Model(&db.ApprovalQueueItem{}).Where("media_name = ? AND status = ?", mediaName, "pending").Count(&count)
	if count != 1 {
		t.Fatalf("Expected 1 approval queue entry after first run, got %d", count)
	}

	// Simulate second engine run: updated score and size
	secondEntry := db.ApprovalQueueItem{
		MediaName:     mediaName,
		MediaType:     mediaType,
		Reason:        "Score: 6.20 (higher score)",
		ScoreDetails:  `[{"name":"size","contribution":3.5},{"name":"age","contribution":2.7}]`,
		Status:        "pending",
		SizeBytes:     1100000000,
		IntegrationID: integrationID,
		ExternalID:    "ext-1",
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	// Run the dedup logic again (should update, not create)
	var existing2 db.ApprovalQueueItem
	result2 := reg.DB.Where(
		"media_name = ? AND media_type = ? AND status = ?",
		mediaName, mediaType, "pending",
	).First(&existing2)
	if result2.Error == nil {
		reg.DB.Model(&existing2).Updates(map[string]any{
			"reason":         secondEntry.Reason,
			"score_details":  secondEntry.ScoreDetails,
			"size_bytes":     secondEntry.SizeBytes,
			"integration_id": secondEntry.IntegrationID,
			"external_id":    secondEntry.ExternalID,
		})
	} else {
		reg.DB.Create(&secondEntry)
	}

	// Verify: still only one entry
	database.Model(&db.ApprovalQueueItem{}).Where("media_name = ? AND status = ?", mediaName, "pending").Count(&count)
	if count != 1 {
		t.Errorf("Expected 1 approval queue entry after second run (dedup), got %d", count)
	}

	// Verify: the entry was updated with the new values
	var updated db.ApprovalQueueItem
	database.Where("media_name = ? AND status = ?", mediaName, "pending").First(&updated)
	if updated.Reason != "Score: 6.20 (higher score)" {
		t.Errorf("Expected updated reason, got %q", updated.Reason)
	}
	if updated.SizeBytes != 1100000000 {
		t.Errorf("Expected updated sizeBytes=1100000000, got %d", updated.SizeBytes)
	}
}

// TestEvaluateAndCleanDisk_BelowThreshold_ClearsQueue verifies that when disk
// usage is below the threshold, evaluateAndCleanDisk clears pending and rejected
// approval queue items but preserves approved items (mid-deletion).
func TestEvaluateAndCleanDisk_BelowThreshold_ClearsQueue(t *testing.T) {
	database, reg := setupEvaluateTestDB(t)
	p := New(reg)

	integrationID := uint(1)

	// Seed approval queue items in all three states
	pending := db.ApprovalQueueItem{
		MediaName: "Firefly", MediaType: "show", Reason: "Score: 0.85",
		SizeBytes: 5000, IntegrationID: integrationID, ExternalID: "1",
		Status: db.StatusPending,
	}
	if err := database.Create(&pending).Error; err != nil {
		t.Fatalf("Failed to create pending item: %v", err)
	}

	snoozedUntil := time.Now().UTC().Add(24 * time.Hour)
	rejected := db.ApprovalQueueItem{
		MediaName: "Serenity", MediaType: "movie", Reason: "Score: 0.70",
		SizeBytes: 3000, IntegrationID: integrationID, ExternalID: "2",
		Status: db.StatusRejected, SnoozedUntil: &snoozedUntil,
	}
	if err := database.Create(&rejected).Error; err != nil {
		t.Fatalf("Failed to create rejected item: %v", err)
	}

	approved := db.ApprovalQueueItem{
		MediaName: "Firefly - Season 1", MediaType: "season", Reason: "Score: 0.90",
		SizeBytes: 8000, IntegrationID: integrationID, ExternalID: "3",
		Status: db.StatusApproved,
	}
	if err := database.Create(&approved).Error; err != nil {
		t.Fatalf("Failed to create approved item: %v", err)
	}

	// Create a disk group that is BELOW threshold (50% used, 80% threshold)
	group := db.DiskGroup{
		MountPath:    "/data",
		TotalBytes:   100000,
		UsedBytes:    50000,
		ThresholdPct: 80.0,
		TargetPct:    70.0,
	}

	// Call evaluateAndCleanDisk — should trigger ClearQueue since below threshold
	result := p.evaluateAndCleanDisk(group, nil, nil, 0)
	if result != 0 {
		t.Errorf("expected 0 deletions queued, got %d", result)
	}

	// Verify: pending and rejected items are deleted
	var remaining []db.ApprovalQueueItem
	database.Find(&remaining)
	if len(remaining) != 1 {
		t.Fatalf("expected 1 remaining item (approved only), got %d", len(remaining))
	}
	if remaining[0].Status != db.StatusApproved {
		t.Errorf("expected remaining item to be approved, got %q", remaining[0].Status)
	}
	if remaining[0].MediaName != "Firefly - Season 1" {
		t.Errorf("expected remaining item to be 'Firefly - Season 1', got %q", remaining[0].MediaName)
	}
}

// TestApprovalDedup_DoesNotTouchApproved verifies that the dedup logic does
// NOT overwrite entries whose status has been changed to "approved" by the user.
func TestApprovalDedup_DoesNotTouchApproved(t *testing.T) {
	database, reg := setupEvaluateTestDB(t)

	mediaName := "Firefly - Season 1"
	mediaType := "season"
	integrationID := uint(1)

	// Create an entry that was approved by the user
	approvedEntry := db.ApprovalQueueItem{
		MediaName:     mediaName,
		MediaType:     mediaType,
		Reason:        "Score: 4.00 (approved)",
		ScoreDetails:  `[]`,
		Status:        "approved",
		SizeBytes:     500000000,
		IntegrationID: integrationID,
		ExternalID:    "ext-2",
		CreatedAt:     time.Now().Add(-30 * time.Minute),
		UpdatedAt:     time.Now().Add(-30 * time.Minute),
	}
	database.Create(&approvedEntry)

	// Now simulate the engine trying to re-queue this item for approval
	newEntry := db.ApprovalQueueItem{
		MediaName:     mediaName,
		MediaType:     mediaType,
		Reason:        "Score: 4.50 (re-evaluated)",
		ScoreDetails:  `[{"name":"size","contribution":4.5}]`,
		Status:        "pending",
		SizeBytes:     550000000,
		IntegrationID: integrationID,
		ExternalID:    "ext-2",
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	// Run the approval dedup logic (WHERE status = "pending")
	var existing db.ApprovalQueueItem
	result := reg.DB.Where(
		"media_name = ? AND media_type = ? AND status = ?",
		mediaName, mediaType, "pending",
	).First(&existing)
	if result.Error == nil {
		reg.DB.Model(&existing).Updates(map[string]any{
			"reason":         newEntry.Reason,
			"score_details":  newEntry.ScoreDetails,
			"size_bytes":     newEntry.SizeBytes,
			"integration_id": newEntry.IntegrationID,
			"external_id":    newEntry.ExternalID,
		})
	} else {
		// No existing "pending" entry found — create a new one
		reg.DB.Create(&newEntry)
	}

	// Verify: the approved entry is untouched
	var approved db.ApprovalQueueItem
	database.Where("media_name = ? AND status = ?", mediaName, "approved").First(&approved)
	if approved.ID == 0 {
		t.Fatal("Expected approved entry to still exist")
	}
	if approved.Reason != "Score: 4.00 (approved)" {
		t.Errorf("Expected approved entry reason untouched, got %q", approved.Reason)
	}
	if approved.SizeBytes != 500000000 {
		t.Errorf("Expected approved entry sizeBytes untouched, got %d", approved.SizeBytes)
	}

	// Verify: a new "pending" entry was created (separate from the approved one)
	var queued db.ApprovalQueueItem
	database.Where("media_name = ? AND status = ?", mediaName, "pending").First(&queued)
	if queued.ID == 0 {
		t.Fatal("Expected new 'pending' entry to be created")
	}
	if queued.Reason != "Score: 4.50 (re-evaluated)" {
		t.Errorf("Expected new pending entry reason, got %q", queued.Reason)
	}

	// Verify: total entries = 2 (one approved, one pending)
	var total int64
	database.Model(&db.ApprovalQueueItem{}).Where("media_name = ?", mediaName).Count(&total)
	if total != 2 {
		t.Errorf("Expected 2 total entries (1 approved + 1 pending), got %d", total)
	}
}
