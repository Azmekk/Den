CREATE TABLE admin_settings (
    id INTEGER PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    open_registration BOOLEAN NOT NULL DEFAULT true,
    instance_name TEXT NOT NULL DEFAULT 'Den',
    max_messages INTEGER NOT NULL DEFAULT 100000,
    max_message_chars INTEGER NOT NULL DEFAULT 2000,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
INSERT INTO admin_settings (id) VALUES (1);
