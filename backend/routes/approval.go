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

		// Optional disk group filter
		var diskGroupID *uint
		if dgStr := c.QueryParam("disk_group_id"); dgStr != "" {
			if parsed, err := strconv.ParseUint(dgStr, 10, 64); err == nil {
				dgID := uint(parsed)
				diskGroupID = &dgID
			}
		}

		items, err := reg.Approval.ListQueue(status, limit, diskGroupID)
		if err != nil {
			return apiError(c, http.StatusInternalServerError, "Failed to fetch approval queue")
		}

		return c.JSON(http.StatusOK, items)
	})

	// Approve a queued item: queue it for deletion.
	// When DeletionsEnabled=false or in dry-run mode, the item is still approved
	// and queued — the DeletionService will simulate (dry-delete) instead of
	// performing an actual deletion.
	g.POST("/approval-queue/:id/approve", func(c echo.Context) error {
		id := c.Param("id")
		entryID, err := strconv.ParseUint(id, 10, 64)
		if err != nil {
			return apiError(c, http.StatusBadRequest, "Invalid ID")
		}

		// Execute the full approval workflow via service — the service handles
		// dry-run determination from preferences internally.
		approved, err := reg.Approval.ExecuteApproval(uint(entryID), services.ExecuteApprovalDeps{
			Integration: reg.Integration,
			Deletion:    reg.Deletion,
			Engine:      reg.Engine,
			Settings:    reg.Settings,
		})
		if err != nil {
			if errors.Is(err, services.ErrApprovalNotPending) {
				return apiError(c, http.StatusBadRequest, err.Error())
			}
			if errors.Is(err, services.ErrApprovalNotFound) {
				return apiError(c, http.StatusNotFound, "Approval queue entry not found")
			}
			slog.Error("Approval execution failed", "error", err)
			return apiError(c, http.StatusInternalServerError, "Failed to approve entry")
		}

		return c.JSON(http.StatusOK, approved)
	})

	// Reject a queued item: snooze it
	g.POST("/approval-queue/:id/reject", func(c echo.Context) error {
		id := c.Param("id")
		entryID, err := strconv.ParseUint(id, 10, 64)
		if err != nil {
			return apiError(c, http.StatusBadRequest, "Invalid ID")
		}

		// Load preferences to get configured snooze duration
		prefs, err := reg.Settings.GetPreferences()
		if err != nil {
			slog.Error("Failed to load preferences for snooze duration", "error", err)
			return apiError(c, http.StatusInternalServerError, "Failed to load preferences")
		}

		rejected, err := reg.Approval.Reject(uint(entryID), prefs.SnoozeDurationHours)
		if err != nil {
			if errors.Is(err, services.ErrApprovalNotPending) {
				return apiError(c, http.StatusBadRequest, err.Error())
			}
			if errors.Is(err, services.ErrApprovalNotFound) {
				return apiError(c, http.StatusNotFound, "Approval queue entry not found")
			}
			return apiError(c, http.StatusInternalServerError, "Failed to reject entry")
		}

		return c.JSON(http.StatusOK, rejected)
	})

	// Unsnooze a rejected item: clear snooze and reset to pending
	g.POST("/approval-queue/:id/unsnooze", func(c echo.Context) error {
		id := c.Param("id")
		entryID, err := strconv.ParseUint(id, 10, 64)
		if err != nil {
			return apiError(c, http.StatusBadRequest, "Invalid ID")
		}

		unsnoozed, err := reg.Approval.Unsnooze(uint(entryID))
		if err != nil {
			if errors.Is(err, services.ErrApprovalNotPending) {
				return apiError(c, http.StatusBadRequest, err.Error())
			}
			if errors.Is(err, services.ErrApprovalNotFound) {
				return apiError(c, http.StatusNotFound, "Approval queue entry not found")
			}
			return apiError(c, http.StatusInternalServerError, "Failed to unsnooze entry")
		}

		return c.JSON(http.StatusOK, unsnoozed)
	})

	// Dismiss a single queued item (pending or rejected only)
	g.DELETE("/approval-queue/:id", func(c echo.Context) error {
		id := c.Param("id")
		entryID, err := strconv.ParseUint(id, 10, 64)
		if err != nil {
			return apiError(c, http.StatusBadRequest, "Invalid ID")
		}

		if err := reg.Approval.Dismiss(uint(entryID)); err != nil {
			if errors.Is(err, services.ErrApprovalNotDismissable) {
				return apiError(c, http.StatusBadRequest, err.Error())
			}
			if errors.Is(err, services.ErrApprovalNotFound) {
				return apiError(c, http.StatusNotFound, "Approval queue entry not found")
			}
			return apiError(c, http.StatusInternalServerError, "Failed to dismiss entry")
		}

		return c.JSON(http.StatusOK, map[string]string{"status": "dismissed"})
	})

	// Approve all pending items in a collection group
	g.POST("/approval-queue/group/approve", func(c echo.Context) error {
		var body struct {
			CollectionGroup string `json:"collectionGroup"`
		}
		if err := c.Bind(&body); err != nil || body.CollectionGroup == "" {
			return apiError(c, http.StatusBadRequest, "collectionGroup is required")
		}

		// Find pending items in this collection group
		items, err := reg.Approval.ListQueue("pending", 200, nil)
		if err != nil {
			return apiError(c, http.StatusInternalServerError, "Failed to query approval queue")
		}

		var groupItems []uint
		for _, item := range items {
			if item.CollectionGroup == body.CollectionGroup {
				groupItems = append(groupItems, item.ID)
			}
		}

		if len(groupItems) == 0 {
			return apiError(c, http.StatusNotFound, "No pending items found for collection group")
		}

		// Execute approval for each item in the group
		deps := services.ExecuteApprovalDeps{
			Integration: reg.Integration,
			Deletion:    reg.Deletion,
			Engine:      reg.Engine,
			Settings:    reg.Settings,
		}

		var approved []any
		var lastErr error
		for _, entryID := range groupItems {
			result, execErr := reg.Approval.ExecuteApproval(entryID, deps)
			if execErr != nil {
				slog.Error("Group approval: failed to approve entry", "entryID", entryID, "error", execErr)
				lastErr = execErr
				continue
			}
			approved = append(approved, result)
		}

		if len(approved) == 0 && lastErr != nil {
			return apiError(c, http.StatusInternalServerError, "Failed to approve any items in collection group")
		}

		return c.JSON(http.StatusOK, map[string]any{
			"approved": len(approved),
			"items":    approved,
		})
	})

	// Reject all pending items in a collection group
	g.POST("/approval-queue/group/reject", func(c echo.Context) error {
		var body struct {
			CollectionGroup string `json:"collectionGroup"`
		}
		if err := c.Bind(&body); err != nil || body.CollectionGroup == "" {
			return apiError(c, http.StatusBadRequest, "collectionGroup is required")
		}

		// Load preferences for snooze duration
		prefs, err := reg.Settings.GetPreferences()
		if err != nil {
			slog.Error("Failed to load preferences for snooze duration", "error", err)
			return apiError(c, http.StatusInternalServerError, "Failed to load preferences")
		}

		rejected, err := reg.Approval.RejectGroup(body.CollectionGroup, prefs.SnoozeDurationHours)
		if err != nil {
			if errors.Is(err, services.ErrApprovalGroupEmpty) {
				return apiError(c, http.StatusNotFound, "No pending items found for collection group")
			}
			return apiError(c, http.StatusInternalServerError, "Failed to reject collection group")
		}

		return c.JSON(http.StatusOK, map[string]any{
			"rejected": len(rejected),
			"items":    rejected,
		})
	})

	// Clear the entire approval queue (pending + rejected items)
	g.POST("/approval-queue/clear", func(c echo.Context) error {
		count, err := reg.Approval.ClearQueue()
		if err != nil {
			return apiError(c, http.StatusInternalServerError, "Failed to clear approval queue")
		}

		return c.JSON(http.StatusOK, map[string]any{"cleared": count})
	})
}
