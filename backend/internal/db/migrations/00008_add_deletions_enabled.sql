-- +goose Up
ALTER TABLE preference_sets ADD COLUMN deletions_enabled BOOLEAN NOT NULL DEFAULT 1;

-- +goose Down
-- SQLite doesn't support DROP COLUMN; this is a no-op for safety
