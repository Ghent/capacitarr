-- +goose Up
-- Allow pending_delete audit actions so live deletes can write an intent row
-- before calling the *arr API, then complete the row to 'deleted'.
-- SQLite requires table recreation to alter CHECK constraints.

CREATE TABLE audit_log_new (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    media_name       TEXT    NOT NULL,
    media_type       TEXT    NOT NULL,
    score_details    TEXT,
    action           TEXT    NOT NULL CHECK(action IN ('deleted','dry_delete','cancelled','pending_delete')),
    size_bytes       INTEGER NOT NULL DEFAULT 0,
    score            REAL    NOT NULL DEFAULT 0,
    trigger          TEXT    NOT NULL DEFAULT 'engine',
    dry_run_reason   TEXT    NOT NULL DEFAULT '',
    integration_id   INTEGER REFERENCES integration_configs(id) ON DELETE SET NULL,
    disk_group_id    INTEGER REFERENCES disk_groups(id) ON DELETE SET NULL,
    collection_group TEXT    NOT NULL DEFAULT '',
    created_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
INSERT INTO audit_log_new SELECT * FROM audit_log;
DROP TABLE audit_log;
ALTER TABLE audit_log_new RENAME TO audit_log;
CREATE INDEX IF NOT EXISTS idx_audit_log_media_name ON audit_log(media_name);
CREATE INDEX IF NOT EXISTS idx_audit_log_action ON audit_log(action);
CREATE INDEX IF NOT EXISTS idx_audit_log_created_at ON audit_log(created_at);
CREATE INDEX IF NOT EXISTS idx_audit_log_disk_group_id ON audit_log(disk_group_id);

-- +goose Down
CREATE TABLE audit_log_old (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    media_name       TEXT    NOT NULL,
    media_type       TEXT    NOT NULL,
    score_details    TEXT,
    action           TEXT    NOT NULL CHECK(action IN ('deleted','dry_delete','cancelled')),
    size_bytes       INTEGER NOT NULL DEFAULT 0,
    score            REAL    NOT NULL DEFAULT 0,
    trigger          TEXT    NOT NULL DEFAULT 'engine',
    dry_run_reason   TEXT    NOT NULL DEFAULT '',
    integration_id   INTEGER REFERENCES integration_configs(id) ON DELETE SET NULL,
    disk_group_id    INTEGER REFERENCES disk_groups(id) ON DELETE SET NULL,
    collection_group TEXT    NOT NULL DEFAULT '',
    created_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
INSERT INTO audit_log_old (
    id, media_name, media_type, score_details, action, size_bytes, score,
    trigger, dry_run_reason, integration_id, disk_group_id, collection_group, created_at
)
SELECT
    id, media_name, media_type, score_details,
    CASE action WHEN 'pending_delete' THEN 'deleted' ELSE action END,
    size_bytes, score, trigger, dry_run_reason, integration_id, disk_group_id,
    collection_group, created_at
FROM audit_log;
DROP TABLE audit_log;
ALTER TABLE audit_log_old RENAME TO audit_log;
CREATE INDEX IF NOT EXISTS idx_audit_log_media_name ON audit_log(media_name);
CREATE INDEX IF NOT EXISTS idx_audit_log_action ON audit_log(action);
CREATE INDEX IF NOT EXISTS idx_audit_log_created_at ON audit_log(created_at);
CREATE INDEX IF NOT EXISTS idx_audit_log_disk_group_id ON audit_log(disk_group_id);
