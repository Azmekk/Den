-- Clean break: truncate all user-related data for fresh start
TRUNCATE messages, channel_reads, dm_pairs, media_uploads, invite_codes, users CASCADE;

-- Drop Supabase-specific columns
ALTER TABLE users DROP COLUMN IF EXISTS supabase_id;
ALTER TABLE users DROP COLUMN IF EXISTS needs_username;

-- Restore password_hash as required
ALTER TABLE users ALTER COLUMN password_hash SET NOT NULL;
ALTER TABLE users ALTER COLUMN password_hash DROP DEFAULT;

-- Add email column
ALTER TABLE users ADD COLUMN email TEXT UNIQUE NOT NULL;

-- Add email verification columns
ALTER TABLE users ADD COLUMN email_verified BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE users ADD COLUMN email_verify_token TEXT;
ALTER TABLE users ADD COLUMN email_verify_expires_at TIMESTAMPTZ;

-- Add TOTP 2FA columns
ALTER TABLE users ADD COLUMN totp_secret TEXT;
ALTER TABLE users ADD COLUMN totp_enabled BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE users ADD COLUMN recovery_codes TEXT[];

-- Add password reset columns
ALTER TABLE users ADD COLUMN password_reset_token TEXT;
ALTER TABLE users ADD COLUMN password_reset_expires_at TIMESTAMPTZ;

-- Recreate refresh_tokens table (was dropped in 000014)
CREATE TABLE refresh_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_refresh_tokens_user_id ON refresh_tokens(user_id);
CREATE INDEX idx_refresh_tokens_expires_at ON refresh_tokens(expires_at);
