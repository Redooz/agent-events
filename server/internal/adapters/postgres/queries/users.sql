-- name: UpsertUserIdentity :one
INSERT INTO users (id, provider, subject, email, created_at)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (provider, subject) DO UPDATE SET email = EXCLUDED.email
RETURNING id, provider, subject, email, created_at;

-- name: GetUserByID :one
SELECT id, provider, subject, email, created_at
FROM users
WHERE id = $1;
