-- +goose Up
-- Capacitarr 2.0 baseline migration.
-- Clean-slate schema — no migration path from 1.x incremental migrations.
-- For 1.x users, a separate migration tool imports configuration data.
-- See: docs/plans/20260318T2119Z-capacitarr-2.0-plan.md

-- ============================================================================
-- Auth
-- ============================================================================

CREATE TABLE auth_configs (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    username     TEXT    NOT NULL,
    password     TEXT    NOT NULL,                        -- bcrypt hash
    api_key      TEXT,                                    -- SHA-256 hash (sha256:<hex>) or legacy plaintext
    api_key_hint TEXT    NOT NULL DEFAULT '',              -- Last 4 chars of plaintext key
    created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX idx_auth_configs_username ON auth_configs(username);
CREATE INDEX idx_auth_configs_api_key ON auth_configs(api_key);

-- ============================================================================
-- Disk Groups
-- ============================================================================

CREATE TABLE disk_groups (
    id                   INTEGER PRIMARY KEY AUTOINCREMENT,
    mount_path           TEXT    NOT NULL,
    total_bytes          INTEGER NOT NULL,
    used_bytes           INTEGER NOT NULL,
    total_bytes_override INTEGER DEFAULT NULL,             -- User-defined total; NULL = use detected
    threshold_pct        REAL    NOT NULL DEFAULT 85,       -- Clean up at this %
    target_pct           REAL    NOT NULL DEFAULT 75,       -- Free down to this %
    created_at           DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at           DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX idx_disk_groups_mount_path ON disk_groups(mount_path);

-- ============================================================================
-- Libraries (NEW in 2.0)
-- Groups integrations into a logical library with optional threshold overrides.
-- A library belongs to a disk group. Integrations belong to a library.
-- Threshold hierarchy: integration override → library override → disk group default.
-- ============================================================================

CREATE TABLE libraries (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    name          TEXT    NOT NULL,
    disk_group_id INTEGER REFERENCES disk_groups(id) ON DELETE SET NULL,
    threshold_pct REAL    DEFAULT NULL,                    -- Override disk group threshold; NULL = inherit
    target_pct    REAL    DEFAULT NULL,                    -- Override disk group target; NULL = inherit
    created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_libraries_disk_group_id ON libraries(disk_group_id);

-- ============================================================================
-- Integration Configs
-- ============================================================================

CREATE TABLE integration_configs (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    type             TEXT    NOT NULL,                     -- plex, sonarr, radarr, lidarr, readarr, tautulli, seerr, jellyfin, emby
    name             TEXT    NOT NULL,                     -- User-friendly name
    url              TEXT    NOT NULL,
    api_key          TEXT    NOT NULL,                     -- API key or Plex token (plaintext — see security note in models.go)
    enabled          INTEGER NOT NULL DEFAULT 1,
    library_id       INTEGER REFERENCES libraries(id) ON DELETE SET NULL,  -- Optional library grouping
    media_size_bytes INTEGER NOT NULL DEFAULT 0,
    media_count      INTEGER NOT NULL DEFAULT 0,
    last_sync        DATETIME,
    last_error           TEXT,
    collection_deletion  INTEGER NOT NULL DEFAULT 0,          -- When enabled, deleting one collection member deletes all
    created_at           DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at           DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_integration_configs_type ON integration_configs(type);
CREATE INDEX idx_integration_configs_library_id ON integration_configs(library_id);

-- ============================================================================
-- Disk Group ↔ Integration junction (repopulated each poll cycle)
-- ============================================================================

CREATE TABLE disk_group_integrations (
    disk_group_id  INTEGER NOT NULL REFERENCES disk_groups(id) ON DELETE CASCADE,
    integration_id INTEGER NOT NULL REFERENCES integration_configs(id) ON DELETE CASCADE,
    PRIMARY KEY (disk_group_id, integration_id)
);

-- ============================================================================
-- Library History (time-series capacity data)
-- ============================================================================

CREATE TABLE library_histories (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp      DATETIME NOT NULL,
    total_capacity INTEGER  NOT NULL,
    used_capacity  INTEGER  NOT NULL,
    resolution     TEXT     NOT NULL,                      -- "raw", "hourly", "daily", "weekly"
    disk_group_id  INTEGER  REFERENCES disk_groups(id) ON DELETE CASCADE,
    library_id     INTEGER  REFERENCES libraries(id) ON DELETE CASCADE,  -- Optional library-level history
    created_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_library_histories_timestamp ON library_histories(timestamp);
CREATE INDEX idx_library_histories_resolution ON library_histories(resolution);
CREATE INDEX idx_library_histories_disk_group_id ON library_histories(disk_group_id);
CREATE INDEX idx_library_histories_library_id ON library_histories(library_id);

-- ============================================================================
-- Preferences (singleton row — global application settings)
-- Scoring factor weights are in the scoring_factor_weights table.
-- ============================================================================

CREATE TABLE preference_sets (
    id                             INTEGER PRIMARY KEY AUTOINCREMENT,
    log_level                      TEXT    NOT NULL DEFAULT 'info',
    audit_log_retention_days       INTEGER NOT NULL DEFAULT 30,
    poll_interval_seconds          INTEGER NOT NULL DEFAULT 300,
    -- Engine settings
    execution_mode                 TEXT    NOT NULL DEFAULT 'dry-run',
    tiebreaker_method              TEXT    NOT NULL DEFAULT 'size_desc',
    deletions_enabled              INTEGER NOT NULL DEFAULT 1,
    snooze_duration_hours          INTEGER NOT NULL DEFAULT 24,
    check_for_updates              INTEGER NOT NULL DEFAULT 1,
    deletion_queue_delay_seconds   INTEGER NOT NULL DEFAULT 30,   -- Grace period before processing queued deletions (10-300)
    -- Analytics thresholds
    dead_content_min_days          INTEGER NOT NULL DEFAULT 90,   -- Minimum days in library for "dead content" report
    stale_content_days             INTEGER NOT NULL DEFAULT 180,  -- Days since last watch for "stale content" report
    updated_at                     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- ============================================================================
-- Scoring Factor Weights (dynamic registry — one row per factor)
-- Auto-seeded from engine.DefaultFactors() on startup. Adding a new factor
-- implementation is all that's needed to register a new weight row.
-- ============================================================================

CREATE TABLE scoring_factor_weights (
    factor_key TEXT    PRIMARY KEY,
    weight     INTEGER NOT NULL DEFAULT 5 CHECK(weight >= 0 AND weight <= 10),
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- ============================================================================
-- Custom Rules (scoring influence)
-- ============================================================================

CREATE TABLE custom_rules (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    integration_id INTEGER REFERENCES integration_configs(id) ON DELETE CASCADE,
    library_id     INTEGER REFERENCES libraries(id) ON DELETE CASCADE,  -- NEW in 2.0: per-library rule scoping
    field          TEXT    NOT NULL,
    operator       TEXT    NOT NULL,
    value          TEXT    NOT NULL,
    effect         TEXT    NOT NULL CHECK(effect IN (
        'always_keep','prefer_keep','lean_keep',
        'lean_remove','prefer_remove','always_remove'
    )),
    enabled        INTEGER NOT NULL DEFAULT 1,
    sort_order     INTEGER NOT NULL DEFAULT 0,
    created_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_custom_rules_integration_id ON custom_rules(integration_id);
CREATE INDEX idx_custom_rules_library_id ON custom_rules(library_id);

-- ============================================================================
-- Approval Queue (state machine: pending → approved/rejected → deleted)
-- ============================================================================

CREATE TABLE approval_queue (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    media_name     TEXT    NOT NULL,
    media_type     TEXT    NOT NULL CHECK(media_type IN ('movie','show','season','episode','artist','album','book')),
    score_details  TEXT,                                   -- JSON-encoded []ScoreFactor
    size_bytes     INTEGER NOT NULL DEFAULT 0,
    score          REAL    NOT NULL DEFAULT 0,
    poster_url     TEXT    NOT NULL DEFAULT '',             -- Poster image URL from *arr
    integration_id INTEGER NOT NULL REFERENCES integration_configs(id) ON DELETE CASCADE,
    external_id    TEXT    NOT NULL DEFAULT '',
    disk_group_id  INTEGER REFERENCES disk_groups(id) ON DELETE SET NULL,
    status         TEXT    NOT NULL DEFAULT 'pending' CHECK(status IN ('pending','approved','rejected')),
    trigger        TEXT    NOT NULL DEFAULT 'engine',       -- "engine", "user"
    user_initiated   INTEGER NOT NULL DEFAULT 0,              -- True when queued by user via POST /delete (preserved on queue clear)
    collection_group TEXT    NOT NULL DEFAULT '',              -- Groups collection members (e.g., "Sonic the Hedgehog Collection")
    snoozed_until    DATETIME,
    created_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_approval_queue_status ON approval_queue(status);
CREATE INDEX idx_approval_queue_media ON approval_queue(media_name, media_type);
CREATE INDEX idx_approval_queue_disk_group_id ON approval_queue(disk_group_id);
CREATE INDEX idx_approval_queue_collection_group ON approval_queue(collection_group)
    WHERE collection_group != '';
CREATE INDEX idx_approval_queue_snoozed ON approval_queue(snoozed_until)
    WHERE snoozed_until IS NOT NULL;

-- ============================================================================
-- Audit Log (permanent deletion/dry-run history — append-only)
-- ============================================================================

CREATE TABLE audit_log (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    media_name     TEXT    NOT NULL,
    media_type     TEXT    NOT NULL,
    score_details  TEXT,                                   -- JSON-encoded []ScoreFactor
    action         TEXT    NOT NULL CHECK(action IN ('deleted','dry_delete','cancelled')),
    size_bytes     INTEGER NOT NULL DEFAULT 0,
    score          REAL    NOT NULL DEFAULT 0,
    trigger        TEXT    NOT NULL DEFAULT 'engine',       -- "engine", "user", "approval"
    dry_run_reason TEXT    NOT NULL DEFAULT '',              -- "deletions_disabled", "execution_mode", "" (empty if not dry-run)
    integration_id   INTEGER REFERENCES integration_configs(id) ON DELETE SET NULL,
    disk_group_id    INTEGER REFERENCES disk_groups(id) ON DELETE SET NULL,
    collection_group TEXT    NOT NULL DEFAULT '',              -- Groups collection deletions (e.g., "Sonic the Hedgehog Collection")
    created_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_audit_log_media_name ON audit_log(media_name);
CREATE INDEX idx_audit_log_action ON audit_log(action);
CREATE INDEX idx_audit_log_created_at ON audit_log(created_at);
CREATE INDEX idx_audit_log_disk_group_id ON audit_log(disk_group_id);

-- ============================================================================
-- Engine Run Stats
-- ============================================================================

CREATE TABLE engine_run_stats (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    run_at         DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    completed_at   DATETIME,
    evaluated      INTEGER  NOT NULL DEFAULT 0,
    flagged        INTEGER  NOT NULL DEFAULT 0,
    deleted        INTEGER  NOT NULL DEFAULT 0,
    freed_bytes    INTEGER  NOT NULL DEFAULT 0,
    execution_mode TEXT     NOT NULL DEFAULT 'dry-run',
    duration_ms    INTEGER  NOT NULL DEFAULT 0,
    error_message  TEXT,
    created_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_engine_run_stats_run_at ON engine_run_stats(run_at);

-- ============================================================================
-- Lifetime Stats (singleton row, never cleared)
-- ============================================================================

CREATE TABLE lifetime_stats (
    id                    INTEGER PRIMARY KEY DEFAULT 1,
    total_bytes_reclaimed INTEGER NOT NULL DEFAULT 0,
    total_items_removed   INTEGER NOT NULL DEFAULT 0,
    total_engine_runs     INTEGER NOT NULL DEFAULT 0,
    created_at            DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at            DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
INSERT INTO lifetime_stats (id) VALUES (1);

-- ============================================================================
-- Notification Configs
-- ============================================================================

CREATE TABLE notification_configs (
    id                   INTEGER PRIMARY KEY AUTOINCREMENT,
    type                 TEXT    NOT NULL,                  -- "discord", "apprise"
    name                 TEXT    NOT NULL,
    webhook_url          TEXT,                              -- Discord webhook or Apprise API endpoint URL
    apprise_tags         TEXT    NOT NULL DEFAULT '',       -- Comma-separated Apprise tags for routing
    enabled              INTEGER NOT NULL DEFAULT 1,
    -- Event subscriptions
    on_cycle_digest      INTEGER NOT NULL DEFAULT 1,
    on_error             INTEGER NOT NULL DEFAULT 1,
    on_mode_changed      INTEGER NOT NULL DEFAULT 1,
    on_server_started    INTEGER NOT NULL DEFAULT 1,
    on_threshold_breach  INTEGER NOT NULL DEFAULT 1,
    on_update_available  INTEGER NOT NULL DEFAULT 1,
    on_approval_activity   INTEGER NOT NULL DEFAULT 1,
    on_integration_status  INTEGER NOT NULL DEFAULT 1,
    created_at             DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at             DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- ============================================================================
-- Activity Events (dashboard feed — 7-day retention, auto-pruned)
-- ============================================================================

CREATE TABLE activity_events (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    event_type TEXT     NOT NULL DEFAULT '',
    message    TEXT     NOT NULL DEFAULT '',
    metadata   TEXT     DEFAULT '',                         -- Optional JSON for extra data
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_activity_events_event_type ON activity_events(event_type);
CREATE INDEX idx_activity_events_created_at ON activity_events(created_at);

-- ============================================================================
-- Media Cache (restart recovery — singleton row, id=1)
-- ============================================================================

-- Persistent media cache for restart recovery.
-- Stores a JSON snapshot of the preview result (scored media items + disk context)
-- so the dashboard and analytics have data immediately on startup without
-- waiting for the first engine run to complete.
-- This is a singleton table (id = 1) — each engine run replaces the row.

CREATE TABLE media_cache (
    id           INTEGER PRIMARY KEY CHECK (id = 1),
    preview_json TEXT     NOT NULL DEFAULT '{}',
    item_count   INTEGER  NOT NULL DEFAULT 0,
    updated_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);


-- +goose Down
DROP TABLE IF EXISTS media_cache;
DROP TABLE IF EXISTS activity_events;
DROP TABLE IF EXISTS notification_configs;
DROP TABLE IF EXISTS lifetime_stats;
DROP TABLE IF EXISTS engine_run_stats;
DROP TABLE IF EXISTS audit_log;
DROP TABLE IF EXISTS approval_queue;
DROP TABLE IF EXISTS custom_rules;
DROP TABLE IF EXISTS scoring_factor_weights;
DROP TABLE IF EXISTS preference_sets;
DROP TABLE IF EXISTS library_histories;
DROP TABLE IF EXISTS disk_group_integrations;
DROP TABLE IF EXISTS integration_configs;
DROP TABLE IF EXISTS libraries;
DROP TABLE IF EXISTS disk_groups;
DROP TABLE IF EXISTS auth_configs;
