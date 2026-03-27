-- +goose Up
-- Add a schema_info table that explicitly identifies the migration lineage.
-- The schema_family column distinguishes 2.0 databases from 1.x databases,
-- replacing the previous heuristic of checking for specific table existence
-- (which broke when the libraries table was dropped by migration 00003).
--
-- Detection runs BEFORE Goose on each startup, so this table must exist from
-- a previous successful run. On the first run after this migration is applied,
-- the schema_info row is written and all subsequent startups can identify the
-- database as 2.0 without ambiguity.

CREATE TABLE IF NOT EXISTS schema_info (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

INSERT OR IGNORE INTO schema_info (key, value) VALUES ('schema_family', 'v2');

-- +goose Down
DROP TABLE IF EXISTS schema_info;
