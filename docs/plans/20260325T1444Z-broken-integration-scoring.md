# Broken Integration Scoring — Factor Exclusion

**Status:** 📋 Planned
**Branch:** TBD (`fix/broken-integration-scoring`)
**Depends on:** None

## Problem

When an enrichment integration (Plex, Tautulli, Seerr, Jellyfin, Emby, Jellystat) has a connection error, its scoring factors still participate in the calculation with zero/default values. This biases scores unfairly:

- A broken Tautulli means `playCount = 0` and `lastPlayed = nil` for all items
- The "Play History" and "Last Played" factors treat everything as "never watched"
- Items that ARE heavily watched appear to have deletion-worthy scores
- Users may unknowingly delete popular content because enrichment data is missing

## Current Behavior

The `EvaluationContext` tracks which integration *types* are **configured** (via `ActiveIntegrationTypes` map). Factors implementing `RequiresIntegration` are excluded when their required type is absent. However, a configured-but-broken integration is treated as "active" — its factors participate with zero data.

## Proposed Solution

### Option A: Exclude factors when their data source has a connection error

1. Extend `EvaluationContext` with a `BrokenIntegrationTypes` map
2. In the poller/preview, after connection testing, populate this map with integration types that failed
3. In `score.go`, when checking whether a factor should participate, also check if its required integration is broken
4. If broken, skip the factor entirely (as if weight = 0) and record it in the factor breakdown as "skipped — integration error"

### Option B: Dampen factor contribution when data source is broken

Instead of excluding entirely, multiply the factor's contribution by 0 (or a configurable dampening factor). This preserves the factor in the breakdown display but shows its contribution as zero with an explanation.

### Recommendation

**Option A** — complete exclusion. Simpler, clearer UX. Users see a factor listed as "skipped" rather than contributing misleadingly. The factor weight UI already shows an `integrationError` flag per factor.

## Implementation Steps

1. Add `BrokenIntegrationTypes map[IntegrationType]bool` to `EvaluationContext` in `factors.go`
2. Update `NewEvaluationContext` to accept broken types
3. In `score.go` `calculateWeightedScore`, skip factors whose required integration is in the broken set
4. In the `ScoreFactor` result, add a `skipped` or `status` field to indicate why a factor was excluded
5. In the poller (`fetch.go` and `preview.go`), collect broken integration types from connection test results and pass to `NewEvaluationContext`
6. Frontend: Display skipped factors with a muted/warning style in `ScoreDetailModal` and `ScoreBreakdown`
7. Add unit tests for the broken-integration exclusion logic

## Files to Modify

| File | Change |
|------|--------|
| `backend/internal/engine/factors.go` | Add `BrokenIntegrationTypes` to `EvaluationContext` |
| `backend/internal/engine/score.go` | Skip factors with broken required integration |
| `backend/internal/engine/score_test.go` | Test broken integration exclusion |
| `backend/internal/poller/fetch.go` | Collect broken types from connection tests |
| `backend/internal/services/preview.go` | Pass broken types to `EvaluationContext` |
| `frontend/app/components/ScoreDetailModal.vue` | Show skipped factors |
| `frontend/app/components/ScoreBreakdown.vue` | Show skipped factors |
| `frontend/app/types/api.ts` | Add `skipped` field to `ScoreFactor` |
