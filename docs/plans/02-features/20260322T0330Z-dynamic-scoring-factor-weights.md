# Dynamic Scoring Factor Weight Registry

> **Status:** ✅ Complete
> **Created:** 2026-03-22
> **Branch:** `refactor/dynamic-scoring-factor-weights` (from `feature/2.0`)

## Problem

The scoring engine has 7 (8 in schema) hardcoded weight columns on the `preference_sets` table:

```
watch_history_weight, last_watched_weight, file_size_weight,
rating_weight, time_in_library_weight, series_status_weight,
request_popularity_weight, quality_bloat_weight
```

Adding a new scoring factor currently requires:
1. A schema migration (new column on `preference_sets`)
2. A new field on `PreferenceSet` Go struct
3. A new case in `GetFactorWeight()` switch statement
4. Manual wiring in `calculateScore()` (which hardcodes each factor inline)
5. Updated weight validation in the route handler
6. Frontend changes to add a new slider

The 2.0 plan (§5 "Pluggable Scoring Factors") called for a `DefaultWeight()` method on the `ScoringFactor` interface and a dynamic scoring loop, but this was only partially implemented. The factor abstraction exists (`factors.go`) but the scoring loop and weight storage remain hardcoded.

Additionally, `RequestPopularityFactor` exists in `DefaultFactors()` but is **not used** by `calculateScore()` — it's silently ignored. And `quality_bloat_weight` has a DB column but no factor implementation.

## Solution

Replace hardcoded weight columns with a `scoring_factor_weights` table and a dynamic scoring loop. After this change, adding a new scoring factor requires **only one file** (the factor implementation with `DefaultWeight()`). The DB, API, UI, and scoring loop all adapt automatically.

## Design

### New Table: `scoring_factor_weights`

```sql
CREATE TABLE scoring_factor_weights (
    factor_key TEXT PRIMARY KEY,
    weight     INTEGER NOT NULL DEFAULT 5 CHECK(weight >= 0 AND weight <= 10),
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

Key = `ScoringFactor.Key()`. Weights seeded from `DefaultWeight()` on startup for any missing keys.

### Updated `ScoringFactor` Interface

```go
type ScoringFactor interface {
    Name() string
    Key() string
    Description() string
    DefaultWeight() int
    Calculate(item integrations.MediaItem) float64
}
```

### New API Endpoints

**GET /api/v1/scoring-factor-weights** — Returns factor metadata + current weights:
```json
[
  { "key": "watch_history", "name": "Watch History", "description": "...", "weight": 10, "defaultWeight": 10 },
  ...
]
```

**PUT /api/v1/scoring-factor-weights** — Accepts weight map:
```json
{ "watch_history": 8, "last_watched": 5, ... }
```

### Scoring Loop

`calculateScore()` changes from ~90 lines of per-factor inline code to a ~15-line loop:

```go
func calculateScore(item MediaItem, factors []ScoringFactor, weights map[string]int) (float64, string, []ScoreFactor) {
    var totalWeight float64
    for _, f := range factors {
        totalWeight += float64(weights[f.Key()])
    }
    // ... iterate, calculate, normalize
}
```

## Affected Files

### Backend — Schema & Models
- `internal/db/migrations/00001_v2_baseline.sql` — Remove 8 weight columns from `preference_sets`, add `scoring_factor_weights` table
- `internal/db/models.go` — Remove weight fields from `PreferenceSet`, remove `GetFactorWeight()`, add `ScoringFactorWeight` model
- `internal/db/db.go` — Update seed logic

### Backend — Engine
- `internal/engine/factors.go` — Add `Description()` and `DefaultWeight()` to interface + implementations
- `internal/engine/score.go` — Rewrite `calculateScore()` to dynamic loop, update `EvaluateMedia()` signature
- `internal/engine/evaluator.go` — Update `Evaluate()` to accept weight map

### Backend — Services
- `internal/services/settings.go` — Add `ListFactorWeights()`, `UpdateFactorWeights()`, `SeedFactorWeights()`, `GetWeightMap()`
- `internal/services/backup.go` — Update export/import for dynamic weights
- `internal/services/preview.go` — Pass weight map to evaluator

### Backend — Routes
- `routes/preferences.go` — Remove hardcoded weight validation
- New: `routes/factorweights.go` — GET/PUT endpoints for factor weights

### Backend — Poller
- `internal/poller/evaluate.go` — Pass weight map to evaluation

### Backend — Tests
- All test files creating `PreferenceSet` with weight fields need updating
- New tests for factor weight service methods and routes

### Frontend
- `app/types/api.ts` — Remove weight fields from `PreferenceSet`, add `ScoringFactorWeight` type
- `app/components/rules/RuleWeightEditor.vue` — Rewrite for dynamic weights from API
- `app/pages/rules.vue` — Fetch/save via new endpoints

## Dead Code Removal

After this change, the following are deleted:
- 8 `*Weight` fields on `PreferenceSet` struct
- `GetFactorWeight()` method on `PreferenceSet`
- Weight validation in `routes/preferences.go`
- Weight column processing in `db.go` seed
- Weight fields in `PreferencesExport` struct
- Weight field mapping in backup import/export
- `WeightKeys` interface in frontend

## Execution Order

1. Schema + models (baseline SQL, models.go, db.go)
2. Engine interface + factors (factors.go)
3. Engine scoring loop (score.go, evaluator.go)
4. Services (settings.go, backup.go)
5. Routes (factorweights.go, preferences.go)
6. Poller + preview updates
7. Frontend (types, component, page)
8. Tests
9. Dead code cleanup
10. `make ci` validation
