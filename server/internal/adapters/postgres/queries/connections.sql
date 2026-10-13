-- name: InsertConnection :exec
INSERT INTO connections (id, event_id, user_a_id, user_b_id, method, created_at)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: GetConnectionBetween :one
SELECT id, event_id, user_a_id, user_b_id, method, created_at
FROM connections
WHERE user_a_id = $1 AND user_b_id = $2;

-- name: ListConnectionsForUser :many
SELECT c.id, c.event_id, c.user_a_id, c.user_b_id, c.method, c.created_at,
    e.name AS event_name,
    (CASE WHEN c.user_a_id = $1 THEN c.user_b_id ELSE c.user_a_id END)::uuid AS other_user_id,
    (CASE WHEN c.user_a_id = $1 THEN ub.email ELSE ua.email END)::text AS other_user_email
FROM connections c
JOIN events e ON e.id = c.event_id
JOIN users ua ON ua.id = c.user_a_id
JOIN users ub ON ub.id = c.user_b_id
WHERE c.user_a_id = $1 OR c.user_b_id = $1
ORDER BY c.created_at;
