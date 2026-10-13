-- name: InsertConnectionRequest :exec
INSERT INTO connection_requests (id, event_id, from_user_id, to_user_id, status, created_at)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: GetConnectionRequestByID :one
SELECT id, event_id, from_user_id, to_user_id, status, created_at, responded_at
FROM connection_requests
WHERE id = $1;

-- name: FindPairConnectionRequests :many
SELECT id, event_id, from_user_id, to_user_id, status, created_at, responded_at
FROM connection_requests
WHERE event_id = $1
  AND ((from_user_id = $2 AND to_user_id = $3) OR (from_user_id = $3 AND to_user_id = $2));

-- name: ListIncomingConnectionRequests :many
SELECT cr.id, cr.event_id, cr.from_user_id, cr.to_user_id, cr.status, cr.created_at, cr.responded_at,
    e.name AS event_name, uf.email AS from_user_email
FROM connection_requests cr
JOIN events e ON e.id = cr.event_id
JOIN users uf ON uf.id = cr.from_user_id
WHERE cr.to_user_id = $1 AND cr.status = 'pending'
ORDER BY cr.created_at;

-- name: AcceptConnectionRequest :execrows
UPDATE connection_requests
SET status = 'accepted', responded_at = $2
WHERE id = $1 AND status = 'pending';

-- name: DeclineConnectionRequest :execrows
UPDATE connection_requests
SET status = 'declined', responded_at = $2
WHERE id = $1 AND status = 'pending';
