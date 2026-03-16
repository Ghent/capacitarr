-- +goose Up
CREATE TABLE IF NOT EXISTS disk_group_integrations (
    disk_group_id   INTEGER NOT NULL REFERENCES disk_groups(id) ON DELETE CASCADE,
    integration_id  INTEGER NOT NULL REFERENCES integration_configs(id) ON DELETE CASCADE,
    PRIMARY KEY (disk_group_id, integration_id)
);

-- +goose Down
DROP TABLE IF EXISTS disk_group_integrations;
