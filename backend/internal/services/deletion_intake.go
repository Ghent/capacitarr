package services

import (
	"capacitarr/internal/engine"
	"capacitarr/internal/integrations"
)

// EngineDeleteRequest contains everything the engine has already resolved.
// DiskGroupID is uint (not *uint) because the engine always resolves disk
// groups during evaluation — it iterates per-disk-group. The intake method
// converts to *uint internally (diskGroupID: &req.DiskGroupID).
type EngineDeleteRequest struct {
	Client             integrations.MediaDeleter
	Item               integrations.MediaItem
	Score              float64
	Factors            []engine.ScoreFactor
	RunStatsID         uint
	DiskGroupID        uint
	CollectionGroup    string
	AddImportExclusion bool
	UpsertAudit        bool
}

// ManualDeleteRequest contains user-submitted identity data for manual deletion.
// The intake layer resolves the integration client, disk group, and mode.
type ManualDeleteRequest struct {
	MediaName     string
	MediaType     string
	IntegrationID uint
	ExternalID    string
	SizeBytes     int64
	Score         float64
	ScoreDetails  string
	PosterURL     string
}

// NOTE: ManualDeleteResult is defined in approval.go (existing type).
// When Phase 5 migrates ManualDelete → QueueManual, the result type will be
// updated with the new fields (QueuedCount, ApprovalCount, TotalCount, ResolvedMode)
// and the old definition removed.
