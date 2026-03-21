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
	preview    PreviewDataSource
	rules      RulesSource
	diskGroups DiskGroupLister
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

// SetDiskGroupLister sets the disk group dependency for path-based filtering.
// Called by Registry after construction to avoid circular initialization.
func (s *WatchAnalyticsService) SetDiskGroupLister(dg DiskGroupLister) {
	s.diskGroups = dg
}

// filterItemsByDiskGroup filters items by disk group mount path.
// Returns all items if diskGroupID is nil.
func (s *WatchAnalyticsService) filterItemsByDiskGroup(items []integrations.MediaItem, diskGroupID *uint) []integrations.MediaItem {
	if diskGroupID == nil || s.diskGroups == nil {
		return items
	}
	group, err := s.diskGroups.GetByID(*diskGroupID)
	if err != nil {
		return items
	}
	mount := strings.TrimRight(group.MountPath, "/") + "/"
	filtered := make([]integrations.MediaItem, 0, len(items)/2)
	for _, item := range items {
		if strings.HasPrefix(item.Path, mount) || strings.HasPrefix(item.Path, group.MountPath) {
			filtered = append(filtered, item)
		}
	}
	return filtered
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
func (s *WatchAnalyticsService) GetDeadContent(minAgeDays int, diskGroupID *uint) *DeadContentReport {
	items := s.filterItemsByDiskGroup(s.preview.GetCachedItems(), diskGroupID)
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
func (s *WatchAnalyticsService) GetStaleContent(staleDays int, diskGroupID *uint) *StaleContentReport {
	items := s.filterItemsByDiskGroup(s.preview.GetCachedItems(), diskGroupID)
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
