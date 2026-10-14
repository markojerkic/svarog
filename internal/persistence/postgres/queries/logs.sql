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
