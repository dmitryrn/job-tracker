-- +goose Up
ALTER TABLE discovery_settings ADD COLUMN adzuna_enabled INTEGER NOT NULL DEFAULT 1;
ALTER TABLE discovery_settings ADD COLUMN remotive_enabled INTEGER NOT NULL DEFAULT 1;
ALTER TABLE discovery_settings ADD COLUMN jobicy_enabled INTEGER NOT NULL DEFAULT 1;

-- +goose Down
ALTER TABLE discovery_settings DROP COLUMN jobicy_enabled;
ALTER TABLE discovery_settings DROP COLUMN remotive_enabled;
ALTER TABLE discovery_settings DROP COLUMN adzuna_enabled;
