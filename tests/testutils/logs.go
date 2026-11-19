package testutils

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/markojerkic/svarog/internal/server/logstore"
)

// InsertLogFixtures seeds retained rows, including records without event keys.
// Returned IDs correspond to the input rows.
func InsertLogFixtures(ctx context.Context, pool *pgxpool.Pool, logs []logstore.Row) ([]int64, error) {
	clients := make([]uuid.UUID, len(logs))
	instances := make([]string, len(logs))
	timestamps := make([]time.Time, len(logs))
	sequences := make([]int64, len(logs))
	messages := make([]string, len(logs))
	for i, row := range logs {
		clients[i], instances[i] = row.Scope.ClientID, row.Instance
		timestamps[i], sequences[i], messages[i] = row.OccurredAt, row.Sequence, row.Message
	}
	rows, err := pool.Query(ctx, `
WITH input AS (
    SELECT
        nextval(pg_get_serial_sequence('logs', 'id')) AS id,
        c.client_id,
        i.instance_name,
        o.occurred_at,
        s.sequence_number,
        m.message,
        c.ordinality
    FROM unnest($1::uuid[]) WITH ORDINALITY AS c(client_id, ordinality)
    JOIN unnest($2::text[]) WITH ORDINALITY AS i(instance_name, ordinality) USING (ordinality)
    JOIN unnest($3::timestamptz[]) WITH ORDINALITY AS o(occurred_at, ordinality) USING (ordinality)
    JOIN unnest($4::bigint[]) WITH ORDINALITY AS s(sequence_number, ordinality) USING (ordinality)
    JOIN unnest($5::text[]) WITH ORDINALITY AS m(message, ordinality) USING (ordinality)
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
 `, clients, instances, timestamps, sequences, messages)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[int64])
}
