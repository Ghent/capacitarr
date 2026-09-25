-- +goose Up
-- Unique hold identity: (disk_group_id, integration_id, external_id).
-- Keep one row per identity (lowest id) before adding the constraint.

DELETE FROM sunset_queue
WHERE id NOT IN (
    SELECT MIN(id) FROM sunset_queue
    GROUP BY disk_group_id, integration_id, external_id
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_sunset_queue_identity
    ON sunset_queue(disk_group_id, integration_id, external_id);

-- +goose Down
DROP INDEX IF EXISTS idx_sunset_queue_identity;
