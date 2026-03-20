# Deletion Pipeline Unification

**Created:** 2026-03-20T20:34Z
**Status:** 🔄 In Progress (Phase 2 complete)
**Base Branch:** `feature/2.0`
**Breaking:** Yes — no backward compatibility required (2.0 baseline migration)

## Overview

This plan unifies the deletion pipeline by:

1. **Replacing force-delete with mode-aware delete** — user-initiated deletions follow the same pipeline as engine-triggered deletions
2. **Routing poller dry-run through the DeletionService** — eliminating the bypass that writes directly to the audit log
3. **Adding a configurable grace period** to the deletion queue — items accumulate and are visible before processing starts
4. **Making the deletion queue card always visible** — not just in approval mode
5. **Dropping the `Reason` string field** — replacing it with structured, machine-readable fields
6. **Consolidating audit actions** — unifying `dry_run` and `dry_delete` into a single action

## Architecture: Current vs Proposed

### Current Architecture

```mermaid
flowchart TD
    subgraph "Engine Poller"
        EVAL["evaluateAndCleanDisk()"]
        EVAL --> MODE{executionMode?}
        MODE -->|auto| AUTO_Q["DeletionService.QueueDeletion()"]
        MODE -->|approval| APPROVAL_Q["ApprovalService.UpsertPending()"]
        MODE -->|dry-run| DIRECT_AUDIT["AuditLog.UpsertDryRun()<br/>BYPASSES DeletionService"]
    end

    subgraph "Force Delete (separate path)"
        FD_API["POST /force-delete"]
        FD_API --> FD_CREATE["ApprovalService.CreateForceDelete()<br/>status=approved, force_delete=true"]
        FD_CREATE --> FD_WAIT["Waits for next engine cycle"]
        FD_WAIT --> FD_POLLER["processForceDeletes()"]
        FD_POLLER --> AUTO_Q
    end

    AUTO_Q --> WORKER["DeletionService.worker()<br/>Processes immediately"]
    APPROVAL_Q --> USER_APPROVE["User approves"]
    USER_APPROVE --> AUTO_Q
```

**Problems:**
- Dry-run bypasses DeletionService — no SSE events, no progress tracking, no rate limiting
- Force-delete is a separate pipeline with its own DB flag, poller method, and API endpoint
- Two different audit actions for dry-runs: `dry_run` (poller direct) vs `dry_delete` (DeletionService)
- `Reason` field stores formatted strings mixing score data, factor summaries, and trigger context
- Deletion queue card only visible in approval mode
- No grace period — items process immediately with no review window

### Proposed Architecture

```mermaid
flowchart TD
    subgraph "Initiation (any source)"
        ENGINE["Engine Poller"]
        USER["User (Delete action)"]
    end

    subgraph "Mode Routing"
        ENGINE --> MODE{executionMode?}
        USER --> MODE
        MODE -->|approval| APPROVAL_Q["ApprovalService.UpsertPending()<br/>user_initiated flag for user actions"]
        MODE -->|auto / dry-run| DELETION_Q["DeletionService.QueueDeletion()"]
    end

    subgraph "Approval Flow"
        APPROVAL_Q -->|user approves| DELETION_Q
    end

    subgraph "Deletion Queue (always visible)"
        DELETION_Q --> GRACE["Grace period (configurable)<br/>Timer resets on any queue change"]
        GRACE -->|timer expires| STREAM["Stream processing<br/>Rate-limited, one at a time"]
    end

    subgraph "Processing"
        STREAM --> BOOL{DeletionsEnabled<br/>AND mode != dry-run?}
        BOOL -->|yes| DELETE["Actual *arr API deletion<br/>Audit log + stats + SSE"]
        BOOL -->|no| DRY["Dry-delete simulation<br/>Audit log + SSE"]
        DRY --> RETURN{Was in approval mode?}
        RETURN -->|yes| RETURN_AQ["Return to approval queue<br/>as pending item"]
        RETURN -->|no| DONE["Discard"]
    end
```

## Phase 1: Unify Dry-Run Through DeletionService ✅

Route the poller's dry-run path through the DeletionService instead of writing directly to the audit log.

**Completed:** 2026-03-20T21:16Z
**Branch:** `refactor/deletion-pipeline-unification`

### Step 1.1: Add `UpsertAudit` flag to `DeleteJob` ✅

**File:** `internal/services/deletion.go`

Added `UpsertAudit bool` field to the `DeleteJob` struct. When `true`, the dry-delete branch in `processJob()` uses `AuditLog.UpsertDryRun()` (upsert semantics for poller dry-runs that repeat every cycle). When `false`, it uses `AuditLog.Create()` (append-only for user-initiated and approval-mode deletions).

### Step 1.2: Update `processJob()` dry-delete branch ✅

**File:** `internal/services/deletion.go`

Updated the `!deletionsEnabled || job.ForceDryRun` branch of `processJob()` to check `job.UpsertAudit` to decide between `AuditLog.UpsertDryRun()` and `AuditLog.Create()`. Also added nil-safety check for `job.Client` before the actual deletion call — nil client now correctly fails with a logged error instead of panicking.

### Step 1.3: Route poller dry-run through DeletionService ✅

**File:** `internal/poller/evaluate.go`

Replaced the direct `AuditLog.UpsertDryRun()` call in the dry-run branch with `DeletionService.QueueDeletion()` using `ForceDryRun=true`, `UpsertAudit=true`, and `Client=nil`. The nil client is safe because dry-run never calls `DeleteMediaItem()`. The dry-run branch now increments `deletionsQueued` and tracks `lastRunFlagged`/`lastRunFreedBytes` like the auto branch.

### Step 1.4: Update `SignalBatchSize()` calls ✅

**File:** `internal/poller/poller.go`

No separate change needed — handled automatically by the Step 1.3 change. The dry-run branch now increments `deletionsQueued` which feeds into the `totalDeletionsQueued` counter that's passed to `SignalBatchSize()`. Updated the `evaluateAndCleanDisk()` docstring to reflect that it returns counts for both auto and dry-run modes.

### Step 1.5: Consolidate audit actions ✅

**Files:** `internal/db/models.go`, `internal/services/deletion.go`, `internal/services/auditlog.go`, `internal/db/migrations/00001_v2_baseline.sql`, `frontend/app/types/api.ts`, `frontend/app/components/AuditLogPanel.vue`

- Removed `ActionDryRun` constant (`"dry_run"`) from `models.go`
- Kept `ActionDryDelete` constant (`"dry_delete"`) as the single dry-run action
- Updated the `audit_log` table CHECK constraint in the baseline migration to remove `dry_run`
- Updated `AuditLog.UpsertDryRun()` to force `entry.Action = db.ActionDryDelete` and match on `db.ActionDryDelete` explicitly
- Updated Action field comment on `AuditLogEntry` struct
- Updated frontend: removed `dry_run` from `AuditAction` type, audit log filter buttons, badge variant mapping, and label function
- Updated all test references: `auditlog_test.go`, `driver_test.go`, `audit_test.go` (routes)

### Step 1.6: Update tests ✅

**Files:** `internal/services/deletion_test.go`, `internal/services/auditlog_test.go`, `internal/db/driver_test.go`, `routes/audit_test.go`

- Added `TestDeletionService_UpsertAudit_UsesUpsertSemantics` — verifies upsert produces 1 entry for repeated items
- Added `TestDeletionService_UpsertAudit_False_AppendsMultiple` — verifies append-only produces N entries
- Added `TestDeletionService_NilClient_DryRunSucceeds` — verifies nil client works in dry-run path
- Added `TestDeletionService_NilClient_ActualDeletion_Fails` — verifies nil client fails safely in actual deletion path
- Updated all existing `ActionDryRun`/`"dry_run"` references to `ActionDryDelete`/`"dry_delete"`

## Phase 2: Replace Force-Delete with Mode-Aware Delete ✅

**Completed:** 2026-03-20T21:39Z

### Step 2.1: Create `ManualDelete` service method ✅

**File:** `internal/services/approval.go`

Created `ManualDelete()` method on `ApprovalService` with supporting types `ManualDeleteItem`, `ManualDeleteDeps`, and `ManualDeleteResult`. In approval mode, items are upserted as pending with `UserInitiated=true`. In auto/dry-run mode, items are queued to the DeletionService via integration client construction and `QueueDeletion()`.

### Step 2.2: Rename `ForceDelete` to `UserInitiated` in DB model ✅

**Files:** `internal/db/models.go`, `internal/db/migrations/00001_v2_baseline.sql`

- Renamed `ForceDelete bool` → `UserInitiated bool` on `ApprovalQueueItem` with JSON tag `"userInitiated"`
- Updated baseline migration column `force_delete` → `user_initiated`
- Added execution mode constants (`ModeAuto`, `ModeDryRun`, `ModeApproval`) to `db` package to resolve goconst lint warnings
- Updated all references across Go code, including `preview.go` (queue enrichment), `score.go` (QueueStatus comment), `deletion.go` (DiskGroupID comment)

### Step 2.3: Update `ClearQueue()` and `ClearQueueForDiskGroup()` ✅

**File:** `internal/services/approval.go`

Updated WHERE clauses from `force_delete = ?` to `user_initiated = ?` to preserve user-initiated items on below-threshold queue clearing.

### Step 2.4: Update `ReconcileQueue()` ✅

**File:** `internal/services/approval.go`

`ListPendingForDiskGroup()` already filtered on `force_delete = false` — updated to `user_initiated = false` so user-initiated items are excluded from reconciliation pruning.

### Step 2.5: Rename API endpoint ✅

**Files:** `routes/approval.go`, `routes/deletion.go`

- Removed `POST /force-delete` handler from `routes/approval.go`
- Added `POST /delete` handler (`handleManualDelete`) to `routes/deletion.go`
- Handler reads `executionMode` and `deletionsEnabled` from preferences, calls `ManualDelete()`
- Response includes `mode` field for frontend toast messages
- Removed unused `db` import from `routes/approval.go`

### Step 2.6: Remove force-delete infrastructure ✅

**Files:** `internal/services/approval.go`, `internal/poller/evaluate.go`

- Removed `CreateForceDelete()`, `ListForceDeletes()`, `RemoveForceDelete()` methods
- Removed `processForceDeletes()` function from poller
- Removed the `processForceDeletes()` call in below-threshold path of `evaluateAndCleanDisk()`

### Step 2.7: Update frontend ✅

**Files:** `frontend/app/types/api.ts`, `frontend/app/components/LibraryTable.vue`, `frontend/app/components/MediaPosterCard.vue`, `frontend/app/pages/library.vue`, all 22 locale JSON files

- Renamed `forceDelete` → `userInitiated` on `ApprovalQueueItem` TypeScript type
- Renamed `force_delete` → `user_initiated` in `EvaluatedItem.queueStatus` union type
- Updated `MediaPosterCard.vue` queue status badge: `force_delete` → `user_initiated`, label "Force Delete" → "Delete"
- Updated `LibraryTable.vue`: emit `delete` instead of `force-delete`, function `confirmDelete()` instead of `confirmForceDelete()`, queue badge references
- Updated `library.vue`: API call `POST /delete`, mode-dependent toast messages (auto/approval/dry-run), renamed handler `handleDelete()`
- Updated all 22 locale files: renamed i18n keys from `forceDelete*` to `delete*`, added mode-specific success messages to `en.json`

### Step 2.8: Update tests ✅

**Files:** `routes/approval_test.go`, `internal/services/preview_test.go`

- Added 3 route tests: `TestManualDelete_ApprovalMode`, `TestManualDelete_DryRunMode`, `TestManualDelete_DeletionsDisabled`
- Removed 2 old force-delete route tests: `TestForceDelete_DryRunMode`, `TestForceDelete_DeletionsDisabled`
- Updated preview test: `TestPreviewService_EnrichWithQueueStatus_ForceDelete` → `TestPreviewService_EnrichWithQueueStatus_UserInitiated`
- No approval service-level force-delete tests existed to remove (they were never created for `CreateForceDelete` etc.)

## Phase 3: Deletion Queue Grace Period

### Step 3.1: Add `DeletionQueueDelaySeconds` preference

**Files:** `internal/db/models.go`, `internal/db/migrations/00001_v2_baseline.sql`

- Add `DeletionQueueDelaySeconds int` to `PreferenceSet` with default 30
- Add column to baseline migration: `deletion_queue_delay_seconds INTEGER NOT NULL DEFAULT 30`
- Validation: minimum 10, maximum 300

### Step 3.2: Implement grace period in DeletionService worker

**File:** `internal/services/deletion.go`

Replace the current immediate-processing worker loop with a grace-period-aware loop:

1. When the first item arrives in the queue (or `SignalBatchSize()` is called), start a grace period timer set to the configured delay
2. The timer resets to the full configured value on any queue mutation (item added via `QueueDeletion()`, item cancelled via `CancelDeletion()`)
3. When the timer expires, the worker begins draining the queue with the existing rate limiter
4. Once draining starts, new items added during draining are processed after the current batch (no new grace period until the queue is empty)

Implementation approach: The worker goroutine blocks on a `select` between the queue channel and a timer channel. The timer is managed by a separate method that resets on mutations.

### Step 3.3: Publish grace period state via SSE

**File:** `internal/events/types.go`, `internal/services/deletion.go`

Add a new SSE event type for the grace period countdown:

```go
type DeletionGracePeriodEvent struct {
    RemainingSeconds int  `json:"remainingSeconds"`
    QueueSize        int  `json:"queueSize"`
    Active           bool `json:"active"` // true = grace period running, false = processing started
}
```

Publish this event:
- When the grace period starts (first item queued)
- Periodically during the grace period (every second or every 5 seconds)
- When the grace period expires and processing begins

### Step 3.4: Update frontend deletion queue card

**File:** `frontend/app/components/DeletionQueueCard.vue`

- Show a countdown timer during the grace period: "Processing starts in 12s..."
- The countdown updates from SSE `DeletionGracePeriodEvent` events
- When the grace period expires, transition to the existing progress bar UI

### Step 3.5: Add grace period setting to advanced settings UI

**File:** `frontend/app/components/settings/SettingsAdvanced.vue`

- Add a slider/input for "Deletion Queue Delay" with range 10-300 seconds, default 30
- Label: "Time to wait before processing queued deletions. Resets when items are added or removed."

### Step 3.6: Implement deletion queue snooze endpoint

**Files:** `routes/deletion.go`, `internal/services/deletion.go`, `internal/services/approval.go`

Add `POST /api/v1/deletion-queue/snooze` endpoint that:

1. Calls `DeletionService.CancelDeletion(mediaName, mediaType)` to remove the item from the deletion queue
2. Creates a rejected/snoozed entry in the approval queue via `ApprovalService.CreateSnoozedEntry()` (new method) with `snoozed_until` set to `now + SnoozeDurationHours`
3. Resets the grace period timer (queue mutation)

The snoozed approval queue entry acts as a "do not re-queue" marker — the poller's `IsSnoozed()` check prevents the engine from re-queuing the item until the snooze expires. This works in all modes (auto, approval, dry-run).

New service method on `ApprovalService`:

```go
func (s *ApprovalService) CreateSnoozedEntry(item db.ApprovalQueueItem, snoozeDurationHours int) error
```

This creates a new approval queue entry with `status=rejected` and `snoozed_until` set. If an entry for the same media already exists, it updates the snooze timestamp.

### Step 3.7: Add snooze button to deletion queue card

**File:** `frontend/app/components/DeletionQueueCard.vue`

Add a snooze button (clock icon) next to the existing cancel button (X icon) on each queued item. Clicking it calls `POST /api/v1/deletion-queue/snooze`.

### Step 3.8: Update tests

**Files:** `internal/services/deletion_test.go`, `routes/deletion_test.go`

- Test grace period timer behavior: starts on first item, resets on mutation, expires and triggers processing
- Test that items queued during processing don't restart the grace period
- Test configurable delay values
- Test snooze endpoint: item removed from deletion queue, snoozed entry created in approval queue
- Test that snoozed items are not re-queued by the engine

## Phase 4: Always-Visible Deletion Queue Card

### Step 4.1: Update card visibility logic

**File:** `frontend/app/components/DeletionQueueCard.vue`

Currently the card is shown when `hasContent || isApprovalMode`. Change to always show the card regardless of mode. When empty, show an appropriate empty state message based on mode.

### Step 4.2: Update empty state messages

**File:** Frontend i18n files

Add mode-specific empty state messages:
- Auto: "No items queued for deletion"
- Approval: "Approve items from the approval queue to see them here"
- Dry-run: "No items queued for dry-run"

## Phase 5: Snoozed Items Card

Extract snoozed items from the approval queue card into a dedicated, always-mode-aware card. This card is visible in all execution modes (auto, approval, dry-run) and only renders when snoozed items exist.

### Step 5.1: Create `SnoozedItemsCard.vue` component

**File:** `frontend/app/components/SnoozedItemsCard.vue`

Create a new card component that:
- Fetches snoozed items from the API (`GET /api/v1/approval-queue?status=rejected` filtered to items with active `snoozedUntil`)
- Displays each item with: media name, type, size, snooze expiration countdown ("Unsnoozed in 18h"), and an unsnooze button
- Uses `v-motion` animations for smooth enter/leave transitions (spring stiffness 260, damping 24 — matching existing cards)
- Animates individual items in/out when snoozes are added or expire
- Hidden when no snoozed items exist (no empty state — card simply doesn't render)
- Calls `POST /api/v1/approval-queue/:id/unsnooze` when the unsnooze button is clicked

### Step 5.2: Remove snoozed section from `ApprovalQueueCard.vue`

**File:** `frontend/app/components/ApprovalQueueCard.vue`

- Remove the `snoozedItems` section and related jump bar navigation
- Remove the `snoozedSectionRef` and IntersectionObserver logic for the snoozed section
- The approval card becomes purely "pending items awaiting review"
- Update the `totalCount` computed to only count pending items

### Step 5.3: Add `SnoozedItemsCard` to dashboard layout

**File:** Dashboard page component

Add the `SnoozedItemsCard` to the dashboard layout, positioned between the approval queue card and the deletion queue card (or after both — determine best visual flow). The card auto-hides when empty via `v-if`.

### Step 5.4: Create `useSnoozedItems` composable

**File:** `frontend/app/composables/useSnoozedItems.ts`

Create a composable that:
- Fetches snoozed items from the API
- Provides reactive `snoozedItems` list
- Provides `unsnooze(id)` method
- Subscribes to SSE events for real-time updates when snoozes are added/expire
- Auto-refreshes on engine run completion (via `runCompletionCounter` from `useEngineControl`)

### Step 5.5: Extend `IsSnoozed()` check to all execution modes

**File:** `internal/poller/evaluate.go`

Currently the `IsSnoozed()` check at line 180 only runs in approval mode. Move it to run for **all modes** (before the mode-specific branching) so that items snoozed from the deletion queue in auto/dry-run mode are respected.

### Step 5.6: Update tests

**Files:** Frontend component tests, `internal/poller/evaluate_test.go`

- Test that `IsSnoozed()` is checked in auto and dry-run modes
- Test that the snoozed card renders when items exist and hides when empty
- Test unsnooze action from the snoozed card

## Phase 6: Dry-Run Return to Approval Queue

### Step 6.1: Track approval source in `DeleteJob`

**File:** `internal/services/deletion.go`

Add a field to `DeleteJob` to track whether the item came from the approval queue:

```go
type DeleteJob struct {
    // ... existing fields ...
    ApprovalEntryID uint // Non-zero if this job originated from an approval queue item
}
```

### Step 6.2: Return dry-deleted items to approval queue

**File:** `internal/services/deletion.go`

In the dry-delete branch of `processJob()`, after logging the audit entry, check if `job.ApprovalEntryID != 0`. If so, call `ApprovalService.ReturnToPending(job.ApprovalEntryID)` to reset the item back to pending status.

### Step 6.3: Add `ReturnToPending` method

**File:** `internal/services/approval.go`

Create a method that resets an approved item back to pending:

```go
func (s *ApprovalService) ReturnToPending(entryID uint) error
```

This updates the status from `approved` back to `pending` and publishes an appropriate event.

### Step 6.4: Wire `ApprovalEntryID` through `ExecuteApproval()`

**File:** `internal/services/approval.go`

When `ExecuteApproval()` queues a `DeleteJob`, set `ApprovalEntryID` to the approval queue item's ID so the DeletionService can return it if dry-deleted.

### Step 6.5: Add `ApprovalService` dependency to `DeletionService`

**File:** `internal/services/deletion.go`, `internal/services/registry.go`

The DeletionService needs to call `ApprovalService.ReturnToPending()`. Add this as a dependency via `SetDependencies()` to avoid circular initialization.

### Step 6.6: Update tests

**Files:** `internal/services/deletion_test.go`, `internal/services/approval_test.go`

- Test that dry-deleted items with `ApprovalEntryID` are returned to pending
- Test that dry-deleted items without `ApprovalEntryID` are not returned
- Test the intentional loop: approve → dry-delete → return to pending → approve again

## Phase 7: Drop `Reason` Field, Add Structured Fields

### Step 7.1: Add structured fields to `AuditLogEntry`

**Files:** `internal/db/models.go`, `internal/db/migrations/00001_v2_baseline.sql`

Add new columns to `audit_log`:

```go
type AuditLogEntry struct {
    // ... existing fields (minus Reason) ...
    Trigger      string `gorm:"not null;default:'engine'" json:"trigger"`      // "engine", "user", "approval"
    DryRunReason string `gorm:"not null;default:''" json:"dryRunReason"`       // "deletions_disabled", "execution_mode", "" (empty if not dry-run)
}
```

Migration:

```sql
-- Remove reason column
-- Add trigger column: "engine", "user", "approval"
-- Add dry_run_reason column: "deletions_disabled", "execution_mode", ""
```

### Step 7.2: Remove `Reason` field from `AuditLogEntry`

**Files:** `internal/db/models.go`, `internal/db/migrations/00001_v2_baseline.sql`

Drop the `reason` column from the `audit_log` table. Since this is a breaking branch, no migration path needed — update the baseline migration directly.

### Step 7.3: Remove `Reason` field from `ApprovalQueueItem`

**Files:** `internal/db/models.go`, `internal/db/migrations/00001_v2_baseline.sql`

Drop the `reason` column from the `approval_queue` table. The approval queue already has `Score` and `ScoreDetails` fields that contain the same data in structured form.

### Step 7.4: Update all audit entry construction sites

**Files:** `internal/services/deletion.go`, `internal/poller/evaluate.go` (if any direct writes remain)

Replace `Reason: fmt.Sprintf(...)` with:
- `Trigger: "engine"` / `"user"` / `"approval"`
- `DryRunReason: "deletions_disabled"` / `"execution_mode"` / `""`

The `Trigger` value needs to be passed through the pipeline. Add a `Trigger string` field to `DeleteJob`.

### Step 7.5: Update `DeleteJob` struct

**File:** `internal/services/deletion.go`

Add `Trigger string` field to `DeleteJob`. Set by:
- Poller: `"engine"`
- `ManualDelete()`: `"user"`
- `ExecuteApproval()`: `"approval"`

### Step 7.6: Update `processJob()` to populate structured fields

**File:** `internal/services/deletion.go`

In both the dry-delete and real-delete branches, populate `Trigger` and `DryRunReason` on the audit entry:

```go
logEntry := db.AuditLogEntry{
    // ... existing fields ...
    Trigger:      job.Trigger,
    DryRunReason: determineDryRunReason(deletionsEnabled, job.ForceDryRun),
}
```

Where `determineDryRunReason()` returns:
- `"deletions_disabled"` if `!deletionsEnabled`
- `"execution_mode"` if `job.ForceDryRun` (and deletions are enabled — meaning the mode forced it)
- `""` if not a dry-run

### Step 7.7: Remove `Reason` from `ApprovalService` methods

**File:** `internal/services/approval.go`

- Remove `Reason` field from `UpsertPending()` item construction
- Remove `Reason` from `CreateForceDelete()` (already being removed in Phase 2)
- Update `ExecuteApproval()` to not pass `Reason` to `DeleteJob`

### Step 7.8: Update `DeleteJob` struct — remove `Reason`

**File:** `internal/services/deletion.go`

Remove the `Reason string` field from `DeleteJob`. The score and factors are already passed separately.

### Step 7.9: Update frontend audit log display

**File:** `frontend/app/components/AuditLogPanel.vue`

- Remove any display of the `reason` field
- Display score from the `score` field
- Display factor breakdown from `scoreDetails` (already parsed as JSON)
- Display trigger as a badge: "Engine" / "User" / "Approval"
- Display dry-run reason when applicable: "Deletions disabled" / "Dry-run mode"

### Step 7.10: Update frontend approval queue display

**File:** `frontend/app/components/ApprovalQueueCard.vue`

- Remove any display of the `reason` field
- Display score from the `score` field
- Display factor breakdown from `scoreDetails`

### Step 7.11: Update `AuditLog.UpsertDryRun()` method

**File:** `internal/services/auditlog.go`

Update the upsert matching to not use `Reason` (it no longer exists). Match on `media_name`, `media_type`, and `action` (already the case).

Update the upsert update fields to include `trigger` and `dry_run_reason` instead of `reason`.

### Step 7.12: Update notification dispatch

**File:** `internal/services/notification_dispatch.go`, `internal/notifications/discord.go`, `internal/notifications/apprise.go`

Check if any notification formatting uses the `Reason` field. If so, replace with structured field rendering. (Based on earlier analysis, notifications use `CycleDigest` which doesn't include `Reason`, but verify.)

### Step 7.13: Update backup/restore

**File:** `internal/services/backup.go`

The backup service serializes and restores DB tables. Ensure the schema changes (dropped `reason`, added `trigger`/`dry_run_reason`) are reflected in backup format.

### Step 7.14: Update tests

**Files:** All test files that construct `AuditLogEntry` or `ApprovalQueueItem` with `Reason`

- Remove `Reason` from all test fixtures
- Add `Trigger` and `DryRunReason` to audit log test fixtures
- Update assertions that check `Reason` content

## API Changes

### Current API Surface (Deletion-Related)

| Method | Endpoint | Purpose | File |
|--------|----------|---------|------|
| `GET` | `/api/v1/approval-queue` | List approval queue items | `routes/approval.go` |
| `POST` | `/api/v1/approval-queue/:id/approve` | Approve item → queue for deletion | `routes/approval.go` |
| `POST` | `/api/v1/approval-queue/:id/reject` | Reject item → snooze | `routes/approval.go` |
| `POST` | `/api/v1/approval-queue/:id/unsnooze` | Clear snooze → reset to pending | `routes/approval.go` |
| `DELETE` | `/api/v1/approval-queue/:id` | Dismiss a pending/rejected item | `routes/approval.go` |
| `POST` | `/api/v1/approval-queue/clear` | Clear all pending + rejected items | `routes/approval.go` |
| `POST` | `/api/v1/force-delete` | Force-delete items (bypass threshold) | `routes/approval.go` |
| `GET` | `/api/v1/deletion-queue` | List items in deletion queue | `routes/deletion.go` |
| `DELETE` | `/api/v1/deletion-queue` | Cancel a queued deletion | `routes/deletion.go` |

### Proposed API Surface

#### Removed Endpoints

| Method | Endpoint | Reason |
|--------|----------|--------|
| `POST` | `/api/v1/force-delete` | Replaced by `POST /api/v1/delete` |

#### New Endpoints

| Method | Endpoint | Purpose | File |
|--------|----------|---------|------|
| `POST` | `/api/v1/delete` | Mode-aware delete (replaces force-delete) | `routes/deletion.go` |
| `POST` | `/api/v1/deletion-queue/snooze` | Snooze a queued item (remove from deletion queue, prevent re-queuing) | `routes/deletion.go` |
| `POST` | `/api/v1/deletion-queue/clear` | Cancel all items in the deletion queue | `routes/deletion.go` |
| `GET` | `/api/v1/deletion-queue/grace-period` | Get current grace period state | `routes/deletion.go` |

#### Modified Endpoints

| Method | Endpoint | Change |
|--------|----------|--------|
| `GET` | `/api/v1/approval-queue` | Response body drops `reason` and `forceDelete` fields, adds `trigger` and `userInitiated` fields |
| `DELETE` | `/api/v1/deletion-queue` | Resets grace period timer on cancellation |

#### Unchanged Endpoints

| Method | Endpoint |
|--------|----------|
| `POST` | `/api/v1/approval-queue/:id/approve` |
| `POST` | `/api/v1/approval-queue/:id/reject` |
| `POST` | `/api/v1/approval-queue/:id/unsnooze` |
| `DELETE` | `/api/v1/approval-queue/:id` |
| `POST` | `/api/v1/approval-queue/clear` |
| `GET` | `/api/v1/deletion-queue` |

### New Endpoint: `POST /api/v1/delete`

Replaces `POST /api/v1/force-delete`. Behavior depends on the current `executionMode`.

**Request Body:**

```json
[
  {
    "mediaName": "Serenity",
    "mediaType": "movie",
    "integrationId": 1,
    "externalId": "123",
    "sizeBytes": 4294967296,
    "scoreDetails": "[...]",
    "posterUrl": "https://..."
  }
]
```

Note: `reason` field is removed from the request body (was only used for the now-dropped `Reason` column).

**Response (all modes):**

```json
{
  "queued": 3,
  "total": 3,
  "mode": "auto"
}
```

**Behavior per mode:**

| Mode | Action | Where items appear |
|------|--------|--------------------|
| `auto` | Items queued to DeletionService immediately | Deletion queue card |
| `approval` | Items inserted as pending with `user_initiated=true` | Approval queue card |
| `dry-run` | Items queued to DeletionService with `ForceDryRun=true` | Deletion queue card |

### New Endpoint: `GET /api/v1/deletion-queue/grace-period`

Returns the current grace period state for the deletion queue.

**Response:**

```json
{
  "active": true,
  "remainingSeconds": 18,
  "queueSize": 5
}
```

When no grace period is active (queue is empty or processing has started):

```json
{
  "active": false,
  "remainingSeconds": 0,
  "queueSize": 0
}
```

### New Endpoint: `POST /api/v1/deletion-queue/snooze`

Removes an item from the deletion queue and creates a snoozed (rejected) entry in the approval queue to prevent the engine from re-queuing it until the snooze expires. This reuses the existing snooze infrastructure (`snoozed_until`, `IsSnoozed()` check in the poller).

**Request Body:**

```json
{
  "mediaName": "Serenity",
  "mediaType": "movie"
}
```

**Response:**

```json
{
  "snoozed": true,
  "snoozedUntil": "2026-03-21T20:46:00Z"
}
```

**Behavior:**
1. Cancel the item in the deletion queue via `CancelDeletion()`
2. Create a rejected/snoozed entry in the approval queue with `snoozed_until` set to `now + SnoozeDurationHours` (from preferences)
3. Reset the grace period timer (queue mutation)
4. The poller's `IsSnoozed()` check prevents re-queuing until the snooze expires

This works in all modes — even in auto/dry-run mode where items don't normally go through the approval queue. The snoozed approval queue entry acts as a "do not re-queue" marker that the poller respects.

### New Endpoint: `POST /api/v1/deletion-queue/clear`

Cancels all items in the deletion queue at once (bulk cancel). Resets the grace period timer.

**Response:**

```json
{
  "cancelled": 5
}
```

**Behavior:**
1. Mark all queued items for cancellation via `CancelDeletion()` for each item
2. Clear the queued items tracking slice
3. Reset the grace period timer
4. Publish SSE event for queue cleared

### SSE Event Changes

#### New Events

| Event Type | Payload | When |
|------------|---------|------|
| `deletion_grace_period` | `{ remainingSeconds, queueSize, active }` | Grace period starts, updates periodically, expires |

#### Modified Events

| Event Type | Change |
|------------|--------|
| `deletion_queued` | No change to payload, but now emitted for dry-run items too (previously only auto mode) |

#### Removed Events

None — all existing SSE events are preserved.

### Response Body Changes

#### `ApprovalQueueItem` (used by `GET /api/v1/approval-queue`)

| Field | Change |
|-------|--------|
| `reason` | **Removed** |
| `forceDelete` | **Removed** — replaced by `userInitiated` |
| `userInitiated` | **Added** — `boolean`, indicates user-initiated vs engine-initiated |
| `trigger` | **Added** — `string`, "engine" or "user" |

#### `AuditLogEntry` (used by `GET /api/v1/audit`)

| Field | Change |
|-------|--------|
| `reason` | **Removed** |
| `trigger` | **Added** — `string`, "engine", "user", or "approval" |
| `dryRunReason` | **Added** — `string`, "deletions_disabled", "execution_mode", or "" |
| `action` | `dry_run` value removed — only `deleted`, `dry_delete`, `cancelled` remain |

#### `DeleteJobSummary` (used by `GET /api/v1/deletion-queue`)

| Field | Change |
|-------|--------|
| `reason` | **Removed** |
| `score` | **Added** — `float64`, the numeric score |

### Route File Reorganization

The `POST /api/v1/delete` endpoint moves from `routes/approval.go` to `routes/deletion.go`, consolidating all deletion-related endpoints in one file:

**`routes/deletion.go` (after):**
- `POST /api/v1/delete` — mode-aware delete (new)
- `GET /api/v1/deletion-queue` — list queued items (existing)
- `DELETE /api/v1/deletion-queue` — cancel a single queued item (existing)
- `POST /api/v1/deletion-queue/clear` — cancel all queued items (new)
- `POST /api/v1/deletion-queue/snooze` — snooze a queued item (new)
- `GET /api/v1/deletion-queue/grace-period` — grace period state (new)

**`routes/approval.go` (after):**
- `GET /api/v1/approval-queue` — list items (existing)
- `POST /api/v1/approval-queue/:id/approve` — approve (existing)
- `POST /api/v1/approval-queue/:id/reject` — reject/snooze (existing)
- `POST /api/v1/approval-queue/:id/unsnooze` — unsnooze (existing)
- `DELETE /api/v1/approval-queue/:id` — dismiss (existing)
- `POST /api/v1/approval-queue/clear` — clear all (existing)

## Summary of DB Schema Changes

All changes are to the baseline migration (`00001_v2_baseline.sql`) since this is a breaking branch.

### `approval_queue` table

| Change | Column | Details |
|--------|--------|---------|
| **Rename** | `force_delete` → `user_initiated` | Same type (INTEGER NOT NULL DEFAULT 0) |
| **Drop** | `reason` | No longer needed — `score` and `score_details` carry the data |
| **Add** | `trigger` | `TEXT NOT NULL DEFAULT 'engine'` — "engine", "user" |

### `audit_log` table

| Change | Column | Details |
|--------|--------|---------|
| **Drop** | `reason` | No longer needed — structured fields replace it |
| **Add** | `trigger` | `TEXT NOT NULL DEFAULT 'engine'` — "engine", "user", "approval" |
| **Add** | `dry_run_reason` | `TEXT NOT NULL DEFAULT ''` — "deletions_disabled", "execution_mode", "" |
| **Update** | `action` CHECK | Remove `dry_run` from allowed values, keep `deleted`, `dry_delete`, `cancelled` |

### `preference_sets` table

| Change | Column | Details |
|--------|--------|---------|
| **Add** | `deletion_queue_delay_seconds` | `INTEGER NOT NULL DEFAULT 30` — range 10-300 |

## Summary of Removed Code

| Component | Location | Reason |
|-----------|----------|--------|
| `CreateForceDelete()` | `services/approval.go` | Replaced by `ManualDelete()` |
| `ListForceDeletes()` | `services/approval.go` | No longer needed without poller processing |
| `RemoveForceDelete()` | `services/approval.go` | No longer needed without poller processing |
| `processForceDeletes()` | `poller/evaluate.go` | Eliminated — user deletes go through unified pipeline |
| `POST /force-delete` route | `routes/approval.go` | Replaced by `POST /delete` |
| `ActionDryRun` constant | `db/models.go` | Consolidated into `ActionDryDelete` |
| Direct audit log writes in poller dry-run | `poller/evaluate.go:220-248` | Routed through DeletionService |
| `Reason` field on `AuditLogEntry` | `db/models.go` | Replaced by structured `Trigger` + `DryRunReason` |
| `Reason` field on `ApprovalQueueItem` | `db/models.go` | Redundant with `Score` + `ScoreDetails` |

## Execution Order

Phases should be executed in order (1 → 2 → 3 → 4 → 5 → 6 → 7). Each phase builds on the previous:

- Phase 1 must come first because Phase 2 depends on dry-run flowing through DeletionService
- Phase 2 must come before Phase 3 because the grace period applies to the unified pipeline
- Phase 3 includes the snooze endpoint which Phase 5 depends on (snoozed items card displays snooze data)
- Phase 4 is a frontend-only change that can happen after Phase 3
- Phase 5 (Snoozed Items Card) depends on Phase 3 (snooze endpoint) and Phase 4 (always-visible deletion queue)
- Phase 6 depends on Phase 2 (approval items flowing through DeletionService)
- Phase 7 can technically happen in parallel with Phases 3-6 but is listed last to minimize merge conflicts
