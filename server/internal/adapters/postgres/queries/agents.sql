-- name: LockUserForUpdate :one
SELECT id FROM users WHERE id = $1 FOR UPDATE;

-- name: CountActiveAgentsByUser :one
SELECT COUNT(*) FROM agents WHERE user_id = $1 AND revoked_at IS NULL;

-- name: InsertAgent :exec
INSERT INTO agents (id, user_id, name, key_hash, created_at, revoked_at, last_used_at)
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: GetAgentByID :one
SELECT id, user_id, name, key_hash, created_at, revoked_at, last_used_at
FROM agents
WHERE id = $1;

-- name: GetAgentByKeyHash :one
SELECT id, user_id, name, key_hash, created_at, revoked_at, last_used_at
FROM agents
WHERE key_hash = $1;

-- name: ListAgentsByUser :many
SELECT id, user_id, name, key_hash, created_at, revoked_at, last_used_at
FROM agents
WHERE user_id = $1
ORDER BY created_at;

-- name: RevokeAgent :execrows
UPDATE agents
SET revoked_at = $2
WHERE id = $1 AND revoked_at IS NULL;

-- name: TouchAgentLastUsed :exec
UPDATE agents SET last_used_at = $2 WHERE id = $1;
