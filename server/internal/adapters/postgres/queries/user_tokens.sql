-- name: InsertUserToken :exec
INSERT INTO user_tokens (token_hash, user_id, expires_at, created_at)
VALUES ($1, $2, $3, $4);

-- name: GetUserTokenByHash :one
SELECT token_hash, user_id, expires_at, created_at
FROM user_tokens
WHERE token_hash = $1;

-- name: DeleteUserTokenByHash :exec
DELETE FROM user_tokens WHERE token_hash = $1;

-- name: DeleteExpiredUserTokens :exec
DELETE FROM user_tokens WHERE expires_at < $1;
