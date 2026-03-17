-- Add supabase_id to users (nullable initially for migration, then NOT NULL)
ALTER TABLE users ADD COLUMN supabase_id TEXT UNIQUE;

-- Drop password_hash (no longer needed with Supabase Auth)
ALTER TABLE users ALTER COLUMN password_hash DROP NOT NULL;
ALTER TABLE users ALTER COLUMN password_hash SET DEFAULT '';

-- Drop refresh_tokens table (Supabase handles sessions)
DROP TABLE IF EXISTS refresh_tokens;

-- Drop invite_codes table (Supabase handles registration policies)
DROP TABLE IF EXISTS invite_codes;
