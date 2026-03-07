package services

import (
	"errors"
	"fmt"
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

// DiskGroupUpserter provides write access to disk groups.
// Defined here to avoid import cycles between IntegrationService and SettingsService.
type DiskGroupUpserter interface {
	UpsertDiskGroup(disk integrations.DiskSpace) (*db.DiskGroup, error)
}

// IntegrationService manages integration CRUD, connection testing, and
// external API lookups (rule values, quality profiles, tags, languages).
// It also owns the RuleValueCache for caching external API responses.
type IntegrationService struct {
	db             *gorm.DB
	bus            *events.EventBus
	settings       DiskGroupUpserter
	ruleValueCache *cache.TTLCache
}

// SetSettingsService wires the SettingsService dependency for disk group upserts.
// Called by Registry after construction to avoid circular initialization.
func (s *IntegrationService) SetSettingsService(settings DiskGroupUpserter) {
	s.settings = settings
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
// On success/failure, the appropriate event is published to the event bus.
func (s *IntegrationService) TestConnection(intType, url, apiKey string, integrationID *int) TestConnectionResult {
	// Resolve masked or empty API keys to the stored value
	if (apiKey == "" || db.IsMaskedKey(apiKey)) && integrationID != nil && *integrationID > 0 {
		existing, err := s.GetByID(uint(*integrationID))
		if err == nil {
			apiKey = existing.APIKey
		}
	}

	// Enrichment-only services have separate client constructors
	switch intType {
	case "tautulli":
		client := integrations.NewTautulliClient(url, apiKey)
		if err := client.TestConnection(); err != nil {
			s.PublishTestFailure(intType, intType, url, err.Error())
			return TestConnectionResult{Success: false, Error: err.Error()}
		}
		s.PublishTestSuccess(intType, intType, url)
		return TestConnectionResult{Success: true, Message: "Connection successful"}

	case "overseerr":
		client := integrations.NewOverseerrClient(url, apiKey)
		if err := client.TestConnection(); err != nil {
			s.PublishTestFailure(intType, intType, url, err.Error())
			return TestConnectionResult{Success: false, Error: err.Error()}
		}
		s.PublishTestSuccess(intType, intType, url)
		return TestConnectionResult{Success: true, Message: "Connection successful"}

	case "jellyfin":
		client := integrations.NewJellyfinClient(url, apiKey)
		if err := client.TestConnection(); err != nil {
			s.PublishTestFailure(intType, intType, url, err.Error())
			return TestConnectionResult{Success: false, Error: err.Error()}
		}
		s.PublishTestSuccess(intType, intType, url)
		return TestConnectionResult{Success: true, Message: "Connection successful"}

	case "emby":
		client := integrations.NewEmbyClient(url, apiKey)
		if err := client.TestConnection(); err != nil {
			s.PublishTestFailure(intType, intType, url, err.Error())
			return TestConnectionResult{Success: false, Error: err.Error()}
		}
		s.PublishTestSuccess(intType, intType, url)
		return TestConnectionResult{Success: true, Message: "Connection successful"}

	case "plex":
		client := integrations.NewPlexClient(url, apiKey)
		if err := client.TestConnection(); err != nil {
			s.PublishTestFailure(intType, intType, url, err.Error())
			return TestConnectionResult{Success: false, Error: err.Error()}
		}
		s.PublishTestSuccess(intType, intType, url)
		return TestConnectionResult{Success: true, Message: "Connection successful"}
	}

	// Standard *arr integrations
	client := integrations.NewClient(intType, url, apiKey)
	if client == nil {
		return TestConnectionResult{Success: false, Error: "Unknown integration type"}
	}

	if err := client.TestConnection(); err != nil {
		s.PublishTestFailure(intType, intType, url, err.Error())
		return TestConnectionResult{Success: false, Error: err.Error()}
	}

	// Invalidate rule value cache on successful test
	if integrationID != nil {
		s.InvalidateRuleValueCache(*integrationID)
	}

	s.PublishTestSuccess(intType, intType, url)
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
	case "seriesstatus":
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

	client := integrations.NewClient(cfg.Type, cfg.URL, cfg.APIKey)
	if client == nil {
		return nil, ErrUnsupportedIntegrationType
	}

	fetcher, ok := client.(integrations.RuleValueFetcher)
	if !ok {
		return nil, ErrIntegrationNoRuleValues
	}

	var result map[string]any

	switch action {
	case "quality":
		profiles, fetchErr := fetcher.GetQualityProfiles()
		if fetchErr != nil {
			return nil, fmt.Errorf("failed to fetch quality profiles: %w", fetchErr)
		}
		result = map[string]any{"type": "closed", "options": profiles}

	case "tag":
		tags, fetchErr := fetcher.GetTags()
		if fetchErr != nil {
			return nil, fmt.Errorf("failed to fetch tags: %w", fetchErr)
		}
		result = map[string]any{"type": "combobox", "suggestions": tags}

	case "genre":
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

	case "language":
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

// Delete removes an integration config.
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
func (s *IntegrationService) SyncAll() ([]SyncResult, error) {
	configs, err := s.ListEnabled()
	if err != nil {
		return nil, err
	}

	results := make([]SyncResult, 0, len(configs))
	for _, cfg := range configs {
		client := integrations.NewClient(cfg.Type, cfg.URL, cfg.APIKey)
		if client == nil {
			continue
		}

		result := SyncResult{
			ID:   cfg.ID,
			Name: cfg.Name,
			Type: cfg.Type,
		}

		// Test connection
		if connErr := client.TestConnection(); connErr != nil {
			result.Status = "error"
			result.Error = connErr.Error()
			results = append(results, result)
			continue
		}

		// Get disk space
		disks, diskErr := client.GetDiskSpace()
		if diskErr != nil {
			result.DiskError = diskErr.Error()
		} else {
			result.DiskSpace = disks
			for _, d := range disks {
				_, _ = s.settings.UpsertDiskGroup(d)
			}
		}

		// Get media items count
		items, mediaErr := client.GetMediaItems()
		if mediaErr != nil {
			result.MediaError = mediaErr.Error()
		} else {
			result.MediaCount = len(items)
		}

		result.Status = "ok"
		results = append(results, result)
	}

	return results, nil
}
