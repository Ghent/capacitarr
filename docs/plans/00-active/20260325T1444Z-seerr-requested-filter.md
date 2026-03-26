# Seerr Requested Filter — Diagnosis & Fix

**Status:** 🔧 In Progress (Phase 1 & 2 code complete, Phase 3 blocked on valid Seerr API key)
**Branch:** `fix/seerr-requested-filter`

## Problem

The "Requested" smart filter on the Library page relies on the Seerr enrichment pipeline to populate `isRequested`, `requestedBy`, and `requestCount` on each `MediaItem`. Three issues prevent this from working correctly:

1. **Seerr connection error** — The Seerr integration currently shows a connection error, so the enricher never runs. Root cause needs diagnosis.
2. **`requestCount` hardcoded to 1** — [`enrichers.go:281`](../../../backend/internal/integrations/enrichers.go:281) sets `item.RequestCount = 1` regardless of how many users requested the same media. The `requestMap` (`map[int]MediaRequest`) overwrites duplicate TMDb IDs, so only the last request per item survives. This means `requestedBy` shows only one user and `requestCount` is always 1, even when multiple users requested the same item.
3. **No `RequestEnricher` unit tests** — [`enrichers_test.go`](../../../backend/internal/integrations/enrichers_test.go) contains tests only for `CollectionEnricher`. The `RequestEnricher` enrichment logic (TMDb ID matching, count aggregation, deduplication) is completely untested.

### Impact

- The Library page "Requested" filter badge always shows 0 (Seerr connection broken → no enrichment).
- Even when Seerr was connected, `requestCount` was always 1, making the `requestcount` rules field useless for distinguishing popularity.
- The `RequestPopularityFactor` scoring factor ([`factors.go:467`](../../../backend/internal/engine/factors.go:467)) only uses `IsRequested` and `WatchedByRequestor` (not `RequestCount`), so scoring itself is not broken — but the rules engine ([`rules.go:259`](../../../backend/internal/engine/rules.go:259)) compares `RequestCount` against user-defined thresholds, which will always see 1.

### Scope

This issue is **specific to the Seerr `RequestEnricher`**. Other enrichers handle duplicates correctly:
- `BulkWatchEnricher` (Plex, Jellyfin, Emby) — upstream `GetBulkWatchData()` methods aggregate duplicates internally (tested: `TestPlexClient_GetBulkWatchData_DuplicateTMDbID`, `TestJellyfinClient_GetBulkWatchDataForUser_DuplicateTMDbKeepsHigherPlayCount`)
- `WatchlistEnricher` — uses `map[int]bool` (boolean membership, duplicates are harmless)
- `CollectionEnricher` — explicitly deduplicates collection names via an `existing` set (tested: `TestCollectionEnricher_DeduplicatesCollectionNames`)

---

## Phase 1: Diagnose Seerr Connection ⚠️

1. [x] Start the application via `docker compose up --build`
2. [x] Navigate to Settings → Integrations and check the Seerr integration status — **"my-seer" (SEERR) is enabled, URL `https://requests.starshadow.com/`, synced successfully**
3. [x] Check container logs for Seerr-related errors — **Found: `"Enrichment failed","enricher":"Seerr Request Data","error":"failed to fetch requests: unexpected status: 403"`**
4. [x] Connection test (`/api/v1/status`) passes — "Synced just now" after clicking Test
5. [ ] **Root cause: HTTP 403 on `/api/v1/request?filter=all`** — The Seerr API key has permission to call `/api/v1/status` (connection test) but returns 403 Forbidden when fetching requests with `filter=all`. In Overseerr/Jellyseerr, the `filter=all` parameter requires an **admin-level API key**. The configured API key likely belongs to a non-admin user. **Fix: regenerate the API key from a Seerr admin account and update the integration in Capacitarr.**
6. [ ] Verify the connection test passes AND the enricher runs without errors after updating the API key

---

## Phase 2: Fix `RequestEnricher` — Aggregate Request Counts ✅

### 2.1 Add unexported aggregation type ✅

**File:** [`enrichers.go`](../../../backend/internal/integrations/enrichers.go)

Add a file-level unexported type above the `Enrich` method, following the codebase convention of no function-local type definitions:

```go
// requestAgg accumulates request data for a single TMDb ID across
// multiple Seerr requests (e.g. when several users request the same item).
type requestAgg struct {
	requestedBy string
	count       int
}
```

### 2.2 Change `requestMap` to aggregate multiple requests per TMDb ID ✅

Replace the current map construction ([`enrichers.go:270-273`](../../../backend/internal/integrations/enrichers.go:270)):

```go
// Before:
requestMap := make(map[int]MediaRequest)
for _, req := range requests {
    requestMap[req.TMDbID] = req
}

// After:
requestMap := make(map[int]requestAgg)
for _, req := range requests {
    agg := requestMap[req.TMDbID]
    agg.count++
    if agg.requestedBy == "" {
        agg.requestedBy = req.RequestedBy
    }
    requestMap[req.TMDbID] = agg
}
```

Key behavior:
- `count` increments for every request — reflects the true number of Seerr requests for this item
- `requestedBy` stores the **first** requestor encountered (deterministic: Seerr returns requests in creation order)

### 2.3 Update the enrichment loop to use aggregated data ✅

Replace the current enrichment assignment ([`enrichers.go:278-281`](../../../backend/internal/integrations/enrichers.go:278)):

```go
// Before:
if req, ok := requestMap[item.TMDbID]; ok {
    item.IsRequested = true
    item.RequestedBy = req.RequestedBy
    item.RequestCount = 1
}

// After:
if agg, ok := requestMap[item.TMDbID]; ok {
    item.IsRequested = true
    item.RequestedBy = agg.requestedBy
    item.RequestCount = agg.count
}
```

### 2.4 Add `RequestEnricher` unit tests ✅

**File:** [`enrichers_test.go`](../../../backend/internal/integrations/enrichers_test.go)

Added `mockRequestProvider` (following the `mockCollectionDataProvider` pattern) and 6 test functions:

```go
// ─── Mock RequestProvider ───────────────────────────────────────────────────

type mockRequestProvider struct {
    requests []MediaRequest
    err      error
}

func (m *mockRequestProvider) GetRequestedMedia() ([]MediaRequest, error) {
    return m.requests, m.err
}
```

Test cases (using canonical media names "Serenity" / "Firefly"):

1. [x] `TestRequestEnricher_BasicMatch` — single request for "Serenity" (TMDb 16320) maps correctly; `IsRequested=true`, `RequestedBy` populated, `RequestCount=1`
2. [x] `TestRequestEnricher_AggregatesMultipleRequests` — three requests for "Serenity" (TMDb 16320) by different users; `RequestCount=3`, `RequestedBy` is first requestor
3. [x] `TestRequestEnricher_SkipsItemsWithoutTMDbID` — item with `TMDbID=0` stays `IsRequested=false`
4. [x] `TestRequestEnricher_NoMatchingRequests` — item with TMDb ID not in Seerr results stays `IsRequested=false`, `RequestCount=0`
5. [x] `TestRequestEnricher_PropagatesProviderError` — `GetRequestedMedia()` returns error, `Enrich()` returns same error
6. [x] `TestRequestEnricher_EmptyRequestList` — zero requests from Seerr, no items enriched, no panic

### 2.5 Run `make ci` ✅

7. [x] Run `make ci` from the `capacitarr/` directory — lint ✅, Go tests ✅ (all pass), frontend tests ✅ (111 pass), govulncheck ✅. Note: `pnpm audit` reports 5 pre-existing vulnerabilities in transitive devDependencies (`picomatch`, `yaml`) — identical on `main`, not introduced by this change.

---

## Phase 3: Verify End-to-End

Once the Seerr connection is healthy and code changes are deployed via `docker compose up --build`:

1. [ ] Navigate to the Library page
2. [ ] Verify the "Requested" filter badge shows a non-zero count
3. [ ] Click "Requested" — verify only items with `isRequested=true` appear
4. [ ] Open a requested item's detail — verify `requestedBy` field is populated
5. [ ] Verify `requestCount` field shows the correct count (cross-reference with Seerr's UI)
6. [ ] Open the score detail modal — verify the "Request Popularity" factor shows a non-zero contribution (0.1 or 0.3)
7. [ ] If the item has watch history from the requestor, verify `watchedByRequestor` is `true` and the factor scores 0.3 instead of 0.1
8. [ ] Create a custom rule using `requestcount` field — verify it evaluates correctly against the aggregated count

---

## Files Changed

| File | Change |
|------|--------|
| [`seerr.go`](../../../backend/internal/integrations/seerr.go) | Change `TestConnection()` from `/api/v1/status` (public, no auth) to `/api/v1/auth/me` (requires valid API key). Replace `seerrStatusResponse` with `seerrAuthMeResponse`. |
| [`seerr_test.go`](../../../backend/internal/integrations/seerr_test.go) | Update `TestConnection` tests to use `/api/v1/auth/me` endpoint and response format. Rename `TestConnection_EmptyVersion` → `TestConnection_InvalidUser`. |
| [`enrichers.go`](../../../backend/internal/integrations/enrichers.go) | Add `requestAgg` type, aggregate `requestMap` by TMDb ID, set `RequestCount` to actual count |
| [`enrichers_test.go`](../../../backend/internal/integrations/enrichers_test.go) | Add `mockRequestProvider` + 6 `RequestEnricher` tests |

## Files Unchanged (Reference Only)

| File | Reason |
|------|--------|
| [`types.go`](../../../backend/internal/integrations/types.go) | `MediaItem.RequestCount` field already exists as `int` — no change needed |
| [`factors.go`](../../../backend/internal/engine/factors.go) | `RequestPopularityFactor.Calculate()` uses `IsRequested`/`WatchedByRequestor` only — scoring is correct as-is |
| [`rules.go`](../../../backend/internal/engine/rules.go) | Already reads `item.RequestCount` at line 259 — will naturally benefit from correct counts |
| [`library.vue`](../../../frontend/app/pages/library.vue) | Filter logic `e.item.isRequested` is correct — no change needed |
| [`api.ts`](../../../frontend/app/types/api.ts) | `requestCount?: number` type already exists — no change needed |

## User Action Required

The Seerr integration "my-seer" has API key `test` (a dummy value) stored in the database. After deploying this fix, the connection test will correctly fail, alerting the user to configure a valid API key from Seerr Settings → General.
