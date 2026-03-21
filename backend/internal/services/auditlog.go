package services

import (
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"capacitarr/internal/db"
)

// AuditListParams holds the query parameters for paginated audit log listing.
type AuditListParams struct {
	Limit   int
	Offset  int
	Search  string
	Action  string
	SortBy  string
	SortDir string
}

// AuditListResult holds the paginated result of an audit log query.
type AuditListResult struct {
	Data   []db.AuditLogEntry `json:"data"`
	Total  int64              `json:"total"`
	Limit  int                `json:"limit"`
	Offset int                `json:"offset"`
}

// GroupedAuditResult represents either a grouped set of TV entries or a standalone entry.
type GroupedAuditResult struct {
	Type  string            `json:"type"`
	Group *AuditGroup       `json:"group,omitempty"`
	Entry *db.AuditLogEntry `json:"entry,omitempty"`
}

// AuditGroup holds a group of related audit entries for a single show.
type AuditGroup struct {
	ShowTitle string             `json:"showTitle"`
	Children  []db.AuditLogEntry `json:"children"`
	TotalSize int64              `json:"totalSize"`
	Action    string             `json:"action"`
	CreatedAt string             `json:"createdAt"`
}

// AuditLogService manages the append-only audit log (deletion/dry-run history).
type AuditLogService struct {
	db *gorm.DB
}

// NewAuditLogService creates a new AuditLogService.
func NewAuditLogService(database *gorm.DB) *AuditLogService {
	return &AuditLogService{db: database}
}

// Create appends a new audit log entry. Entries are immutable after creation.
func (s *AuditLogService) Create(entry db.AuditLogEntry) error {
	entry.CreatedAt = time.Now().UTC()
	if err := s.db.Create(&entry).Error; err != nil {
		return fmt.Errorf("failed to create audit log entry: %w", err)
	}
	return nil
}

// UpsertDryRun creates or updates a dry-run audit log entry.
// If an entry with the same media_name, media_type, and action=dry_delete already
// exists, it is updated. Otherwise, a new entry is created. The entry's Action
// field is forced to ActionDryDelete regardless of what the caller passes.
func (s *AuditLogService) UpsertDryRun(entry db.AuditLogEntry) error {
	entry.CreatedAt = time.Now().UTC()
	entry.Action = db.ActionDryDelete

	// Try to find an existing dry-run entry for the same media
	var existing db.AuditLogEntry
	result := s.db.Where(
		"media_name = ? AND media_type = ? AND action = ?",
		entry.MediaName, entry.MediaType, db.ActionDryDelete,
	).First(&existing)

	if result.Error == nil {
		// Update existing entry
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

	// Create new entry
	return s.db.Clauses(clause.OnConflict{DoNothing: true}).Create(&entry).Error
}

// ListRecent returns the most recent N audit log entries, ordered newest first.
func (s *AuditLogService) ListRecent(limit int) ([]db.AuditLogEntry, error) {
	logs := make([]db.AuditLogEntry, 0, limit)
	if err := s.db.Order("created_at desc").Limit(limit).Find(&logs).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch recent audit logs: %w", err)
	}
	return logs, nil
}

// ListGrouped returns audit log entries grouped by show title for TV content.
// Seasons and episodes with the same show title are grouped together; all other
// media types are returned as standalone entries.
func (s *AuditLogService) ListGrouped(limit int) ([]GroupedAuditResult, error) {
	logs := make([]db.AuditLogEntry, 0)
	if err := s.db.Order("created_at desc").Limit(limit).Find(&logs).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch audit logs: %w", err)
	}

	groups := make(map[string]*AuditGroup)
	standalone := make([]db.AuditLogEntry, 0)
	var orderedGroupKeys []string

	for _, log := range logs {
		if log.MediaType == "season" || log.MediaType == "episode" {
			showTitle := log.MediaName
			if idx := strings.Index(log.MediaName, " - Season"); idx > 0 {
				showTitle = log.MediaName[:idx]
			} else if idx := strings.Index(log.MediaName, " - S"); idx > 0 {
				showTitle = log.MediaName[:idx]
			}

			if grp, ok := groups[showTitle]; ok {
				grp.Children = append(grp.Children, log)
				grp.TotalSize += log.SizeBytes
			} else {
				groups[showTitle] = &AuditGroup{
					ShowTitle: showTitle,
					Children:  []db.AuditLogEntry{log},
					TotalSize: log.SizeBytes,
					Action:    log.Action,
					CreatedAt: log.CreatedAt.Format(time.RFC3339),
				}
				orderedGroupKeys = append(orderedGroupKeys, showTitle)
			}
		} else {
			standalone = append(standalone, log)
		}
	}

	result := make([]GroupedAuditResult, 0, len(orderedGroupKeys)+len(standalone))
	for _, key := range orderedGroupKeys {
		grp := groups[key]
		result = append(result, GroupedAuditResult{Type: "group", Group: grp})
	}
	for _, log := range standalone {
		entry := log
		result = append(result, GroupedAuditResult{Type: "single", Entry: &entry})
	}

	return result, nil
}

// ListPaginated returns a paginated, searchable, sortable list of audit log entries.
func (s *AuditLogService) ListPaginated(params AuditListParams) (*AuditListResult, error) {
	query := s.db.Model(&db.AuditLogEntry{})

	if params.Search != "" {
		query = query.Where("media_name LIKE ?", "%"+params.Search+"%")
	}

	if params.Action != "" {
		query = query.Where("action = ?", params.Action)
	}

	allowedSortColumns := map[string]string{
		"created_at": "created_at",
		"media_name": "media_name",
		"size_bytes": "size_bytes",
		"action":     "action",
	}
	sortBy := "created_at"
	if col, ok := allowedSortColumns[params.SortBy]; ok {
		sortBy = col
	}
	sortDir := "desc"
	if params.SortDir == "asc" || params.SortDir == "desc" {
		sortDir = params.SortDir
	}
	orderClause := sortBy + " " + sortDir

	var total int64
	query.Count(&total)

	logs := make([]db.AuditLogEntry, 0)
	if err := query.Order(orderClause).Limit(params.Limit).Offset(params.Offset).Find(&logs).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch audit logs: %w", err)
	}

	return &AuditListResult{
		Data:   logs,
		Total:  total,
		Limit:  params.Limit,
		Offset: params.Offset,
	}, nil
}

// PruneOlderThan deletes audit log entries older than the given duration.
// Returns the number of entries deleted.
func (s *AuditLogService) PruneOlderThan(retentionDays int) (int64, error) {
	if retentionDays <= 0 {
		return 0, nil // 0 = keep forever
	}

	cutoff := time.Now().UTC().AddDate(0, 0, -retentionDays)
	result := s.db.Where("created_at < ?", cutoff).Delete(&db.AuditLogEntry{})
	if result.Error != nil {
		return 0, fmt.Errorf("failed to prune audit log: %w", result.Error)
	}
	return result.RowsAffected, nil
}
