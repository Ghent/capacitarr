# Absolute Byte Thresholds for Disk Groups

**Status:** ⏸️ Deferred to post-2.0  
**Priority:** Feature Request  
**Estimated Effort:** M (1–2 days)

## Summary

Allow disk group thresholds to be set as absolute byte values (e.g., "start cleaning at 500 GB used" / "free down to 400 GB used") instead of only percentages. This gives users precise control over capacity management, especially useful when disk sizes are very large or when percentage-based thresholds are unintuitive.

## Current Behavior

The `DiskGroup` model stores `ThresholdPct` (default 85%) and `TargetPct` (default 75%). The poller computes `currentPct` and compares against `ThresholdPct` to decide whether to evaluate media for deletion. The `targetBytesToFree` is derived from the percentage differential.

**Key files:**
- `backend/internal/db/models.go` — `DiskGroup` struct
- `backend/internal/poller/evaluate.go` — `evaluateAndCleanDisk()` (single consumption point)
- `backend/internal/services/diskgroup.go` — `UpdateThresholds()`
- `backend/routes/disk_groups.go` — API validation
- Frontend disk group settings UI

## Proposed Design

Add a `ThresholdMode` field (`"percent"` | `"absolute"`) to `DiskGroup`, plus `ThresholdBytes` and `TargetBytes` columns. Default to `"percent"` for backward compatibility. The poller branches on mode to use either percentage or byte comparison.

## Implementation Steps

1. **Database migration** — Add `threshold_mode TEXT DEFAULT 'percent'`, `threshold_bytes INTEGER DEFAULT 0`, `target_bytes INTEGER DEFAULT 0` to `disk_groups`
2. **Model update** — Add fields to `DiskGroup` struct
3. **Service update** — Extend `UpdateThresholds()` to accept mode and byte values; validate based on mode
4. **Poller update** — Branch in `evaluateAndCleanDisk()`: percent mode uses existing math, absolute mode compares `UsedBytes` against `ThresholdBytes` directly
5. **Route update** — Extend PUT `/disk-groups/:id` request struct and validation
6. **Event update** — Generalize `ThresholdBreachedEvent` and `ThresholdChangedEvent` to carry mode-appropriate values
7. **Frontend update** — Add mode toggle in disk group settings; conditionally render percent sliders or byte inputs
8. **Tests** — Add absolute-mode test cases for poller, service, and route layers

## Design Notes

- Framing as "used bytes" (not "free bytes") is consistent with the existing percent model where higher = fuller
- Validation must prevent threshold bytes > effective total and target bytes > threshold bytes
- Consider a warning if absolute thresholds become stale after disk replacement
- `LibraryHistory` charts that render threshold lines need to handle both modes
