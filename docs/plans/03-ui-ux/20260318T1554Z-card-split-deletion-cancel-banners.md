# Card Split, Deletion Queue Cancel, and Poster Banner Indicators

**Status:** ✅ Complete
**Created:** 2026-03-18T15:54Z  
**Branch:** `feature/queue-management-fixes` (continuation)  
**Depends on:** `20260318T1351Z-queue-management-fixes.md` (✅ Complete)

## Background

Follow-up improvements identified during review of the queue management fixes:

1. **Card split** — The single ApprovalQueueCard mixes user-actionable items (pending/snoozed) with system-progress items (approved/deleting). Split into two cards with distinct purposes.
2. **Deletion queue cancellation** — Items in the DeletionService in-memory channel cannot be removed. Add a skip-list cancellation mechanism.
3. **Poster banner indicators** — The queue status indicators on poster cards are too subtle (tiny corner pill). Replace with full-width banner overlays.

## Phase 1: Deletion Queue Cancellation Backend ✅

**Status:** ✅ Complete  
**Completed:** 2026-03-18T16:14Z

Add the ability to cancel queued deletions before they execute.

**Deviations from plan:**
- Added migration `00011_add_cancelled_audit_action.sql` to extend the `audit_log` table's CHECK constraint to include `'cancelled'`. The baseline migration (`00001`) had `CHECK(action IN ('deleted','dry_run','dry_delete'))` which rejected the new `ActionCancelled` value at the database level.
- The `clearCancelled()` method was made unexported (lowercase) per the task instructions, differing from the plan which showed `ClearCancelled()`.
- Routes are registered at `/api/deletion-queue` (following the project's existing `/api/` prefix pattern) rather than `/api/v1/deletion-queue`.
- Created a new `routes/deletion.go` file with `RegisterDeletionQueueRoutes()` rather than adding to `approval.go`, for cleaner separation of concerns.

### Step 1.1: Add cancellation set to DeletionService

**File:** `backend/internal/services/deletion.go`

Add a `cancelled sync.Map` field to `DeletionService`. Add methods:
- `CancelDeletion(mediaName, mediaType string) bool` — adds `mediaName:mediaType` to the cancelled set, returns true if the item was found in the queue (best-effort check)
- `IsCancelled(mediaName, mediaType string) bool` — checks the set
- `ClearCancelled()` — clears the set (called at the start of each batch via `SignalBatchSize`)

### Step 1.2: Check cancellation in processJob

**File:** `backend/internal/services/deletion.go`

In `processJob()`, after loading the item name but before the `DeletionsEnabled` check, check `IsCancelled()`. If cancelled:
- Log as "Deletion cancelled by user"
- Create audit log entry with a new `ActionCancelled` action constant
- Publish a new `DeletionCancelledEvent` SSE event
- Increment `batchProcessed` and `batchSucceeded` (it was "successfully" handled)
- Return early (do not delete)
- Remove from cancelled set after processing

### Step 1.3: Add ActionCancelled constant

**File:** `backend/internal/db/models.go`

Add `ActionCancelled = "cancelled"` to the audit log action constants.

### Step 1.4: Add DeletionCancelledEvent

**File:** `backend/internal/events/types.go`

Add:
```go
type DeletionCancelledEvent struct {
    MediaName string `json:"mediaName"`
    MediaType string `json:"mediaType"`
    SizeBytes int64  `json:"sizeBytes"`
}
```

### Step 1.5: Add ListQueuedItems method to DeletionService

**File:** `backend/internal/services/deletion.go`

Add `ListQueuedItems() []DeleteJobSummary` that returns a snapshot of items currently in the channel. Since Go channels do not support peeking, maintain a parallel `queuedItems []DeleteJobSummary` slice (protected by a mutex) that is appended to on `QueueDeletion()` and removed from on `processJob()`.

Add a `DeleteJobSummary` struct:
```go
type DeleteJobSummary struct {
    MediaName     string `json:"mediaName"`
    MediaType     string `json:"mediaType"`
    SizeBytes     int64  `json:"sizeBytes"`
    IntegrationID uint   `json:"integrationId"`
    Reason        string `json:"reason"`
}
```

### Step 1.6: Add DELETE /api/v1/deletion-queue route

**File:** `backend/routes/approval.go` (or a new `deletion.go` routes file)

Add a DELETE handler:
```
DELETE /api/v1/deletion-queue?mediaName=X&mediaType=Y
```
- Calls `reg.Deletion.CancelDeletion(mediaName, mediaType)`
- Returns 200 with `{ "cancelled": true }` or 404 if not found

### Step 1.7: Add GET /api/v1/deletion-queue route

**File:** `backend/routes/approval.go` (or new `deletion.go`)

Add a GET handler that returns `reg.Deletion.ListQueuedItems()` — the list of items currently queued for deletion. This feeds the Deletion Queue card.

### Step 1.8: Add service tests

**File:** `backend/internal/services/deletion_test.go`

Test cases:
- `CancelDeletion()` marks an item as cancelled
- `processJob()` skips cancelled items and creates audit log entry
- `ListQueuedItems()` returns current queue snapshot
- `SignalBatchSize()` clears the cancelled set

### Step 1.9: Add route tests

**File:** `backend/routes/approval_test.go` (or new file)

Test cases:
- DELETE deletion-queue returns 200 when item exists
- GET deletion-queue returns list of queued items

### Step 1.10: Run `make ci`

---

## Phase 2: Card Split — Approval Queue + Deletion Queue ✅

**Status:** ✅ Complete  
**Completed:** 2026-03-18T16:33Z

Split the single ApprovalQueueCard into two separate cards.

**Deviations from plan:**
- The `DeletionProgress` type uses `processed` (not `batchProcessed`) to match the backend `DeletionProgressEvent` struct — the `progressPercent` computed in DeletionQueueCard uses `processed` accordingly.
- The backend `DeletionFailedEvent` does not include `sizeBytes`, so the `deletion_failed` SSE handler sets `sizeBytes: 0` for failed items. The template conditionally hides the size display when `sizeBytes` is 0.
- Removed `LoaderCircleIcon` import from ApprovalQueueCard since it was only used in the removed Section 3.
- Removed `engineDeletionProgress` and `engineIsDeletionActive` from the `useEngineControl()` destructuring in `index.vue` since they were only used in the removed standalone deletion progress card. The `DeletionProgress` type import is retained for the sparkline handler.
- The API endpoint uses `/api/deletion-queue` (not `/api/v1/deletion-queue`) matching the Phase 1 route registration.
- The `currentSizeBytes` field referenced in the task spec does not exist on the `DeletionProgress` type — the "currently deleting" section shows only the item name (no size), matching the backend data.

### Step 2.1: Create DeletionQueueCard component

**File:** `frontend/app/components/DeletionQueueCard.vue`

New component that shows:
- **Header:** "Deletion Queue" title with item count badge
- **Progress bar:** Current batch progress (absorbs the standalone deletion progress card from `index.vue` lines 333-379)
- **Currently deleting:** Item name with animated spinner
- **Queued section:** List of items waiting to be deleted, each with a cancel [✕] button
- **Completed section:** Items from the current batch that have been processed (success/failure)

Data sources:
- `useEngineControl()` for `deletionProgress`, `isDeletionActive`
- New `useDeletionQueue()` composable for queued items list
- SSE events: `deletion_progress`, `deletion_success`, `deletion_failed`, `deletion_batch_complete`, `deletion_cancelled`

### Step 2.2: Create useDeletionQueue composable

**File:** `frontend/app/composables/useDeletionQueue.ts`

New composable that:
- Fetches `GET /api/v1/deletion-queue` for the list of queued items
- Provides `cancelItem(mediaName, mediaType)` that calls `DELETE /api/v1/deletion-queue`
- Tracks completed items from the current batch via SSE events (`deletion_success`, `deletion_failed`, `deletion_cancelled`)
- Clears completed items on `deletion_batch_complete`
- Auto-refreshes on `deletion_progress` events (queue shrinks as items are processed)

### Step 2.3: Remove In-Progress section from ApprovalQueueCard

**File:** `frontend/app/components/ApprovalQueueCard.vue`

Remove:
- Section 3 "In Progress (Approved/Deleting)" — lines 913-970
- The `approvedItems` display (both grid and list views)
- The `progressSectionRef` and related jump-bar logic
- The "deleting" badge count in the header

The card now only shows Pending and Snoozed sections.

### Step 2.4: Remove standalone Deletion Progress card from dashboard

**File:** `frontend/app/pages/index.vue`

Remove the standalone deletion progress card (lines 333-379) — this functionality is now inside `DeletionQueueCard`.

### Step 2.5: Add DeletionQueueCard to dashboard

**File:** `frontend/app/pages/index.vue`

Add `<DeletionQueueCard />` between the Engine Activity card and the Approval Queue card. It should be visible whenever there are items in the deletion pipeline (queued, in-progress, or recently completed in the current batch).

### Step 2.6: Update DeletionQueueCard types

**File:** `frontend/app/types/api.ts`

Add:
```typescript
export interface DeletionQueueItem {
  mediaName: string;
  mediaType: string;
  sizeBytes: number;
  integrationId: number;
  reason: string;
}

export interface DeletionCompletedItem {
  mediaName: string;
  mediaType: string;
  sizeBytes: number;
  status: 'success' | 'failed' | 'cancelled';
  timestamp: string;
}
```

### Step 2.7: Add i18n keys

**File:** `frontend/app/locales/en.json`

Add keys for the deletion queue card:
- `deletion.title`, `deletion.subtitle`
- `deletion.queued`, `deletion.completed`
- `deletion.cancelItem`, `deletion.cancelled`
- `deletion.noItems`

### Step 2.8: Run `make ci`

---

## Phase 3: Poster Banner Indicators ✅

**Status:** ✅ Complete
**Completed:** 2026-03-18T16:42Z

**Deviations from plan:**
- Used `bottom-10` (Tailwind utility, 40px) instead of the `bottom-[calc(theme(spacing.8)+theme(spacing.2))]` shown in the plan template — both resolve to 40px but `bottom-10` is simpler and more readable.
- Used ellipsis character `…` in "Deleting…" label instead of `...` — matches existing project conventions for Unicode punctuation.
- Prettier required multi-line `case`/`return` formatting in the computed; plan showed single-line style.
- Step 3.3 confirmed `LibraryTable.vue` already passes `:queue-status` — no changes needed (as anticipated by the plan).
- `security:ci` fails due to pre-existing `h3` vulnerability in `@nuxt/eslint` → `@eslint/config-inspector` transitive dependency — unrelated to Phase 3 changes. All lint, test, and Go security stages pass.

Replace the tiny corner pill queue status indicator with a full-width banner overlay.

### Step 3.1: Replace queue status pill with banner in MediaPosterCard

**File:** `frontend/app/components/MediaPosterCard.vue`

Replace the current bottom-left pill (lines 199-214) with a full-width banner:

```html
<!-- Queue status banner (above title gradient) -->
<div
  v-if="queueStatus"
  class="absolute inset-x-0 bottom-[calc(theme(spacing.8)+theme(spacing.2))] z-10 flex items-center justify-center gap-1 py-1 text-[10px] font-semibold uppercase tracking-wider backdrop-blur-sm"
  :class="{
    'bg-amber-500/70 text-white': queueStatus === 'pending',
    'bg-emerald-500/70 text-white': queueStatus === 'approved',
    'bg-red-500/70 text-white': queueStatus === 'force_delete',
    'bg-red-500/70 text-white animate-pulse': queueStatus === 'deleting',
  }"
>
  <ClockIcon v-if="queueStatus === 'pending'" class="w-3 h-3" />
  <CheckIcon v-else-if="queueStatus === 'approved'" class="w-3 h-3" />
  <ZapIcon v-else-if="queueStatus === 'force_delete'" class="w-3 h-3" />
  <LoaderCircleIcon v-else-if="queueStatus === 'deleting'" class="w-3 h-3 animate-spin" />
  <span>{{ queueStatusLabel }}</span>
</div>
```

Add a computed `queueStatusLabel`:
```typescript
const queueStatusLabel = computed(() => {
  switch (props.queueStatus) {
    case 'pending': return 'Pending';
    case 'approved': return 'Approved';
    case 'force_delete': return 'Force Delete';
    case 'deleting': return 'Deleting...';
    default: return '';
  }
});
```

### Step 3.2: Position the banner correctly

The banner should sit above the title area (which uses the bottom gradient). The title area is approximately 32px from the bottom (`p-2` = 8px padding + ~24px for title text). Position the banner at `bottom-10` (40px) so it sits just above the title gradient.

### Step 3.3: Add banner to LibraryTable grid view

**File:** `frontend/app/components/LibraryTable.vue`

Ensure the `queueStatus` prop is passed through to `MediaPosterCard` in the grid view. The list view already has inline badges — those can stay as-is since list rows have more horizontal space.

### Step 3.4: Run `make ci`

All lint (Go + ESLint + Prettier + typecheck), test (Go + Vitest), and Go security stages pass. The `security:ci` stage fails due to a pre-existing `h3` vulnerability unrelated to this change.

---

## File Change Summary

### Backend files modified:
- `backend/internal/services/deletion.go` — cancellation set, ListQueuedItems, DeleteJobSummary
- `backend/internal/services/deletion_test.go` — cancellation tests
- `backend/internal/db/models.go` — ActionCancelled constant
- `backend/internal/events/types.go` — DeletionCancelledEvent
- `backend/routes/approval.go` (or new `deletion.go`) — GET/DELETE deletion-queue routes

### Frontend files modified:
- `frontend/app/components/DeletionQueueCard.vue` — NEW component
- `frontend/app/composables/useDeletionQueue.ts` — NEW composable
- `frontend/app/components/ApprovalQueueCard.vue` — remove In-Progress section
- `frontend/app/components/MediaPosterCard.vue` — replace pill with banner
- `frontend/app/pages/index.vue` — add DeletionQueueCard, remove standalone progress card
- `frontend/app/types/api.ts` — DeletionQueueItem, DeletionCompletedItem types
- `frontend/app/locales/en.json` — deletion queue i18n keys
