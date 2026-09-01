-- +goose Up
CREATE TABLE discovery_settings (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    adzuna_query TEXT NOT NULL,
    adzuna_country TEXT NOT NULL,
    adzuna_max_days_old INTEGER NOT NULL,
    adzuna_max_pages INTEGER NOT NULL,
    adzuna_results_per_page INTEGER NOT NULL,
    adzuna_workplace TEXT NOT NULL,
    remotive_query TEXT NOT NULL,
    remotive_category TEXT NOT NULL,
    jobicy_count INTEGER NOT NULL,
    jobicy_geo TEXT NOT NULL,
    jobicy_industry TEXT NOT NULL,
    jobicy_tag TEXT NOT NULL
);

INSERT INTO discovery_settings (
    id, adzuna_query, adzuna_country, adzuna_max_days_old, adzuna_max_pages, adzuna_results_per_page, adzuna_workplace,
    remotive_query, remotive_category, jobicy_count, jobicy_geo, jobicy_industry, jobicy_tag
) VALUES (1, 'software engineer', 'de', 30, 5, 50, 'remote-hybrid', 'software engineer', 'software-development', 50, 'europe', 'engineering', '');

-- +goose Down
DROP TABLE discovery_settings;
