# Batch Database Queries During Poll Cycle

**Created:** 2026-03-22T04:27Z  
**Status:** ✅ Complete  
**Branch:** `refactor/batch-db-queries`

## Problem

During each poll cycle, `evaluateAndCleanDisk()` in `poller/evaluate.go` executes
**one or more database queries per candidate item** in a tight loop. For a library
with N candidates, this produces up to **3N individual queries** per evaluation
(1 snooze check + 2 for upsert). With 500 candidates, that's ~1500 DB round-trips
per cycle — unnecessarily slow given the data can be batched.

### Current Per-Candidate Queries

| Call | Mode | Queries per Item | Location |
|------|------|-----------------|----------|
| `ApprovalService.IsSnoozed()` | All modes | 1 SELECT COUNT | `evaluate.go:149` |
| `ApprovalService.UpsertPending()` | Approval | 1 SELECT + 1 INSERT/UPDATE | `evaluate.go:192` |
| `AuditLogService.UpsertDryRun()` | Dry-run (via worker) | 1 SELECT + 1 INSERT/UPDATE | `deletion.go:446` |
| `AuditLogService.Create()` | Auto (via worker) | 1 INSERT | `deletion.go:449` |

### What Doesn't Need Batching

- **Per-disk-group queries** (outer loop): Only 1–3 iterations per cycle.
- **Deletion worker** `processJob()`: Rate-limited by external API calls, not DB.
- **`ReconcileQueue()`**: Already a single bulk UPDATE.

## Solution

Three incremental changes, each independently shippable.

---

## Phase 1 — Pre-fetch Snoozed Set (P0)

**Goal:** Replace N per-item `IsSnoozed()` calls with a single bulk query + in-memory lookup.

### Step 1.1: Add `ListSnoozedKeys()` to `ApprovalService`

File: `internal/services/approval.go`

Add a new method that returns all currently-snoozed (media_name, media_type) pairs
for a given disk group in one query:

```go
// ListSnoozedKeys returns the set of "mediaName|mediaType" keys that are
// currently snoozed for the given disk group. The caller can do O(1) map
// lookups instead of per-item DB queries.
func (s *ApprovalService) ListSnoozedKeys(diskGroupID uint) (map[string]bool, error) {
    type row struct {
        MediaName string
        MediaType string
    }
    var rows []row
    err := s.db.Model(&db.ApprovalQueueItem{}).
        Select("media_name, media_type").
        Where("status = ? AND snoozed_until IS NOT NULL AND snoozed_until > ? AND disk_group_id = ?",
            db.StatusRejected, time.Now().UTC(), diskGroupID).
        Find(&rows).Error
    if err != nil {
        return nil, fmt.Errorf("list snoozed keys: %w", err)
    }
    keys := make(map[string]bool, len(rows))
    for _, r := range rows {
        keys[r.MediaName+"|"+r.MediaType] = true
    }
    return keys, nil
}
```

### Step 1.2: Update `evaluateAndCleanDisk()` to use pre-fetched set

File: `internal/poller/evaluate.go`

Before the candidate loop, call `ListSnoozedKeys()` once. Replace the per-item
`IsSnoozed()` call with an in-memory map lookup:

```go
// Before the loop:
snoozedKeys, err := p.reg.Approval.ListSnoozedKeys(group.ID)
if err != nil {
    slog.Error("Failed to pre-fetch snoozed keys", "component", "poller", "error", err)
    snoozedKeys = make(map[string]bool) // degrade gracefully
}

// In the loop (replaces p.reg.Approval.IsSnoozed()):
key := ev.Item.Title + "|" + string(ev.Item.Type)
if snoozedKeys[key] {
    skippedSnoozed++
    continue
}
```

### Step 1.3: Add unit test for `ListSnoozedKeys()`

File: `internal/services/approval_test.go`

Test cases:
- Returns empty map when no snoozed items exist.
- Returns correct keys for snoozed items in the target disk group.
- Does not return items from other disk groups.
- Does not return items whose snooze has expired.
- Does not return pending or approved items.

### Step 1.4: Update `evaluateAndCleanDisk` test for snooze pre-fetch

File: `internal/poller/evaluate_test.go`

Update existing snooze-related tests to verify the new flow works end-to-end.

### Step 1.5: Run `make ci`

Verify all lints and tests pass.

**Impact:** Eliminates N queries → 1 query. Largest single improvement.

---

## Phase 2 — Batch Approval Queue Upserts (P1)

**Goal:** Replace per-item `UpsertPending()` calls with a single batch operation.

### Step 2.1: Add `BulkUpsertPending()` to `ApprovalService`

File: `internal/services/approval.go`

Add a method that accepts a slice of `ApprovalQueueItem` and performs the upsert
in a single transaction using GORM's `Clauses(clause.OnConflict{...})`:

```go
// BulkUpsertPending creates or updates multiple pending approval queue items
// in a single transaction. Items are matched on (media_name, media_type, status='pending').
func (s *ApprovalService) BulkUpsertPending(items []db.ApprovalQueueItem) (created int, updated int, err error)
```

Implementation notes:
- SQLite supports `INSERT ... ON CONFLICT` (upsert) syntax.
- GORM's `Clauses(clause.OnConflict{Columns: ..., DoUpdates: ...})` with
  `CreateInBatches()` handles this cleanly.
- A unique index on `(media_name, media_type)` WHERE `status = 'pending'`
  is needed. Check if this already exists; if not, add it via migration.
- Return counts of created vs. updated for logging.

### Step 2.2: Refactor `evaluateAndCleanDisk()` approval mode

File: `internal/poller/evaluate.go`

Collect all approval-mode candidates into a slice during the loop, then call
`BulkUpsertPending()` once after the loop completes:

```go
var pendingItems []db.ApprovalQueueItem
for _, ev := range candidates {
    // ... existing skip logic (snooze, dedup, zero-score) ...
    pendingItems = append(pendingItems, db.ApprovalQueueItem{...})
    neededKeys[ev.Item.Title+"|"+string(ev.Item.Type)] = true
    bytesFreed += ev.Item.SizeBytes
}

if len(pendingItems) > 0 {
    created, updated, err := p.reg.Approval.BulkUpsertPending(pendingItems)
    // ... logging, error handling, update atomic counters ...
}
```

### Step 2.3: Add unit tests for `BulkUpsertPending()`

File: `internal/services/approval_test.go`

Test cases:
- Empty slice is a no-op.
- Creates new items when none exist.
- Updates existing pending items (score, size, poster change).
- Does not clobber rejected/approved items.
- Handles mixed creates and updates in one call.

### Step 2.4: Check/add partial unique index on approval queue

File: `internal/db/migrations/` (new migration if needed)

If no unique index on `(media_name, media_type)` WHERE `status = 'pending'` exists,
add one. SQLite supports partial indexes via `CREATE UNIQUE INDEX ... WHERE ...`.

### Step 2.5: Run `make ci`

Verify all lints and tests pass.

**Impact:** Replaces 2N queries with 1 batched transaction.

---

## Phase 3 — Batch Dry-Run Audit Upserts (P2)

**Goal:** Replace per-item `UpsertDryRun()` calls in the deletion worker with a
batch flush.

### Step 3.1: Add `BulkUpsertDryRun()` to `AuditLogService`

File: `internal/services/auditlog.go`

```go
// BulkUpsertDryRun creates or updates multiple dry-run audit log entries
// in a single transaction. Matched on (media_name, media_type, action='dry_delete').
func (s *AuditLogService) BulkUpsertDryRun(entries []db.AuditLogEntry) error
```

### Step 3.2: Collect dry-run audit entries in `DeletionService` and flush

File: `internal/services/deletion.go`

Instead of writing one audit entry per `processJob()` call, collect dry-run entries
in a slice during `drainAll()` and flush them in one batch after the drain loop:

```go
func (s *DeletionService) drainAll() {
    var dryRunEntries []db.AuditLogEntry
    for {
        job, ok := s.dequeueJob()
        if !ok { break }
        // ... existing processJob logic, but collect entries instead of writing ...
    }
    if len(dryRunEntries) > 0 {
        s.auditLog.BulkUpsertDryRun(dryRunEntries)
    }
}
```

### Step 3.3: Add unit tests for `BulkUpsertDryRun()`

File: `internal/services/auditlog_test.go`

### Step 3.4: Run `make ci`

Verify all lints and tests pass.

**Impact:** Replaces 2N queries with 1 batched transaction for dry-run mode.

---

## Expected Results

| Metric | Before | After (All Phases) |
|--------|--------|-------------------|
| Queries per 500 candidates (approval mode) | ~1500 | ~5 |
| Queries per 500 candidates (dry-run mode) | ~1500 | ~5 |
| Queries per 500 candidates (auto mode) | ~500 (async) | ~1 + async worker writes |

## Risks & Mitigations

- **SQLite write lock contention:** Large batch INSERTs hold the write lock longer.
  Mitigate by using `CreateInBatches(items, 100)` to chunk into 100-item batches.
- **Partial failure semantics:** If a batch upsert fails mid-way, some items may
  be written while others are not. Wrap in a transaction so it's all-or-nothing.
- **`IsSnoozed()` still used elsewhere:** The per-item method should remain for
  non-bulk callers (e.g., single-item checks in route handlers). Don't remove it.
