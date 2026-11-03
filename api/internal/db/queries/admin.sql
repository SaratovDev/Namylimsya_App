-- name: AdminDashboardUsers :one
SELECT 
    COUNT(*) AS total,
    COUNT(*) FILTER (WHERE created_at >= now() - interval '7 days') AS new_7d,
    COUNT(*) FILTER (WHERE is_banned = true) AS banned
FROM users;

-- name: AdminDashboardEvents :one
SELECT 
    COUNT(*) AS total,
    COUNT(*) FILTER (WHERE status = 'active') AS active,
    COUNT(*) FILTER (WHERE status = 'cancelled') AS cancelled,
    COUNT(*) FILTER (WHERE starts_at >= now() AND status = 'active') AS upcoming
FROM events;

-- name: AdminDashboardApplications :one
SELECT 
    COUNT(*) AS total,
    COUNT(*) FILTER (WHERE status = 'pending') AS pending,
    COUNT(*) FILTER (WHERE status = 'approved') AS approved,
    COUNT(*) FILTER (WHERE status = 'rejected') AS rejected,
    COUNT(*) FILTER (WHERE status = 'cancelled') AS cancelled
FROM applications;

-- name: AdminTopCategories :many
SELECT category, COUNT(*) AS count
FROM events
WHERE status != 'cancelled'
GROUP BY category
ORDER BY count DESC
LIMIT 5;

-- name: AdminTopOrganizers :many
SELECT u.id, u.username, u.email, COUNT(e.id) AS events_count
FROM users u
JOIN events e ON e.organizer_id = u.id
WHERE e.status != 'cancelled'
GROUP BY u.id, u.username, u.email
ORDER BY events_count DESC
LIMIT 5;

-- name: AdminActivityChart :many
SELECT 
    to_char(date_trunc('day', created_at), 'YYYY-MM-DD') AS day,
    COUNT(*) AS registrations
FROM users
WHERE created_at >= now() - interval '30 days'
GROUP BY day
ORDER BY day;

-- name: AdminListUsers :many
SELECT id, email, username, role, is_banned, created_at
FROM users
WHERE (sqlc.narg('search')::text IS NULL OR email ILIKE '%' || sqlc.narg('search') || '%' OR username ILIKE '%' || sqlc.narg('search') || '%')
  AND (sqlc.narg('role')::text IS NULL OR role = sqlc.narg('role'))
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: AdminUpdateUserRole :one
UPDATE users SET role = $2, updated_at = now()
WHERE id = $1
RETURNING id, email, username, role, is_banned;

-- name: AdminBanUser :one
UPDATE users SET is_banned = $2, ban_reason = $3, updated_at = now()
WHERE id = $1
RETURNING id, email, username, role, is_banned, ban_reason;

-- name: AdminListEvents :many
SELECT e.id, e.title, e.city, e.category, e.starts_at, e.status, e.organizer_id,
       u.username AS organizer_name
FROM events e
JOIN users u ON u.id = e.organizer_id
WHERE (sqlc.narg('status')::text IS NULL OR e.status = sqlc.narg('status'))
ORDER BY e.created_at DESC
LIMIT $1 OFFSET $2;

-- name: AdminDeleteEvent :exec
DELETE FROM events WHERE id = $1;