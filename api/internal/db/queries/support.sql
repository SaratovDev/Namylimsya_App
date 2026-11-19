-- name: CreateSupportTicket :one
INSERT INTO support_tickets (user_id, type, subject, message)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: CreateComplaintSupportTicket :one
INSERT INTO support_tickets (user_id, type, subject, message, complaint_id)
VALUES ($1, 'complaint', $2, $3, $4)
RETURNING *;

-- name: ListSupportTickets :many
SELECT t.*, u.email AS user_email
FROM support_tickets t
LEFT JOIN users u ON u.id = t.user_id
WHERE (sqlc.narg('status')::text IS NULL OR t.status = sqlc.narg('status'))
ORDER BY t.created_at DESC
LIMIT $1 OFFSET $2;

-- name: GetSupportTicket :one
SELECT t.*, u.email AS user_email, u.username AS user_username
FROM support_tickets t
LEFT JOIN users u ON u.id = t.user_id
WHERE t.id = $1;

-- name: ResolveSupportTicket :one
UPDATE support_tickets
SET status = 'resolved',
    resolution_comment = $2,
    resolved_at = now(),
    resolved_by = $3
WHERE id = $1 AND status = 'open'
RETURNING *;

-- name: RejectSupportTicket :one
UPDATE support_tickets
SET status = 'rejected',
    resolution_comment = $2,
    resolved_at = now(),
    resolved_by = $3
WHERE id = $1 AND status = 'open'
RETURNING *;

-- name: ListTicketReplies :many
SELECT * FROM ticket_replies
WHERE ticket_id = $1
ORDER BY created_at ASC;

-- name: CreateTicketReply :one
INSERT INTO ticket_replies (ticket_id, admin_id, admin_email, body)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: CountSupportTickets :one
SELECT COUNT(*) FROM support_tickets WHERE status = 'open';