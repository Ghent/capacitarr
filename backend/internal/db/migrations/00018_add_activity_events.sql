-- +goose Up
CREATE TABLE IF NOT EXISTS activity_events (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    event_type TEXT    NOT NULL DEFAULT '',
    message    TEXT    NOT NULL DEFAULT '',
    metadata   TEXT    DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_activity_events_event_type ON activity_events(event_type);
CREATE INDEX IF NOT EXISTS idx_activity_events_created_at ON activity_events(created_at);

-- +goose Down
DROP INDEX IF EXISTS idx_activity_events_created_at;
DROP INDEX IF EXISTS idx_activity_events_event_type;
DROP TABLE IF EXISTS activity_events;
