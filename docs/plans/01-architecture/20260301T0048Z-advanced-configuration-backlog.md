# Advanced Configuration Backlog

**Date:** 2026-03-01
**Status:** ❌ Closed — Will not implement
**Type:** Ideas that were evaluated and rejected

These items were considered during development but have been decided against. The simple interval dropdown covers all realistic polling use cases, and the hardcoded 3-second deletion rate is a sensible default that doesn't warrant user configuration.

---

## 1. Manual Deletion Rate Limiting Configuration

**Current state:** The deletion worker in `poller.go` uses a hardcoded `rate.Every(3*time.Second)` with burst of 1. This is arbitrary — it doesn't adapt to disk I/O load, queue depth, or file size.

**Proposed:** Add a `DeletionRateSeconds` field to `PreferenceSet` (default: 3, range: 1-30). Expose in Settings > General as an advanced option (collapsed by default).

**Considerations:**
- Too fast (1s) could overwhelm disk I/O on spinning drives
- Too slow (30s) means 50 queued items take 25 minutes
- Adaptive rate based on queue depth is more complex but smarter
- Size-aware pacing (longer wait after large file deletion) is another option

---

## 2. Manual Cron Entry Configuration

**Current state:** The poll interval will be configurable via simple presets (30s, 1m, 5m, 15m, 30m, 1h) in Phase 4 of the production readiness plan.

**Proposed:** For power users, allow entering a raw cron expression (e.g., `*/5 * * * *`) instead of using presets. This enables schedules like "every weekday at 2am" or "every 6 hours".

**Considerations:**
- Requires cron expression validation in the backend
- UI needs a text input with syntax help/examples
- The `robfig/cron` library already supports standard cron expressions
- Risk of user misconfiguration (e.g., `* * * * *` = every minute)
- Should show "next run" preview when editing the expression
