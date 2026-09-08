-- +goose Up
DROP TABLE resume_competency_bullets;
DROP TABLE resume_competencies;

-- +goose Down
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
