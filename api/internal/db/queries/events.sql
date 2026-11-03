-- name: CreateEvent :one
INSERT INTO events (
    organizer_id, title, category, description, starts_at, city, address,
    lat, lng, location, hide_exact_location, participant_limit
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, 
    ST_SetSRID(ST_MakePoint($9, $8), 4326)::geography, -- $8=lat, $9=lng
    $10, $11
) RETURNING *;

-- name: GetEventByID :one
SELECT * FROM events WHERE id = $1;

-- name: CountActiveEventsByOrganizer :one
SELECT COUNT(*) FROM events 
WHERE organizer_id = $1 AND status = 'active';

-- name: GetEventsFeed :many
SELECT e.*, u.username AS organizer_name, u.avatar_url AS organizer_avatar
FROM events e
JOIN users u ON u.id = e.organizer_id
WHERE e.status = 'active' 
  AND e.starts_at >= now()
  AND (sqlc.narg('category')::text IS NULL OR e.category = sqlc.narg('category'))
  AND (sqlc.narg('city')::text IS NULL OR e.city = sqlc.narg('city'))
  AND (sqlc.narg('date_from')::timestamptz IS NULL OR e.starts_at >= sqlc.narg('date_from'))
  AND (sqlc.narg('date_to')::timestamptz IS NULL OR e.starts_at <= sqlc.narg('date_to'))
  AND (sqlc.narg('cursor_time')::timestamptz IS NULL OR e.starts_at > sqlc.narg('cursor_time'))
ORDER BY e.starts_at ASC
LIMIT $1;

-- name: GetEventsArchive :many
SELECT e.*, u.username AS organizer_name, u.avatar_url AS organizer_avatar
FROM events e
JOIN users u ON u.id = e.organizer_id
WHERE (e.status = 'completed' OR e.status = 'cancelled' OR e.starts_at < now())
  AND (sqlc.narg('cursor_time')::timestamptz IS NULL OR e.starts_at < sqlc.narg('cursor_time'))
ORDER BY e.starts_at DESC
LIMIT $1;

-- name: CancelEvent :one
UPDATE events
SET status = 'cancelled', cancel_reason = $2, updated_at = now()
WHERE id = $1 AND organizer_id = $3 AND status = 'active'
RETURNING *;