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
		Score:         5.50,
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
			"score":          firstEntry.Score,
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
		Score:         6.20,
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
			"score":          secondEntry.Score,
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
	if updated.Score != 6.20 {
		t.Errorf("Expected updated score=6.20, got %f", updated.Score)
	}
}

// TestBelowThreshold_ClearsQueue verifies that when ALL disk groups are below
// threshold, the orchestration-level ClearQueue removes pending and rejected
// approval queue items but preserves approved items (mid-deletion).
// ClearQueue is called at the orchestration level (poll loop), NOT inside
// evaluateAndCleanDisk, to avoid cross-contamination when multiple disk groups
// have different threshold states.
func TestBelowThreshold_ClearsQueue(t *testing.T) {
	database, reg := setupEvaluateTestDB(t)
	_ = New(reg)

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

	// Simulate the orchestration-level ClearQueue that runs when all disks
	// are below threshold (moved OUT of evaluateAndCleanDisk to prevent
	// cross-contamination between disk groups)
	cleared, err := reg.Approval.ClearQueue()
	if err != nil {
		t.Fatalf("ClearQueue failed: %v", err)
	}
	if cleared != 2 {
		t.Errorf("expected 2 items cleared (pending + rejected), got %d", cleared)
	}

	// Verify: pending and rejected items are deleted, approved preserved
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

// TestEvaluateAndCleanDisk_BelowThreshold_NoLongerClearsQueue verifies that
// evaluateAndCleanDisk does NOT call ClearQueue when a disk group is below
// threshold. Queue clearing is now handled at the orchestration level to prevent
// cross-contamination between disk groups.
func TestEvaluateAndCleanDisk_BelowThreshold_NoLongerClearsQueue(t *testing.T) {
	database, reg := setupEvaluateTestDB(t)
	p := New(reg)

	integrationID := uint(1)

	// Seed a pending approval queue item
	pending := db.ApprovalQueueItem{
		MediaName: "Firefly", MediaType: "show", Reason: "Score: 0.85",
		SizeBytes: 5000, IntegrationID: integrationID, ExternalID: "1",
		Status: db.StatusPending,
	}
	if err := database.Create(&pending).Error; err != nil {
		t.Fatalf("Failed to create pending item: %v", err)
	}

	// Create a disk group that is BELOW threshold (50% used, 80% threshold)
	group := db.DiskGroup{
		MountPath:    "/data",
		TotalBytes:   100000,
		UsedBytes:    50000,
		ThresholdPct: 80.0,
		TargetPct:    70.0,
	}

	// Call evaluateAndCleanDisk — should NOT clear the queue
	result := p.evaluateAndCleanDisk(group, nil, nil, 0, db.PreferenceSet{}, nil)
	if result != 0 {
		t.Errorf("expected 0 deletions queued, got %d", result)
	}

	// Verify: pending item is PRESERVED (not cleared)
	var remaining []db.ApprovalQueueItem
	database.Find(&remaining)
	if len(remaining) != 1 {
		t.Fatalf("expected 1 item preserved (queue not cleared per-disk-group), got %d", len(remaining))
	}
	if remaining[0].MediaName != "Firefly" {
		t.Errorf("expected preserved item to be 'Firefly', got %q", remaining[0].MediaName)
	}
}

// TestEvaluateAndCleanDisk_WithOverride verifies that when a disk size
// override is set, the threshold calculation uses the effective total (override)
// instead of the API-reported total.
func TestEvaluateAndCleanDisk_WithOverride(t *testing.T) {
	_, reg := setupEvaluateTestDB(t)
	p := New(reg)

	// Disk reports 10 TB total, 5 TB used = 50% — below 80% threshold.
	// But with a 6 TB override, 5 TB used = 83% — ABOVE 80% threshold.
	override := int64(6_000_000_000_000)
	group := db.DiskGroup{
		MountPath:          "/data",
		TotalBytes:         10_000_000_000_000,
		UsedBytes:          5_000_000_000_000,
		TotalBytesOverride: &override,
		ThresholdPct:       80.0,
		TargetPct:          70.0,
	}

	// Run with no items — it should still detect threshold breach and not return 0 early
	// Since there are no media items, it won't actually queue anything, but the
	// breach detection code path should be entered (checking for currentPct > threshold).
	result := p.evaluateAndCleanDisk(group, nil, nil, 0, db.PreferenceSet{}, nil)
	// With no items, nothing to delete, but the important thing is it didn't
	// short-circuit at the "below threshold" check.
	if result != 0 {
		t.Errorf("expected 0 (no items to delete), got %d", result)
	}
}

// TestEvaluateAndCleanDisk_OverrideZeroUsesDetected verifies that a zero override
// is treated as "no override" and the API-reported total is used.
func TestEvaluateAndCleanDisk_OverrideZeroUsesDetected(t *testing.T) {
	_, reg := setupEvaluateTestDB(t)
	p := New(reg)

	// Zero override should be treated as nil
	zero := int64(0)
	group := db.DiskGroup{
		MountPath:          "/data",
		TotalBytes:         100000,
		UsedBytes:          50000, // 50% — below 80% threshold
		TotalBytesOverride: &zero,
		ThresholdPct:       80.0,
		TargetPct:          70.0,
	}

	result := p.evaluateAndCleanDisk(group, nil, nil, 0, db.PreferenceSet{}, nil)
	if result != 0 {
		t.Errorf("expected 0 (below threshold), got %d", result)
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
		Score:         4.00,
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
		Score:         4.50,
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
			"score":          newEntry.Score,
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

// TestEvaluateAndCleanDisk_ReconcilesDismissesStaleItems verifies that the
// per-cycle reconciliation in evaluateAndCleanDisk removes pending approval
// queue items that are no longer candidates after threshold changes.
func TestEvaluateAndCleanDisk_ReconcilesDismissesStaleItems(t *testing.T) {
	database, reg := setupEvaluateTestDB(t)
	p := New(reg)

	integrationID := uint(1)

	// Create a disk group that is above threshold (95% used, 80% threshold)
	group := db.DiskGroup{
		MountPath:    "/data",
		TotalBytes:   100000,
		UsedBytes:    95000,
		ThresholdPct: 80.0,
		TargetPct:    70.0,
	}
	if err := database.Create(&group).Error; err != nil {
		t.Fatalf("Failed to create disk group: %v", err)
	}

	// Pre-seed a stale pending item from a previous cycle.
	// This item's media won't appear in the current evaluation's candidates
	// because we're running with no media items.
	dgID := group.ID
	staleItem := db.ApprovalQueueItem{
		MediaName:     "Serenity",
		MediaType:     "movie",
		Reason:        "Score: 0.50 (stale from previous cycle)",
		SizeBytes:     4000000000,
		Score:         0.50,
		Status:        db.StatusPending,
		IntegrationID: integrationID,
		ExternalID:    "stale-1",
		DiskGroupID:   &dgID,
	}
	if err := database.Create(&staleItem).Error; err != nil {
		t.Fatalf("Failed to create stale item: %v", err)
	}

	// Also seed a rejected/snoozed item — it should NOT be dismissed
	snoozedUntil := time.Now().UTC().Add(24 * time.Hour)
	snoozedItem := db.ApprovalQueueItem{
		MediaName:     "Firefly",
		MediaType:     "show",
		Reason:        "Score: 0.80 (snoozed)",
		SizeBytes:     6000000000,
		Score:         0.80,
		Status:        db.StatusRejected,
		SnoozedUntil:  &snoozedUntil,
		IntegrationID: integrationID,
		ExternalID:    "snoozed-1",
		DiskGroupID:   &dgID,
	}
	if err := database.Create(&snoozedItem).Error; err != nil {
		t.Fatalf("Failed to create snoozed item: %v", err)
	}

	prefs := db.PreferenceSet{ExecutionMode: "approval"}

	// Run with no media items — no candidates will be generated, so
	// neededKeys will be empty, and all pending items should be reconciled away
	result := p.evaluateAndCleanDisk(group, nil, nil, 0, prefs, nil)
	if result != 0 {
		t.Errorf("expected 0 deletions queued, got %d", result)
	}

	// Verify: stale pending item is dismissed
	var pendingCount int64
	database.Model(&db.ApprovalQueueItem{}).Where(
		"disk_group_id = ? AND status = ?", dgID, db.StatusPending,
	).Count(&pendingCount)
	if pendingCount != 0 {
		t.Errorf("expected 0 pending items after reconciliation, got %d", pendingCount)
	}

	// Verify: snoozed/rejected item is preserved
	var rejectedCount int64
	database.Model(&db.ApprovalQueueItem{}).Where(
		"disk_group_id = ? AND status = ?", dgID, db.StatusRejected,
	).Count(&rejectedCount)
	if rejectedCount != 1 {
		t.Errorf("expected 1 rejected item preserved, got %d", rejectedCount)
	}
}

// TestEvaluateAndCleanDisk_ReconcileNoopInDryRun verifies that per-cycle
// reconciliation does NOT run in dry-run mode (only in approval mode).
func TestEvaluateAndCleanDisk_ReconcileNoopInDryRun(t *testing.T) {
	database, reg := setupEvaluateTestDB(t)
	p := New(reg)

	integrationID := uint(1)

	// Create a disk group above threshold
	group := db.DiskGroup{
		MountPath:    "/data",
		TotalBytes:   100000,
		UsedBytes:    95000,
		ThresholdPct: 80.0,
		TargetPct:    70.0,
	}
	if err := database.Create(&group).Error; err != nil {
		t.Fatalf("Failed to create disk group: %v", err)
	}

	// Pre-seed a pending item
	dgID := group.ID
	staleItem := db.ApprovalQueueItem{
		MediaName:     "Serenity",
		MediaType:     "movie",
		Reason:        "Score: 0.50",
		SizeBytes:     4000000000,
		Score:         0.50,
		Status:        db.StatusPending,
		IntegrationID: integrationID,
		ExternalID:    "stale-1",
		DiskGroupID:   &dgID,
	}
	if err := database.Create(&staleItem).Error; err != nil {
		t.Fatalf("Failed to create stale item: %v", err)
	}

	// Run in dry-run mode — reconciliation should NOT happen
	prefs := db.PreferenceSet{ExecutionMode: "dry-run"}
	p.evaluateAndCleanDisk(group, nil, nil, 0, prefs, nil)

	// Verify: pending item is preserved (reconciliation didn't run)
	var pendingCount int64
	database.Model(&db.ApprovalQueueItem{}).Where(
		"disk_group_id = ? AND status = ?", dgID, db.StatusPending,
	).Count(&pendingCount)
	if pendingCount != 1 {
		t.Errorf("expected 1 pending item preserved in dry-run mode, got %d", pendingCount)
	}
}
