# Audit Remediation Plan — Round 2

**Created:** 2026-03-07T15:35Z
**Status:** ✅ Complete
**Branch:** `refactor/audit-remediation-2`
**Scope:** Sentinel error consistency, dead code removal, test utility cleanup

---

## Context

A full codebase audit following the service layer remediation (20260307T0302Z) and
notification overhaul identified 7 findings requiring remediation. This plan
addresses all findings.

---

## Phase 1: Add Sentinel Errors to Services

### Step 1.1 — RulesService sentinel errors
**File:** `backend/internal/services/rules.go`

Added `ErrRuleNotFound` and `ErrRuleValidation` sentinel errors. Updated:
- `Create()` — validation errors now wrap `ErrRuleValidation`
- `Update()` — not-found errors now wrap `ErrRuleNotFound`
- `Delete()` — not-found errors now wrap `ErrRuleNotFound`

### Step 1.2 — AuthService sentinel errors
**File:** `backend/internal/services/auth.go`

Added `ErrWrongPassword` and `ErrUserNotFound` sentinel errors. Updated:
- `ChangePassword()` — wrong password returns `ErrWrongPassword`, user not found wraps `ErrUserNotFound`
- `ChangeUsername()` — wrong password returns `ErrWrongPassword`, user not found wraps `ErrUserNotFound`

### Step 1.3 — IntegrationService sentinel errors
**File:** `backend/internal/services/integration.go`

Added `ErrUnsupportedIntegrationType`, `ErrIntegrationNoRuleValues`, `ErrUnknownAction` sentinel errors. Updated:
- `FetchRuleValues()` — uses sentinel errors instead of `fmt.Errorf` strings
- `FetchRuleValues()` — propagates `ErrNotFound` from `GetByID()` without re-wrapping

---

## Phase 2: Update Route Handlers to Use errors.Is()

### Step 2.1 — routes/rules.go (BUG FIX)
Replaced dead `errors.Is(err, errors.New("rule not found"))` (always false) and
fragile `err.Error() == "rule not found: record not found"` string matching with
`errors.Is(err, services.ErrRuleNotFound)`.

Replaced `isValidationError()` string prefix matching function with
`errors.Is(err, services.ErrRuleValidation)`.

Removed the dead `isValidationError()` function entirely.

### Step 2.2 — routes/auth.go
Replaced `err.Error() == "current password is incorrect"` and
`err.Error() == "password is incorrect"` with
`errors.Is(err, services.ErrWrongPassword)`.

### Step 2.3 — routes/notifications.go
Replaced `err.Error() == "not found"` with
`errors.Is(err, services.ErrNotFound)`.

### Step 2.4 — routes/rulefields.go
Replaced all `errMsg ==` and `strings.HasPrefix(errMsg, ...)` string matching with
`errors.Is()` checks against `services.ErrNotFound`, `services.ErrUnsupportedIntegrationType`,
`services.ErrIntegrationNoRuleValues`, and `services.ErrUnknownAction`.

---

## Phase 3: Code Cleanup

### Step 3.1 — Remove empty notifications/types.go
**File:** `backend/internal/notifications/types.go`

Deleted — contained only `package notifications` with no declarations.

### Step 3.2 — Replace containsSubstring with strings.Contains
**File:** `backend/internal/notifications/sender_test.go`

Replaced custom `containsSubstring()` and `findSubstring()` functions with
`strings.Contains()` from the standard library.

---

---

## Round 2 Re-Audit (post-fix verification)

Comprehensive re-scan of the entire codebase confirmed:

- **Zero** remaining `.Error() ==` string-based error matching
- **Zero** remaining `HasPrefix(err...)` error string matching
- **Zero** `t.Skip()` calls in tests
- **Zero** `!important` in CSS
- **Zero** `@ts-ignore` or `eslint-disable` in frontend
- **Zero** `fmt.Println`/`log.Printf` in backend
- **All** `//nolint` directives have justification comments
- **All** route handlers delegate to services (test files use direct DB for setup/assertions, which is expected)
- Frontend `console.warn` calls are all in catch blocks for graceful degradation — acceptable browser-side pattern

### Round 2 Fixes

**Step R2.1** — Add justification to `//nolint:errcheck` in `deletion.go:160`

The `rate.Limiter.Wait(context.Background())` call was missing the required justification
comment. Added: `// Wait with background context never returns non-nil error`

**Step R2.2** — DRY up rulefields enrichment logic in `routes/rulefields.go`

Extracted `detectEnrichment()` and `appendEnrichmentFields()` helpers, reducing
~80 lines of duplicated enrichment field detection/appending to a single call site.
Net change: -22 lines (66 added, 88 removed).

---

## Verification

`make ci` passes: 0 lint issues, all Go tests pass, all 71 vitest tests pass,
no vulnerabilities.
