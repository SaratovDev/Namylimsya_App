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
UPDATE users SET 
    is_banned = $2, 
    ban_reason = $3, 
    ban_expires_at = $4,
    updated_at = now()
WHERE id = $1
RETURNING id, email, username, role, is_banned, ban_reason, ban_expires_at;

-- name: AdminListEvents :many
SELECT 
    e.id, e.title, e.city, e.category, e.starts_at, e.status,
    e.organizer_id, e.hidden, e.created_at,
    u.username AS organizer_name,
    (SELECT COUNT(*) FROM applications a WHERE a.event_id = e.id AND a.status = 'approved')::bigint AS participants_count,
    (SELECT COUNT(*) FROM applications a WHERE a.event_id = e.id AND a.status = 'pending')::bigint AS pending_count
FROM events e
JOIN users u ON u.id = e.organizer_id
WHERE (sqlc.narg('status')::text IS NULL OR e.status = sqlc.narg('status'))
  AND (sqlc.narg('search')::text IS NULL OR e.title ILIKE '%' || sqlc.narg('search') || '%')
  AND (sqlc.narg('category')::text IS NULL OR e.category = sqlc.narg('category'))
  AND (sqlc.narg('city')::text IS NULL OR e.city = sqlc.narg('city'))
ORDER BY e.created_at DESC
LIMIT $1 OFFSET $2;

-- name: AdminDeleteEvent :exec
DELETE FROM events WHERE id = $1;

-- name: AdminGetUserDetails :one
SELECT 
    u.id, u.email, u.username, u.gender, u.birth_date, u.show_age,
    u.city, u.bio, u.hobbies, u.telegram, u.vk, u.avatar_url,
    u.role, u.is_banned, u.ban_reason, u.ban_expires_at,
    u.email_verified, u.consent_at, u.consent_version, u.age_confirmed_at,
    u.created_at, u.updated_at,
    (SELECT COUNT(*) FROM events e WHERE e.organizer_id = u.id)::bigint          AS organized_events_count,
    (SELECT COUNT(*) FROM applications a WHERE a.user_id = u.id)::bigint         AS applications_total,
    (SELECT COUNT(*) FROM applications a WHERE a.user_id = u.id AND a.status = 'approved')::bigint AS applications_approved,
    (SELECT COUNT(*) FROM applications a WHERE a.user_id = u.id AND a.status = 'pending')::bigint  AS applications_pending,
    (SELECT COUNT(*) FROM applications a WHERE a.user_id = u.id AND a.status = 'rejected')::bigint AS applications_rejected,
    (SELECT COUNT(*) FROM applications a WHERE a.user_id = u.id AND a.status = 'cancelled')::bigint AS applications_cancelled
FROM users u
WHERE u.id = $1;

-- name: AdminGetUserRole :one
SELECT id, role, is_banned FROM users WHERE id = $1;

-- name: AdminInsertAuditLog :one
INSERT INTO admin_audit_log (admin_id, admin_email, action, target_type, target_id, details)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: AdminListAuditLog :many
SELECT * FROM admin_audit_log
WHERE (sqlc.narg('admin_id')::uuid IS NULL OR admin_id = sqlc.narg('admin_id'))
  AND (sqlc.narg('action')::text IS NULL OR action = sqlc.narg('action'))
  AND (sqlc.narg('target_type')::text IS NULL OR target_type = sqlc.narg('target_type'))
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;