package poller

import (
	"log/slog"

	"capacitarr/internal/db"
)

// RecoverOrphanedApprovals reverts any audit log entries stuck in "Approved"
// status back to "Queued for Approval". This handles the case where a user
// approved an item but the deletion worker never processed it (e.g. the
// server restarted before the deletion queue was drained). Called on startup
// and at the beginning of each poll cycle.
func RecoverOrphanedApprovals() {
	type orphan struct {
		ID        uint
		MediaName string
	}

	var orphans []orphan
	if err := db.DB.Raw("SELECT id, media_name FROM audit_logs WHERE action = ?", db.ActionApproved).Scan(&orphans).Error; err != nil {
		slog.Error("Failed to query orphaned approvals", "component", "poller", "operation", "recover_orphans", "error", err)
		return
	}

	if len(orphans) == 0 {
		return
	}

	for _, o := range orphans {
		slog.Info("Recovering orphaned approval back to queue",
			"component", "poller",
			"id", o.ID,
			"media", o.MediaName,
		)
	}

	if err := db.DB.Exec("UPDATE audit_logs SET action = ? WHERE action = ?", db.ActionQueuedForApproval, db.ActionApproved).Error; err != nil {
		slog.Error("Failed to revert orphaned approvals", "component", "poller", "operation", "recover_orphans", "error", err)
		return
	}

	slog.Info("Orphaned approvals recovered", "component", "poller", "count", len(orphans))
}
