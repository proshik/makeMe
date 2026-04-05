-- name: GetWordsByLevel :many
SELECT * FROM words WHERE level = $1 ORDER BY random() LIMIT $2;

-- name: GetWordByID :one
SELECT * FROM words WHERE id = $1;

-- name: GetNewWordsForUser :many
SELECT w.* FROM words w
LEFT JOIN review_cards rc ON rc.word_id = w.id AND rc.user_id = $1
WHERE w.level = $2 AND rc.id IS NULL
ORDER BY random()
LIMIT $3;

-- name: GetDueCards :many
SELECT * FROM review_cards
WHERE user_id = $1 AND next_review_date <= CURRENT_DATE
ORDER BY next_review_date
LIMIT $2;

-- name: UpsertReviewCard :exec
INSERT INTO review_cards (user_id, word_id, ease_factor, interval_days, repetitions, next_review_date, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, now())
ON CONFLICT (user_id, word_id) DO UPDATE
    SET ease_factor = EXCLUDED.ease_factor,
        interval_days = EXCLUDED.interval_days,
        repetitions = EXCLUDED.repetitions,
        next_review_date = EXCLUDED.next_review_date,
        updated_at = now();
