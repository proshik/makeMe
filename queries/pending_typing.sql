-- name: UpsertPendingTyping :exec
INSERT INTO pending_typing_prompts (user_id, chat_id, source, payload, expires_at, created_at)
VALUES ($1, $2, $3, $4, $5, now())
ON CONFLICT (user_id) DO UPDATE
    SET chat_id    = EXCLUDED.chat_id,
        source     = EXCLUDED.source,
        payload    = EXCLUDED.payload,
        expires_at = EXCLUDED.expires_at,
        created_at = now();

-- name: GetPendingTyping :one
SELECT user_id, chat_id, source, payload, expires_at, created_at
FROM pending_typing_prompts
WHERE user_id = $1;

-- name: DeletePendingTyping :exec
DELETE FROM pending_typing_prompts WHERE user_id = $1;
