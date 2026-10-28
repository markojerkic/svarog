-- name: SetArchivePolicy :execrows
INSERT INTO archive_policies (client_id, archive_after_weeks)
SELECT c.id, sqlc.arg(archive_after_weeks)
FROM clients c
WHERE c.id = sqlc.arg(client_id)
  AND c.project_id = sqlc.arg(project_id)
ON CONFLICT (client_id) DO UPDATE
SET archive_after_weeks = EXCLUDED.archive_after_weeks, updated_at = now();

-- name: GetArchivePolicy :one
SELECT COALESCE(p.archive_after_weeks, 0)::integer AS archive_after_weeks
FROM clients c
LEFT JOIN archive_policies p ON p.client_id = c.id
WHERE c.id = sqlc.arg(client_id)
  AND c.project_id = sqlc.arg(project_id);

-- name: DeleteArchivePolicy :exec
DELETE FROM archive_policies p
USING clients c
WHERE p.client_id = c.id
  AND c.id = sqlc.arg(client_id)
  AND c.project_id = sqlc.arg(project_id);

-- name: TryLockArchiveClient :one
SELECT pg_try_advisory_lock(hashtextextended('svarog:archive:' || sqlc.arg(client_id)::uuid::text, 0)) AS locked;

-- name: UnlockArchiveClient :one
SELECT pg_advisory_unlock(hashtextextended('svarog:archive:' || sqlc.arg(client_id)::uuid::text, 0)) AS unlocked;

-- name: InsertBuildingArchive :one
INSERT INTO archives (
    id, client_id, status, storage_backend, object_key, file_name, cutoff_at, claimed_at
) VALUES (
    sqlc.arg(id), sqlc.arg(client_id), 'building', sqlc.arg(storage_backend),
    sqlc.arg(object_key), sqlc.arg(file_name), sqlc.arg(cutoff_at), now()
)
RETURNING *;

-- name: ClaimArchiveLogs :execrows
UPDATE logs
SET archive_id = sqlc.arg(archive_id)
WHERE client_id = sqlc.arg(client_id)
  AND occurred_at < sqlc.arg(cutoff_at)
  AND archive_id IS NULL;

-- name: ListArchiveLogs :many
SELECT id, instance_name, occurred_at, sequence_number, message
FROM logs
WHERE client_id = sqlc.arg(client_id)
  AND archive_id = sqlc.arg(archive_id)
  AND (
    NOT sqlc.arg(has_cursor)::bool
    OR (occurred_at, sequence_number, id)
       < (sqlc.arg(cursor_at)::timestamptz, sqlc.arg(cursor_sequence)::bigint, sqlc.arg(cursor_id)::bigint)
  )
ORDER BY occurred_at DESC, sequence_number DESC, id DESC
LIMIT sqlc.arg(page_size);

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

-- name: DeleteArchivedLogs :execrows
DELETE FROM logs
WHERE client_id = sqlc.arg(client_id)
  AND archive_id = sqlc.arg(archive_id);

-- name: GetBuildingArchive :one
SELECT * FROM archives
WHERE client_id = sqlc.arg(client_id) AND status = 'building';

-- name: ReleaseArchiveLogs :exec
UPDATE logs l
SET archive_id = NULL
FROM archives a
WHERE l.archive_id = a.id
  AND l.client_id = a.client_id
  AND a.id = sqlc.arg(id)
  AND a.client_id = sqlc.arg(client_id)
  AND a.status = 'building';

-- name: DeleteBuildingArchive :execrows
DELETE FROM archives
WHERE id = sqlc.arg(id)
  AND client_id = sqlc.arg(client_id)
  AND status = 'building';

-- name: ListReadyArchives :many
SELECT a.*
FROM archives a
JOIN clients c ON c.id = a.client_id
WHERE c.project_id = sqlc.arg(project_id)
  AND a.client_id = sqlc.arg(client_id)
  AND a.status = 'ready'
  AND (
    NOT sqlc.arg(has_cursor)::bool
    OR (a.created_at, a.id) < (sqlc.arg(cursor_at)::timestamptz, sqlc.arg(cursor_id)::uuid)
  )
ORDER BY a.created_at DESC, a.id DESC
LIMIT sqlc.arg(page_size);

-- name: GetReadyArchive :one
SELECT a.*
FROM archives a
JOIN clients c ON c.id = a.client_id
WHERE c.project_id = sqlc.arg(project_id)
  AND a.client_id = sqlc.arg(client_id)
  AND a.id = sqlc.arg(id)
  AND a.status = 'ready';
