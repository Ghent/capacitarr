package routes

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"

	"capacitarr/internal/db"
)

// RegisterAuditRoutes sets up the API endpoints for audit logs
func RegisterAuditRoutes(g *echo.Group, database *gorm.DB) {
	// Activity sparkline: audit log counts grouped by time buckets, split by flagged/deleted
	g.GET("/audit/activity", func(c echo.Context) error {
		since := c.QueryParam("since")
		if since == "" {
			since = "24h"
		}

		dur, err := parseDuration(since)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid since parameter"})
		}

		cutoff := time.Now().UTC().Add(-dur).Format("2006-01-02 15:04:05")

		// Auto-adjust bucket size based on time range
		bucketMinutes := bucketMinutesForDuration(dur)

		type BucketRow struct {
			Bucket  string `json:"bucket"`
			Flagged int    `json:"flagged"`
			Deleted int    `json:"deleted"`
		}

		var rows []BucketRow
		query := fmt.Sprintf(
			`SELECT strftime('%%Y-%%m-%%d %%H:', created_at) || printf('%%02d', (CAST(strftime('%%M', created_at) AS INTEGER) / %d) * %d) AS bucket,
			 SUM(CASE WHEN action = 'Dry-Run' THEN 1 ELSE 0 END) AS flagged,
			 SUM(CASE WHEN action = 'Deleted' THEN 1 ELSE 0 END) AS deleted
			 FROM audit_logs WHERE created_at >= ? GROUP BY bucket ORDER BY bucket`,
			bucketMinutes, bucketMinutes,
		)
		if err := database.Raw(query, cutoff).Scan(&rows).Error; err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to query activity"})
		}

		type ActivityPoint struct {
			Timestamp string `json:"timestamp"`
			Flagged   int    `json:"flagged"`
			Deleted   int    `json:"deleted"`
		}

		result := make([]ActivityPoint, len(rows))
		for i, r := range rows {
			result[i] = ActivityPoint{Timestamp: r.Bucket, Flagged: r.Flagged, Deleted: r.Deleted}
		}

		return c.JSON(http.StatusOK, result)
	})

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

		var logs []db.AuditLog
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
		var standalone []db.AuditLog
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

		var result []GroupedResult
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

		var logs []db.AuditLog
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
}

// bucketMinutesForDuration returns the grouping bucket size in minutes based on the time range.
func bucketMinutesForDuration(d time.Duration) int {
	switch {
	case d <= 1*time.Hour:
		return 5
	case d <= 6*time.Hour:
		return 15
	case d <= 24*time.Hour:
		return 15
	case d <= 7*24*time.Hour:
		return 60
	default:
		return 360 // 6 hours
	}
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
