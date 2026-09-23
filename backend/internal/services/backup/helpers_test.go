package backup

import (
	"fmt"
	"testing"

	_ "github.com/ncruces/go-sqlite3/embed"
	"github.com/ncruces/go-sqlite3/gormlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"capacitarr/internal/db"
	"capacitarr/internal/events"
)

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

// testDiskGroups implements diskGroupStore with the same List/ImportUpsert
// SQL as DiskGroupService so backup tests keep exercising real persistence.
type testDiskGroups struct {
	db *gorm.DB
}

func newTestDiskGroups(database *gorm.DB) *testDiskGroups {
	return &testDiskGroups{db: database}
}

func (s *testDiskGroups) List() ([]db.DiskGroup, error) {
	groups := make([]db.DiskGroup, 0)
	if err := s.db.Find(&groups).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch disk groups: %w", err)
	}
	return groups, nil
}

func (s *testDiskGroups) ImportUpsert(mountPath string, threshold, target float64, totalOverride *int64) error {
	var existing db.DiskGroup
	err := s.db.Where("mount_path = ?", mountPath).First(&existing).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return fmt.Errorf("failed to check disk group %q: %w", mountPath, err)
	}
	if err == gorm.ErrRecordNotFound {
		dg := db.DiskGroup{
			MountPath:          mountPath,
			ThresholdPct:       threshold,
			TargetPct:          target,
			TotalBytesOverride: totalOverride,
		}
		if createErr := s.db.Create(&dg).Error; createErr != nil {
			return fmt.Errorf("failed to create disk group %q: %w", mountPath, createErr)
		}
	} else {
		existing.ThresholdPct = threshold
		existing.TargetPct = target
		existing.TotalBytesOverride = totalOverride
		existing.StaleSince = nil
		if saveErr := s.db.Save(&existing).Error; saveErr != nil {
			return fmt.Errorf("failed to update disk group %q: %w", mountPath, saveErr)
		}
	}
	return nil
}
