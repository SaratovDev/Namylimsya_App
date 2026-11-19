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
SELECT COUNT(*) FROM applications a
JOIN events e ON e.id = a.event_id
WHERE a.user_id = $1
  AND a.status IN ('pending', 'approved')
  AND e.status = 'active'
  AND e.starts_at > now();

-- name: CountApprovedApplicationsForEvent :one
SELECT COUNT(*) FROM applications 
WHERE event_id = $1 AND status = 'approved';

-- name: UpdateApplicationStatus :one
-- allowed_from — допустимые исходные статусы. WHERE status = ANY(...) защищает
-- от гонки: если параллельный запрос уже сменил статус, UPDATE не затронет
-- строку и вернёт pgx.ErrNoRows — вызывающий код маппит в бизнес-ошибку.
-- Без этого Reject+Approve одновременно оба "выигрывали" и оба слали уведомления.
UPDATE applications
SET status = $2, cancel_reason = $3, updated_at = now()
WHERE id = $1
  AND status = ANY(sqlc.arg('allowed_from')::text[])
RETURNING *;

-- name: ListEventApplications :many
SELECT a.*, u.username, u.avatar_url
FROM applications a
JOIN users u ON u.id = a.user_id
WHERE a.event_id = $1
ORDER BY a.created_at ASC;

-- name: ListApprovedForEvent :many
SELECT * FROM applications 
WHERE event_id = $1 AND status = 'approved';

-- name: GetEventForUpdate :one
-- Блокирует строку события до конца транзакции. Используется в Apply и Approve
-- для сериализации операций, меняющих состояние заявок на это событие.
SELECT * FROM events WHERE id = $1 FOR UPDATE;

-- name: ListActiveApplicationsForEvent :many
-- Pending + approved заявки. Для уведомлений перед каскадной отменой.
SELECT * FROM applications
WHERE event_id = $1 AND status IN ('pending', 'approved')
ORDER BY created_at ASC;

-- name: CancelAllActiveApplicationsForEvent :exec
-- Каскадная отмена при отмене события.
UPDATE applications
SET status = 'cancelled',
    cancel_reason = COALESCE(sqlc.narg('cancel_reason')::text, 'event_cancelled'),
    updated_at = now()
WHERE event_id = $1 AND status IN ('pending', 'approved');

-- name: ListApplicationsByUser :many
SELECT 
    a.*,
    e.title           AS event_title,
    e.starts_at       AS event_starts_at,
    e.city            AS event_city,
    e.category        AS event_category,
    e.status          AS event_status,
    e.cancel_reason   AS event_cancel_reason,
    e.organizer_id    AS event_organizer_id,
    u.username        AS organizer_name,
    u.avatar_url      AS organizer_avatar
FROM applications a
JOIN events e ON e.id = a.event_id
JOIN users u ON u.id = e.organizer_id
WHERE a.user_id = $1
ORDER BY a.created_at DESC;