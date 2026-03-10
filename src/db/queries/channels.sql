-- name: ListChannels :many
SELECT * FROM channels
WHERE is_voice = false
ORDER BY position ASC, created_at ASC;

-- name: ListVoiceChannels :many
SELECT * FROM channels
WHERE is_voice = true
ORDER BY position ASC, created_at ASC;

-- name: ListAllChannels :many
SELECT * FROM channels
ORDER BY position ASC, created_at ASC;

-- name: GetChannel :one
SELECT * FROM channels
WHERE id = $1;

-- name: CreateChannel :one
INSERT INTO channels (name, topic, position, is_voice)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: UpdateChannel :one
UPDATE channels
SET name = $1, topic = $2, position = $3
WHERE id = $4
RETURNING *;

-- name: DeleteChannel :exec
DELETE FROM channels
WHERE id = $1;
