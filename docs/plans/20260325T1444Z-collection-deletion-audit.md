# Collection Deletion — End-to-End Audit

**Status:** 📋 Planned
**Branch:** TBD (`audit/collection-deletion`)
**Depends on:** Working Radarr/Plex/Jellyfin/Emby integrations

## Problem

The collection deletion feature allows all items in a media collection to be deleted together when one member is selected for deletion. This feature has been implemented across multiple files, but has not been thoroughly verified end-to-end. Given the pattern of features being added but not fully wired, a comprehensive audit is needed.

## Audit Checklist

### 1. Frontend Toggle
- [ ] `collectionDeletion` toggle on integration card works (just added)
- [ ] `collectionDeletion` toggle in edit modal works
- [ ] Toggle state persists after page reload
- [ ] Toggle is only shown for supported types: Radarr, Plex, Jellyfin, Emby

### 2. Backend API
- [ ] PUT `/api/v1/integrations/:id` accepts and persists `collectionDeletion` field
- [ ] The field is returned in GET `/api/v1/integrations` response
- [ ] Toggling publishes `IntegrationUpdatedEvent` for cache invalidation

### 3. Collection Data Enrichment
- [ ] `CollectionDataProvider` interface implemented for Plex, Jellyfin, Emby
- [ ] `CollectionEnricher` runs in the enrichment pipeline
- [ ] `collections` field populated on `MediaItem` when enrichment succeeds
- [ ] Collections appear in the Library UI (indigo badge on items)

### 4. Collection Resolution
- [ ] `CollectionResolver` interface implemented for Radarr
- [ ] Plex/Jellyfin/Emby implement `CollectionResolver` (or is it only Radarr?)
- [ ] `ResolveCollectionMembers()` returns all items in the same collection

### 5. Poller Evaluation
- [ ] `evaluate.go` checks `collectionDeletion` flag per integration
- [ ] When enabled and an item is selected for deletion, collection members are expanded
- [ ] Collection expansion happens BEFORE the deletion queue
- [ ] `collectionGroup` field is set on expanded items for audit trail

### 6. Deletion Service
- [ ] `deletion.go` handles collection-grouped items
- [ ] All collection members are deleted together
- [ ] Audit log records the collection group name

### 7. Approval Queue
- [ ] Collection groups appear correctly in the approval queue
- [ ] Approving/rejecting affects the entire collection group
- [ ] The UI shows collection membership (indigo badge)

### 8. Notifications
- [ ] Deletion notifications mention collection name when applicable
- [ ] Cycle digest includes collection deletion counts

### 9. Edge Cases
- [ ] What happens when collection deletion is enabled but no collection data is available?
- [ ] What happens when an item belongs to multiple collections?
- [ ] What happens when collection members span different integrations?
- [ ] What happens when some collection members are protected by rules?

## Files to Audit

| Component | Files |
|-----------|-------|
| Frontend toggle | `SettingsIntegrations.vue` |
| Backend API | `routes/integrations.go`, `services/integration.go` |
| Collection enrichment | `integrations/enrichers.go`, `integrations/plex.go`, `integrations/jellyfin.go`, `integrations/emby.go` |
| Collection resolution | `integrations/registry.go`, `integrations/radarr.go` |
| Poller evaluation | `poller/evaluate.go` |
| Deletion service | `services/deletion.go`, `services/approval.go` |
| Audit logging | `services/auditlog.go` |
| Notifications | `services/notification_dispatch.go` |

## Verification Method

This audit requires a running instance with:
- At least one Radarr integration with `collectionDeletion` enabled
- A Plex/Jellyfin/Emby integration for collection data enrichment
- Movies that belong to collections (e.g., "Serenity" in a Firefly collection)
- The engine set to dry-run mode to safely test without actual deletions
