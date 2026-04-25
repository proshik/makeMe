-- name: UpsertWord :exec
INSERT INTO words (english, russian, level, example, transcription)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (english, level) DO UPDATE
    SET russian = EXCLUDED.russian,
        example = EXCLUDED.example,
        transcription = EXCLUDED.transcription;

-- name: UpdateWordAudio :exec
UPDATE words SET transcription = $2, audio_url = $3 WHERE id = $1;

-- name: GetWordsByLevel :many
SELECT id, english, russian, level, example, transcription, audio_url FROM words WHERE level = $1 ORDER BY random() LIMIT $2;

-- name: GetWordByID :one
SELECT id, english, russian, level, example, transcription, audio_url FROM words WHERE id = $1;

-- name: GetNewWordsForUser :many
SELECT w.id, w.english, w.russian, w.level, w.example, w.transcription, w.audio_url FROM words w
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

-- name: CountLearnedWords :one
SELECT COUNT(*)::int AS count FROM review_cards
WHERE user_id = $1 AND repetitions >= 3;

-- name: GetInProgressWordsByLevel :many
SELECT w.id, w.english, w.russian, w.level, w.example, w.transcription, w.audio_url
FROM words w
JOIN word_learning_progress wlp ON wlp.word_id = w.id AND wlp.user_id = $1
WHERE w.level = $2 AND wlp.learned = false
ORDER BY wlp.updated_at DESC
LIMIT $3;

-- name: GetNewWordsExcludingProgress :many
SELECT w.id, w.english, w.russian, w.level, w.example, w.transcription, w.audio_url
FROM words w
LEFT JOIN word_learning_progress wlp ON wlp.word_id = w.id AND wlp.user_id = $1
LEFT JOIN review_cards rc ON rc.word_id = w.id AND rc.user_id = $1
WHERE w.level = $2 AND wlp.user_id IS NULL AND rc.id IS NULL
ORDER BY md5($1::text || w.id::text)
LIMIT $3;

-- name: GetReviewCardByUserWord :one
SELECT id, user_id, word_id, ease_factor, interval_days, repetitions, next_review_date, updated_at
FROM review_cards
WHERE user_id = $1 AND word_id = $2;
