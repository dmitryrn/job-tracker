-- +goose Up
CREATE TABLE provider_runs_next (
    provider TEXT PRIMARY KEY,
    last_run_at TEXT NOT NULL
);
INSERT INTO provider_runs_next (provider, last_run_at)
SELECT provider, last_run_at FROM provider_runs;
DROP TABLE provider_runs;
ALTER TABLE provider_runs_next RENAME TO provider_runs;

-- +goose Down
CREATE TABLE provider_runs_previous (
    provider TEXT PRIMARY KEY,
    last_run_at TEXT NOT NULL,
    last_completed_at TEXT,
    status TEXT NOT NULL CHECK (status IN ('running', 'succeeded', 'failed')),
    last_error TEXT
);
INSERT INTO provider_runs_previous (provider, last_run_at, last_completed_at, status, last_error)
SELECT provider, last_run_at, NULL, 'succeeded', NULL FROM provider_runs;
DROP TABLE provider_runs;
ALTER TABLE provider_runs_previous RENAME TO provider_runs;
