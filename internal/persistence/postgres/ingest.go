package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/markojerkic/svarog/internal/persistence/postgres/sqlc"
)

// InsertIngestLogs commits new events and skips events still present in logs.
// Only newly inserted rows are returned, in input order. Archiving removes the
// event key with the log; this is intentionally not a permanent receipt ledger.
func InsertIngestLogs(ctx context.Context, pool *pgxpool.Pool, logs []IngestLogInsert) ([]InsertedIngestLog, error) {
	if len(logs) == 0 {
		return nil, nil
	}
	arg := sqlc.InsertIngestLogsParams{
		ClientIds: make([]uuid.UUID, len(logs)), InstanceNames: make([]string, len(logs)),
		OccurredAt: make([]time.Time, len(logs)), SequenceNumbers: make([]int64, len(logs)),
		Messages: make([]string, len(logs)), EventKeys: make([][]byte, len(logs)),
	}
	for i := range logs {
		log := &logs[i]
		arg.ClientIds[i] = log.Log.ClientID
		arg.InstanceNames[i] = log.Log.InstanceName
		arg.OccurredAt[i] = log.Log.OccurredAt
		arg.SequenceNumbers[i] = log.Log.SequenceNumber
		arg.Messages[i] = log.Log.Message
		arg.EventKeys[i] = log.EventKey[:]
	}
	rows, err := sqlc.New(pool).InsertIngestLogs(ctx, arg)
	if err != nil {
		return nil, fmt.Errorf("insert ingest logs: %w", err)
	}
	inserted := make([]InsertedIngestLog, len(rows))
	for i, row := range rows {
		inserted[i] = InsertedIngestLog{Ordinal: row.Ordinal, ID: row.ID}
	}
	return inserted, nil
}
