-- +goose Up
ALTER TABLE jobs ADD COLUMN profile_match_score INTEGER CHECK (profile_match_score BETWEEN 0 AND 10);

-- +goose Down
ALTER TABLE jobs DROP COLUMN profile_match_score;
