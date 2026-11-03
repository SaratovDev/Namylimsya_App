-- name: CreateMessage :one
INSERT INTO chat_messages (event_id, user_id, text, reply_to_id)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetMessages :many
SELECT 
    m.id, m.event_id, m.user_id, m.text, m.reply_to_id, m.created_at, m.deleted_at,
    u.username AS user_name, u.avatar_url AS user_avatar
FROM chat_messages m
JOIN users u ON u.id = m.user_id
WHERE m.event_id = $1
  AND m.deleted_at IS NULL
  AND (sqlc.narg('cursor_time')::timestamptz IS NULL OR m.created_at < sqlc.narg('cursor_time'))
ORDER BY m.created_at DESC
LIMIT $2;

-- name: MarkChatRead :exec
INSERT INTO chat_reads (user_id, event_id, last_read_at)
VALUES ($1, $2, now())
ON CONFLICT (user_id, event_id) DO UPDATE SET last_read_at = now();

-- name: GetLastRead :one
SELECT last_read_at FROM chat_reads WHERE user_id = $1 AND event_id = $2;

-- name: CountUnreadMessages :one
SELECT COUNT(*) FROM chat_messages m
WHERE m.event_id = $1
  AND m.deleted_at IS NULL
  AND m.user_id != $2
  AND m.created_at > COALESCE(
    (SELECT last_read_at FROM chat_reads WHERE user_id = $2 AND event_id = $1),
    '1970-01-01'::timestamptz
  );