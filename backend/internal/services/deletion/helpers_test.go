package deletion

import (
	"fmt"
	"testing"
	"time"

	_ "github.com/ncruces/go-sqlite3/embed"
	"github.com/ncruces/go-sqlite3/gormlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	gormlogger "gorm.io/gorm/logger"

	"capacitarr/internal/db"
	"capacitarr/internal/events"
)

// setupTestDB creates an in-memory SQLite database with migrations applied.
// Local helper — testutil pulls in routes → services and would cycle.
func setupTestDB(t *testing.T) *gorm.DB {
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
	sqlDB.SetMaxOpenConns(1)

	if err := db.RunMigrations(sqlDB); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}
	if err := db.AutoMigrateAll(database); err != nil {
		t.Fatalf("AutoMigrate failed: %v", err)
	}

	pref := db.PreferenceSet{ID: 1, DefaultDiskGroupMode: db.ModeDryRun, LogLevel: db.LogLevelInfo, AuditLogRetentionDays: 30}
	if err := database.FirstOrCreate(&pref, db.PreferenceSet{ID: 1}).Error; err != nil {
		t.Fatalf("Failed to seed preferences: %v", err)
	}

	return database
}

func newTestBus(t *testing.T) *events.EventBus {
	t.Helper()
	bus := events.NewEventBus()
	t.Cleanup(func() { bus.Close() })
	return bus
}

func seedIntegration(t *testing.T, database *gorm.DB) uint {
	t.Helper()
	ic := db.IntegrationConfig{
		Type:   "sonarr",
		Name:   "Test Sonarr",
		URL:    "http://localhost:8989",
		APIKey: "test-key",
	}
	if err := database.Create(&ic).Error; err != nil {
		t.Fatalf("Failed to seed integration: %v", err)
	}
	return ic.ID
}

func seedPendingItem(t *testing.T, database *gorm.DB, integrationID uint) db.ApprovalQueueItem {
	t.Helper()
	item := db.ApprovalQueueItem{
		MediaName:     "Firefly",
		MediaType:     "show",
		SizeBytes:     5069636198,
		Score:         0.85,
		IntegrationID: integrationID,
		ExternalID:    "1",
		Status:        db.StatusPending,
	}
	if err := database.Create(&item).Error; err != nil {
		t.Fatalf("Failed to seed approval queue item: %v", err)
	}
	return item
}

// AuditLogService is a test-only GORM auditor so deletion tests keep calling
// NewAuditLogService without importing the parent services package (cycle).
// Method bodies match services.AuditLogService for the deletionAuditor surface.
type AuditLogService struct {
	db *gorm.DB
}

func NewAuditLogService(database *gorm.DB) *AuditLogService {
	return &AuditLogService{db: database}
}

func (s *AuditLogService) Create(entry db.AuditLogEntry) error {
	entry.CreatedAt = time.Now().UTC()
	if err := s.db.Create(&entry).Error; err != nil {
		return fmt.Errorf("failed to create audit log entry: %w", err)
	}
	return nil
}

func (s *AuditLogService) CreateIntent(entry db.AuditLogEntry) (uint, error) {
	entry.Action = db.ActionPendingDelete
	var err error
	for attempt := 1; attempt <= 3; attempt++ {
		entry.CreatedAt = time.Now().UTC()
		err = s.db.Create(&entry).Error
		if err == nil {
			return entry.ID, nil
		}
		entry.ID = 0
	}
	return 0, fmt.Errorf("failed to create pending delete audit after 3 attempts: %w", err)
}

func (s *AuditLogService) MarkDeleted(id uint) error {
	result := s.db.Model(&db.AuditLogEntry{}).
		Where("id = ? AND action = ?", id, db.ActionPendingDelete).
		Update("action", db.ActionDeleted)
	if result.Error != nil {
		return fmt.Errorf("failed to mark audit entry deleted: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("pending delete audit entry %d not found", id)
	}
	return nil
}

func (s *AuditLogService) UpsertDryRun(entry db.AuditLogEntry) error {
	entry.CreatedAt = time.Now().UTC()
	entry.Action = db.ActionDryDelete

	var existing db.AuditLogEntry
	result := s.db.Where(
		"media_name = ? AND media_type = ? AND action = ?",
		entry.MediaName, entry.MediaType, db.ActionDryDelete,
	).First(&existing)

	if result.Error == nil {
		return s.db.Model(&existing).Updates(map[string]any{
			"score_details":  entry.ScoreDetails,
			"size_bytes":     entry.SizeBytes,
			"score":          entry.Score,
			"trigger":        entry.Trigger,
			"dry_run_reason": entry.DryRunReason,
			"integration_id": entry.IntegrationID,
			"disk_group_id":  entry.DiskGroupID,
			"created_at":     entry.CreatedAt,
		}).Error
	}

	return s.db.Clauses(clause.OnConflict{DoNothing: true}).Create(&entry).Error
}

func (s *AuditLogService) BulkUpsertDryRun(entries []db.AuditLogEntry) error {
	if len(entries) == 0 {
		return nil
	}

	now := time.Now().UTC()

	return s.db.Transaction(func(tx *gorm.DB) error {
		orConditions := tx.Where("1 = 0")
		for _, e := range entries {
			orConditions = orConditions.Or("media_name = ? AND media_type = ?", e.MediaName, e.MediaType)
		}

		var existing []db.AuditLogEntry
		if err := tx.Where("action = ?", db.ActionDryDelete).Where(orConditions).Find(&existing).Error; err != nil {
			return fmt.Errorf("failed to load existing dry-run entries: %w", err)
		}

		existingMap := make(map[string]uint, len(existing))
		for _, e := range existing {
			existingMap[db.MediaKey(e.MediaName, e.MediaType)] = e.ID
		}

		var toCreate []db.AuditLogEntry
		for _, entry := range entries {
			entry.Action = db.ActionDryDelete
			entry.CreatedAt = now
			key := db.MediaKey(entry.MediaName, entry.MediaType)

			if id, exists := existingMap[key]; exists {
				if err := tx.Model(&db.AuditLogEntry{}).Where("id = ?", id).Updates(map[string]any{
					"score_details":  entry.ScoreDetails,
					"size_bytes":     entry.SizeBytes,
					"score":          entry.Score,
					"trigger":        entry.Trigger,
					"dry_run_reason": entry.DryRunReason,
					"integration_id": entry.IntegrationID,
					"disk_group_id":  entry.DiskGroupID,
					"created_at":     now,
				}).Error; err != nil {
					return fmt.Errorf("failed to update dry-run entry %q: %w", entry.MediaName, err)
				}
			} else {
				toCreate = append(toCreate, entry)
			}
		}

		if len(toCreate) > 0 {
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).CreateInBatches(toCreate, 100).Error; err != nil {
				return fmt.Errorf("failed to batch create dry-run entries: %w", err)
			}
		}

		return nil
	})
}
