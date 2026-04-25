-- name: GetWordProgress :one
SELECT user_id, word_id, en_ru_mc_correct, ru_en_mc_correct,
       en_ru_typing_correct, ru_en_typing_correct,
       learned, learned_at, started_at, updated_at
FROM word_learning_progress
WHERE user_id = $1 AND word_id = $2;

-- name: BulkGetProgressForWords :many
SELECT user_id, word_id, en_ru_mc_correct, ru_en_mc_correct,
       en_ru_typing_correct, ru_en_typing_correct,
       learned, learned_at, started_at, updated_at
FROM word_learning_progress
WHERE user_id = $1 AND word_id = ANY($2::bigint[]);

-- name: IncEnRuMC :exec
INSERT INTO word_learning_progress (user_id, word_id, en_ru_mc_correct, started_at, updated_at)
VALUES ($1, $2, 1, now(), now())
ON CONFLICT (user_id, word_id) DO UPDATE
    SET en_ru_mc_correct = word_learning_progress.en_ru_mc_correct + 1,
        updated_at = now();

-- name: IncRuEnMC :exec
INSERT INTO word_learning_progress (user_id, word_id, ru_en_mc_correct, started_at, updated_at)
VALUES ($1, $2, 1, now(), now())
ON CONFLICT (user_id, word_id) DO UPDATE
    SET ru_en_mc_correct = word_learning_progress.ru_en_mc_correct + 1,
        updated_at = now();

-- name: IncEnRuTyping :exec
INSERT INTO word_learning_progress (user_id, word_id, en_ru_typing_correct, started_at, updated_at)
VALUES ($1, $2, 1, now(), now())
ON CONFLICT (user_id, word_id) DO UPDATE
    SET en_ru_typing_correct = word_learning_progress.en_ru_typing_correct + 1,
        updated_at = now();

-- name: IncRuEnTyping :exec
INSERT INTO word_learning_progress (user_id, word_id, ru_en_typing_correct, started_at, updated_at)
VALUES ($1, $2, 1, now(), now())
ON CONFLICT (user_id, word_id) DO UPDATE
    SET ru_en_typing_correct = word_learning_progress.ru_en_typing_correct + 1,
        updated_at = now();

-- name: ResetEnRuMC :exec
INSERT INTO word_learning_progress (user_id, word_id, en_ru_mc_correct, started_at, updated_at)
VALUES ($1, $2, 0, now(), now())
ON CONFLICT (user_id, word_id) DO UPDATE
    SET en_ru_mc_correct = 0, updated_at = now();

-- name: ResetRuEnMC :exec
INSERT INTO word_learning_progress (user_id, word_id, ru_en_mc_correct, started_at, updated_at)
VALUES ($1, $2, 0, now(), now())
ON CONFLICT (user_id, word_id) DO UPDATE
    SET ru_en_mc_correct = 0, updated_at = now();

-- name: ResetEnRuTyping :exec
INSERT INTO word_learning_progress (user_id, word_id, en_ru_typing_correct, started_at, updated_at)
VALUES ($1, $2, 0, now(), now())
ON CONFLICT (user_id, word_id) DO UPDATE
    SET en_ru_typing_correct = 0, updated_at = now();

-- name: ResetRuEnTyping :exec
INSERT INTO word_learning_progress (user_id, word_id, ru_en_typing_correct, started_at, updated_at)
VALUES ($1, $2, 0, now(), now())
ON CONFLICT (user_id, word_id) DO UPDATE
    SET ru_en_typing_correct = 0, updated_at = now();

-- name: MarkLearned :exec
UPDATE word_learning_progress
SET learned = true, learned_at = now(), updated_at = now()
WHERE user_id = $1 AND word_id = $2;

-- name: MarkAlreadyKnown :exec
INSERT INTO word_learning_progress (
    user_id, word_id,
    en_ru_mc_correct, ru_en_mc_correct,
    en_ru_typing_correct, ru_en_typing_correct,
    learned, learned_at, started_at, updated_at
) VALUES ($1, $2, 2, 2, 2, 2, true, now(), now(), now())
ON CONFLICT (user_id, word_id) DO UPDATE
    SET en_ru_mc_correct     = 2,
        ru_en_mc_correct     = 2,
        en_ru_typing_correct = 2,
        ru_en_typing_correct = 2,
        learned              = true,
        learned_at           = now(),
        updated_at           = now();

-- name: CountInProgressForUser :one
SELECT COUNT(*)::int AS count FROM word_learning_progress
WHERE user_id = $1 AND learned = false;

-- name: CountLearnedViaWordsForUser :one
SELECT COUNT(*)::int AS count FROM word_learning_progress
WHERE user_id = $1 AND learned = true;
