-- Drop refresh_tokens table
DROP TABLE IF EXISTS refresh_tokens;

-- Drop new auth columns
ALTER TABLE users DROP COLUMN IF EXISTS password_reset_expires_at;
ALTER TABLE users DROP COLUMN IF EXISTS password_reset_token;
ALTER TABLE users DROP COLUMN IF EXISTS recovery_codes;
ALTER TABLE users DROP COLUMN IF EXISTS totp_enabled;
ALTER TABLE users DROP COLUMN IF EXISTS totp_secret;
ALTER TABLE users DROP COLUMN IF EXISTS email_verify_expires_at;
ALTER TABLE users DROP COLUMN IF EXISTS email_verify_token;
ALTER TABLE users DROP COLUMN IF EXISTS email_verified;
ALTER TABLE users DROP COLUMN IF EXISTS email;

-- Restore password_hash as nullable
ALTER TABLE users ALTER COLUMN password_hash DROP NOT NULL;
ALTER TABLE users ALTER COLUMN password_hash SET DEFAULT '';

-- Restore Supabase columns
ALTER TABLE users ADD COLUMN supabase_id TEXT UNIQUE;
ALTER TABLE users ADD COLUMN needs_username BOOLEAN NOT NULL DEFAULT false;
