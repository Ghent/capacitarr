-- +goose Up
-- Drop the vestigial execution_mode column from engine_run_stats.
-- This column recorded the global DefaultDiskGroupMode preference at each run,
-- which is inaccurate for multi-disk-group setups. Per-group modes are now
-- stored in the disk_group_modes JSON column (added in migration 00011).
ALTER TABLE engine_run_stats DROP COLUMN execution_mode;

-- +goose Down
ALTER TABLE engine_run_stats ADD COLUMN execution_mode TEXT NOT NULL DEFAULT 'dry-run';
