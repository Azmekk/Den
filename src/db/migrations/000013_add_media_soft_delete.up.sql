ALTER TABLE media_uploads ADD COLUMN deleted_at TIMESTAMPTZ;
CREATE INDEX media_uploads_deleted ON media_uploads(deleted_at);
