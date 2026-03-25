# Dashboard Polish, Charts, and Bugfixes

**Created:** 2026-03-19T16:24Z
**Status:** ✅ Complete

## Summary

Five items covering sparkline/chart visual upgrades, disk space graph enhancements, engine startup behavior, deletion queue visibility in approval mode, and a score storage bug in the audit log.

---

## Phase 1: Schema — Add `score` Column to Approval Queue and Audit Log

The numeric score is currently only stored inside a formatted reason string (`"Score: 0.85 (...)"`). This phase adds a dedicated `score REAL` column to both `approval_queue` and `audit_log` tables, making the score a first-class field.

### Step 1.1: Update baseline migration SQL

**File:** `backend/internal/db/migrations/00001_v2_baseline.sql`

Add `score REAL NOT NULL DEFAULT 0` to both tables:

- `approval_queue` table (after `size_bytes` line): add `score REAL NOT NULL DEFAULT 0`
- `audit_log` table (after `size_bytes` line): add `score REAL NOT NULL DEFAULT 0`

### Step 1.2: Update GORM models

**File:** `backend/internal/db/models.go`

- Add `Score float64` field to `ApprovalQueueItem` struct (with `gorm:"not null;default:0" json:"score"`)
- Add `Score float64` field to `AuditLogEntry` struct (with `gorm:"not null;default:0" json:"score"`)

### Step 1.3: Update ApprovalService to store score

**File:** `backend/internal/services/approval.go`

- `UpsertPending()`: include `Score` in the item and in the `Updates()` map
- `ExecuteApproval()`: read `approved.Score` instead of hardcoding `0` in the `DeleteJob`

### Step 1.4: Update poller to pass score to approval and audit entries

**File:** `backend/internal/poller/evaluate.go`

- In the approval mode branch (line ~156): set `Score: ev.Score` on the `ApprovalQueueItem`
- In the dry-run mode branch (line ~185): set `Score: ev.Score` on the `AuditLogEntry`

### Step 1.5: Update DeletionService to pass score to audit entries

**File:** `backend/internal/services/deletion.go`

- In `processJob()` for actual deletions (line ~307): set `Score: job.Score` on the `AuditLogEntry`
- In `processJob()` for dry-delete (line ~254): set `Score: job.Score` on the `AuditLogEntry`
- In `processJob()` for cancelled items (line ~215): score is 0 (correct — no score for cancellations)

### Step 1.6: Update AuditLogService upsert to include score

**File:** `backend/internal/services/auditlog.go`

- `UpsertDryRun()`: add `"score"` to the `Updates()` map so re-evaluated dry-run entries get their score updated

### Step 1.7: Update frontend ScoreBreakdown to prefer score field

**File:** `frontend/app/components/ScoreBreakdown.vue`

- Update `scoreDisplay` computed to prefer the `score` field from the API response: if the parent passes a `score` prop or the data has a numeric `score` field, use it directly. Fall back to parsing `reason` string for backward compatibility.
- Add a `score` prop to the component (optional, `number | undefined`)

### Step 1.8: Update frontend AuditLogPanel to pass score

**File:** `frontend/app/components/AuditLogPanel.vue`

- Pass `score` from the API response data to `ScoreBreakdown` component

### Step 1.9: Update frontend types

**File:** `frontend/app/types/api.ts`

- Add `score: number` to `AuditLogEntry` type (or equivalent)
- Add `score: number` to `ApprovalQueueItem` type (or equivalent)

### Step 1.10: Update tests

- `backend/internal/services/approval_test.go`: verify that `Score` is stored and returned correctly through `UpsertPending()`, `Approve()`, and `ExecuteApproval()`
- `backend/internal/services/auditlog_test.go`: verify that `Score` is stored and returned correctly
- `backend/internal/services/deletion_test.go`: verify that audit log entries contain the correct `Score` from the `DeleteJob`
- `backend/internal/poller/evaluate_test.go`: verify that score is passed to approval/audit entries
- `backend/routes/audit_test.go`: verify score field appears in API responses

---

## Phase 2: Engine Run on Startup

The poller currently waits one full poll interval before the first engine run. Add an immediate run on startup.

### Step 2.1: Add immediate poll on startup

**File:** `backend/internal/poller/poller.go`

In `Start()`, add `p.safePoll()` before the timer loop:

```go
func (p *Poller) Start() {
    go func() {
        // Run immediately on startup so the dashboard has data without waiting
        // for the first poll interval to elapse.
        p.safePoll()

        timer := time.NewTimer(p.getPollInterval())
        defer timer.Stop()
        for {
            // ... unchanged
        }
    }()
}
```

### Step 2.2: Update poller tests

**File:** `backend/internal/poller/poller_test.go`

- Add or update a test that verifies the poller runs immediately on `Start()` before any timer tick
- Verify that this immediate run doesn't interfere with the RunNow channel or timer reset logic

---

## Phase 3: Deletion Queue Visibility in Approval Mode

Two sub-problems: (a) the DeletionQueueCard hides when empty in approval mode, and (b) there is no real-time signal when items enter the deletion queue after approval.

### Step 3.1: Add `DeletionQueuedEvent` SSE event type

**File:** `backend/internal/events/types.go`

Add a new event type:

```go
type DeletionQueuedEvent struct {
    MediaName     string `json:"mediaName"`
    MediaType     string `json:"mediaType"`
    SizeBytes     int64  `json:"sizeBytes"`
    IntegrationID uint   `json:"integrationId"`
}
```

Register it in the event type map with key `"deletion_queued"`.

### Step 3.2: Publish DeletionQueuedEvent from DeletionService

**File:** `backend/internal/services/deletion.go`

In `QueueDeletion()`, after successfully sending to the channel and updating `queuedItems`, publish the event:

```go
s.bus.Publish(events.DeletionQueuedEvent{
    MediaName:     job.Item.Title,
    MediaType:     string(job.Item.Type),
    SizeBytes:     job.Item.SizeBytes,
    IntegrationID: job.Item.IntegrationID,
})
```

### Step 3.3: Subscribe to DeletionQueuedEvent in useDeletionQueue composable

**File:** `frontend/app/composables/useDeletionQueue.ts`

Add an SSE handler for `deletion_queued`:

```ts
on('deletion_queued', () => {
    fetchQueue();
});
```

### Step 3.4: Always show DeletionQueueCard and ApprovalQueueCard in approval mode

**File:** `frontend/app/components/DeletionQueueCard.vue`

Change the card visibility to also show when in approval mode:

```vue
<UiCard v-if="hasContent || isApprovalMode" ...>
```

Import `useEngineControl` to get the execution mode, or accept it as a prop from the dashboard.

Add an empty state inside the card for when there are no items:

```vue
<div v-if="!hasContent" class="text-center py-6 text-muted-foreground text-sm">
    {{ t('deletion.emptyInApproval') }}
</div>
```

**File:** `frontend/app/pages/index.vue`

Currently the ApprovalQueueCard is `v-if="approvalQueueVisible"` which just checks `isApprovalMode` — this is already correct for always-visible behavior. Verify it shows an empty state when the queue is empty.

### Step 3.5: Add `deletion_queued` to activity event types

**File:** `frontend/app/pages/index.vue`

Add `'deletion_queued'` to the `activityEventTypes` array so it also appears in the recent activity feed.

### Step 3.6: Update tests

- `backend/internal/services/deletion_test.go`: verify that `QueueDeletion()` publishes `DeletionQueuedEvent`
- `frontend/app/composables/useDeletionQueue.test.ts` (if exists): verify the `deletion_queued` handler triggers `fetchQueue()`

---

## Phase 4: Sparkline and Chart Visual Upgrades

Enhance all ECharts instances with glow effects, gradient fills, tooltips, animated loading, visual maps, emphasis focus, and **proper theme-aware chart colors**.

### Color Strategy

The current charts use `primaryColor` / `destructiveColor` / `successColor` — these are **semantic** colors not designed for chart aesthetics. The CSS already defines chart-specific variables (`--color-chart-1` through `--color-chart-4`) that vary per theme but are currently unused in charts.

**New color assignments:**

| Series | Before | After |
|--------|--------|-------|
| Flagged items sparkline | `primaryColor` | `chart1Color` (theme-aware chart primary) |
| Deleted items sparkline | `destructiveColor` | `destructiveColor` (keep — red = deleted is semantic) |
| Duration sparkline | `successColor` | `chart3Color` (theme-aware chart accent) |
| Capacity Used line | `primaryColor` | `chart1Color` |
| Capacity Total line | `successColor` | `chart2Color` |
| Threshold mark line | — | `destructiveColor` (semantic: danger zone boundary) |
| Target mark line | — | `successColor` (semantic: safe zone boundary) |

This makes charts look different and polished across all themes (violet, ocean, emerald, future themes) instead of always being violet+red.

The `useThemeColors()` composable already resolves `chart1Color`–`chart4Color` from CSS variables. The composable just needs to be updated in the chart components to destructure these additional colors.

### Step 4.1: Create a shared ECharts theme/defaults composable

**File:** `frontend/app/composables/useEChartsDefaults.ts` (new file)

Extract shared ECharts config into a reusable composable. This composable uses `chart1Color`–`chart4Color` for data series, `destructiveColor`/`successColor` only for semantic marks, and includes an algorithmic palette generator for charts with unbounded categories.

```ts
export function useEChartsDefaults() {
    const { isDark } = useAppColorMode();
    const {
        chart1Color, chart2Color, chart3Color, chart4Color,
        destructiveColor, successColor,
    } = useThemeColors();

    /** Line style with glow shadow */
    function glowLineStyle(color: string, width = 2) {
        return { width, color, shadowBlur: 8, shadowColor: color + '80' };
    }

    /** 3-stop vertical gradient fill — top opaque → mid translucent → bottom transparent */
    function gradientArea(color: string) {
        return {
            color: {
                type: 'linear', x: 0, y: 0, x2: 0, y2: 1,
                colorStops: [
                    { offset: 0, color: color + '66' },    // 40% opacity
                    { offset: 0.6, color: color + '26' },  // 15% opacity
                    { offset: 1, color: color + '05' },    // ~2% opacity — near-transparent
                ],
            },
        };
    }

    /** Horizontal gradient for bar charts — left color → right transparent */
    function gradientBar(color: string) {
        return {
            type: 'linear', x: 0, y: 0, x2: 1, y2: 0,
            colorStops: [
                { offset: 0, color: color },
                { offset: 1, color: color + '99' },
            ],
        };
    }

    /** Frosted glass tooltip that adapts to light/dark */
    function tooltipConfig() {
        return {
            backgroundColor: isDark.value ? 'rgba(24,24,27,0.85)' : 'rgba(255,255,255,0.92)',
            borderColor: isDark.value ? 'rgba(63,63,70,0.6)' : 'rgba(228,228,231,0.8)',
            textStyle: {
                color: isDark.value ? '#fafafa' : '#18181b',
                fontSize: 12,
            },
            extraCssText: 'backdrop-filter: blur(8px); border-radius: 8px; box-shadow: 0 4px 12px rgba(0,0,0,0.15);',
        };
    }

    /** Emphasis focus on series hover */
    function emphasisConfig() {
        return { focus: 'series', blurScope: 'coordinateSystem' };
    }

    /**
     * Generate N harmonious colors from a base hex color.
     * Spreads hues across a ~120° analogous arc of the base hue,
     * alternating lightness for visual contrast. Perfect for pie/donut
     * charts and any chart with an unbounded number of categories.
     *
     * @param baseHex - The base color in hex (e.g. chart1Color)
     * @param count   - Number of colors needed
     * @returns Array of hex color strings
     */
    function generatePalette(baseHex: string, count: number): string[] {
        const hsl = hexToHSL(baseHex);
        const arc = Math.min(120, count * 25);
        return Array.from({ length: count }, (_, i) => {
            const hue = (hsl.h + (i * arc) / Math.max(count, 1)) % 360;
            const lightness = isDark.value
                ? 55 + (i % 3) * 8   // Dark mode: lighter base
                : 45 + (i % 3) * 8;  // Light mode: darker base
            return hslToHex(hue, hsl.s, lightness);
        });
    }

    return {
        chart1Color, chart2Color, chart3Color, chart4Color,
        destructiveColor, successColor,
        glowLineStyle, gradientArea, gradientBar,
        tooltipConfig, emphasisConfig, generatePalette,
    };
}
```

The `hexToHSL()` and `hslToHex()` utility functions will be added as private helpers within the composable file.

### Step 4.2: Upgrade main flagged/deleted sparkline

**File:** `frontend/app/pages/index.vue`

In `sparklineEChartsOption` computed (line ~1160):

- Switch flagged series from `primaryColor` to `chart1Color`
- Keep deleted series as `destructiveColor` (semantic)
- Use `glowLineStyle()` for line style with shadow glow
- Use `gradientArea()` for 3-stop gradient fills
- Add `emphasisConfig()` to each series
- Add crosshair tooltip: `axisPointer: { type: 'cross', lineStyle: { color: chart1Color, opacity: 0.3 } }`
- Use `tooltipConfig()` for frosted glass tooltip instead of hardcoded `rgba(0,0,0,0.8)`
- Add staggered animation: `animationDelay: (idx) => idx * 10`

### Step 4.3: Upgrade duration mini sparkline

**File:** `frontend/app/pages/index.vue`

In `durationSparklineEChartsOption` computed (line ~1243):

- Switch from `successColor` to `chart3Color`
- Apply `glowLineStyle()`, `gradientArea()`, `tooltipConfig()`, `emphasisConfig()` from composable
- Add visual map for duration: color shifts as duration increases:
  ```js
  visualMap: {
      show: false,
      min: 0,
      max: maxDurationMs,
      inRange: { color: [chart3Color, warningColor, destructiveColor] }
  }
  ```
  (Note: `warningColor` may need to be added to `useThemeColors()` — it already exists as `--color-warning` CSS variable. If not worth the extra plumbing, use the amber fallback `'#f59e0b'`.)

### Step 4.4: Upgrade CapacityChart

**File:** `frontend/app/components/CapacityChart.vue`

- Switch used-capacity series from `primaryColor` to `chart1Color`
- Switch total-capacity series from `successColor` to `chart2Color`
- Apply `glowLineStyle()`, `gradientArea()`, `tooltipConfig()`, `emphasisConfig()` from composable
- Update the color array to use chart colors: `[chart1Color, chart2Color]`

### Step 4.5: Add threshold and target lines to CapacityChart

**File:** `frontend/app/components/CapacityChart.vue`

Accept new props for threshold/target percentages from the parent `DiskGroupSection`:

```ts
const props = defineProps<{
    mode: string;
    diskGroupId?: number;
    since?: string;
    thresholdPct?: number;
    targetPct?: number;
}>();
```

Add `markLine` to the first series configuration in percentage mode:

```js
markLine: {
    silent: true,
    symbol: 'none',
    data: [
        {
            yAxis: props.thresholdPct,
            lineStyle: { color: destructiveColor, type: 'dashed', width: 1 },
            label: { formatter: 'Threshold {c}%', position: 'insideEndTop', fontSize: 10 }
        },
        {
            yAxis: props.targetPct,
            lineStyle: { color: successColor, type: 'dashed', width: 1 },
            label: { formatter: 'Target {c}%', position: 'insideEndTop', fontSize: 10 }
        }
    ]
}
```

**File:** `frontend/app/components/DiskGroupSection.vue`

Pass threshold/target to CapacityChart:

```vue
<CapacityChart
    :threshold-pct="group.thresholdPct"
    :target-pct="group.targetPct"
    ...
/>
```

### Step 4.6: Upgrade all Insights tab charts

**File:** `frontend/app/pages/insights.vue`

The Insights tab has 7 ECharts instances that are currently flat and unstyled. All should be upgraded to use the `useEChartsDefaults` composable for visual consistency with the dashboard charts.

**4.6a — Quality Profile Donut** (`qualityDonutOption`)

Currently has **no explicit colors** — relies on ECharts default palette which clashes with every theme.

- Use `generatePalette(chart1Color, dataLength)` for slice colors — generates N harmonious colors from the theme's chart hue
- Add inner ring shadow: `itemStyle: { shadowBlur: 6, shadowColor: 'rgba(0,0,0,0.15)' }`
- Add emphasis glow on hover: `emphasis: { itemStyle: { shadowBlur: 12, shadowColor: 'rgba(0,0,0,0.3)' } }`
- Use `tooltipConfig()` for frosted glass tooltip
- Add `animationType: 'scale'` for entry animation
- Improve label: add `rich` text styling with font weight for name and lighter for percentage

**4.6b — Genre Distribution Bar** (`genreBarOption`)

Currently uses single flat `primaryColor` bars.

- Use `gradientBar(chart1Color)` for bar fill
- Add `borderRadius: [0, 4, 4, 0]` for rounded right edges (horizontal bars)
- Add `emphasis: { itemStyle: { shadowBlur: 8 } }`
- Use `tooltipConfig()`
- Add subtle grid styling: dashed splitLines at low opacity

**4.6c — Year Distribution Area** (`yearAreaOption`)

Currently uses flat `areaStyle: { opacity: 0.3 }`.

- Use `chart1Color` instead of `primaryColor`
- Apply `glowLineStyle(chart1Color)` and `gradientArea(chart1Color)`
- Use `tooltipConfig()` with crosshair `axisPointer`
- Add `emphasis` config

**4.6d — Integration Treemap** (`integrationTreemapOption`)

Already uses `chart1Color`–`chart4Color` — minor improvements only.

- Add `itemStyle: { borderWidth: 2, borderColor: isDark ? '#18181b' : '#fafafa' }` for cell gaps
- Add label text shadow: `textShadowBlur: 2, textShadowColor: 'rgba(0,0,0,0.5)'`
- Use `tooltipConfig()`

**4.6e — Growth Over Time Line** (`growthLineOption`)

Currently uses `chart2Color` for used and `textColor` for total (nearly invisible in dark mode).

- Used line: `chart1Color` with `glowLineStyle()` and `gradientArea()`
- Total line: `chart2Color` with dashed style (keep dashed, but use visible chart color)
- Use `tooltipConfig()`
- Add `emphasis` config

**4.6f — Quality Stacked Bar** (`qualityStackedBarOption`)

Uses `chart1Color` and `chart3Color` — keep these, but add visual polish.

- Add `borderRadius: [4, 4, 0, 0]` for rounded top edges
- Add subtle gradient to bars via `gradientBar()`
- Use `tooltipConfig()` with shadow axisPointer
- Add `emphasis` config

**4.6g — Popularity Heatmap** (`popularityHeatmapOption`)

**Biggest offender** — hardcoded colors `['#1a1a2e', '#16213e', '#0f3460', '#e94560']` that don't match any theme.

- Replace `visualMap.inRange.color` with theme-derived gradient:
  ```js
  color: isDark
      ? ['transparent', chart1Color + '33', chart1Color + '99', chart1Color]
      : ['#fafafa', chart1Color + '33', chart1Color + '99', chart1Color]
  ```
- Use `tooltipConfig()`
- Add `itemStyle: { borderWidth: 1, borderColor: isDark ? '#27272a' : '#f4f4f5' }` for cell definition
- Improve label: show play count on cells large enough (`label: { show: true, formatter: fn, fontSize: 10 }`)

---

## Phase 5: Run `make ci`

### Step 5.1: Run full CI checks

Run `make ci` from the `capacitarr/` directory to verify all lint, test, and security checks pass after all changes.

### Step 5.2: Fix any issues found

Address any lint errors, test failures, or type errors discovered by the CI run.

---

## Files Modified (Summary)

### Backend
| File | Change |
|------|--------|
| `backend/internal/db/migrations/00001_v2_baseline.sql` | Add `score` column to `approval_queue` and `audit_log` |
| `backend/internal/db/models.go` | Add `Score float64` to `ApprovalQueueItem` and `AuditLogEntry` |
| `backend/internal/events/types.go` | Add `DeletionQueuedEvent` |
| `backend/internal/services/approval.go` | Store/read `Score` field, fix `ExecuteApproval` |
| `backend/internal/services/auditlog.go` | Include `score` in upsert |
| `backend/internal/services/deletion.go` | Publish `DeletionQueuedEvent`, pass `Score` to audit entries |
| `backend/internal/poller/poller.go` | Add immediate `safePoll()` on startup |
| `backend/internal/poller/evaluate.go` | Pass `Score` to approval/audit entries |
| Test files for above | Updated assertions for `Score` field and new event |

### Frontend
| File | Change |
|------|--------|
| `frontend/app/pages/index.vue` | Enhanced sparkline ECharts options, add `deletion_queued` to activity types |
| `frontend/app/pages/insights.vue` | All 7 charts upgraded with theme colors, gradients, glows, tooltips, palette gen |
| `frontend/app/components/CapacityChart.vue` | Visual upgrades, threshold/target mark lines |
| `frontend/app/components/DiskGroupSection.vue` | Pass threshold/target props to CapacityChart |
| `frontend/app/components/DeletionQueueCard.vue` | Always visible in approval mode with empty state |
| `frontend/app/components/ScoreBreakdown.vue` | Prefer `score` prop over reason string parsing |
| `frontend/app/components/AuditLogPanel.vue` | Pass `score` to ScoreBreakdown |
| `frontend/app/composables/useDeletionQueue.ts` | Subscribe to `deletion_queued` SSE event |
| `frontend/app/composables/useEChartsDefaults.ts` | New shared composable for chart styling + palette generator |
| `frontend/app/types/api.ts` | Add `score` field to relevant types |
| `frontend/app/locales/en.json` | Add `deletion.emptyInApproval` translation key |
