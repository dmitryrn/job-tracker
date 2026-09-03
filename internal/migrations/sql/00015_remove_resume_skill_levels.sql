-- +goose Up
ALTER TABLE resume_skills DROP COLUMN level;

-- +goose Down
ALTER TABLE resume_skills ADD COLUMN level TEXT NOT NULL DEFAULT '';
