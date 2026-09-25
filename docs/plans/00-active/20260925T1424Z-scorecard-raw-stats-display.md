# Scorecard Raw Stats Display

**Status:** In progress
**Priority:** Medium (trust / score transparency)
**Origin:** Scorecard design discussion — show the stat the engine used and how it became the raw score

---

## Problem

The Score Detail modal shows `rawScore × weight = contribution` but not the library data behind `rawScore`. Users cannot tell why Play History is `0.17`, or whether Rating `0.50` is a real 5.0 vs missing data. Custom rules already expose `matchedValue`. Weight factors do not.

Approval, audit, and sunset persist `ScoreDetails` JSON, not the live `MediaItem`. Reconstructing stats in Vue from today's Plex data would lie on historical cards.

## Design

Each `ScoringFactor` emits an `inputLabel` at evaluation time. Persist it on `ScoreFactor`. The modal prints it under the factor name. No per-factor switch in the frontend.

Line shape:

- Formula: `3 plays → 0.5 ÷ 3 = 0.17`
- Lookup: `Ended → 1.00`, `Never played → 1.00`, `No rating → 0.50`
- Cap: `Jan 10, 2023 → 1354 ÷ 365 → 1.00` (`→` instead of `=` when the engine clipped)

Right column stays `raw × weight = contribution`.

## Constraints

- Do not overload `matchedValue`.
- Do not format by factor name/key in Vue.
- Skip reasons stay the only line on skipped factors.
- Old stored JSON without `inputLabel` renders no second line.
- English backend strings, same as `matchedValue`.
- Compact `ScoreBreakdown` bar is unchanged.
