-- +goose Up
-- One-off data cleanup: remove grammar_questions whose prompts were rewritten
-- in the JSON seed to fix ambiguity. The seed upsert key is (level, prompt),
-- so without an explicit DELETE these rows would remain as orphans in
-- production after the seed runs with the new prompts.

DELETE FROM grammar_questions WHERE level = 'B1' AND prompt = 'He ___ finished by now — it was such a simple task.';
DELETE FROM grammar_questions WHERE level = 'C1' AND prompt = 'I ___ have helped you move, but I was away that weekend.';
DELETE FROM grammar_questions WHERE level = 'A2' AND prompt = 'They ___ in a small village for ten years.';
DELETE FROM grammar_questions WHERE level = 'A2' AND prompt = 'Have you ___ been to New York?';

-- +goose Down
-- Intentionally empty: this is a one-off data cleanup and is not reversible
-- from SQL alone (the prompts would need to be reinserted from a snapshot).
SELECT 1;
