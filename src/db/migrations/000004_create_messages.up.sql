CREATE TABLE messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    channel_id UUID REFERENCES channels(id) ON DELETE CASCADE,
    dm_pair_id UUID REFERENCES dm_pairs(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    content TEXT NOT NULL,
    pinned BOOLEAN NOT NULL DEFAULT false,
    edited_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (
        (channel_id IS NOT NULL AND dm_pair_id IS NULL) OR
        (channel_id IS NULL AND dm_pair_id IS NOT NULL)
    )
);

CREATE INDEX idx_messages_channel_id ON messages(channel_id, created_at);
CREATE INDEX idx_messages_dm_pair_id ON messages(dm_pair_id, created_at);
CREATE INDEX idx_messages_user_id ON messages(user_id);
CREATE INDEX idx_messages_content_gin ON messages USING GIN (to_tsvector('english', content));

CREATE VIEW pinned_messages AS
    SELECT * FROM messages WHERE pinned = true;
