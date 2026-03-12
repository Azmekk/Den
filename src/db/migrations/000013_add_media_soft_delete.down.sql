DROP INDEX IF EXISTS media_uploads_deleted;
ALTER TABLE media_uploads DROP COLUMN IF EXISTS deleted_at;
