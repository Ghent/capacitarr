-- +goose Up
-- Baseline migration for the service-layer-event-bus refactor.
-- This is a clean-slate schema — no migration path from previous versions.
-- Existing databases are incompatible; users start fresh on upgrade.

-- ============================================================================
-- Auth
-- ============================================================================

CREATE TABLE auth_configs (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    username   TEXT NOT NULL,
    password   TEXT NOT NULL,
    api_key    TEXT,
    api_key_hint TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX idx_auth_configs_username ON auth_configs(username);
CREATE INDEX idx_auth_configs_api_key ON auth_configs(api_key);

-- ============================================================================
-- Disk Groups
-- ============================================================================

CREATE TABLE disk_groups (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    mount_path    TEXT    NOT NULL,
    total_bytes   INTEGER NOT NULL,
    used_bytes    INTEGER NOT NULL,
    threshold_pct REAL    NOT NULL DEFAULT 85,
    target_pct    REAL    NOT NULL DEFAULT 75,
    created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX idx_disk_groups_mount_path ON disk_groups(mount_path);

-- ============================================================================
-- Integration Configs
-- ============================================================================

CREATE TABLE integration_configs (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    type             TEXT    NOT NULL,
    name             TEXT    NOT NULL,
    url              TEXT    NOT NULL,
    api_key          TEXT    NOT NULL,
    enabled          INTEGER NOT NULL DEFAULT 1,
    media_size_bytes INTEGER NOT NULL DEFAULT 0,
    media_count      INTEGER NOT NULL DEFAULT 0,
    last_sync        DATETIME,
    last_error       TEXT,
    created_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_integration_configs_type ON integration_configs(type);

-- ============================================================================
-- Library History (time-series capacity data)
-- ============================================================================

CREATE TABLE library_histories (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp      DATETIME NOT NULL,
    total_capacity INTEGER  NOT NULL,
    used_capacity  INTEGER  NOT NULL,
    resolution     TEXT     NOT NULL,
    disk_group_id  INTEGER  REFERENCES disk_groups(id) ON DELETE CASCADE,
    created_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_library_histories_timestamp ON library_histories(timestamp);
CREATE INDEX idx_library_histories_resolution ON library_histories(resolution);
CREATE INDEX idx_library_histories_disk_group_id ON library_histories(disk_group_id);

-- ============================================================================
-- Preferences
-- ============================================================================

CREATE TABLE preference_sets (
    id                       INTEGER PRIMARY KEY AUTOINCREMENT,
    log_level                TEXT    NOT NULL DEFAULT 'info',
    audit_log_retention_days INTEGER NOT NULL DEFAULT 30,
    poll_interval_seconds    INTEGER NOT NULL DEFAULT 300,
    watch_history_weight     INTEGER NOT NULL DEFAULT 10,
    last_watched_weight      INTEGER NOT NULL DEFAULT 8,
    file_size_weight         INTEGER NOT NULL DEFAULT 6,
    rating_weight            INTEGER NOT NULL DEFAULT 5,
    time_in_library_weight   INTEGER NOT NULL DEFAULT 4,
    series_status_weight     INTEGER NOT NULL DEFAULT 3,
    execution_mode           TEXT    NOT NULL DEFAULT 'dry-run',
    tiebreaker_method        TEXT    NOT NULL DEFAULT 'size_desc',
    deletions_enabled        INTEGER NOT NULL DEFAULT 1,
    snooze_duration_hours    INTEGER NOT NULL DEFAULT 24,
    check_for_updates        INTEGER NOT NULL DEFAULT 1,
    updated_at               DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- ============================================================================
-- Custom Rules (scoring influence)
-- ============================================================================

CREATE TABLE custom_rules (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    integration_id INTEGER REFERENCES integration_configs(id) ON DELETE CASCADE,
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

-- ============================================================================
-- Approval Queue (state machine: pending → approved/rejected → deleted)
-- ============================================================================

CREATE TABLE approval_queue (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    media_name     TEXT    NOT NULL,
    media_type     TEXT    NOT NULL CHECK(media_type IN ('movie','show','season','episode','artist','album','book')),
    reason         TEXT    NOT NULL,
    score_details  TEXT,
    size_bytes     INTEGER NOT NULL DEFAULT 0,
    integration_id INTEGER NOT NULL REFERENCES integration_configs(id) ON DELETE CASCADE,
    external_id    TEXT    NOT NULL DEFAULT '',
    status         TEXT    NOT NULL DEFAULT 'pending' CHECK(status IN ('pending','approved','rejected')),
    snoozed_until  DATETIME,
    created_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_approval_queue_status ON approval_queue(status);
CREATE INDEX idx_approval_queue_media ON approval_queue(media_name, media_type);
CREATE INDEX idx_approval_queue_snoozed ON approval_queue(snoozed_until)
    WHERE snoozed_until IS NOT NULL;

-- ============================================================================
-- Audit Log (permanent deletion/dry-run history — append-only)
-- ============================================================================

CREATE TABLE audit_log (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    media_name     TEXT    NOT NULL,
    media_type     TEXT    NOT NULL,
    reason         TEXT    NOT NULL,
    score_details  TEXT,
    action         TEXT    NOT NULL CHECK(action IN ('deleted','dry_run','dry_delete')),
    size_bytes     INTEGER NOT NULL DEFAULT 0,
    integration_id INTEGER REFERENCES integration_configs(id) ON DELETE SET NULL,
    created_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_audit_log_media_name ON audit_log(media_name);
CREATE INDEX idx_audit_log_action ON audit_log(action);
CREATE INDEX idx_audit_log_created_at ON audit_log(created_at);

-- ============================================================================
-- Engine Run Stats
-- ============================================================================

CREATE TABLE engine_run_stats (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    run_at         DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
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
    type                 TEXT    NOT NULL,
    name                 TEXT    NOT NULL,
    webhook_url          TEXT,
    enabled              INTEGER NOT NULL DEFAULT 1,
    on_threshold_breach  INTEGER NOT NULL DEFAULT 1,
    on_deletion_executed INTEGER NOT NULL DEFAULT 1,
    on_engine_error      INTEGER NOT NULL DEFAULT 1,
    on_engine_complete   INTEGER NOT NULL DEFAULT 0,
    created_at           DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at           DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- ============================================================================
-- In-App Notifications
-- ============================================================================

CREATE TABLE in_app_notifications (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    title      TEXT    NOT NULL,
    message    TEXT    NOT NULL,
    severity   TEXT    NOT NULL DEFAULT 'info',
    read       INTEGER NOT NULL DEFAULT 0,
    event_type TEXT    NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_in_app_notifications_read ON in_app_notifications(read);
CREATE INDEX idx_in_app_notifications_created_at ON in_app_notifications(created_at);

-- ============================================================================
-- Activity Events (dashboard feed — 7-day retention, auto-pruned)
-- ============================================================================

CREATE TABLE activity_events (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    event_type TEXT     NOT NULL DEFAULT '',
    message    TEXT     NOT NULL DEFAULT '',
    metadata   TEXT     DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_activity_events_event_type ON activity_events(event_type);
CREATE INDEX idx_activity_events_created_at ON activity_events(created_at);


-- +goose Down
DROP TABLE IF EXISTS activity_events;
DROP TABLE IF EXISTS in_app_notifications;
DROP TABLE IF EXISTS notification_configs;
DROP TABLE IF EXISTS lifetime_stats;
DROP TABLE IF EXISTS engine_run_stats;
DROP TABLE IF EXISTS audit_log;
DROP TABLE IF EXISTS approval_queue;
DROP TABLE IF EXISTS custom_rules;
DROP TABLE IF EXISTS preference_sets;
DROP TABLE IF EXISTS library_histories;
DROP TABLE IF EXISTS integration_configs;
DROP TABLE IF EXISTS disk_groups;
DROP TABLE IF EXISTS auth_configs;
