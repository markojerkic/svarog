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

-- name: InsertBuildingArchive :exec
INSERT INTO archives (
    id, client_id, status, storage_backend, object_key, file_name, cutoff_at
) VALUES (
    sqlc.arg(id), sqlc.arg(client_id), 'building', 'disk',
    sqlc.arg(object_key), sqlc.arg(file_name), sqlc.arg(cutoff_at)
);

-- name: MarkArchiveReady :execrows
UPDATE archives
SET status = 'ready',
    size_bytes = sqlc.arg(size_bytes)::bigint,
    sha256 = sqlc.arg(sha256)::bytea,
    log_count = sqlc.arg(log_count)::bigint,
    from_at = sqlc.arg(from_at)::timestamptz,
    to_at = sqlc.arg(to_at)::timestamptz,
    updated_at = now()
WHERE id = sqlc.arg(id)
  AND client_id = sqlc.arg(client_id)
  AND status = 'building';

-- name: DeleteBuildingArchive :exec
DELETE FROM archives
WHERE id = sqlc.arg(id) AND status = 'building';

-- name: DeleteClientLogsBefore :execrows
DELETE FROM logs
WHERE client_id = sqlc.arg(client_id)
  AND occurred_at < sqlc.arg(cutoff_at);
