-- name: UpsertScore :exec
INSERT INTO scores (user_id, group_id, period, period_key, raw, normalized)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (user_id, group_id, period, period_key)
DO UPDATE SET
    raw        = scores.raw + EXCLUDED.raw,
    normalized = scores.normalized + EXCLUDED.normalized;

-- name: GetLeaderboard :many
SELECT u.id, u.first_name,
       COALESCE(s.raw, 0)::int       AS raw,
       COALESCE(s.normalized, 0.0)   AS normalized
FROM memberships m
JOIN users u ON u.id = m.user_id
LEFT JOIN scores s ON s.user_id = m.user_id
    AND s.group_id = m.group_id
    AND s.period     = $2
    AND s.period_key = $3
WHERE m.group_id = $1
ORDER BY normalized DESC;

-- name: GetUserMonthlyScore :one
SELECT COALESCE(SUM(normalized), 0)::float8 AS normalized
FROM scores
WHERE user_id = $1 AND period = 'month' AND period_key = $2;

-- name: GetUserScoreDays :many
SELECT DISTINCT period_key
FROM scores
WHERE user_id = $1 AND period = 'day'
ORDER BY period_key DESC;
