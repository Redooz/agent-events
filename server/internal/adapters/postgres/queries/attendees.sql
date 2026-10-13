-- name: UpsertAttendee :exec
INSERT INTO event_attendees (event_id, user_id, created_at)
VALUES ($1, $2, $3)
ON CONFLICT (event_id, user_id) DO NOTHING;

-- name: IsEventAttendee :one
SELECT EXISTS (
    SELECT 1 FROM event_attendees
    WHERE event_id = $1 AND user_id = $2
) AS attended;

-- name: ListEventAttendees :many
SELECT ea.user_id, u.email, ea.created_at
FROM event_attendees ea
JOIN users u ON u.id = ea.user_id
WHERE ea.event_id = $1
ORDER BY ea.created_at;
