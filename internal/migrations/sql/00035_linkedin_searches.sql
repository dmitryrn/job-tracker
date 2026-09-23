-- +goose Up
CREATE TABLE linkedin_searches (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    enabled INTEGER NOT NULL,
    sort_order INTEGER NOT NULL UNIQUE,
    query TEXT NOT NULL,
    location TEXT NOT NULL,
    posted_within TEXT NOT NULL,
    workplace TEXT NOT NULL,
    experience_level TEXT NOT NULL,
    result_limit INTEGER NOT NULL
);

INSERT INTO linkedin_searches (
    id,
    name,
    enabled,
    sort_order,
    query,
    location,
    posted_within,
    workplace,
    experience_level,
    result_limit
)
SELECT
    1,
    'Europe',
    linkedin_enabled,
    1,
    linkedin_query,
    linkedin_location,
    linkedin_posted_within,
    linkedin_workplace,
    linkedin_experience_level,
    linkedin_limit
FROM discovery_settings;

INSERT INTO linkedin_searches (
    id,
    name,
    enabled,
    sort_order,
    query,
    location,
    posted_within,
    workplace,
    experience_level,
    result_limit
)
SELECT
    2,
    'Serbia',
    linkedin_enabled,
    2,
    linkedin_query,
    'Serbia',
    linkedin_posted_within,
    linkedin_workplace,
    linkedin_experience_level,
    linkedin_limit
FROM discovery_settings;

-- +goose Down
DROP TABLE linkedin_searches;
