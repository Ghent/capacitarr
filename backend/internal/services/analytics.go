package services

import (
	"math"
	"sort"

	"capacitarr/internal/db"
	"capacitarr/internal/engine"
	"capacitarr/internal/integrations"
)

const unknownLabel = "Unknown"

// PreviewDataSource is the interface for accessing preview cache data.
// Satisfied by PreviewService.
type PreviewDataSource interface {
	GetCachedItems() []integrations.MediaItem
}

// RulesSource provides read access to enabled custom rules for analytics filtering.
// Satisfied by RulesService.
type RulesSource interface {
	GetEnabledRules() ([]db.CustomRule, error)
}

// AnalyticsService provides library composition and quality analytics.
// All computations run over the in-memory preview cache — no DB queries needed.
type AnalyticsService struct {
	preview PreviewDataSource
	rules   RulesSource
}

// NewAnalyticsService creates a new AnalyticsService.
func NewAnalyticsService(preview PreviewDataSource) *AnalyticsService {
	return &AnalyticsService{preview: preview}
}

// SetRulesSource sets the rules source for protected-item filtering.
// Called by Registry after construction to avoid circular initialization.
func (s *AnalyticsService) SetRulesSource(rules RulesSource) {
	s.rules = rules
}

// ─── Quality analytics ──────────────────────────────────────────────────────

// QualityDistribution holds detailed quality breakdown data.
type QualityDistribution struct {
	Profiles []QualityProfile `json:"profiles"`
}

// QualityProfile is a quality tier with count and size.
type QualityProfile struct {
	Name      string `json:"name"`
	Count     int    `json:"count"`
	SizeBytes int64  `json:"sizeBytes"`
}

// GetQualityDistribution returns detailed quality profile breakdown.
func (s *AnalyticsService) GetQualityDistribution() *QualityDistribution {
	items := s.preview.GetCachedItems()
	profileMap := make(map[string]*QualityProfile)

	for _, item := range items {
		qp := item.QualityProfile
		if qp == "" {
			qp = unknownLabel
		}
		if _, ok := profileMap[qp]; !ok {
			profileMap[qp] = &QualityProfile{Name: qp}
		}
		profileMap[qp].Count++
		profileMap[qp].SizeBytes += item.SizeBytes
	}

	profiles := make([]QualityProfile, 0, len(profileMap))
	for _, p := range profileMap {
		profiles = append(profiles, *p)
	}
	sort.Slice(profiles, func(i, j int) bool {
		return profiles[i].Count > profiles[j].Count
	})

	return &QualityDistribution{Profiles: profiles}
}

// ─── Bloat detection ────────────────────────────────────────────────────────

// SizeAnomaly represents an item whose size is anomalous for its quality profile and media type.
type SizeAnomaly struct {
	Title          string  `json:"title"`
	QualityProfile string  `json:"qualityProfile"`
	MediaType      string  `json:"mediaType"`
	SizeBytes      int64   `json:"sizeBytes"`
	MedianBytes    int64   `json:"medianBytes"`
	Ratio          float64 `json:"ratio"` // item size / median size
	IntegrationID  uint    `json:"integrationId"`
}

// SizeAnomalyReport is the response for the bloat detection endpoint.
type SizeAnomalyReport struct {
	Items          []SizeAnomaly `json:"items"`
	ProtectedCount int           `json:"protectedCount"`
}

// groupKey combines quality profile and media type for size anomaly grouping.
type groupKey struct {
	qualityProfile string
	mediaType      string
}

// GetSizeAnomalies returns items that are > 2x the median size for their
// (qualityProfile, mediaType) group. Items with always_keep protection are
// excluded and counted separately.
func (s *AnalyticsService) GetSizeAnomalies() *SizeAnomalyReport {
	items := s.preview.GetCachedItems()
	enabledRules := s.getEnabledRules()

	// Group sizes by (qualityProfile, mediaType)
	profileSizes := make(map[groupKey][]int64)
	profileItems := make(map[groupKey][]integrations.MediaItem)
	protectedCount := 0

	for _, item := range items {
		// Shows are excluded because their SizeBytes is the sum of all seasons —
		// including both would double-count TV storage.
		if item.Type == integrations.MediaTypeShow {
			continue
		}
		if item.SizeBytes == 0 {
			continue
		}
		qp := item.QualityProfile
		if qp == "" {
			continue // Skip items with unknown quality profile
		}

		// Exclude absolutely protected items
		if len(enabledRules) > 0 {
			isProtected, _, _, _ := engine.ApplyRulesExported(item, enabledRules)
			if isProtected {
				protectedCount++
				continue
			}
		}

		mt := string(item.Type)
		if mt == "" {
			mt = "unknown"
		}
		key := groupKey{qualityProfile: qp, mediaType: mt}
		profileSizes[key] = append(profileSizes[key], item.SizeBytes)
		profileItems[key] = append(profileItems[key], item)
	}

	var anomalies []SizeAnomaly
	for key, sizes := range profileSizes {
		if len(sizes) < 3 {
			continue // Need at least 3 items for meaningful median
		}
		median := medianInt64(sizes)
		if median == 0 {
			continue
		}
		for _, item := range profileItems[key] {
			ratio := float64(item.SizeBytes) / float64(median)
			if ratio > 2.0 {
				anomalies = append(anomalies, SizeAnomaly{
					Title:          item.Title,
					QualityProfile: key.qualityProfile,
					MediaType:      key.mediaType,
					SizeBytes:      item.SizeBytes,
					MedianBytes:    median,
					Ratio:          math.Round(ratio*100) / 100,
					IntegrationID:  item.IntegrationID,
				})
			}
		}
	}

	// Sort by ratio descending (worst offenders first)
	sort.Slice(anomalies, func(i, j int) bool {
		return anomalies[i].Ratio > anomalies[j].Ratio
	})

	return &SizeAnomalyReport{
		Items:          anomalies,
		ProtectedCount: protectedCount,
	}
}

// ─── Storage sunburst ───────────────────────────────────────────────────────

// SunburstNode holds hierarchical data for the storage sunburst chart.
type SunburstNode struct {
	Name     string         `json:"name"`
	Value    int64          `json:"value"` // bytes
	Children []SunburstNode `json:"children,omitempty"`
}

// GetStorageSunburst returns hierarchical storage data grouped by media type
// and then by quality profile within each type.
// Structure: root → [movies, seasons, artists, books, ...] → [quality profiles]
// Shows are excluded because their SizeBytes is the sum of all seasons —
// including both would double-count TV storage.
func (s *AnalyticsService) GetStorageSunburst() []SunburstNode {
	items := s.preview.GetCachedItems()

	// First level: media type → second level: quality profile → size
	type profileData struct {
		name      string
		sizeBytes int64
	}
	typeMap := make(map[string]map[string]*profileData)

	for _, item := range items {
		if item.Type == integrations.MediaTypeShow {
			continue
		}
		mt := string(item.Type)
		if mt == "" {
			mt = "unknown"
		}
		qp := item.QualityProfile
		if qp == "" {
			qp = unknownLabel
		}

		if _, ok := typeMap[mt]; !ok {
			typeMap[mt] = make(map[string]*profileData)
		}
		if _, ok := typeMap[mt][qp]; !ok {
			typeMap[mt][qp] = &profileData{name: qp}
		}
		typeMap[mt][qp].sizeBytes += item.SizeBytes
	}

	// Build the tree
	nodes := make([]SunburstNode, 0, len(typeMap))
	for mt, profiles := range typeMap {
		children := make([]SunburstNode, 0, len(profiles))
		var typeTotal int64
		for _, pd := range profiles {
			children = append(children, SunburstNode{
				Name:  pd.name,
				Value: pd.sizeBytes,
			})
			typeTotal += pd.sizeBytes
		}
		// Sort children by value descending
		sort.Slice(children, func(i, j int) bool {
			return children[i].Value > children[j].Value
		})
		nodes = append(nodes, SunburstNode{
			Name:     mt,
			Value:    typeTotal,
			Children: children,
		})
	}

	// Sort top-level nodes by value descending
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].Value > nodes[j].Value
	})

	return nodes
}

// ─── Helpers ────────────────────────────────────────────────────────────────

// getEnabledRules returns the enabled rules from the rules source, or nil if unavailable.
func (s *AnalyticsService) getEnabledRules() []db.CustomRule {
	if s.rules == nil {
		return nil
	}
	rules, err := s.rules.GetEnabledRules()
	if err != nil {
		return nil
	}
	return rules
}

func medianInt64(vals []int64) int64 {
	sorted := make([]int64, len(vals))
	copy(sorted, vals)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	n := len(sorted)
	if n%2 == 0 {
		return (sorted[n/2-1] + sorted[n/2]) / 2
	}
	return sorted[n/2]
}
