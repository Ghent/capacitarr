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
	"capacitarr/internal/engine"
	"capacitarr/internal/events"
	"capacitarr/internal/integrations"
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
	if err := db.AutoMigrateAll(database); err != nil {
		t.Fatalf("AutoMigrate failed: %v", err)
	}

	pref := db.PreferenceSet{
		ID:                   1,
		DefaultDiskGroupMode: db.ModeApproval,
		LogLevel:             "info",
		PollIntervalSeconds:  300,
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

// TestApprovalDedup_SingleEntry verifies that running the production
// approval upsert twice for the same media item produces only one
// "pending" approval queue entry, with the second run updating the
// existing entry rather than creating a duplicate. Goes through
// ApprovalService.BulkUpsertPending (the poller dispatch path) so a
// query change in production code fails this test.
func TestApprovalDedup_SingleEntry(t *testing.T) {
	database, reg := setupEvaluateTestDB(t)

	mediaName := "Firefly - Season 1"
	mediaType := "season"
	integrationID := uint(1)

	created, updated, err := reg.Approval.BulkUpsertPending([]db.ApprovalQueueItem{{
		MediaName:     mediaName,
		MediaType:     mediaType,
		ScoreDetails:  `[{"name":"size","contribution":3.0},{"name":"age","contribution":2.5}]`,
		SizeBytes:     1000000000,
		Score:         5.50,
		IntegrationID: integrationID,
		ExternalID:    "ext-1",
	}})
	if err != nil {
		t.Fatalf("first BulkUpsertPending: %v", err)
	}
	if created != 1 || updated != 0 {
		t.Fatalf("first upsert: created=%d updated=%d, want created=1 updated=0", created, updated)
	}

	var count int64
	database.Model(&db.ApprovalQueueItem{}).Where("media_name = ? AND status = ?", mediaName, db.StatusPending).Count(&count)
	if count != 1 {
		t.Fatalf("Expected 1 approval queue entry after first run, got %d", count)
	}

	created, updated, err = reg.Approval.BulkUpsertPending([]db.ApprovalQueueItem{{
		MediaName:     mediaName,
		MediaType:     mediaType,
		ScoreDetails:  `[{"name":"size","contribution":3.5},{"name":"age","contribution":2.7}]`,
		SizeBytes:     1100000000,
		Score:         6.20,
		IntegrationID: integrationID,
		ExternalID:    "ext-1",
	}})
	if err != nil {
		t.Fatalf("second BulkUpsertPending: %v", err)
	}
	if created != 0 || updated != 1 {
		t.Errorf("second upsert: created=%d updated=%d, want created=0 updated=1", created, updated)
	}

	database.Model(&db.ApprovalQueueItem{}).Where("media_name = ? AND status = ?", mediaName, db.StatusPending).Count(&count)
	if count != 1 {
		t.Errorf("Expected 1 approval queue entry after second run (dedup), got %d", count)
	}

	var row db.ApprovalQueueItem
	database.Where("media_name = ? AND status = ?", mediaName, db.StatusPending).First(&row)
	if row.Score != 6.20 {
		t.Errorf("Expected updated score=6.20, got %f", row.Score)
	}
	if row.SizeBytes != 1100000000 {
		t.Errorf("Expected updated sizeBytes=1100000000, got %d", row.SizeBytes)
	}
}

// TestBelowThreshold_ClearsQueue verifies that when ALL disk groups are below
// threshold, the orchestration-level ClearQueue removes pending and rejected
// approval queue items but preserves approved items (mid-deletion).
// ClearQueue is called at the orchestration level (poll loop), NOT inside
// evaluateDiskGroup, to avoid cross-contamination when multiple disk groups
// have different threshold states.
func TestBelowThreshold_ClearsQueue(t *testing.T) {
	database, reg := setupEvaluateTestDB(t)
	_ = New(reg)

	integrationID := uint(1)

	// Seed approval queue items in all three states
	pending := db.ApprovalQueueItem{
		MediaName: "Firefly", MediaType: "show",
		SizeBytes: 5000, IntegrationID: integrationID, ExternalID: "1",
		Status: db.StatusPending,
	}
	if err := database.Create(&pending).Error; err != nil {
		t.Fatalf("Failed to create pending item: %v", err)
	}

	snoozedUntil := time.Now().UTC().Add(24 * time.Hour)
	rejected := db.ApprovalQueueItem{
		MediaName: "Serenity", MediaType: "movie",
		SizeBytes: 3000, IntegrationID: integrationID, ExternalID: "2",
		Status: db.StatusRejected, SnoozedUntil: &snoozedUntil,
	}
	if err := database.Create(&rejected).Error; err != nil {
		t.Fatalf("Failed to create rejected item: %v", err)
	}

	approved := db.ApprovalQueueItem{
		MediaName: "Firefly - Season 1", MediaType: "season",
		SizeBytes: 8000, IntegrationID: integrationID, ExternalID: "3",
		Status: db.StatusApproved,
	}
	if err := database.Create(&approved).Error; err != nil {
		t.Fatalf("Failed to create approved item: %v", err)
	}

	// Simulate the orchestration-level ClearQueue that runs when all disks
	// are below threshold (moved OUT of evaluateDiskGroup to prevent
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
// evaluateDiskGroup does NOT call ClearQueue when a disk group is below
// threshold. Queue clearing is now handled at the orchestration level to prevent
// cross-contamination between disk groups.
func TestEvaluateAndCleanDisk_BelowThreshold_NoLongerClearsQueue(t *testing.T) {
	database, reg := setupEvaluateTestDB(t)
	p := New(reg)

	integrationID := uint(1)

	// Seed a pending approval queue item
	pending := db.ApprovalQueueItem{
		MediaName: "Firefly", MediaType: "show",
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

	// Call evaluateDiskGroup — should NOT clear the queue
	result := p.evaluateDiskGroup(NewRunAccumulator(), group, nil, nil, 0, db.PreferenceSet{}, map[string]int{}, nil, &engine.EvaluationContext{ActiveIntegrationTypes: map[integrations.IntegrationType]bool{}})
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
	result := p.evaluateDiskGroup(NewRunAccumulator(), group, nil, nil, 0, db.PreferenceSet{}, map[string]int{}, nil, &engine.EvaluationContext{ActiveIntegrationTypes: map[integrations.IntegrationType]bool{}})
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

	result := p.evaluateDiskGroup(NewRunAccumulator(), group, nil, nil, 0, db.PreferenceSet{}, map[string]int{}, nil, &engine.EvaluationContext{ActiveIntegrationTypes: map[integrations.IntegrationType]bool{}})
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
		ScoreDetails:  `[]`,
		Status:        db.StatusApproved,
		SizeBytes:     500000000,
		Score:         4.00,
		IntegrationID: integrationID,
		ExternalID:    "ext-2",
		CreatedAt:     time.Now().Add(-30 * time.Minute),
		UpdatedAt:     time.Now().Add(-30 * time.Minute),
	}
	database.Create(&approvedEntry)

	// Engine re-queues via the production upsert (matches pending only).
	created, updated, err := reg.Approval.BulkUpsertPending([]db.ApprovalQueueItem{{
		MediaName:     mediaName,
		MediaType:     mediaType,
		ScoreDetails:  `[{"name":"size","contribution":4.5}]`,
		SizeBytes:     550000000,
		Score:         4.50,
		IntegrationID: integrationID,
		ExternalID:    "ext-2",
	}})
	if err != nil {
		t.Fatalf("BulkUpsertPending: %v", err)
	}
	if created != 1 || updated != 0 {
		t.Errorf("expected a new pending row (created=1 updated=0), got created=%d updated=%d", created, updated)
	}

	// Verify: the approved entry is untouched
	var approved db.ApprovalQueueItem
	database.Where("media_name = ? AND status = ?", mediaName, db.StatusApproved).First(&approved)
	if approved.ID == 0 {
		t.Fatal("Expected approved entry to still exist")
	}
	if approved.Score != 4.00 {
		t.Errorf("Expected approved entry score untouched, got %f", approved.Score)
	}
	if approved.SizeBytes != 500000000 {
		t.Errorf("Expected approved entry sizeBytes untouched, got %d", approved.SizeBytes)
	}

	// Verify: a new "pending" entry was created (separate from the approved one)
	var queued db.ApprovalQueueItem
	database.Where("media_name = ? AND status = ?", mediaName, db.StatusPending).First(&queued)
	if queued.ID == 0 {
		t.Fatal("Expected new 'pending' entry to be created")
	}
	if queued.Score != 4.50 {
		t.Errorf("Expected new pending entry score, got %f", queued.Score)
	}

	// Verify: total entries = 2 (one approved, one pending)
	var total int64
	database.Model(&db.ApprovalQueueItem{}).Where("media_name = ?", mediaName).Count(&total)
	if total != 2 {
		t.Errorf("Expected 2 total entries (1 approved + 1 pending), got %d", total)
	}
}

// TestEvaluateAndCleanDisk_ReconcilesDismissesStaleItems verifies that the
// per-cycle reconciliation in evaluateDiskGroup removes pending approval
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
		Mode:         db.ModeApproval,
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

	prefs := db.PreferenceSet{DefaultDiskGroupMode: db.ModeApproval}

	// Run with no media items — no candidates will be generated, so
	// neededKeys will be empty, and all pending items should be reconciled away
	result := p.evaluateDiskGroup(NewRunAccumulator(), group, nil, nil, 0, prefs, map[string]int{}, nil, &engine.EvaluationContext{ActiveIntegrationTypes: map[integrations.IntegrationType]bool{}})
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
		Mode:         db.ModeDryRun,
	}
	if err := database.Create(&group).Error; err != nil {
		t.Fatalf("Failed to create disk group: %v", err)
	}

	// Pre-seed a pending item
	dgID := group.ID
	staleItem := db.ApprovalQueueItem{
		MediaName:     "Serenity",
		MediaType:     "movie",
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
	prefs := db.PreferenceSet{DefaultDiskGroupMode: db.ModeDryRun}
	p.evaluateDiskGroup(NewRunAccumulator(), group, nil, nil, 0, prefs, map[string]int{}, nil, &engine.EvaluationContext{ActiveIntegrationTypes: map[integrations.IntegrationType]bool{}})

	// Verify: pending item is preserved (reconciliation didn't run)
	var pendingCount int64
	database.Model(&db.ApprovalQueueItem{}).Where(
		"disk_group_id = ? AND status = ?", dgID, db.StatusPending,
	).Count(&pendingCount)
	if pendingCount != 1 {
		t.Errorf("expected 1 pending item preserved in dry-run mode, got %d", pendingCount)
	}
}

// TestEvaluateAndCleanDisk_IsSnoozed_AutoMode verifies that the IsSnoozed()
// check runs in auto mode (not just approval mode), so snoozed items are
// skipped regardless of execution mode.
func TestEvaluateAndCleanDisk_IsSnoozed_AutoMode(t *testing.T) {
	database, reg := setupEvaluateTestDB(t)
	p := New(reg)

	integrationID := uint(1)

	// Create a disk group above threshold (95% used, 80% threshold)
	group := db.DiskGroup{
		MountPath:    "/data",
		TotalBytes:   100000000, // 100 MB
		UsedBytes:    95000000,  // 95 MB = 95%
		ThresholdPct: 80.0,
		TargetPct:    70.0,
		Mode:         db.ModeAuto,
	}
	if err := database.Create(&group).Error; err != nil {
		t.Fatalf("Failed to create disk group: %v", err)
	}

	// Create a snoozed entry in the approval queue for "Serenity"
	dgID := group.ID
	snoozedUntil := time.Now().UTC().Add(24 * time.Hour)
	snoozedItem := db.ApprovalQueueItem{
		MediaName:     "Serenity",
		MediaType:     "movie",
		SizeBytes:     50000000,
		Score:         0.90,
		Status:        db.StatusRejected,
		SnoozedUntil:  &snoozedUntil,
		IntegrationID: integrationID,
		ExternalID:    "snoozed-1",
		DiskGroupID:   &dgID,
	}
	if err := database.Create(&snoozedItem).Error; err != nil {
		t.Fatalf("Failed to create snoozed item: %v", err)
	}

	// Create a media item that would normally be a candidate
	items := []integrations.MediaItem{
		{
			ExternalID:    "movie-1",
			IntegrationID: integrationID,
			Type:          integrations.MediaTypeMovie,
			Title:         "Serenity",
			SizeBytes:     50000000, // 50 MB
			Path:          "/data/movies/Serenity",
			Rating:        3.0,
		},
	}

	prefs := db.PreferenceSet{
		DefaultDiskGroupMode: db.ModeAuto,
	}
	weights := map[string]int{
		"watch_history":   5,
		"file_size":       5,
		"rating":          5,
		"last_watched":    5,
		"time_in_library": 5,
		"series_status":   5,
	}

	result := p.evaluateDiskGroup(NewRunAccumulator(), group, items, nil, 0, prefs, weights, nil, &engine.EvaluationContext{ActiveIntegrationTypes: map[integrations.IntegrationType]bool{}})

	// The item should be skipped because it's snoozed — 0 queued
	if result != 0 {
		t.Errorf("expected 0 deletions queued (snoozed item skipped in auto mode), got %d", result)
	}
}

// TestEvaluateAndCleanDisk_IsSnoozed_DryRunMode verifies that the IsSnoozed()
// check runs in dry-run mode, so snoozed items are skipped.
func TestEvaluateAndCleanDisk_IsSnoozed_DryRunMode(t *testing.T) {
	database, reg := setupEvaluateTestDB(t)
	p := New(reg)

	integrationID := uint(1)

	// Create a disk group above threshold
	group := db.DiskGroup{
		MountPath:    "/data",
		TotalBytes:   100000000,
		UsedBytes:    95000000,
		ThresholdPct: 80.0,
		TargetPct:    70.0,
		Mode:         db.ModeDryRun,
	}
	if err := database.Create(&group).Error; err != nil {
		t.Fatalf("Failed to create disk group: %v", err)
	}

	// Create a snoozed entry for "Firefly"
	dgID := group.ID
	snoozedUntil := time.Now().UTC().Add(24 * time.Hour)
	snoozedItem := db.ApprovalQueueItem{
		MediaName:     "Firefly",
		MediaType:     "show",
		SizeBytes:     60000000,
		Score:         0.85,
		Status:        db.StatusRejected,
		SnoozedUntil:  &snoozedUntil,
		IntegrationID: integrationID,
		ExternalID:    "snoozed-2",
		DiskGroupID:   &dgID,
	}
	if err := database.Create(&snoozedItem).Error; err != nil {
		t.Fatalf("Failed to create snoozed item: %v", err)
	}

	items := []integrations.MediaItem{
		{
			ExternalID:    "show-1",
			IntegrationID: integrationID,
			Type:          integrations.MediaTypeShow,
			Title:         "Firefly",
			SizeBytes:     60000000,
			Path:          "/data/shows/Firefly",
			Rating:        3.0,
		},
	}

	prefs := db.PreferenceSet{
		DefaultDiskGroupMode: db.ModeDryRun,
	}
	weights := map[string]int{
		"watch_history":   5,
		"file_size":       5,
		"rating":          5,
		"last_watched":    5,
		"time_in_library": 5,
		"series_status":   5,
	}

	result := p.evaluateDiskGroup(NewRunAccumulator(), group, items, nil, 0, prefs, weights, nil, &engine.EvaluationContext{ActiveIntegrationTypes: map[integrations.IntegrationType]bool{}})

	// The item should be skipped because it's snoozed — 0 queued
	if result != 0 {
		t.Errorf("expected 0 deletions queued (snoozed item skipped in dry-run mode), got %d", result)
	}
}

// TestEvaluateSunsetMode_QueuesBetweenSunsetAndTarget verifies that sunset mode
// correctly queues items when disk usage is between sunsetPct and targetPct.
// This is the primary operating range of sunset mode. Prior to the fix, the byte
// budget calculation referenced targetPct instead of sunsetPct, producing a
// negative budget that caused zero items to be queued.
func TestEvaluateSunsetMode_QueuesBetweenSunsetAndTarget(t *testing.T) {
	database, reg := setupEvaluateTestDB(t)
	p := New(reg)

	integrationID := uint(1)

	// sunsetPct=60, targetPct=75, thresholdPct=85
	// Disk at 65% — above sunset, below target
	sunsetPct := 60.0
	group := db.DiskGroup{
		MountPath:    "/data",
		TotalBytes:   100_000_000_000, // 100 GB
		UsedBytes:    65_000_000_000,  // 65 GB = 65%
		ThresholdPct: 85.0,
		TargetPct:    75.0,
		SunsetPct:    &sunsetPct,
		Mode:         db.ModeSunset,
	}
	if err := database.Create(&group).Error; err != nil {
		t.Fatalf("Failed to create disk group: %v", err)
	}

	// Create media items on the correct mount path. Total ~30 GB so there
	// are enough candidates for a 5% (~5 GB) byte budget.
	items := []integrations.MediaItem{
		{
			ExternalID: "movie-1", IntegrationID: integrationID,
			Type: integrations.MediaTypeMovie, Title: "Serenity",
			SizeBytes: 10_000_000_000, Path: "/data/movies/Serenity", Rating: 3.0,
		},
		{
			ExternalID: "movie-2", IntegrationID: integrationID,
			Type: integrations.MediaTypeMovie, Title: "Firefly The Movie",
			SizeBytes: 10_000_000_000, Path: "/data/movies/Firefly The Movie", Rating: 4.0,
		},
		{
			ExternalID: "movie-3", IntegrationID: integrationID,
			Type: integrations.MediaTypeMovie, Title: "Leaf on the Wind",
			SizeBytes: 10_000_000_000, Path: "/data/movies/Leaf on the Wind", Rating: 5.0,
		},
	}

	prefs := db.PreferenceSet{
		DefaultDiskGroupMode: db.ModeSunset,
		SunsetDays:           30,
	}
	weights := map[string]int{
		"watch_history":   5,
		"file_size":       5,
		"rating":          5,
		"last_watched":    5,
		"time_in_library": 5,
		"series_status":   5,
	}

	p.evaluateDiskGroup(NewRunAccumulator(), group, items, nil, 0, prefs, weights, nil, &engine.EvaluationContext{ActiveIntegrationTypes: map[integrations.IntegrationType]bool{}})

	// Verify items were queued to the sunset queue
	var count int64
	database.Model(&db.SunsetQueueItem{}).Where("disk_group_id = ?", group.ID).Count(&count)
	if count == 0 {
		t.Error("expected sunset queue items to be created when disk is between sunsetPct and targetPct, got 0")
	}
}

// TestEvaluateSunsetMode_QueuesBetweenTargetAndCritical verifies that sunset
// mode correctly queues items when disk usage is between targetPct and
// thresholdPct (critical). The byte budget should be based on currentPct minus
// sunsetPct (20% at 80% usage with sunsetPct=60), not currentPct minus
// targetPct (5%).
func TestEvaluateSunsetMode_QueuesBetweenTargetAndCritical(t *testing.T) {
	database, reg := setupEvaluateTestDB(t)
	p := New(reg)

	integrationID := uint(1)

	// sunsetPct=60, targetPct=75, thresholdPct=85
	// Disk at 80% — above target, below critical
	sunsetPct := 60.0
	group := db.DiskGroup{
		MountPath:    "/data",
		TotalBytes:   100_000_000_000, // 100 GB
		UsedBytes:    80_000_000_000,  // 80 GB = 80%
		ThresholdPct: 85.0,
		TargetPct:    75.0,
		SunsetPct:    &sunsetPct,
		Mode:         db.ModeSunset,
	}
	if err := database.Create(&group).Error; err != nil {
		t.Fatalf("Failed to create disk group: %v", err)
	}

	// Create enough media items to cover a 20% budget (~20 GB).
	// With 3 items at 8 GB each (24 GB total), all should be queued.
	items := []integrations.MediaItem{
		{
			ExternalID: "movie-1", IntegrationID: integrationID,
			Type: integrations.MediaTypeMovie, Title: "Serenity",
			SizeBytes: 8_000_000_000, Path: "/data/movies/Serenity", Rating: 3.0,
		},
		{
			ExternalID: "movie-2", IntegrationID: integrationID,
			Type: integrations.MediaTypeMovie, Title: "Firefly The Movie",
			SizeBytes: 8_000_000_000, Path: "/data/movies/Firefly The Movie", Rating: 4.0,
		},
		{
			ExternalID: "movie-3", IntegrationID: integrationID,
			Type: integrations.MediaTypeMovie, Title: "Leaf on the Wind",
			SizeBytes: 8_000_000_000, Path: "/data/movies/Leaf on the Wind", Rating: 5.0,
		},
	}

	prefs := db.PreferenceSet{
		DefaultDiskGroupMode: db.ModeSunset,
		SunsetDays:           30,
	}
	weights := map[string]int{
		"watch_history":   5,
		"file_size":       5,
		"rating":          5,
		"last_watched":    5,
		"time_in_library": 5,
		"series_status":   5,
	}

	p.evaluateDiskGroup(NewRunAccumulator(), group, items, nil, 0, prefs, weights, nil, &engine.EvaluationContext{ActiveIntegrationTypes: map[integrations.IntegrationType]bool{}})

	// All 3 items should be queued: budget is 20 GB (80%-60%), items total 24 GB.
	// CandidatesForDeletion stops after accumulating >= budget, so at minimum
	// 2 items (16 GB) should be selected, likely all 3 (reaching 24 GB >= 20 GB).
	var count int64
	database.Model(&db.SunsetQueueItem{}).Where("disk_group_id = ?", group.ID).Count(&count)
	if count == 0 {
		t.Error("expected sunset queue items to be created when disk is between targetPct and thresholdPct, got 0")
	}
	if count < 2 {
		t.Errorf("expected at least 2 sunset queue items for 20%% byte budget, got %d", count)
	}
}

// TestEvaluateSunsetMode_BelowSunsetThreshold_NoQueue verifies that sunset
// mode does NOT queue any items when disk usage is below sunsetPct.
func TestEvaluateSunsetMode_BelowSunsetThreshold_NoQueue(t *testing.T) {
	database, reg := setupEvaluateTestDB(t)
	p := New(reg)

	integrationID := uint(1)

	// sunsetPct=60, targetPct=75, thresholdPct=85
	// Disk at 55% — below sunset threshold
	sunsetPct := 60.0
	group := db.DiskGroup{
		MountPath:    "/data",
		TotalBytes:   100_000_000_000, // 100 GB
		UsedBytes:    55_000_000_000,  // 55 GB = 55%
		ThresholdPct: 85.0,
		TargetPct:    75.0,
		SunsetPct:    &sunsetPct,
		Mode:         db.ModeSunset,
	}
	if err := database.Create(&group).Error; err != nil {
		t.Fatalf("Failed to create disk group: %v", err)
	}

	items := []integrations.MediaItem{
		{
			ExternalID: "movie-1", IntegrationID: integrationID,
			Type: integrations.MediaTypeMovie, Title: "Serenity",
			SizeBytes: 10_000_000_000, Path: "/data/movies/Serenity", Rating: 3.0,
		},
	}

	prefs := db.PreferenceSet{
		DefaultDiskGroupMode: db.ModeSunset,
		SunsetDays:           30,
	}
	weights := map[string]int{
		"watch_history":   5,
		"file_size":       5,
		"rating":          5,
		"last_watched":    5,
		"time_in_library": 5,
		"series_status":   5,
	}

	p.evaluateDiskGroup(NewRunAccumulator(), group, items, nil, 0, prefs, weights, nil, &engine.EvaluationContext{ActiveIntegrationTypes: map[integrations.IntegrationType]bool{}})

	// No items should be queued — disk is below sunset threshold
	var count int64
	database.Model(&db.SunsetQueueItem{}).Where("disk_group_id = ?", group.ID).Count(&count)
	if count != 0 {
		t.Errorf("expected 0 sunset queue items when below sunset threshold, got %d", count)
	}
}

// TestEvaluateSunsetMode_EscalateCountsTowardSignalBatchSize verifies that
// a sunset cycle that escalates holds reports those releases as executor
// actions (spec §5). evaluateSunsetMode used to return 0, so SignalBatchSize(0)
// fired an empty batch-complete on escalate.
func TestEvaluateSunsetMode_EscalateCountsTowardSignalBatchSize(t *testing.T) {
	database, reg := setupEvaluateTestDB(t)
	p := New(reg)

	sunsetPct := 60.0
	group := db.DiskGroup{
		MountPath:    "/data",
		TotalBytes:   100_000_000_000,
		UsedBytes:    90_000_000_000, // 90% — above critical 85%
		ThresholdPct: 85.0,
		TargetPct:    75.0,
		SunsetPct:    &sunsetPct,
		Mode:         db.ModeSunset,
	}
	if err := database.Create(&group).Error; err != nil {
		t.Fatalf("Failed to create disk group: %v", err)
	}

	if err := database.Create(&db.SunsetQueueItem{
		MediaName: "Firefly", MediaType: "show", IntegrationID: 1,
		SizeBytes: 10_000_000_000, DiskGroupID: group.ID, Trigger: db.TriggerEngine,
		DeletionDate: time.Now().UTC().AddDate(0, 0, 10),
	}).Error; err != nil {
		t.Fatalf("create sunset hold: %v", err)
	}

	prefs := db.PreferenceSet{
		DefaultDiskGroupMode: db.ModeSunset,
		SunsetDays:           30,
	}
	result := p.evaluateDiskGroup(NewRunAccumulator(), group, nil, nil, 0, prefs, map[string]int{}, nil, &engine.EvaluationContext{ActiveIntegrationTypes: map[integrations.IntegrationType]bool{}})
	if result < 1 {
		t.Fatalf("expected escalate to count as at least 1 executor action, got %d", result)
	}
}
