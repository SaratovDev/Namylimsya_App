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
    username   = COALESCE(NULLIF($2::text, ''), username),
    city       = COALESCE(NULLIF($3::text, ''), city),
    bio        = COALESCE($4::text, bio),
    hobbies    = COALESCE($5::text[], hobbies),
    show_age   = COALESCE($6::boolean, show_age),
    updated_at = now()
WHERE id = $1
RETURNING *;