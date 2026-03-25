# Broken Integration Scoring — Factor Exclusion

**Status:** 📋 Planned
**Branch:** `fix/broken-integration-scoring`
**Depends on:** None
**Commit prefix:** `fix(engine):`

## Problem

When an enrichment integration (Plex, Tautulli, Seerr, Jellyfin, Emby, Jellystat) has a connection error, its scoring factors still participate in the calculation with zero/default values. This biases scores unfairly:

- A broken Tautulli means `playCount = 0` and `lastPlayed = nil` for all items
- The "Play History" and "Last Played" factors treat everything as "never watched"
- Items that ARE heavily watched appear to have deletion-worthy scores
- Users may unknowingly delete popular content because enrichment data is missing

## Current Behavior (verified 2026-03-25)

### EvaluationContext

[`EvaluationContext`](../../backend/internal/engine/factors.go:57) tracks which integration *types* are **configured** via `ActiveIntegrationTypes map[IntegrationType]bool`. It has no concept of broken/erroring types. [`NewEvaluationContext`](../../backend/internal/engine/factors.go:69) accepts a flat `[]string` of type names.

### Factor exclusion via RequiresIntegration

[`isFactorApplicable`](../../backend/internal/engine/factors.go:80) checks two optional interfaces:
- [`RequiresIntegration`](../../backend/internal/engine/factors.go:40) — factor requires a *specific* integration type to be active
- [`MediaTypeScoped`](../../backend/internal/engine/factors.go:48) — factor only applies to certain media types (e.g. shows, seasons)

**Only [`RequestPopularityFactor`](../../backend/internal/engine/factors.go:312) implements `RequiresIntegration`** (requires Seerr). The two most impacted factors — [`WatchHistoryFactor`](../../backend/internal/engine/factors.go:121) and [`RecencyFactor`](../../backend/internal/engine/factors.go:148) — do **NOT** implement it, because play data can come from multiple integration types (Plex, Tautulli, Jellyfin, Jellystat, Emby).

### Factor weights API (partial error tracking exists)

[`factorweights.go`](../../backend/routes/factorweights.go:34) already queries `LastError` from enabled integrations via [`reg.Integration.ListEnabled()`](../../backend/routes/factorweights.go:43) and populates an `erroring` map. The `integrationError` flag is returned in the factor weights API response and displayed in the UI. However, this only works for factors implementing `RequiresIntegration` — currently just `RequestPopularityFactor`.

### EvaluationContext timing in poller

In [`poller.go:200-204`](../../backend/internal/poller/poller.go:200), `EvaluationContext` is built from enabled config types **before** [`fetchAllIntegrations()`](../../backend/internal/poller/poller.go:207) runs connection tests and **before** the enrichment pipeline runs at line 211. Even if `BrokenIntegrationTypes` existed, it wouldn't be populated at the right time. The `evalCtx` is first consumed at line 283 ([`evaluateAndCleanDisk`](../../backend/internal/poller/poller.go:283)) and line 338 ([`SetPreviewCache`](../../backend/internal/poller/poller.go:338)), so moving construction later is safe.

### Connection tests in fetch.go

[`fetchAllIntegrations()`](../../backend/internal/poller/fetch.go:41) iterates all connectors, calls `TestConnection()`, and updates sync status via [`IntegrationService.UpdateSyncStatus()`](../../backend/internal/services/integration.go). Failed connectors are logged and skipped but the failure information is not captured in any data structure passed back — it's only persisted to the DB via the service.

### Preview service

[`PreviewService.refreshPreviewFromScratch()`](../../backend/internal/services/preview.go:377) builds its own `EvaluationContext` from `ListEnabled()` — no connection testing, no error checking. It must be updated in parallel with the poller. For broken types, it can read `LastError` from the integration configs returned by [`s.integrations.ListEnabled()`](../../backend/internal/services/preview.go:380) (same pattern used in [`factorweights.go:42-61`](../../backend/routes/factorweights.go:42)).

### Enrichment pipeline

[`EnrichmentPipeline.Run()`](../../backend/internal/integrations/enrichment_pipeline.go:47) returns [`EnrichmentStats`](../../backend/internal/integrations/enrichment_pipeline.go:38) including `ZeroMatchers` (enricher names that ran but produced zero matches) and `EnrichersRun` count. Failed enrichers (non-nil error from `Enrich()`) log warnings but are not tracked in stats — they're silently skipped. The pipeline already counts matches per-enricher using before/after deltas for play count, requested, and watchlist state.

### ScoreFactor struct

[`ScoreFactor`](../../backend/internal/engine/score.go:15) has no `skipped` or `status` field — exclusion currently means the factor simply doesn't appear in the factors slice.

### Frontend types

[`ScoreFactor`](../../frontend/app/types/api.ts:238) in `api.ts` has no `skipped` field. [`ScoringFactorWeight`](../../frontend/app/types/api.ts:70) already has `integrationError?: boolean`.

### Test patterns (must follow)

- Test helpers construct `EvaluationContext` as struct literals ([`allActiveCtx()`](../../backend/internal/engine/score_test.go:14), direct `&engine.EvaluationContext{...}` in [`evaluate_test.go`](../../backend/internal/poller/evaluate_test.go:272), [`preview_test.go`](../../backend/internal/services/preview_test.go:147), [`analytics_test.go`](../../backend/routes/analytics_test.go:78))
- [`TestUniversalFactors_DoNotImplementOptionalInterfaces`](../../backend/internal/engine/factors_test.go:228) verifies WatchHistory and Recency do NOT implement `RequiresIntegration` — this test will need updating since they'll now implement `RequiresAnyIntegration` (but still not `RequiresIntegration`)
- Canonical media names: "Firefly" for shows, "Serenity" for movies
- Direct `calculateScore()` calls in `score_test.go` for isolated factor testing

## Proposed Solution

Two complementary layers of protection, implemented together in a single branch:

**Layer 1 — Connection-level exclusion:** Exclude factors when ALL their data sources have a connection error. Catches "integration is unreachable" — the most common failure mode.

**Layer 2 — Enrichment pipeline result tracking:** After enrichment completes, check whether enrichers actually produced results. If all play-data enrichers either failed or produced zero matches, mark play-data factors as unreliable. Catches "connected but returned nothing useful" — e.g. corrupt Tautulli database, misconfigured library mapping, or API key with wrong permissions.

### Design rationale

WatchHistory and Recency can't declare a single required integration because play data comes from multiple sources (Plex, Tautulli, Jellyfin, Jellystat, Emby). If a user has Tautulli + Plex and Tautulli is broken, Plex may still provide play data — so the factor should NOT be skipped. The factor should only be skipped when **all** configured play-data integrations are broken (Layer 1) **or** when all play-data enrichers ran but produced zero matches (Layer 2).

This is expressed by `RequiresAnyIntegration` returning `[plex, tautulli, jellyfin, jellystat, emby]`: skip only if every type in the set is either not configured or broken/failed.

### Skip reason differentiation

The UI distinguishes between the two layers with different skip reasons:
- **Layer 1:** `"integration connection error"` — all data sources for this factor have connection errors
- **Layer 2:** `"no enrichment data received"` — enrichers ran but produced no usable data

### isFactorApplicable return value change

[`isFactorApplicable`](../../backend/internal/engine/factors.go:80) currently returns `bool`. Change to `(bool, string)` where the string is the skip reason (empty if applicable). This lets [`calculateScore`](../../backend/internal/engine/score.go:95) include skipped factors with the correct reason in the output.

## Implementation Steps

### Phase 1: Backend engine — interfaces and context

1. **Add `RequiresAnyIntegration` optional interface to `factors.go`** (follows the same pattern as existing [`RequiresIntegration`](../../backend/internal/engine/factors.go:40)):
   ```go
   // RequiresAnyIntegration is optionally implemented by scoring factors that
   // depend on data from any one of several enrichment integrations. The factor
   // is excluded when ALL listed types are either absent or broken.
   type RequiresAnyIntegration interface {
       RequiredIntegrationTypes() []integrations.IntegrationType
   }
   ```

2. **Add `EnrichmentCapabilityProvider` optional interface to `enrichment_pipeline.go`** (does NOT modify the core `Enricher` interface — optional capability, same pattern as `RequiresIntegration` on `ScoringFactor`):
   ```go
   // EnrichmentCapabilityProvider is optionally implemented by enrichers to
   // declare which enrichment capability they contribute to. Used by the
   // pipeline to detect when all enrichers for a capability have failed.
   type EnrichmentCapabilityProvider interface {
       EnrichmentCapability() string
   }
   ```

3. **Define enrichment capability constants** in `enrichment_pipeline.go`:
   ```go
   const (
       EnrichCapWatchData   = "watch_data"
       EnrichCapRequestData = "request_data"
       EnrichCapWatchlist   = "watchlist_data"
   )
   ```

4. **Add `RequiresEnrichmentCapability` optional interface to `factors.go`**:
   ```go
   // RequiresEnrichmentCapability is optionally implemented by scoring factors
   // that depend on a specific enrichment capability. When the required
   // capability is in the failed set (all enrichers for that capability
   // produced zero results), the factor is excluded from scoring.
   type RequiresEnrichmentCapability interface {
       RequiredEnrichmentCapability() string
   }
   ```

5. **Add `BrokenIntegrationTypes` and `FailedEnrichmentCapabilities` to `EvaluationContext`:**
   ```go
   type EvaluationContext struct {
       ActiveIntegrationTypes       map[integrations.IntegrationType]bool
       BrokenIntegrationTypes       map[integrations.IntegrationType]bool
       FailedEnrichmentCapabilities map[string]bool
   }
   ```
   Go zero-initializes maps as `nil`, so existing struct literals in tests remain valid — `nil` maps return `false` on key lookup.

6. **Update `NewEvaluationContext` signature** to accept broken types:
   ```go
   func NewEvaluationContext(activeTypes []string, brokenTypes []string) *EvaluationContext
   ```
   `FailedEnrichmentCapabilities` is set separately after enrichment runs (different lifecycle than connection testing).

7. **Implement `RequiresAnyIntegration` and `RequiresEnrichmentCapability`** on `WatchHistoryFactor` and `RecencyFactor`:
   ```go
   func (f *WatchHistoryFactor) RequiredIntegrationTypes() []integrations.IntegrationType {
       return []integrations.IntegrationType{
           integrations.IntegrationTypePlex,
           integrations.IntegrationTypeTautulli,
           integrations.IntegrationTypeJellyfin,
           integrations.IntegrationTypeJellystat,
           integrations.IntegrationTypeEmby,
       }
   }

   func (f *WatchHistoryFactor) RequiredEnrichmentCapability() string {
       return integrations.EnrichCapWatchData
   }
   ```
   Same for `RecencyFactor`. `RequestPopularityFactor` keeps existing `RequiresIntegration` (Seerr) and adds `RequiredEnrichmentCapability() → EnrichCapRequestData`.

8. **Update `isFactorApplicable` to return `(bool, string)`** and check all interfaces:
   - `RequiresIntegration`: single type must be active AND not in broken set → skip reason `"integration connection error"`
   - `RequiresAnyIntegration`: at least one type must be active AND not broken → skip reason `"integration connection error"`
   - `RequiresEnrichmentCapability`: capability must NOT be in the failed set → skip reason `"no enrichment data received"`
   - `MediaTypeScoped`: unchanged per-item check → skip reason `""` (not shown, silently excluded)
   - Return empty string when factor is applicable

### Phase 2: Backend engine — score output

9. **Add `Skipped` and `SkipReason` fields to `ScoreFactor` in `score.go`:**
   ```go
   type ScoreFactor struct {
       Name         string  `json:"name"`
       RawScore     float64 `json:"rawScore"`
       Weight       int     `json:"weight"`
       Contribution float64 `json:"contribution"`
       Type         string  `json:"type"`
       MatchedValue string  `json:"matchedValue,omitempty"`
       RuleID       *uint   `json:"ruleId,omitempty"`
       Skipped      bool    `json:"skipped,omitempty"`
       SkipReason   string  `json:"skipReason,omitempty"`
   }
   ```

10. **Update `calculateScore` in `score.go`:** When `isFactorApplicable` returns `(false, reason)` and reason is non-empty (i.e. not `MediaTypeScoped`'s silent exclusion), include the factor in the result slice with `Skipped: true`, `SkipReason: reason`, zero scores. Skipped factors must NOT participate in weight normalization or score calculation.

### Phase 3: Enrichment pipeline failure detection

11. **Implement `EnrichmentCapabilityProvider` on all enrichers** in `enrichers.go`:
    | Enricher | Capability |
    |----------|-----------|
    | `BulkWatchEnricher` | `EnrichCapWatchData` |
    | `TautulliEnricher` | `EnrichCapWatchData` |
    | `JellystatEnricher` | `EnrichCapWatchData` |
    | `RequestEnricher` | `EnrichCapRequestData` |
    | `WatchlistEnricher` | `EnrichCapWatchlist` |

12. **Add `FailedEnrichers` and `FailedCapabilities` to `EnrichmentStats`:**
    ```go
    type EnrichmentStats struct {
        EnrichersRun       int
        ItemsProcessed     int
        TotalMatches       int
        ZeroMatchers       []string
        FailedEnrichers    []string // enricher names that returned non-nil error
        FailedCapabilities []string // capabilities where ALL enrichers failed or zero-matched
    }
    ```

13. **Update `EnrichmentPipeline.Run()`** to:
    - Track failed enrichers (error from `Enrich()`) → append to `FailedEnrichers`
    - After all enrichers run, group enrichers by capability (via `EnrichmentCapabilityProvider`)
    - For each capability: if ALL enrichers for that capability are in `FailedEnrichers` or `ZeroMatchers`, add the capability to `FailedCapabilities`
    - Only flag a capability as failed when at least one enricher was registered for it (don't flag capabilities with zero enrichers)

### Phase 4: Poller and preview integration

14. **Add `brokenTypes []string` to `fetchResult`** in `fetch.go`. During the connection test loop at [line 41](../../backend/internal/poller/fetch.go:41), when `TestConnection()` returns an error, look up the integration's type via [`IntegrationService.GetByID()`](../../backend/internal/services/integration.go) and append to `brokenTypes`.

15. **Fix timing in `poller.go`:** Move `EvaluationContext` construction from [line 204](../../backend/internal/poller/poller.go:204) to after enrichment completes (after [line 221](../../backend/internal/poller/poller.go:221)):
    ```go
    // After enrichment pipeline runs:
    evalCtx := engine.NewEvaluationContext(configTypes, fetched.brokenTypes)
    evalCtx.FailedEnrichmentCapabilities = toSet(enrichStats.FailedCapabilities)
    ```

16. **Update `PreviewService.refreshPreviewFromScratch`** in `preview.go`:
    - **Layer 1 (broken types):** After calling `s.integrations.ListEnabled()`, iterate configs and collect types where `cfg.LastError != ""` → `brokenTypes`. Pass to `NewEvaluationContext`.
    - **Layer 2 (failed capabilities):** After `pipeline.Run()` returns, set `evalCtx.FailedEnrichmentCapabilities` from `enrichStats.FailedCapabilities`.
    - Service methods used: `s.integrations.ListEnabled()` (existing), `s.integrations.BuildIntegrationRegistry()` (existing).

17. **Update `factorweights.go`:** Extend [`hasIntegrationError`](../../backend/routes/factorweights.go:77) to also check `RequiresAnyIntegration`. If ALL types in the set have errors, return `true`. Service method used: `reg.Integration.ListEnabled()` (existing).

### Phase 5: Frontend display

18. **Add `skipped` and `skipReason` to `ScoreFactor` type in `api.ts`:**
    ```typescript
    export interface ScoreFactor {
      name: string;
      rawScore: number;
      weight: number;
      contribution: number;
      type: string;
      matchedValue?: string;
      ruleId?: number;
      skipped?: boolean;
      skipReason?: string;
    }
    ```

19. **Update `ScoreBreakdown.vue`:** Filter skipped factors out of `weightFactors` (the bar segments and abbreviation labels). Add a new section below the weight factors showing skipped factors with a muted/warning style — use `text-amber-500` and a `⚠` prefix with the `skipReason` text. Use shadcn `UiTooltip` for the full reason on hover.

20. **Update `ScoreDetailModal.vue`:** In the "Weighted Score" section, show skipped factors after the contributing factors with `opacity-50` styling, strikethrough on the name, and the skip reason as a right-aligned badge. Don't include them in the base score total or normalization note.

### Phase 6: Tests

21. **`factors_test.go` — new tests:**
    - Test `RequiresAnyIntegration` interface on WatchHistory and Recency (returns correct types list)
    - Test `RequiresEnrichmentCapability` interface on WatchHistory, Recency, RequestPopularity (returns correct capability string)
    - Test `isFactorApplicable` with broken types: all watch-data types broken → `(false, "integration connection error")`, one healthy → `(true, "")`, none broken → `(true, "")`
    - Test `isFactorApplicable` with failed capabilities: `watch_data` failed → `(false, "no enrichment data received")`, not failed → `(true, "")`
    - Test that both layers compose: broken types checked first, then capabilities

22. **`factors_test.go` — update existing:**
    - **`TestUniversalFactors_DoNotImplementOptionalInterfaces`**: Remove WatchHistory and Recency from the `universalFactors` slice (they now implement `RequiresAnyIntegration`). Add a new test verifying they implement `RequiresAnyIntegration` but NOT `RequiresIntegration`.
    - **`TestNewEvaluationContext`**: Update to test 2-arg signature with `brokenTypes`.
    - **`TestIsFactorApplicable`**: Add cases for broken types and failed capabilities.

23. **`score_test.go` — new/updated tests:**
    - Test `calculateScore` includes skipped factors in output with `Skipped: true` and correct `SkipReason`
    - Test that skipped factors don't affect score normalization (total weight excludes skipped)
    - **`allActiveCtx()`**: No changes needed — nil `BrokenIntegrationTypes` and `FailedEnrichmentCapabilities` maps work correctly (nil map lookup returns zero value `false`)

24. **`enrichment_pipeline_test.go` — new tests:**
    - Test `FailedCapabilities` populated when all watch-data enrichers fail (error + zero-match)
    - Test `FailedCapabilities` empty when at least one watch-data enricher succeeds
    - Test `FailedEnrichers` tracks enrichers that return errors
    - Test capability not flagged when no enrichers registered for it

25. **`fetch_test.go` — new test:**
    - Test that `fetchResult.brokenTypes` contains the types of integrations that failed connection tests

26. **`preview_test.go` — new/updated tests:**
    - Test that broken types are derived from `LastError` in configs from `ListEnabled()`
    - Test that `FailedEnrichmentCapabilities` are threaded through from enrichment stats
    - Existing tests: struct literals `&engine.EvaluationContext{ActiveIntegrationTypes: ...}` remain valid — new fields zero-initialize to nil

27. **`factorweights_test.go` — new test:**
    - Test `integrationError` flag for multi-source factors: all watch-data types erroring → `true`, mixed → `false`

28. **Verify with `make ci`** — all lint, test, and security checks must pass before committing.

## Files to Modify

| File | Change |
|------|--------|
| `backend/internal/engine/factors.go` | Add `RequiresAnyIntegration`, `RequiresEnrichmentCapability` interfaces; add `BrokenIntegrationTypes`, `FailedEnrichmentCapabilities` to `EvaluationContext`; update `NewEvaluationContext` to 2-arg; implement interfaces on WatchHistory/Recency/RequestPopularity; update `isFactorApplicable` to return `(bool, string)` |
| `backend/internal/engine/factors_test.go` | Test new interfaces, broken-type exclusion, failed-capability exclusion; update `TestUniversalFactors` and `TestNewEvaluationContext` |
| `backend/internal/engine/score.go` | Add `Skipped`/`SkipReason` to `ScoreFactor`; update `calculateScore` to include skipped factors with reason |
| `backend/internal/engine/score_test.go` | Test skipped factors in output, normalization excludes skipped |
| `backend/internal/integrations/enrichment_pipeline.go` | Add capability constants, `EnrichmentCapabilityProvider` interface, `FailedEnrichers`/`FailedCapabilities` to `EnrichmentStats`, capability-based failure detection in `Run()` |
| `backend/internal/integrations/enrichment_pipeline_test.go` | Test `FailedCapabilities` and `FailedEnrichers` detection |
| `backend/internal/integrations/enrichers.go` | Implement `EnrichmentCapabilityProvider` on all enricher types |
| `backend/internal/poller/fetch.go` | Add `brokenTypes` to `fetchResult`, collect during connection test loop |
| `backend/internal/poller/fetch_test.go` | Test broken type collection |
| `backend/internal/poller/poller.go` | Move `EvaluationContext` construction after fetch + enrichment; pass broken types and failed capabilities |
| `backend/internal/services/preview.go` | Derive broken types from `LastError` via `ListEnabled()`; capture `FailedCapabilities` from enrichment; pass to `NewEvaluationContext` |
| `backend/internal/services/preview_test.go` | Test broken type detection, failed capability threading |
| `backend/routes/factorweights.go` | Extend `hasIntegrationError` for `RequiresAnyIntegration` |
| `backend/routes/factorweights_test.go` | Test error flag for multi-source factors |
| `frontend/app/types/api.ts` | Add `skipped`/`skipReason` to `ScoreFactor` |
| `frontend/app/components/ScoreBreakdown.vue` | Filter skipped factors from bar; show in separate muted section |
| `frontend/app/components/ScoreDetailModal.vue` | Show skipped factors with strikethrough/muted style and reason badge |

## Callers of NewEvaluationContext (must all update to 2-arg)

| Call site | Current | After |
|-----------|---------|-------|
| [`poller.go:204`](../../backend/internal/poller/poller.go:204) | `NewEvaluationContext(configTypes)` | `NewEvaluationContext(configTypes, fetched.brokenTypes)` — moved after fetch + enrichment |
| [`preview.go:384`](../../backend/internal/services/preview.go:384) | `NewEvaluationContext(configTypes)` | `NewEvaluationContext(configTypes, brokenTypes)` — brokenTypes from `LastError` |
| [`factors_test.go:294`](../../backend/internal/engine/factors_test.go:294) | `NewEvaluationContext([]string{...})` | `NewEvaluationContext([]string{...}, nil)` |

## Direct EvaluationContext struct literals (backward-compatible, no changes required)

These construct `EvaluationContext` directly and only set `ActiveIntegrationTypes`. The new fields (`BrokenIntegrationTypes`, `FailedEnrichmentCapabilities`) zero-initialize to `nil`, which is handled by nil-safe map lookups. No changes needed unless a test specifically needs to verify broken/failed behavior.

- [`score_test.go:14`](../../backend/internal/engine/score_test.go:14) (`allActiveCtx()`)
- [`factors_test.go:248, 255`](../../backend/internal/engine/factors_test.go:248)
- [`evaluate_test.go:272, 310, 335, 489, 551, 627, 697`](../../backend/internal/poller/evaluate_test.go:272)
- [`preview_test.go:147, 180, 205, 247, 311`](../../backend/internal/services/preview_test.go:147)
- [`analytics_test.go:78`](../../backend/routes/analytics_test.go:78)

## Risk Assessment

| Risk | Mitigation |
|------|------------|
| Transient connection error causes factor to be skipped for one poll cycle | Connection tests run every poll cycle. Next successful poll restores the factor. Window is one poll interval. Acceptable trade-off — brief under-scoring is vastly preferable to deleting watched content. |
| All enrichers zero-match legitimately (library has genuinely unwatched content) | Only flag capability as failed when enrichers are registered AND items exist AND ALL enrichers zero-matched. If the library is entirely unwatched, the enricher-succeeded-with-zero-matches scenario is correct behavior — the factors should still participate. Only flag when enrichers *error* or when the zero-match rate is suspicious (all enrichers for a capability, not just one). |
| Breaking change to `NewEvaluationContext` signature | Only 3 callers: 2 production + 1 test. All internal. Compile-time enforcement prevents missed updates. |
| Breaking change to `isFactorApplicable` return type | Internal function, only called from `calculateScore` and `isFactorApplicableForAPI`. Compile-time enforcement. |
| Preview and poller diverge on broken-type detection | Preview reads `LastError` from DB (written by poller's `UpdateSyncStatus()`). They converge within one poll cycle. Both run enrichment independently so Layer 2 detection is per-caller. |
| Users confused by missing factors | Skipped factors are visible in the breakdown with explanatory text, not silently removed. Factor weights page already has `integrationError` badge. Two distinct skip reasons help users diagnose the root cause. |
| New enrichers forget to implement `EnrichmentCapabilityProvider` | Document the requirement in the `Enricher` interface comment. Add a test in `enrichment_pipeline_test.go` that verifies all enrichers used in the standard pipeline implement `EnrichmentCapabilityProvider`. |
