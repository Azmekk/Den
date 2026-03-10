-- name: GetAdminSettings :one
SELECT id, open_registration, instance_name, max_messages, max_message_chars, updated_at
FROM admin_settings
WHERE id = 1;

-- name: UpdateAdminSettings :exec
UPDATE admin_settings
SET open_registration = $1,
    instance_name = $2,
    max_messages = $3,
    max_message_chars = $4,
    updated_at = now()
WHERE id = 1;
