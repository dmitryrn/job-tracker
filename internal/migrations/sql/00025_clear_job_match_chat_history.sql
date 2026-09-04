-- +goose Up
DELETE FROM job_match_chat_items;

-- +goose Down
-- Chat history deletion is irreversible.
