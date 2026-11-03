-- name: CreateUser :one
INSERT INTO users (
    email, password_hash, username, gender, birth_date, city
) VALUES (
    $1, $2, $3, $4, $5, $6
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