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
)

// Rule value action identifiers used in FetchRuleValues switch.
const (
	ruleActionQuality    = "quality"
	ruleActionTag        = "tag"
	ruleActionGenre      = "genre"
	ruleActionLanguage   = "language"
	ruleActionCollection = "collection"
)

// DiskGroupManager provides disk group operations needed by IntegrationService.
// Defined here to avoid import cycles between IntegrationService and DiskGroupService.
type DiskGroupManager interface {
	Upsert(disk integrations.DiskSpace) (*db.DiskGroup, error)
	RemoveAll() (int64, error)
}

// IntegrationService manages integration CRUD, connection testing, and
// external API lookups (rule values, quality profiles, tags, languages).
// It also owns the RuleValueCache for caching external API responses.
type IntegrationService struct {
	db             *gorm.DB
	bus            *events.EventBus
	diskGroups     DiskGroupManager
	ruleValueCache *cache.TTLCache
}

// SetDiskGroupService wires the DiskGroupService dependency for disk group operations.
// Called by Registry after construction to avoid circular initialization.
func (s *IntegrationService) SetDiskGroupService(dg DiskGroupManager) {
	s.diskGroups = dg
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

// FetchCollectionValues returns collection names from all enabled Plex integrations.
// Results are cached with the standard TTL. The returned slice is sorted alphabetically.
// Uses PlexClient.GetCollectionNames() which handles deduplication and sorting internally.
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

	// Collect unique collection names across all Plex integrations.
	seen := make(map[string]bool)
	for _, cfg := range configs {
		if cfg.Type != string(integrations.IntegrationTypePlex) {
			continue
		}

		client := integrations.NewPlexClient(cfg.URL, cfg.APIKey)
		names, fetchErr := client.GetCollectionNames()
		if fetchErr != nil {
			slog.Warn("Failed to fetch Plex collection names",
				"component", "integration_service", "integrationId", cfg.ID, "error", fetchErr)
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

// testClient runs a connection test, publishes success/failure events, and returns the result.
// This helper eliminates repetition across enrichment-only and standard integration types.
func (s *IntegrationService) testClient(intType, url string, testFn func() error) TestConnectionResult {
	if err := testFn(); err != nil {
		s.PublishTestFailure(intType, intType, url, err.Error())
		return TestConnectionResult{Success: false, Error: err.Error()}
	}
	s.PublishTestSuccess(intType, intType, url)
	return TestConnectionResult{Success: true, Message: "Connection successful"}
}

// TestConnection tests connectivity to an integration given a type, URL, and API key.
// If apiKey is empty or masked and integrationID is provided, the stored key is used.
// On success/failure, the appropriate event is published to the event bus.
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

	result := s.testClient(intType, url, conn.TestConnection)

	// Invalidate rule value cache on successful test of *arr integrations
	if result.Success && integrationID != nil {
		s.InvalidateRuleValueCache(*integrationID)
	}

	return result
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
			"type": "closed",
			"options": []integrations.NameValue{
				{Value: "continuing", Label: "Continuing"},
				{Value: "ended", Label: "Ended"},
				{Value: "upcoming", Label: "Upcoming"},
				{Value: "deleted", Label: "Deleted"},
			},
		}
		s.ruleValueCache.Set(cacheKey, result)
		return result, nil

	case "monitored", "requested", "incollection", "watchedbyreq":
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
			"type": "closed",
			"options": []integrations.NameValue{
				{Value: "movie", Label: "Movie"},
				{Value: "show", Label: "Show"},
				{Value: "season", Label: "Season"},
				{Value: "artist", Label: "Artist"},
				{Value: "book", Label: "Book"},
			},
		}
		s.ruleValueCache.Set(cacheKey, result)
		return result, nil

	// Collection field — aggregates from all enabled Plex integrations (not per-integration)
	case ruleActionCollection:
		collections, collectErr := s.FetchCollectionValues()
		if collectErr != nil {
			return nil, fmt.Errorf("failed to fetch collection values: %w", collectErr)
		}
		result := map[string]any{"type": "combobox", "suggestions": collections}
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
			"type": "combobox",
			"suggestions": []integrations.NameValue{
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
			},
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

// Update modifies an existing integration config.
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
			"component", "integration_service", "error", err)
		return nil // Integration was deleted, don't fail the request
	}
	if len(remaining) == 0 && s.diskGroups != nil {
		if removed, rmErr := s.diskGroups.RemoveAll(); rmErr != nil {
			slog.Error("Failed to remove disk groups after last integration deleted",
				"component", "integration_service", "error", rmErr)
		} else if removed > 0 {
			slog.Info("Removed all disk groups after last integration deleted",
				"component", "integration_service", "count", removed)
		}
	}

	return nil
}

// PublishTestSuccess publishes a successful connection test event.
func (s *IntegrationService) PublishTestSuccess(intType, name, url string) {
	s.bus.Publish(events.IntegrationTestEvent{
		IntegrationType: intType,
		Name:            name,
		URL:             url,
	})
}

// PublishTestFailure publishes a failed connection test event.
func (s *IntegrationService) PublishTestFailure(intType, name, url, errMsg string) {
	s.bus.Publish(events.IntegrationTestFailedEvent{
		IntegrationType: intType,
		Name:            name,
		URL:             url,
		Error:           errMsg,
	})
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

// UpdateSyncStatus updates the last_sync and last_error fields on an integration config.
func (s *IntegrationService) UpdateSyncStatus(id uint, lastSync *time.Time, lastError string) error {
	result := s.db.Model(&db.IntegrationConfig{}).Where("id = ?", id).Updates(map[string]any{
		"last_sync":  lastSync,
		"last_error": lastError,
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
			result.Status = "error"
			result.Error = "Unknown integration type"
			results = append(results, result)
			continue
		}

		// Test connection via Connectable interface
		conn, ok := rawClient.(integrations.Connectable)
		if !ok {
			result.Status = "error"
			result.Error = "Integration does not support connection testing"
			results = append(results, result)
			continue
		}
		if connErr := conn.TestConnection(); connErr != nil {
			result.Status = "error"
			result.Error = connErr.Error()
			results = append(results, result)
			continue
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
						_, _ = s.diskGroups.Upsert(d)
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
