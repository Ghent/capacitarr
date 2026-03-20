package services

import (
	"math"
	"sort"
	"strings"
	"time"

	"capacitarr/internal/db"
	"capacitarr/internal/engine"
	"capacitarr/internal/integrations"
)

// WatchAnalyticsService provides watch-intelligence analytics (dead content,
// stale content). Requires enrichment data from a media server — items
// without enrichment are excluded to avoid false positives.
type WatchAnalyticsService struct {
	preview PreviewDataSource
	rules   RulesSource
}

// NewWatchAnalyticsService creates a new WatchAnalyticsService.
func NewWatchAnalyticsService(preview PreviewDataSource) *WatchAnalyticsService {
	return &WatchAnalyticsService{preview: preview}
}

// SetRulesSource sets the rules source for protected-item filtering.
// Called by Registry after construction to avoid circular initialization.
func (s *WatchAnalyticsService) SetRulesSource(rules RulesSource) {
	s.rules = rules
}

// ─── Dead content ───────────────────────────────────────────────────────────

// DeadContentItem is an item that has never been watched and isn't on a watchlist.
type DeadContentItem struct {
	Title         string `json:"title"`
	Type          string `json:"type"`
	SizeBytes     int64  `json:"sizeBytes"`
	DaysInLibrary int    `json:"daysInLibrary"`
	IntegrationID uint   `json:"integrationId"`
}

// DeadContentReport is the response for the dead content analytics endpoint.
type DeadContentReport struct {
	Items          []DeadContentItem `json:"items"`
	TotalCount     int               `json:"totalCount"`
	TotalSize      int64             `json:"totalSize"`
	ProtectedCount int               `json:"protectedCount"`
}

// GetDeadContent returns items with PlayCount == 0, not on watchlist,
// and added more than minAgeDays ago. Items with always_keep protection
// are excluded and counted separately.
func (s *WatchAnalyticsService) GetDeadContent(minAgeDays int) *DeadContentReport {
	items := s.preview.GetCachedItems()
	enabledRules := s.getEnabledRules()
	now := time.Now()
	minAge := time.Duration(minAgeDays) * 24 * time.Hour

	var dead []DeadContentItem
	var totalSize int64
	protectedCount := 0

	for _, item := range items {
		// Only include items that have enrichment data (to avoid false positives)
		if !hasEnrichmentData(item) {
			continue
		}
		if item.PlayCount > 0 || item.OnWatchlist {
			continue
		}
		if item.AddedAt == nil || now.Sub(*item.AddedAt) < minAge {
			continue
		}

		// Exclude absolutely protected items
		if len(enabledRules) > 0 {
			isProtected, _, _, _ := engine.ApplyRulesExported(item, enabledRules)
			if isProtected {
				protectedCount++
				continue
			}
		}

		daysInLib := int(now.Sub(*item.AddedAt).Hours() / 24)
		dead = append(dead, DeadContentItem{
			Title:         item.Title,
			Type:          string(item.Type),
			SizeBytes:     item.SizeBytes,
			DaysInLibrary: daysInLib,
			IntegrationID: item.IntegrationID,
		})
		totalSize += item.SizeBytes
	}

	// Sort by size descending (biggest dead items first)
	sort.Slice(dead, func(i, j int) bool {
		return dead[i].SizeBytes > dead[j].SizeBytes
	})

	return &DeadContentReport{
		Items:          dead,
		TotalCount:     len(dead),
		TotalSize:      totalSize,
		ProtectedCount: protectedCount,
	}
}

// ─── Stale content ──────────────────────────────────────────────────────────

// StaleContentItem is an item that was watched but hasn't been touched in a long time.
type StaleContentItem struct {
	Title            string  `json:"title"`
	Type             string  `json:"type"`
	SizeBytes        int64   `json:"sizeBytes"`
	DaysSinceWatched int     `json:"daysSinceWatched"`
	PlayCount        int     `json:"playCount"`
	StalenessScore   float64 `json:"stalenessScore"`
	IntegrationID    uint    `json:"integrationId"`
}

// StaleContentReport is the response for the stale content analytics endpoint.
type StaleContentReport struct {
	Items          []StaleContentItem `json:"items"`
	TotalCount     int                `json:"totalCount"`
	TotalSize      int64              `json:"totalSize"`
	ProtectedCount int                `json:"protectedCount"`
}

// GetStaleContent returns items where LastPlayed > staleDays ago and PlayCount > 0.
// Items with always_keep protection are excluded and counted separately.
func (s *WatchAnalyticsService) GetStaleContent(staleDays int) *StaleContentReport {
	items := s.preview.GetCachedItems()
	enabledRules := s.getEnabledRules()
	now := time.Now()
	staleDuration := time.Duration(staleDays) * 24 * time.Hour

	var stale []StaleContentItem
	var totalSize int64
	protectedCount := 0

	for _, item := range items {
		if !hasEnrichmentData(item) {
			continue
		}
		if item.PlayCount == 0 || item.LastPlayed == nil {
			continue
		}
		if now.Sub(*item.LastPlayed) < staleDuration {
			continue
		}

		// Exclude absolutely protected items
		if len(enabledRules) > 0 {
			isProtected, _, _, _ := engine.ApplyRulesExported(item, enabledRules)
			if isProtected {
				protectedCount++
				continue
			}
		}

		daysSince := int(now.Sub(*item.LastPlayed).Hours() / 24)
		score := stalenessScore(item, daysSince)

		stale = append(stale, StaleContentItem{
			Title:            item.Title,
			Type:             string(item.Type),
			SizeBytes:        item.SizeBytes,
			DaysSinceWatched: daysSince,
			PlayCount:        item.PlayCount,
			StalenessScore:   math.Round(score*100) / 100,
			IntegrationID:    item.IntegrationID,
		})
		totalSize += item.SizeBytes
	}

	// Sort by staleness score descending
	sort.Slice(stale, func(i, j int) bool {
		return stale[i].StalenessScore > stale[j].StalenessScore
	})

	return &StaleContentReport{
		Items:          stale,
		TotalCount:     len(stale),
		TotalSize:      totalSize,
		ProtectedCount: protectedCount,
	}
}

// ─── Status breakdown ───────────────────────────────────────────────────────

// StatusBreakdown is the response for the status breakdown analytics endpoint.
// It returns a tree of status → items, where items have dynamic depth based on
// media type (movies are leaves, seasons nest under their parent show, etc.).
type StatusBreakdown struct {
	Statuses []StatusGroup `json:"statuses"`
}

// StatusGroup represents a single status bucket (dead, stale, protected, active).
type StatusGroup struct {
	Name       string     `json:"name"` // "dead", "stale", "protected", "active"
	TotalSize  int64      `json:"totalSize"`
	TotalCount int        `json:"totalCount"`
	Children   []TreeNode `json:"children"`
}

// TreeNode is a recursive node in the status breakdown tree.
// Leaf nodes have Value > 0 and no Children. Container nodes (shows, artists)
// have Children and their Value is the sum of children (computed by ECharts).
type TreeNode struct {
	Name     string     `json:"name"`
	Value    int64      `json:"value,omitempty"`    // bytes — set on leaf nodes only
	Children []TreeNode `json:"children,omitempty"` // set on container nodes only
}

// GetLibraryStatusBreakdown classifies every enriched item into exactly one
// status bucket (priority: protected > dead > stale > active), then builds a
// hierarchical tree with dynamic depth based on media type.
//
// Hierarchy:
//   - Movies: status → movie title (leaf)
//   - TV: status → show title → season title (leaf)
//   - Music/Books: status → title (leaf)
//
// Classification priority:
//  1. Protected — always_keep via rules engine
//  2. Dead — PlayCount == 0, not on watchlist, AddedAt > 7 days ago
//  3. Stale — LastPlayed before 180 days ago
//  4. Active — everything else with enrichment data
//
// Items of type "show" are skipped (seasons carry the storage data).
// Items without enrichment data are excluded entirely.
func (s *WatchAnalyticsService) GetLibraryStatusBreakdown() *StatusBreakdown {
	items := s.preview.GetCachedItems()
	enabledRules := s.getEnabledRules()
	now := time.Now()

	const (
		deadMinAgeDays = 7
		staleDays      = 180
	)

	deadMinAge := time.Duration(deadMinAgeDays) * 24 * time.Hour
	staleDuration := time.Duration(staleDays) * 24 * time.Hour

	// Classify each item into a status bucket
	type classifiedItem struct {
		item   integrations.MediaItem
		bucket string
	}
	var classified []classifiedItem

	for _, item := range items {
		// Skip shows — seasons carry the actual storage data
		if item.Type == integrations.MediaTypeShow {
			continue
		}
		if !hasEnrichmentData(item) {
			continue
		}

		var bucket string

		// Priority 1: Protected
		if len(enabledRules) > 0 {
			isProtected, _, _, _ := engine.ApplyRulesExported(item, enabledRules)
			if isProtected {
				bucket = "protected"
			}
		}

		// Priority 2: Dead
		if bucket == "" && item.PlayCount == 0 && !item.OnWatchlist &&
			item.AddedAt != nil && now.Sub(*item.AddedAt) >= deadMinAge {
			bucket = "dead"
		}

		// Priority 3: Stale
		if bucket == "" && item.PlayCount > 0 && item.LastPlayed != nil &&
			now.Sub(*item.LastPlayed) >= staleDuration {
			bucket = "stale"
		}

		// Priority 4: Active
		if bucket == "" {
			bucket = "active"
		}

		classified = append(classified, classifiedItem{item: item, bucket: bucket})
	}

	// Build tree per status bucket
	statusOrder := []string{"dead", "stale", "protected", "active"}
	bucketItems := make(map[string][]classifiedItem)
	for _, ci := range classified {
		bucketItems[ci.bucket] = append(bucketItems[ci.bucket], ci)
	}

	result := &StatusBreakdown{
		Statuses: make([]StatusGroup, 0, len(statusOrder)),
	}

	for _, name := range statusOrder {
		items := bucketItems[name]
		group := StatusGroup{Name: name}

		// Group items into a tree: seasons nest under their ShowTitle,
		// everything else is a direct child (leaf).
		showMap := make(map[string][]TreeNode) // showTitle → season nodes
		var directChildren []TreeNode

		for _, ci := range items {
			group.TotalSize += ci.item.SizeBytes
			group.TotalCount++

			if ci.item.Type == integrations.MediaTypeSeason && ci.item.ShowTitle != "" {
				// Nest under parent show
				showMap[ci.item.ShowTitle] = append(showMap[ci.item.ShowTitle], TreeNode{
					Name:  ci.item.Title,
					Value: ci.item.SizeBytes,
				})
			} else {
				// Direct child (movie, artist, book, or season without ShowTitle)
				directChildren = append(directChildren, TreeNode{
					Name:  ci.item.Title,
					Value: ci.item.SizeBytes,
				})
			}
		}

		// Build show container nodes
		showNodes := make([]TreeNode, 0, len(showMap))
		for showTitle, seasons := range showMap {
			// Sort seasons by size descending
			sort.Slice(seasons, func(i, j int) bool {
				return seasons[i].Value > seasons[j].Value
			})
			showNodes = append(showNodes, TreeNode{
				Name:     showTitle,
				Children: seasons,
			})
		}

		// Sort show nodes by total size descending
		sort.Slice(showNodes, func(i, j int) bool {
			var si, sj int64
			for _, c := range showNodes[i].Children {
				si += c.Value
			}
			for _, c := range showNodes[j].Children {
				sj += c.Value
			}
			return si > sj
		})

		// Sort direct children by size descending
		sort.Slice(directChildren, func(i, j int) bool {
			return directChildren[i].Value > directChildren[j].Value
		})

		// Merge: shows first, then direct items
		group.Children = append(group.Children, showNodes...)
		group.Children = append(group.Children, directChildren...)

		result.Statuses = append(result.Statuses, group)
	}

	return result
}

// ─── Helpers ────────────────────────────────────────────────────────────────

// getEnabledRules returns the enabled rules from the rules source, or nil if unavailable.
func (s *WatchAnalyticsService) getEnabledRules() []db.CustomRule {
	if s.rules == nil {
		return nil
	}
	rules, err := s.rules.GetEnabledRules()
	if err != nil {
		return nil
	}
	return rules
}

// hasEnrichmentData returns true if the item has been through the enrichment
// pipeline (has watch data or watchlist status). Items without enrichment
// should be excluded from watch analytics to avoid false positives.
func hasEnrichmentData(item integrations.MediaItem) bool {
	return item.PlayCount > 0 || item.LastPlayed != nil || item.OnWatchlist || item.IsRequested || len(item.WatchedByUsers) > 0
}

// stalenessScore calculates a staleness score for content that was watched
// but hasn't been touched in a long time.
// Formula: daysSinceLastPlayed / 365 * (seriesEnded ? 1.5 : 1.0) * (!onWatchlist ? 1.2 : 0.5)
func stalenessScore(item integrations.MediaItem, daysSince int) float64 {
	base := float64(daysSince) / 365.0

	// Ended series are more stale
	statusMultiplier := 1.0
	if strings.ToLower(item.SeriesStatus) == "ended" {
		statusMultiplier = 1.5
	}

	// Watchlisted items are less stale
	watchlistMultiplier := 1.2
	if item.OnWatchlist {
		watchlistMultiplier = 0.5
	}

	return base * statusMultiplier * watchlistMultiplier
}
