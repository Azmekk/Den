-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1;

-- name: GetUserByUsername :one
SELECT * FROM users WHERE username = $1;

-- name: CreateUser :one
INSERT INTO users (username, password_hash, display_name, is_admin)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: CountUsers :one
SELECT count(*) FROM users;

-- name: UpdateUserPassword :exec
UPDATE users SET password_hash = $2, updated_at = now() WHERE id = $1;

-- name: SetUserAdmin :exec
UPDATE users SET is_admin = $2, updated_at = now() WHERE id = $1;

-- name: ListUsers :many
SELECT id, username, display_name, avatar_url, is_admin FROM users ORDER BY username;

-- name: DeleteUser :exec
DELETE FROM users WHERE id = $1;
