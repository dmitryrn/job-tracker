-- +goose Up
ALTER TABLE user_profiles DROP COLUMN location;

-- +goose Down
ALTER TABLE user_profiles ADD COLUMN location TEXT NOT NULL DEFAULT '';
