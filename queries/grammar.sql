-- name: GetGrammarByLevel :many
SELECT id, level, category, prompt, options, answer, explanation
FROM grammar_questions
WHERE level = $1
ORDER BY id;
