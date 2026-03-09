-- +goose Up
ALTER TABLE notification_configs ADD COLUMN apprise_tags TEXT NOT NULL DEFAULT '';
DELETE FROM notification_configs WHERE type = 'slack';

-- +goose Down
ALTER TABLE notification_configs DROP COLUMN apprise_tags;
