-- name: InsertMention :exec
INSERT INTO message_mentions (message_id, user_id) VALUES ($1, $2);

-- name: DeleteMentionsByMessage :exec
DELETE FROM message_mentions WHERE message_id = $1;
