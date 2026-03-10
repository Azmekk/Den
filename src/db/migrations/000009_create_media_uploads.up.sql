CREATE TABLE media_uploads (
  id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  uploader_id  UUID NOT NULL REFERENCES users(id),
  bucket_key   TEXT NOT NULL,
  content_hash TEXT NOT NULL,
  media_type   TEXT NOT NULL CHECK (media_type IN ('image', 'video')),
  expires_at   TIMESTAMPTZ NOT NULL DEFAULT (NOW() + INTERVAL '24 hours'),
  created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX media_uploads_expires ON media_uploads(expires_at);
CREATE INDEX media_uploads_hash ON media_uploads(content_hash);
