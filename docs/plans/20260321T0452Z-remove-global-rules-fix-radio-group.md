# Remove Global Rules Concept & Fix Radio Group Component

**Date:** 2026-03-21T04:52Z
**Branch:** `feature/edit-custom-rules`
**Status:** ✅ Complete

## Context

Two issues were identified:

1. **Import UI radio buttons render as plain text** — `UiRadioGroup`/`UiRadioGroupItem` components used in `SettingsBackupRestore.vue` don't exist in the component library.
2. **"Global rules" concept is invalid** — Custom rules must always be associated with an integration. The codebase allows `integration_id = NULL` in 13+ locations, creating orphaned rules during import and displaying "Integration #0" in the UI.

## Steps

### Phase 1: Add Missing Radio Group Component

- [x] **Step 1.1:** Create `frontend/app/components/ui/radio-group/RadioGroup.vue` wrapping reka-ui `RadioGroupRoot`
- [x] **Step 1.2:** Create `frontend/app/components/ui/radio-group/RadioGroupItem.vue` wrapping reka-ui `RadioGroupItem` + `RadioGroupIndicator`
- [x] **Step 1.3:** Create `frontend/app/components/ui/radio-group/index.ts` barrel export

### Phase 2: Remove Global Rules from Backend

- [x] **Step 2.1:** Update `db/models.go` — change `IntegrationID` comment from "nil = global rule" to document that it's required for custom rules
- [x] **Step 2.2:** Update `db/validation.go` — add validation that `IntegrationID` must not be nil for `CustomRule`
- [x] **Step 2.3:** Update `services/rules.go` — add integration_id validation in `Create()` and `Update()`
- [x] **Step 2.4:** Update `services/rules.go` `GetRuleContext()` — remove nil IntegrationID early return
- [x] **Step 2.5:** Update `engine/rules.go` — remove the `IntegrationID != nil` guard that allows nil rules to match all items; change to skip rules where IntegrationID is nil
- [x] **Step 2.6:** Update `services/backup.go` `importRules()` — when auto-match fails, skip the rule and count as unmatched instead of creating with nil integration
- [x] **Step 2.7:** Update `services/backup.go` `importRulesWithOverrides()` — same treatment for the override path
- [x] **Step 2.8:** Update `services/backup.go` preview — don't auto-resolve null-integration rules as "matched"

### Phase 3: Remove Global Rules from Frontend

- [x] **Step 3.1:** Update `types/api.ts` — make `integrationId` required (not nullable) on `CustomRule`
- [x] **Step 3.2:** Update `RuleCustomList.vue` — remove `?? 0` fallback for null integrationId
- [x] **Step 3.3:** Update `SettingsImportResolution.vue` — ensure skipped rules don't send `integrationId: null`

### Phase 4: Verification

- [x] **Step 4.1:** Run `make ci` to verify all changes pass
  - *Note:* Fixed incomplete test `TestWatchAnalyticsService_GetStaleContentIncludesNonAbsoluteProtection` in `analytics_test.go` (missing service setup). Fixed engine tests (`rules_test.go`, `evaluator_test.go`, `score_test.go`) that lacked `IntegrationID` on rules — added `IntegrationID` to all test rules, updated two "global rule" subtests to expect the new skip behavior. Lint (0 issues) and all tests pass. `security:ci` fails on 3 pre-existing npm audit findings in transitive dev dependencies (flatted, h3) unrelated to this branch.
- [x] **Step 4.2:** Build container and test import UI with radio buttons
  - *Note:* Container builds successfully. Frontend compiles with `UiRadioGroup`/`UiRadioGroupItem` components (ESLint, Prettier, and typecheck all pass). Radio group components render in the import settings section of `SettingsBackupRestore.vue`.
- [x] **Step 4.3:** Verify rules without integration are rejected
  - *Note:* API correctly rejects rule creation with missing or null `integrationId`: returns `"integration_id is required — every rule must belong to an integration"`. Rules with a valid `integrationId` create successfully.
