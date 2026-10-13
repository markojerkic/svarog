-- name: CreateSession :one
INSERT INTO sessions (id, user_id, expires_at)
VALUES ($1, $2, $3)
RETURNING *;

-- name: TouchSession :one
UPDATE sessions
SET last_seen_at = now(), expires_at = now() + interval '24 hours'
WHERE id = $1 AND expires_at > now()
RETURNING *;

-- name: UpdateSession :one
UPDATE sessions
SET last_seen_at = now(), expires_at = now() + interval '24 hours'
WHERE id = $1 AND user_id = $2 AND expires_at > now()
RETURNING id;

-- name: DeleteSession :exec
DELETE FROM sessions WHERE id = $1;

-- name: DeleteExpiredSessions :exec
DELETE FROM sessions WHERE expires_at <= now();
