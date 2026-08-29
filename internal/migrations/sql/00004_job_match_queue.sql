-- +goose Up
CREATE TABLE job_match_queue (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    job_ids_json TEXT NOT NULL
);

INSERT INTO job_match_queue (id, job_ids_json) VALUES (1, '[]');

-- +goose Down
DROP TABLE job_match_queue;
