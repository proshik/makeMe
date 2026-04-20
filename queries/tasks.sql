-- name: CreateDailySet :one
INSERT INTO daily_sets (user_id, group_id, date, round)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetLatestDailySet :one
SELECT * FROM daily_sets
WHERE user_id = $1 AND group_id = $2 AND date = $3
ORDER BY round DESC
LIMIT 1;

-- name: AddTask :one
INSERT INTO tasks (daily_set_id, type, level, prompt, options, answer, metadata, position)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetTasksByDailySet :many
SELECT * FROM tasks WHERE daily_set_id = $1 ORDER BY position;

-- name: GetTaskByID :one
SELECT t.id, t.daily_set_id, t.type, t.level, t.prompt,
       t.options, t.answer, t.metadata, t.position,
       ds.group_id
FROM tasks t
JOIN daily_sets ds ON ds.id = t.daily_set_id
WHERE t.id = $1;

-- name: AddTaskResult :one
INSERT INTO task_results (task_id, user_id, response, correct, score)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetTaskResult :one
SELECT * FROM task_results WHERE task_id = $1 AND user_id = $2;

-- name: CountTaskResults :one
SELECT
    (SELECT COUNT(*) FROM task_results tr
     JOIN tasks t ON t.id = tr.task_id
     WHERE t.daily_set_id = $1 AND tr.user_id = $2) AS answered,
    (SELECT COUNT(*) FROM tasks WHERE daily_set_id = $1) AS total;

-- name: CountCorrectTaskResults :one
SELECT COUNT(*) FROM task_results tr
JOIN tasks t ON t.id = tr.task_id
WHERE t.daily_set_id = $1 AND tr.user_id = $2 AND tr.correct = true;
