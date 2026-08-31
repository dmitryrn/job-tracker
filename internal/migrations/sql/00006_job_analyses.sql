-- +goose Up
CREATE TABLE job_analyses (
    job_id INTEGER PRIMARY KEY REFERENCES jobs(id) ON DELETE CASCADE,
    analyzer_version TEXT NOT NULL,
    prompt_version TEXT NOT NULL,
    input_sha256 TEXT NOT NULL,
    model TEXT NOT NULL,
    analyzed_at TEXT NOT NULL,
    normalized_description TEXT NOT NULL,
    analysis_json TEXT NOT NULL
);

-- +goose Down
DROP TABLE job_analyses;
