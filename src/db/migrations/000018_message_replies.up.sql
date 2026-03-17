ALTER TABLE messages ADD COLUMN reply_to_id UUID REFERENCES messages(id) ON DELETE SET NULL;
ALTER TABLE messages ADD COLUMN is_reply BOOLEAN NOT NULL DEFAULT false;
CREATE INDEX idx_messages_reply_to_id ON messages(reply_to_id);
