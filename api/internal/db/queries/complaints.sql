-- name: CreateComplaint :one
INSERT INTO complaints (reporter_id, target_type, target_id, reason)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: ListComplaints :many
SELECT c.*, u.email AS reporter_email
FROM complaints c
LEFT JOIN users u ON u.id = c.reporter_id
WHERE (sqlc.narg('status')::text IS NULL OR c.status = sqlc.narg('status'))
ORDER BY c.created_at DESC
LIMIT $1 OFFSET $2;

-- name: GetComplaint :one
SELECT c.*, u.email AS reporter_email
FROM complaints c
LEFT JOIN users u ON u.id = c.reporter_id
WHERE c.id = $1;

-- name: ResolveComplaint :one
UPDATE complaints
SET status = 'resolved',
    resolution_comment = $2,
    resolved_at = now(),
    resolved_by = $3
WHERE id = $1 AND status = 'open'
RETURNING *;

-- name: RejectComplaint :one
UPDATE complaints
SET status = 'rejected',
    resolution_comment = $2,
    resolved_at = now(),
    resolved_by = $3
WHERE id = $1 AND status = 'open'
RETURNING *;

-- name: AdminGetComplaintTargetEvent :one
SELECT 
    e.id, e.title, e.status, e.city, e.starts_at,
    e.organizer_id, u.username AS organizer_name
FROM events e
JOIN users u ON u.id = e.organizer_id
WHERE e.id = $1;

-- name: AdminGetComplaintTargetUser :one
SELECT id, username, email, is_banned, role, created_at
FROM users WHERE id = $1;

-- name: AdminGetComplaintTargetMessage :one
SELECT 
    m.id, m.event_id, m.user_id, m.text, m.created_at, m.deleted_at,
    u.username AS user_name,
    e.title AS event_title
FROM chat_messages m
JOIN users u ON u.id = m.user_id
JOIN events e ON e.id = m.event_id
WHERE m.id = $1;

-- name: AdminSoftDeleteMessage :one
UPDATE chat_messages
SET deleted_at = now()
WHERE id = $1 AND deleted_at IS NULL
RETURNING id, deleted_at;