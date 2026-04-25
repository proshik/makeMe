-- name: CreateWordsSession :one
INSERT INTO words_sessions (user_id, started_on, word_ids, created_at)
VALUES ($1, $2, $3, now())
RETURNING id, user_id, started_on, word_ids, last_word_id, created_at, completed_at, error_counts, deferred_word_ids;

-- name: GetOpenWordsSession :one
SELECT id, user_id, started_on, word_ids, last_word_id, created_at, completed_at, error_counts, deferred_word_ids
FROM words_sessions
WHERE user_id = $1 AND completed_at IS NULL
ORDER BY id DESC
LIMIT 1;

-- name: GetWordsSessionByID :one
SELECT id, user_id, started_on, word_ids, last_word_id, created_at, completed_at, error_counts, deferred_word_ids
FROM words_sessions
WHERE id = $1;

-- name: UpdateLastWord :exec
UPDATE words_sessions SET last_word_id = $2 WHERE id = $1;

-- name: CompleteWordsSession :exec
UPDATE words_sessions SET completed_at = now() WHERE id = $1 AND completed_at IS NULL;

-- name: IncSessionWordError :exec
UPDATE words_sessions
SET error_counts = jsonb_set(
    error_counts,
    ARRAY[$2::text],
    to_jsonb(COALESCE((error_counts->>($2::text))::int, 0) + 1)
)
WHERE id = $1;

-- name: GetSessionWordError :one
SELECT COALESCE((error_counts->>($2::text))::int, 0)::int AS count
FROM words_sessions
WHERE id = $1;

-- name: DeferSessionWord :exec
UPDATE words_sessions
SET deferred_word_ids = array_append(deferred_word_ids, $2::bigint)
WHERE id = $1 AND NOT ($2::bigint = ANY(deferred_word_ids));
