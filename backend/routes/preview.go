package routes

import (
	"log/slog"
	"net/http"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"

	"capacitarr/internal/db"
	"capacitarr/internal/engine"
	"capacitarr/internal/integrations"
)

// RegisterPreviewRoutes sets up the score preview and engine trigger endpoints.
func RegisterPreviewRoutes(protected *echo.Group, database *gorm.DB) {
	protected.GET("/preview", func(c echo.Context) error {
		var configs []db.IntegrationConfig
		if err := database.Where("enabled = ?", true).Find(&configs).Error; err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to load integrations"})
		}

		var allItems []integrations.MediaItem
		slog.Debug("Preview: fetching media from integrations", "component", "api", "configCount", len(configs))
		for _, cfg := range configs {
			slog.Debug("Preview: checking integration", "component", "api", "name", cfg.Name, "type", cfg.Type)
			if cfg.Type == "plex" || cfg.Type == "tautulli" || cfg.Type == "overseerr" || cfg.Type == "jellyfin" || cfg.Type == "emby" {
				slog.Debug("Preview: skipping non-arr integration", "component", "api", "type", cfg.Type)
				continue
			}
			client := CreateClient(cfg.Type, cfg.URL, cfg.APIKey)
			if client == nil {
				slog.Warn("Preview: CreateClient returned nil", "component", "api", "type", cfg.Type)
				continue
			}
			items, err := client.GetMediaItems()
			if err != nil {
				slog.Warn("Preview: media fetch failed", "component", "api", "integration", cfg.Name, "type", cfg.Type, "error", err)
				continue
			}
			slog.Debug("Preview: fetched items", "component", "api", "integration", cfg.Name, "type", cfg.Type, "count", len(items))
			for i := range items {
				items[i].IntegrationID = cfg.ID
			}
			allItems = append(allItems, items...)
		}
		slog.Debug("Preview: total items collected", "component", "api", "count", len(allItems))

		var prefs db.PreferenceSet
		database.FirstOrCreate(&prefs, db.PreferenceSet{ID: 1})

		var rules []db.ProtectionRule
		database.Find(&rules)

		evaluated := engine.EvaluateMedia(allItems, prefs, rules)
		slog.Debug("Preview: evaluated items", "component", "api", "count", len(evaluated))

		// Sort by score descending with tiebreaker
		engine.SortEvaluated(evaluated, prefs.TiebreakerMethod)

		// Build disk context from disk groups (needed for deletion line in UI)
		var diskGroups []db.DiskGroup
		database.Find(&diskGroups)

		type diskContextPayload struct {
			TotalBytes   int64   `json:"totalBytes"`
			UsedBytes    int64   `json:"usedBytes"`
			TargetPct    float64 `json:"targetPct"`
			ThresholdPct float64 `json:"thresholdPct"`
			BytesToFree  int64   `json:"bytesToFree"`
		}

		var diskCtx *diskContextPayload
		if len(diskGroups) > 0 {
			// Pick the disk group that is over threshold with the most bytes to free.
			// If none are over threshold, pick the one with the most potential bytes to free.
			var bestGroup *db.DiskGroup
			var bestBytesToFree int64

			for i := range diskGroups {
				dg := &diskGroups[i]
				if dg.TotalBytes == 0 {
					continue
				}
				usedPct := float64(dg.UsedBytes) / float64(dg.TotalBytes) * 100
				var btf int64
				if usedPct >= dg.ThresholdPct {
					btf = dg.UsedBytes - int64(float64(dg.TotalBytes)*dg.TargetPct/100)
					if btf < 0 {
						btf = 0
					}
				}
				if bestGroup == nil || btf > bestBytesToFree {
					bestGroup = dg
					bestBytesToFree = btf
				}
			}

			if bestGroup != nil {
				diskCtx = &diskContextPayload{
					TotalBytes:   bestGroup.TotalBytes,
					UsedBytes:    bestGroup.UsedBytes,
					TargetPct:    bestGroup.TargetPct,
					ThresholdPct: bestGroup.ThresholdPct,
					BytesToFree:  bestBytesToFree,
				}
			}
		}

		slog.Debug("Preview: returning all evaluated items", "component", "api", "evaluatedCount", len(evaluated))

		return c.JSON(http.StatusOK, map[string]interface{}{
			"items":       evaluated,
			"diskContext": diskCtx,
		})
	})
}
