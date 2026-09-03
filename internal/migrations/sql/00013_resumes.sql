-- +goose Up
CREATE TABLE resumes (
    id INTEGER PRIMARY KEY,
    title TEXT NOT NULL,
    is_default INTEGER NOT NULL DEFAULT 0 CHECK (is_default IN (0, 1)),
    full_name TEXT NOT NULL,
    headline TEXT NOT NULL,
    location TEXT NOT NULL DEFAULT '',
    email TEXT NOT NULL DEFAULT '',
    phone TEXT NOT NULL DEFAULT '',
    summary TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE UNIQUE INDEX resumes_one_default ON resumes (is_default) WHERE is_default = 1;

CREATE TABLE resume_links (
    id INTEGER PRIMARY KEY,
    resume_id INTEGER NOT NULL REFERENCES resumes(id) ON DELETE CASCADE,
    label TEXT NOT NULL,
    url TEXT NOT NULL,
    sort_order INTEGER NOT NULL
);

CREATE TABLE resume_skills (
    id INTEGER PRIMARY KEY,
    resume_id INTEGER NOT NULL REFERENCES resumes(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    level TEXT NOT NULL DEFAULT '',
    sort_order INTEGER NOT NULL
);

CREATE TABLE resume_competencies (
    id INTEGER PRIMARY KEY,
    resume_id INTEGER NOT NULL REFERENCES resumes(id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    sort_order INTEGER NOT NULL
);

CREATE TABLE resume_competency_bullets (
    id INTEGER PRIMARY KEY,
    competency_id INTEGER NOT NULL REFERENCES resume_competencies(id) ON DELETE CASCADE,
    content TEXT NOT NULL,
    sort_order INTEGER NOT NULL
);

CREATE TABLE resume_experience (
    id INTEGER PRIMARY KEY,
    resume_id INTEGER NOT NULL REFERENCES resumes(id) ON DELETE CASCADE,
    company TEXT NOT NULL,
    title TEXT NOT NULL,
    location TEXT NOT NULL DEFAULT '',
    start_date TEXT NOT NULL,
    end_date TEXT NOT NULL DEFAULT '',
    is_current INTEGER NOT NULL DEFAULT 0 CHECK (is_current IN (0, 1)),
    stack TEXT NOT NULL DEFAULT '',
    sort_order INTEGER NOT NULL
);

CREATE TABLE resume_experience_bullets (
    id INTEGER PRIMARY KEY,
    experience_id INTEGER NOT NULL REFERENCES resume_experience(id) ON DELETE CASCADE,
    content TEXT NOT NULL,
    sort_order INTEGER NOT NULL
);

CREATE TABLE resume_education (
    id INTEGER PRIMARY KEY,
    resume_id INTEGER NOT NULL REFERENCES resumes(id) ON DELETE CASCADE,
    institution TEXT NOT NULL,
    location TEXT NOT NULL DEFAULT '',
    degree TEXT NOT NULL,
    field_of_study TEXT NOT NULL DEFAULT '',
    start_date TEXT NOT NULL DEFAULT '',
    end_date TEXT NOT NULL DEFAULT '',
    details TEXT NOT NULL DEFAULT '',
    sort_order INTEGER NOT NULL
);

-- +goose Down
DROP TABLE resume_education;
DROP TABLE resume_experience_bullets;
DROP TABLE resume_experience;
DROP TABLE resume_competency_bullets;
DROP TABLE resume_competencies;
DROP TABLE resume_skills;
DROP TABLE resume_links;
DROP INDEX resumes_one_default;
DROP TABLE resumes;
