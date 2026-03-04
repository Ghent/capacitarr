// Package poller orchestrates periodic media library polling and capacity evaluation.
package poller

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"golang.org/x/time/rate"
	"gorm.io/gorm"

	"capacitarr/internal/db"
	"capacitarr/internal/engine"
	"capacitarr/internal/integrations"
	"capacitarr/internal/notifications"
)

type deleteJob struct {
	client  integrations.Integration
	item    integrations.MediaItem
	reason  string
	score   float64
	factors []engine.ScoreFactor
}

var deleteQueue = make(chan deleteJob, 500)

// QueueDeletion enqueues a media item for background deletion. Returns an error
// if the queue is full. Used by the approval route to process approved items.
func QueueDeletion(client integrations.Integration, item integrations.MediaItem, reason string, score float64, factors []engine.ScoreFactor) error {
	select {
	case deleteQueue <- deleteJob{
		client:  client,
		item:    item,
		reason:  reason,
		score:   score,
		factors: factors,
	}:
		return nil
	default:
		return fmt.Errorf("deletion queue is full")
	}
}

var (
	metricsProcessed int64
	metricsFailed    int64

	// Currently-deleting item name (atomic.Value storing string)
	currentlyDeletingVal atomic.Value
)

// init starts the background deletion worker before anything else.
func init() {
	go deletionWorker()
}

func deletionWorker() {
	// Rate limit: 1 deletion every 3 seconds to protect disk I/O, burst of 1.
	// This is much smarter than arbitrary sleeps, as it smooths out load dynamically.
	limiter := rate.NewLimiter(rate.Every(3*time.Second), 1)

	for job := range deleteQueue {
		// Wait blocks until a token is available
		_ = limiter.Wait(context.Background()) //nolint:errcheck // context.Background() never cancels

		currentlyDeletingVal.Store(job.item.Title)

		// Check whether actual deletions are enabled via user preference
		var prefs db.PreferenceSet
		deletionsEnabled := false
		if err := db.DB.First(&prefs, 1).Error; err == nil {
			deletionsEnabled = prefs.DeletionsEnabled
		}

		factorsJSON, _ := json.Marshal(job.factors) //nolint:errcheck // marshal of known-safe struct

		if !deletionsEnabled {
			// Dry-Delete: log but do not actually remove the file
			slog.Warn("Dry-Delete: deletions disabled in settings",
				"component", "poller",
				"item", job.item.Title,
				"type", job.item.Type,
				"size", job.item.SizeBytes,
				"score", job.score,
			)
			currentlyDeletingVal.Store("")
			atomic.AddInt64(&metricsProcessed, 1)

			logEntry := db.AuditLog{
				MediaName:    job.item.Title,
				MediaType:    string(job.item.Type),
				Reason:       fmt.Sprintf("Score: %.2f (%s)", job.score, job.reason),
				ScoreDetails: string(factorsJSON),
				Action:       "Dry-Delete",
				SizeBytes:    job.item.SizeBytes,
				CreatedAt:    time.Now(),
			}
			if err := db.DB.Create(&logEntry).Error; err != nil {
				slog.Error("Failed to create audit log entry", "component", "poller", "operation", "create_audit_log", "error", err)
			}

			// Notify: deletion executed (dry-delete)
			notifications.Dispatch(notifications.NotificationEvent{
				Type:    notifications.EventDeletionExecuted,
				Title:   "Deletion Executed (Dry-Delete)",
				Message: fmt.Sprintf("%s flagged for removal (dry-delete mode, score: %.2f)", job.item.Title, job.score),
				Fields: map[string]string{
					"Media":  job.item.Title,
					"Action": "Dry-Delete",
					"Score":  fmt.Sprintf("%.2f", job.score),
					"Size":   fmt.Sprintf("%d bytes", job.item.SizeBytes),
				},
			})

			slog.Info("Background engine action completed", "component", "poller",
				"media", job.item.Title, "action", "Dry-Delete", "freed", job.item.SizeBytes)
			continue
		}

		// Actual deletion path
		if err := job.client.DeleteMediaItem(job.item); err != nil {
			slog.Error("Background deletion failed", "component", "poller", "operation", "delete_media", "item", job.item.Title, "error", err)
			atomic.AddInt64(&metricsFailed, 1)
			currentlyDeletingVal.Store("")
			continue
		}

		currentlyDeletingVal.Store("")
		atomic.AddInt64(&metricsProcessed, 1)

		// Increment lifetime stats (atomic DB update, not for dry-runs)
		db.DB.Model(&db.LifetimeStats{}).Where("id = 1").
			UpdateColumns(map[string]interface{}{
				"total_bytes_reclaimed": gorm.Expr("total_bytes_reclaimed + ?", job.item.SizeBytes),
				"total_items_removed":   gorm.Expr("total_items_removed + ?", 1),
			})

		logEntry := db.AuditLog{
			MediaName:    job.item.Title,
			MediaType:    string(job.item.Type),
			Reason:       fmt.Sprintf("Score: %.2f (%s)", job.score, job.reason),
			ScoreDetails: string(factorsJSON),
			Action:       "Deleted",
			SizeBytes:    job.item.SizeBytes,
			CreatedAt:    time.Now(),
		}
		if err := db.DB.Create(&logEntry).Error; err != nil {
			slog.Error("Failed to create audit log entry", "component", "poller", "operation", "create_audit_log", "error", err)
		}

		// Notify: deletion executed (actual)
		notifications.Dispatch(notifications.NotificationEvent{
			Type:    notifications.EventDeletionExecuted,
			Title:   "Deletion Executed",
			Message: fmt.Sprintf("%s was deleted (score: %.2f)", job.item.Title, job.score),
			Fields: map[string]string{
				"Media":  job.item.Title,
				"Action": "Deleted",
				"Score":  fmt.Sprintf("%.2f", job.score),
				"Size":   fmt.Sprintf("%d bytes", job.item.SizeBytes),
			},
		})

		slog.Info("Background engine action completed", "component", "poller",
			"media", job.item.Title, "action", "Deleted", "freed", job.item.SizeBytes)
	}
}
