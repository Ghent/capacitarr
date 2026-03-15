package poller

import (
	"log/slog"
	"time"

	"capacitarr/internal/db"
	"capacitarr/internal/integrations"
	"capacitarr/internal/services"
)

// fetchResult holds the aggregated results from fetching all integration data.
type fetchResult struct {
	allItems       []integrations.MediaItem
	serviceClients map[uint]integrations.Integration
	rootFolders    map[string]bool
	diskMap        map[string]integrations.DiskSpace
	enrichment     integrations.EnrichmentClients
}

// connectEnrichment tests an enrichment client's connection and updates sync status.
// Returns true on success, false on failure. Logs are emitted at appropriate levels.
func connectEnrichment(cfg db.IntegrationConfig, testFn func() error, integrationSvc *services.IntegrationService) bool {
	start := time.Now()
	now := time.Now()
	if err := testFn(); err != nil {
		slog.Warn("Enrichment connection failed", "component", "poller",
			"operation", cfg.Type+"_connect", "integration", cfg.Name, "error", err)
		_ = integrationSvc.UpdateSyncStatus(cfg.ID, nil, err.Error())
		return false
	}
	_ = integrationSvc.UpdateSyncStatus(cfg.ID, &now, "")
	slog.Debug("Enrichment connected", "component", "poller",
		"integration", cfg.Name, "type", cfg.Type, "duration", time.Since(start).String())
	return true
}

// fetchAllIntegrations queries all enabled integrations and collects media items,
// root folders, disk space info, and enrichment clients.
func fetchAllIntegrations(configs []db.IntegrationConfig, integrationSvc *services.IntegrationService) fetchResult {
	result := fetchResult{
		serviceClients: make(map[uint]integrations.Integration),
		rootFolders:    make(map[string]bool),
		diskMap:        make(map[string]integrations.DiskSpace),
	}

	for _, cfg := range configs {
		fetchStart := time.Now()
		now := time.Now()

		// Enrichment-only services — create client, test connection, continue
		switch cfg.Type {
		case "tautulli":
			result.enrichment.Tautulli = integrations.NewTautulliClient(cfg.URL, cfg.APIKey)
			connectEnrichment(cfg, result.enrichment.Tautulli.TestConnection, integrationSvc)
			continue
		case "overseerr":
			result.enrichment.Overseerr = integrations.NewOverseerrClient(cfg.URL, cfg.APIKey)
			connectEnrichment(cfg, result.enrichment.Overseerr.TestConnection, integrationSvc)
			continue
		case "jellyfin":
			result.enrichment.Jellyfin = integrations.NewJellyfinClient(cfg.URL, cfg.APIKey)
			connectEnrichment(cfg, result.enrichment.Jellyfin.TestConnection, integrationSvc)
			continue
		case "emby":
			result.enrichment.Emby = integrations.NewEmbyClient(cfg.URL, cfg.APIKey)
			connectEnrichment(cfg, result.enrichment.Emby.TestConnection, integrationSvc)
			continue
		case "plex":
			result.enrichment.Plex = integrations.NewPlexClient(cfg.URL, cfg.APIKey)
			connectEnrichment(cfg, result.enrichment.Plex.TestConnection, integrationSvc)
			continue
		}

		client := integrations.NewClient(cfg.Type, cfg.URL, cfg.APIKey)
		if client == nil {
			slog.Debug("No client for integration type", "component", "poller", "type", cfg.Type, "integration", cfg.Name)
			continue
		}
		result.serviceClients[cfg.ID] = client

		// Fetch media items for per-integration usage tracking (Sonarr/Radarr only)
		slog.Debug("Fetching media items", "component", "poller", "integration", cfg.Name, "type", cfg.Type)
		items, err := client.GetMediaItems()
		if err != nil {
			slog.Warn("Media items fetch failed", "component", "poller", "operation", "fetch_media",
				"integration", cfg.Name, "type", cfg.Type, "error", err)
		} else {
			for i := range items {
				items[i].IntegrationID = cfg.ID
				items[i].Path = normalizePath(items[i].Path)
			}
			result.allItems = append(result.allItems, items...)

			var totalSize int64
			// For Sonarr, only count show-level items to avoid double-counting seasons
			for _, item := range items {
				if cfg.Type == "sonarr" && item.Type != integrations.MediaTypeShow {
					continue
				}
				totalSize += item.SizeBytes
			}
			mediaCount := len(items)
			if cfg.Type == "sonarr" {
				// Count unique shows only
				mediaCount = 0
				for _, item := range items {
					if item.Type == integrations.MediaTypeShow {
						mediaCount++
					}
				}
			}
			_ = integrationSvc.UpdateMediaStats(cfg.ID, totalSize, mediaCount)
			slog.Debug("Media items fetched", "component", "poller",
				"integration", cfg.Name, "type", cfg.Type,
				"itemCount", len(items), "duration", time.Since(fetchStart).String())
		}

		// Get root folders (Sonarr/Radarr only)
		folders, err := client.GetRootFolders()
		if err != nil {
			slog.Warn("Root folder fetch failed", "component", "poller", "operation", "fetch_root_folders",
				"integration", cfg.Name, "type", cfg.Type, "error", err)
		}
		for _, f := range folders {
			normalized := normalizePath(f)
			result.rootFolders[normalized] = true
			if normalized != f {
				slog.Debug("Path normalized", "component", "poller",
					"integration", cfg.Name, "type", "rootFolder",
					"original", f, "normalized", normalized)
			}
			slog.Debug("Root folder found", "component", "poller",
				"integration", cfg.Name, "path", normalized)
		}

		// Get disk space
		disks, err := client.GetDiskSpace()
		if err != nil {
			slog.Warn("Disk space fetch failed", "component", "poller", "operation", "fetch_disk_space",
				"integration", cfg.Name, "type", cfg.Type, "error", err)
			_ = integrationSvc.UpdateSyncStatus(cfg.ID, nil, err.Error())
			continue
		}

		// Update last sync time, clear error
		_ = integrationSvc.UpdateSyncStatus(cfg.ID, &now, "")

		// Collect all disk entries — normalize paths for cross-platform compatibility
		for _, d := range disks {
			if d.Path == "" {
				continue
			}
			originalPath := d.Path
			d.Path = normalizePath(d.Path)
			if d.Path != originalPath {
				slog.Debug("Path normalized", "component", "poller",
					"integration", cfg.Name, "type", "diskSpace",
					"original", originalPath, "normalized", d.Path)
			}
			slog.Debug("Disk space entry found", "component", "poller",
				"integration", cfg.Name, "path", d.Path,
				"totalBytes", d.TotalBytes, "freeBytes", d.FreeBytes)
			if existing, ok := result.diskMap[d.Path]; ok {
				if d.TotalBytes > existing.TotalBytes {
					result.diskMap[d.Path] = d
				}
			} else {
				result.diskMap[d.Path] = d
			}
		}
	}

	return result
}
