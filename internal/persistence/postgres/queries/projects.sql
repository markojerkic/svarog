-- name: CreateProject :one
INSERT INTO projects (id, name) VALUES ($1, $2) RETURNING *;

-- name: GetProjectByID :one
SELECT * FROM projects WHERE id = $1;

-- name: GetProjectByName :one
SELECT * FROM projects WHERE name = $1;

-- name: GetProjectsByIDs :many
SELECT * FROM projects WHERE id = ANY(sqlc.arg(ids)::uuid[])
ORDER BY lower(name), id;

-- name: ListProjects :many
SELECT * FROM projects
WHERE strpos(lower(name), lower(sqlc.arg(search)::text)) > 0
ORDER BY lower(name), id
LIMIT sqlc.arg(page_size) OFFSET sqlc.arg(page_offset);

-- name: CountProjects :one
SELECT count(*) FROM projects
WHERE strpos(lower(name), lower(sqlc.arg(search)::text)) > 0;

-- name: UpdateProject :one
UPDATE projects SET name = $2, updated_at = now() WHERE id = $1 RETURNING *;

-- name: DeleteProject :one
DELETE FROM projects WHERE id = $1 RETURNING id;

-- name: CreateClient :one
INSERT INTO clients (id, project_id, name) VALUES ($1, $2, $3) RETURNING *;

-- name: ActivateClientByName :one
INSERT INTO clients (id, project_id, name) VALUES ($1, $2, $3)
ON CONFLICT (project_id, name) DO UPDATE SET is_active = true, updated_at = now()
RETURNING *;

-- name: GetClientByID :one
SELECT * FROM clients WHERE id = $1;

-- name: ListProjectClients :many
SELECT * FROM clients WHERE project_id = $1 ORDER BY lower(name), id;

-- name: ListActiveProjectClients :many
SELECT * FROM clients WHERE project_id = $1 AND is_active ORDER BY lower(name), id;

-- name: DeactivateClient :one
UPDATE clients SET is_active = false, updated_at = now()
WHERE id = $1 AND project_id = $2 RETURNING *;

-- name: DeactivateUnlistedClients :exec
UPDATE clients SET is_active = false, updated_at = now()
WHERE project_id = $1 AND is_active AND NOT (name = ANY(sqlc.arg(names)::text[]));

-- name: ActiveProjectClientExists :one
SELECT EXISTS(SELECT 1 FROM clients WHERE project_id = $1 AND id = $2 AND is_active);

-- name: AddUserToProject :exec
INSERT INTO user_projects (user_id, project_id) VALUES ($1, $2)
ON CONFLICT DO NOTHING;

-- name: RemoveUserFromProject :exec
DELETE FROM user_projects WHERE user_id = $1 AND project_id = $2;

-- name: DeleteUserProjects :exec
DELETE FROM user_projects WHERE user_id = $1;

-- name: LockUserForProjectAssignment :one
SELECT id FROM users WHERE id = $1 FOR UPDATE;

-- name: ListUserProjects :many
SELECT p.* FROM projects p JOIN user_projects up ON up.project_id = p.id
WHERE up.user_id = $1 ORDER BY lower(p.name), p.id;

-- name: UserHasProjectAccess :one
SELECT EXISTS(
    SELECT 1 FROM users u JOIN projects p ON p.id = sqlc.arg(project_id)
    WHERE u.id = sqlc.arg(user_id) AND (u.role = 'admin' OR EXISTS(
        SELECT 1 FROM user_projects up WHERE up.user_id = u.id AND up.project_id = p.id
    ))
);
