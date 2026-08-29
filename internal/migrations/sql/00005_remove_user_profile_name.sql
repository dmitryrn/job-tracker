-- +goose Up
CREATE TABLE user_profiles_without_name (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    headline TEXT NOT NULL,
    location TEXT NOT NULL,
    work_authorization TEXT NOT NULL,
    summary TEXT NOT NULL,
    skills_json TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

INSERT INTO user_profiles_without_name (id, headline, location, work_authorization, summary, skills_json, updated_at)
SELECT id, headline, location, work_authorization, summary, skills_json, updated_at FROM user_profiles;

DROP TABLE user_profiles;
ALTER TABLE user_profiles_without_name RENAME TO user_profiles;

-- +goose Down
CREATE TABLE user_profiles_with_name (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    name TEXT NOT NULL,
    headline TEXT NOT NULL,
    location TEXT NOT NULL,
    work_authorization TEXT NOT NULL,
    summary TEXT NOT NULL,
    skills_json TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

INSERT INTO user_profiles_with_name (id, name, headline, location, work_authorization, summary, skills_json, updated_at)
SELECT id, '', headline, location, work_authorization, summary, skills_json, updated_at FROM user_profiles;

DROP TABLE user_profiles;
ALTER TABLE user_profiles_with_name RENAME TO user_profiles;
