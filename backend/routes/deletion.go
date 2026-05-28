package routes

import (
	"log/slog"
	"net/http"

	"github.com/labstack/echo/v4"

	"capacitarr/internal/services"
)

// RegisterDeletionQueueRoutes registers deletion queue management endpoints.
func RegisterDeletionQueueRoutes(g *echo.Group, reg *services.Registry) {
	g.GET("/deletion-queue", handleListDeletionQueue(reg))
	g.DELETE("/deletion-queue", handleCancelDeletion(reg))
	g.POST("/deletion-queue/snooze", handleSnoozeDeletion(reg))
	g.POST("/deletion-queue/clear", handleClearDeletionQueue(reg))
	g.GET("/deletion-queue/grace-period", handleGracePeriodState(reg))
	g.POST("/delete", handleManualDelete(reg))
}

func handleListDeletionQueue(reg *services.Registry) echo.HandlerFunc {
	return func(c echo.Context) error {
		items := reg.Deletion.ListQueuedItems()
		return c.JSON(http.StatusOK, items)
	}
}

func handleCancelDeletion(reg *services.Registry) echo.HandlerFunc {
	return func(c echo.Context) error {
		mediaName := c.QueryParam("mediaName")
		mediaType := c.QueryParam("mediaType")

		if mediaName == "" || mediaType == "" {
			return apiError(c, http.StatusBadRequest, "mediaName and mediaType query parameters are required")
		}

		cancelled := reg.Deletion.CancelDeletion(mediaName, mediaType)
		if !cancelled {
			return apiError(c, http.StatusNotFound, "item not found in deletion queue")
		}

		return c.JSON(http.StatusOK, map[string]string{"status": "cancelled"})
	}
}

func handleSnoozeDeletion(reg *services.Registry) echo.HandlerFunc {
	return func(c echo.Context) error {
		var body struct {
			MediaName string `json:"mediaName"`
			MediaType string `json:"mediaType"`
		}
		if err := c.Bind(&body); err != nil {
			return apiError(c, http.StatusBadRequest, "Invalid request body")
		}
		if body.MediaName == "" || body.MediaType == "" {
			return apiError(c, http.StatusBadRequest, "mediaName and mediaType are required")
		}

		snoozedUntil, err := reg.Deletion.SnoozeDeletionItem(body.MediaName, body.MediaType)
		if err != nil {
			return apiError(c, http.StatusInternalServerError, "Failed to snooze item")
		}

		return c.JSON(http.StatusOK, map[string]any{
			"snoozed":      true,
			"snoozedUntil": snoozedUntil,
		})
	}
}

func handleClearDeletionQueue(reg *services.Registry) echo.HandlerFunc {
	return func(c echo.Context) error {
		count := reg.Deletion.ClearQueue()
		return c.JSON(http.StatusOK, map[string]any{"status": "cleared", "cancelled": count})
	}
}

func handleGracePeriodState(reg *services.Registry) echo.HandlerFunc {
	return func(c echo.Context) error {
		active, remaining, queueSize := reg.Deletion.GracePeriodState()
		return c.JSON(http.StatusOK, map[string]any{
			"active":           active,
			"remainingSeconds": remaining,
			"queueSize":        queueSize,
		})
	}
}

// handleManualDelete processes user-initiated deletions. Behaviour depends on
// the current execution mode:
//   - auto/dry-run: items are queued to the DeletionService immediately
//   - approval: items are upserted as pending approval queue entries
func handleManualDelete(reg *services.Registry) echo.HandlerFunc {
	return func(c echo.Context) error {
		var items []struct {
			MediaName     string  `json:"mediaName"`
			MediaType     string  `json:"mediaType"`
			IntegrationID uint    `json:"integrationId"`
			ExternalID    string  `json:"externalId"`
			SizeBytes     int64   `json:"sizeBytes"`
			ScoreDetails  string  `json:"scoreDetails"`
			PosterURL     string  `json:"posterUrl"`
			Score         float64 `json:"score"`
		}
		if err := c.Bind(&items); err != nil {
			return apiError(c, http.StatusBadRequest, "Invalid request body")
		}
		if len(items) == 0 {
			return apiError(c, http.StatusBadRequest, "No items provided")
		}

		// Convert request items to ManualDeleteItem
		deleteItems := make([]services.ManualDeleteItem, 0, len(items))
		for _, item := range items {
			deleteItems = append(deleteItems, services.ManualDeleteItem{
				MediaName:     item.MediaName,
				MediaType:     item.MediaType,
				IntegrationID: item.IntegrationID,
				ExternalID:    item.ExternalID,
				SizeBytes:     item.SizeBytes,
				ScoreDetails:  item.ScoreDetails,
				PosterURL:     item.PosterURL,
				Score:         item.Score,
			})
		}

		// Delegate to DeletionService via ApprovalService — QueueManual
		// handles mode resolution, approval routing, and dry-run determination.
		result, err := reg.Approval.ManualDelete(deleteItems, services.ManualDeleteDeps{
			Deletion: reg.Deletion,
		})
		if err != nil {
			slog.Error("Manual delete failed", "error", err)
			return apiError(c, http.StatusInternalServerError, "Failed to process delete request")
		}

		return c.JSON(http.StatusOK, result)
	}
}
