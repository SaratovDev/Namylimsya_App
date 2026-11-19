-- name: CreateUser :one
INSERT INTO users (
    email, password_hash, username, gender, birth_date, city,
    consent_at, consent_version, age_confirmed_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9
) RETURNING *;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = $1;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1;

-- name: UpdateUser :one
UPDATE users
SET
    username = $2, gender = $3, birth_date = $4, city = $5, bio = $6,
    hobbies = $7, telegram = $8, vk = $9, avatar_url = $10, show_age = $11,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: GetPublicUser :one
SELECT 
    id, username, gender, birth_date, show_age, city, bio, hobbies, avatar_url,
    created_at, updated_at
FROM users WHERE id = $1;

-- name: UnbanExpiredUsers :exec
-- Снимает is_banned с пользователей, у которых истёк ban_expires_at.
-- Вызывается фоновым воркером раз в несколько минут, чтобы БД не копила
-- «зомби»-забаненных и Login/Refresh не путались.
UPDATE users
SET is_banned = false,
    ban_reason = NULL,
    ban_expires_at = NULL,
    updated_at = now()
WHERE is_banned = true
  AND ban_expires_at IS NOT NULL
  AND ban_expires_at < now();

-- name: GetPublicUserFull :one
-- Расширенный публичный профиль: базовые поля + счётчики + роль/бан.
-- Используется на /users/[id]. Старый GetPublicUser оставляем для
-- мест, где counts не нужны (пока таких нет, но не ломаем API).
SELECT
    u.id, u.username, u.gender, u.birth_date, u.show_age, u.city, u.bio,
    u.hobbies, u.avatar_url, u.role, u.is_banned, u.created_at, u.updated_at,
    (
        SELECT COUNT(*) FROM events e
        WHERE e.organizer_id = u.id
          AND e.status = 'completed'
          AND EXISTS (
              SELECT 1 FROM applications a
              WHERE a.event_id = e.id AND a.status = 'approved'
          )
    )::bigint AS organized_count,
    (
        SELECT COUNT(*) FROM applications a
        JOIN events e ON e.id = a.event_id
        WHERE a.user_id = u.id
          AND a.status = 'approved'
          AND e.status = 'completed'
    )::bigint AS attended_count
FROM users u
WHERE u.id = $1;

-- name: ListUserOrganizedCompleted :many
-- Завершённые события, которые юзер организовал и на которые были
-- одобренные участники. Отменённые — не включаем.
SELECT
    e.id, e.title, e.category, e.starts_at, e.city,
    (
        SELECT COUNT(*) FROM applications a
        WHERE a.event_id = e.id AND a.status = 'approved'
    )::bigint AS participants_count
FROM events e
WHERE e.organizer_id = $1
  AND e.status = 'completed'
  AND EXISTS (
      SELECT 1 FROM applications a
      WHERE a.event_id = e.id AND a.status = 'approved'
  )
ORDER BY e.starts_at DESC
LIMIT $2;

-- name: ListUserAttendedCompleted :many
-- Завершённые события, куда юзер был approved. Пока без QR-чекина —
-- «посетил» = «был одобрен». После A3 (QR) заменим на attendance.
SELECT
    e.id, e.title, e.category, e.starts_at, e.city,
    u.username AS organizer_name, u.avatar_url AS organizer_avatar
FROM applications a
JOIN events e ON e.id = a.event_id
JOIN users u ON u.id = e.organizer_id
WHERE a.user_id = $1
  AND a.status = 'approved'
  AND e.status = 'completed'
ORDER BY e.starts_at DESC
LIMIT $2;

-- name: GetUsernameHistory :many
SELECT id, old_username, changed_at
FROM username_history
WHERE user_id = $1
ORDER BY changed_at DESC;

-- name: InsertUsernameHistory :exec
INSERT INTO username_history (user_id, old_username)
VALUES ($1, $2);

-- name: UpdateUserFull :one
-- Полное обновление профиля (включая username с проверкой «один раз»).
-- username передаётся уже валидированным в сервисе.
UPDATE users
SET
    username = $2,
    gender = $3,
    birth_date = $4,
    city = $5,
    bio = $6,
    hobbies = $7,
    telegram = $8,
    vk = $9,
    avatar_url = $10,
    show_age = $11,
    username_changed_at = CASE 
        WHEN $2 != username THEN now() 
        ELSE username_changed_at 
    END,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: UpdateUserPassword :exec
UPDATE users SET password_hash = $2, updated_at = now() WHERE id = $1;

-- name: MarkEmailVerified :exec
UPDATE users SET email_verified = true, updated_at = now() WHERE id = $1;