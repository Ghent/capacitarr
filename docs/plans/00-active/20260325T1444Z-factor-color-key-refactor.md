# Factor Color Mapping — Key-Based Refactor

**Date:** 2026-03-25
**Status:** ✅ Complete
**Scope:** `capacitarr` (single repo)
**Branch:** `refactor/factor-color-keys`
**Depends on:** None

---

## Problem

The score detail UI maps factor colors using the factor's **display name** (e.g., `"Play History"`, `"Show Status"`). This is brittle — if a backend developer renames a factor, the frontend color mapping silently falls back to gray (`#6b7280`). This bug was already encountered with `ScoreDetailModal` using stale names like `"Watch History"` and `"Series Status"`.

The same `FACTOR_COLORS` map is duplicated in two components:
- `frontend/app/components/ScoreBreakdown.vue` — compact bar chart
- `frontend/app/components/ScoreDetailModal.vue` — detailed modal view

## Solution

Use the factor's stable **key** (machine-readable identifier like `"watch_history"`) instead of the display name for color mapping. The backend already exposes both via the `ScoringFactor` interface:
- `Name()` → human-readable display name (can change)
- `Key()` → stable machine identifier (should never change)

However, the `ScoreFactor` JSON output struct in `score.go` currently only includes `name`. This plan adds a `key` field to the JSON response and migrates the frontend color mapping to use it.

### Current Backend Keys

| Factor | `Name()` | `Key()` |
|--------|----------|---------|
| `WatchHistoryFactor` | `"Play History"` | `"watch_history"` |
| `RecencyFactor` | `"Last Played"` | `"last_watched"` |
| `FileSizeFactor` | `"File Size"` | `"file_size"` |
| `RatingFactor` | `"Rating"` | `"rating"` |
| `LibraryAgeFactor` | `"Time in Library"` | `"time_in_library"` |
| `SeriesStatusFactor` | `"Show Status"` | `"series_status"` |
| `RequestPopularityFactor` | `"Request Popularity"` | `"request_popularity"` |

### Guiding Principles

- **Single source of truth.** Color and abbreviation maps move to a shared utility. No duplication between components.
- **No backward-compatibility fallbacks.** This is just colors — if old cached data lacks a `key` field, it gets the default gray. No name-based fallback maps.
- **Conventional commits.** Each logical change is a separate commit for git-cliff.

---

## Phase 1: Backend — Add `key` to `ScoreFactor` JSON

### Step 1.1 — Add `Key` field to `ScoreFactor` struct

**File:** `backend/internal/engine/score.go`

Add a `Key` field to the `ScoreFactor` struct:

```go
type ScoreFactor struct {
	Name         string  `json:"name"`
	Key          string  `json:"key"`                    // stable machine identifier for frontend color mapping
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

### Step 1.2 — Populate `Key` in `calculateScore()`

**File:** `backend/internal/engine/score.go`

There are 3 construction sites in `calculateScore()` that build `ScoreFactor` structs. Add `Key: f.Key()` to each:

1. **Applicable factors** (~line 145): `Key: f.Key(),`
2. **Skipped factors with reason** (~line 155): `Key: f.Key(),`
3. **Zero-weight skipped factors** (~line 120): `Key: s.factor.Key(),`

### Step 1.3 — Populate `Key` in `applyRules()`

**File:** `backend/internal/engine/rules.go`

There are 6 construction sites in `applyRules()` that build `ScoreFactor` structs for rule matches. Rule-type factors don't have scoring-factor keys — set `Key` to empty string. Since the `Key` field doesn't have `omitempty`, rule factors will serialize as `"key": ""` which is fine — the frontend only uses `key` for color lookup on weight-type factors.

### Step 1.4 — Update backend tests

**File:** `backend/internal/engine/score_test.go`

Existing tests check `f.Name == "Play History"` etc. Add corresponding `f.Key` assertions alongside each name check:

```go
if f.Name == "Play History" {
    if f.Key != "watch_history" {
        t.Errorf("Expected key 'watch_history', got %q", f.Key)
    }
}
```

**File:** `backend/internal/engine/rules_test.go`

Verify existing rule-factor tests still compile and pass after the struct change.

**Commit:** `refactor(engine): add key field to ScoreFactor JSON output`

---

## Phase 2: Frontend — Shared Color Utility

### Step 2.1 — Create `factorColors.ts`

**File:** `frontend/app/utils/factorColors.ts`

Extract color map, abbreviation map, and helper functions into a single shared module keyed on machine identifiers.

```typescript
/**
 * Factor color and abbreviation mapping.
 * Single source of truth — keyed on stable machine identifiers from the backend.
 */

const FACTOR_COLORS: Record<string, string> = {
  watch_history: '#8b5cf6',
  last_watched: '#3b82f6',
  file_size: '#f59e0b',
  rating: '#10b981',
  time_in_library: '#f97316',
  series_status: '#ec4899',
  request_popularity: '#06b6d4',
};

const FACTOR_ABBRS: Record<string, string> = {
  watch_history: 'P:',
  last_watched: 'LP:',
  file_size: 'S:',
  rating: 'Rt:',
  time_in_library: 'A:',
  series_status: 'Sh:',
  request_popularity: 'Rq:',
};

const DEFAULT_COLOR = '#6b7280';

/** Resolve factor color by key. Returns default gray if key is unknown or absent. */
export function factorColor(key?: string): string {
  if (key && FACTOR_COLORS[key]) return FACTOR_COLORS[key];
  return DEFAULT_COLOR;
}

/** Resolve factor abbreviation by key. Falls back to first two chars of key. */
export function factorAbbr(key?: string): string {
  if (key && FACTOR_ABBRS[key]) return FACTOR_ABBRS[key];
  return (key ?? '').slice(0, 2) + ':';
}
```

### Step 2.2 — Create `factorColors.test.ts`

**File:** `frontend/app/utils/factorColors.test.ts`

Follow the existing vitest pattern from `format.test.ts`.

```typescript
import { describe, it, expect } from 'vitest';
import { factorColor, factorAbbr } from './factorColors';

describe('factorColor', () => {
  it('returns color for known key', () => {
    expect(factorColor('watch_history')).toBe('#8b5cf6');
  });

  it('returns default gray for unknown key', () => {
    expect(factorColor('unknown')).toBe('#6b7280');
  });

  it('returns default gray when key is undefined', () => {
    expect(factorColor(undefined)).toBe('#6b7280');
  });
});

describe('factorAbbr', () => {
  it('returns abbreviation for known key', () => {
    expect(factorAbbr('file_size')).toBe('S:');
  });

  it('generates abbreviation for unknown key', () => {
    expect(factorAbbr('custom_factor')).toBe('cu:');
  });
});
```

**Commit:** `refactor(frontend): extract factor color mapping to shared utility`

---

## Phase 3: Frontend — Migrate Components

### Step 3.1 — Update `ScoreFactor` type

**File:** `frontend/app/types/api.ts`

Add `key` to the `ScoreFactor` interface:

```typescript
export interface ScoreFactor {
  name: string;
  key?: string;  // stable machine identifier for color mapping
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

### Step 3.2 — Migrate `ScoreBreakdown.vue`

**File:** `frontend/app/components/ScoreBreakdown.vue`

1. Remove the local `FACTOR_COLORS`, `FACTOR_ABBRS` constants and `factorColor()`, `factorAbbr()` functions.
2. Import from the shared utility:
   ```typescript
   import { factorColor, factorAbbr } from '~/utils/factorColors';
   ```
3. Update all template calls: `factorColor(f.name)` → `factorColor(f.key)` and `factorAbbr(f.name)` → `factorAbbr(f.key)`.
4. Remove `LEGACY_LABELS` — it was a name-translation map for the legacy `reason` string parser. The legacy path can use the default gray; it's old data that will age out.

### Step 3.3 — Migrate `ScoreDetailModal.vue`

**File:** `frontend/app/components/ScoreDetailModal.vue`

1. Remove the local `FACTOR_COLORS` constant and `factorColor()` function.
2. Import from the shared utility:
   ```typescript
   import { factorColor } from '~/utils/factorColors';
   ```
3. Update template call: `factorColor(f.name)` → `factorColor(f.key)`.

**Commit:** `refactor(frontend): use key-based factor color mapping in score components`

---

## Verification

After implementation:

1. [ ] `make ci` passes (lint, test, security checks)
2. [ ] Start the app via `docker compose up --build`
3. [ ] Navigate to the Library page and trigger a preview scan
4. [ ] Verify score breakdown colors appear correctly in the item list
5. [ ] Click an item to open the score detail modal — verify factor colors match
6. [ ] Open browser DevTools → Network → inspect the preview API response — verify each factor has a `key` field

---

## Files Modified

| File | Change |
|------|--------|
| `backend/internal/engine/score.go` | Add `Key` field to `ScoreFactor` struct; populate at 3 construction sites |
| `backend/internal/engine/rules.go` | Set `Key` field at 6 rule-factor construction sites |
| `backend/internal/engine/score_test.go` | Add `Key` assertions for weight factors |
| `backend/internal/engine/rules_test.go` | Verify compilation after struct change |
| `frontend/app/types/api.ts` | Add `key?: string` to `ScoreFactor` interface |
| `frontend/app/utils/factorColors.ts` | **New** — shared color + abbreviation utility |
| `frontend/app/utils/factorColors.test.ts` | **New** — vitest tests for the shared utility |
| `frontend/app/components/ScoreBreakdown.vue` | Remove local maps, import shared utility, use `f.key` |
| `frontend/app/components/ScoreDetailModal.vue` | Remove local map, import shared utility, use `f.key` |

## Benefits

- Color mappings never silently break when factor names are renamed
- Single source of truth for colors and abbreviations — no duplication between components
- Simple key-only lookup — no fallback complexity
