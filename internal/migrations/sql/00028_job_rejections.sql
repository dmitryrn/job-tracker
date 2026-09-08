-- +goose Up
CREATE TABLE job_rejections (
    job_id INTEGER PRIMARY KEY REFERENCES jobs(id) ON DELETE CASCADE,
    reason TEXT NOT NULL,
    rejected_at TEXT NOT NULL
);

-- +goose Down
DROP TABLE job_rejections;
