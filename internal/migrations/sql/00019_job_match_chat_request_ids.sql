-- +goose Up
ALTER TABLE job_match_chat_messages ADD COLUMN request_id TEXT;

CREATE UNIQUE INDEX job_match_chat_messages_job_id_request_id_role
    ON job_match_chat_messages(job_id, request_id, role)
    WHERE request_id IS NOT NULL;

-- +goose Down
DROP INDEX job_match_chat_messages_job_id_request_id_role;
ALTER TABLE job_match_chat_messages DROP COLUMN request_id;
