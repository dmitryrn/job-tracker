-- +goose Up
ALTER TABLE jobs ADD COLUMN last_viewed_at TEXT;

-- +goose Down
ALTER TABLE jobs DROP COLUMN last_viewed_at;
