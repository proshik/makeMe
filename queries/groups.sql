-- name: CreateGroup :one
INSERT INTO groups (name, invite_code, created_by)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetGroupByInviteCode :one
SELECT * FROM groups WHERE invite_code = $1;

-- name: GetGroupByID :one
SELECT * FROM groups WHERE id = $1;

-- name: AddMembership :exec
INSERT INTO memberships (group_id, user_id, role)
VALUES ($1, $2, $3)
ON CONFLICT (group_id, user_id) DO NOTHING;

-- name: IsMember :one
SELECT EXISTS(
    SELECT 1 FROM memberships WHERE group_id = $1 AND user_id = $2
) AS is_member;

-- name: ListGroupsByUser :many
SELECT g.* FROM groups g
JOIN memberships m ON m.group_id = g.id
WHERE m.user_id = $1
ORDER BY g.created_at DESC;
