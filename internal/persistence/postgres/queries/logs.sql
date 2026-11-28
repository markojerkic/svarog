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

-- name: SearchLogs :many
WITH scoped AS (
    SELECT l.id, l.client_id, c.project_id, l.instance_name, l.occurred_at, l.sequence_number, l.message,
           l.message ILIKE sqlc.arg(primary_pattern)::text ESCAPE E'\\'
           AND l.message ILIKE ALL(sqlc.arg(remaining_patterns)::text[])
           AND (sqlc.narg(instances)::text[] IS NULL OR l.instance_name = ANY(sqlc.narg(instances)::text[])) AS matches
    FROM logs l
    JOIN clients c ON c.id = l.client_id
    WHERE c.project_id = sqlc.arg(project_id)
      AND l.client_id = sqlc.arg(client_id)
      -- A single instance has its own ordered index. Start the probe there.
      AND (cardinality(sqlc.narg(instances)::text[]) IS DISTINCT FROM 1
           OR l.instance_name = (sqlc.narg(instances)::text[])[1])
      AND l.occurred_at >= COALESCE(sqlc.narg(after_at)::timestamptz, '-infinity'::timestamptz)
      AND l.occurred_at <= COALESCE(sqlc.narg(through_at)::timestamptz, 'infinity'::timestamptz)
      AND (l.occurred_at, l.sequence_number, l.id) >
          (COALESCE(sqlc.narg(after_at)::timestamptz, '-infinity'::timestamptz),
           sqlc.arg(after_sequence)::bigint, sqlc.arg(after_id)::bigint)
      AND (l.occurred_at, l.sequence_number, l.id) <=
          (COALESCE(sqlc.narg(through_at)::timestamptz, 'infinity'::timestamptz),
           sqlc.arg(through_sequence)::bigint, sqlc.arg(through_id)::bigint)
), candidates AS (
    -- With dates, filter before LIMIT so either search index is available.
    -- Without dates, inspect at most eight pages of consecutive client rows.
    SELECT * FROM scoped
    WHERE sqlc.arg(probe)::boolean OR matches
    ORDER BY CASE WHEN sqlc.arg(newer)::boolean THEN occurred_at END ASC,
             CASE WHEN sqlc.arg(newer)::boolean THEN sequence_number END ASC,
             CASE WHEN sqlc.arg(newer)::boolean THEN id END ASC,
             occurred_at DESC, sequence_number DESC, id DESC
    LIMIT (CASE WHEN sqlc.arg(probe)::boolean THEN 8 ELSE 1 END)::bigint * sqlc.arg(page_size)::int
)
SELECT id, client_id, project_id, instance_name, occurred_at, sequence_number, message
FROM candidates
WHERE NOT sqlc.arg(probe)::boolean OR matches
-- Repeat the direction expressions so a custom plan can use the ordering of candidates.
ORDER BY CASE WHEN sqlc.arg(newer)::boolean THEN occurred_at END ASC,
         CASE WHEN sqlc.arg(newer)::boolean THEN sequence_number END ASC,
         CASE WHEN sqlc.arg(newer)::boolean THEN id END ASC,
         occurred_at DESC, sequence_number DESC, id DESC
LIMIT sqlc.arg(page_size);

-- name: SearchLogHits :many
-- Materialize only the sort keys so a small LIMIT cannot favor an ordered
-- scan of an entire client. Fetch messages by ID only for the requested page.
WITH hits AS MATERIALIZED (
    SELECT l.id, c.project_id, l.occurred_at, l.sequence_number
    FROM logs l
    JOIN clients c ON c.id = l.client_id
    WHERE c.project_id = sqlc.arg(project_id)
      AND l.client_id = sqlc.arg(client_id)
      AND l.occurred_at >= COALESCE(sqlc.narg(after_at)::timestamptz, '-infinity'::timestamptz)
      AND l.occurred_at <= COALESCE(sqlc.narg(through_at)::timestamptz, 'infinity'::timestamptz)
      AND (l.occurred_at, l.sequence_number, l.id) >
          (COALESCE(sqlc.narg(after_at)::timestamptz, '-infinity'::timestamptz),
           sqlc.arg(after_sequence)::bigint, sqlc.arg(after_id)::bigint)
      AND (l.occurred_at, l.sequence_number, l.id) <=
          (COALESCE(sqlc.narg(through_at)::timestamptz, 'infinity'::timestamptz),
           sqlc.arg(through_sequence)::bigint, sqlc.arg(through_id)::bigint)
      AND l.message ILIKE sqlc.arg(primary_pattern)::text ESCAPE E'\\'
      AND l.message ILIKE ALL(sqlc.arg(remaining_patterns)::text[])
      AND (sqlc.narg(instances)::text[] IS NULL OR l.instance_name = ANY(sqlc.narg(instances)::text[]))
), page AS (
    SELECT hits.*,
           CASE WHEN sqlc.arg(newer)::boolean THEN hits.occurred_at END AS newer_at,
           CASE WHEN sqlc.arg(newer)::boolean THEN hits.sequence_number END AS newer_sequence,
           CASE WHEN sqlc.arg(newer)::boolean THEN hits.id END AS newer_id
    FROM hits
    ORDER BY newer_at, newer_sequence, newer_id,
             occurred_at DESC, sequence_number DESC, id DESC
    LIMIT sqlc.arg(page_size)::int
)
SELECT l.id, l.client_id, page.project_id, l.instance_name, l.occurred_at, l.sequence_number, l.message
FROM page
JOIN logs l ON l.id = page.id
ORDER BY page.newer_at, page.newer_sequence, page.newer_id,
         page.occurred_at DESC, page.sequence_number DESC, page.id DESC;

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
), palette AS (
    SELECT instance_name,
           (row_number() OVER (ORDER BY instance_name) - 1) * 360::float8 / count(*) OVER () AS hue
    FROM names
)
SELECT instance_name, hue::float8 AS hue FROM palette
WHERE sqlc.narg(visible_names)::text[] IS NULL OR instance_name = ANY(sqlc.narg(visible_names)::text[])
ORDER BY instance_name;

-- name: ListInstanceOptions :many
-- The first entry for each name in logs_client_instance_order_idx is its latest log.
WITH RECURSIVE scope AS (
    SELECT c.id FROM clients c
    WHERE c.id = sqlc.arg(client_id) AND c.project_id = sqlc.arg(project_id)
), latest(instance_name, occurred_at) AS (
    (
        SELECT l.instance_name, l.occurred_at FROM logs l
        WHERE l.client_id = (SELECT scope.id FROM scope)
        ORDER BY l.instance_name, l.occurred_at DESC LIMIT 1
    )
    UNION ALL
    SELECT n.instance_name, n.occurred_at FROM latest prev
    CROSS JOIN LATERAL (
        SELECT l.instance_name, l.occurred_at FROM logs l
        WHERE l.client_id = (SELECT scope.id FROM scope) AND l.instance_name > prev.instance_name
        ORDER BY l.instance_name, l.occurred_at DESC LIMIT 1
    ) n
), options AS (
    SELECT instance_name, (occurred_at AT TIME ZONE 'UTC')::date AS last_day
    FROM latest
    WHERE strpos(lower(COALESCE(NULLIF(instance_name, ''), sqlc.arg(unnamed_label)::text)), lower(sqlc.arg(term)::text)) > 0
)
SELECT instance_name, last_day FROM options
WHERE sqlc.narg(after_day)::date IS NULL OR last_day < sqlc.narg(after_day)
   OR (last_day = sqlc.narg(after_day)
       AND (lower(instance_name), instance_name) > (lower(sqlc.arg(after_name)::text), sqlc.arg(after_name)::text))
ORDER BY last_day DESC, lower(instance_name), instance_name
LIMIT sqlc.arg(page_size);
