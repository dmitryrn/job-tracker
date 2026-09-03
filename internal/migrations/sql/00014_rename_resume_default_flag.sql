-- +goose Up
ALTER TABLE resumes RENAME COLUMN is_default TO base_resume;
DROP INDEX resumes_one_default;
CREATE UNIQUE INDEX resumes_one_base ON resumes (base_resume) WHERE base_resume = 1;

-- +goose Down
DROP INDEX resumes_one_base;
ALTER TABLE resumes RENAME COLUMN base_resume TO is_default;
CREATE UNIQUE INDEX resumes_one_default ON resumes (is_default) WHERE is_default = 1;
