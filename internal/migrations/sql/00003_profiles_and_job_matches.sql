-- +goose Up
CREATE TABLE user_profiles (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    name TEXT NOT NULL,
    headline TEXT NOT NULL,
    location TEXT NOT NULL,
    work_authorization TEXT NOT NULL,
    summary TEXT NOT NULL,
    skills_json TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE job_matches (
    job_id INTEGER PRIMARY KEY REFERENCES jobs(id) ON DELETE CASCADE,
    content TEXT NOT NULL,
    created_at TEXT NOT NULL
);

-- +goose Down
DROP TABLE job_matches;
DROP TABLE user_profiles;
