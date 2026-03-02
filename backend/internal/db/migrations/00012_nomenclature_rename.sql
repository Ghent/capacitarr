-- +goose Up
-- Migration 00012: Rename protection_rules → custom_rules, availability_weight → series_status_weight

-- Rename the protection_rules table to custom_rules
ALTER TABLE protection_rules RENAME TO custom_rules;

-- Rename the availability_weight column in preference_sets
ALTER TABLE preference_sets RENAME COLUMN availability_weight TO series_status_weight;

-- Update any existing custom rules that use "availability" as a field name
UPDATE custom_rules SET field = 'seriesstatus' WHERE field = 'availability';

-- +goose Down
UPDATE custom_rules SET field = 'availability' WHERE field = 'seriesstatus';
ALTER TABLE preference_sets RENAME COLUMN series_status_weight TO availability_weight;
ALTER TABLE custom_rules RENAME TO protection_rules;
