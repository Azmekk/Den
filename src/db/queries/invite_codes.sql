-- name: CreateInviteCode :one
INSERT INTO invite_codes (code, max_uses, expires_at, created_by)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetInviteCodeByCode :one
SELECT * FROM invite_codes WHERE code = $1;

-- name: ListInviteCodes :many
SELECT ic.*, u.username AS created_by_username
FROM invite_codes ic
JOIN users u ON u.id = ic.created_by
ORDER BY ic.created_at DESC;

-- name: IncrementInviteCodeUseCount :exec
UPDATE invite_codes SET use_count = use_count + 1 WHERE id = $1;

-- name: DeleteInviteCode :exec
DELETE FROM invite_codes WHERE id = $1;
