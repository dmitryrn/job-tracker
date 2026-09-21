-- +goose Up
ALTER TABLE user_profiles ADD COLUMN github_url TEXT NOT NULL DEFAULT '';
ALTER TABLE user_profiles ADD COLUMN linkedin_url TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE user_profiles DROP COLUMN linkedin_url;
ALTER TABLE user_profiles DROP COLUMN github_url;
