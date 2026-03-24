package services

import (
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
	"gorm.io/gorm"

	"capacitarr/internal/db"
	"capacitarr/internal/engine"
	"capacitarr/internal/events"
	"capacitarr/internal/integrations"
)

// IntegrationLister provides read access to enabled integrations and registry construction.
// Defined here to avoid import cycles between PreviewService and IntegrationService.
type IntegrationLister interface {
	ListEnabled() ([]db.IntegrationConfig, error)
	BuildIntegrationRegistry() (*integrations.IntegrationRegistry, error)
}

// RulesProvider provides read access to custom rules.
// Defined here to avoid import cycles between PreviewService and RulesService.
type RulesProvider interface {
	List() ([]db.CustomRule, error)
}

// DiskGroupLister provides read access to disk groups.
// Defined here to avoid import cycles between PreviewService and DiskGroupService.
// Used by PreviewService (List), AnalyticsService, and WatchAnalyticsService (GetByID).
type DiskGroupLister interface {
	List() ([]db.DiskGroup, error)
	GetByID(id uint) (*db.DiskGroup, error)
}

// ApprovalQueueReader provides read access to approval queue items.
// Defined here to avoid import cycles between PreviewService and ApprovalService.
type ApprovalQueueReader interface {
	ListQueue(status string, limit int, diskGroupID *uint) ([]db.ApprovalQueueItem, error)
}

// DeletionStateReader provides read access to the current deletion state.
// Defined here to avoid import cycles between PreviewService and DeletionService.
type DeletionStateReader interface {
	CurrentlyDeleting() string
}

// PreviewResult holds the full result of a score preview computation.
type PreviewResult struct {
	Items       []engine.EvaluatedItem `json:"items"`
	DiskContext *DiskContext           `json:"diskContext"`
}

// DiskContext provides disk usage information for the deletion line in the UI.
type DiskContext struct {
	TotalBytes   int64   `json:"totalBytes"`
	UsedBytes    int64   `json:"usedBytes"`
	TargetPct    float64 `json:"targetPct"`
	ThresholdPct float64 `json:"thresholdPct"`
	BytesToFree  int64   `json:"bytesToFree"`
}

// PreviewService owns the global scored-media preview for the Library
// Management page and other UI consumers. It caches the result of
// evaluating all media items and provides SSE-based notifications when
// the cache is updated or invalidated. The cache is also persisted to the
// database so it survives container restarts.
type PreviewService struct {
	database *gorm.DB
	bus      *events.EventBus

	// Cross-service dependencies (set via SetDependencies)
	integrations IntegrationLister
	preferences  SettingsReader
	rules        RulesProvider
	diskGroups   DiskGroupLister

	// Queue status enrichment dependencies (set via SetQueueDependencies)
	approvalQueue ApprovalQueueReader
	deletionState DeletionStateReader

	// Preview cache
	previewMu    sync.RWMutex
	previewCache *PreviewResult
	previewSF    singleflight.Group

	// Cache invalidation subscriber
	invalidCh chan events.Event
	done      chan struct{}
}

// NewPreviewService creates a new PreviewService.
func NewPreviewService(database *gorm.DB, bus *events.EventBus) *PreviewService {
	return &PreviewService{
		database: database,
		bus:      bus,
		done:     make(chan struct{}),
	}
}

// SetDependencies wires cross-service dependencies that cannot be injected
// at construction time due to circular initialization in the registry.
func (s *PreviewService) SetDependencies(integ IntegrationLister, settings SettingsReader, rules RulesProvider, diskGroups DiskGroupLister) {
	s.integrations = integ
	s.preferences = settings
	s.rules = rules
	s.diskGroups = diskGroups
}

// SetQueueDependencies wires the approval queue and deletion state readers
// used by EnrichWithQueueStatus to annotate preview items with queue state.
func (s *PreviewService) SetQueueDependencies(approval ApprovalQueueReader, deletion DeletionStateReader) {
	s.approvalQueue = approval
	s.deletionState = deletion
}

// GetPreview returns the cached preview result if available, or computes
// a fresh one from scratch (cold start). If force is true, the cache is
// bypassed and a fresh computation is performed. Concurrent cache-miss
// requests are coalesced via singleflight.
func (s *PreviewService) GetPreview(force bool) (*PreviewResult, error) {
	if !force {
		s.previewMu.RLock()
		if s.previewCache != nil {
			result := s.previewCache
			s.previewMu.RUnlock()
			return result, nil
		}
		s.previewMu.RUnlock()
	}

	// Coalesce concurrent cache-miss (or force) requests
	val, err, _ := s.previewSF.Do("preview", func() (any, error) {
		result, buildErr := s.buildPreviewFromScratch()
		if buildErr != nil {
			return nil, buildErr
		}

		s.previewMu.Lock()
		s.previewCache = result
		s.previewMu.Unlock()

		now := time.Now().UTC()
		s.bus.Publish(events.PreviewUpdatedEvent{
			ItemCount: len(result.Items),
			Timestamp: now,
		})
		s.bus.Publish(events.AnalyticsUpdatedEvent{
			ItemCount: len(result.Items),
			Timestamp: now,
		})

		return result, nil
	})
	if err != nil {
		return nil, err
	}

	return val.(*PreviewResult), nil
}

// SetPreviewCache populates the cache with pre-fetched, enriched items.
// Called by the poller at the end of each cycle. The service runs
// EvaluateMedia + SortEvaluated + DiskContext assembly internally.
// The result is also persisted to the database for restart recovery.
func (s *PreviewService) SetPreviewCache(items []integrations.MediaItem, prefs db.PreferenceSet, weights map[string]int, rules []db.CustomRule, evalCtx *engine.EvaluationContext) {
	result := s.buildPreview(items, prefs, weights, rules, evalCtx)

	s.previewMu.Lock()
	s.previewCache = result
	s.previewMu.Unlock()

	// Persist to DB for restart recovery
	s.PersistToDB()

	now := time.Now().UTC()
	s.bus.Publish(events.PreviewUpdatedEvent{
		ItemCount: len(result.Items),
		Timestamp: now,
	})
	s.bus.Publish(events.AnalyticsUpdatedEvent{
		ItemCount: len(result.Items),
		Timestamp: now,
	})
}

// GetCachedItems returns the raw MediaItem slice from the preview cache.
// Returns nil if the cache is empty. Implements PreviewDataSource for analytics.
func (s *PreviewService) GetCachedItems() []integrations.MediaItem {
	s.previewMu.RLock()
	defer s.previewMu.RUnlock()
	if s.previewCache == nil {
		return nil
	}
	items := make([]integrations.MediaItem, len(s.previewCache.Items))
	for i, eval := range s.previewCache.Items {
		items[i] = eval.Item
	}
	return items
}

// InvalidatePreviewCache clears the cached preview and publishes an
// invalidation event so connected clients can show a stale indicator.
func (s *PreviewService) InvalidatePreviewCache(reason string) {
	s.previewMu.Lock()
	s.previewCache = nil
	s.previewMu.Unlock()

	s.bus.Publish(events.PreviewInvalidatedEvent{
		Reason: reason,
	})

	slog.Debug("Preview cache invalidated", "component", "preview", "reason", reason)
}

// StartCacheInvalidation subscribes to the event bus and invalidates the
// preview cache when configuration changes that affect scoring are detected.
// Call Stop() to shut down the subscriber goroutine.
func (s *PreviewService) StartCacheInvalidation() {
	s.invalidCh = s.bus.Subscribe()
	go s.runInvalidationListener()
}

// Stop unsubscribes from the event bus and waits for the invalidation
// goroutine to finish.
func (s *PreviewService) Stop() {
	if s.invalidCh != nil {
		s.bus.Unsubscribe(s.invalidCh)
		<-s.done
	}
}

func (s *PreviewService) runInvalidationListener() {
	defer close(s.done)

	for event := range s.invalidCh {
		switch event.(type) {
		case events.SettingsChangedEvent:
			s.InvalidatePreviewCache("settings_changed")
		case events.RuleCreatedEvent:
			s.InvalidatePreviewCache("rule_created")
		case events.RuleUpdatedEvent:
			s.InvalidatePreviewCache("rule_updated")
		case events.RuleDeletedEvent:
			s.InvalidatePreviewCache("rule_deleted")
		case events.IntegrationAddedEvent:
			s.InvalidatePreviewCache("integration_added")
		case events.IntegrationUpdatedEvent:
			s.InvalidatePreviewCache("integration_updated")
		case events.IntegrationRemovedEvent:
			s.InvalidatePreviewCache("integration_removed")
		case events.ThresholdChangedEvent:
			s.InvalidatePreviewCache("threshold_changed")
		}
	}
}

// buildPreview evaluates and scores pre-fetched items. Used by
// SetPreviewCache (poller-driven population).
func (s *PreviewService) buildPreview(items []integrations.MediaItem, prefs db.PreferenceSet, weights map[string]int, rules []db.CustomRule, evalCtx *engine.EvaluationContext) *PreviewResult {
	evaluated := engine.EvaluateMedia(items, engine.DefaultFactors(), weights, rules, evalCtx)
	engine.SortEvaluated(evaluated, prefs.TiebreakerMethod)
	s.EnrichWithQueueStatus(evaluated)

	diskCtx := s.buildDiskContext()

	return &PreviewResult{
		Items:       evaluated,
		DiskContext: diskCtx,
	}
}

// buildPreviewFromScratch fetches everything from integrations, enriches,
// evaluates, and scores. Used on cold start (no cache, no poller data).
func (s *PreviewService) buildPreviewFromScratch() (*PreviewResult, error) {
	registry, err := s.integrations.BuildIntegrationRegistry()
	if err != nil {
		return nil, err
	}

	// Fetch media items from all MediaSources via the registry
	var allItems []integrations.MediaItem
	for id, source := range registry.MediaSources() {
		items, fetchErr := source.GetMediaItems()
		if fetchErr != nil {
			continue
		}
		for i := range items {
			items[i].IntegrationID = id
		}
		allItems = append(allItems, items...)
	}

	// Build TMDb→RatingKey map from Plex for Tautulli enrichment
	tmdbToRatingKey := make(map[int]string)
	for id := range registry.Connectors() {
		if plex, ok := registry.PlexClient(id); ok {
			plexMap, mapErr := plex.GetTMDbToRatingKeyMap()
			if mapErr != nil {
				continue
			}
			for tmdbID, ratingKey := range plexMap {
				tmdbToRatingKey[tmdbID] = ratingKey
			}
		}
	}

	// Build Jellyfin Item ID → TMDb ID map for Jellystat enrichment
	jellyfinIDToTMDbID := make(map[string]int)
	for id := range registry.Connectors() {
		if jf, ok := registry.JellyfinClient(id); ok {
			jfMap, mapErr := jf.GetItemIDToTMDbIDMap()
			if mapErr != nil {
				continue
			}
			for itemID, tmdbID := range jfMap {
				jellyfinIDToTMDbID[itemID] = tmdbID
			}
		}
	}

	// Build and run the enrichment pipeline
	pipeline := integrations.BuildEnrichmentPipeline(registry)
	integrations.RegisterTautulliEnrichers(pipeline, registry, tmdbToRatingKey)
	integrations.RegisterJellystatEnrichers(pipeline, registry, jellyfinIDToTMDbID)
	_ = pipeline.Run(allItems)

	prefs, err := s.preferences.GetPreferences()
	if err != nil {
		return nil, err
	}

	weights, err := s.preferences.GetWeightMap()
	if err != nil {
		return nil, err
	}

	rules, err := s.rules.List()
	if err != nil {
		return nil, err
	}

	// Build EvaluationContext from enabled integrations so the scoring engine
	// can exclude factors whose prerequisites are not met.
	enabledConfigs, err := s.integrations.ListEnabled()
	if err != nil {
		return nil, err
	}
	configTypes := make([]string, len(enabledConfigs))
	for i, cfg := range enabledConfigs {
		configTypes[i] = cfg.Type
	}
	evalCtx := engine.NewEvaluationContext(configTypes)

	return s.buildPreview(allItems, prefs, weights, rules, evalCtx), nil
}

// EnrichWithQueueStatus annotates each EvaluatedItem with its current queue
// state (pending, approved, user_initiated, deleting). This is a best-effort
// enrichment — if the approval queue or deletion state is unavailable, items
// are left unannotated.
func (s *PreviewService) EnrichWithQueueStatus(items []engine.EvaluatedItem) {
	if s.approvalQueue == nil && s.deletionState == nil {
		return
	}

	// Build lookup map from approval queue: "mediaName|mediaType" → queueInfo
	type queueInfo struct {
		status        string
		userInitiated bool
		id            uint
	}
	lookup := make(map[string]queueInfo)

	if s.approvalQueue != nil {
		// Fetch pending items
		pending, err := s.approvalQueue.ListQueue(db.StatusPending, 10000, nil)
		if err != nil {
			slog.Warn("Failed to fetch pending approval queue for enrichment", "component", "preview", "error", err)
		} else {
			for _, entry := range pending {
				key := entry.MediaName + "|" + entry.MediaType
				lookup[key] = queueInfo{status: db.StatusPending, userInitiated: entry.UserInitiated, id: entry.ID}
			}
		}

		// Fetch approved items
		approved, err := s.approvalQueue.ListQueue(db.StatusApproved, 10000, nil)
		if err != nil {
			slog.Warn("Failed to fetch approved approval queue for enrichment", "component", "preview", "error", err)
		} else {
			for _, entry := range approved {
				key := entry.MediaName + "|" + entry.MediaType
				lookup[key] = queueInfo{status: db.StatusApproved, userInitiated: entry.UserInitiated, id: entry.ID}
			}
		}
	}

	// Check currently deleting item
	var currentlyDeleting string
	if s.deletionState != nil {
		currentlyDeleting = s.deletionState.CurrentlyDeleting()
	}

	// Annotate items
	for i := range items {
		title := items[i].Item.Title
		mediaType := string(items[i].Item.Type)
		key := title + "|" + mediaType

		// Check if currently being deleted (highest priority)
		if currentlyDeleting != "" && currentlyDeleting == title {
			items[i].QueueStatus = "deleting"
			if info, ok := lookup[key]; ok {
				items[i].ApprovalQueueID = &info.id
			}
			continue
		}

		// Check approval queue
		if info, ok := lookup[key]; ok {
			id := info.id
			items[i].ApprovalQueueID = &id
			if info.userInitiated {
				items[i].QueueStatus = "user_initiated"
			} else {
				items[i].QueueStatus = info.status
			}
		}
	}
}

// buildDiskContext assembles the DiskContext from disk groups.
func (s *PreviewService) buildDiskContext() *DiskContext {
	diskGroups, err := s.diskGroups.List()
	if err != nil {
		return nil
	}

	if len(diskGroups) == 0 {
		return nil
	}

	var bestGroup *db.DiskGroup
	var bestBytesToFree int64

	for i := range diskGroups {
		dg := &diskGroups[i]
		if dg.TotalBytes == 0 {
			continue
		}
		usedPct := float64(dg.UsedBytes) / float64(dg.TotalBytes) * 100
		var btf int64
		if usedPct >= dg.ThresholdPct {
			btf = dg.UsedBytes - int64(float64(dg.TotalBytes)*dg.TargetPct/100)
			if btf < 0 {
				btf = 0
			}
		}
		if bestGroup == nil || btf > bestBytesToFree {
			bestGroup = dg
			bestBytesToFree = btf
		}
	}

	if bestGroup == nil {
		return nil
	}

	return &DiskContext{
		TotalBytes:   bestGroup.TotalBytes,
		UsedBytes:    bestGroup.UsedBytes,
		TargetPct:    bestGroup.TargetPct,
		ThresholdPct: bestGroup.ThresholdPct,
		BytesToFree:  bestBytesToFree,
	}
}

// ─── DB persistence (restart recovery) ──────────────────────────────────────

// LoadFromDB loads the persisted preview cache from the database.
// Called once during startup to restore the last engine run's data so the
// dashboard and analytics have data immediately without waiting for the
// first engine run. Returns true if data was loaded, false otherwise.
func (s *PreviewService) LoadFromDB() bool {
	var row db.MediaCache
	if err := s.database.First(&row, 1).Error; err != nil {
		slog.Debug("No persisted media cache found — dashboard will populate on first engine run",
			"component", "preview")
		return false
	}

	var result PreviewResult
	if err := json.Unmarshal([]byte(row.PreviewJSON), &result); err != nil {
		slog.Error("Failed to deserialize persisted media cache",
			"component", "preview", "error", err)
		return false
	}

	s.previewMu.Lock()
	s.previewCache = &result
	s.previewMu.Unlock()

	slog.Info("Restored media cache from database",
		"component", "preview", "items", row.ItemCount,
		"cachedAt", row.UpdatedAt.Format(time.RFC3339))
	return true
}

// PersistToDB writes the current preview cache to the database for restart
// recovery. Called at the end of each engine cycle after SetPreviewCache.
// Uses an upsert pattern (delete + create) to replace the singleton row.
func (s *PreviewService) PersistToDB() {
	s.previewMu.RLock()
	cache := s.previewCache
	s.previewMu.RUnlock()

	if cache == nil {
		return
	}

	data, err := json.Marshal(cache)
	if err != nil {
		slog.Error("Failed to serialize preview cache for persistence",
			"component", "preview", "error", err)
		return
	}

	row := db.MediaCache{
		ID:          1,
		PreviewJSON: string(data),
		ItemCount:   len(cache.Items),
		UpdatedAt:   time.Now().UTC(),
	}

	// Upsert: delete existing then create. SQLite doesn't support ON CONFLICT
	// with CHECK constraints reliably, so explicit delete+create is safest.
	if err := s.database.Where("id = ?", 1).Delete(&db.MediaCache{}).Error; err != nil {
		slog.Error("Failed to clear old media cache row",
			"component", "preview", "error", err)
		return
	}
	if err := s.database.Create(&row).Error; err != nil {
		slog.Error("Failed to persist media cache to database",
			"component", "preview", "error", err)
		return
	}

	slog.Debug("Persisted media cache to database",
		"component", "preview", "items", row.ItemCount)
}

// ClearPersistedCache deletes the media cache row from the database.
// Called by DataService.Reset to clear all scraped data.
func (s *PreviewService) ClearPersistedCache() {
	if err := s.database.Where("id = ?", 1).Delete(&db.MediaCache{}).Error; err != nil {
		slog.Error("Failed to clear persisted media cache",
			"component", "preview", "error", err)
	}
}
