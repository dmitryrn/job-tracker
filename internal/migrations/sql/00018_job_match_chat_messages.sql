-- +goose Up
CREATE TABLE job_match_chat_messages (
    id INTEGER PRIMARY KEY,
    job_id INTEGER NOT NULL REFERENCES job_matches(job_id) ON DELETE CASCADE,
    role TEXT NOT NULL CHECK (role IN ('user', 'assistant')),
    content TEXT NOT NULL,
    created_at TEXT NOT NULL
);

CREATE INDEX job_match_chat_messages_job_id_id ON job_match_chat_messages(job_id, id);

-- +goose Down
DROP TABLE job_match_chat_messages;
