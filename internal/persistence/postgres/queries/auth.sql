-- name: CreateUser :one
INSERT INTO users (
    id, username, first_name, last_name, password_hash, role, needs_password_reset
) VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1;

-- name: GetUserByUsername :one
SELECT * FROM users WHERE username = $1;

-- name: ListUsers :many
SELECT id, username, first_name, last_name, role, needs_password_reset, created_at, updated_at
FROM users
WHERE strpos(lower(username), lower(sqlc.arg(search)::text)) > 0
   OR strpos(lower(first_name), lower(sqlc.arg(search)::text)) > 0
   OR strpos(lower(last_name), lower(sqlc.arg(search)::text)) > 0
ORDER BY lower(username), id
LIMIT sqlc.arg(page_size) OFFSET sqlc.arg(page_offset);

-- name: CountUsers :one
SELECT count(*) FROM users
WHERE strpos(lower(username), lower(sqlc.arg(search)::text)) > 0
   OR strpos(lower(first_name), lower(sqlc.arg(search)::text)) > 0
   OR strpos(lower(last_name), lower(sqlc.arg(search)::text)) > 0;

-- name: UpdateUserProfile :one
UPDATE users
SET username = $2, first_name = $3, last_name = $4, role = $5, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: UpdateUserPassword :one
UPDATE users
SET password_hash = $2, needs_password_reset = $3, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: DeleteUser :one
DELETE FROM users WHERE id = $1 RETURNING id;

-- name: UpsertLoginToken :exec
INSERT INTO login_tokens (token_hash, user_id, expires_at)
VALUES ($1, $2, $3)
ON CONFLICT (user_id) DO UPDATE
SET token_hash = EXCLUDED.token_hash,
    created_at = now(),
    expires_at = EXCLUDED.expires_at;

-- name: ConsumeLoginToken :one
DELETE FROM login_tokens
WHERE token_hash = $1 AND expires_at > now()
RETURNING user_id;
