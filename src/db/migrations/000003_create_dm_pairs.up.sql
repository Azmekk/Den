CREATE TABLE dm_pairs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_a UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    user_b UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_a, user_b),
    CHECK (user_a < user_b)
);

CREATE INDEX idx_dm_pairs_user_a ON dm_pairs(user_a);
CREATE INDEX idx_dm_pairs_user_b ON dm_pairs(user_b);
