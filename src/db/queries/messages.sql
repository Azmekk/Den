-- name: CreateMessage :one
INSERT INTO messages (channel_id, user_id, content, reply_to_id, is_reply)
VALUES ($1, $2, $3, sqlc.narg('reply_to_id'), @is_reply)
RETURNING *;

-- name: GetMessageByID :one
SELECT m.*, u.username AS reply_to_username, u.id AS reply_to_user_id
FROM messages m
LEFT JOIN messages rm ON rm.id = m.reply_to_id
LEFT JOIN users u ON u.id = rm.user_id
WHERE m.id = $1;

-- name: GetLatestMessagesByChannel :many
SELECT sub.id, sub.channel_id, sub.user_id, sub.content, sub.pinned, sub.edited_at, sub.created_at,
       sub.username, sub.display_name, sub.avatar_url,
       sub.reply_to_id, sub.is_reply, rm.content AS reply_to_content, ru.id AS reply_to_user_id, ru.username AS reply_to_username
FROM (
  SELECT m.id, m.channel_id, m.user_id, m.content, m.pinned, m.edited_at, m.created_at,
         u.username, u.display_name, u.avatar_url, m.reply_to_id, m.is_reply
  FROM messages m
  JOIN users u ON u.id = m.user_id
  WHERE m.channel_id = $1
  ORDER BY m.created_at DESC, m.id DESC
  LIMIT 50
) sub
LEFT JOIN messages rm ON rm.id = sub.reply_to_id
LEFT JOIN users ru ON ru.id = rm.user_id
ORDER BY sub.created_at ASC, sub.id ASC;

-- name: GetMessagesByChannel :many
SELECT sub.id, sub.channel_id, sub.user_id, sub.content, sub.pinned, sub.edited_at, sub.created_at,
       sub.username, sub.display_name, sub.avatar_url,
       sub.reply_to_id, sub.is_reply, rm.content AS reply_to_content, ru.id AS reply_to_user_id, ru.username AS reply_to_username
FROM (
  SELECT m.id, m.channel_id, m.user_id, m.content, m.pinned, m.edited_at, m.created_at,
         u.username, u.display_name, u.avatar_url, m.reply_to_id, m.is_reply
  FROM messages m
  JOIN users u ON u.id = m.user_id
  WHERE m.channel_id = @channel_id
    AND (m.created_at < @before_time OR (m.created_at = @before_time AND m.id < @before_id))
  ORDER BY m.created_at DESC, m.id DESC
  LIMIT 50
) sub
LEFT JOIN messages rm ON rm.id = sub.reply_to_id
LEFT JOIN users ru ON ru.id = rm.user_id
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
INSERT INTO messages (dm_pair_id, user_id, content, reply_to_id, is_reply)
VALUES ($1, $2, $3, sqlc.narg('reply_to_id'), @is_reply)
RETURNING *;

-- name: GetLatestDMMessages :many
SELECT sub.id, sub.dm_pair_id, sub.user_id, sub.content, sub.pinned, sub.edited_at, sub.created_at,
       sub.username, sub.display_name, sub.avatar_url,
       sub.reply_to_id, sub.is_reply, rm.content AS reply_to_content, ru.id AS reply_to_user_id, ru.username AS reply_to_username
FROM (
  SELECT m.id, m.dm_pair_id, m.user_id, m.content, m.pinned, m.edited_at, m.created_at,
         u.username, u.display_name, u.avatar_url, m.reply_to_id, m.is_reply
  FROM messages m
  JOIN users u ON u.id = m.user_id
  WHERE m.dm_pair_id = $1
  ORDER BY m.created_at DESC, m.id DESC
  LIMIT 50
) sub
LEFT JOIN messages rm ON rm.id = sub.reply_to_id
LEFT JOIN users ru ON ru.id = rm.user_id
ORDER BY sub.created_at ASC, sub.id ASC;

-- name: GetDMMessagesByPair :many
SELECT sub.id, sub.dm_pair_id, sub.user_id, sub.content, sub.pinned, sub.edited_at, sub.created_at,
       sub.username, sub.display_name, sub.avatar_url,
       sub.reply_to_id, sub.is_reply, rm.content AS reply_to_content, ru.id AS reply_to_user_id, ru.username AS reply_to_username
FROM (
  SELECT m.id, m.dm_pair_id, m.user_id, m.content, m.pinned, m.edited_at, m.created_at,
         u.username, u.display_name, u.avatar_url, m.reply_to_id, m.is_reply
  FROM messages m
  JOIN users u ON u.id = m.user_id
  WHERE m.dm_pair_id = @dm_pair_id
    AND (m.created_at < @before_time OR (m.created_at = @before_time AND m.id < @before_id))
  ORDER BY m.created_at DESC, m.id DESC
  LIMIT 50
) sub
LEFT JOIN messages rm ON rm.id = sub.reply_to_id
LEFT JOIN users ru ON ru.id = rm.user_id
ORDER BY sub.created_at ASC, sub.id ASC;

-- name: SetMessagePinned :one
UPDATE messages SET pinned = $2 WHERE id = $1
RETURNING *;

-- name: GetPinnedMessagesByChannel :many
SELECT m.id, m.channel_id, m.user_id, m.content, m.pinned, m.edited_at, m.created_at,
       u.username, u.display_name, u.avatar_url,
       m.reply_to_id, m.is_reply, rm.content AS reply_to_content, ru.id AS reply_to_user_id, ru.username AS reply_to_username
FROM messages m
JOIN users u ON u.id = m.user_id
LEFT JOIN messages rm ON rm.id = m.reply_to_id
LEFT JOIN users ru ON ru.id = rm.user_id
WHERE m.channel_id = $1 AND m.pinned = true
ORDER BY m.created_at DESC;

-- name: GetPinnedDMMessages :many
SELECT m.id, m.dm_pair_id, m.user_id, m.content, m.pinned, m.edited_at, m.created_at,
       u.username, u.display_name, u.avatar_url,
       m.reply_to_id, m.is_reply, rm.content AS reply_to_content, ru.id AS reply_to_user_id, ru.username AS reply_to_username
FROM messages m
JOIN users u ON u.id = m.user_id
LEFT JOIN messages rm ON rm.id = m.reply_to_id
LEFT JOIN users ru ON ru.id = rm.user_id
WHERE m.dm_pair_id = $1 AND m.pinned = true
ORDER BY m.created_at DESC;

-- name: SearchMessages :many
SELECT m.id, m.channel_id, m.user_id, m.content, m.pinned, m.edited_at, m.created_at,
       u.username, u.display_name, u.avatar_url, c.name AS channel_name,
       m.reply_to_id, m.is_reply, rm.content AS reply_to_content, ru.id AS reply_to_user_id, ru.username AS reply_to_username
FROM messages m
JOIN users u ON u.id = m.user_id
JOIN channels c ON c.id = m.channel_id
LEFT JOIN messages rm ON rm.id = m.reply_to_id
LEFT JOIN users ru ON ru.id = rm.user_id
WHERE m.channel_id IS NOT NULL
  AND (sqlc.narg('channel_id')::uuid IS NULL OR m.channel_id = sqlc.narg('channel_id'))
  AND (sqlc.narg('author_id')::uuid IS NULL OR m.user_id = sqlc.narg('author_id'))
  AND (sqlc.narg('after_time')::timestamptz IS NULL OR m.created_at >= sqlc.narg('after_time'))
  AND (sqlc.narg('before_time')::timestamptz IS NULL OR m.created_at <= sqlc.narg('before_time'))
  AND (sqlc.narg('query')::text IS NULL OR to_tsvector('english', m.content) @@ plainto_tsquery('english', sqlc.narg('query')))
ORDER BY m.created_at DESC
LIMIT 50;

-- name: GetMessagesAroundTarget :many
SELECT sub.id, sub.channel_id, sub.user_id, sub.content, sub.pinned, sub.edited_at, sub.created_at,
       sub.username, sub.display_name, sub.avatar_url,
       sub.reply_to_id, sub.is_reply, rm.content AS reply_to_content, ru.id AS reply_to_user_id, ru.username AS reply_to_username
FROM (
  (
    SELECT m.id, m.channel_id, m.user_id, m.content, m.pinned, m.edited_at, m.created_at,
           u.username, u.display_name, u.avatar_url, m.reply_to_id, m.is_reply
    FROM messages m
    JOIN users u ON u.id = m.user_id
    WHERE m.channel_id = @channel_id
      AND (m.created_at < (SELECT t.created_at FROM messages t WHERE t.id = @target_id)
           OR (m.created_at = (SELECT t.created_at FROM messages t WHERE t.id = @target_id) AND m.id < @target_id))
    ORDER BY m.created_at DESC, m.id DESC
    LIMIT 25
  )
  UNION ALL
  (
    SELECT m.id, m.channel_id, m.user_id, m.content, m.pinned, m.edited_at, m.created_at,
           u.username, u.display_name, u.avatar_url, m.reply_to_id, m.is_reply
    FROM messages m
    JOIN users u ON u.id = m.user_id
    WHERE m.id = @target_id
  )
  UNION ALL
  (
    SELECT m.id, m.channel_id, m.user_id, m.content, m.pinned, m.edited_at, m.created_at,
           u.username, u.display_name, u.avatar_url, m.reply_to_id, m.is_reply
    FROM messages m
    JOIN users u ON u.id = m.user_id
    WHERE m.channel_id = @channel_id
      AND (m.created_at > (SELECT t.created_at FROM messages t WHERE t.id = @target_id)
           OR (m.created_at = (SELECT t.created_at FROM messages t WHERE t.id = @target_id) AND m.id > @target_id))
    ORDER BY m.created_at ASC, m.id ASC
    LIMIT 25
  )
) sub
LEFT JOIN messages rm ON rm.id = sub.reply_to_id
LEFT JOIN users ru ON ru.id = rm.user_id
ORDER BY sub.created_at ASC, sub.id ASC;

-- name: GetMessagesAfterCursor :many
SELECT m.id, m.channel_id, m.user_id, m.content, m.pinned, m.edited_at, m.created_at,
       u.username, u.display_name, u.avatar_url,
       m.reply_to_id, m.is_reply, rm.content AS reply_to_content, ru.id AS reply_to_user_id, ru.username AS reply_to_username
FROM messages m
JOIN users u ON u.id = m.user_id
LEFT JOIN messages rm ON rm.id = m.reply_to_id
LEFT JOIN users ru ON ru.id = rm.user_id
WHERE m.channel_id = @channel_id
  AND (m.created_at > @after_time OR (m.created_at = @after_time AND m.id > @after_id))
ORDER BY m.created_at ASC, m.id ASC
LIMIT 50;

-- name: GetAllChannelMessages :many
SELECT m.id, m.user_id, m.content, m.pinned, m.edited_at, m.created_at
FROM messages m
WHERE m.channel_id = $1
ORDER BY m.created_at ASC, m.id ASC;

-- name: GetAllDMMessages :many
SELECT m.id, m.user_id, m.content, m.pinned, m.edited_at, m.created_at
FROM messages m
WHERE m.dm_pair_id = $1
ORDER BY m.created_at ASC, m.id ASC;

-- name: DeleteMessagesByUserID :execrows
DELETE FROM messages WHERE user_id = $1;

-- name: CountChannels :one
SELECT count(*) FROM channels;
