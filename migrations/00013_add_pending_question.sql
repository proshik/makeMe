-- +goose Up
ALTER TABLE assessment_sessions ADD COLUMN pending_question JSONB;

-- +goose Down
ALTER TABLE assessment_sessions DROP COLUMN pending_question;
