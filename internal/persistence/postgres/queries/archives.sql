-- name: CreateArchivePolicy :execrows
INSERT INTO archive_policies (client_id, archive_after_weeks)
SELECT c.id, sqlc.arg(archive_after_weeks)
FROM clients c
WHERE c.id = sqlc.arg(client_id)
  AND c.project_id = sqlc.arg(project_id);

-- name: GetArchivePolicy :one
SELECT c.project_id, p.client_id, p.archive_after_weeks, p.updated_at
FROM archive_policies p
JOIN clients c ON c.id = p.client_id
WHERE p.client_id = sqlc.arg(client_id)
  AND c.project_id = sqlc.arg(project_id);

-- name: UpdateArchivePolicy :execrows
UPDATE archive_policies
SET archive_after_weeks = sqlc.arg(archive_after_weeks), updated_at = now()
WHERE client_id = sqlc.arg(client_id);

-- name: DeleteArchivePolicy :execrows
DELETE FROM archive_policies
WHERE client_id = sqlc.arg(client_id);
