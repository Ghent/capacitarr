-- +goose Up
-- Add poster_url column to approval_queue for grid view poster display.
ALTER TABLE approval_queue ADD COLUMN poster_url TEXT NOT NULL DEFAULT '';

-- +goose Down
-- SQLite does not support DROP COLUMN before 3.35.0; recreate table without poster_url.
-- However, since ncruces/go-sqlite3 bundles a modern SQLite, we can use ALTER TABLE DROP COLUMN.
ALTER TABLE approval_queue DROP COLUMN poster_url;
