package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// sqlc v1.31.1 cannot type multi-argument unnest, so this batch insert stays handwritten.
const insertLogs = `
WITH input AS (
    SELECT
        nextval(pg_get_serial_sequence('logs', 'id')) AS id,
        row.client_id,
        row.instance_name,
        row.occurred_at,
        row.sequence_number,
        row.message,
        row.ordinality
    FROM unnest(
        $1::uuid[],
        $2::text[],
        $3::timestamptz[],
        $4::bigint[],
        $5::text[]
    ) WITH ORDINALITY AS row(
        client_id,
        instance_name,
        occurred_at,
        sequence_number,
        message,
        ordinality
    )
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
ORDER BY input.ordinality
`

func InsertLogs(ctx context.Context, pool *pgxpool.Pool, logs []LogInsert) ([]int64, error) {
	if len(logs) == 0 {
		return nil, nil
	}
	clientIDs := make([]uuid.UUID, len(logs))
	instanceNames := make([]string, len(logs))
	occurredAt := make([]time.Time, len(logs))
	sequenceNumbers := make([]int64, len(logs))
	messages := make([]string, len(logs))
	for i, log := range logs {
		clientIDs[i] = log.ClientID
		instanceNames[i] = log.InstanceName
		occurredAt[i] = log.OccurredAt
		sequenceNumbers[i] = log.SequenceNumber
		messages[i] = log.Message
	}

	rows, err := pool.Query(ctx, insertLogs, clientIDs, instanceNames, occurredAt, sequenceNumbers, messages)
	if err != nil {
		return nil, fmt.Errorf("insert logs: %w", err)
	}
	defer rows.Close()

	ids := make([]int64, 0, len(logs))
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("insert logs: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("insert logs: %w", err)
	}
	if len(ids) != len(logs) {
		return nil, fmt.Errorf("insert logs: got %d ids for %d rows", len(ids), len(logs))
	}
	return ids, nil
}
