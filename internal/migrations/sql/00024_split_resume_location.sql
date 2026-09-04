-- +goose Up
ALTER TABLE resumes ADD COLUMN town TEXT NOT NULL DEFAULT '';
ALTER TABLE resumes ADD COLUMN country TEXT NOT NULL DEFAULT '';

UPDATE resumes
SET town = CASE WHEN instr(location, ',') > 0 THEN trim(substr(location, 1, instr(location, ',') - 1)) ELSE trim(location) END,
    country = CASE WHEN instr(location, ',') > 0 THEN trim(substr(location, instr(location, ',') + 1)) ELSE '' END;

ALTER TABLE resumes DROP COLUMN location;

-- +goose Down
ALTER TABLE resumes ADD COLUMN location TEXT NOT NULL DEFAULT '';

UPDATE resumes
SET location = CASE
    WHEN town != '' AND country != '' THEN town || ', ' || country
    WHEN town != '' THEN town
    ELSE country
END;

ALTER TABLE resumes DROP COLUMN country;
ALTER TABLE resumes DROP COLUMN town;
