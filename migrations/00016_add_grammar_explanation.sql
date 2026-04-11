-- +goose Up
ALTER TABLE grammar_questions ADD COLUMN explanation TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE grammar_questions DROP COLUMN explanation;
