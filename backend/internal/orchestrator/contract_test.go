package orchestrator

import (
	"capacitarr/internal/events"
	"capacitarr/internal/services"
)

// Compile-time check that the production services satisfy the narrow
// interfaces the orchestrator actually calls. No behavior, no skips.
var (
	_ Approval           = (*services.ApprovalService)(nil)
	_ Deletion           = (*services.DeletionService)(nil)
	_ IntegrationConfigs = (*services.IntegrationService)(nil)
	_ Sunset             = (*services.SunsetService)(nil)
	_ Publisher          = (*events.EventBus)(nil)
)
