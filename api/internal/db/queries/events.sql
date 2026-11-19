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
SELECT 
    e.*,
    u.username AS organizer_name,
    u.avatar_url AS organizer_avatar,
    (
        SELECT COUNT(*) FROM applications a 
        WHERE a.event_id = e.id AND a.status = 'approved'
    )::bigint AS participants_count,
    COALESCE(
        (
            SELECT a.status::text FROM applications a
            WHERE a.event_id = e.id AND a.user_id = sqlc.narg('viewer_id')
            LIMIT 1
        ),
        ''
    ) AS user_application_status,
    a_viewer.id AS user_application_id,
    CASE
        WHEN e.hide_exact_location = false THEN true
        WHEN e.organizer_id = sqlc.narg('viewer_id') THEN true
        WHEN EXISTS (
            SELECT 1 FROM applications a
            WHERE a.event_id = e.id
              AND a.user_id = sqlc.narg('viewer_id')
              AND a.status = 'approved'
        ) THEN true
        ELSE false
    END AS can_see_exact_location
FROM events e
JOIN users u ON u.id = e.organizer_id
LEFT JOIN applications a_viewer ON a_viewer.event_id = e.id AND a_viewer.user_id = sqlc.narg('viewer_id')
WHERE e.id = $1;

-- name: CountActiveEventsByOrganizer :one
SELECT COUNT(*) FROM events 
WHERE organizer_id = $1 AND status = 'active' AND starts_at > now();

-- name: GetEventsFeed :many
SELECT 
    e.*, 
    u.username AS organizer_name, 
    u.avatar_url AS organizer_avatar,
    COALESCE(COUNT(DISTINCT a_approved.id), 0)::bigint AS participants_count,
    COALESCE(a_viewer.status::text, '') AS user_application_status,
    a_viewer.id AS user_application_id,
    CASE
        WHEN e.organizer_id = sqlc.narg('viewer_id') THEN (
            SELECT COUNT(*) FROM applications a 
            WHERE a.event_id = e.id AND a.status = 'pending'
        )::bigint
        ELSE 0::bigint
    END AS pending_applications_count,
    CASE
        WHEN e.hide_exact_location = false THEN true
        WHEN e.organizer_id = sqlc.narg('viewer_id') THEN true
        WHEN a_viewer.status = 'approved' THEN true
        ELSE false
    END AS can_see_exact_location
FROM events e
JOIN users u ON u.id = e.organizer_id
LEFT JOIN applications a_approved ON a_approved.event_id = e.id AND a_approved.status = 'approved'
LEFT JOIN applications a_viewer ON a_viewer.event_id = e.id AND a_viewer.user_id = sqlc.narg('viewer_id')
WHERE e.status = 'active' 
  AND e.hidden = false
  AND e.starts_at >= now()
  AND (
    sqlc.narg('category')::text IS NULL 
    OR e.category = ANY(string_to_array(sqlc.narg('category')::text, ','))
)
  AND (sqlc.narg('city')::text IS NULL OR e.city = sqlc.narg('city'))
  AND (sqlc.narg('date_from')::timestamptz IS NULL OR e.starts_at >= sqlc.narg('date_from'))
  AND (sqlc.narg('date_to')::timestamptz IS NULL OR e.starts_at <= sqlc.narg('date_to'))
  -- Композитный курсор: (starts_at, id) строго больше последней позиции.
  -- Без id два события в одну секунду на границе страницы пропадали
  -- или дублировались (starts_at > cursor — строгое неравенство).
  AND (
    sqlc.narg('cursor_time')::timestamptz IS NULL
    OR (e.starts_at, e.id) > (sqlc.narg('cursor_time')::timestamptz, sqlc.narg('cursor_id')::uuid)
  )
GROUP BY e.id, u.username, u.avatar_url, a_viewer.id, a_viewer.status
ORDER BY e.starts_at ASC, e.id ASC
LIMIT $1;

-- name: GetEventsArchive :many
SELECT 
    e.*, 
    u.username AS organizer_name, 
    u.avatar_url AS organizer_avatar,
    COALESCE(a_viewer.status::text, '') AS user_application_status,
    CASE
        WHEN e.hide_exact_location = false THEN true
        WHEN e.organizer_id = sqlc.narg('viewer_id') THEN true
        WHEN a_viewer.status = 'approved' THEN true
        ELSE false
    END AS can_see_exact_location
FROM events e
JOIN users u ON u.id = e.organizer_id
LEFT JOIN applications a_viewer ON a_viewer.event_id = e.id AND a_viewer.user_id = sqlc.narg('viewer_id')
WHERE (
    -- Завершённые — все.
    e.status = 'completed'
    -- «Идёт сейчас»: активное, началось не позже now, кончится не раньше now.
    -- Дефолтная длительность 2 часа (ends_at пока нет). После 2 часов
    -- EventWorker переведёт в completed — в архив попадёт уже как завершённое.
    OR (e.status = 'active' AND e.starts_at <= now() AND e.starts_at + interval '2 hours' > now())
    
    -- Отменённые — только тем, кто был причастен (орг или подавал любую заявку).
    -- ВАЖНО: при отмене события Cancel каскадно переводит pending/approved
    -- в cancelled — поэтому проверять `status IN ('approved','pending')` бессмысленно
    -- (все заявки уже cancelled). Ищем ЛЮБУЮ заявку: если юзер интересовался
    -- событием (подавал заявку) — он должен видеть, что оно отменилось.
    OR (e.status = 'cancelled' AND (
        e.organizer_id = sqlc.narg('viewer_id')
        OR EXISTS (
            SELECT 1 FROM applications a
            WHERE a.event_id = e.id AND a.user_id = sqlc.narg('viewer_id')
        )
    ))
)
  -- Те же фильтры, что в ленте.
  AND (sqlc.narg('category')::text IS NULL OR e.category = ANY(string_to_array(sqlc.narg('category')::text, ',')))
  AND (sqlc.narg('city')::text IS NULL OR e.city = sqlc.narg('city'))
  AND (sqlc.narg('date_from')::timestamptz IS NULL OR e.starts_at >= sqlc.narg('date_from'))
  AND (sqlc.narg('date_to')::timestamptz IS NULL OR e.starts_at <= sqlc.narg('date_to'))
  AND (
    sqlc.narg('cursor_time')::timestamptz IS NULL
    OR (e.starts_at, e.id) < (sqlc.narg('cursor_time')::timestamptz, sqlc.narg('cursor_id')::uuid)
  )
ORDER BY e.starts_at DESC, e.id DESC
LIMIT $1;

-- name: CancelEvent :one
UPDATE events
SET status = 'cancelled', cancel_reason = $2, updated_at = now()
WHERE id = $1 AND organizer_id = $3 AND status = 'active'
RETURNING *;

-- name: UpdateEvent :one
UPDATE events
SET 
    title = $2,
    category = $3,
    description = $4,
    starts_at = $5,
    city = $6,
    address = $7,
    lat = $8,
    lng = $9,
    location = ST_SetSRID(ST_MakePoint($9, $8), 4326)::geography,
    hide_exact_location = $10,
    participant_limit = $11,
    updated_at = now()
WHERE id = $1 AND organizer_id = $12
RETURNING *;

-- name: GetMyOrganizedEvents :many
SELECT 
    e.*, 
    u.username AS organizer_name, 
    u.avatar_url AS organizer_avatar,
    (SELECT COUNT(*) FROM applications a WHERE a.event_id = e.id AND a.status = 'approved')::bigint AS participants_count,
    ''::text AS user_application_status,
    NULL::uuid AS user_application_id,
    (SELECT COUNT(*) FROM applications a WHERE a.event_id = e.id AND a.status = 'pending')::bigint AS pending_applications_count,
    true AS can_see_exact_location
FROM events e
JOIN users u ON u.id = e.organizer_id
WHERE e.organizer_id = $1
ORDER BY e.starts_at DESC;

-- name: AdminCancelEvent :one
UPDATE events
SET status = 'cancelled', cancel_reason = $2, updated_at = now()
WHERE id = $1 AND status = 'active'
RETURNING *;

-- name: AdminHideEvent :one
UPDATE events
SET hidden = true, updated_at = now()
WHERE id = $1
RETURNING id, hidden;

-- name: AdminUnhideEvent :one
UPDATE events
SET hidden = false, updated_at = now()
WHERE id = $1
RETURNING id, hidden;

-- name: AdminGetEventDetails :one
SELECT 
    e.*,
    u.username AS organizer_name,
    u.email AS organizer_email,
    u.avatar_url AS organizer_avatar,
    (SELECT COUNT(*) FROM applications a WHERE a.event_id = e.id AND a.status = 'approved')::bigint AS participants_count,
    (SELECT COUNT(*) FROM applications a WHERE a.event_id = e.id AND a.status = 'pending')::bigint  AS pending_count,
    (SELECT COUNT(*) FROM applications a WHERE a.event_id = e.id AND a.status = 'rejected')::bigint AS rejected_count,
    (SELECT COUNT(*) FROM chat_messages m WHERE m.event_id = e.id AND m.deleted_at IS NULL)::bigint AS messages_count
FROM events e
JOIN users u ON u.id = e.organizer_id
WHERE e.id = $1;

-- name: AdminGetRecentMessages :many
SELECT 
    m.id, m.event_id, m.user_id, m.text, m.reply_to_id, m.created_at, m.deleted_at,
    u.username AS user_name, u.avatar_url AS user_avatar
FROM chat_messages m
JOIN users u ON u.id = m.user_id
WHERE m.event_id = $1
ORDER BY m.created_at DESC
LIMIT $2;

-- name: MarkPastEventsCompleted :exec
-- Событие считается завершённым через 2 часа после старта (дефолтная
-- длительность). Пока этот интервал не истёк, событие видно в архиве
-- с плашкой «идёт сейчас». Если появится ends_at — заменить на него.
UPDATE events SET status = 'completed', updated_at = now()
WHERE status = 'active' AND starts_at + interval '2 hours' < now();

-- name: ListApprovedParticipants :many
-- Участники события со статусом approved. Организатор — всегда первый.
-- Pending/rejected/cancelled НЕ возвращаются (pending виден только орг
-- через /events/{id}/applications).
SELECT 
    u.id, u.username, u.avatar_url, u.city, u.birth_date, u.show_age,
    (u.id = e.organizer_id) AS is_organizer
FROM applications a
JOIN users u ON u.id = a.user_id
JOIN events e ON e.id = a.event_id
WHERE a.event_id = $1 AND a.status = 'approved'
ORDER BY (u.id = e.organizer_id) DESC, a.created_at ASC;