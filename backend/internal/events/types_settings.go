package events

import "fmt"

// =============================================================================
// Settings Events
// =============================================================================

// SettingsChangedEvent is published when preferences are saved.
type SettingsChangedEvent struct {
	Changes map[string]any `json:"changes,omitempty"` // Fields that changed
}

// EventType implements Event.
func (e SettingsChangedEvent) EventType() string { return "settings_changed" }

// EventMessage implements Event.
func (e SettingsChangedEvent) EventMessage() string { return "Settings updated" }

// ThresholdChangedEvent is published when disk group thresholds are updated.
type ThresholdChangedEvent struct {
	MountPath    string  `json:"mountPath"`
	ThresholdPct float64 `json:"thresholdPct"`
	TargetPct    float64 `json:"targetPct"`
}

// DiskGroupModeChangedEvent is published when a disk group's execution
// preset changes. Important-tier; per-group (spec §7 / §10.3).
type DiskGroupModeChangedEvent struct {
	DiskGroupID uint   `json:"diskGroupId"`
	MountPath   string `json:"mountPath"`
	OldMode     string `json:"oldMode"`
	NewMode     string `json:"newMode"`
}

// EventType implements Event.
func (e DiskGroupModeChangedEvent) EventType() string { return "mode_changed" }

// EventMessage implements Event.
func (e DiskGroupModeChangedEvent) EventMessage() string {
	return fmt.Sprintf("Mode changed for %s: %s → %s", e.MountPath, e.OldMode, e.NewMode)
}

// EventType implements Event.
func (e ThresholdChangedEvent) EventType() string { return "threshold_changed" }

// EventMessage implements Event.
func (e ThresholdChangedEvent) EventMessage() string {
	return fmt.Sprintf("Thresholds updated for %s: trigger at %.0f%%, target %.0f%%",
		e.MountPath, e.ThresholdPct, e.TargetPct)
}

// SettingsExportedEvent is published when settings are exported.
type SettingsExportedEvent struct {
	Sections []string `json:"sections"`
}

// EventType implements Event.
func (e SettingsExportedEvent) EventType() string { return "settings_exported" }

// EventMessage implements Event.
func (e SettingsExportedEvent) EventMessage() string {
	return fmt.Sprintf("Settings exported: %v", e.Sections)
}

// SettingsImportedEvent is published when settings are imported.
type SettingsImportedEvent struct {
	Sections []string       `json:"sections"`
	Result   map[string]any `json:"result"`
}

// EventType implements Event.
func (e SettingsImportedEvent) EventType() string { return "settings_imported" }

// EventMessage implements Event.
func (e SettingsImportedEvent) EventMessage() string {
	return fmt.Sprintf("Settings imported: %v", e.Sections)
}
