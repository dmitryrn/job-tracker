-- +goose Up
CREATE TABLE provider_runs (
    provider TEXT PRIMARY KEY,
    last_run_at TEXT NOT NULL,
    last_completed_at TEXT,
    status TEXT NOT NULL CHECK (status IN ('running', 'succeeded', 'failed')),
    last_error TEXT
);

-- +goose Down
DROP TABLE provider_runs;
