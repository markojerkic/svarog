-- name: CreateOAuthGrant :exec
INSERT INTO oauth_grants (
    id, user_id, client_id, redirect_uri, resource, scope,
    code_hash, code_challenge, code_expires_at, expires_at
) VALUES (
    sqlc.arg(id), sqlc.arg(user_id), sqlc.arg(client_id), sqlc.arg(redirect_uri),
    sqlc.arg(resource), sqlc.arg(scope), sqlc.arg(code_hash), sqlc.arg(code_challenge),
    sqlc.arg(code_expires_at), sqlc.arg(code_expires_at)
);

-- name: DeleteExpiredOAuthGrants :exec
DELETE FROM oauth_grants WHERE expires_at <= now();

-- name: GetOAuthGrantByCode :one
SELECT id, client_id, redirect_uri, resource, scope, code_challenge, code_expires_at,
       (refresh_token_hash IS NOT NULL)::boolean AS exchanged
FROM oauth_grants
WHERE code_hash = $1;

-- name: IssueOAuthGrantTokens :execrows
-- A concurrent exchange of the same code updates no row.
UPDATE oauth_grants
SET access_token_hash = sqlc.arg(access_token_hash),
    access_expires_at = sqlc.arg(access_expires_at)::timestamptz,
    refresh_token_hash = sqlc.arg(refresh_token_hash),
    expires_at = sqlc.arg(expires_at)
WHERE id = sqlc.arg(id)
  AND refresh_token_hash IS NULL
  AND code_expires_at > now();

-- name: GetOAuthGrantForRefresh :one
SELECT id, client_id, resource, scope, expires_at,
       (refresh_token_hash IS NOT DISTINCT FROM sqlc.arg(token_hash)::bytea)::boolean AS is_current
FROM oauth_grants
WHERE id = sqlc.arg(id);

-- name: RotateOAuthGrantTokens :execrows
-- A concurrent refresh with the same token updates no row.
UPDATE oauth_grants
SET refresh_token_hash = sqlc.arg(new_refresh_token_hash),
    access_token_hash = sqlc.arg(access_token_hash),
    access_expires_at = sqlc.arg(access_expires_at)::timestamptz,
    expires_at = sqlc.arg(expires_at)
WHERE id = sqlc.arg(id)
  AND refresh_token_hash = sqlc.arg(refresh_token_hash)
  AND expires_at > now();

-- name: GetOAuthAccessGrant :one
SELECT g.id, g.user_id, g.resource, g.scope,
       g.access_expires_at::timestamptz AS access_expires_at,
       u.needs_password_reset
FROM oauth_grants g
JOIN users u ON u.id = g.user_id
WHERE g.access_token_hash = $1
  AND g.access_expires_at > now();

-- name: DeleteOAuthGrant :exec
DELETE FROM oauth_grants WHERE id = $1;

-- name: DeleteOAuthGrantsForUser :exec
DELETE FROM oauth_grants WHERE user_id = $1;
