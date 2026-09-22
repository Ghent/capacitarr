package events

import "testing"

// TestEventTypeAndMessage is the single table-driven ceremony test for
// EventType()/EventMessage() string contracts. Copies in services tests
// were deleted; if a message format changes, update this table.
func TestEventTypeAndMessage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		event   Event
		wantTyp string
		wantMsg string
	}{
		{
			name: "deletion progress",
			event: DeletionProgressEvent{
				CurrentItem: "Serenity",
				QueueDepth:  3,
				Processed:   2,
				Succeeded:   1,
				Failed:      1,
				BatchTotal:  5,
			},
			wantTyp: "deletion_progress",
			wantMsg: "Deletion progress: 2/5 completed (1 succeeded, 1 failed)",
		},
		{
			name: "deletion cancelled",
			event: DeletionCancelledEvent{
				MediaName: "Firefly",
				MediaType: "show",
				SizeBytes: 1024,
			},
			wantTyp: "deletion_cancelled",
			wantMsg: "Deletion cancelled: Firefly",
		},
		{
			name: "deletion queued",
			event: DeletionQueuedEvent{
				MediaName:     "Serenity",
				MediaType:     "movie",
				SizeBytes:     1024 * 1024 * 100,
				IntegrationID: 1,
			},
			wantTyp: "deletion_queued",
			wantMsg: "Queued for deletion: Serenity",
		},
		{
			name: "deletion grace period active",
			event: DeletionGracePeriodEvent{
				RemainingSeconds: 25,
				QueueSize:        3,
				Active:           true,
			},
			wantTyp: "deletion_grace_period",
			wantMsg: "Deletion grace period active: 25s remaining, 3 items queued",
		},
		{
			name: "deletion grace period expired",
			event: DeletionGracePeriodEvent{
				RemainingSeconds: 0,
				QueueSize:        3,
				Active:           false,
			},
			wantTyp: "deletion_grace_period",
			wantMsg: "Deletion grace period expired: processing 3 items",
		},
		{
			name: "approval returned to pending",
			event: ApprovalReturnedToPendingEvent{
				EntryID:   1,
				MediaName: "Firefly",
				MediaType: "show",
			},
			wantTyp: "approval_returned_to_pending",
			wantMsg: "Returned to pending after dry-delete: Firefly",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.event.EventType(); got != tt.wantTyp {
				t.Errorf("EventType() = %q, want %q", got, tt.wantTyp)
			}
			if got := tt.event.EventMessage(); got != tt.wantMsg {
				t.Errorf("EventMessage() = %q, want %q", got, tt.wantMsg)
			}
		})
	}
}
