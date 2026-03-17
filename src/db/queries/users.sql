-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1;

-- name: GetUserByUsername :one
SELECT * FROM users WHERE username = $1;

-- name: GetUserBySupabaseID :one
SELECT * FROM users WHERE supabase_id = $1;

-- name: CreateUser :one
INSERT INTO users (username, password_hash, display_name, is_admin, supabase_id, needs_username)
VALUES ($1, '', $2, $3, $4, $5)
RETURNING *;

-- name: CountUsers :one
SELECT count(*) FROM users;

-- name: SetUserAdmin :exec
UPDATE users SET is_admin = $2, updated_at = now() WHERE id = $1;

-- name: ListUsers :many
SELECT id, username, display_name, avatar_url, color, is_admin, banned FROM users ORDER BY username;

-- name: DeleteUser :exec
DELETE FROM users WHERE id = $1;

-- name: GetUsersByUsernames :many
SELECT id, username FROM users WHERE username = ANY($1::text[]);

-- name: UpdateUserDisplayName :one
UPDATE users SET display_name = $2, updated_at = now() WHERE id = $1 RETURNING *;

-- name: UpdateUserColor :one
UPDATE users SET color = $2, updated_at = now() WHERE id = $1 RETURNING *;

-- name: UpdateUserAvatarUrl :one
UPDATE users SET avatar_url = $2, updated_at = now() WHERE id = $1 RETURNING *;

-- name: SetUserBanned :exec
UPDATE users SET banned = $2, updated_at = now() WHERE id = $1;

-- name: SetUserUsername :one
UPDATE users SET username = $2, needs_username = false, updated_at = now() WHERE id = $1 RETURNING *;

-- name: GetUserBannedBySupabaseID :one
SELECT banned FROM users WHERE supabase_id = $1;
