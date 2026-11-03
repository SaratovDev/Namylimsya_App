-- name: CreateApplication :one
INSERT INTO applications (event_id, user_id)
VALUES ($1, $2)
RETURNING *;

-- name: GetApplication :one
SELECT * FROM applications WHERE id = $1;

-- name: GetUserApplicationForEvent :one
SELECT * FROM applications 
WHERE event_id = $1 AND user_id = $2;

-- name: CountActiveApplicationsByUser :one
SELECT COUNT(*) FROM applications 
WHERE user_id = $1 AND status IN ('pending', 'approved');

-- name: CountApprovedApplicationsForEvent :one
SELECT COUNT(*) FROM applications 
WHERE event_id = $1 AND status = 'approved';

-- name: UpdateApplicationStatus :one
UPDATE applications
SET status = $2, cancel_reason = $3, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: ListEventApplications :many
SELECT a.*, u.username, u.avatar_url
FROM applications a
JOIN users u ON u.id = a.user_id
WHERE a.event_id = $1
ORDER BY a.created_at ASC;