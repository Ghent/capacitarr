package deletion

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"capacitarr/internal/db"
	"capacitarr/internal/engine"
	"capacitarr/internal/integrations"
	"capacitarr/internal/services/approval"
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
	ForceDryRun        bool // When true, item is from a dry-run disk group (Client may be nil)
}

// ---------------------------------------------------------------------------
// Intake methods — each constructs a fully-populated deleteJob from its
// specific input type and calls enqueue(). This ensures consistent
// field population regardless of the deletion trigger.
// ---------------------------------------------------------------------------

// QueueFromEngine enqueues an engine-evaluated item for deletion.
// The engine has already resolved the client, disk group, and all scoring — no
// DB lookups are performed here (hot path).
func (s *DeletionService) QueueFromEngine(req EngineDeleteRequest) error {
	mode := db.ModeAuto
	if req.ForceDryRun {
		mode = db.ModeDryRun
	}

	diskGroupID := req.DiskGroupID
	return s.enqueue(deleteJob{
		Client:             req.Client,
		Item:               req.Item,
		Score:              req.Score,
		Factors:            req.Factors,
		Trigger:            db.TriggerEngine,
		RunStatsID:         req.RunStatsID,
		DiskGroupID:        &diskGroupID,
		CollectionGroup:    req.CollectionGroup,
		EnqueuedMode:       mode,
		ForceDryRun:        req.ForceDryRun,
		AddImportExclusion: req.AddImportExclusion,
		UpsertAudit:        req.UpsertAudit,
	})
}

// QueueFromApproval enqueues an approved item for deletion.
// Performs full resolution: looks up integration → creates client → resolves
// disk group mode → determines dry-run status.
func (s *DeletionService) QueueFromApproval(item *db.ApprovalQueueItem) error {
	if s.clients == nil {
		return fmt.Errorf("deletion service not fully wired (clients resolver unavailable)")
	}

	// 1. Resolve client
	client, err := s.clients.GetDeleter(item.IntegrationID)
	if err != nil {
		return fmt.Errorf("failed to resolve client for approval %d: %w", item.ID, err)
	}

	// 2. Get integration config for AddImportExclusion
	config, err := s.clients.GetIntegrationConfig(item.IntegrationID)
	if err != nil {
		return fmt.Errorf("failed to get integration config for approval %d: %w", item.ID, err)
	}

	// 3. Parse stored score details into factors
	var factors []engine.ScoreFactor
	if item.ScoreDetails != "" {
		if jsonErr := json.Unmarshal([]byte(item.ScoreDetails), &factors); jsonErr != nil {
			slog.Error("Failed to parse score details for approval intake",
				"component", "services", "id", item.ID, "error", jsonErr)
		}
	}

	// 4. Resolve execution mode from disk group or global default
	prefs, err := s.settings.GetPreferences()
	if err != nil {
		return fmt.Errorf("failed to load preferences for approval %d: %w", item.ID, err)
	}

	diskGroupMode := prefs.DefaultDiskGroupMode // fallback
	if item.DiskGroupID != nil && s.diskGroupsFull != nil {
		if group, groupErr := s.diskGroupsFull.GetByID(*item.DiskGroupID); groupErr == nil {
			diskGroupMode = group.Mode
		}
	}

	forceDryRun := !prefs.DeletionsEnabled || diskGroupMode == db.ModeDryRun

	// 5. Construct MediaItem from approval data
	mediaItem := integrations.MediaItem{
		ExternalID:    item.ExternalID,
		IntegrationID: item.IntegrationID,
		Type:          integrations.MediaType(item.MediaType),
		Title:         item.MediaName,
		SizeBytes:     item.SizeBytes,
	}

	// 6. Enqueue
	return s.enqueue(deleteJob{
		Client:             client,
		Item:               mediaItem,
		Score:              item.Score,
		Factors:            factors,
		Trigger:            db.TriggerApproval,
		DiskGroupID:        item.DiskGroupID,
		CollectionGroup:    item.CollectionGroup,
		ForceDryRun:        forceDryRun,
		ApprovalEntryID:    item.ID,
		EnqueuedMode:       diskGroupMode,
		AddImportExclusion: config.AddImportExclusion,
	})
}

// QueueFromSunset enqueues a sunset-expired item for deletion.
// Performs full resolution: looks up integration → creates client → retrieves
// import exclusion config.
func (s *DeletionService) QueueFromSunset(item *db.SunsetQueueItem) error {
	if s.clients == nil {
		return fmt.Errorf("deletion service not fully wired (clients resolver unavailable)")
	}

	// 1. Resolve client
	client, err := s.clients.GetDeleter(item.IntegrationID)
	if err != nil {
		return fmt.Errorf("failed to resolve client for sunset item %d: %w", item.ID, err)
	}

	// 2. Get integration config for AddImportExclusion
	addImportExclusion := true // safe default
	if config, cfgErr := s.clients.GetIntegrationConfig(item.IntegrationID); cfgErr == nil {
		addImportExclusion = config.AddImportExclusion
	}

	// 3. Parse stored score details into factors
	var factors []engine.ScoreFactor
	if item.ScoreDetails != "" {
		if jsonErr := json.Unmarshal([]byte(item.ScoreDetails), &factors); jsonErr != nil {
			slog.Error("Failed to parse score details for sunset intake",
				"component", "services", "mediaName", item.MediaName, "error", jsonErr)
		}
	}

	// 4. Construct MediaItem from sunset data. IntegrationID must match
	// QueueFromApproval so the executor can attribute the job (spec §5).
	mediaItem := integrations.MediaItem{
		Title:         item.MediaName,
		Type:          integrations.MediaType(item.MediaType),
		SizeBytes:     item.SizeBytes,
		ExternalID:    item.ExternalID,
		IntegrationID: item.IntegrationID,
	}

	// 5. Enqueue
	diskGroupID := item.DiskGroupID
	return s.enqueue(deleteJob{
		Client:             client,
		Item:               mediaItem,
		Score:              item.Score,
		Factors:            factors,
		Trigger:            db.TriggerEngine,
		DiskGroupID:        &diskGroupID,
		CollectionGroup:    item.CollectionGroup,
		EnqueuedMode:       db.ModeSunset,
		SunsetQueueItemID:  item.ID,
		AddImportExclusion: addImportExclusion,
	})
}

// QueueManual enqueues user-initiated deletions. Resolves each item's
// integration client, disk group, and mode. Items whose resolved mode is
// "approval" are routed to the approval queue instead of being deleted.
func (s *DeletionService) QueueManual(items []approval.ManualDeleteRequest, approvalUpserter approval.ApprovalReturnerUpserter) (approval.ManualDeleteResult, error) {
	if s.clients == nil || s.diskGroupsFull == nil {
		return approval.ManualDeleteResult{}, fmt.Errorf("deletion service not fully wired")
	}

	var queuedCount, approvalCount, queueFullSkipped int
	var firstResolvedMode string

	prefs, err := s.settings.GetPreferences()
	if err != nil {
		return approval.ManualDeleteResult{}, fmt.Errorf("failed to load preferences for manual delete: %w", err)
	}

	for _, item := range items {
		// 1. Resolve disk group and mode for this item's integration
		diskGroupID := s.diskGroupsFull.GetDiskGroupIDForIntegration(item.IntegrationID)
		resolvedMode := prefs.DefaultDiskGroupMode
		if diskGroupID != nil && s.diskGroupsFull != nil {
			if group, groupErr := s.diskGroupsFull.GetByID(*diskGroupID); groupErr == nil {
				resolvedMode = group.Mode
			}
		}

		// Track the first resolved mode for the API response
		if firstResolvedMode == "" {
			firstResolvedMode = resolvedMode
		}

		// 2. If approval mode, route to approval queue
		if resolvedMode == db.ModeApproval {
			if approvalUpserter != nil {
				if _, upsertErr := approvalUpserter.UpsertPending(db.ApprovalQueueItem{
					MediaName:     item.MediaName,
					MediaType:     item.MediaType,
					ScoreDetails:  item.ScoreDetails,
					SizeBytes:     item.SizeBytes,
					Score:         item.Score,
					PosterURL:     item.PosterURL,
					IntegrationID: item.IntegrationID,
					ExternalID:    item.ExternalID,
					DiskGroupID:   diskGroupID,
					Trigger:       db.TriggerUser,
					UserInitiated: true,
				}); upsertErr != nil {
					slog.Error("Failed to upsert manual delete as pending",
						"component", "services", "media", item.MediaName, "error", upsertErr)
					continue
				}
			}
			approvalCount++
			continue
		}

		// 2b. Sunset: user-initiated hold, not a live delete (spec §6.2).
		if resolvedMode == db.ModeSunset {
			if s.sunsetHolds == nil || diskGroupID == nil {
				slog.Error("Failed to queue manual sunset hold — missing hold creator or disk group",
					"component", "services", "media", item.MediaName)
				continue
			}
			deletionDate := time.Now().UTC().AddDate(0, 0, prefs.SunsetDays)
			_, holdErr := s.sunsetHolds.QueueUserHold(db.SunsetQueueItem{
				MediaName:     item.MediaName,
				MediaType:     item.MediaType,
				IntegrationID: item.IntegrationID,
				ExternalID:    item.ExternalID,
				SizeBytes:     item.SizeBytes,
				Score:         item.Score,
				ScoreDetails:  item.ScoreDetails,
				PosterURL:     item.PosterURL,
				DiskGroupID:   *diskGroupID,
				Trigger:       db.TriggerUser,
				DeletionDate:  deletionDate,
			})
			if holdErr != nil {
				slog.Error("Failed to queue manual sunset hold",
					"component", "services", "media", item.MediaName, "error", holdErr)
				continue
			}
			approvalCount++
			continue
		}

		// 3. Resolve client
		client, clientErr := s.clients.GetDeleter(item.IntegrationID)
		if clientErr != nil {
			slog.Error("Failed to resolve client for manual delete",
				"component", "services", "media", item.MediaName, "error", clientErr)
			continue
		}

		// 4. Get integration config
		config, cfgErr := s.clients.GetIntegrationConfig(item.IntegrationID)
		addImportExclusion := true
		if cfgErr == nil {
			addImportExclusion = config.AddImportExclusion
		}

		// 5. Parse score details into factors
		var factors []engine.ScoreFactor
		if item.ScoreDetails != "" {
			if jsonErr := json.Unmarshal([]byte(item.ScoreDetails), &factors); jsonErr != nil {
				slog.Error("Failed to parse score details for manual delete",
					"component", "services", "media", item.MediaName, "error", jsonErr)
			}
		}

		// 6. Determine dry-run
		forceDryRun := !prefs.DeletionsEnabled || resolvedMode == db.ModeDryRun

		// 7. Construct MediaItem
		mediaItem := integrations.MediaItem{
			ExternalID:    item.ExternalID,
			IntegrationID: item.IntegrationID,
			Type:          integrations.MediaType(item.MediaType),
			Title:         item.MediaName,
			SizeBytes:     item.SizeBytes,
		}

		// 8. Enqueue
		if queueErr := s.enqueue(deleteJob{
			Client:             client,
			Item:               mediaItem,
			Score:              item.Score,
			Factors:            factors,
			Trigger:            db.TriggerUser,
			DiskGroupID:        diskGroupID,
			ForceDryRun:        forceDryRun,
			EnqueuedMode:       resolvedMode,
			AddImportExclusion: addImportExclusion,
		}); queueErr != nil {
			if errors.Is(queueErr, ErrDeletionQueueFull) {
				queueFullSkipped++
			}
			continue
		}
		queuedCount++
	}

	// Report the resolved mode (from the first item's disk group, not the global default)
	reportedMode := firstResolvedMode
	if reportedMode == "" {
		reportedMode = prefs.DefaultDiskGroupMode
	}

	return approval.ManualDeleteResult{
		Queued:           queuedCount + approvalCount,
		Total:            len(items),
		Mode:             reportedMode,
		QueueFullSkipped: queueFullSkipped,
	}, nil
}
