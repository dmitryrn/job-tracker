-- +goose Up
ALTER TABLE discovery_settings ADD COLUMN linkedin_posted_within TEXT NOT NULL DEFAULT '';
ALTER TABLE discovery_settings ADD COLUMN linkedin_workplace TEXT NOT NULL DEFAULT '';
ALTER TABLE discovery_settings ADD COLUMN linkedin_experience_level TEXT NOT NULL DEFAULT '';
UPDATE discovery_settings SET linkedin_location = 'Europe' WHERE linkedin_location = '';

-- +goose Down
ALTER TABLE discovery_settings DROP COLUMN linkedin_experience_level;
ALTER TABLE discovery_settings DROP COLUMN linkedin_workplace;
ALTER TABLE discovery_settings DROP COLUMN linkedin_posted_within;
