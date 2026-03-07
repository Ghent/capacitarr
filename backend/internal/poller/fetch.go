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

		// Tautulli is an enrichment-only service, not a full Integration
		if cfg.Type == "tautulli" {
			result.enrichment.Tautulli = integrations.NewTautulliClient(cfg.URL, cfg.APIKey)
			if err := result.enrichment.Tautulli.TestConnection(); err != nil {
				slog.Warn("Tautulli connection failed", "component", "poller", "operation", "tautulli_connect", "integration", cfg.Name, "error", err)
				_ = integrationSvc.UpdateSyncStatus(cfg.ID, nil, err.Error())
			} else {
				_ = integrationSvc.UpdateSyncStatus(cfg.ID, &now, "")
				slog.Debug("Tautulli connected", "component", "poller", "integration", cfg.Name, "duration", time.Since(fetchStart).String())
			}
			continue
		}

		// Overseerr is an enrichment-only service for tracking media requests
		if cfg.Type == "overseerr" {
			result.enrichment.Overseerr = integrations.NewOverseerrClient(cfg.URL, cfg.APIKey)
			if err := result.enrichment.Overseerr.TestConnection(); err != nil {
				slog.Warn("Overseerr connection failed", "component", "poller", "operation", "overseerr_connect", "integration", cfg.Name, "error", err)
				_ = integrationSvc.UpdateSyncStatus(cfg.ID, nil, err.Error())
			} else {
				_ = integrationSvc.UpdateSyncStatus(cfg.ID, &now, "")
				slog.Debug("Overseerr connected", "component", "poller", "integration", cfg.Name, "duration", time.Since(fetchStart).String())
			}
			continue
		}

		// Jellyfin is an enrichment-only service for watch history
		if cfg.Type == "jellyfin" {
			result.enrichment.Jellyfin = integrations.NewJellyfinClient(cfg.URL, cfg.APIKey)
			if err := result.enrichment.Jellyfin.TestConnection(); err != nil {
				slog.Warn("Jellyfin connection failed", "component", "poller", "operation", "jellyfin_connect", "integration", cfg.Name, "error", err)
				_ = integrationSvc.UpdateSyncStatus(cfg.ID, nil, err.Error())
			} else {
				_ = integrationSvc.UpdateSyncStatus(cfg.ID, &now, "")
				slog.Debug("Jellyfin connected", "component", "poller", "integration", cfg.Name, "duration", time.Since(fetchStart).String())
			}
			continue
		}

		// Emby is an enrichment-only service for watch history
		if cfg.Type == "emby" {
			result.enrichment.Emby = integrations.NewEmbyClient(cfg.URL, cfg.APIKey)
			if err := result.enrichment.Emby.TestConnection(); err != nil {
				slog.Warn("Emby connection failed", "component", "poller", "operation", "emby_connect", "integration", cfg.Name, "error", err)
				_ = integrationSvc.UpdateSyncStatus(cfg.ID, nil, err.Error())
			} else {
				_ = integrationSvc.UpdateSyncStatus(cfg.ID, &now, "")
				slog.Debug("Emby connected", "component", "poller", "integration", cfg.Name, "duration", time.Since(fetchStart).String())
			}
			continue
		}

		// Plex is an enrichment-only service for watch history cross-referencing
		if cfg.Type == "plex" {
			result.enrichment.Plex = integrations.NewPlexClient(cfg.URL, cfg.APIKey)
			if err := result.enrichment.Plex.TestConnection(); err != nil {
				slog.Warn("Plex connection failed", "component", "poller",
					"operation", "plex_connect", "integration", cfg.Name, "error", err)
				_ = integrationSvc.UpdateSyncStatus(cfg.ID, nil, err.Error())
			} else {
				_ = integrationSvc.UpdateSyncStatus(cfg.ID, &now, "")
				slog.Debug("Plex connected for enrichment", "component", "poller",
					"integration", cfg.Name, "duration", time.Since(fetchStart).String())
			}
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
			result.rootFolders[f] = true
			slog.Debug("Root folder found", "component", "poller",
				"integration", cfg.Name, "path", f)
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

		// Collect all disk entries
		for _, d := range disks {
			if d.Path == "" {
				continue
			}
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
