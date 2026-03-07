package routes

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"capacitarr/internal/services"
)

// RegisterApprovalRoutes sets up the API endpoints for the approval queue.
func RegisterApprovalRoutes(g *echo.Group, reg *services.Registry) {
	// List approval queue items
	g.GET("/approval-queue", func(c echo.Context) error {
		limit := 200
		if l := c.QueryParam("limit"); l != "" {
			if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
				limit = parsed
			}
		}
		if limit > 2000 {
			limit = 2000
		}

		status := c.QueryParam("status")
		items, err := reg.Approval.ListQueue(status, limit)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch approval queue"})
		}

		return c.JSON(http.StatusOK, items)
	})

	// Approve a queued item: queue it for deletion
	g.POST("/approval-queue/:id/approve", func(c echo.Context) error {
		id := c.Param("id")
		entryID, err := strconv.ParseUint(id, 10, 64)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid ID"})
		}

		// Safety check: block approvals when deletions are disabled
		prefs, err := reg.Settings.GetPreferences()
		if err != nil {
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

		// Execute the full approval workflow via service
		approved, err := reg.Approval.ExecuteApproval(uint(entryID), services.ExecuteApprovalDeps{
			Integration: reg.Integration,
			Deletion:    reg.Deletion,
		})
		if err != nil {
			if errors.Is(err, services.ErrApprovalNotPending) {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
			}
			if errors.Is(err, services.ErrApprovalNotFound) {
				return c.JSON(http.StatusNotFound, map[string]string{"error": "Approval queue entry not found"})
			}
			slog.Error("Approval execution failed", "error", err)
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to approve entry"})
		}

		return c.JSON(http.StatusOK, approved)
	})

	// Reject a queued item: snooze it
	g.POST("/approval-queue/:id/reject", func(c echo.Context) error {
		id := c.Param("id")
		entryID, err := strconv.ParseUint(id, 10, 64)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid ID"})
		}

		// Load preferences to get configured snooze duration
		prefs, err := reg.Settings.GetPreferences()
		if err != nil {
			slog.Error("Failed to load preferences for snooze duration", "error", err)
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": "Failed to load preferences",
			})
		}

		rejected, err := reg.Approval.Reject(uint(entryID), prefs.SnoozeDurationHours)
		if err != nil {
			if errors.Is(err, services.ErrApprovalNotPending) {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
			}
			if errors.Is(err, services.ErrApprovalNotFound) {
				return c.JSON(http.StatusNotFound, map[string]string{"error": "Approval queue entry not found"})
			}
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to reject entry"})
		}

		return c.JSON(http.StatusOK, rejected)
	})

	// Unsnooze a rejected item: clear snooze and reset to pending
	g.POST("/approval-queue/:id/unsnooze", func(c echo.Context) error {
		id := c.Param("id")
		entryID, err := strconv.ParseUint(id, 10, 64)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid ID"})
		}

		unsnoozed, err := reg.Approval.Unsnooze(uint(entryID))
		if err != nil {
			if errors.Is(err, services.ErrApprovalNotPending) {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
			}
			if errors.Is(err, services.ErrApprovalNotFound) {
				return c.JSON(http.StatusNotFound, map[string]string{"error": "Approval queue entry not found"})
			}
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to unsnooze entry"})
		}

		return c.JSON(http.StatusOK, unsnoozed)
	})
}
