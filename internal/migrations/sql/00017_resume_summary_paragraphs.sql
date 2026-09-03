-- +goose Up
CREATE TABLE resume_summary_paragraphs (
    id INTEGER PRIMARY KEY,
    resume_id INTEGER NOT NULL REFERENCES resumes(id) ON DELETE CASCADE,
    content TEXT NOT NULL,
    sort_order INTEGER NOT NULL
);

WITH RECURSIVE paragraphs(resume_id, content, remainder, sort_order) AS (
    SELECT id,
        TRIM(SUBSTR(REPLACE(summary, CHAR(13), ''), 1, INSTR(REPLACE(summary, CHAR(13), '') || CHAR(10) || CHAR(10), CHAR(10) || CHAR(10)) - 1)),
        SUBSTR(REPLACE(summary, CHAR(13), '') || CHAR(10) || CHAR(10), INSTR(REPLACE(summary, CHAR(13), '') || CHAR(10) || CHAR(10), CHAR(10) || CHAR(10)) + 2),
        0
    FROM resumes
    WHERE summary <> ''
    UNION ALL
    SELECT resume_id,
        TRIM(SUBSTR(remainder, 1, INSTR(remainder, CHAR(10) || CHAR(10)) - 1)),
        SUBSTR(remainder, INSTR(remainder, CHAR(10) || CHAR(10)) + 2),
        sort_order + 1
    FROM paragraphs
    WHERE remainder <> ''
)
INSERT INTO resume_summary_paragraphs (resume_id, content, sort_order)
SELECT resume_id, content, sort_order FROM paragraphs WHERE content <> '';

ALTER TABLE resumes DROP COLUMN summary;

-- +goose Down
ALTER TABLE resumes ADD COLUMN summary TEXT NOT NULL DEFAULT '';

UPDATE resumes AS resume
SET summary = COALESCE((
    SELECT GROUP_CONCAT(content, CHAR(10) || CHAR(10))
    FROM (
        SELECT content FROM resume_summary_paragraphs
        WHERE resume_id = resume.id
        ORDER BY sort_order, id
    )
), '');

DROP TABLE resume_summary_paragraphs;
