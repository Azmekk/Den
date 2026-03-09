-- name: UpsertChannelRead :exec
INSERT INTO channel_reads (user_id, channel_id, last_read_at)
VALUES ($1, $2, NOW())
ON CONFLICT (user_id, channel_id) DO UPDATE SET last_read_at = NOW();

-- name: GetUnreadCounts :many
SELECT
  c.id AS channel_id,
  COUNT(m.id)::int AS unread_count,
  COUNT(mm.user_id)::int AS mention_count
FROM channels c
LEFT JOIN channel_reads cr ON cr.channel_id = c.id AND cr.user_id = $1
LEFT JOIN messages m ON m.channel_id = c.id AND m.created_at > COALESCE(cr.last_read_at, '1970-01-01'::timestamptz)
LEFT JOIN message_mentions mm ON mm.message_id = m.id AND mm.user_id = $1
WHERE c.is_voice = false
GROUP BY c.id;
