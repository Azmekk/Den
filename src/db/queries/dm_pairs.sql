-- name: CreateDMPair :one
INSERT INTO dm_pairs (user_a, user_b)
VALUES (LEAST(@user_a::uuid, @user_b::uuid), GREATEST(@user_a::uuid, @user_b::uuid))
ON CONFLICT (user_a, user_b) DO UPDATE SET user_a = dm_pairs.user_a
RETURNING *;

-- name: GetDMPair :one
SELECT * FROM dm_pairs
WHERE id = $1;

-- name: GetDMPairByUsers :one
SELECT * FROM dm_pairs
WHERE user_a = LEAST(@user_a::uuid, @user_b::uuid) AND user_b = GREATEST(@user_a::uuid, @user_b::uuid);

-- name: ListDMPairsForUser :many
SELECT dp.id, dp.user_a, dp.user_b, dp.created_at,
       u.id AS other_user_id, u.username AS other_username,
       u.display_name AS other_display_name, u.avatar_url AS other_avatar_url
FROM dm_pairs dp
JOIN users u ON u.id = CASE WHEN dp.user_a = $1 THEN dp.user_b ELSE dp.user_a END
WHERE dp.user_a = $1 OR dp.user_b = $1
ORDER BY dp.created_at DESC;
