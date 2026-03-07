-- name: CreateMessage :one
INSERT INTO messages (channel_id, user_id, content)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetMessageByID :one
SELECT * FROM messages
WHERE id = $1;

-- name: GetLatestMessagesByChannel :many
SELECT m.id, m.channel_id, m.user_id, m.content, m.pinned, m.edited_at, m.created_at,
       u.username, u.display_name, u.avatar_url
FROM messages m
JOIN users u ON u.id = m.user_id
WHERE m.channel_id = $1
ORDER BY m.created_at DESC, m.id DESC
LIMIT 50;

-- name: GetMessagesByChannel :many
SELECT m.id, m.channel_id, m.user_id, m.content, m.pinned, m.edited_at, m.created_at,
       u.username, u.display_name, u.avatar_url
FROM messages m
JOIN users u ON u.id = m.user_id
WHERE m.channel_id = @channel_id
  AND (m.created_at < @before_time OR (m.created_at = @before_time AND m.id < @before_id))
ORDER BY m.created_at DESC, m.id DESC
LIMIT 50;

-- name: UpdateMessageContent :one
UPDATE messages
SET content = $1, edited_at = now()
WHERE id = $2
RETURNING *;

-- name: DeleteMessage :exec
DELETE FROM messages
WHERE id = $1;
