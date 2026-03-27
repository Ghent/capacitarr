-- +goose Up
-- Add a rollup_state table to persist the last successful rollup timestamp
-- per resolution tier. This makes cron rollup jobs idempotent and tolerant
-- of scheduling delays — each job processes data since its last checkpoint
-- instead of computing rollup windows from time.Now().

CREATE TABLE IF NOT EXISTS rollup_states (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    resolution     TEXT NOT NULL UNIQUE,
    last_completed DATETIME NOT NULL
);

-- Seed initial rows so the first rollup uses a reasonable lookback.
-- Each resolution starts 2x its period in the past.
INSERT OR IGNORE INTO rollup_states (resolution, last_completed) VALUES ('hourly', datetime('now', '-2 hours'));
INSERT OR IGNORE INTO rollup_states (resolution, last_completed) VALUES ('daily', datetime('now', '-2 days'));
INSERT OR IGNORE INTO rollup_states (resolution, last_completed) VALUES ('weekly', datetime('now', '-14 days'));

-- +goose Down
DROP TABLE IF EXISTS rollup_states;
