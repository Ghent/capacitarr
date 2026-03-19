package integrations

import (
	"log/slog"
	"strings"
)

// EnrichmentClients holds optional enrichment-only service clients discovered
// during the fetch phase. These are not full Integration implementations —
// they provide watch history, request data, and other metadata used to enrich
// *arr media items before scoring.
type EnrichmentClients struct {
	Tautulli *TautulliClient
	Seerr    *SeerrClient
	Plex     *PlexClient
	Jellyfin *JellyfinClient
	Emby     *EmbyClient
}

// EnrichItems applies watch history and request data from enrichment services
// (Tautulli, Plex, Seerr, Jellyfin, Emby) to the collected media items.
// Enrichment priority: Tautulli > Plex > Jellyfin > Emby (each source only
// enriches items that don't already have watch data, i.e. PlayCount == 0).
func EnrichItems(items []MediaItem, ec EnrichmentClients) {
	// ─── Enrichment: Tautulli watch history ──────────────────────────────────
	if ec.Tautulli != nil && len(items) > 0 {
		slog.Info("Enriching items with Tautulli watch data", "component", "enrichment", "itemCount", len(items))
		for i := range items {
			item := &items[i]
			if item.ExternalID == "" {
				continue
			}
			var watchData *TautulliWatchData
			var err error
			if item.Type == MediaTypeShow {
				watchData, err = ec.Tautulli.GetShowWatchHistory(item.ExternalID)
			} else {
				watchData, err = ec.Tautulli.GetWatchHistory(item.ExternalID)
			}
			if err != nil {
				slog.Debug("Tautulli enrichment failed", "component", "enrichment", "title", item.Title, "error", err)
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

	// ─── Enrichment: Plex watch history ────────────────────────────────────
	if ec.Plex != nil && len(items) > 0 {
		slog.Info("Enriching items with Plex watch data", "component", "enrichment", "itemCount", len(items))
		watchMap, err := ec.Plex.GetBulkWatchData()
		if err != nil {
			slog.Warn("Failed to fetch Plex watch data", "component", "enrichment",
				"operation", "fetch_plex_watch", "error", err)
		} else {
			matched := 0
			for i := range items {
				item := &items[i]
				titleKey := normalizedTitleKey(item)
				if wd, ok := watchMap[titleKey]; ok {
					// Only enrich if we don't already have watch data (Tautulli takes priority)
					if item.PlayCount == 0 {
						item.PlayCount = wd.PlayCount
						item.LastPlayed = wd.LastPlayed
						matched++
					}
				}
			}
			slog.Info("Plex enrichment complete", "component", "enrichment",
				"libraryItems", len(watchMap), "matched", matched)
		}
	}

	// ─── Enrichment: Seerr request data ──────────────────────────────────
	if ec.Seerr != nil && len(items) > 0 {
		slog.Info("Enriching items with Seerr request data", "component", "enrichment", "itemCount", len(items))
		requests, err := ec.Seerr.GetRequestedMedia()
		if err != nil {
			slog.Warn("Failed to fetch Seerr requests", "component", "enrichment", "operation", "fetch_seerr", "error", err)
		} else {
			// Build lookup by TMDb ID
			requestMap := make(map[int]MediaRequest)
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
			slog.Debug("Seerr enrichment complete", "component", "enrichment", "requests", len(requests), "matched", matched)
		}
	}

	// ─── Enrichment: Jellyfin watch history ─────────────────────────────────
	if ec.Jellyfin != nil && len(items) > 0 {
		slog.Info("Enriching items with Jellyfin watch data", "component", "enrichment", "itemCount", len(items))
		userID, err := ec.Jellyfin.GetAdminUserID()
		if err != nil {
			slog.Warn("Failed to get Jellyfin admin user", "component", "enrichment", "operation", "jellyfin_admin_user", "error", err)
		} else {
			watchMap, err := ec.Jellyfin.GetBulkWatchDataForUser(userID)
			if err != nil {
				slog.Warn("Failed to fetch Jellyfin watch data", "component", "enrichment", "operation", "fetch_jellyfin_watch", "error", err)
			} else {
				matched := 0
				for i := range items {
					item := &items[i]
					titleKey := normalizedTitleKey(item)
					if wd, ok := watchMap[titleKey]; ok {
						if item.PlayCount == 0 {
							item.PlayCount = wd.PlayCount
							item.LastPlayed = wd.LastPlayed
							matched++
						}
					}
				}
				slog.Info("Jellyfin enrichment complete", "component", "enrichment", "libraryItems", len(watchMap), "matched", matched)
			}
		}
	}

	// ─── Enrichment: Emby watch history ─────────────────────────────────────
	if ec.Emby != nil && len(items) > 0 {
		slog.Info("Enriching items with Emby watch data", "component", "enrichment", "itemCount", len(items))
		userID, err := ec.Emby.GetAdminUserID()
		if err != nil {
			slog.Warn("Failed to get Emby admin user", "component", "enrichment", "operation", "emby_admin_user", "error", err)
		} else {
			watchMap, err := ec.Emby.GetBulkWatchDataForUser(userID)
			if err != nil {
				slog.Warn("Failed to fetch Emby watch data", "component", "enrichment", "operation", "fetch_emby_watch", "error", err)
			} else {
				matched := 0
				for i := range items {
					item := &items[i]
					titleKey := normalizedTitleKey(item)
					if wd, ok := watchMap[titleKey]; ok {
						if item.PlayCount == 0 {
							item.PlayCount = wd.PlayCount
							item.LastPlayed = wd.LastPlayed
							matched++
						}
					}
				}
				slog.Info("Emby enrichment complete", "component", "enrichment", "libraryItems", len(watchMap), "matched", matched)
			}
		}
	}

	// ─── Enrichment: Watchlist / Favorites ──────────────────────────────────
	// Priority: Plex on-deck > Jellyfin favorites > Emby favorites.
	// First source that identifies an item as on-watchlist wins (no overwrite).
	watchlistSet := make(map[string]bool)
	if ec.Plex != nil && len(items) > 0 {
		slog.Info("Enriching items with Plex on-deck (watchlist) data", "component", "enrichment", "itemCount", len(items))
		onDeck, err := ec.Plex.GetOnDeckItems()
		if err != nil {
			slog.Warn("Failed to fetch Plex on-deck items", "component", "enrichment", "operation", "fetch_plex_ondeck", "error", err)
		} else {
			for k, v := range onDeck {
				watchlistSet[k] = v
			}
			slog.Info("Plex on-deck enrichment fetched", "component", "enrichment", "onDeckItems", len(onDeck))
		}
	}
	if ec.Jellyfin != nil && len(items) > 0 {
		slog.Info("Enriching items with Jellyfin favorites (watchlist) data", "component", "enrichment", "itemCount", len(items))
		userID, err := ec.Jellyfin.GetAdminUserID()
		if err != nil {
			slog.Warn("Failed to get Jellyfin admin user for favorites", "component", "enrichment", "operation", "jellyfin_admin_user_favs", "error", err)
		} else {
			favs, err := ec.Jellyfin.GetFavoritedItems(userID)
			if err != nil {
				slog.Warn("Failed to fetch Jellyfin favorites", "component", "enrichment", "operation", "fetch_jellyfin_favs", "error", err)
			} else {
				for k, v := range favs {
					if !watchlistSet[k] {
						watchlistSet[k] = v
					}
				}
				slog.Info("Jellyfin favorites enrichment fetched", "component", "enrichment", "favoriteItems", len(favs))
			}
		}
	}
	if ec.Emby != nil && len(items) > 0 {
		slog.Info("Enriching items with Emby favorites (watchlist) data", "component", "enrichment", "itemCount", len(items))
		userID, err := ec.Emby.GetAdminUserID()
		if err != nil {
			slog.Warn("Failed to get Emby admin user for favorites", "component", "enrichment", "operation", "emby_admin_user_favs", "error", err)
		} else {
			favs, err := ec.Emby.GetFavoritedItems(userID)
			if err != nil {
				slog.Warn("Failed to fetch Emby favorites", "component", "enrichment", "operation", "fetch_emby_favs", "error", err)
			} else {
				for k, v := range favs {
					if !watchlistSet[k] {
						watchlistSet[k] = v
					}
				}
				slog.Info("Emby favorites enrichment fetched", "component", "enrichment", "favoriteItems", len(favs))
			}
		}
	}
	// Apply watchlist flags to items
	if len(watchlistSet) > 0 {
		matched := 0
		for i := range items {
			item := &items[i]
			titleKey := normalizedTitleKey(item)
			if watchlistSet[titleKey] {
				item.OnWatchlist = true
				matched++
			}
		}
		slog.Info("Watchlist enrichment complete", "component", "enrichment", "watchlistItems", len(watchlistSet), "matched", matched)
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

// normalizedTitleKey returns the lowercase title key for matching.
// For season items with a ShowTitle, uses the show title instead.
func normalizedTitleKey(item *MediaItem) string {
	titleKey := strings.ToLower(strings.TrimSpace(item.Title))
	if item.ShowTitle != "" {
		titleKey = strings.ToLower(strings.TrimSpace(item.ShowTitle))
	}
	return titleKey
}
