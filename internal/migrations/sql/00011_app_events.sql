-- +goose Up
CREATE TABLE app_events (
    id INTEGER PRIMARY KEY,
    occurred_at TEXT NOT NULL,
    provider TEXT NOT NULL DEFAULT '',
    run_id TEXT NOT NULL DEFAULT '',
    type TEXT NOT NULL,
    level TEXT NOT NULL CHECK (level IN ('info', 'error')),
    message TEXT NOT NULL,
    data_json TEXT NOT NULL DEFAULT '{}'
);
CREATE INDEX app_events_provider_occurred_at ON app_events (provider, occurred_at DESC, id DESC);
CREATE INDEX app_events_run_id ON app_events (run_id, id DESC);

-- +goose Down
DROP TABLE app_events;
