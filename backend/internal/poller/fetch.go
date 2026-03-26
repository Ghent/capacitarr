package poller

import (
	"log/slog"
	"time"

	"capacitarr/internal/integrations"
	"capacitarr/internal/services"
)

// fetchResult holds the aggregated results from fetching all integration data.
type fetchResult struct {
	allItems          []integrations.MediaItem
	rootFolders       map[string]bool
	diskMap           map[string]integrations.DiskSpace
	mountIntegrations map[string][]uint // mount path → integration IDs that reported it
	registry          *integrations.IntegrationRegistry
	pipeline          *integrations.EnrichmentPipeline
	brokenTypes       []string // integration types that failed connection testing
}

// fetchAllIntegrations builds an IntegrationRegistry, fetches media items from
// all MediaSources, fetches disk space from all DiskReporters, and constructs
// the enrichment pipeline from discovered capabilities.
func fetchAllIntegrations(integrationSvc *services.IntegrationService) fetchResult {
	result := fetchResult{
		rootFolders:       make(map[string]bool),
		diskMap:           make(map[string]integrations.DiskSpace),
		mountIntegrations: make(map[string][]uint),
	}

	// Build the capability-based registry using factory pattern
	registry, err := integrationSvc.BuildIntegrationRegistry()
	if err != nil {
		slog.Error("Failed to build integration registry", "component", "poller", "error", err)
		return result
	}
	result.registry = registry

	// Test all connectable integrations and update sync status.
	// Detect error→healthy transitions to publish recovery events.
	// Collect broken integration types for EvaluationContext.
	brokenSet := make(map[string]bool)
	for id, conn := range registry.Connectors() {
		now := time.Now()
		if connErr := conn.TestConnection(); connErr != nil {
			slog.Warn("Integration connection failed", "component", "poller",
				"integrationID", id, "error", connErr)
			_ = integrationSvc.UpdateSyncStatus(id, nil, connErr.Error())

			// Look up the integration type for this broken connector
			if cfg, cfgErr := integrationSvc.GetByID(id); cfgErr == nil {
				brokenSet[cfg.Type] = true
			}
			continue
		}

		// Check if this integration was previously in an error state
		integrationSvc.PublishRecoveryIfNeeded(id)

		_ = integrationSvc.UpdateSyncStatus(id, &now, "")
	}
	for t := range brokenSet {
		result.brokenTypes = append(result.brokenTypes, t)
	}

	// Fetch media items from all MediaSources
	for id, source := range registry.MediaSources() {
		fetchStart := time.Now()
		items, fetchErr := source.GetMediaItems()
		if fetchErr != nil {
			slog.Warn("Media items fetch failed", "component", "poller",
				"integrationID", id, "error", fetchErr)
			continue
		}
		for i := range items {
			items[i].IntegrationID = id
			items[i].Path = normalizePath(items[i].Path)
			// Attribute native collection data to the item's own integration.
			// Media-server enrichment will add its own sources later.
			if len(items[i].Collections) > 0 {
				items[i].CollectionSources = make(map[string]uint, len(items[i].Collections))
				for _, col := range items[i].Collections {
					items[i].CollectionSources[col] = id
				}
			}
		}

		// When ShowLevelOnly is enabled for this integration, drop season-level
		// items so only show-level entries are scored and queued.
		cfg, cfgErr := integrationSvc.GetByID(id)
		if cfgErr == nil && cfg.ShowLevelOnly {
			originalCount := len(items)
			filtered := items[:0]
			for _, item := range items {
				if item.Type != integrations.MediaTypeSeason {
					filtered = append(filtered, item)
				}
			}
			items = filtered
			slog.Debug("ShowLevelOnly filter applied", "component", "poller",
				"integrationID", id, "removedSeasons", originalCount-len(items))
		}

		result.allItems = append(result.allItems, items...)

		// Update media stats via service
		var totalSize int64
		mediaCount := len(items)

		// Determine integration type by checking if it's a show-type source
		hasShows := false
		for _, item := range items {
			if item.Type == integrations.MediaTypeShow {
				hasShows = true
				break
			}
		}

		if hasShows {
			// Sonarr-type: count only show-level items to avoid double-counting seasons
			mediaCount = 0
			for _, item := range items {
				if item.Type == integrations.MediaTypeShow {
					mediaCount++
					totalSize += item.SizeBytes
				}
			}
		} else {
			for _, item := range items {
				totalSize += item.SizeBytes
			}
		}
		_ = integrationSvc.UpdateMediaStats(id, totalSize, mediaCount)
		slog.Debug("Media items fetched", "component", "poller",
			"integrationID", id, "itemCount", len(items),
			"duration", time.Since(fetchStart).String())
	}

	// Fetch root folders and disk space from all DiskReporters
	for id, reporter := range registry.DiskReporters() {
		folders, err := reporter.GetRootFolders()
		if err != nil {
			slog.Warn("Root folder fetch failed", "component", "poller",
				"integrationID", id, "error", err)
		}
		for _, f := range folders {
			normalized := normalizePath(f)
			result.rootFolders[normalized] = true
			slog.Debug("Root folder found", "component", "poller",
				"integrationID", id, "path", normalized)
		}

		disks, err := reporter.GetDiskSpace()
		if err != nil {
			slog.Warn("Disk space fetch failed", "component", "poller",
				"integrationID", id, "error", err)
			continue
		}

		for _, d := range disks {
			if d.Path == "" {
				continue
			}
			d.Path = normalizePath(d.Path)
			slog.Debug("Disk space entry found", "component", "poller",
				"integrationID", id, "path", d.Path,
				"totalBytes", d.TotalBytes, "freeBytes", d.FreeBytes)
			if existing, ok := result.diskMap[d.Path]; ok {
				if d.TotalBytes > existing.TotalBytes {
					result.diskMap[d.Path] = d
				}
			} else {
				result.diskMap[d.Path] = d
			}
			result.mountIntegrations[d.Path] = append(result.mountIntegrations[d.Path], id)
		}
	}

	// Build enrichment pipeline from registry capabilities
	pipeline := integrations.BuildEnrichmentPipeline(registry)

	// Build TMDb→RatingKey map from Plex for Tautulli enrichment.
	// Tautulli queries by Plex ratingKey, but *arr items have TMDb IDs.
	// This map bridges the gap. Built per poll cycle — not cached.
	tmdbToRatingKey := make(map[int]string)
	for id := range registry.Connectors() {
		if plex, ok := registry.PlexClient(id); ok {
			plexMap, mapErr := plex.GetTMDbToRatingKeyMap()
			if mapErr != nil {
				slog.Warn("Failed to build TMDb→RatingKey map from Plex",
					"component", "poller", "integrationID", id, "error", mapErr)
				continue
			}
			for tmdbID, ratingKey := range plexMap {
				tmdbToRatingKey[tmdbID] = ratingKey
			}
			slog.Debug("Built TMDb→RatingKey map from Plex", "component", "poller",
				"integrationID", id, "mappings", len(plexMap))
		}
	}

	integrations.RegisterTautulliEnrichers(pipeline, registry, tmdbToRatingKey)

	// Build Jellyfin Item ID → TMDb ID map for Jellystat enrichment.
	// Jellystat stores items by Jellyfin Item ID, but *arr items use TMDb IDs.
	// This map bridges the gap. Built per poll cycle — not cached.
	jellyfinIDToTMDbID := make(map[string]int)
	for id := range registry.Connectors() {
		if jf, ok := registry.JellyfinClient(id); ok {
			jfMap, mapErr := jf.GetItemIDToTMDbIDMap()
			if mapErr != nil {
				slog.Warn("Failed to build Jellyfin ID→TMDb ID map",
					"component", "poller", "integrationID", id, "error", mapErr)
				continue
			}
			for itemID, tmdbID := range jfMap {
				jellyfinIDToTMDbID[itemID] = tmdbID
			}
			slog.Debug("Built Jellyfin ID→TMDb ID map", "component", "poller",
				"integrationID", id, "mappings", len(jfMap))
		}
	}

	integrations.RegisterJellystatEnrichers(pipeline, registry, jellyfinIDToTMDbID)

	// Build Emby Item ID → TMDb ID map for Tracearr enrichment.
	// Emby items use Emby Item IDs as rating keys in Tracearr.
	embyIDToTMDbID := make(map[string]int)
	for id := range registry.Connectors() {
		if emby, ok := registry.EmbyClient(id); ok {
			embyMap, mapErr := emby.GetItemIDToTMDbIDMap()
			if mapErr != nil {
				slog.Warn("Failed to build Emby ID→TMDb ID map",
					"component", "poller", "integrationID", id, "error", mapErr)
				continue
			}
			for itemID, tmdbID := range embyMap {
				embyIDToTMDbID[itemID] = tmdbID
			}
			slog.Debug("Built Emby ID→TMDb ID map", "component", "poller",
				"integrationID", id, "mappings", len(embyMap))
		}
	}

	// Build unified ratingKey→TMDb ID map for Tracearr enrichment.
	// Tracearr items use the media server's internal item ID as rating_key.
	ratingKeyToTMDbID := make(map[string]int)
	for tmdbID, ratingKey := range tmdbToRatingKey {
		ratingKeyToTMDbID[ratingKey] = tmdbID
	}
	for itemID, tmdbID := range jellyfinIDToTMDbID {
		ratingKeyToTMDbID[itemID] = tmdbID
	}
	for itemID, tmdbID := range embyIDToTMDbID {
		ratingKeyToTMDbID[itemID] = tmdbID
	}

	integrations.RegisterTracearrEnrichers(pipeline, registry, ratingKeyToTMDbID)
	result.pipeline = pipeline

	return result
}
