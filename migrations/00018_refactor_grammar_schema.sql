-- +goose Up

-- 1. Create grammar_categories table (one row per category name)
CREATE TABLE grammar_categories (
    name        TEXT PRIMARY KEY,
    explanation TEXT NOT NULL DEFAULT ''
);

-- 2. Deduplicate grammar_questions before adding UNIQUE constraint
--    (keep lowest id when level+prompt duplicate exists)
DELETE FROM grammar_questions
WHERE id NOT IN (
    SELECT MIN(id) FROM grammar_questions GROUP BY level, prompt
);

-- 3. Add UNIQUE constraint to support upsert
ALTER TABLE grammar_questions
    ADD CONSTRAINT grammar_questions_level_prompt_key UNIQUE (level, prompt);

-- 4. Move explanation out of grammar_questions (now lives in grammar_categories)
ALTER TABLE grammar_questions DROP COLUMN explanation;

-- +goose Down
ALTER TABLE grammar_questions ADD COLUMN explanation TEXT NOT NULL DEFAULT '';
ALTER TABLE grammar_questions DROP CONSTRAINT grammar_questions_level_prompt_key;
DROP TABLE grammar_categories;
