# Structured Score Transparency

> **Status:** ✅ Complete — `ScoreFactor` struct, `scoreDetails` JSON in audit logs, `ScoreBreakdown.vue` + `ScoreDetailModal.vue` all implemented.

## Problem
Score details are currently embedded as a formatted string in the `reason` field (e.g., `"Score: 0.89 (Watch:0.28, Recency:0.22, Size:0.12, Rating:0.14, Age:0.11, Status:0.08)"`). This is fragile, not parseable for arbitrary factor counts, and won't scale as custom rules, Tautulli watch data, Overseerr request status, and other integrations add more scoring dimensions.

## Solution
Replace string-based score reasons with structured JSON data throughout the stack.

### Phase 1: Backend Data Model

1. Add `ScoreFactor` struct to `engine/score.go`:
```go
type ScoreFactor struct {
    Name         string  `json:"name"`         // "Watch History", "File Size", etc.
    RawScore     float64 `json:"rawScore"`     // 0.0-1.0 before weighting
    Weight       int     `json:"weight"`       // weight applied (0-10)
    Contribution float64 `json:"contribution"` // normalized contribution to final score
    Type         string  `json:"type"`         // "weight" or "rule"
}
```

2. Add `Factors []ScoreFactor` field to `EvaluatedItem`

3. Modify `calculateScore()` to return `[]ScoreFactor` alongside the score

4. Modify `applyRules()` to return rule effects as `[]ScoreFactor` with type="rule"

5. Add `ScoreDetails` TEXT column to `AuditLog` model (JSON-encoded `[]ScoreFactor`)

6. When writing audit logs in `poller.go`, JSON-encode the factors into `ScoreDetails`

### Phase 2: API Changes

1. The `/api/v1/preview` endpoint already returns `EvaluatedItem` — the new `factors` field will be included automatically

2. The `/api/v1/audit` endpoint returns `AuditLog` — the new `scoreDetails` field will be included

### Phase 3: Frontend

1. Rewrite `ScoreBreakdown.vue` to consume `factors[]` array from structured JSON instead of parsing strings

2. Render factors dynamically — any number of factors, not hardcoded to 6

3. Distinguish "weight" factors (stacked bar) from "rule" factors (badges/pills)

4. Handle backward compatibility: if `scoreDetails` is empty, fall back to displaying `reason` as plain text
