-- name: CreateEmote :one
INSERT INTO custom_emotes (name, filename, uploaded_by)
VALUES ($1, $2, $3)
RETURNING id, name, filename, uploaded_by, created_at;

-- name: GetEmoteByID :one
SELECT id, name, filename, uploaded_by, created_at
FROM custom_emotes
WHERE id = $1;

-- name: GetEmoteByName :one
SELECT id, name, filename, uploaded_by, created_at
FROM custom_emotes
WHERE name = $1;

-- name: ListEmotes :many
SELECT id, name, filename, uploaded_by, created_at
FROM custom_emotes
ORDER BY name;

-- name: DeleteEmote :one
DELETE FROM custom_emotes
WHERE id = $1
RETURNING filename;

-- name: GetEmotesByNames :many
SELECT id, name, filename, uploaded_by, created_at
FROM custom_emotes
WHERE name = ANY($1::text[]);
