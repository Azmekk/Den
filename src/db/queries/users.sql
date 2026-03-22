-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1;

-- name: GetUserByUsername :one
SELECT * FROM users WHERE username = $1;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = $1;

-- name: CreateUser :one
INSERT INTO users (username, email, password_hash, display_name, is_admin)
VALUES ($1, $2, $3, $4, $5)
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

-- name: SetUserPasswordHash :exec
UPDATE users SET password_hash = $2, updated_at = now() WHERE id = $1;

-- name: SetUserTOTPSecret :exec
UPDATE users SET totp_secret = $2, totp_enabled = $3, recovery_codes = $4, updated_at = now() WHERE id = $1;

-- name: ClearUserTOTP :exec
UPDATE users SET totp_secret = NULL, totp_enabled = false, recovery_codes = NULL, updated_at = now() WHERE id = $1;

-- name: SetUserRecoveryCodes :exec
UPDATE users SET recovery_codes = $2, updated_at = now() WHERE id = $1;

-- name: SetEmailVerified :exec
UPDATE users SET email_verified = true, email_verify_token = NULL, email_verify_expires_at = NULL, updated_at = now() WHERE id = $1;

-- name: SetEmailVerifyToken :exec
UPDATE users SET email_verify_token = $2, email_verify_expires_at = $3, updated_at = now() WHERE id = $1;

-- name: SetPasswordResetToken :exec
UPDATE users SET password_reset_token = $2, password_reset_expires_at = $3, updated_at = now() WHERE id = $1;

-- name: ClearPasswordResetToken :exec
UPDATE users SET password_reset_token = NULL, password_reset_expires_at = NULL, updated_at = now() WHERE id = $1;

-- name: GetUserByPasswordResetToken :one
SELECT * FROM users WHERE password_reset_token = $1 AND password_reset_expires_at > now();

-- name: GetUserByEmailVerifyToken :one
SELECT * FROM users WHERE email_verify_token = $1 AND email_verify_expires_at > now();
