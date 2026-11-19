-- name: InsertIngestLogs :many
WITH input AS (
    SELECT c.client_id, p.project_id, i.instance_name, o.occurred_at,
           s.sequence_number, m.message, k.event_key, c.ordinality
    FROM unnest(sqlc.arg(client_ids)::uuid[]) WITH ORDINALITY AS c(client_id, ordinality)
    JOIN unnest(sqlc.arg(project_ids)::uuid[]) WITH ORDINALITY AS p(project_id, ordinality) USING (ordinality)
    JOIN unnest(sqlc.arg(instance_names)::text[]) WITH ORDINALITY AS i(instance_name, ordinality) USING (ordinality)
    JOIN unnest(sqlc.arg(occurred_at)::timestamptz[]) WITH ORDINALITY AS o(occurred_at, ordinality) USING (ordinality)
    JOIN unnest(sqlc.arg(sequence_numbers)::bigint[]) WITH ORDINALITY AS s(sequence_number, ordinality) USING (ordinality)
    JOIN unnest(sqlc.arg(messages)::text[]) WITH ORDINALITY AS m(message, ordinality) USING (ordinality)
    JOIN unnest(sqlc.arg(event_keys)::bytea[]) WITH ORDINALITY AS k(event_key, ordinality) USING (ordinality)
), scoped_input AS (
    SELECT i.*, c.id IS NOT NULL AS active_client
    FROM input i
    LEFT JOIN clients c ON c.id = i.client_id AND c.project_id = i.project_id AND c.is_active
), unique_input AS (
    SELECT DISTINCT ON (client_id, event_key) *
    FROM scoped_input
    WHERE active_client
    ORDER BY client_id, event_key, ordinality
), new_input AS (
    SELECT nextval(pg_get_serial_sequence('logs', 'id')) AS id, i.*
    FROM unique_input i
), inserted AS (
    INSERT INTO logs (
        id, client_id, instance_name, occurred_at, sequence_number, message, event_key
    ) OVERRIDING SYSTEM VALUE
    SELECT id, client_id, instance_name, occurred_at, sequence_number, message, event_key
    FROM new_input
    ORDER BY client_id, event_key
    ON CONFLICT (client_id, event_key) WHERE event_key IS NOT NULL DO NOTHING
    RETURNING id
)
SELECT s.ordinality::bigint AS ordinal, COALESCE(inserted.id, 0)::bigint AS id,
       s.active_client::boolean AS active_client
FROM scoped_input s
LEFT JOIN new_input n ON n.ordinality = s.ordinality
LEFT JOIN inserted ON inserted.id = n.id
ORDER BY s.ordinality;

-- name: ListLogsOlder :many
-- The Go store intersects the cursor and time range before binding these keys.
SELECT l.id, l.client_id, c.project_id, l.instance_name,
       l.occurred_at, l.sequence_number, l.message
FROM logs l
JOIN clients c ON c.id = l.client_id
WHERE c.project_id = sqlc.arg(project_id)
  AND l.client_id = sqlc.arg(client_id)
  AND (l.occurred_at, l.sequence_number, l.id) >
      (COALESCE(sqlc.narg(after_at)::timestamptz, '-infinity'::timestamptz),
       sqlc.arg(after_sequence)::bigint, sqlc.arg(after_id)::bigint)
  AND (l.occurred_at, l.sequence_number, l.id) <=
      (COALESCE(sqlc.narg(through_at)::timestamptz, 'infinity'::timestamptz),
       sqlc.arg(through_sequence)::bigint, sqlc.arg(through_id)::bigint)
ORDER BY l.occurred_at DESC, l.sequence_number DESC, l.id DESC
LIMIT sqlc.arg(page_size);

-- name: ListLogsNewer :many
SELECT l.id, l.client_id, c.project_id, l.instance_name,
       l.occurred_at, l.sequence_number, l.message
FROM logs l
JOIN clients c ON c.id = l.client_id
WHERE c.project_id = sqlc.arg(project_id)
  AND l.client_id = sqlc.arg(client_id)
  AND (l.occurred_at, l.sequence_number, l.id) >
      (COALESCE(sqlc.narg(after_at)::timestamptz, '-infinity'::timestamptz),
       sqlc.arg(after_sequence)::bigint, sqlc.arg(after_id)::bigint)
  AND (l.occurred_at, l.sequence_number, l.id) <=
      (COALESCE(sqlc.narg(through_at)::timestamptz, 'infinity'::timestamptz),
       sqlc.arg(through_sequence)::bigint, sqlc.arg(through_id)::bigint)
ORDER BY l.occurred_at ASC, l.sequence_number ASC, l.id ASC
LIMIT sqlc.arg(page_size);

-- name: ListLogsOlderByInstances :many
-- Take one page of keys per distinct instance, then fetch the merged page's rows.
-- A bounded scan of the client index can find the last key of the merged page.
-- If it falls short, each instance scan uses the original bounds and page limit.
WITH client_scope AS (
    SELECT c.id, c.project_id
    FROM clients c
    WHERE c.project_id = sqlc.arg(project_id) AND c.id = sqlc.arg(client_id)
), page_cutoff AS (
    SELECT prefix.occurred_at, prefix.sequence_number, prefix.id
    FROM (
        SELECT l.occurred_at, l.sequence_number, l.id, l.instance_name
        FROM logs l
        WHERE l.client_id = (SELECT c.id FROM client_scope c)
          AND (l.occurred_at, l.sequence_number, l.id) >
              (COALESCE(sqlc.narg(after_at)::timestamptz, '-infinity'::timestamptz),
               sqlc.arg(after_sequence)::bigint, sqlc.arg(after_id)::bigint)
          AND (l.occurred_at, l.sequence_number, l.id) <=
              (COALESCE(sqlc.narg(through_at)::timestamptz, 'infinity'::timestamptz),
               sqlc.arg(through_sequence)::bigint, sqlc.arg(through_id)::bigint)
        ORDER BY l.occurred_at DESC, l.sequence_number DESC, l.id DESC
        LIMIT 2 * sqlc.arg(page_size)::int::bigint
    ) prefix
    WHERE prefix.instance_name = ANY(sqlc.arg(instances)::text[])
    ORDER BY prefix.occurred_at DESC, prefix.sequence_number DESC, prefix.id DESC
    OFFSET (sqlc.arg(page_size)::int - 1) LIMIT 1
)
SELECT l.id, l.client_id, page.project_id, l.instance_name,
       l.occurred_at, l.sequence_number, l.message
FROM (
    SELECT candidate.id, c.project_id, candidate.occurred_at, candidate.sequence_number
    FROM client_scope c
    LEFT JOIN page_cutoff cutoff ON true
    CROSS JOIN (
        SELECT DISTINCT instance_name
        FROM unnest(sqlc.arg(instances)::text[]) AS selected(instance_name)
    ) i
    CROSS JOIN LATERAL (
        SELECT l.id, l.occurred_at, l.sequence_number
        FROM logs l
        WHERE l.client_id = c.id
          AND l.instance_name = i.instance_name
          -- Without a cutoff, the lower time bound has the minimum sequence.
          AND (l.occurred_at, l.sequence_number, l.id) >=
              (COALESCE(cutoff.occurred_at, sqlc.narg(after_at)::timestamptz, '-infinity'::timestamptz),
               COALESCE(cutoff.sequence_number, sqlc.arg(after_sequence)::bigint),
               COALESCE(cutoff.id, sqlc.arg(after_id)::bigint))
          AND (l.occurred_at, l.sequence_number, l.id) <=
              (COALESCE(sqlc.narg(through_at)::timestamptz, 'infinity'::timestamptz),
               sqlc.arg(through_sequence)::bigint, sqlc.arg(through_id)::bigint)
        ORDER BY l.occurred_at DESC, l.sequence_number DESC, l.id DESC
        LIMIT sqlc.arg(page_size)
    ) candidate
    ORDER BY candidate.occurred_at DESC, candidate.sequence_number DESC, candidate.id DESC
    LIMIT sqlc.arg(page_size)
) page
JOIN logs l ON l.id = page.id
ORDER BY page.occurred_at DESC, page.sequence_number DESC, page.id DESC;

-- name: ListLogsNewerByInstances :many
WITH client_scope AS (
    SELECT c.id, c.project_id
    FROM clients c
    WHERE c.project_id = sqlc.arg(project_id) AND c.id = sqlc.arg(client_id)
), page_cutoff AS (
    SELECT prefix.occurred_at, prefix.sequence_number, prefix.id
    FROM (
        SELECT l.occurred_at, l.sequence_number, l.id, l.instance_name
        FROM logs l
        WHERE l.client_id = (SELECT c.id FROM client_scope c)
          AND (l.occurred_at, l.sequence_number, l.id) >
              (COALESCE(sqlc.narg(after_at)::timestamptz, '-infinity'::timestamptz),
               sqlc.arg(after_sequence)::bigint, sqlc.arg(after_id)::bigint)
          AND (l.occurred_at, l.sequence_number, l.id) <=
              (COALESCE(sqlc.narg(through_at)::timestamptz, 'infinity'::timestamptz),
               sqlc.arg(through_sequence)::bigint, sqlc.arg(through_id)::bigint)
        ORDER BY l.occurred_at ASC, l.sequence_number ASC, l.id ASC
        LIMIT 2 * sqlc.arg(page_size)::int::bigint
    ) prefix
    WHERE prefix.instance_name = ANY(sqlc.arg(instances)::text[])
    ORDER BY prefix.occurred_at ASC, prefix.sequence_number ASC, prefix.id ASC
    OFFSET (sqlc.arg(page_size)::int - 1) LIMIT 1
)
SELECT l.id, l.client_id, page.project_id, l.instance_name,
       l.occurred_at, l.sequence_number, l.message
FROM (
    SELECT candidate.id, c.project_id, candidate.occurred_at, candidate.sequence_number
    FROM client_scope c
    LEFT JOIN page_cutoff cutoff ON true
    CROSS JOIN (
        SELECT DISTINCT instance_name
        FROM unnest(sqlc.arg(instances)::text[]) AS selected(instance_name)
    ) i
    CROSS JOIN LATERAL (
        SELECT l.id, l.occurred_at, l.sequence_number
        FROM logs l
        WHERE l.client_id = c.id
          AND l.instance_name = i.instance_name
          AND (l.occurred_at, l.sequence_number, l.id) >
              (COALESCE(sqlc.narg(after_at)::timestamptz, '-infinity'::timestamptz),
               sqlc.arg(after_sequence)::bigint, sqlc.arg(after_id)::bigint)
          AND (l.occurred_at, l.sequence_number, l.id) <=
              (COALESCE(cutoff.occurred_at, sqlc.narg(through_at)::timestamptz, 'infinity'::timestamptz),
               COALESCE(cutoff.sequence_number, sqlc.arg(through_sequence)::bigint),
               COALESCE(cutoff.id, sqlc.arg(through_id)::bigint))
        ORDER BY l.occurred_at ASC, l.sequence_number ASC, l.id ASC
        LIMIT sqlc.arg(page_size)
    ) candidate
    ORDER BY candidate.occurred_at ASC, candidate.sequence_number ASC, candidate.id ASC
    LIMIT sqlc.arg(page_size)
) page
JOIN logs l ON l.id = page.id
ORDER BY page.occurred_at ASC, page.sequence_number ASC, page.id ASC;

-- name: SearchLogsOlder :many
-- Repeat the time bounds so the search planner can estimate range selectivity
-- from the timestamp histogram when choosing between B-tree and trigram indexes.
SELECT l.id, l.client_id, c.project_id, l.instance_name,
       l.occurred_at, l.sequence_number, l.message
FROM logs l
JOIN clients c ON c.id = l.client_id
WHERE c.project_id = sqlc.arg(project_id)
  AND l.client_id = sqlc.arg(client_id)
  AND l.message ILIKE sqlc.arg(primary_pattern)::text ESCAPE E'\\'
  AND l.message ILIKE ALL(sqlc.arg(remaining_patterns)::text[])
  AND l.occurred_at >= COALESCE(sqlc.narg(after_at)::timestamptz, '-infinity'::timestamptz)
  AND l.occurred_at <= COALESCE(sqlc.narg(through_at)::timestamptz, 'infinity'::timestamptz)
  AND (l.occurred_at, l.sequence_number, l.id) >
      (COALESCE(sqlc.narg(after_at)::timestamptz, '-infinity'::timestamptz),
       sqlc.arg(after_sequence)::bigint, sqlc.arg(after_id)::bigint)
  AND (l.occurred_at, l.sequence_number, l.id) <=
      (COALESCE(sqlc.narg(through_at)::timestamptz, 'infinity'::timestamptz),
       sqlc.arg(through_sequence)::bigint, sqlc.arg(through_id)::bigint)
ORDER BY l.occurred_at DESC, l.sequence_number DESC, l.id DESC
LIMIT sqlc.arg(page_size);

-- name: SearchLogsNewer :many
SELECT l.id, l.client_id, c.project_id, l.instance_name,
       l.occurred_at, l.sequence_number, l.message
FROM logs l
JOIN clients c ON c.id = l.client_id
WHERE c.project_id = sqlc.arg(project_id)
  AND l.client_id = sqlc.arg(client_id)
  AND l.message ILIKE sqlc.arg(primary_pattern)::text ESCAPE E'\\'
  AND l.message ILIKE ALL(sqlc.arg(remaining_patterns)::text[])
  AND l.occurred_at >= COALESCE(sqlc.narg(after_at)::timestamptz, '-infinity'::timestamptz)
  AND l.occurred_at <= COALESCE(sqlc.narg(through_at)::timestamptz, 'infinity'::timestamptz)
  AND (l.occurred_at, l.sequence_number, l.id) >
      (COALESCE(sqlc.narg(after_at)::timestamptz, '-infinity'::timestamptz),
       sqlc.arg(after_sequence)::bigint, sqlc.arg(after_id)::bigint)
  AND (l.occurred_at, l.sequence_number, l.id) <=
      (COALESCE(sqlc.narg(through_at)::timestamptz, 'infinity'::timestamptz),
       sqlc.arg(through_sequence)::bigint, sqlc.arg(through_id)::bigint)
ORDER BY l.occurred_at ASC, l.sequence_number ASC, l.id ASC
LIMIT sqlc.arg(page_size);

-- name: SearchLogsOlderByInstances :many
SELECT l.id, l.client_id, c.project_id, l.instance_name,
       l.occurred_at, l.sequence_number, l.message
FROM logs l
JOIN clients c ON c.id = l.client_id
WHERE c.project_id = sqlc.arg(project_id)
  AND l.client_id = sqlc.arg(client_id)
  AND l.message ILIKE sqlc.arg(primary_pattern)::text ESCAPE E'\\'
  AND l.message ILIKE ALL(sqlc.arg(remaining_patterns)::text[])
  AND l.instance_name = ANY(sqlc.arg(instances)::text[])
  AND l.occurred_at >= COALESCE(sqlc.narg(after_at)::timestamptz, '-infinity'::timestamptz)
  AND l.occurred_at <= COALESCE(sqlc.narg(through_at)::timestamptz, 'infinity'::timestamptz)
  AND (l.occurred_at, l.sequence_number, l.id) >
      (COALESCE(sqlc.narg(after_at)::timestamptz, '-infinity'::timestamptz),
       sqlc.arg(after_sequence)::bigint, sqlc.arg(after_id)::bigint)
  AND (l.occurred_at, l.sequence_number, l.id) <=
      (COALESCE(sqlc.narg(through_at)::timestamptz, 'infinity'::timestamptz),
       sqlc.arg(through_sequence)::bigint, sqlc.arg(through_id)::bigint)
ORDER BY l.occurred_at DESC, l.sequence_number DESC, l.id DESC
LIMIT sqlc.arg(page_size);

-- name: SearchLogsNewerByInstances :many
SELECT l.id, l.client_id, c.project_id, l.instance_name,
       l.occurred_at, l.sequence_number, l.message
FROM logs l
JOIN clients c ON c.id = l.client_id
WHERE c.project_id = sqlc.arg(project_id)
  AND l.client_id = sqlc.arg(client_id)
  AND l.message ILIKE sqlc.arg(primary_pattern)::text ESCAPE E'\\'
  AND l.message ILIKE ALL(sqlc.arg(remaining_patterns)::text[])
  AND l.instance_name = ANY(sqlc.arg(instances)::text[])
  AND l.occurred_at >= COALESCE(sqlc.narg(after_at)::timestamptz, '-infinity'::timestamptz)
  AND l.occurred_at <= COALESCE(sqlc.narg(through_at)::timestamptz, 'infinity'::timestamptz)
  AND (l.occurred_at, l.sequence_number, l.id) >
      (COALESCE(sqlc.narg(after_at)::timestamptz, '-infinity'::timestamptz),
       sqlc.arg(after_sequence)::bigint, sqlc.arg(after_id)::bigint)
  AND (l.occurred_at, l.sequence_number, l.id) <=
      (COALESCE(sqlc.narg(through_at)::timestamptz, 'infinity'::timestamptz),
       sqlc.arg(through_sequence)::bigint, sqlc.arg(through_id)::bigint)
ORDER BY l.occurred_at ASC, l.sequence_number ASC, l.id ASC
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
-- Seek to the next distinct instance using logs_client_instance_order_idx.
-- Work scales with the number of instances rather than the number of log rows.
WITH RECURSIVE scope AS (
    SELECT c.id FROM clients c
    WHERE c.id = sqlc.arg(client_id)
      AND c.project_id = sqlc.arg(project_id)
), names(instance_name) AS (
    (
        SELECT l.instance_name FROM logs l
        WHERE l.client_id = (SELECT scope.id FROM scope)
        ORDER BY l.instance_name
        LIMIT 1
    )
    UNION ALL
    SELECT next_name.instance_name
    FROM names previous
    CROSS JOIN LATERAL (
        SELECT l.instance_name FROM logs l
        WHERE l.client_id = (SELECT scope.id FROM scope)
          AND l.instance_name > previous.instance_name
        ORDER BY l.instance_name
        LIMIT 1
    ) next_name
)
SELECT instance_name FROM names ORDER BY instance_name;
