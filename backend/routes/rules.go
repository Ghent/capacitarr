package routes

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"

	"capacitarr/internal/cache"
	"capacitarr/internal/db"
	"capacitarr/internal/engine"
	"capacitarr/internal/integrations"
	"capacitarr/internal/logger"
)

// RuleValueCache is the package-level TTL cache for rule value lookups.
// Exported so that integration test/sync endpoints can invalidate it.
var RuleValueCache = cache.New(5 * time.Minute)

// RegisterRuleRoutes sets up the endpoints for managing preferences and custom rules
func RegisterRuleRoutes(protected *echo.Group, database *gorm.DB) {
	// ---------------------------------------------------------
	// PREFERENCE SET
	// ---------------------------------------------------------
	protected.GET("/preferences", func(c echo.Context) error {
		var pref db.PreferenceSet
		// Always return the first/only record, or implicitly create default
		if err := database.FirstOrCreate(&pref, db.PreferenceSet{ID: 1}).Error; err != nil {
			slog.Error("Failed to fetch preferences", "error", err)
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch preferences"})
		}
		return c.JSON(http.StatusOK, pref)
	})

	protected.PUT("/preferences", func(c echo.Context) error {
		var payload db.PreferenceSet
		if err := c.Bind(&payload); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request payload"})
		}
		// Force ID to 1 to ensure a single singleton record
		payload.ID = 1

		// Validate weight values (0-10)
		weights := []int{
			payload.WatchHistoryWeight, payload.LastWatchedWeight,
			payload.FileSizeWeight, payload.RatingWeight,
			payload.TimeInLibraryWeight, payload.AvailabilityWeight,
		}
		for _, w := range weights {
			if w < 0 || w > 10 {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": "Weight values must be between 0 and 10"})
			}
		}

		// Validate tiebreaker method
		validTiebreakers := map[string]bool{"size_desc": true, "size_asc": true, "name_asc": true, "oldest_first": true, "newest_first": true}
		if payload.TiebreakerMethod == "" {
			payload.TiebreakerMethod = "size_desc"
		}
		if !validTiebreakers[payload.TiebreakerMethod] {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Tiebreaker method must be size_desc, size_asc, name_asc, oldest_first, or newest_first"})
		}

		// Validate execution mode
		validModes := map[string]bool{"dry-run": true, "approval": true, "auto": true}
		if !validModes[payload.ExecutionMode] {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Execution mode must be dry-run, approval, or auto"})
		}

		// Validate log level
		validLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
		if !validLevels[payload.LogLevel] {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Log level must be debug, info, warn, or error"})
		}

		// Validate poll interval (minimum 30s, default 300s)
		if payload.PollIntervalSeconds < 30 {
			payload.PollIntervalSeconds = 300
		}

		if err := database.Save(&payload).Error; err != nil {
			slog.Error("Failed to update preferences", "error", err)
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to update preferences"})
		}

		// Apply dynamic log level
		logger.SetLevel(payload.LogLevel)

		return c.JSON(http.StatusOK, payload)
	})

	// ---------------------------------------------------------
	// RULE FIELD OPTIONS (dynamic based on integrations)
	// Accepts optional ?service_type=sonarr to filter fields.
	// Without the parameter, returns all fields (backward compat).
	// ---------------------------------------------------------
	protected.GET("/rule-fields", func(c echo.Context) error {
		serviceType := c.QueryParam("service_type")

		// Base fields available for all *arr integration types
		fields := []map[string]interface{}{
			{"field": "title", "label": "Title", "type": "string", "operators": []string{"==", "!=", "contains", "!contains"}},
			{"field": "quality", "label": "Quality Profile", "type": "string", "operators": []string{"==", "!="}},
			{"field": "tag", "label": "Tags", "type": "string", "operators": []string{"contains", "!contains"}},
			{"field": "genre", "label": "Genre", "type": "string", "operators": []string{"==", "!=", "contains", "!contains"}},
			{"field": "rating", "label": "Rating", "type": "number", "operators": []string{"==", "!=", ">", ">=", "<", "<="}},
			{"field": "sizebytes", "label": "Size (bytes)", "type": "number", "operators": []string{">", ">=", "<", "<="}},
			{"field": "timeinlibrary", "label": "Time in Library (days)", "type": "number", "operators": []string{">", ">=", "<", "<="}},
			{"field": "monitored", "label": "Monitored", "type": "boolean", "operators": []string{"=="}},
			{"field": "year", "label": "Year", "type": "number", "operators": []string{"==", "!=", ">", ">=", "<", "<="}},
			{"field": "language", "label": "Language", "type": "string", "operators": []string{"==", "!="}},
		}

		// When service_type is specified, add type-specific fields
		if serviceType == "sonarr" || serviceType == "" {
			// Sonarr-specific fields (TV)
			sonarrFields := []map[string]interface{}{
				{"field": "availability", "label": "Show Status", "type": "string", "operators": []string{"==", "!="}},
				{"field": "seasoncount", "label": "Season Count", "type": "number", "operators": []string{"==", "!=", ">", ">=", "<", "<="}},
				{"field": "episodecount", "label": "Episode Count", "type": "number", "operators": []string{"==", "!=", ">", ">=", "<", "<="}},
			}

			if serviceType == "sonarr" {
				fields = append(fields, sonarrFields...)
			} else {
				// No service_type filter: conditionally add based on enabled integrations
				var configs []db.IntegrationConfig
				database.Where("enabled = ?", true).Find(&configs)
				hasTV := false
				for _, cfg := range configs {
					if cfg.Type == "sonarr" {
						hasTV = true
						break
					}
				}
				if hasTV {
					fields = append(fields, sonarrFields...)
				}
			}
		}

		// Enrichment fields from Tautulli / Overseerr / media servers
		// These apply to all *arr services when the enrichment service is enabled
		if serviceType == "" {
			// No filter: check which enrichment services are enabled
			var configs []db.IntegrationConfig
			database.Where("enabled = ?", true).Find(&configs)
			hasTautulli := false
			hasOverseerr := false
			hasMediaServer := false
			for _, cfg := range configs {
				switch cfg.Type {
				case "tautulli":
					hasTautulli = true
				case "overseerr":
					hasOverseerr = true
				case "plex", "jellyfin", "emby":
					hasMediaServer = true
				}
			}
			if hasTautulli || hasMediaServer {
				fields = append(fields,
					map[string]interface{}{"field": "playcount", "label": "Play Count", "type": "number", "operators": []string{"==", "!=", ">", ">=", "<", "<="}},
				)
			}
			if hasOverseerr {
				fields = append(fields,
					map[string]interface{}{"field": "requested", "label": "Is Requested", "type": "boolean", "operators": []string{"=="}},
					map[string]interface{}{"field": "requestcount", "label": "Request Count", "type": "number", "operators": []string{"==", "!=", ">", ">=", "<", "<="}},
				)
			}
		} else {
			// service_type is specified — enrichment fields always available for *arr services
			arrTypes := map[string]bool{"sonarr": true, "radarr": true, "lidarr": true, "readarr": true}
			if arrTypes[serviceType] {
				var configs []db.IntegrationConfig
				database.Where("enabled = ?", true).Find(&configs)
				hasTautulli := false
				hasOverseerr := false
				hasMediaServer := false
				for _, cfg := range configs {
					switch cfg.Type {
					case "tautulli":
						hasTautulli = true
					case "overseerr":
						hasOverseerr = true
					case "plex", "jellyfin", "emby":
						hasMediaServer = true
					}
				}
				if hasTautulli || hasMediaServer {
					fields = append(fields,
						map[string]interface{}{"field": "playcount", "label": "Play Count", "type": "number", "operators": []string{"==", "!=", ">", ">=", "<", "<="}},
					)
				}
				if hasOverseerr {
					fields = append(fields,
						map[string]interface{}{"field": "requested", "label": "Is Requested", "type": "boolean", "operators": []string{"=="}},
						map[string]interface{}{"field": "requestcount", "label": "Request Count", "type": "number", "operators": []string{"==", "!=", ">", ">=", "<", "<="}},
					)
				}
			}
		}

		// Media Type field (always available)
		fields = append(fields,
			map[string]interface{}{"field": "type", "label": "Media Type", "type": "string", "operators": []string{"==", "!="}},
		)

		return c.JSON(http.StatusOK, fields)
	})

	// ---------------------------------------------------------
	// RULE VALUES — Autocomplete for rule value input
	// GET /api/v1/rule-values?integration_id=X&action=Y
	// Returns value options based on integration + action type.
	// ---------------------------------------------------------
	protected.GET("/rule-values", func(c echo.Context) error {
		integrationIDStr := c.QueryParam("integration_id")
		action := c.QueryParam("action")
		if integrationIDStr == "" || action == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "integration_id and action are required"})
		}

		integrationID, err := strconv.Atoi(integrationIDStr)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid integration_id"})
		}

		// Check cache first
		cacheKey := fmt.Sprintf("%d:%s", integrationID, action)
		if cached, ok := RuleValueCache.Get(cacheKey); ok {
			return c.JSON(http.StatusOK, cached)
		}

		// Handle static/built-in value types that don't need an API call
		switch action {
		case "availability": // Show Status
			result := map[string]interface{}{
				"type": "closed",
				"options": []integrations.NameValue{
					{Value: "continuing", Label: "Continuing"},
					{Value: "ended", Label: "Ended"},
					{Value: "upcoming", Label: "Upcoming"},
					{Value: "deleted", Label: "Deleted"},
				},
			}
			RuleValueCache.Set(cacheKey, result)
			return c.JSON(http.StatusOK, result)

		case "monitored", "requested": // Boolean fields
			result := map[string]interface{}{
				"type": "closed",
				"options": []integrations.NameValue{
					{Value: "true", Label: "Yes"},
					{Value: "false", Label: "No"},
				},
			}
			RuleValueCache.Set(cacheKey, result)
			return c.JSON(http.StatusOK, result)

		case "type": // Media Type
			result := map[string]interface{}{
				"type": "closed",
				"options": []integrations.NameValue{
					{Value: "movie", Label: "Movie"},
					{Value: "show", Label: "Show"},
					{Value: "season", Label: "Season"},
					{Value: "artist", Label: "Artist"},
					{Value: "book", Label: "Book"},
				},
			}
			RuleValueCache.Set(cacheKey, result)
			return c.JSON(http.StatusOK, result)

		// Free-text fields — return input metadata
		case "title":
			return c.JSON(http.StatusOK, map[string]interface{}{
				"type": "free", "inputType": "text", "placeholder": "e.g., Breaking Bad", "suffix": "",
			})
		case "rating":
			return c.JSON(http.StatusOK, map[string]interface{}{
				"type": "free", "inputType": "number", "placeholder": "e.g., 7.5", "suffix": "",
			})
		case "sizebytes":
			return c.JSON(http.StatusOK, map[string]interface{}{
				"type": "free", "inputType": "number", "placeholder": "e.g., 5368709120", "suffix": "bytes (≈ GB)",
			})
		case "timeinlibrary":
			return c.JSON(http.StatusOK, map[string]interface{}{
				"type": "free", "inputType": "number", "placeholder": "e.g., 30", "suffix": "days",
			})
		case "year":
			return c.JSON(http.StatusOK, map[string]interface{}{
				"type": "free", "inputType": "number", "placeholder": "e.g., 2020", "suffix": "",
			})
		case "seasoncount":
			return c.JSON(http.StatusOK, map[string]interface{}{
				"type": "free", "inputType": "number", "placeholder": "e.g., 5", "suffix": "",
			})
		case "episodecount":
			return c.JSON(http.StatusOK, map[string]interface{}{
				"type": "free", "inputType": "number", "placeholder": "e.g., 100", "suffix": "",
			})
		case "playcount":
			return c.JSON(http.StatusOK, map[string]interface{}{
				"type": "free", "inputType": "number", "placeholder": "e.g., 0", "suffix": "",
			})
		case "requestcount":
			return c.JSON(http.StatusOK, map[string]interface{}{
				"type": "free", "inputType": "number", "placeholder": "e.g., 3", "suffix": "",
			})
		}

		// Dynamic fields — require API call to the *arr service
		var cfg db.IntegrationConfig
		if err := database.First(&cfg, integrationID).Error; err != nil {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "Integration not found"})
		}

		// Create the appropriate client and check if it implements RuleValueFetcher
		client := CreateClient(cfg.Type, cfg.URL, cfg.APIKey)
		if client == nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Unsupported integration type for rule values"})
		}

		fetcher, ok := client.(integrations.RuleValueFetcher)
		if !ok {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Integration does not support rule value lookups"})
		}

		var result map[string]interface{}

		switch action {
		case "quality":
			profiles, err := fetcher.GetQualityProfiles()
			if err != nil {
				slog.Warn("Failed to fetch quality profiles for rule values", "integration_id", integrationID, "error", err)
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch quality profiles"})
			}
			result = map[string]interface{}{"type": "closed", "options": profiles}

		case "tag":
			tags, err := fetcher.GetTags()
			if err != nil {
				slog.Warn("Failed to fetch tags for rule values", "integration_id", integrationID, "error", err)
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch tags"})
			}
			result = map[string]interface{}{"type": "combobox", "suggestions": tags}

		case "genre":
			// Genre suggestions are free-form combobox with no API source
			result = map[string]interface{}{
				"type": "combobox",
				"suggestions": []integrations.NameValue{
					{Value: "Action", Label: "Action"},
					{Value: "Adventure", Label: "Adventure"},
					{Value: "Animation", Label: "Animation"},
					{Value: "Comedy", Label: "Comedy"},
					{Value: "Crime", Label: "Crime"},
					{Value: "Documentary", Label: "Documentary"},
					{Value: "Drama", Label: "Drama"},
					{Value: "Fantasy", Label: "Fantasy"},
					{Value: "Horror", Label: "Horror"},
					{Value: "Mystery", Label: "Mystery"},
					{Value: "Romance", Label: "Romance"},
					{Value: "Sci-Fi", Label: "Sci-Fi"},
					{Value: "Thriller", Label: "Thriller"},
				},
			}

		case "language":
			langs, err := fetcher.GetLanguages()
			if err != nil {
				slog.Warn("Failed to fetch languages for rule values", "integration_id", integrationID, "error", err)
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch languages"})
			}
			if langs == nil {
				// Service doesn't support language lookup — return free input
				return c.JSON(http.StatusOK, map[string]interface{}{
					"type": "free", "inputType": "text", "placeholder": "e.g., English", "suffix": "",
				})
			}
			result = map[string]interface{}{"type": "closed", "options": langs}

		default:
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Unknown action: " + action})
		}

		RuleValueCache.Set(cacheKey, result)
		return c.JSON(http.StatusOK, result)
	})

	// ---------------------------------------------------------
	// CUSTOM RULES (protection/targeting)
	// ---------------------------------------------------------
	protected.GET("/protections", func(c echo.Context) error {
		var rules []db.ProtectionRule
		if err := database.Find(&rules).Error; err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch custom rules"})
		}
		return c.JSON(http.StatusOK, rules)
	})

	protected.POST("/protections", func(c echo.Context) error {
		var newRule db.ProtectionRule
		if err := c.Bind(&newRule); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request payload"})
		}

		// Validate required fields for the new payload shape
		if newRule.Field == "" || newRule.Operator == "" || newRule.Value == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Field, Operator, and Value are required"})
		}

		// New payload: require effect field
		if newRule.Effect != "" {
			validEffects := map[string]bool{
				"always_keep": true, "prefer_keep": true, "lean_keep": true,
				"lean_remove": true, "prefer_remove": true, "always_remove": true,
			}
			if !validEffects[newRule.Effect] {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": "Effect must be one of: always_keep, prefer_keep, lean_keep, lean_remove, prefer_remove, always_remove"})
			}
		} else if newRule.Type != "" && newRule.Intensity != "" {
			// Legacy payload: type + intensity — auto-populate effect
			switch {
			case newRule.Type == "protect" && newRule.Intensity == "absolute":
				newRule.Effect = "always_keep"
			case newRule.Type == "protect" && newRule.Intensity == "strong":
				newRule.Effect = "prefer_keep"
			case newRule.Type == "protect":
				newRule.Effect = "lean_keep"
			case newRule.Type == "target" && newRule.Intensity == "absolute":
				newRule.Effect = "always_remove"
			case newRule.Type == "target" && newRule.Intensity == "strong":
				newRule.Effect = "prefer_remove"
			case newRule.Type == "target":
				newRule.Effect = "lean_remove"
			}
		} else {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Either 'effect' or both 'type' and 'intensity' are required"})
		}

		if err := database.Create(&newRule).Error; err != nil {
			slog.Error("Failed to create custom rule", "error", err)
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to create rule"})
		}
		return c.JSON(http.StatusCreated, newRule)
	})

	protected.DELETE("/protections/:id", func(c echo.Context) error {
		id := c.Param("id")
		if err := database.Delete(&db.ProtectionRule{}, id).Error; err != nil {
			slog.Error("Failed to delete custom rule", "id", id, "error", err)
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to delete rule"})
		}
		return c.NoContent(http.StatusNoContent)
	})

	// ---------------------------------------------------------
	// LIVE PREVIEW
	// ---------------------------------------------------------
	protected.GET("/preview", func(c echo.Context) error {
		var configs []db.IntegrationConfig
		if err := database.Where("enabled = ?", true).Find(&configs).Error; err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to load integrations"})
		}

		var allItems []integrations.MediaItem
		slog.Info("Preview: fetching media from integrations", "configCount", len(configs))
		for _, cfg := range configs {
			slog.Info("Preview: checking integration", "name", cfg.Name, "type", cfg.Type, "enabled", cfg.Enabled)
			if cfg.Type == "plex" || cfg.Type == "tautulli" || cfg.Type == "overseerr" || cfg.Type == "jellyfin" || cfg.Type == "emby" {
				slog.Info("Preview: skipping non-arr integration", "type", cfg.Type)
				continue
			}
			client := CreateClient(cfg.Type, cfg.URL, cfg.APIKey)
			if client == nil {
				slog.Warn("Preview: CreateClient returned nil", "type", cfg.Type)
				continue
			}
			items, err := client.GetMediaItems()
			if err != nil {
				slog.Warn("Preview: media fetch failed", "integration", cfg.Name, "type", cfg.Type, "error", err)
				continue
			}
			slog.Info("Preview: fetched items", "integration", cfg.Name, "type", cfg.Type, "count", len(items))
			for i := range items {
				items[i].IntegrationID = cfg.ID
			}
			allItems = append(allItems, items...)
		}
		slog.Info("Preview: total items collected", "count", len(allItems))

		var prefs db.PreferenceSet
		database.FirstOrCreate(&prefs, db.PreferenceSet{ID: 1})

		var rules []db.ProtectionRule
		database.Find(&rules)

		evaluated := engine.EvaluateMedia(allItems, prefs, rules)
		slog.Info("Preview: evaluated items", "count", len(evaluated))

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

		slog.Info("Preview: returning all evaluated items", "evaluatedCount", len(evaluated))

		return c.JSON(http.StatusOK, map[string]interface{}{
			"items":       evaluated,
			"diskContext": diskCtx,
		})
	})
}
