-- name: InsertScan :exec
INSERT INTO connection_scans (event_id, scanner_id, scanned_id, created_at)
VALUES ($1, $2, $3, $4)
ON CONFLICT (event_id, scanner_id, scanned_id) DO NOTHING;

-- name: ScanExists :one
SELECT EXISTS (
    SELECT 1 FROM connection_scans
    WHERE event_id = $1 AND scanner_id = $2 AND scanned_id = $3
) AS exists;
