# API Contract Generation

**Status:** Implemented (PR)
**Priority:** High (drift between three hand-synced contracts)
**Origin:** Principal-engineer review item #4

---

## Summary

Three API contracts are edited by hand and drift:

1. Go structs (routes, `internal/db/models.go`, service types)
2. `frontend/app/types/api.ts`
3. `docs/reference/api/openapi.yaml` (OpenAPI 3.1, `info.version` still 3.0.0)

`useApi.ts` is untyped `ofetch`; call sites cast. This plan stops the triple-edit.

## Chosen direction

- **Runtime source of truth:** Go types / handlers
- **Published contract:** OpenAPI
- **Frontend types:** generated from OpenAPI
- **CI:** fail on drift

**This PR is the conservative first slice:** generate TypeScript from the existing `openapi.yaml` and fail CI if the generated file was hand-edited. Generating OpenAPI from Go (swag / huma / a custom emitter) is a follow-up. Annotating every handler for swag, or adopting Huma, is not reviewable in the same change as the TS pipeline.

The published YAML is stale in places (SunsetQueueItem is referenced but missing; DiskGroup omits `mode`; WorkerStats still has `lastRunFlagged`). This PR updates those schemas to match current Go so generated types typecheck. After that, resource types come from the generator, not from new hand-written structs in `api.ts`.

## Layout

| Path | Role |
|------|------|
| `docs/reference/api/openapi.yaml` | Published contract (hand-maintained until the Go generator lands) |
| `frontend/app/types/generated/openapi.ts` | Generated. Do not edit. |
| `frontend/app/types/api.ts` | Barrel: re-exports generated schema types under the names composables already import |
| `frontend/app/types/api-extra.ts` | Client/UI-only types (`ApiError`, `SelectedDetailItem`) and SSE payloads not in the REST spec |

## Commands

- `make api:generate` — regenerate `openapi.ts` from the YAML
- `make api:check` — regenerate into a temp file and diff (CI)
- `make check` and the frontend lint job run `api:check`

## Follow-up (not this PR)

Generate OpenAPI from Go so the YAML is no longer hand-edited. Keep the same TS generate + drift check. Do not add a heavy HTTP framework for that step.

## Done when

- [x] One generate command (`make api:generate`)
- [x] CI drift check (`make api:check` + lint-frontend job)
- [x] Composables still typecheck
- [x] No new hand-written structs in `api.ts` for resources that exist as OpenAPI schemas
