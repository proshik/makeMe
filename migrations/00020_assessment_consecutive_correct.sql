-- +goose Up
ALTER TABLE assessment_sessions ADD COLUMN consecutive_correct INT NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE assessment_sessions DROP COLUMN consecutive_correct;
