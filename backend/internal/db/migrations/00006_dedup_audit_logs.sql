-- +goose Up
-- One-time cleanup: remove duplicate dry-run audit entries.
-- Keeps only the most recent entry per unique (media_name, media_type, action)
-- combination for dry-run entries. Auto/approval entries are never touched.
DELETE FROM audit_logs
WHERE action IN ('Dry-Run', 'Dry-Delete')
  AND id NOT IN (
    SELECT MAX(id)
    FROM audit_logs
    WHERE action IN ('Dry-Run', 'Dry-Delete')
    GROUP BY media_name, media_type, action
  );

-- +goose Down
-- Cannot undo deletion of duplicate rows; data is non-recoverable.
