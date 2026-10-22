-- name: InsertLogs :many
WITH input AS (
    SELECT
        nextval(pg_get_serial_sequence('logs', 'id')) AS id,
        c.client_id,
        i.instance_name,
        o.occurred_at,
        s.sequence_number,
        m.message,
        c.ordinality
    FROM unnest(sqlc.arg(client_ids)::uuid[]) WITH ORDINALITY AS c(client_id, ordinality)
    JOIN unnest(sqlc.arg(instance_names)::text[]) WITH ORDINALITY AS i(instance_name, ordinality) USING (ordinality)
    JOIN unnest(sqlc.arg(occurred_at)::timestamptz[]) WITH ORDINALITY AS o(occurred_at, ordinality) USING (ordinality)
    JOIN unnest(sqlc.arg(sequence_numbers)::bigint[]) WITH ORDINALITY AS s(sequence_number, ordinality) USING (ordinality)
    JOIN unnest(sqlc.arg(messages)::text[]) WITH ORDINALITY AS m(message, ordinality) USING (ordinality)
),
inserted AS (
    INSERT INTO logs (
        id, client_id, instance_name, occurred_at, sequence_number, message
    )
    OVERRIDING SYSTEM VALUE
    SELECT id, client_id, instance_name, occurred_at, sequence_number, message
    FROM input
    RETURNING id
)
SELECT input.id
FROM input
JOIN inserted ON inserted.id = input.id
ORDER BY input.ordinality;

-- name: ListLogs :many
SELECT l.id, l.client_id, c.project_id, l.instance_name,
       l.occurred_at, l.sequence_number, l.message
FROM logs l
JOIN clients c ON c.id = l.client_id
WHERE c.project_id = sqlc.arg(project_id)
  AND l.client_id = sqlc.arg(client_id)
  AND (
    NOT sqlc.arg(filter_instances)::bool
    OR l.instance_name = ANY(sqlc.arg(instances)::text[])
  )
  AND (
    NOT sqlc.arg(has_cursor)::bool
    OR (
      sqlc.arg(backward)::bool
      AND sqlc.arg(inclusive)::bool
      AND sqlc.arg(has_cursor_id)::bool
      AND (l.occurred_at, l.sequence_number, l.id)
          <= (sqlc.arg(cursor_at), sqlc.arg(cursor_sequence)::bigint, sqlc.arg(cursor_id)::bigint)
    )
    OR (
      sqlc.arg(backward)::bool
      AND NOT sqlc.arg(inclusive)::bool
      AND NOT sqlc.arg(has_cursor_id)::bool
      AND (l.occurred_at, l.sequence_number)
          < (sqlc.arg(cursor_at), sqlc.arg(cursor_sequence)::bigint)
    )
    OR (
      sqlc.arg(backward)::bool
      AND NOT sqlc.arg(inclusive)::bool
      AND sqlc.arg(has_cursor_id)::bool
      AND (l.occurred_at, l.sequence_number, l.id)
          < (sqlc.arg(cursor_at), sqlc.arg(cursor_sequence)::bigint, sqlc.arg(cursor_id)::bigint)
    )
    OR (
      NOT sqlc.arg(backward)::bool
      AND NOT sqlc.arg(has_cursor_id)::bool
      AND (l.occurred_at, l.sequence_number)
          > (sqlc.arg(cursor_at), sqlc.arg(cursor_sequence)::bigint)
    )
    OR (
      NOT sqlc.arg(backward)::bool
      AND sqlc.arg(has_cursor_id)::bool
      AND (l.occurred_at, l.sequence_number, l.id)
          > (sqlc.arg(cursor_at), sqlc.arg(cursor_sequence)::bigint, sqlc.arg(cursor_id)::bigint)
    )
  )
ORDER BY
  CASE WHEN sqlc.arg(has_cursor)::bool AND NOT sqlc.arg(backward)::bool THEN l.occurred_at END ASC,
  CASE WHEN sqlc.arg(has_cursor)::bool AND NOT sqlc.arg(backward)::bool THEN l.sequence_number END ASC,
  CASE WHEN sqlc.arg(has_cursor)::bool AND NOT sqlc.arg(backward)::bool THEN l.id END ASC,
  l.occurred_at DESC, l.sequence_number DESC, l.id DESC
LIMIT sqlc.arg(page_size);

-- name: SearchLogs :many
SELECT l.id, l.client_id, c.project_id, l.instance_name,
       l.occurred_at, l.sequence_number, l.message
FROM logs l
JOIN clients c ON c.id = l.client_id
WHERE c.project_id = sqlc.arg(project_id)
  AND l.client_id = sqlc.arg(client_id)
  AND to_tsvector('simple', l.message) @@ plainto_tsquery('simple', sqlc.arg(search))
  AND (
    NOT sqlc.arg(filter_instances)::bool
    OR l.instance_name = ANY(sqlc.arg(instances)::text[])
  )
  AND (
    NOT sqlc.arg(has_cursor)::bool
    OR (
      sqlc.arg(backward)::bool
      AND sqlc.arg(inclusive)::bool
      AND sqlc.arg(has_cursor_id)::bool
      AND (l.occurred_at, l.sequence_number, l.id)
          <= (sqlc.arg(cursor_at), sqlc.arg(cursor_sequence)::bigint, sqlc.arg(cursor_id)::bigint)
    )
    OR (
      sqlc.arg(backward)::bool
      AND NOT sqlc.arg(inclusive)::bool
      AND NOT sqlc.arg(has_cursor_id)::bool
      AND (l.occurred_at, l.sequence_number)
          < (sqlc.arg(cursor_at), sqlc.arg(cursor_sequence)::bigint)
    )
    OR (
      sqlc.arg(backward)::bool
      AND NOT sqlc.arg(inclusive)::bool
      AND sqlc.arg(has_cursor_id)::bool
      AND (l.occurred_at, l.sequence_number, l.id)
          < (sqlc.arg(cursor_at), sqlc.arg(cursor_sequence)::bigint, sqlc.arg(cursor_id)::bigint)
    )
    OR (
      NOT sqlc.arg(backward)::bool
      AND NOT sqlc.arg(has_cursor_id)::bool
      AND (l.occurred_at, l.sequence_number)
          > (sqlc.arg(cursor_at), sqlc.arg(cursor_sequence)::bigint)
    )
    OR (
      NOT sqlc.arg(backward)::bool
      AND sqlc.arg(has_cursor_id)::bool
      AND (l.occurred_at, l.sequence_number, l.id)
          > (sqlc.arg(cursor_at), sqlc.arg(cursor_sequence)::bigint, sqlc.arg(cursor_id)::bigint)
    )
  )
ORDER BY
  CASE WHEN sqlc.arg(has_cursor)::bool AND NOT sqlc.arg(backward)::bool THEN l.occurred_at END ASC,
  CASE WHEN sqlc.arg(has_cursor)::bool AND NOT sqlc.arg(backward)::bool THEN l.sequence_number END ASC,
  CASE WHEN sqlc.arg(has_cursor)::bool AND NOT sqlc.arg(backward)::bool THEN l.id END ASC,
  l.occurred_at DESC, l.sequence_number DESC, l.id DESC
LIMIT sqlc.arg(page_size);

-- name: GetLogAnchor :one
SELECT l.id, l.client_id, c.project_id, l.instance_name,
       l.occurred_at, l.sequence_number, l.message
FROM logs l
JOIN clients c ON c.id = l.client_id
WHERE l.id = sqlc.arg(id)
  AND l.client_id = sqlc.arg(client_id)
  AND c.project_id = sqlc.arg(project_id);

-- name: ListInstances :many
SELECT DISTINCT l.instance_name
FROM logs l
JOIN clients c ON c.id = l.client_id
WHERE c.project_id = sqlc.arg(project_id)
  AND l.client_id = sqlc.arg(client_id)
ORDER BY l.instance_name;
