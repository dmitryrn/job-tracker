-- +goose Up
DROP INDEX job_match_chat_items_job_id_request_id_type;
CREATE UNIQUE INDEX job_match_chat_items_job_id_request_id_type
    ON job_match_chat_items(job_id, request_id, item_type)
    WHERE request_id IS NOT NULL AND item_type = 'user_message';

-- +goose Down
DROP INDEX job_match_chat_items_job_id_request_id_type;
CREATE UNIQUE INDEX job_match_chat_items_job_id_request_id_type
    ON job_match_chat_items(job_id, request_id, item_type)
    WHERE request_id IS NOT NULL;
