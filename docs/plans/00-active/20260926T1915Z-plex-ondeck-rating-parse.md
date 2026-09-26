# Plex on-deck Rating array parse failure

**Status:** In progress
**Priority:** High (watchlist protection silently gone)
**Origin:** GitHub issue #67

---

## Problem

The Watchlist/Favorites enricher fails every poll when Plex on-deck JSON
includes both a scalar `rating` and a PascalCase `Rating` array. Go's
`encoding/json` matches keys case-insensitively, so the array is decoded
into `plexMetadata.Rating float64` and the whole `/library/onDeck`
response is rejected.

Users who rely on `watchlist == true → always_keep` lose protection for
every on-deck title, not just the item that carried the array. The
pipeline logs the error and continues; `OnWatchlist` stays false.

## Reporter diagnosis

Correct. The collision is real and matches a Plex XML-to-JSON quirk this
file already solved for `guid` / `Guid`.

The suggested `[]json.RawMessage` sink is the right *mechanism* (exact
tag so the array no longer falls through to `rating`). It is not the
pattern this package already uses. Follow `GUID` / `GUIDs` instead:

```go
Rating  float64      `json:"rating"`
Ratings []plexRating `json:"Rating,omitempty"`
```

A custom type that accepts number-or-array is unnecessary here. If both
keys are present, case-insensitive matching still visits the same field
unless the array has its own exact tag.

## In scope

1. Give the PascalCase array its own exact tag on `plexMetadata`.
2. Request `includeGuids=1` on `/library/onDeck`. Library fetch and search
   already require it for TMDb extraction; on-deck did not. A successful
   parse with an empty `Guid` array still yields zero watchlist matches.
3. Regression tests against a raw JSON fixture that contains both
   `rating` (number) and `Rating` (array) on one item. Struct-encoded
   tests cannot reproduce the collision.

`plexMetadata` is shared by library fetch, on-deck, and hub search. The
struct fix covers all three. Library fetch currently swallows unmarshal
errors with `continue` — that is a separate reliability issue.

## Out of scope

- Per-item unmarshal so one bad row cannot fail the container. Wrong
  shape for this bug; revisit if Plex adds more colliding keys.
- Episode on-deck TMDb vs show TMDb matching. Different bug.
- Logging the silent `continue` in `fetchMediaItems`. Different PR.
- Changing how `AudienceRating` / `Rating` feed `MediaItem.Rating`.

## Tests

- Unmarshal `plexMediaResponse` from raw JSON with both keys; assert no
  error and scalar `rating` preserved.
- `GetOnDeckItems` against that payload returns the item's TMDb ID.
- `GetOnDeckItems` request includes `includeGuids=1`.

## Done when

- [x] On-deck parse no longer fails on a `Rating` array
- [x] On-deck request asks for GUIDs
- [x] `go test ./internal/integrations/` is green
