# Seerr Requested Filter — Verification

**Status:** 📋 Planned (blocked on Seerr connection)
**Branch:** N/A (verification only, no code changes expected)

## Problem

The "Requested" smart filter on the Library page checks `e.item.isRequested`. This field is populated by the Seerr enrichment pipeline. With the Seerr integration currently showing a connection error, the filter cannot be verified.

## Verification Steps

Once the Seerr integration is connected and healthy:

1. [ ] Navigate to the Library page
2. [ ] Verify the "Requested" filter badge shows a non-zero count
3. [ ] Click "Requested" — verify only items with `isRequested=true` appear
4. [ ] Open a requested item's score detail — verify the "Request Popularity" factor shows non-zero contribution
5. [ ] Verify `requestedBy` field is populated in the item detail
6. [ ] Verify `requestCount` field is populated

## If Verification Fails

Check the enrichment pipeline:
- `backend/internal/integrations/seerr.go` — `GetRequestedMedia()` implementation
- `backend/internal/integrations/enrichers.go` — Seerr enricher registration
- `backend/internal/integrations/enrichment_pipeline.go` — Pipeline execution

The `isRequested` field on `MediaItem` is set by the Seerr enricher matching TMDb IDs between Seerr requests and *arr media items.
