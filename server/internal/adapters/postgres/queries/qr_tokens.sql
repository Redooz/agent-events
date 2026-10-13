-- name: InsertQRToken :exec
INSERT INTO event_qr_tokens (token_hash, event_id, user_id, expires_at, created_at)
VALUES ($1, $2, $3, $4, $5);

-- name: GetQRTokenByHash :one
SELECT token_hash, event_id, user_id, expires_at, used_at, created_at
FROM event_qr_tokens
WHERE token_hash = $1;

-- name: MarkQRTokenUsed :execrows
UPDATE event_qr_tokens
SET used_at = $2
WHERE token_hash = $1 AND used_at IS NULL;

-- name: DeleteExpiredQRTokens :exec
DELETE FROM event_qr_tokens WHERE expires_at < $1;
