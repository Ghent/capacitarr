package routes

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"

	"capacitarr/internal/services"
)

// RegisterAuditRoutes sets up the API endpoints for the audit log (history-only).
// Approval queue endpoints are in approval.go.
func RegisterAuditRoutes(g *echo.Group, reg *services.Registry) {
	// Recent audit: lightweight list of the most recent N entries (for dashboard mini-feed)
	g.GET("/audit-log/recent", func(c echo.Context) error {
		limit := 5
		if l := c.QueryParam("limit"); l != "" {
			if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
				limit = parsed
			}
		}
		if limit > 100 {
			limit = 100
		}

		logs, err := reg.AuditLog.ListRecent(limit)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch recent audit logs"})
		}

		return c.JSON(http.StatusOK, logs)
	})

	// Grouped audit: show-level and season-level entries grouped into a tree
	g.GET("/audit-log/grouped", func(c echo.Context) error {
		limit := 200
		if l := c.QueryParam("limit"); l != "" {
			if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
				limit = parsed
			}
		}
		if limit > 2000 {
			limit = 2000
		}

		result, err := reg.AuditLog.ListGrouped(limit)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch audit logs"})
		}

		return c.JSON(http.StatusOK, result)
	})

	// Paginated audit log with search and sort
	g.GET("/audit-log", func(c echo.Context) error {
		limit := 50
		if l := c.QueryParam("limit"); l != "" {
			if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
				limit = parsed
			}
		}
		if limit > 1000 {
			limit = 1000
		}

		offset := 0
		if o := c.QueryParam("offset"); o != "" {
			if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
				offset = parsed
			}
		}

		result, err := reg.AuditLog.ListPaginated(services.AuditListParams{
			Limit:   limit,
			Offset:  offset,
			Search:  strings.TrimSpace(c.QueryParam("search")),
			Action:  strings.TrimSpace(c.QueryParam("action")),
			SortBy:  strings.TrimSpace(c.QueryParam("sort_by")),
			SortDir: strings.ToLower(strings.TrimSpace(c.QueryParam("sort_dir"))),
		})
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch audit logs"})
		}

		return c.JSON(http.StatusOK, result)
	})
}
