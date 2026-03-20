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

		return c.JSON(http.StatusOK, map[string]bool{"cancelled": true})
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

		// Load preferences for execution mode and deletions-enabled flag
		prefs, err := reg.Settings.GetPreferences()
		if err != nil {
			slog.Error("Failed to load preferences for manual delete", "error", err)
			return apiError(c, http.StatusInternalServerError, "Failed to load preferences")
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

		result, err := reg.Approval.ManualDelete(deleteItems, prefs.ExecutionMode, prefs.DeletionsEnabled, services.ManualDeleteDeps{
			Integration: reg.Integration,
			Deletion:    reg.Deletion,
			Engine:      reg.Engine,
		})
		if err != nil {
			slog.Error("Manual delete failed", "error", err)
			return apiError(c, http.StatusInternalServerError, "Failed to process delete request")
		}

		return c.JSON(http.StatusOK, result)
	}
}
