-- +goose Up
ALTER TABLE user_profiles ADD COLUMN work_history_json TEXT NOT NULL DEFAULT '[]';
ALTER TABLE user_profiles ADD COLUMN education_json TEXT NOT NULL DEFAULT '[]';

-- +goose Down
ALTER TABLE user_profiles DROP COLUMN education_json;
ALTER TABLE user_profiles DROP COLUMN work_history_json;
