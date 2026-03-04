package poller

import (
	"log/slog"
	"strings"
	"time"

	"capacitarr/internal/db"
	"capacitarr/internal/integrations"
)

// enrichmentClients holds optional enrichment-only service clients discovered
// during the fetch phase. These are not full Integration implementations.
type enrichmentClients struct {
	tautulli  *integrations.TautulliClient
	overseerr *integrations.OverseerrClient
	jellyfin  *integrations.JellyfinClient
	emby      *integrations.EmbyClient
}

// fetchResult holds the aggregated results from fetching all integration data.
type fetchResult struct {
	allItems       []integrations.MediaItem
	serviceClients map[uint]integrations.Integration
	rootFolders    map[string]bool
	diskMap        map[string]integrations.DiskSpace
	enrichment     enrichmentClients
}

// fetchAllIntegrations queries all enabled integrations and collects media items,
// root folders, disk space info, and enrichment clients.
func fetchAllIntegrations(configs []db.IntegrationConfig) fetchResult {
	result := fetchResult{
		serviceClients: make(map[uint]integrations.Integration),
		rootFolders:    make(map[string]bool),
		diskMap:        make(map[string]integrations.DiskSpace),
	}

	for _, cfg := range configs {
		fetchStart := time.Now()

		// Tautulli is an enrichment-only service, not a full Integration
		if cfg.Type == "tautulli" {
			result.enrichment.tautulli = integrations.NewTautulliClient(cfg.URL, cfg.APIKey)
			now := time.Now()
			if err := result.enrichment.tautulli.TestConnection(); err != nil {
				slog.Warn("Tautulli connection failed", "component", "poller", "operation", "tautulli_connect", "integration", cfg.Name, "error", err)
				db.DB.Model(&cfg).Updates(map[string]interface{}{
					"last_error": err.Error(),
				})
			} else {
				db.DB.Model(&cfg).Updates(map[string]interface{}{
					"last_sync":  &now,
					"last_error": "",
				})
				slog.Debug("Tautulli connected", "component", "poller", "integration", cfg.Name, "duration", time.Since(fetchStart).String())
			}
			continue
		}

		// Overseerr is an enrichment-only service for tracking media requests
		if cfg.Type == "overseerr" {
			result.enrichment.overseerr = integrations.NewOverseerrClient(cfg.URL, cfg.APIKey)
			now := time.Now()
			if err := result.enrichment.overseerr.TestConnection(); err != nil {
				slog.Warn("Overseerr connection failed", "component", "poller", "operation", "overseerr_connect", "integration", cfg.Name, "error", err)
				db.DB.Model(&cfg).Updates(map[string]interface{}{
					"last_error": err.Error(),
				})
			} else {
				db.DB.Model(&cfg).Updates(map[string]interface{}{
					"last_sync":  &now,
					"last_error": "",
				})
				slog.Debug("Overseerr connected", "component", "poller", "integration", cfg.Name, "duration", time.Since(fetchStart).String())
			}
			continue
		}

		// Jellyfin is an enrichment-only service for watch history
		if cfg.Type == "jellyfin" {
			result.enrichment.jellyfin = integrations.NewJellyfinClient(cfg.URL, cfg.APIKey)
			now := time.Now()
			if err := result.enrichment.jellyfin.TestConnection(); err != nil {
				slog.Warn("Jellyfin connection failed", "component", "poller", "operation", "jellyfin_connect", "integration", cfg.Name, "error", err)
				db.DB.Model(&cfg).Updates(map[string]interface{}{
					"last_error": err.Error(),
				})
			} else {
				db.DB.Model(&cfg).Updates(map[string]interface{}{
					"last_sync":  &now,
					"last_error": "",
				})
				slog.Debug("Jellyfin connected", "component", "poller", "integration", cfg.Name, "duration", time.Since(fetchStart).String())
			}
			continue
		}

		// Emby is an enrichment-only service for watch history
		if cfg.Type == "emby" {
			result.enrichment.emby = integrations.NewEmbyClient(cfg.URL, cfg.APIKey)
			now := time.Now()
			if err := result.enrichment.emby.TestConnection(); err != nil {
				slog.Warn("Emby connection failed", "component", "poller", "operation", "emby_connect", "integration", cfg.Name, "error", err)
				db.DB.Model(&cfg).Updates(map[string]interface{}{
					"last_error": err.Error(),
				})
			} else {
				db.DB.Model(&cfg).Updates(map[string]interface{}{
					"last_sync":  &now,
					"last_error": "",
				})
				slog.Debug("Emby connected", "component", "poller", "integration", cfg.Name, "duration", time.Since(fetchStart).String())
			}
			continue
		}

		client := CreateClient(cfg.Type, cfg.URL, cfg.APIKey)
		if client == nil {
			slog.Debug("No client for integration type", "component", "poller", "type", cfg.Type, "integration", cfg.Name)
			continue
		}
		result.serviceClients[cfg.ID] = client

		if cfg.Type == "plex" {
			// Plex is only used for protection rules, not disk usage tracking
			now := time.Now()
			db.DB.Model(&cfg).Updates(map[string]interface{}{
				"last_sync":  &now,
				"last_error": "",
			})
			slog.Debug("Plex synced (protection rules only)", "component", "poller", "integration", cfg.Name)
			continue
		}

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
			db.DB.Model(&cfg).Updates(map[string]interface{}{
				"media_size_bytes": totalSize,
				"media_count":      mediaCount,
			})
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
			db.DB.Model(&cfg).Updates(map[string]interface{}{
				"last_error": err.Error(),
			})
			continue
		}

		// Update last sync time, clear error
		now := time.Now()
		db.DB.Model(&cfg).Updates(map[string]interface{}{
			"last_sync":  &now,
			"last_error": "",
		})

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

// enrichItems applies watch history and request data from enrichment services
// (Tautulli, Overseerr, Jellyfin, Emby) to the collected media items.
func enrichItems(items []integrations.MediaItem, ec enrichmentClients) {
	// ─── Enrichment: Tautulli watch history ──────────────────────────────────
	if ec.tautulli != nil && len(items) > 0 {
		slog.Info("Enriching items with Tautulli watch data", "component", "poller", "itemCount", len(items))
		for i := range items {
			item := &items[i]
			if item.ExternalID == "" {
				continue
			}
			var watchData *integrations.TautulliWatchData
			var err error
			if item.Type == integrations.MediaTypeShow {
				watchData, err = ec.tautulli.GetShowWatchHistory(item.ExternalID)
			} else {
				watchData, err = ec.tautulli.GetWatchHistory(item.ExternalID)
			}
			if err != nil {
				slog.Debug("Tautulli enrichment failed", "component", "poller", "title", item.Title, "error", err)
				continue
			}
			if watchData != nil {
				item.PlayCount = watchData.PlayCount
				item.LastPlayed = watchData.LastPlayed
				if len(watchData.Users) > 0 {
					item.WatchedByUsers = watchData.Users
				}
			}
		}
	}

	// ─── Enrichment: Overseerr request data ──────────────────────────────────
	if ec.overseerr != nil && len(items) > 0 {
		slog.Info("Enriching items with Overseerr request data", "component", "poller", "itemCount", len(items))
		requests, err := ec.overseerr.GetRequestedMedia()
		if err != nil {
			slog.Warn("Failed to fetch Overseerr requests", "component", "poller", "operation", "fetch_overseerr", "error", err)
		} else {
			// Build lookup by TMDb ID
			requestMap := make(map[int]integrations.OverseerrMediaRequest)
			for _, req := range requests {
				requestMap[req.TMDbID] = req
			}
			matched := 0
			for i := range items {
				item := &items[i]
				if item.TMDbID > 0 {
					if req, ok := requestMap[item.TMDbID]; ok {
						item.IsRequested = true
						item.RequestedBy = req.RequestedBy
						item.RequestCount = 1
						matched++
					}
				}
			}
			slog.Debug("Overseerr enrichment complete", "component", "poller", "requests", len(requests), "matched", matched)
		}
	}

	// ─── Enrichment: Jellyfin watch history ─────────────────────────────────
	if ec.jellyfin != nil && len(items) > 0 {
		slog.Info("Enriching items with Jellyfin watch data", "component", "poller", "itemCount", len(items))
		userID, err := ec.jellyfin.GetAdminUserID()
		if err != nil {
			slog.Warn("Failed to get Jellyfin admin user", "component", "poller", "operation", "jellyfin_admin_user", "error", err)
		} else {
			watchMap, err := ec.jellyfin.GetBulkWatchData(userID)
			if err != nil {
				slog.Warn("Failed to fetch Jellyfin watch data", "component", "poller", "operation", "fetch_jellyfin_watch", "error", err)
			} else {
				matched := 0
				for i := range items {
					item := &items[i]
					// Match by normalized title (show title for seasons, direct title otherwise)
					titleKey := strings.ToLower(strings.TrimSpace(item.Title))
					if item.ShowTitle != "" {
						titleKey = strings.ToLower(strings.TrimSpace(item.ShowTitle))
					}
					if wd, ok := watchMap[titleKey]; ok {
						// Only enrich if we don't already have watch data (Tautulli takes priority)
						if item.PlayCount == 0 {
							item.PlayCount = wd.PlayCount
							item.LastPlayed = wd.LastPlayed
							matched++
						}
					}
				}
				slog.Info("Jellyfin enrichment complete", "component", "poller", "libraryItems", len(watchMap), "matched", matched)
			}
		}
	}

	// ─── Enrichment: Emby watch history ─────────────────────────────────────
	if ec.emby != nil && len(items) > 0 {
		slog.Info("Enriching items with Emby watch data", "component", "poller", "itemCount", len(items))
		userID, err := ec.emby.GetAdminUserID()
		if err != nil {
			slog.Warn("Failed to get Emby admin user", "component", "poller", "operation", "emby_admin_user", "error", err)
		} else {
			watchMap, err := ec.emby.GetBulkWatchData(userID)
			if err != nil {
				slog.Warn("Failed to fetch Emby watch data", "component", "poller", "operation", "fetch_emby_watch", "error", err)
			} else {
				matched := 0
				for i := range items {
					item := &items[i]
					// Match by normalized title (show title for seasons, direct title otherwise)
					titleKey := strings.ToLower(strings.TrimSpace(item.Title))
					if item.ShowTitle != "" {
						titleKey = strings.ToLower(strings.TrimSpace(item.ShowTitle))
					}
					if wd, ok := watchMap[titleKey]; ok {
						// Only enrich if we don't already have watch data
						if item.PlayCount == 0 {
							item.PlayCount = wd.PlayCount
							item.LastPlayed = wd.LastPlayed
							matched++
						}
					}
				}
				slog.Info("Emby enrichment complete", "component", "poller", "libraryItems", len(watchMap), "matched", matched)
			}
		}
	}

	// ─── Cross-reference: did the requestor watch it? ───────────────────────
	for i := range items {
		item := &items[i]
		if item.IsRequested && item.RequestedBy != "" && len(item.WatchedByUsers) > 0 {
			for _, user := range item.WatchedByUsers {
				if strings.EqualFold(user, item.RequestedBy) {
					item.WatchedByRequestor = true
					break
				}
			}
		}
	}
}
