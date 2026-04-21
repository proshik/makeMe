-- name: GetGrammarByLevel :many
SELECT q.id, q.level, q.category, q.prompt, q.options, q.answer,
       COALESCE(c.explanation, '') AS explanation
FROM grammar_questions q
LEFT JOIN grammar_categories c ON c.name = q.category
WHERE q.level = $1
ORDER BY q.id;

-- name: GetGrammarCategoriesByLevel :many
SELECT DISTINCT q.category, COALESCE(c.explanation, '') AS explanation
FROM grammar_questions q
LEFT JOIN grammar_categories c ON c.name = q.category
WHERE q.level = $1
ORDER BY q.category;

-- name: GetGrammarQuestionsByCategory :many
SELECT q.id, q.level, q.category, q.prompt, q.options, q.answer,
       COALESCE(c.explanation, '') AS explanation
FROM grammar_questions q
LEFT JOIN grammar_categories c ON c.name = q.category
WHERE q.category = $1
ORDER BY q.id
LIMIT $2;

-- name: UpsertGrammarQuestion :exec
INSERT INTO grammar_questions (level, category, prompt, options, answer)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (level, prompt) DO UPDATE
SET category = EXCLUDED.category,
    options  = EXCLUDED.options,
    answer   = EXCLUDED.answer;

-- name: UpsertGrammarCategory :exec
INSERT INTO grammar_categories (name, explanation)
VALUES ($1, $2)
ON CONFLICT (name) DO UPDATE
SET explanation = EXCLUDED.explanation;
