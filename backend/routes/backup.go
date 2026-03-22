package routes

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"capacitarr/internal/services"
)

// importSettingsRequest is the request body for POST /settings/import.
type importSettingsRequest struct {
	Payload  services.SettingsExportEnvelope `json:"payload"`
	Sections services.ImportSections         `json:"sections"`
}

// commitImportRequest is the request body for POST /settings/import/commit.
type commitImportRequest struct {
	Payload   services.SettingsExportEnvelope `json:"payload"`
	Sections  services.ImportSections         `json:"sections"`
	Overrides []services.RuleOverride         `json:"overrides"`
}

// RegisterBackupRoutes sets up the settings export/import endpoints.
func RegisterBackupRoutes(protected *echo.Group, reg *services.Registry, appVersion string) {
	protected.GET("/settings/export", func(c echo.Context) error {
		sections := parseExportSections(c.QueryParam("sections"))

		envelope, err := reg.Backup.Export(sections, appVersion)
		if err != nil {
			return apiError(c, http.StatusInternalServerError, "Failed to export settings")
		}

		now := time.Now().UTC()
		filename := fmt.Sprintf("capacitarr-settings-%s.json", now.Format("2006-01-02"))
		c.Response().Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))

		return c.JSON(http.StatusOK, envelope)
	})

	protected.POST("/settings/import", func(c echo.Context) error {
		var req importSettingsRequest
		if err := c.Bind(&req); err != nil {
			return apiError(c, http.StatusBadRequest, "Invalid request body")
		}

		if req.Payload.Version != 1 {
			return apiError(c, http.StatusBadRequest, "Unsupported export version")
		}

		result, err := reg.Backup.Import(req.Payload, req.Sections)
		if err != nil {
			return apiError(c, http.StatusInternalServerError, "Failed to import settings")
		}

		return c.JSON(http.StatusOK, result)
	})

	// Preview analyzes the export file against the current database and reports
	// how each rule would be resolved, without committing any changes.
	protected.POST("/settings/import/preview", func(c echo.Context) error {
		var req importSettingsRequest
		if err := c.Bind(&req); err != nil {
			return apiError(c, http.StatusBadRequest, "Invalid request body")
		}

		if req.Payload.Version != 1 {
			return apiError(c, http.StatusBadRequest, "Unsupported export version")
		}

		preview, err := reg.Backup.PreviewImport(req.Payload, req.Sections)
		if err != nil {
			return apiError(c, http.StatusInternalServerError, "Failed to preview import")
		}

		return c.JSON(http.StatusOK, preview)
	})

	// Commit executes the import using user-provided overrides for rule
	// integration assignments from the preview step.
	protected.POST("/settings/import/commit", func(c echo.Context) error {
		var req commitImportRequest
		if err := c.Bind(&req); err != nil {
			return apiError(c, http.StatusBadRequest, "Invalid request body")
		}

		if req.Payload.Version != 1 {
			return apiError(c, http.StatusBadRequest, "Unsupported export version")
		}

		result, err := reg.Backup.CommitImport(req.Payload, req.Sections, req.Overrides)
		if err != nil {
			return apiError(c, http.StatusInternalServerError, "Failed to commit import")
		}

		return c.JSON(http.StatusOK, result)
	})
}

// parseExportSections parses a comma-separated sections query param.
// If empty, all sections are selected.
func parseExportSections(raw string) services.ExportSections {
	if raw == "" {
		return services.ExportSections{
			Preferences:          true,
			Rules:                true,
			Integrations:         true,
			DiskGroups:           true,
			NotificationChannels: true,
		}
	}

	sections := services.ExportSections{}
	for _, s := range strings.Split(raw, ",") {
		switch strings.TrimSpace(s) {
		case "preferences":
			sections.Preferences = true
		case "rules":
			sections.Rules = true
		case "integrations":
			sections.Integrations = true
		case "diskGroups":
			sections.DiskGroups = true
		case "notifications", "notificationChannels":
			sections.NotificationChannels = true
		}
	}
	return sections
}
