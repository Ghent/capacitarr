package services

import (
	"errors"
	"time"

	"capacitarr/internal/db"
	"capacitarr/internal/integrations"
)

// Mocks that leftover services tests (sunset, metrics, approval) used from
// deletion_test.go while everything lived in one package.

type mockSettingsReader struct {
	deletionsEnabled          bool
	deletionQueueDelaySeconds int
	executionMode             string
	snoozeDurationHours       int
}

func (m *mockSettingsReader) GetPreferences() (db.PreferenceSet, error) {
	delay := m.deletionQueueDelaySeconds
	if delay == 0 {
		delay = 30
	}
	mode := m.executionMode
	if mode == "" {
		mode = db.ModeDryRun
	}
	snooze := m.snoozeDurationHours
	if snooze == 0 {
		snooze = 24
	}
	return db.PreferenceSet{
		DeletionsEnabled:          m.deletionsEnabled,
		DeletionQueueDelaySeconds: delay,
		DefaultDiskGroupMode:      mode,
		SnoozeDurationHours:       snooze,
	}, nil
}

func (m *mockSettingsReader) GetWeightMap() (map[string]int, error) {
	return map[string]int{}, nil
}

type mockEngineStatsWriter struct{}

func (m *mockEngineStatsWriter) IncrementDeletedStats(_ uint, _ int64) error { return nil }

type mockDeletionStatsWriter struct{}

func (m *mockDeletionStatsWriter) IncrementDeletionStats(_ int64) error { return nil }

type mockDiskGroupModeReader struct {
	mode        string
	diskGroupID *uint
}

func (m *mockDiskGroupModeReader) GetByID(_ uint) (*db.DiskGroup, error) {
	if m.mode == "" {
		return nil, errors.New("not found")
	}
	return &db.DiskGroup{Mode: m.mode}, nil
}

func (m *mockDiskGroupModeReader) GetDiskGroupIDForIntegration(_ uint) *uint {
	return m.diskGroupID
}

type mockClientResolver struct {
	deleter integrations.MediaDeleter
	config  *db.IntegrationConfig
	err     error
}

func (m *mockClientResolver) GetDeleter(_ uint) (integrations.MediaDeleter, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.deleter, nil
}

func (m *mockClientResolver) GetIntegrationConfig(_ uint) (*db.IntegrationConfig, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.config != nil {
		return m.config, nil
	}
	return &db.IntegrationConfig{AddImportExclusion: true}, nil
}

type mockSunsetQueueCleaner struct {
	removedIDs []uint
}

func (m *mockSunsetQueueCleaner) RemoveCompleted(id uint) error {
	m.removedIDs = append(m.removedIDs, id)
	return nil
}

type mockIntegration struct {
	deleteErr   error
	deleteCalls int
}

func (m *mockIntegration) TestConnection() error { return nil }

func (m *mockIntegration) GetDiskSpace() ([]integrations.DiskSpace, error) {
	return nil, nil
}

func (m *mockIntegration) GetRootFolders() ([]string, error) { return nil, nil }

func (m *mockIntegration) GetMediaItems() ([]integrations.MediaItem, error) {
	return nil, nil
}

func (m *mockIntegration) DeleteMediaItem(_ integrations.MediaItem, _ integrations.DeleteOptions) error {
	m.deleteCalls++
	return m.deleteErr
}

type mockApprovalReturner struct {
	returnedIDs []uint
	removedIDs  []uint
}

func (m *mockApprovalReturner) ReturnToPending(entryID uint) error {
	m.returnedIDs = append(m.returnedIDs, entryID)
	return nil
}

func (m *mockApprovalReturner) RemoveEntry(entryID uint) error {
	m.removedIDs = append(m.removedIDs, entryID)
	return nil
}

type mockApprovalSnoozer struct {
	calledName          string
	calledType          string
	calledIntegrationID uint
	calledDiskGroupID   *uint
	calledDuration      int
}

func (m *mockApprovalSnoozer) CreateSnoozedEntry(name, mediaType string, integrationID uint, diskGroupID *uint, duration int) (*time.Time, error) {
	m.calledName = name
	m.calledType = mediaType
	m.calledIntegrationID = integrationID
	m.calledDiskGroupID = diskGroupID
	m.calledDuration = duration
	t := time.Now().Add(time.Duration(duration) * time.Hour)
	return &t, nil
}
