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
    username = COALESCE(sqlc.narg('username'), username),
    city = COALESCE(sqlc.narg('city'), city),
    bio = COALESCE(sqlc.narg('bio'), bio),
    hobbies = COALESCE(sqlc.narg('hobbies'), hobbies),
    show_age = COALESCE(sqlc.narg('show_age'), show_age),
    updated_at = now()
WHERE id = $1
RETURNING *;