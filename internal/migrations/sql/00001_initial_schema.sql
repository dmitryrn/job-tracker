-- +goose Up
CREATE TABLE companies (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    normalized_name TEXT NOT NULL UNIQUE,
    first_seen_at TEXT NOT NULL,
    last_seen_at TEXT NOT NULL
);

CREATE TABLE jobs (
    id INTEGER PRIMARY KEY,
    source TEXT NOT NULL,
    source_job_id TEXT NOT NULL,
    company_id INTEGER REFERENCES companies(id),
    source_url TEXT NOT NULL,
    title TEXT NOT NULL,
    body_text TEXT NOT NULL,
    location TEXT,
    workplace TEXT NOT NULL,
    employment_type TEXT,
    salary_min INTEGER,
    salary_max INTEGER,
    posted_at TEXT,
    first_seen_at TEXT NOT NULL,
    last_seen_at TEXT NOT NULL,
    metadata_json TEXT NOT NULL,
    UNIQUE(source, source_job_id)
);

-- +goose Down
DROP TABLE jobs;
DROP TABLE companies;
