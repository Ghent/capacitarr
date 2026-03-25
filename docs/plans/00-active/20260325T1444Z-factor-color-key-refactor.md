# Factor Color Mapping — Key-Based Refactor

**Status:** 📋 Planned
**Branch:** TBD (`refactor/factor-color-keys`)
**Depends on:** None

## Problem

The score detail UI maps factor colors using the factor's **display name** (e.g., `"Play History"`, `"Show Status"`). This is brittle — if a backend developer renames a factor, the frontend color mapping silently falls back to gray (`#6b7280`). This bug was already encountered with `ScoreDetailModal` using stale names like `"Watch History"` and `"Series Status"`.

The same color map is duplicated in two components:
- `ScoreBreakdown.vue` — compact bar chart
- `ScoreDetailModal.vue` — detailed modal view

## Proposed Solution

Use the factor's stable **key** (machine-readable identifier like `"watch_history"`) instead of the display name for color mapping. The backend already has both:
- `Name()` → human-readable display name (can change)
- `Key()` → stable machine identifier (should never change)

### Steps

1. **Backend:** Include `key` in the `ScoreFactor` JSON response alongside `name`. Currently the `score.go` output only includes `name`. Add a `key` field.

2. **Frontend types:** Add `key?: string` to the `ScoreFactor` interface in `api.ts`.

3. **Color mapping:** Change both `ScoreBreakdown.vue` and `ScoreDetailModal.vue` to use `f.key` for color lookup instead of `f.name`. Fall back to `f.name` for backward compatibility with cached/legacy data.

4. **Single source of truth:** Extract the color map and abbreviation map into a shared utility file (e.g., `utils/factorColors.ts`) so both components reference the same mapping.

5. **Key-based color map:**

```typescript
// utils/factorColors.ts
export const FACTOR_COLORS: Record<string, string> = {
  watch_history: '#8b5cf6',
  last_played: '#3b82f6',
  file_size: '#f59e0b',
  rating: '#10b981',
  time_in_library: '#f97316',
  series_status: '#ec4899',
  request_popularity: '#06b6d4',
};

export function factorColor(key?: string, name?: string): string {
  if (key && FACTOR_COLORS[key]) return FACTOR_COLORS[key];
  // Fallback to name-based lookup for legacy data
  return NAME_FALLBACK_COLORS[name ?? ''] ?? '#6b7280';
}
```

## Files to Modify

| File | Change |
|------|--------|
| `backend/internal/engine/score.go` | Add `Key` field to `ScoreFactor` output |
| `frontend/app/types/api.ts` | Add `key?: string` to `ScoreFactor` |
| `frontend/app/utils/factorColors.ts` | New shared color utility |
| `frontend/app/components/ScoreBreakdown.vue` | Use shared utility |
| `frontend/app/components/ScoreDetailModal.vue` | Use shared utility |

## Benefits

- Color mappings never silently break when factor names are renamed
- Single source of truth for colors — no duplication between components
- Backward compatible with cached data that doesn't have keys yet
