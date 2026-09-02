-- +goose Up
ALTER TABLE discovery_settings ADD COLUMN linkedin_enabled INTEGER NOT NULL DEFAULT 0;
ALTER TABLE discovery_settings ADD COLUMN linkedin_query TEXT NOT NULL DEFAULT 'software engineer';
ALTER TABLE discovery_settings ADD COLUMN linkedin_location TEXT NOT NULL DEFAULT '';
ALTER TABLE discovery_settings ADD COLUMN linkedin_limit INTEGER NOT NULL DEFAULT 25;

-- +goose Down
ALTER TABLE discovery_settings DROP COLUMN linkedin_limit;
ALTER TABLE discovery_settings DROP COLUMN linkedin_location;
ALTER TABLE discovery_settings DROP COLUMN linkedin_query;
ALTER TABLE discovery_settings DROP COLUMN linkedin_enabled;
