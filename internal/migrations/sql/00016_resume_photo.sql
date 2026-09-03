-- +goose Up
ALTER TABLE resumes ADD COLUMN photo_content_type TEXT NOT NULL DEFAULT '';
ALTER TABLE resumes ADD COLUMN photo_data BLOB;

-- +goose Down
ALTER TABLE resumes DROP COLUMN photo_data;
ALTER TABLE resumes DROP COLUMN photo_content_type;
