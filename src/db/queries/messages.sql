-- name: CreateMessage :one
INSERT INTO messages (channel_id, user_id, content)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetMessageByID :one
SELECT * FROM messages
WHERE id = $1;

-- name: GetLatestMessagesByChannel :many
SELECT sub.id, sub.channel_id, sub.user_id, sub.content, sub.pinned, sub.edited_at, sub.created_at,
       sub.username, sub.display_name, sub.avatar_url
FROM (
  SELECT m.id, m.channel_id, m.user_id, m.content, m.pinned, m.edited_at, m.created_at,
         u.username, u.display_name, u.avatar_url
  FROM messages m
  JOIN users u ON u.id = m.user_id
  WHERE m.channel_id = $1
  ORDER BY m.created_at DESC, m.id DESC
  LIMIT 50
) sub
ORDER BY sub.created_at ASC, sub.id ASC;

-- name: GetMessagesByChannel :many
SELECT sub.id, sub.channel_id, sub.user_id, sub.content, sub.pinned, sub.edited_at, sub.created_at,
       sub.username, sub.display_name, sub.avatar_url
FROM (
  SELECT m.id, m.channel_id, m.user_id, m.content, m.pinned, m.edited_at, m.created_at,
         u.username, u.display_name, u.avatar_url
  FROM messages m
  JOIN users u ON u.id = m.user_id
  WHERE m.channel_id = @channel_id
    AND (m.created_at < @before_time OR (m.created_at = @before_time AND m.id < @before_id))
  ORDER BY m.created_at DESC, m.id DESC
  LIMIT 50
) sub
ORDER BY sub.created_at ASC, sub.id ASC;

-- name: UpdateMessageContent :one
UPDATE messages
SET content = $1, edited_at = now()
WHERE id = $2
RETURNING *;

-- name: DeleteMessage :exec
DELETE FROM messages
WHERE id = $1;

-- name: CountMessages :one
SELECT count(*) FROM messages;

-- name: DeleteOldestMessages :exec
DELETE FROM messages
WHERE id IN (
  SELECT id FROM messages
  WHERE pinned = false
  ORDER BY created_at ASC
  LIMIT $1
);

-- name: CreateDMMessage :one
INSERT INTO messages (dm_pair_id, user_id, content)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetLatestDMMessages :many
SELECT sub.id, sub.dm_pair_id, sub.user_id, sub.content, sub.pinned, sub.edited_at, sub.created_at,
       sub.username, sub.display_name, sub.avatar_url
FROM (
  SELECT m.id, m.dm_pair_id, m.user_id, m.content, m.pinned, m.edited_at, m.created_at,
         u.username, u.display_name, u.avatar_url
  FROM messages m
  JOIN users u ON u.id = m.user_id
  WHERE m.dm_pair_id = $1
  ORDER BY m.created_at DESC, m.id DESC
  LIMIT 50
) sub
ORDER BY sub.created_at ASC, sub.id ASC;

-- name: GetDMMessagesByPair :many
SELECT sub.id, sub.dm_pair_id, sub.user_id, sub.content, sub.pinned, sub.edited_at, sub.created_at,
       sub.username, sub.display_name, sub.avatar_url
FROM (
  SELECT m.id, m.dm_pair_id, m.user_id, m.content, m.pinned, m.edited_at, m.created_at,
         u.username, u.display_name, u.avatar_url
  FROM messages m
  JOIN users u ON u.id = m.user_id
  WHERE m.dm_pair_id = @dm_pair_id
    AND (m.created_at < @before_time OR (m.created_at = @before_time AND m.id < @before_id))
  ORDER BY m.created_at DESC, m.id DESC
  LIMIT 50
) sub
ORDER BY sub.created_at ASC, sub.id ASC;

-- name: SetMessagePinned :one
UPDATE messages SET pinned = $2 WHERE id = $1
RETURNING *;

-- name: GetPinnedMessagesByChannel :many
SELECT m.id, m.channel_id, m.user_id, m.content, m.pinned, m.edited_at, m.created_at,
       u.username, u.display_name, u.avatar_url
FROM messages m
JOIN users u ON u.id = m.user_id
WHERE m.channel_id = $1 AND m.pinned = true
ORDER BY m.created_at DESC;

-- name: GetPinnedDMMessages :many
SELECT m.id, m.dm_pair_id, m.user_id, m.content, m.pinned, m.edited_at, m.created_at,
       u.username, u.display_name, u.avatar_url
FROM messages m
JOIN users u ON u.id = m.user_id
WHERE m.dm_pair_id = $1 AND m.pinned = true
ORDER BY m.created_at DESC;

-- name: CountChannels :one
SELECT count(*) FROM channels;
