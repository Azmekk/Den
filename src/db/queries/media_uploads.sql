-- name: InsertMediaUpload :one
INSERT INTO media_uploads (uploader_id, bucket_key, content_hash, media_type, file_size)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetMediaUploadByHash :one
SELECT * FROM media_uploads WHERE content_hash = $1 AND deleted_at IS NULL LIMIT 1;

-- name: ExtendMediaUploadExpiry :exec
UPDATE media_uploads SET expires_at = NOW() + INTERVAL '24 hours' WHERE id = $1;

-- name: GetExpiredMediaUploads :many
SELECT id, bucket_key FROM media_uploads WHERE expires_at < NOW() AND deleted_at IS NULL;

-- name: SoftDeleteMediaUploadsByIDs :exec
UPDATE media_uploads SET deleted_at = NOW() WHERE id = ANY($1::uuid[]);

-- name: SoftDeleteMediaUploadByID :one
UPDATE media_uploads SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL RETURNING bucket_key;

-- name: HardDeleteMediaUploadsByIDs :exec
DELETE FROM media_uploads WHERE id = ANY($1::uuid[]);

-- name: ListActiveMediaUploads :many
SELECT m.id, m.uploader_id, u.username AS uploader_username, m.bucket_key, m.media_type, m.file_size, m.expires_at, m.created_at
FROM media_uploads m
JOIN users u ON u.id = m.uploader_id
WHERE m.deleted_at IS NULL
ORDER BY m.created_at DESC
LIMIT $1 OFFSET $2;

-- name: CountActiveMediaUploads :one
SELECT COUNT(*)::bigint FROM media_uploads WHERE deleted_at IS NULL;

-- name: ListDeletedMediaUploads :many
SELECT m.id, m.uploader_id, u.username AS uploader_username, m.bucket_key, m.media_type, m.file_size, m.expires_at, m.created_at, m.deleted_at
FROM media_uploads m
JOIN users u ON u.id = m.uploader_id
WHERE m.deleted_at IS NOT NULL
ORDER BY m.deleted_at DESC
LIMIT $1 OFFSET $2;

-- name: CountDeletedMediaUploads :one
SELECT COUNT(*)::bigint FROM media_uploads WHERE deleted_at IS NOT NULL;

-- name: GetMediaStats :one
SELECT COUNT(*)::bigint AS total_count, COALESCE(SUM(file_size), 0)::bigint AS total_size
FROM media_uploads WHERE deleted_at IS NULL;

-- name: GetMediaStatsByType :many
SELECT media_type, COUNT(*)::bigint AS count, COALESCE(SUM(file_size), 0)::bigint AS total_size
FROM media_uploads
WHERE deleted_at IS NULL
GROUP BY media_type;

