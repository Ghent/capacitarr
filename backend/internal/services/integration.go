package services

import (
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"time"

	"gorm.io/gorm"

	"capacitarr/internal/cache"
	"capacitarr/internal/db"
	"capacitarr/internal/events"
	"capacitarr/internal/integrations"
)

// Sentinel errors for integration and general service operations.
var (
	ErrNotFound                   = errors.New("record not found")
	ErrUnsupportedIntegrationType = errors.New("unsupported integration type for rule values")
	ErrIntegrationNoRuleValues    = errors.New("integration does not support rule value lookups")
	ErrUnknownAction              = errors.New("unknown action")
	ErrDeletionQueueFull          = errors.New("deletion queue is full")
)

// Rule value action identifiers used in FetchRuleValues switch.
const (
	ruleActionQuality    = "quality"
	ruleActionTag        = "tag"
	ruleActionGenre      = "genre"
	ruleActionLanguage   = "language"
	ruleActionCollection = "collection"
	ruleActionLabel      = "label"
)

// seriesStatusOptions defines the closed set of series status values (from Sonarr API)
// used for rule autocomplete. Extracted as a package-level variable so updates are
// made in one place and drift-detection tests can verify consistency.
var seriesStatusOptions = []integrations.NameValue{
	{Value: "continuing", Label: "Continuing"},
	{Value: "ended", Label: "Ended"},
	{Value: "upcoming", Label: "Upcoming"},
	{Value: "deleted", Label: "Deleted"},
}

// mediaTypeOptions defines the closed set of media type values used for rule
// autocomplete. Must stay in sync with the MediaType* constants in
// integrations/types.go — drift is caught by TestMediaTypeOptions_MatchesConstants.
var mediaTypeOptions = []integrations.NameValue{
	{Value: string(integrations.MediaTypeMovie), Label: "Movie"},
	{Value: string(integrations.MediaTypeShow), Label: "Show"},
	{Value: string(integrations.MediaTypeSeason), Label: "Season"},
	{Value: string(integrations.MediaTypeEpisode), Label: "Episode"},
	{Value: string(integrations.MediaTypeArtist), Label: "Artist"},
	{Value: string(integrations.MediaTypeBook), Label: "Book"},
}

// genreSuggestions defines common genre names used as combobox suggestions for
// rule creation. This is a convenience list — users can type any genre since the
// field is a combobox, not a closed select.
var genreSuggestions = []integrations.NameValue{
	{Value: "Action", Label: "Action"},
	{Value: "Adventure", Label: "Adventure"},
	{Value: "Animation", Label: "Animation"},
	{Value: "Comedy", Label: "Comedy"},
	{Value: "Crime", Label: "Crime"},
	{Value: "Documentary", Label: "Documentary"},
	{Value: "Drama", Label: "Drama"},
	{Value: "Fantasy", Label: "Fantasy"},
	{Value: "Horror", Label: "Horror"},
	{Value: "Mystery", Label: "Mystery"},
	{Value: "Romance", Label: "Romance"},
	{Value: "Sci-Fi", Label: "Sci-Fi"},
	{Value: "Thriller", Label: "Thriller"},
}

// DiskGroupManager provides disk group operations needed by IntegrationService.
// Defined here to avoid import cycles between IntegrationService and DiskGroupService.
type DiskGroupManager interface {
	Upsert(disk integrations.DiskSpace) (*db.DiskGroup, error)
	RemoveAll() (int64, error)    // Immediate hard delete — used by IntegrationService.Delete() (deliberate user action)
	MarkAllStale() (int64, error) // Soft-delete — used by poller (involuntary, all integrations disabled)
	HasSunsetModeForIntegration(integrationID uint) (bool, error)
	SunsetLinkedIntegrationIDs() (map[uint]bool, error)
}

// HealthReporter is the interface consumed by IntegrationService.SyncAll to
// report connection test results to the health monitor. Defined as an interface
// to avoid import cycles (IntegrationHealthService depends on IntegrationService).
type HealthReporter interface {
	ReportFailure(id uint, err error)
	ReportSuccess(id uint)
}

// IntegrationService manages integration CRUD, connection testing, and
// external API lookups (rule values, quality profiles, tags, languages).
// It also owns the RuleValueCache for caching external API responses.
type IntegrationService struct {
	db             *gorm.DB
	bus            *events.EventBus
	diskGroups     DiskGroupManager
	healthReporter HealthReporter
	ruleValueCache *cache.TTLCache
}

// Wired returns true when all lazily-injected dependencies are non-nil.
// Used by Registry.Validate() to catch missing wiring at startup.
func (s *IntegrationService) Wired() bool {
	return s.diskGroups != nil
}

// SetDiskGroupService wires the DiskGroupService dependency for disk group operations.
// Called by Registry after construction to avoid circular initialization.
func (s *IntegrationService) SetDiskGroupService(dg DiskGroupManager) {
	s.diskGroups = dg
}

// SetHealthReporter wires the HealthReporter so SyncAll can report connection
// results to the health service. Called by Registry after construction.
func (s *IntegrationService) SetHealthReporter(hr HealthReporter) {
	s.healthReporter = hr
}

// NewIntegrationService creates a new IntegrationService with an embedded rule value cache.
func NewIntegrationService(database *gorm.DB, bus *events.EventBus) *IntegrationService {
	return &IntegrationService{
		db:             database,
		bus:            bus,
		ruleValueCache: cache.New(5 * time.Minute),
	}
}

// CloseCache stops the background cache janitor. Call during graceful shutdown.
func (s *IntegrationService) CloseCache() {
	s.ruleValueCache.Close()
}

// FetchCollectionValues returns collection names from all enabled media server
// integrations (Plex, Jellyfin, Emby). Results are cached with the standard TTL.
// The returned slice is sorted alphabetically and deduplicated across all servers.
func (s *IntegrationService) FetchCollectionValues() ([]integrations.NameValue, error) {
	const cacheKey = "global:collections"

	if cached, ok := s.ruleValueCache.Get(cacheKey); ok {
		if nv, ok := cached.([]integrations.NameValue); ok {
			return nv, nil
		}
	}

	configs, err := s.ListEnabled()
	if err != nil {
		return nil, fmt.Errorf("failed to list enabled integrations: %w", err)
	}

	// Collect unique collection names across all media server integrations.
	// Uses the CollectionNameFetcher capability interface instead of a switch
	// on integration type — new integrations that implement GetCollectionNames()
	// are automatically discovered.
	seen := make(map[string]bool)
	for _, cfg := range configs {
		client := integrations.CreateClient(cfg.Type, cfg.URL, cfg.APIKey)
		if client == nil {
			continue
		}

		fetcher, ok := client.(integrations.CollectionNameFetcher)
		if !ok {
			continue
		}

		names, fetchErr := fetcher.GetCollectionNames()
		if fetchErr != nil {
			slog.Error("Failed to fetch collection names",
				"component", "services", "integrationId", cfg.ID, "type", cfg.Type, "error", fetchErr)
			continue
		}
		for _, name := range names {
			seen[name] = true
		}
	}

	result := make([]integrations.NameValue, 0, len(seen))
	for name := range seen {
		result = append(result, integrations.NameValue{Value: name, Label: name})
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Label < result[j].Label
	})

	s.ruleValueCache.Set(cacheKey, result)
	return result, nil
}

// FetchLabelValues returns label/tag names from all enabled media server
// integrations (Plex, Jellyfin, Emby). Results are cached with the standard TTL.
// The returned slice is sorted alphabetically and deduplicated across all servers.
func (s *IntegrationService) FetchLabelValues() ([]integrations.NameValue, error) {
	const cacheKey = "global:labels"

	if cached, ok := s.ruleValueCache.Get(cacheKey); ok {
		if nv, ok := cached.([]integrations.NameValue); ok {
			return nv, nil
		}
	}

	configs, err := s.ListEnabled()
	if err != nil {
		return nil, fmt.Errorf("failed to list enabled integrations: %w", err)
	}

	// Collect unique label names across all media server integrations.
	// Uses the LabelNameFetcher capability interface — new integrations that
	// implement GetLabelNames() are automatically discovered.
	seen := make(map[string]bool)
	for _, cfg := range configs {
		client := integrations.CreateClient(cfg.Type, cfg.URL, cfg.APIKey)
		if client == nil {
			continue
		}

		fetcher, ok := client.(integrations.LabelNameFetcher)
		if !ok {
			continue
		}

		names, fetchErr := fetcher.GetLabelNames()
		if fetchErr != nil {
			slog.Error("Failed to fetch label names",
				"component", "services", "integrationId", cfg.ID, "type", cfg.Type, "error", fetchErr)
			continue
		}
		for _, name := range names {
			seen[name] = true
		}
	}

	result := make([]integrations.NameValue, 0, len(seen))
	for name := range seen {
		result = append(result, integrations.NameValue{Value: name, Label: name})
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Label < result[j].Label
	})

	s.ruleValueCache.Set(cacheKey, result)
	return result, nil
}

// InvalidateRuleValueCache removes all cached entries for a specific integration.
func (s *IntegrationService) InvalidateRuleValueCache(integrationID int) {
	s.ruleValueCache.InvalidatePrefix(strconv.Itoa(integrationID) + ":")
}

// InvalidateAllRuleValueCaches removes all cached rule value entries.
func (s *IntegrationService) InvalidateAllRuleValueCaches() {
	s.ruleValueCache.InvalidateAll()
}

// TestConnectionResult holds the outcome of a connection test.
type TestConnectionResult struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

// TestConnection tests connectivity to an integration given a type, URL, and API key.
// If apiKey is empty or masked and integrationID is provided, the stored key is used.
//
// This is a pure diagnostic — it does NOT publish events, send notifications,
// or mutate health/sync state. The user is staring at the result in the UI.
// Health state tracking is the responsibility of IntegrationHealthService.
func (s *IntegrationService) TestConnection(intType, url, apiKey string, integrationID *int) TestConnectionResult {
	// Resolve masked or empty API keys to the stored value
	if (apiKey == "" || db.IsMaskedKey(apiKey)) && integrationID != nil && *integrationID > 0 {
		existing, err := s.GetByID(uint(*integrationID))
		if err == nil {
			apiKey = existing.APIKey
		}
	}

	// All integration types use the factory + capability interface discovery
	rawClient := integrations.CreateClient(intType, url, apiKey)
	if rawClient == nil {
		return TestConnectionResult{Success: false, Error: "Unknown integration type"}
	}

	conn, ok := rawClient.(integrations.Connectable)
	if !ok {
		return TestConnectionResult{Success: false, Error: "Integration type does not support connection testing"}
	}

	if err := conn.TestConnection(); err != nil {
		return TestConnectionResult{Success: false, Error: err.Error()}
	}

	// On success, invalidate cached rule values so fresh data is fetched
	if integrationID != nil && *integrationID > 0 {
		s.InvalidateRuleValueCache(*integrationID)
	}

	return TestConnectionResult{Success: true, Message: "Connection successful"}
}

// FetchRuleValues retrieves autocomplete values for a given rule field action
// from the specified integration. Results are cached with a 5-minute TTL.
// Returns (result, error). Static field types (booleans, free-text) are handled
// inline without an external API call.
func (s *IntegrationService) FetchRuleValues(integrationID uint, action string) (any, error) {
	cacheKey := fmt.Sprintf("%d:%s", integrationID, action)

	// Check cache first
	if cached, ok := s.ruleValueCache.Get(cacheKey); ok {
		return cached, nil
	}

	// Static field types — no external API call needed
	switch action {
	case ruleFieldSeriesStatus:
		result := map[string]any{
			"type":    "closed",
			"options": seriesStatusOptions,
		}
		s.ruleValueCache.Set(cacheKey, result)
		return result, nil

	case "monitored", "requested", "incollection", "watchlist", "watchedbyreq", "haslabel":
		result := map[string]any{
			"type": "closed",
			"options": []integrations.NameValue{
				{Value: "true", Label: "Yes"},
				{Value: "false", Label: "No"},
			},
		}
		s.ruleValueCache.Set(cacheKey, result)
		return result, nil

	case "type":
		result := map[string]any{
			"type":    "closed",
			"options": mediaTypeOptions,
		}
		s.ruleValueCache.Set(cacheKey, result)
		return result, nil

	// Collection field — aggregates from all enabled media server integrations
	case ruleActionCollection:
		collections, collectErr := s.FetchCollectionValues()
		if collectErr != nil {
			return nil, fmt.Errorf("failed to fetch collection values: %w", collectErr)
		}
		result := map[string]any{"type": "combobox", "suggestions": collections}
		s.ruleValueCache.Set(cacheKey, result)
		return result, nil

	// Label field — aggregates from all enabled media server integrations
	case ruleActionLabel:
		labels, labelErr := s.FetchLabelValues()
		if labelErr != nil {
			return nil, fmt.Errorf("failed to fetch label values: %w", labelErr)
		}
		result := map[string]any{"type": "combobox", "suggestions": labels}
		s.ruleValueCache.Set(cacheKey, result)
		return result, nil

	// Free-text field metadata — no caching needed, return immediately
	case "title":
		return map[string]any{
			"type": "free", "inputType": "text", "placeholder": "e.g., Breaking Bad", "suffix": "",
		}, nil
	case "rating":
		return map[string]any{
			"type": "free", "inputType": "number", "placeholder": "e.g., 7.5", "suffix": "",
		}, nil
	case "sizebytes":
		return map[string]any{
			"type": "free", "inputType": "number", "placeholder": "e.g., 5368709120", "suffix": "bytes (≈ GB)",
		}, nil
	case "timeinlibrary":
		return map[string]any{
			"type": "free", "inputType": "number", "placeholder": "e.g., 30", "suffix": "days",
		}, nil
	case "year":
		return map[string]any{
			"type": "free", "inputType": "number", "placeholder": "e.g., 2020", "suffix": "",
		}, nil
	case "seasoncount":
		return map[string]any{
			"type": "free", "inputType": "number", "placeholder": "e.g., 5", "suffix": "",
		}, nil
	case "episodecount":
		return map[string]any{
			"type": "free", "inputType": "number", "placeholder": "e.g., 100", "suffix": "",
		}, nil
	case "playcount":
		return map[string]any{
			"type": "free", "inputType": "number", "placeholder": "e.g., 0", "suffix": "",
		}, nil
	case "requestcount":
		return map[string]any{
			"type": "free", "inputType": "number", "placeholder": "e.g., 3", "suffix": "",
		}, nil
	case "lastplayed":
		return map[string]any{
			"type": "free", "inputType": "number", "placeholder": "e.g., 30", "suffix": "days",
		}, nil
	case "requestedby":
		return map[string]any{
			"type": "free", "inputType": "text", "placeholder": "e.g., john", "suffix": "",
		}, nil
	}

	// Dynamic fields — require API call to the *arr service
	cfg, err := s.GetByID(integrationID)
	if err != nil {
		return nil, err
	}

	client := integrations.CreateClient(cfg.Type, cfg.URL, cfg.APIKey)
	if client == nil {
		return nil, ErrUnsupportedIntegrationType
	}

	fetcher, ok := client.(integrations.RuleValueFetcher)
	if !ok {
		return nil, ErrIntegrationNoRuleValues
	}

	var result map[string]any

	switch action {
	case ruleActionQuality:
		profiles, fetchErr := fetcher.GetQualityProfiles()
		if fetchErr != nil {
			return nil, fmt.Errorf("failed to fetch quality profiles: %w", fetchErr)
		}
		result = map[string]any{"type": "closed", "options": profiles}

	case ruleActionTag:
		tags, fetchErr := fetcher.GetTags()
		if fetchErr != nil {
			return nil, fmt.Errorf("failed to fetch tags: %w", fetchErr)
		}
		result = map[string]any{"type": "combobox", "suggestions": tags}

	case ruleActionGenre:
		result = map[string]any{
			"type":        "combobox",
			"suggestions": genreSuggestions,
		}

	case ruleActionLanguage:
		langs, fetchErr := fetcher.GetLanguages()
		if fetchErr != nil {
			return nil, fmt.Errorf("failed to fetch languages: %w", fetchErr)
		}
		if langs == nil {
			return map[string]any{
				"type": "free", "inputType": "text", "placeholder": "e.g., English", "suffix": "",
			}, nil
		}
		result = map[string]any{"type": "closed", "options": langs}

	default:
		return nil, fmt.Errorf("%w: %s", ErrUnknownAction, action)
	}

	s.ruleValueCache.Set(cacheKey, result)
	return result, nil
}

// Create persists a new integration config.
func (s *IntegrationService) Create(config db.IntegrationConfig) (*db.IntegrationConfig, error) {
	if err := s.db.Create(&config).Error; err != nil {
		return nil, fmt.Errorf("failed to create integration: %w", err)
	}

	s.bus.Publish(events.IntegrationAddedEvent{
		IntegrationID:   config.ID,
		IntegrationType: config.Type,
		Name:            config.Name,
	})

	return &config, nil
}

// Update modifies an existing integration config (full-replace). Production
// code uses PartialUpdate; this method exists for test convenience.
func (s *IntegrationService) Update(id uint, config db.IntegrationConfig) (*db.IntegrationConfig, error) {
	var existing db.IntegrationConfig
	if err := s.db.First(&existing, id).Error; err != nil {
		return nil, fmt.Errorf("integration not found: %w", err)
	}

	config.ID = id
	if err := s.db.Save(&config).Error; err != nil {
		return nil, fmt.Errorf("failed to update integration: %w", err)
	}

	s.bus.Publish(events.IntegrationUpdatedEvent{
		IntegrationID:   config.ID,
		IntegrationType: config.Type,
		Name:            config.Name,
	})

	return &config, nil
}

// IntegrationUpdate holds the fields that can be partially updated on an integration.
// Pointer fields distinguish "not provided" (nil) from "explicitly set to false".
type IntegrationUpdate struct {
	Name               string `json:"name"`
	URL                string `json:"url"`
	APIKey             string `json:"apiKey"`
	Enabled            *bool  `json:"enabled"`
	CollectionDeletion *bool  `json:"collectionDeletion"`
	ShowLevelOnly      *bool  `json:"showLevelOnly"`
	AddImportExclusion *bool  `json:"addImportExclusion"`
}

// PartialUpdate merges the provided fields into the existing integration config
// and saves the result. Clears LastError and LastSync because the configuration
// has changed and previous sync state is no longer valid.
func (s *IntegrationService) PartialUpdate(id uint, update IntegrationUpdate) (*db.IntegrationConfig, error) {
	existing, err := s.GetByID(id)
	if err != nil {
		return nil, err
	}

	if update.Name != "" {
		existing.Name = update.Name
	}
	if update.URL != "" {
		existing.URL = update.URL
	}
	if update.APIKey != "" && !db.IsMaskedKey(update.APIKey) {
		existing.APIKey = update.APIKey
	}
	if update.Enabled != nil {
		existing.Enabled = *update.Enabled
	}
	if update.CollectionDeletion != nil {
		existing.CollectionDeletion = *update.CollectionDeletion
	}
	if update.ShowLevelOnly != nil {
		existing.ShowLevelOnly = *update.ShowLevelOnly
	}
	if update.AddImportExclusion != nil {
		existing.AddImportExclusion = *update.AddImportExclusion
	}

	// Config changed → clear stale sync status
	existing.LastError = ""
	existing.LastSync = nil

	if err := s.db.Save(existing).Error; err != nil {
		return nil, fmt.Errorf("failed to update integration: %w", err)
	}

	s.bus.Publish(events.IntegrationUpdatedEvent{
		IntegrationID:   existing.ID,
		IntegrationType: existing.Type,
		Name:            existing.Name,
	})

	// Return with masked API key
	existing.APIKey = db.MaskAPIKey(existing.APIKey)
	return existing, nil
}

// Delete removes an integration config. If no enabled integrations remain
// after deletion, all disk groups are removed immediately since they can
// no longer be validated by any integration.
func (s *IntegrationService) Delete(id uint) error {
	var config db.IntegrationConfig
	if err := s.db.First(&config, id).Error; err != nil {
		return fmt.Errorf("integration not found: %w", err)
	}

	if err := s.db.Delete(&config).Error; err != nil {
		return fmt.Errorf("failed to delete integration: %w", err)
	}

	s.bus.Publish(events.IntegrationRemovedEvent{
		IntegrationID:   config.ID,
		IntegrationType: config.Type,
		Name:            config.Name,
	})

	// If no enabled integrations remain, remove all disk groups immediately
	remaining, err := s.ListEnabled()
	if err != nil {
		slog.Error("Failed to check remaining integrations after delete",
			"component", "services", "error", err)
		return nil // Integration was deleted, don't fail the request
	}
	if len(remaining) == 0 && s.diskGroups != nil {
		if removed, rmErr := s.diskGroups.RemoveAll(); rmErr != nil {
			slog.Error("Failed to remove disk groups after last integration deleted",
				"component", "services", "error", rmErr)
		} else if removed > 0 {
			slog.Info("Removed all disk groups after last integration deleted",
				"component", "services", "count", removed)
		}
	}

	return nil
}

// List returns all integration configs ordered by created_at ascending.
func (s *IntegrationService) List() ([]db.IntegrationConfig, error) {
	var configs []db.IntegrationConfig
	if err := s.db.Order("created_at asc").Find(&configs).Error; err != nil {
		return nil, fmt.Errorf("failed to list integrations: %w", err)
	}
	return configs, nil
}

// GetByID returns a single integration config by primary key.
// Returns ErrNotFound if the record does not exist.
func (s *IntegrationService) GetByID(id uint) (*db.IntegrationConfig, error) {
	var config db.IntegrationConfig
	if err := s.db.First(&config, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get integration: %w", err)
	}
	return &config, nil
}

// IsShowLevelOnlyEffective returns true when the engine should treat the
// integration as show-level-only. This is the case when either:
//   - The stored ShowLevelOnly setting is true, OR
//   - The integration is Sonarr and linked to at least one sunset-mode disk group.
//
// The stored DB value is never mutated — this is a virtual override.
func (s *IntegrationService) IsShowLevelOnlyEffective(id uint) (bool, error) {
	cfg, err := s.GetByID(id)
	if err != nil {
		return false, err
	}

	// If the user explicitly enabled ShowLevelOnly, that takes precedence.
	if cfg.ShowLevelOnly {
		return true, nil
	}

	// Only Sonarr integrations emit season-level items.
	if cfg.Type != string(integrations.IntegrationTypeSonarr) {
		return false, nil
	}

	// Check if any linked disk group is in sunset mode.
	return s.diskGroups.HasSunsetModeForIntegration(id)
}

// IntegrationResponse wraps an IntegrationConfig with computed override fields
// for the API layer. The stored ShowLevelOnly value is never mutated.
type IntegrationResponse struct {
	db.IntegrationConfig
	ShowLevelOnlyOverride       bool   `json:"showLevelOnlyOverride"`
	ShowLevelOnlyOverrideReason string `json:"showLevelOnlyOverrideReason"`
}

// showLevelOverrideReason is the standard explanation returned when the
// virtual show-level-only override is active.
const showLevelOverrideReason = "Linked to sunset-mode disk group"

// GetWithOverrideState returns a single integration enriched with the
// computed ShowLevelOnly override state.
func (s *IntegrationService) GetWithOverrideState(id uint) (*IntegrationResponse, error) {
	cfg, err := s.GetByID(id)
	if err != nil {
		return nil, err
	}

	resp := &IntegrationResponse{IntegrationConfig: *cfg}
	resp.APIKey = db.MaskAPIKey(resp.APIKey)
	if cfg.Type == string(integrations.IntegrationTypeSonarr) && !cfg.ShowLevelOnly {
		hasSunset, sunsetErr := s.diskGroups.HasSunsetModeForIntegration(id)
		if sunsetErr == nil && hasSunset {
			resp.ShowLevelOnlyOverride = true
			resp.ShowLevelOnlyOverrideReason = showLevelOverrideReason
		}
	}
	return resp, nil
}

// ListWithOverrideState returns all integrations enriched with the computed
// ShowLevelOnly override state. Uses a single batch query to determine which
// integrations are linked to sunset-mode disk groups.
func (s *IntegrationService) ListWithOverrideState() ([]IntegrationResponse, error) {
	configs, err := s.List()
	if err != nil {
		return nil, err
	}

	sunsetLinked, sunsetErr := s.diskGroups.SunsetLinkedIntegrationIDs()
	if sunsetErr != nil {
		// Non-fatal: return configs without override info rather than failing the list.
		sunsetLinked = nil
	}

	result := make([]IntegrationResponse, len(configs))
	for i, cfg := range configs {
		result[i] = IntegrationResponse{IntegrationConfig: cfg}
		result[i].APIKey = db.MaskAPIKey(result[i].APIKey)
		if cfg.Type == string(integrations.IntegrationTypeSonarr) && !cfg.ShowLevelOnly && sunsetLinked[cfg.ID] {
			result[i].ShowLevelOnlyOverride = true
			result[i].ShowLevelOnlyOverrideReason = showLevelOverrideReason
		}
	}
	return result, nil
}

// ListEnabled returns all integration configs where enabled = true.
func (s *IntegrationService) ListEnabled() ([]db.IntegrationConfig, error) {
	var configs []db.IntegrationConfig
	if err := s.db.Where("enabled = ?", true).Find(&configs).Error; err != nil {
		return nil, fmt.Errorf("failed to list enabled integrations: %w", err)
	}
	return configs, nil
}

// DetectEnrichment scans enabled integrations and returns which enrichment
// services are available (Tautulli, Seerr, Plex/Jellyfin/Emby, Sonarr).
// Used by RulesService.GetFieldDefinitions() and the rule context endpoint.
func (s *IntegrationService) DetectEnrichment() EnrichmentPresence {
	configs, _ := s.ListEnabled()
	var p EnrichmentPresence
	for _, cfg := range configs {
		switch cfg.Type {
		case string(integrations.IntegrationTypeTautulli):
			p.HasTautulli = true
		case string(integrations.IntegrationTypeSeerr):
			p.HasSeerr = true
		case string(integrations.IntegrationTypePlex), string(integrations.IntegrationTypeJellyfin), string(integrations.IntegrationTypeEmby):
			p.HasMedia = true
		case string(integrations.IntegrationTypeSonarr):
			p.HasSonarr = true
		case string(integrations.IntegrationTypeJellystat):
			p.HasJellystat = true
		case string(integrations.IntegrationTypeTracearr):
			p.HasTracearr = true
		}
	}
	return p
}

// BuildIntegrationRegistry creates an IntegrationRegistry populated with clients
// for all enabled integrations, using the factory + capability-based pattern.
// Clients are created via RegisterAllFactories and auto-discovered for their
// capabilities.
func (s *IntegrationService) BuildIntegrationRegistry() (*integrations.IntegrationRegistry, error) {
	configs, err := s.ListEnabled()
	if err != nil {
		return nil, fmt.Errorf("failed to list enabled integrations: %w", err)
	}

	// Ensure factories are registered
	integrations.RegisterAllFactories()

	registry := integrations.NewIntegrationRegistry()
	for _, cfg := range configs {
		client := integrations.CreateClient(cfg.Type, cfg.URL, cfg.APIKey)
		if client != nil {
			registry.Register(cfg.ID, client)
		}
	}

	return registry, nil
}

// UpdateSyncStatus updates the sync status fields on an integration config.
// On failure (lastError non-empty), the consecutive failure counter is
// incremented. On success, the counter is reset. This is a pure DB update
// method — health state tracking is owned by IntegrationHealthService.
func (s *IntegrationService) UpdateSyncStatus(id uint, lastSync *time.Time, lastError string) error {
	if lastError != "" {
		// Failure path: increment consecutive_failures
		result := s.db.Model(&db.IntegrationConfig{}).Where("id = ?", id).Updates(map[string]any{
			"last_sync":            lastSync,
			"last_error":           lastError,
			"consecutive_failures": gorm.Expr("consecutive_failures + 1"),
		})
		if result.Error != nil {
			return fmt.Errorf("failed to update sync status: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return ErrNotFound
		}
		return nil
	}

	// Success path: reset consecutive_failures to 0
	result := s.db.Model(&db.IntegrationConfig{}).Where("id = ?", id).Updates(map[string]any{
		"last_sync":            lastSync,
		"last_error":           lastError,
		"consecutive_failures": 0,
	})
	if result.Error != nil {
		return fmt.Errorf("failed to update sync status: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdateSyncStatusDirect updates sync status fields with explicit values.
// Used by IntegrationHealthService to write health state directly without
// the auto-increment behavior of UpdateSyncStatus.
func (s *IntegrationService) UpdateSyncStatusDirect(id uint, lastSync *time.Time, lastError string, consecutiveFailures int) error {
	result := s.db.Model(&db.IntegrationConfig{}).Where("id = ?", id).Updates(map[string]any{
		"last_sync":            lastSync,
		"last_error":           lastError,
		"consecutive_failures": consecutiveFailures,
	})
	if result.Error != nil {
		return fmt.Errorf("failed to update sync status: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdateMediaStats updates the media size and count for an integration.
func (s *IntegrationService) UpdateMediaStats(id uint, sizeBytes int64, count int) error {
	result := s.db.Model(&db.IntegrationConfig{}).Where("id = ?", id).Updates(map[string]any{
		"media_size_bytes": sizeBytes,
		"media_count":      count,
	})
	if result.Error != nil {
		return fmt.Errorf("failed to update media stats: %w", result.Error)
	}
	return nil
}

// syncStatusError is the status string for sync results that failed.
// Extracted as a constant to satisfy goconst when the same string literal
// ("error") also exists as eventKindError in the notifications subsystem.
const syncStatusError = "error"

// SyncResult holds the outcome of syncing a single integration.
type SyncResult struct {
	ID         uint                     `json:"id"`
	Name       string                   `json:"name"`
	Type       string                   `json:"type"`
	Status     string                   `json:"status"`
	Error      string                   `json:"error,omitempty"`
	DiskError  string                   `json:"diskError,omitempty"`
	DiskSpace  []integrations.DiskSpace `json:"diskSpace,omitempty"`
	MediaCount int                      `json:"mediaCount,omitempty"`
	MediaError string                   `json:"mediaError,omitempty"`
}

// SyncAll fetches data from all enabled integrations: tests connections,
// retrieves disk space (upserting DiskGroups), and counts media items.
// Uses the factory+capability pattern to discover what each integration supports.
func (s *IntegrationService) SyncAll() ([]SyncResult, error) {
	configs, err := s.ListEnabled()
	if err != nil {
		return nil, err
	}

	// Ensure factories are registered
	integrations.RegisterAllFactories()

	results := make([]SyncResult, 0, len(configs))
	for _, cfg := range configs {
		result := SyncResult{
			ID:   cfg.ID,
			Name: cfg.Name,
			Type: cfg.Type,
		}

		rawClient := integrations.CreateClient(cfg.Type, cfg.URL, cfg.APIKey)
		if rawClient == nil {
			result.Status = syncStatusError
			result.Error = "Unknown integration type"
			results = append(results, result)
			continue
		}

		// Test connection via Connectable interface
		conn, ok := rawClient.(integrations.Connectable)
		if !ok {
			result.Status = syncStatusError
			result.Error = "Integration does not support connection testing"
			results = append(results, result)
			continue
		}
		if connErr := conn.TestConnection(); connErr != nil {
			result.Status = syncStatusError
			result.Error = connErr.Error()
			if s.healthReporter != nil {
				s.healthReporter.ReportFailure(cfg.ID, connErr)
			}
			results = append(results, result)
			continue
		}
		if s.healthReporter != nil {
			s.healthReporter.ReportSuccess(cfg.ID)
		}

		// Get disk space if integration is a DiskReporter
		if reporter, ok := rawClient.(integrations.DiskReporter); ok {
			disks, diskErr := reporter.GetDiskSpace()
			if diskErr != nil {
				result.DiskError = diskErr.Error()
			} else {
				result.DiskSpace = disks
				if s.diskGroups != nil {
					for _, d := range disks {
						if _, upsertErr := s.diskGroups.Upsert(d); upsertErr != nil {
							slog.Error("Failed to upsert disk group during sync",
								"component", "services", "path", d.Path, "error", upsertErr)
							if result.DiskError == "" {
								result.DiskError = upsertErr.Error()
							}
						}
					}
				}
			}
		}

		// Get media items count if integration is a MediaSource
		if source, ok := rawClient.(integrations.MediaSource); ok {
			items, mediaErr := source.GetMediaItems()
			if mediaErr != nil {
				result.MediaError = mediaErr.Error()
			} else {
				result.MediaCount = len(items)
			}
		}

		result.Status = "ok"
		results = append(results, result)
	}

	return results, nil
}
