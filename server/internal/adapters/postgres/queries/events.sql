-- name: InsertEvent :exec
INSERT INTO events (id, user_id, name, description, mode, date, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8);

-- name: GetEventByID :one
SELECT id, user_id, name, description, mode, date, created_at, updated_at
FROM events
WHERE id = $1;

-- name: ListEvents :many
SELECT id, user_id, name, description, mode, date, created_at, updated_at
FROM events
ORDER BY created_at;

-- name: UpdateEvent :execrows
UPDATE events
SET name = $2, description = $3, mode = $4, updated_at = $5
WHERE id = $1;

-- name: DeleteEvent :execrows
DELETE FROM events WHERE id = $1;
