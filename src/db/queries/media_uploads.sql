-- name: InsertMediaUpload :one
INSERT INTO media_uploads (uploader_id, bucket_key, content_hash, media_type)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetMediaUploadByHash :one
SELECT * FROM media_uploads WHERE content_hash = $1 LIMIT 1;

-- name: ExtendMediaUploadExpiry :exec
UPDATE media_uploads SET expires_at = NOW() + INTERVAL '24 hours' WHERE id = $1;

-- name: GetExpiredMediaUploads :many
SELECT id, bucket_key FROM media_uploads WHERE expires_at < NOW();

-- name: DeleteMediaUploadsByIDs :exec
DELETE FROM media_uploads WHERE id = ANY($1::uuid[]);
