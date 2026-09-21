-- +goose Up
CREATE TABLE applications (
    job_id INTEGER PRIMARY KEY REFERENCES jobs(id) ON DELETE CASCADE,
    applied_at TEXT NOT NULL
);

-- +goose Down
DROP TABLE applications;
