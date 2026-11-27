-- name: CreateUser :one
INSERT INTO users (telegram_id, username, first_name)
VALUES ($1, $2, $3)
ON CONFLICT (telegram_id) DO UPDATE
    SET username   = EXCLUDED.username,
        first_name = EXCLUDED.first_name,
        updated_at = now()
RETURNING *;

-- name: GetUserByTelegramID :one
SELECT * FROM users WHERE telegram_id = $1;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1;

-- name: UpdateUserLevel :exec
UPDATE users SET level = $2, onboarded = true, updated_at = now() WHERE id = $1;

-- name: ListAllUsers :many
SELECT id, telegram_id, username, first_name, level, onboarded, created_at, updated_at, show_rule_on_correct FROM users ORDER BY id;

-- name: UpdateUserShowRuleOnCorrect :exec
UPDATE users SET show_rule_on_correct = $2, updated_at = now() WHERE id = $1;
