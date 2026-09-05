-- +goose Up
CREATE TABLE app_events_next (
    id INTEGER PRIMARY KEY,
    occurred_at TEXT NOT NULL,
    provider TEXT NOT NULL DEFAULT '',
    run_id TEXT NOT NULL DEFAULT '',
    type TEXT NOT NULL,
    level TEXT NOT NULL,
    message TEXT NOT NULL,
    data_json TEXT NOT NULL DEFAULT '{}'
);
INSERT INTO app_events_next (id, occurred_at, provider, run_id, type, level, message, data_json)
SELECT id, occurred_at, provider, run_id, type, level, message, data_json FROM app_events;
DROP TABLE app_events;
ALTER TABLE app_events_next RENAME TO app_events;
CREATE INDEX app_events_provider_occurred_at ON app_events (provider, occurred_at DESC, id DESC);
CREATE INDEX app_events_run_id ON app_events (run_id, id DESC);

-- +goose Down
CREATE TABLE app_events_previous (
    id INTEGER PRIMARY KEY,
    occurred_at TEXT NOT NULL,
    provider TEXT NOT NULL DEFAULT '',
    run_id TEXT NOT NULL DEFAULT '',
    type TEXT NOT NULL,
    level TEXT NOT NULL CHECK (level IN ('info', 'error')),
    message TEXT NOT NULL,
    data_json TEXT NOT NULL DEFAULT '{}'
);
INSERT INTO app_events_previous (id, occurred_at, provider, run_id, type, level, message, data_json)
SELECT id, occurred_at, provider, run_id, type, CASE level WHEN 'warn' THEN 'info' ELSE level END, message, data_json FROM app_events;
DROP TABLE app_events;
ALTER TABLE app_events_previous RENAME TO app_events;
CREATE INDEX app_events_provider_occurred_at ON app_events (provider, occurred_at DESC, id DESC);
CREATE INDEX app_events_run_id ON app_events (run_id, id DESC);
