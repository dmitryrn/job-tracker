-- +goose Up
ALTER TABLE jobs ADD COLUMN workplace_classification_json TEXT;

-- +goose Down
ALTER TABLE jobs DROP COLUMN workplace_classification_json;
