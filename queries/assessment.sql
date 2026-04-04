-- name: CreateAssessmentSession :one
INSERT INTO assessment_sessions (user_id)
VALUES ($1)
RETURNING *;

-- name: GetActiveAssessmentSession :one
SELECT * FROM assessment_sessions
WHERE user_id = $1 AND status = 'in_progress'
ORDER BY created_at DESC
LIMIT 1;

-- name: UpdateAssessmentSession :exec
UPDATE assessment_sessions
SET status = $2, current_level = $3, determined_level = $4, completed_at = $5
WHERE id = $1;

-- name: AddAssessmentAnswer :exec
INSERT INTO assessment_answers (session_id, question_level, question_text, options, correct_option, user_option, correct)
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: CountAssessmentAnswers :one
SELECT COUNT(*) FROM assessment_answers WHERE session_id = $1;

-- name: CountCorrectByLevel :many
SELECT question_level,
       COUNT(*) AS total,
       COUNT(*) FILTER (WHERE correct) AS correct_count
FROM assessment_answers
WHERE session_id = $1
GROUP BY question_level;
