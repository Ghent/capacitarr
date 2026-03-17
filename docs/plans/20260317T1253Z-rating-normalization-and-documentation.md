# Rating Normalization and Documentation

**Created:** 2026-03-17T12:53Z
**Status:** ✅ Complete
**Scope:** Backend (rating normalization) + Frontend (tooltip/help documentation)
**Discovered during:** Issue #2 field mapping audit

## Background

During the comprehensive audit for issue #2, a rating inconsistency was discovered:

- **Sonarr** stores `Rating` as the raw TheTVDB aggregated value (0–10 scale)
- **Radarr** stores `Rating` as IMDB value with TMDB fallback (0–10 scale)
- **Lidarr** stores `Rating` as MusicBrainz value **divided by 10** (0–1 scale)
- **Readarr** stores `Rating` as GoodReads value (0–5 scale)
- **Plex** stores `Rating` as audienceRating with critic fallback (0–10 scale, only used if Plex is the item producer)

The scoring engine at `score.go:126-130` auto-detects scales (0–10 vs 0–100) but cannot handle the 0–1 case from Lidarr or the 0–5 case from Readarr. Custom rules like `rating < 5` will behave differently for Lidarr items (0–1 scale) vs Sonarr/Radarr items (0–10 scale).

## Goals

1. Normalize all ratings to a consistent 0–10 scale before storing on `MediaItem.Rating`
2. Add user-facing documentation explaining where ratings come from per integration
3. Add a tooltip on the Rating rule field in the rules UI

## Plan

### Step 1: Normalize Lidarr ratings to 0–10 scale

**File:** `backend/internal/integrations/lidarr.go`

Remove the `/ 10.0` normalization in the MediaItem builder. Lidarr's `ratings.value` is already 0–10:

```go
// Current (wrong):
rating := a.Ratings.Value / 10.0

// Fixed:
rating := a.Ratings.Value
```

**Why:** The scoring engine already handles 0–10 normalization at `score.go:127`. Pre-normalizing in the builder causes double-normalization.

### Step 2: Normalize Readarr rating scale

**File:** `backend/internal/integrations/readarr.go`

Readarr's `ratings.value` comes from GoodReads, which uses a 0–5 scale. Multiplied by 2 to normalize to 0–10:

```go
// Readarr ratings.value is GoodReads scale (0–5).
// Normalize to 0–10 so the scoring engine handles it consistently.
rating := b.Ratings.Value * 2.0
```

**Note:** Confirmed via Readarr source code analysis — `Ratings.Value` maps directly from GoodReads `AverageRating` (0–5 scale). Test mock data updated to use realistic 0–5 values (e.g., 4.25 → 8.5 after normalization).

### Step 3: Add rating source tooltip to rules UI

**File:** `frontend/app/components/rules/` (appropriate component)

Add an info tooltip next to the "Rating" rule field that displays:

> **Rating sources by integration:**
> - **Sonarr:** TheTVDB community rating (0–10)
> - **Radarr:** IMDb rating, with TMDb fallback (0–10)
> - **Lidarr:** MusicBrainz rating (0–10)
> - **Readarr:** GoodReads rating (0–10, normalized)
>
> All ratings are on a 0–10 scale.

### Step 4: Add rating documentation to help/about section

**File:** `frontend/app/pages/` or `frontend/app/components/` (about/help area)

Add a "Data Sources" or "How Scoring Works" section that explains:

- Where each field comes from per integration type
- Rating sources and their original scales
- How enrichment data (watch history, requested by, collections) is layered on top

### Step 5: Update tests

- Update Lidarr test assertions to expect the non-normalized rating value
- Add Readarr rating normalization test if scale conversion is needed

### Step 6: Run make ci

Verify all changes pass lint, tests, and security scans.

## Files Affected

| File | Change |
|------|--------|
| `backend/internal/integrations/lidarr.go` | Removed rating `/10.0` normalization — pass through 0–10 directly |
| `backend/internal/integrations/lidarr_test.go` | Updated expected rating from 0.92 to 9.2 |
| `backend/internal/integrations/readarr.go` | Added `* 2.0` normalization for GoodReads 0–5 → 0–10 |
| `backend/internal/integrations/readarr_test.go` | Updated mock data to realistic 0–5 values (4.25, 3.95) |
| `frontend/app/components/rules/RuleWeightEditor.vue` | Added info tooltip with rating sources on Rating slider |
| `frontend/app/pages/help.vue` | Added "Data Sources" collapsible section |
| `frontend/app/locales/en.json` | Added i18n keys for rating tooltip and data sources |

## Out of Scope

- Changing the scoring engine's rating normalization logic (it already handles 0–10 correctly)
- Adding per-source rating display in the preview/dashboard (would be a separate enhancement)
- Storing multiple ratings per item (the one-rating-per-item model is fine for scoring)
