-- +goose Up
-- Add fields to audit_logs for reconstructing delete jobs from approval entries
ALTER TABLE audit_logs ADD COLUMN integration_id INTEGER DEFAULT NULL;
ALTER TABLE audit_logs ADD COLUMN external_id TEXT DEFAULT '';

-- +goose Down
-- SQLite does not support DROP COLUMN before 3.35; handled by full rebuild if needed
