-- +goose Up
ALTER TABLE grammar_questions
    ADD COLUMN subcategory TEXT NOT NULL DEFAULT 'general';

CREATE INDEX IF NOT EXISTS grammar_questions_category_subcategory_idx
    ON grammar_questions (category, subcategory);

-- +goose Down
DROP INDEX IF EXISTS grammar_questions_category_subcategory_idx;
ALTER TABLE grammar_questions DROP COLUMN subcategory;
