-- +goose Up
ALTER TABLE assessment_answers ADD COLUMN skipped BOOLEAN NOT NULL DEFAULT FALSE;

-- +goose Down
ALTER TABLE assessment_answers DROP COLUMN skipped;
