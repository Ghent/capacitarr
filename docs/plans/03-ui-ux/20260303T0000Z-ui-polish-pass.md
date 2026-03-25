# UI Polish Pass — 11 Issues

**Created:** 2026-03-03T00:00Z
**Status:** ✅ Complete

## Summary

Comprehensive UI/UX polish pass addressing 11 issues identified during user testing.

## Changes

### Issue 1: Advanced Tab Full Destructive Styling
- **File:** `frontend/app/pages/settings.vue`
- Made the "Advanced" tab button fully destructive-styled (red background/border tint) in both active and inactive states, instead of just red text

### Issue 2: License + CLA Text
- **File:** `frontend/app/pages/help.vue`
- Changed license display from "PolyForm Noncommercial 1.0.0" to "PolyForm Noncommercial 1.0.0 + CLA"

### Issue 3: Score Detail Modal Formula Redesign
- **File:** `frontend/app/components/ScoreDetailModal.vue`
- Complete redesign: replaced colored bar + dots + factor table with a transparent formula view
- Shows weighted score computation: `rawScore × weight = contribution` for each factor
- Shows custom rule modifiers with their multiplier values
- Shows final calculation: `baseScore × ruleModifier = finalScore`
- Special handling for protected items (always_keep rules)
- Moved media type badge to the right of the title

### Issue 4: Effect Badge Remove Background Color
- **File:** `frontend/app/pages/rules.vue`
- Changed effect badges from solid color backgrounds to transparent outline-style badges
- The emoji icons already communicate the effect level; solid backgrounds were redundant

### Issue 5: Advanced Tab Active Text White
- **File:** `frontend/app/pages/settings.vue`
- Added explicit `data-[state=active]:text-white` to override dark mode muted text
- Combined with Issue 1 fix

### Issue 6: Popup Opacity Consistency
- **Files:** `frontend/app/components/ui/popover/PopoverContent.vue`, `frontend/app/components/ui/dropdown-menu/DropdownMenuContent.vue`
- Added `bg-popover/95 backdrop-blur-sm` to both popover and dropdown menu content components for consistent translucency

### Issue 7: Theme Swatches Use Actual Colors
- **Files:** `frontend/app/composables/useTheme.ts`, `frontend/app/components/Navbar.vue`, `frontend/app/pages/settings.vue`
- Added `primaryColor` field to `ThemeMeta` with actual CSS oklch values
- Updated swatch rendering in both navbar dropdown and settings page to use actual primary color instead of computed approximation
- Fixes slate theme showing blue instead of gray

### Issue 8: Clear X Overlap Fix
- **File:** `frontend/app/components/RuleBuilder.vue`
- Added `min-w-0` to combobox container and trigger button to prevent overflow into adjacent grid column
- Added `truncate` to the display text

### Issue 9: Combobox Enter Fix
- **File:** `frontend/app/components/RuleBuilder.vue`
- Changed `@keydown.enter.prevent` to `@keydown.enter.stop.prevent` on `UiCommandInput`
- The `.stop` modifier prevents the event from propagating to the Command component's internal ListboxItem handler, which was auto-selecting matched items before the custom handler could fire

### Issue 10: Clear All Notifications
- **Frontend:** `frontend/app/components/Navbar.vue`, `frontend/app/composables/useNotifications.ts`, `frontend/app/locales/en.json`
- **Backend:** `backend/routes/notifications.go`
- Added "Clear all" button next to "Mark all read" in notification popover
- Added `clearAll()` composable function and `DELETE /api/v1/notifications` backend endpoint
- Added i18n key `nav.clearAll`

### Issue 11: Notification DB Cleanup
- **File:** `backend/internal/jobs/cron.go`
- Added daily cron job `pruneOldNotifications()` that deletes in-app notifications older than the audit log retention period
- Reuses existing `PreferenceSet.AuditLogRetentionDays` setting (default 30 days)
- If retention is 0 (forever), no cleanup is performed
