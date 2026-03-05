package routes

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"

	"capacitarr/internal/db"
	"capacitarr/internal/engine"
	"capacitarr/internal/integrations"
	"capacitarr/internal/poller"
)

// RegisterAuditRoutes sets up the API endpoints for audit logs
func RegisterAuditRoutes(g *echo.Group, database *gorm.DB) {
	// Grouped audit: show-level and season-level entries grouped into a tree
	g.GET("/audit/grouped", func(c echo.Context) error {
		limit := 200
		if l := c.QueryParam("limit"); l != "" {
			if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
				limit = parsed
			}
		}
		if limit > 2000 {
			limit = 2000
		}

		logs := make([]db.AuditLog, 0)
		if err := database.Order("created_at desc").Limit(limit).Find(&logs).Error; err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch audit logs"})
		}

		// Group by show title for TV content (seasons/episodes)
		type AuditGroup struct {
			ShowTitle string        `json:"showTitle"`
			Children  []db.AuditLog `json:"children"`
			// Summary fields
			TotalSize int64  `json:"totalSize"`
			Action    string `json:"action"`
			CreatedAt string `json:"createdAt"`
		}

		groups := make(map[string]*AuditGroup)
		standalone := make([]db.AuditLog, 0)
		var orderedGroupKeys []string

		for _, log := range logs {
			if log.MediaType == "season" || log.MediaType == "episode" {
				// Extract show title: "Show - Season X" → "Show"
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
						Children:  []db.AuditLog{log},
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

		// Build result: interleave groups and standalone items by time
		type GroupedResult struct {
			Type  string       `json:"type"` // "group" or "single"
			Group *AuditGroup  `json:"group,omitempty"`
			Entry *db.AuditLog `json:"entry,omitempty"`
		}

		result := make([]GroupedResult, 0, len(orderedGroupKeys)+len(standalone))
		for _, key := range orderedGroupKeys {
			grp := groups[key]
			result = append(result, GroupedResult{Type: "group", Group: grp})
		}
		for _, log := range standalone {
			entry := log
			result = append(result, GroupedResult{Type: "single", Entry: &entry})
		}

		return c.JSON(http.StatusOK, result)
	})

	g.GET("/audit", func(c echo.Context) error {
		limit := 50
		if l := c.QueryParam("limit"); l != "" {
			if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
				limit = parsed
			}
		}
		// Cap limit to prevent excessively large queries
		if limit > 1000 {
			limit = 1000
		}

		offset := 0
		if o := c.QueryParam("offset"); o != "" {
			if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
				offset = parsed
			}
		}

		// Build query with optional search and action filters
		query := database.Model(&db.AuditLog{})

		if search := strings.TrimSpace(c.QueryParam("search")); search != "" {
			query = query.Where("media_name LIKE ?", "%"+search+"%")
		}

		if action := strings.TrimSpace(c.QueryParam("action")); action != "" {
			query = query.Where("action = ?", action)
		}

		// Sorting: whitelist allowed columns to prevent SQL injection
		allowedSortColumns := map[string]string{
			"created_at": "created_at",
			"media_name": "media_name",
			"size_bytes": "size_bytes",
			"action":     "action",
		}
		sortBy := "created_at"
		if sb := strings.TrimSpace(c.QueryParam("sort_by")); sb != "" {
			if col, ok := allowedSortColumns[sb]; ok {
				sortBy = col
			}
		}
		sortDir := "desc"
		if sd := strings.ToLower(strings.TrimSpace(c.QueryParam("sort_dir"))); sd == "asc" || sd == "desc" {
			sortDir = sd
		}
		orderClause := sortBy + " " + sortDir

		logs := make([]db.AuditLog, 0)
		var total int64

		// Get total count with filters applied
		query.Count(&total)

		// Get paginated logs with requested sort order
		if err := query.Order(orderClause).Limit(limit).Offset(offset).Find(&logs).Error; err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch audit logs"})
		}

		return c.JSON(http.StatusOK, map[string]interface{}{
			"data":   logs,
			"total":  total,
			"limit":  limit,
			"offset": offset,
		})
	})

	// Approve a queued-for-approval audit entry: queue the item for deletion
	g.POST("/audit/:id/approve", func(c echo.Context) error {
		id := c.Param("id")

		var entry db.AuditLog
		if err := database.First(&entry, id).Error; err != nil {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "Audit entry not found"})
		}

		if entry.Action != db.ActionQueuedForApproval {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"error": "Entry is not queued for approval",
			})
		}

		// Safety check: block approvals when deletions are disabled
		var prefs db.PreferenceSet
		if err := database.FirstOrCreate(&prefs, db.PreferenceSet{ID: 1}).Error; err != nil {
			slog.Error("Failed to load preferences for approval check", "error", err)
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": "Failed to load preferences",
			})
		}
		if !prefs.DeletionsEnabled {
			return c.JSON(http.StatusConflict, map[string]string{
				"error": "Deletions are currently disabled in settings. Enable deletions before approving items.",
			})
		}

		if entry.IntegrationID == nil {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"error": "Audit entry has no associated integration",
			})
		}

		// Look up the integration to construct a client
		var integration db.IntegrationConfig
		if err := database.First(&integration, *entry.IntegrationID).Error; err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"error": "Integration not found",
			})
		}

		client := poller.CreateClient(integration.Type, integration.URL, integration.APIKey)
		if client == nil {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"error": "Unsupported integration type",
			})
		}

		// Reconstruct the MediaItem from stored audit data
		item := integrations.MediaItem{
			ExternalID:    entry.ExternalID,
			IntegrationID: *entry.IntegrationID,
			Type:          integrations.MediaType(entry.MediaType),
			Title:         entry.MediaName,
			SizeBytes:     entry.SizeBytes,
		}

		// Parse stored score details back into factors
		var factors []engine.ScoreFactor
		if entry.ScoreDetails != "" {
			if err := json.Unmarshal([]byte(entry.ScoreDetails), &factors); err != nil {
				slog.Warn("Failed to parse score details for approval", "id", entry.ID, "error", err)
			}
		}

		// Queue for background deletion
		if err := poller.QueueDeletion(client, item, entry.Reason, 0, factors, 0); err != nil {
			return c.JSON(http.StatusServiceUnavailable, map[string]string{
				"error": "Deletion queue is full, try again later",
			})
		}

		// Update audit entry to "Approved"
		if err := database.Model(&entry).Update("action", db.ActionApproved).Error; err != nil {
			slog.Error("Failed to update audit entry to Approved", "id", entry.ID, "error", err)
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": "Failed to update audit entry",
			})
		}

		return c.JSON(http.StatusOK, map[string]string{"status": "approved"})
	})

	// Reject a queued-for-approval audit entry and snooze it
	g.POST("/audit/:id/reject", func(c echo.Context) error {
		id := c.Param("id")

		var entry db.AuditLog
		if err := database.First(&entry, id).Error; err != nil {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "Audit entry not found"})
		}

		if entry.Action != db.ActionQueuedForApproval {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"error": "Entry is not queued for approval",
			})
		}

		// Load preferences to get configured snooze duration
		var prefs db.PreferenceSet
		if err := database.FirstOrCreate(&prefs, db.PreferenceSet{ID: 1}).Error; err != nil {
			slog.Error("Failed to load preferences for snooze duration", "error", err)
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": "Failed to load preferences",
			})
		}

		snoozedUntil := time.Now().UTC().Add(time.Duration(prefs.SnoozeDurationHours) * time.Hour)

		if err := database.Model(&entry).Updates(map[string]interface{}{
			"action":        db.ActionRejected,
			"snoozed_until": snoozedUntil,
		}).Error; err != nil {
			slog.Error("Failed to update audit entry to Rejected", "id", entry.ID, "error", err)
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": "Failed to update audit entry",
			})
		}

		return c.JSON(http.StatusOK, map[string]string{"status": "rejected"})
	})

	// Unsnooze a snoozed audit entry: clear snooze and reset to Pending
	g.POST("/audit/:id/unsnooze", func(c echo.Context) error {
		id := c.Param("id")

		var entry db.AuditLog
		if err := database.First(&entry, id).Error; err != nil {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "Audit entry not found"})
		}

		if err := database.Model(&entry).Updates(map[string]interface{}{
			"snoozed_until": nil,
			"action":        db.ActionQueuedForApproval,
		}).Error; err != nil {
			slog.Error("Failed to unsnooze audit entry", "id", entry.ID, "error", err)
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": "Failed to unsnooze audit entry",
			})
		}

		// Reload the updated entry
		if err := database.First(&entry, id).Error; err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to reload audit entry"})
		}

		return c.JSON(http.StatusOK, entry)
	})
}

// parseDuration parses shorthand duration strings like "1h", "24h", "7d", "30d".
func parseDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty duration")
	}

	suffix := s[len(s)-1:]
	numStr := s[:len(s)-1]

	n, err := strconv.Atoi(numStr)
	if err != nil {
		return 0, fmt.Errorf("invalid duration number: %s", numStr)
	}

	switch suffix {
	case "h":
		return time.Duration(n) * time.Hour, nil
	case "d":
		return time.Duration(n) * 24 * time.Hour, nil
	case "m":
		return time.Duration(n) * time.Minute, nil
	default:
		return 0, fmt.Errorf("unsupported duration suffix: %s", suffix)
	}
}
