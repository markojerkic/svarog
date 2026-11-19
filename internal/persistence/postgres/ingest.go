package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/markojerkic/svarog/internal/persistence/postgres/sqlc"
)

// InsertIngestLogs checks client ownership and activity in the insert statement
// and returns one outcome per input row, in input order. Deduplication keys
// last only as long as the corresponding logs are retained.
func InsertIngestLogs(ctx context.Context, pool *pgxpool.Pool, logs []IngestLogInsert) ([]IngestLogResult, error) {
	if len(logs) == 0 {
		return nil, nil
	}
	arg := sqlc.InsertIngestLogsParams{
		ClientIds: make([]uuid.UUID, len(logs)), ProjectIds: make([]uuid.UUID, len(logs)),
		InstanceNames: make([]string, len(logs)),
		OccurredAt:    make([]time.Time, len(logs)), SequenceNumbers: make([]int64, len(logs)),
		Messages: make([]string, len(logs)), EventKeys: make([][]byte, len(logs)),
	}
	for i := range logs {
		log := &logs[i]
		arg.ClientIds[i] = log.Log.ClientID
		arg.ProjectIds[i] = log.ProjectID
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
	results := make([]IngestLogResult, len(rows))
	for i, row := range rows {
		status := IngestLogDuplicate
		switch {
		case !row.ActiveClient:
			status = IngestLogRejectedClient
		case row.ID != 0:
			status = IngestLogInserted
		}
		results[i] = IngestLogResult{Ordinal: row.Ordinal, ID: row.ID, Status: status}
	}
	return results, nil
}
